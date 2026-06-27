# Gap Analysis — device-location-select（圃場所在地の住所セレクト）

> 生成: `/kiro-validate-gap`（2026-06-26）／ 対象要件: `.kiro/specs/device-location-select/requirements.md`（R1〜R8）
> 方針: **情報提供であり決定ではない**（gap-analysis.md 原則）。design フェーズが選ぶための Option A/B/C・工数/リスク・Research Needed を整理する。
> 調査法: 6観点の並行コードベース調査（master-data / migration / seed / form-handler-view / tomselect-htmx / test-pattern）。

---

## 0. 調査サマリ（最重要の発見）

1. **規約衝突（最重要）**: steering `structure.md` §98-100 が **「外部キー制約は張らない」「マスターデータは DB テーブルではなく Go 定数 + VARCHAR + CHECK 制約」「参照/マスタ系テーブルは現状ゼロ」** を確立規約として定めている。実コードでも FK は 0 件、マスタテーブルは 0 件。phase-01 プロンプトが前提にした「市町村マスタ**テーブル** + 地区マスタ**テーブル** + FK」は**この規約と正面衝突**する。→ 中核設計判断（§4）。
2. **41市町村と地区の非対称**: 41市町村は既存 enum 規約（`domain.Metric` 型 + `VARCHAR + CHECK IN(...)` + `binding:"oneof=..."`）の素直な延長で表せる。一方 **地区（旧町村/大字＝数百件）は CHECK 列挙が非現実的・`oneof` で親依存を表現不能・カスケード（親子）の前例ゼロ**で、規約が破綻する。地区の扱いが設計の山場。
3. **カスケード select はコード前例ゼロ・ガイドにのみ存在**: 市町村→地区の連動 select は実装前例が無い。`HTMX実装ガイド §16(TS-1〜)・C12` に「ラッパー div を hx-target にし `<select>` タグ全体を返す」公式パターンが文書化済みだが**初実装**。加えて **App.templ には Tom Select 用の `htmx:beforeSwap/afterSwap` ライフサイクルが未配線**（ECharts 版のみ実装済み）の可能性が高く、配線追加が要る。
4. **マイグレーションの初パターン**: `ALTER TABLE ADD COLUMN` も migration 内 `INSERT/UPDATE`（backfill）も**前例ゼロ**。所在地は任意（既存 `location` が nullable）なので新カラムは nullable でよく、NOT NULL backfill 圧力は無い。
5. **拡張点は明確**: フォーム/ハンドラ/View は手本が揃う（`AlertRuleForm` の `<option selected?=...>`、`is_active` radio 復元、`DeviceOption` DTO、`DeviceRepo`=Querier 部分 interface）。テストも `fakeDeviceRepo`＋`render→Contains` の確立形。ただし**実DBテスト用ヘルパ（`openTestDB`/`truncateAll`）は未実装**で、マスタ件数や backfill 検証を DB で書くなら先に用意が要る（→ Go定数案ならこの負担が無い）。

**総合推奨（design への提案・決定ではない）**: 既存規約への整合と新規インフラ最小化の観点から、**Option A（市町村・地区とも Go 定数）を起点**とし、地区データが大量・可変と判明したら**地区のみ DB テーブル化（Option C）**へ段階移行する。律速は「地区の粒度・件数・出典」の確定（Research Needed #1）。

---

## 1. 現状調査（既存資産と確立規約）

### 1-1. 確立規約（design が必ず従う前提）

