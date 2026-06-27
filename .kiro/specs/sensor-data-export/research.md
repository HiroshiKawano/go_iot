# ギャップ分析（sensor-data-export）

> `/kiro-validate-gap` による要件↔既存コードのギャップ分析。設計フェーズ（`/kiro-spec-design`）の判断材料。意思決定ではなく選択肢と制約の提示。
> 確認日: 2026-06-27（P1/P2/P3 マージ後・HEAD: future/phase-04-sensor-data-export）

## 分析サマリ

- **CSV 出力はハンドラ層では完全新規**: コードベース全体に `encoding/csv`・`Content-Disposition`・`attachment`・export 経路は**一つも存在しない**（grep 確認）。CSV のファイル応答・文字コード・エスケープ・ファイル名・ストリーミングは新規実装で、本フェーズ最大の未知領域。
- **集計の計算資産は完全に揃っている**: `internal/chart`（`Mean`/`MinMax`/`DiurnalRange`/`StdDev`/`CV`/`VPDSeries`/`TimeInRange`）と、JST 暦日バケット作法 `dailyStatRows`（device_show.go）・JST 時刻バケット作法 `vpdHourlyRows`（device_show_vpd.go）が既存。集計帳票は**新規計算をほぼ書かず流用で組める**。
- **期間フィルタ・認可・consumer interface も既存**: `parseDateBounds`（package-level・共有可能）／`authz.RequireDeviceOwner`（非所有・不在とも 404）／`ReadingsRepo`（DIP 最小 interface）がそのまま使える。
- **不足クエリは1本**: BETWEEN・昇順・LIMIT なしの「期間内全行取得」クエリが無い（既存 BETWEEN 系は Paginated/Summary/Count のみ）。日次 GROUP BY も `ListDailySensorAggregates` は `recorded_at >= $2` の**単一境界**かつ `DATE()` がサーバ TZ 依存で JST バケットに使えない。
- **核心の設計緊張**: 「CSV ストリーミングで大期間に耐える（Req9）」と「集計帳票を Go 側で全行集計する（新 SQL 不要・JST 整合）」が、**同じ全行取得を共有するか／別経路にするか**で衝突する。これが design の最重要判断。

## 現状調査（資産棚卸し）

### 再利用できる既存資産

| 資産 | 場所 | 本フェーズでの用途 |
|---|---|---|
| `parseDateBounds(from,to)` | readings.go:213 | CSV・帳票・一覧の**区間共有**（未指定センチネル 1970/9999・to は end-of-day・JST 暦日・形式不正は errs）。package-level 関数でそのまま呼べる |
| `ReadingsHandler` / `ReadingsRepo` | readings.go:39-52 | 拡張点。consumer 最小 interface にクエリを追加するだけで DI 維持 |
| `authz.RequireDeviceOwner` | readings.go:69 経由 | CSV/帳票の所有者認可（非所有・不在→404 列挙防止）を同型で流用 |
| `chart.Mean/MinMax/DiurnalRange/StdDev/CV` | stats.go:71-107 | 日次/時間別の 平均/最高/最低/日較差/σ/CV 列（`[]float64` 純関数・time 非依存） |
| `chart.VPDSeries` / `chart.TimeInRange` | vpd.go:41,57 | 適正帯滞在率（VPD 系列→在帯率 0..1） |
| `domain.Crop.VPDRange()` / `DefaultVPDLower/Upper` | crop.go:30,73 | 作物別適正帯・未設定既定 0.3〜1.5 kPa。作物別=ゴーヤ等 0.4-1.2／葉野菜 0.3-1.0 |
| `dailyStatRows(rows, pick)` → `component.DailyStatRow` | device_show.go:381 | JST 暦日バケット集計の**作法の手本**（temp/hum 別に pick 関数で抽出・空日は emptyDailyRow・`statEmptyMark="—"`） |
| `vpdHourlyRows(rows, vpd, lower, upper)` → `component.VPDHourlyRow` | device_show_vpd.go:127 | JST hour-of-day(0-23) バケットの手本（純粋層に time を持ち込まず handler 境界で `In(jst)`） |
| `jst` / `statEmptyMark` / `formatActual` / `formatPercent` / `pgconv.*` | handler パッケージ内 | 整形ヘルパ共用 |
| `.data-table` / `.summary-grid` / `.filter-form` / `.card` | mocks/html/readings.html ＋ style.css | 集計帳票表・CSV ボタン・項目フィルタの「器」を**独自クラス新設せず**流用 |
| ルート node `/devices/:device` | main.go:156-165 | `/chart`・`/edit`・`/readings` と同階層。GET 子経路を追加（GET ゆえ CSRF 対象外） |