| 規約 | 内容 | 典拠 |
|---|---|---|
| **マスタ = Go 定数** | enum/選択肢系は DB テーブルにせず `internal/domain` に `type X string` + const + `Label()/Valid()/ParseX()/AllX()` の5点セット。DB は `VARCHAR(N) + CHECK IN(...)`、フォームは `binding:"oneof=..."` で三重ミラー | structure.md §100 / `domain/metric.go:8-58`・`comparison_operator.go:8-74` |
| **FK 張らない** | 参照整合性はアプリ層 JOIN。`user_id`/`device_id` 等は制約無しの素の `BIGINT` | structure.md §98 / migrations に `FOREIGN KEY` 0件 |
| **マスタテーブル 0件** | DB7表は全て業務データ（users/devices/device_tokens/sensor_readings/alert_rules/alert_histories/sessions）。reference/lookup 表は皆無 | migrations 00001-00007 |
| **論理削除** | 全クエリ `WHERE deleted_at IS NULL`、索引も部分索引 | structure.md §99 |
| **domain 純粋性** | `internal/domain` は `fmt` のみ依存。pgx/gin/templ を import しない | structure.md §49 / `domain/*.go` |
| **依存方向** | 下向き一方向。handler/auth は `repository.Querier`（interface）依存。view→repository/service 禁止（domain 表示メソッドは可） | structure.md §35-57 / tech.md データアクセス方針 |
| **CSS/モック単一ソース** | 正本 `mocks/html/style.css`・templ はモック写経・独自クラス新設禁止。フォーム器（input→select）変更はモックに先反映 | structure.md §78-94 / tech.md CSS方針 |

### 1-2. 再利用できる手本（拡張の足場）

- **enum 写経テンプレ**: `domain/metric.go`（`type Metric string`＋const＋`Label/Valid/ParseMetric/AllMetrics`）。市町村は同型で `domain/municipality.go` を新設可。
- **select + selected 復元**: `AlertRuleForm.templ` の `for _, m := range domain.AllMetrics() { <option value={string(m)} selected?={ v.Metric==string(m) }>{m.Label()}</option> }`（先頭に空「選択してください」）。
- **入力値復元**: `is_active` radio の `checked?={ v.IsActive=="1" }`（`device.go:radioFromIsActive`／`device_form.go:parseIsActive`）。
- **選択肢 DTO**: `component.DeviceOption{ID,Name,Selected}`（`alert_rule_view.go:5-11`）。市町村/地区も同形の `{Value,Label,Selected}` を踏襲。
- **フォーム検証配線**: `device_form.go` の `deviceForm`(binding タグ)＋`deviceFieldKey`＋`deviceValidationMessage`＋`toDeviceFieldErrors`。**新フィールドは `deviceFieldKey` と `deviceValidationMessage` 両 switch に case 追加が必須**（片方忘れると汎用文言に落ちる）。
- **handler 依存**: `DeviceHandler.Repo` は `DeviceRepo`（`repository.Querier` の部分 interface）。マスタ一覧取得が要るなら List メソッドを宣言追加→sqlc 生成で満たす。
- **非破壊・冪等 seed**: `cmd/seed-testsensor`（TRUNCATE せず一意キーで検索→無ければ INSERT）。`make seed` は全表 TRUNCATE で**本番禁止**（マスタを業務表群と混ぜない判断材料）。
- **テスト**: handler=`fakeDeviceRepo`(map＋`*Called`/`*Err`注入)＋`httptest`、view=`render(t,Component)→buffer→assertContains`、検証=`validator.New().SetTagName("binding")` の table-driven、sqlc 新クエリ=`var _ interface{...}=Querier(nil)` コンパイル時ガード。

---

## 2. 要件 → 資産マップ（gap タグ: ✅整備済 / ⚠️Constraint / ❓Unknown / ❌Missing）

| 要件 | 必要な技術要素 | 既存資産 | gap |
|---|---|---|---|
| R1 住所セレクト表示 | 市町村/地区 select、検索可能select、空option、任意項目 | AlertRuleForm select＋selected、initTomSelect(`js-tom-select`)、DeviceOption DTO | ✅器の手本あり／❌market/地区の選択肢源（§4で決定） |
| R2 カスケード | 市町村連動の地区候補更新（全画面再読込なし）、失敗時非劣化 | HTMXガイド §16 TS-1（ラッパーdiv）・C12 | ❌実装前例ゼロ・初実装／⚠️App.templ に Tom Select 用 before/afterSwap 未配線の疑い |
| R3 保存（登録・更新） | devices に所在地保存、既存フロー維持 | CreateDevice/UpdateDevice($4=location)、buildCreateView/buildEditView | ❌devices に市町村/地区カラム無し（migration＋sqlc拡張）／⚠️ALTER前例ゼロ |
| R4 選択値復元 | 編集/エラー時の selected 復元 | radio checked 復元・AlertRuleForm selected | ✅手本あり |
| R5 検証 | 存在しない市町村拒否、地区⊂市町村、地区のみ拒否、同時エラー | binding `oneof`＋`toDeviceFieldErrors`（CHECKミラー） | ⚠️市町村は oneof 可だが**地区は親依存で oneof 不能→handler 手続き検証が新規**／two-switch同期 |
| R6 表示（詳細・カード） | 市町村（地区併記）表示、未設定空表示 | DeviceInfoPanel `<dl>`・DeviceCard `.meta`・`deviceLocationOrDefault` | ✅追加箇所明確（views struct＋build関数＋templ行） |
| R7 非破壊移行 | 既存location→市町村ベストエフォート、不能はNULL残置、件数非減少 | seed-testsensor 非破壊冪等 | ❌migration内backfill前例ゼロ／❓backfill手段（migration UPDATE vs seed vs 手動）／⚠️実DBテストヘルパ未実装 |
| R8 マスタ整備 | 41市町村正規マスタ、地区連動、冪等非破壊投入 | domain enum 規約（Go定数）／seed CLI 規約 | ⚠️**規約はGo定数を指す（テーブルでない）**／❓地区の件数・出典 |

---

## 3. 中核設計判断（§4）の前提となる「地区の非対称性」

| 観点 | 市町村（41・固定） | 地区（旧町村/大字・数百件・粒度未確定） |
|---|---|---|
| Go enum 化 | ◎ `domain.Municipality` で metric 同型に可 | △ 数百 const をソースに持つのは重い |
| DB CHECK IN(...) | ○ 41件列挙は技術的に可（PostgreSQL上限なし） | ✕ 数百件列挙は可読性・migration肥大・追加時ALTERで非現実的 |
| `binding:"oneof=..."` | △ 41値で可だがタグ長大 | ✕ **親依存（選んだ市町村で許容が変わる）＝oneof 表現不能** |
| カスケード | — | ✕ 親子関係の domain/テーブル前例ゼロ |
| 検証 | oneof or `AllMunicipalities` 照合 | **handler 手続き検証必須**（`domain.Valid()`＋`DistrictsOf(親).contains(子)`） |

→ **市町村は規約に乗る／地区は規約を超える**。この非対称が Option を分ける。

---

## 4. 実装アプローチ（マスタ表現の中核判断）

### Option A — 市町村・地区とも Go 定数（規約純度・最小インフラ）
`internal/domain` に `municipality.go`（41 enum＋`Label/Valid/Parse/All`）と `district.go`（`type District string`＋`var districtsByMunicipality map[Municipality][]District`＋`DistrictsOf()`）を新設。devices には `municipality VARCHAR + CHECK IN(41)`／`district VARCHAR`（地区はCHECK無し・app層照合）を ALTER で追加。カスケードは domain データから handler が子 select を組む（DBマスタ参照なし）。

- ✅ structure.md §100 規約に完全整合。マスタテーブル/seed CLI/FK/実DBテストヘルパ**いずれも不要**。domain 純粋性（fmt のみ）維持。カスケードの「子候補」もGoから出るのでHTMX往復もDB不要（or クライアントJSONも容易）。
- ❌ 数百件の地区を**Goソースにハードコード**（大きな静的データファイル）。地区の参照整合は app層のみ（CHECK 無し）。マスタ更新＝コード変更＋デプロイ。
- 典拠: master-data reader（domain拡張点）／seed reader（Go定数なら migration/seed 不要）。