### 既存クエリの境界（CSV/帳票の不足を確定）

| クエリ | WHERE | 並び | LIMIT | 本フェーズでの可否 |
|---|---|---|---|---|
| `GetSensorReadingsSummary` | `BETWEEN $2 AND $3` | — | — | 集計ボックス（既存・無回帰維持） |
| `ListSensorReadingsPaginated` | `BETWEEN $2 AND $3` | DESC | あり | 一覧（既存）。CSV の「全行」には LIMIT が邪魔 |
| `CountSensorReadingsInRange` | `BETWEEN $2 AND $3` | — | — | ページ件数（既存） |
| `ListDailySensorAggregates` | `recorded_at >= $2`（**単一境界**） | DATE 昇順 | — | ❌ 後端なし＋`DATE()` がサーバ TZ 依存で JST 帳票に使えない |
| **（不足）期間内全行** | `BETWEEN` 想定 | **ASC** | **なし** | **新設候補**（CSV と Go 集計の共通入力） |

## 要件 → 資産マップ（ギャップ種別: Missing / Unknown / Constraint）

| 要件 | 既存資産 | ギャップ |
|---|---|---|
| R1 CSV 全行ダウンロード（昇順・ページなし・項目フィルタ） | parseDateBounds / ReadingsRepo | **Missing**: BETWEEN・ASC・LIMIT なしの全行クエリ。**Missing**: CSV 生成（`encoding/csv`）。**Unknown**: 項目未選択時の既定（要件で温湿度両方と固定済） |
| R2 メタ情報（device名/locality/crop） | devices.locality/crop・Locality.Label・Crop.Label | **Unknown**: 各行反復 vs 先頭メタブロック（design 判断）。device 名は `device.Name` 既取得 |
| R3 可搬性（文字化け回避・エスケープ・添付・ファイル名） | — | **Missing**: 文字コード（BOM 付 UTF-8 or Shift_JIS）・`Content-Disposition`・RFC 5987 `filename*`。エスケープは `encoding/csv` に委任 |
| R4 日次帳票（平均/最高/最低/日較差/σ/CV/適正帯滞在率） | dailyStatRows 作法・stats・VPD | **Constraint**: 適正帯滞在率列は DailyStatRow に無い→新 View 構造。`DATE()` SQL は TZ 依存ゆえ Go バケット推奨 |
| R5 時間別帳票（JST 時間帯バケット） | vpdHourlyRows 作法 | **Missing**: 温湿度版の時間別行（VPDHourlyRow は VPD 専用）。**Unknown**: バケット幅 1h or 昼夜 |
| R6 適正帯滞在率（作物別・未設定既定） | VPDSeries+TimeInRange+Crop.VPDRange | ほぼ流用可。バケット単位で VPDSeries→TimeInRange を回す薄い実装 |
| R7 期間フィルタ共有・値一致 | parseDateBounds 共有 | **Constraint**: CSV・帳票・一覧が同一 (fromTS,toTS) を使う実装規律。単一源を崩さない |
| R8 所有者認可・S6 無回帰 | RequireDeviceOwner / 既存 Index | 流用可。**Constraint**: 既存 Index・クエリを壊さない（追加のみ） |
| R9 大期間で安定 | — | **Unknown/最重要**: ストリーミング（逐次 Flush／バッチ）と、帳票の Go 全行集計が要するメモリの両立 |

## 実装アプローチ（CSV）

### Option A: 既存 Index に `?format=csv` 分岐を追加（extend）
- **内容**: `ReadingsHandler.Index` 冒頭で `c.Query("format")=="csv"` を判定し HTML/ファイル応答を分岐。
- ✅ ルート1本・フィルタ完全共有。❌ HTML 描画に集中した Index（readings.go 311行）にファイル応答・ストリーミングが混入し単一責務が崩れる。❌ テストが HTML/CSV 両モードで肥大。

### Option B: 専用ハンドラ＋専用経路（new・推奨）
- **内容**: 新ファイル `readings_export.go` に `ReadingsHandler.Export`（または別ハンドラ）を追加し、新経路 `GET /devices/:device/readings.csv`（or `/export.csv`）を main.go:165 隣に登録。`parseDateBounds`・`RequireDeviceOwner` を共有。CSV 整形は純度の高いヘルパ（`io.Writer` 受け）に切り出してテスト容易化。
- ✅ ファイル応答・ストリーミングを HTML から分離。✅ CSV ヘルパを単体テスト可能。✅ readings.go は無改修に近い。❌ ルート1本増・interface に全行クエリ追加。
- **経路の注意**: `:device` は param node。`/devices/:device/readings` と `/devices/:device/readings.csv` は静的セグメント違いの兄弟で Gin で共存可（`/devices/create` 静的と `:device` 共存実績あり）。`?format=csv` 案は Option A 寄り。最終形は design 判断。