### Option B — DB マスタテーブル（スケール・更新容易だが規約逸脱）
`municipalities`/`districts` テーブルを新設（**FK は張らない**＝規約準拠／参照は app層 JOIN）。投入は `cmd/seed-municipalities` 等（seed-testsensor の非破壊冪等パターン）。devices に `municipality_id`/`district_id`（制約無し BIGINT）。表示は JOIN クエリ or handler でマスタ名引当。

- ✅ 地区数百件でもスケール。データを Go ソースに持たない。seed 再投入で更新（デプロイ不要）。
- ❌ **§100 規約から逸脱（プロジェクト初のマスタテーブル）**。新規 seed CLI／migration 内 or seed での投入／**実DBテストヘルパ（openTestDB/truncateAll）新規実装**／件数検証テスト新規。表面積が最大。FK無しゆえ整合性はコード責務。
- 典拠: seed reader（マスタ投入CLIの定型）／migration reader（INSERT前例ゼロ）／test reader（実DBヘルパ未実装）。

### Option C — ハイブリッド（市町村=Go定数/CHECK・地区=DBテーブル）
市町村は Option A（Go定数＋VARCHAR＋CHECK、41件は規約に綺麗に乗る）。地区のみ Option B（数百件＝テーブル化が妥当）。devices は `municipality VARCHAR+CHECK`／`district_id BIGINT`（or `district VARCHAR`）。

- ✅ 件数で規約とスケールを両取り。市町村は規約純度を保ち、肥大する地区だけテーブルへ。
- ❌ 2方式併存で一貫性の認知負荷。地区テーブルのために結局 seed CLI＋実DBテスト基盤が要る（B のコストの一部を負う）。
- 典拠: master-data reader gap（地区は CHECK 回避・テーブル or app層）。

> **比較の決め手は「地区の件数・更新頻度」**（Research Needed #1）。少数・静的なら A、大量・可変なら C、全面DB志向で更新運用重視なら B。ロードマップは地区を「段階導入・未整備市町村は市町村のみ可」と許容しており、**A 起点→必要時 C** が段階性に最も合う。

---

## 5. 副次設計判断（design で確定）

1. **`location`（既存自由入力列）の去就**: ①新カラム `municipality`/`district` を ADD し `location` は当面残置（未使用 or 補足）／②`location` を撤去／③改名。所在地が任意ゆえ新カラムは nullable で可（NOT NULL backfill 不要）。要件は補足欄を新設しない方針（Boundary Out of scope）。
2. **backfill 手段**: 既存データは極小（テスト seed＋実機1〜2台）。(a) migration 内 `UPDATE`（**初前例**）／(b) `cmd/seed-*` で移行／(c) 手動。R7 は「対応可能分のみ・不能はNULL残置・件数非減少」なので冪等 `WHERE municipality IS NULL AND location IN(...)` 型が安全。実DB検証は §5.13（tx 内同一SQL→分岐＋非対象無傷→Rollback）だが**ヘルパ未実装**。
3. **カスケード実現方式**: **既定＝HTMX ラッパーdiv（TS-1）**——カスケード先 `<select class="js-tom-select">` を `<div id="...-wrapper">` で囲み、市町村 select の `hx-target`=ラッパーdiv／`hx-swap=innerHTML`／`hx-trigger=change`、サーバは `<option>` 群でなく**子 select タグ全体**を返す専用 templ。既存 `initTomSelect` に乗る。**代替＝クライアントJSON**（全地区を埋め込みJSで絞る・往復ゼロだが命令的 Tom Select destroy/init を自前化＝ガイドが警告）。Option A なら子候補が Go から出るため両案とも実装容易。
4. **Tom Select ライフサイクル配線（要確認）**: App.templ は ECharts 用 before/afterSwap は実装済みだが **Tom Select 用 C12§2 ハンドラが未配線の疑い**。未配線ならカスケード先が swap 後に再初期化されず壊れる → 配線追加が R2 実装の前提作業。
5. **検証配線**: 市町村＝`oneof`（41）or `AllMunicipalities` 照合／地区＝**handler 手続き検証**（`domain.Valid()`＋`DistrictsOf(親).contains(子)`、市町村未選択での地区送信は拒否）。`deviceFieldKey`/`deviceValidationMessage` の**両 switch に case 追加必須**。
6. **マイグレーション**: 新規 `00008_*.sql`（連番は手動。`make migrate-create` はタイムスタンプ命名の恐れ）。`ALTER TABLE devices ADD COLUMN ...`（nullable）＋（市町村列に）`CHECK IN(41)`＋必要なら部分索引。Down で `DROP COLUMN`。COMMENT 日本語付与。**変更後 `make sqlc` と `make db-snapshot` 必須**。
7. **モック先行反映**: input→select 化は `mocks/html/device-create.html`・`device-edit.html`＋正本 `style.css` に**先に反映**してから templ 写経（§31・モック反映ルール）。