### Option C: ハイブリッド（推奨の実体）
- `parseDateBounds`（共有・既存）＋ 新全行クエリ（共有入力）＋ 専用 Export ハンドラ（Option B）＋ 帳票は Index 側で組み立て（後述）。CSV と帳票が**同一の全行取得**を入力にできるかは Req9 と要調整（下記 Research）。

## 実装アプローチ（集計帳票）

### Option A: Go 側で全行集計（新 SQL 不要・推奨）
- **内容**: 期間内全行を取得（CSV と同じ新クエリ）し、`dailyStatRows`／`vpdHourlyRows` を温湿度＋VPD 用に一般化して JST バケット集計。適正帯滞在率は各バケットで `VPDSeries→TimeInRange`。
- ✅ 新規 SQL を増やさない。✅ JST バケットを Go 境界で統一（`DATE()` の TZ バグ回避）。✅ CSV と入力共有で値が構造的に一致（R7）。❌ 大期間で全行をメモリに載せる（Req9 と緊張・帳票は表示用ゆえ期間上限で緩和可）。

### Option B: BETWEEN 版の日次集計 SQL を新設（DB 集計）
- **内容**: `ListDailySensorAggregates` の BETWEEN・JST 版（`DATE(recorded_at AT TIME ZONE 'Asia/Tokyo')` 等）を新設。
- ✅ DB 側集計でメモリ軽い。❌ 適正帯滞在率は VPD 計算が要り SQL で完結しない（結局 Go へ）。❌ σ/CV/日較差まで SQL に寄せると複雑。❌ TZ 変換を SQL に持ち込む新たな注意点。
- **既存資産との整合**: device_show 側は Go バケット（dailyStatRows）。SQL 集計に分岐すると作法が二系統化。

### 推奨の方向（design で確定）
- 帳票は **Option A（Go 全行集計）** が既存作法・JST 整合・新 SQL 最小の点で有力。CSV と帳票が同一全行を共有できれば R7 が自然に満たされる。ただし **Req9（大期間ストリーミング）** との両立が要設計（下記）。

## 工数・リスク

| 区分 | 工数 | リスク | 根拠 |
|---|---|---|---|
| CSV エクスポート | **M（3-7日）** | **Medium** | 計算資産は流用だが、文字コード/BOM・RFC 5987 ファイル名・ストリーミング・添付ヘッダは新規かつ既知の落とし穴多数。新クエリ1本＋テスト（整形/エスケープ/メタ/空期間/大量行） |
| 集計帳票（日次/時間別） | **M（3-7日）** | **Medium** | バケット作法は流用できるが、温湿度＋適正帯滞在率の新 View 構造・templ・モック反映（単一ソース）が必要。JST 境界・空バケット・日跨ぎのテスト |
| **全体** | **M** | **Medium** | 既存パターン上の拡張だが、CSV の可搬性と Req9 の両立、モック反映、無回帰維持が同時に要る |

## 設計フェーズへの申し送り

### 推奨アプローチ（提案・design で確定）
1. **CSV = Option B/C（専用ハンドラ＋専用経路＋共有 parseDateBounds）**。CSV 整形は `io.Writer` 受けの純度高いヘルパに切り出す。
2. **帳票 = Option A（Go 全行集計・新 SQL 最小）**。`dailyStatRows`/`vpdHourlyRows` を温湿度＋適正帯滞在率へ一般化（重複実装を避け device_show 作法に揃える）。
3. **全行取得クエリ1本を新設**（BETWEEN・ASC・LIMIT なし）し、CSV と帳票の共通入力にする。`ReadingsRepo` に追加。`make sqlc`（DDL なし・goose 00009 のまま・`make db-snapshot` 不要）。
4. **モック反映**: 項目フィルタ・CSV ボタン・集計帳票表を `mocks/html/readings.html` ＋ `style.css`（正本）へ追加し `make sync-css`。CSV ファイル出力自体は描画を伴わずモック対象外。