---

## 6. 工数・リスク

| 区分 | 工数 | リスク | 根拠 |
|---|---|---|---|
| スキーマ（ALTER＋CHECK＋sqlc再生成＋snapshot） | S | Low | 1 migration・nullable で backfill圧無し。ALTER は初だが定型 |
| domain enum（市町村41） | S | Low | metric.go 同型の写経 |
| **地区データ整備**（Go定数 or テーブル投入） | **M** | **Medium** | 数百件の正規名称を正確に転記／出典確定が必要（Research #1） |
| フォーム/ハンドラ/View 改修 | M | Low | 拡張点・手本が明確（select/復元/DTO/two-switch） |
| **カスケード select（初実装＋Tom Select ライフサイクル配線）** | **M** | **Medium** | コード前例ゼロ・§16 TS-1〜の罠・App.templ 配線確認 |
| 非破壊移行（backfill） | S〜M | Medium | migration backfill 初前例／実DBテストヘルパ未実装 |
| テスト（80%） | M | Low | fakeRepo/render-Contains 確立。件数/移行のみ実DB |
| **総合** | **M（3〜7日）** | **Medium** | 律速＝地区データ確定＋カスケード初実装。Option B/C 採用時は実DBテスト基盤新設で +S〜M |

---

## 7. design フェーズへの推奨と Research Needed

### 推奨アプローチ（design が最終決定）
- **マスタ表現は Option A（Go定数）を起点**に置く（§100 規約整合・新規インフラ最小・段階性に合致）。**地区の件数/更新頻度が大と判明したら地区のみ Option C（テーブル化）**へ。Option B（全面テーブル）は更新運用を重視する明確な理由がある時のみ。
- **カスケードは HTMX ラッパーdiv（TS-1）**を既定。App.templ の Tom Select before/afterSwap 配線の有無を最初に確認し、無ければ配線を R2 の前提タスクに含める。
- **新カラムは nullable で ADD**し `location` は当面残置（撤去は別途）。backfill は冪等 `WHERE municipality IS NULL` 型で、データ極小ゆえ migration UPDATE か seed のどちらでも可（design で一方に確定）。
- tasks には **Foundation として「migration＋`make sqlc`＋`make db-snapshot`」「モック先行反映＋`make sync-css`」「（B/C採用時）実DBテストヘルパ実装」** を明示。

### Research Needed（design で詰める）
1. **【最優先】地区（旧町村/大字）の粒度・件数・正式名称・出典**（総務省コード/自治体オープンデータ等）。これが A/B/C と CHECK 可否・Goソース規模を決める。離島（石垣/宮古/竹富/与那国）の集落粒度も含む。
2. **市町村の正規リストとキー**: 41市町村の正式名称、`code`（全国地方公共団体コード）で持つか名称で持つか。
3. **App.templ の Tom Select `htmx:beforeSwap/afterSwap`（C12§2）配線の実在確認**。未配線ならカスケード swap 再初期化が効かない。
4. **backfill の実装場所**（migration UPDATE＝初前例 / seed / 手動）と、実DB件数・移行テストの可否（`openTestDB`/`truncateAll` ヘルパ新規実装の要否）。
5. **`location` 列の最終処遇**（残置/撤去/改名）と移行マッピング規則。
6. **`region`（圏域＝北部/中部/南部/宮古/八重山）保持の採否**（フェーズ13 地域ロールアップ用。市町村 enum の付随メソッド `Region()` で安価に持てる＝Go定数案と好相性）。

---

# 設計フェーズ追記（Design Decisions & Synthesis・2026-06-26）

`/kiro-spec-design -y` で確定した設計判断と synthesis 結果。詳細は `design.md` を正とする。

## 確定した設計判断
- **マスタ表現 = Option A（市町村・地区とも Go 定数）**を採用。理由: steering §100「マスタ=Go定数+VARCHAR+CHECK」規約への完全整合と、新規インフラ（マスタテーブル/FK/seed CLI/実DBテスト基盤）の最小化。`domain.Municipality`(41・CHECK ミラー)＋`domain.District`(`districtsByMunicipality` map・CHECK 無し・app層検証のみ)。
- **カスケード = HTMX ラッパーdiv（TS-1）＋ App.templ に Tom Select swap ライフサイクル新規配線**。Research Needed #3 を実コードで確定: App.templ の `htmx:afterSwap`(line 182-183)は **ECharts 専用**で Tom Select 未配線 → 第2の独立リスナを新設（カスケード基盤）。
- **`location` 列は残置**（移行元・非破壊）、新規 `municipality`/`district`(nullable VARCHAR) を 00008 で追加。所在地は任意。

## 設計レビュー（3レンズ）で変更した点
- **backfill を SQL(migration 内 DML) → Go(`BackfillLocations` + `cmd/migrate-locations`) へ変更**。理由: executability レビュー指摘の「実DBテスト基盤が隠れた大タスク」を、backfill を Go ロジック化して **fakeRepo で DB 非依存にテスト**することで根本解消。併せて goose の DML 分割問題・41名の三重列挙も回避し、`domain` を単一ソースに再利用（DRY）。migration 00008 は **DDL のみ**（既存規約どおり）。
- DistrictSelect は **hx 属性なしの末端 select**（TS-3 適用外）・**swap は innerHTML 固定**（TS-12 回避）を契約化。
- 市町村 select も handler が `[]SelectOption` を組む（地区と対称・selected 復元一貫性）。
- 検証は **early-return せず errors 累積**（R5.4 同時表示）。`DistrictsOf` ⊆ `ValidDistrict`（同一ソース）を不変条件化。

## Synthesis（3レンズ）
- **Generalization**: `SelectOption{Value,Label,Selected}` DTO ＋ App.templ の Tom Select swap ライフサイクルを、**再利用可能なカスケード基盤**として一般化（将来の連動 select に流用可）。実装は市町村→地区に限定。
- **Build vs Adopt**: Tom Select / HTMX / `domain` enum 規約をいずれも**既存採用物の踏襲**。新規ライブラリ依存ゼロ。
- **Simplification**: マスタテーブル/FK/seed CLI/実DBテスト基盤を**作らない**。`region`(圏域) 列は持たない（YAGNI・将来 `Municipality.Region()` で安価）。`district` 索引は作らない（集計 UI 対象外）。新 DTO は `DeviceOption` が int64 キーで不適合のため最小限の `SelectOption` のみ新設。

---

# 方針転換（2026-06-26）: 2段カスケード → 単一「地域」平坦化＋確定データ