### Research Needed（design で判断・要件の未確定事項と対応）
- **【最重要】Req9 とメモリの両立**: CSV ストリーミング（逐次 Flush／バッチ・カーソル）と、帳票の Go 全行集計が要する全行ロードをどう両立するか。案: (a) CSV はバッチ取得でストリーム、帳票は別途（期間上限 or 集計 SQL）／(b) 帳票はメモリ許容（UI 期間で有界）し CSV のみストリーム。**集計と CSV で取得経路を分けるか共有するかの判断**。
- **CSV 文字コード**: BOM 付 UTF-8 か Shift_JIS（Excel 直開き vs R/Python）。客の利用先を引継ぎメモ/デモで確認。
- **CSV エンドポイント形**: `/readings.csv` か `/export.csv` か `?format=csv`（`:device` param node と非競合な形）。
- **CSV ファイル名**: device 名＋期間を含めるか。日本語は RFC 5987 `filename*`＋ASCII フォールバック。
- **メタ情報の持たせ方**: 各行反復 vs 先頭メタブロック（外部 pivot しやすさ優先）。
- **項目フィルタの範囲**: 温度/湿度のみか、VPD 等派生列も選べるか（VPD は VPDSeries で算出可・露点は P6 まで不可）。
- **時間別バケット幅**: 1時間（hour-of-day）か昼夜区分か。長期間時の粒度出し分け（時間別＝短期間・日次＝長期間）。
- **欠測の扱い**: CSV で欠測行を出さない／空セル。帳票の空バケットは「—」（既存 emptyDailyRow に整合）。
- **結露時間列（スコープ判断）**: 露点 Td（P6）依存で未実装。本フェーズ帳票は P6 で露点列を**非破壊追加**できる列構造に留めるか、最小露点近似を含めるか。**要件では out of scope** として確定済（design で列構造の拡張余地のみ担保）。
- **テスト方針**: `2cc_sdd/テストガイダンス集.md` の「CRUD・CSV」節を design の Testing Strategy で参照（`encoding/csv` の往復検証・httptest でのヘッダ/Content-Disposition 検証・大量行ストリーミングの検証手法）。

---

# 設計フェーズ discovery / synthesis（2026-06-27 追記）

> `/kiro-spec-design -y` 実行時の Light Discovery（Extension）＋ Synthesis 所見。design.md の判断根拠。

## Discovery（Extension・integration-focused）

### HTMX実装ガイド 所見（CSV/フィルタ/帳票の UI 制約）
- **§2 変換ルール（行483/1757）**: ファイルダウンロードは **HTMX 非対象**。`<a hx-boost="false">`（or 通常リンク/フォーム）で処理。CSV ボタンは HTMX swap させない。→ CSV ボタンは plain link、現在の from/to/items を query で引き継ぐ。
- **§4 readings 既存仕様（行1634-1654）**: フィルタフォームは GET＋`hx-push-url=true`、ターゲット `#device-readings-list`（集計+一覧+ページャを内包）、`innerHTML` swap。日付不正は 200＋インラインエラー＋空一覧。
- **§7 バリデーション（行1761-1914）**: GET 検索の form は swap 外ゆえエラーは fragment 内へ集約（既存 DeviceReadingsList の `.error-message`）。HTML/HTMX は 200＋空一覧、JSON は 422。→ CSV（ファイル経路）は 200/空一覧の対象外ゆえ **400** で「データ無出力」とする。
- **§10 ページネーション（行2326-2424）**: ページャは fragment 内・`hx-target=#device-readings-list`。帳票表も fragment 内に置けばフィルタ/ページ送りで同時更新される。
- **§31（行4567-4600）**: `.data-table`/`.summary-box`/`.btn`/`.empty-message`/`.pagination` を流用・独自クラス新設禁止。器の変更（input→checkbox 追加）はモック正本へ先に反映してから templ 写経。
- **§40-B（行5120-5176）**: CSS 正本＝`mocks/html/style.css`、`make sync-css` で配信へ。追加 CSS は `@layer components` へ最小追記。
- **記述なし**: Content-Disposition/BOM/download 属性の実装ガイドは HTMX ガイドに無し（CSV 整形は本 design＋テストガイダンス §33 に従う）。

### 配置判断（CSV ボタン・帳票表）
- **CSV ボタン＋帳票表とも fragment `#device-readings-list` 内**に置く。handler が適用済み from/to/items で `CSVURL` を再生成するため、フィルタ適用ごとに CSV リンクが最新化され、表示データと CSV が同一区間で一致する（R7）。項目チェックボックスは form 内（fragment 外）で状態保持。