## 決定（ユーザー＝沖縄実地知見の判断）
- **沖縄の農家はほぼ60歳以上で、市町村合併前（旧町村）の呼び名で土地を認識している**（ユーザー言明・権威）。
- これを受け、住所選択を **市町村→地区の2段カスケードから「単一の検索可能な地域セレクト」へ平坦化**することを決定。ユーザーは「知っている名前（旧町村／未合併は市町村名）」を1つ選ぶ。集計用に親市町村を内部で保持/導出。
- **副次効果**: カスケード／HTMX swap／Tom Select swap ライフサイクル／地区フラグメント端点（ShowDistrictOptions）／DistrictSelect.templ が**すべて不要**になり、設計で最もリスキー・新規だった部分が消える（=単純化）。集計軸（市町村）は維持。

## 確定データ（web 調査＋相互検証・confidence high。出典: 総務省 市町村合併資料集／沖縄県公式 市町村合併概要PDF／benricho 平成の大合併／uub.jp）
- **41市町村**: 11市（那覇・宜野湾・石垣・浦添・名護・糸満・沖縄・豊見城・うるま・宮古島・南城）／11町（本部・金武・嘉手納・北谷・西原・与那原・南風原・久米島・八重瀬・竹富・与那国）／19村（国頭・大宜味・東・今帰仁・恩納・宜野座・伊江・読谷・北中城・中城・渡嘉敷・座間味・粟国・渡名喜・南大東・北大東・伊平屋・伊是名・多良間）。
- **平成の大合併＝5件のみ**（現市町村 ← 旧町村）:
  1. 久米島町(2002) ← 仲里村・具志川村
  2. うるま市(2005) ← 石川市・具志川市・与那城町・勝連町
  3. 宮古島市(2005) ← 平良市・城辺町・下地町・上野村・伊良部町
  4. 八重瀬町(2006) ← 東風平町・具志頭村
  5. 南城市(2006) ← 佐敷町・知念村・玉城村・大里村
- **未合併36市町村**: 現名＝旧名（41 − 合併後5）。
- **注意**: 豊見城市(2002)は豊見城村の単独市制施行（全国初・合併でない）→ 旧町村リストに含めない。**旧具志川市(→うるま市) と 旧具志川村(→久米島町) は同名異所**＝地域マスタで区別必須。

## 地域マスタ（Locality）= 53件
- **未合併36**（地域値＝市町村名・親市町村＝自身）＋ **旧町村17**（5合併×旧町村）= **53地域**。
- 旧町村17件: 仲里村・具志川村（久米島町）／石川市・具志川市・与那城町・勝連町（うるま市）／平良市・城辺町・下地町・上野村・伊良部町（宮古島市）／東風平町・具志頭村（八重瀬町）／佐敷町・知念村・玉城村・大里村（南城市）。

## 平坦化の実装方針（design 改訂の核）
- `domain.Municipality`(41・集計軸) と `domain.Locality`(53) を Go 定数で新設。`Locality.Municipality()` が親市町村を返す（合併は旧町村→現市町村、未合併は自身）。
- **同名区別**: Locality の**値**は旧町村の正式名（具志川市／具志川村 と一意）。**表示 Label()** は合併地域=「短縮名（現市町村）」例「佐敷（南城市）」「具志川（久米島町）」「具志川（うるま市）」、未合併=市町村名。**検索エイリアス**=短縮名・正式名・現市町村名。
- `devices` に **`locality VARCHAR(20)`(nullable・CHECK 53値) 1列のみ追加**（市町村は `Locality.Municipality()` で導出。将来の SQL 集計で必要なら非破壊で municipality 列を後付け可＝locality から確定導出のため破綻しない）。`location` は残置（移行元）。
- UI は **単一 `<select name="locality" class="js-tom-select">`**（53 option・検索可能）。カスケード関連の設計要素を全廃。