### テストガイダンス集 所見（Testing Strategy 導出）
- **§33 CSV エクスポート（行5563-5690）**: `httptest` の `w.Body.Bytes()` で全量検証。Content-Type/Content-Disposition・UTF-8 BOM 先頭3バイト・`csv.NewReader` 往復・エスケープ・期間適用/ページング無視・大量行を検証。→ design Testing Strategy へ反映済。
- **§3 ステータス（238-317）/§6 認可（2214-2287）**: 200/400/404/500 分岐、非所有→404＋**DB 副作用ゼロを captor で確認**（BOLA）。
- **§5 Querier モック（1298-1376）/§54**: 新クエリ `ListSensorReadingsInRange` を手書きモックに override 追加（埋め込み Querier ＋ nil スタブ）。
- **§4/§20 templ（679-760/4249-）**: 帳票 `.data-table` を `Render→strings.Contains` で検証。
- **§49 カバレッジ80%**: GET 表示・各 DB 500・空期間・形式不正の経路網羅。

### DB スキーマ現状（権威）
- `sensor_readings`: temperature/humidity numeric(5,2)・recorded_at/created_at timestamptz・(device_id, recorded_at DESC) 部分索引 WHERE deleted_at IS NULL。CSV/帳票は2列＋メタから導出可。
- `devices`: locality VARCHAR(20)（CHECK・53地域）・crop VARCHAR(20)（CHECK・9作物）。ともに NULL 許容＝未設定。**新規カラム不要・スキーマ非変更**。

## Synthesis

### 1. Generalization
- CSV 全行取得（R1）と帳票バケット集計（R4/R5）は**同一の「期間内全行（BETWEEN・ASC）」を入力**にできる。→ 単一クエリ `ListSensorReadingsInRange` を両者の共通源にし、R7（値一致）を構造的に担保。`dailyStatRows`/`vpdHourlyRows`（device_show）は「JST バケット集計」の一般形ゆえ、温湿度＋適正帯滞在率版へ流用（実装の特殊化のみ追加）。

### 2. Build vs Adopt
- **Adopt**: 標準 `encoding/csv`（エスケープ/引用符）、既存 `chart` 純粋層（統計/VPD）、`parseDateBounds`/`authz`/`Crop.VPDRange`。新規計算・新規 DB ポートは作らない。
- **Build（最小）**: CSV 整形ヘルパ（BOM/メタ列/RFC5987 ファイル名＝stdlib に無い部分）と、温湿度＋滞在率の帳票バケット組立のみ。

### 3. Simplification
- **キーセットバッチ・ストリーミングは作らない**（年スケールは現要件外＝R9 の bar は「数ヶ月」）。全行 materialize＋逐次 Flush で数ヶ月を満たす最小実装。年スケール顕在化時に interface 互換でバッチ版へ差し替える（Risk 記録）。
- **BETWEEN 日次集計 SQL を新設しない**（`DATE()` の TZ 依存バグ回避＋Go バケットへ一本化）。集計 SQL は増やさず Go 側に寄せる。
- CSV と帳票で**取得経路を分離しない**（同一クエリ共有）。ただし CSV は逐次 Flush（R9）、帳票は materialize（期間有界）と書き出し方のみ分岐。

## Design Decisions（確定）
| 決定 | 採用 | 根拠 |
|---|---|---|
| CSV 経路 | `GET /devices/:device/readings.csv` 専用ハンドラ | HTML 描画から分離（単一責務）・`:device` 静的兄弟で共存見込み |
| 文字コード | UTF-8 + BOM | Excel 直開き互換・R/Python も許容（§33 定石）。Shift_JIS は1関数差替で対応可 |
| メタ列 | 各行反復（device名/地点/作物） | 外部 pivot しやすさ優先（先頭メタブロックは非採用） |
| 全行クエリ | `ListSensorReadingsInRange` 1本（BETWEEN/ASC/LIMIT なし） | CSV・帳票の共通源・DDL なし |
| 帳票集計 | Go バケット（chart 流用・JST 境界） | `DATE()` TZ バグ回避・既存作法一貫・新 SQL 最小 |
| R9 | materialize＋逐次 Flush | 数ヶ月 bar を満たす最小（キーセットは将来の非破壊差替） |
| 項目フィルタ | 温度/湿度のみ（未選択は両方） | 派生指標(VPD等)は本フェーズ対象外（将来余地） |
| 時間別バケット | hour-of-day(0-23) | `vpdHourlyRows` と一貫 |

## Risks & Mitigations
- 年スケール CSV のメモリ圧 → キーセットバッチ版へ interface 互換で差替（本フェーズ未実装）。
- `/readings.csv` の Gin ルート競合 → 実装時に登録順含め検証（静的セグメント兄弟）。
- 帳票の列数（温度6＋湿度6＋滞在率）が横長 → `.table-wrapper` の横スクロールで吸収（既存クラス）。モック正本で確認。
