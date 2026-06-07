# Gap Analysis — device-detail

> 既存コードベースと requirements.md（R1〜R8）の差分分析。design フェーズの実装戦略を決めるための情報提供であり、最終決定ではない。
> 調査日: 2026-06-08 / 対象: デバイス詳細画面（GET /devices/{device} 系）

## サマリ

- 本機能は **ブラウンフィールド拡張**。S1（基盤: Session 認証・CSRF・MethodOverride・共通レイアウト）/ S4（デバイス登録・編集）/ dashboard が完成済みで、認可・描画・配線パターンは確立されている。
- **最大の新規能力は SVG グラフ自作生成**（`internal/service/` には `alert_evaluator.go` のみ、SVG 前例ゼロ）。純粋・テスト容易だが視覚仕様の確定が必要。配置先（service / 専用 chart パッケージ / view ヘルパ）は structure.md が「service 層の線引きは最初の HTMX 画面後に確定」としている未決事項で、**本セッションが初の HTMX 画面**＝その判断対象。
- **HTMX 基盤は既に配線済み**（`App.templ` に HTMX 2.x + Alpine 3.x + `htmx:configRequest` の CSRF 自動付与 + csrf-token meta）。本機能は「初の HTMX 利用画面」だが土台は揃っており、リスクは中程度に留まる。
- **要確認のクエリギャップ 1 件**: R5「最新10件・降順」のクエリが**存在しない**。既存 `ListRecentSensorReadings` は「指定時刻以降・昇順（24hグラフ用）」、`GetLatestSensorReading` は LIMIT 1。新規 sqlc クエリ追加 + 再生成が必要。
- 総合: **Effort = M（3〜7日）／ Risk = Medium**。新規ピースは多いが SVG 以外は確立パターン上。SVG と「初 HTMX 部分更新」が主な不確実性。

---

## 1. Requirement-to-Asset Map（要件→資産対応、ギャップ: Missing / Unknown / Constraint）

### R1 初期表示（フルページ）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| ページハンドラ + フルページ描画 | `handler/dashboard.go`・`device.go` の `renderPage(c,status,comp)`（`auth.go:196`） | ✅ 再利用 |
| 認証後共通レイアウト | `layout/App.templ`（HTMX/Alpine/CSRF 配線済み）+ `AppLayoutData` | ✅ 再利用 |
| DeviceShow ページ templ + View 構造体 | — | **Missing**（新規 `page/DeviceShow.templ` + `page.DeviceShowView`） |
| 既定 24h 初期描画 | — | **Missing**（Show ハンドラで period 既定 24h） |

### R2 デバイス情報パネル
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| デバイス基本情報取得 | `GetDevice(ctx,id)`（Querier 既存） | ✅ |
| 場所 nullable 整形 | `deviceLocation(d)`（handler pkg 既存） | ✅ 再利用 |
| 稼働状態 ●/○ 表示 | モック `device-show.html` の `status-active` パターン | Constraint（モック写経）+ **Missing**（`DeviceInfoPanel` templ） |
| 最終通信「YYYY-MM-DD HH:MM:SS」絶対表記 | `timefmt` は **RelativeJP のみ**（相対表記） | **Missing**（絶対整形 helper 追加 or `time.Format("2006-01-02 15:04:05")` を timefmt に） |
| 最終通信なしの判別表示 | `lastCommText`（"通信実績なし"）パターン | ✅ パターン流用 |

### R3 期間切替によるグラフ部分更新（技術的中心）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| HTMX 本体 + CSRF 自動付与 | `App.templ`（htmx.min.js + `htmx:configRequest`→X-CSRF-Token + csrf meta） | ✅ **既に配線済み** |
| period バリデーション（oneof 24h/7d/30d） | `ShouldBind*` + binding タグの前例（auth/device） | ✅ パターン流用（`ShouldBindQuery`） |
| グラフ領域フラグメント返却エンドポイント | — | **Missing**（`DeviceHandler.Chart` 新規） |
| フラグメント描画（レイアウト無しで component 直 Render） | `renderPage` はあるが **component 直 Render helper は無し** | **Missing**（小: `comp.Render(ctx, c.Writer)` or `renderComponent` 追加） |
| `DeviceChartArea` component templ（温度+湿度グラフ+期間ボタン） | — | **Missing** |
| アクティブ期間のサーバー側往復（フルフラグメント swap） | — | Unknown→design（HTMX実装ガイド §10-D / §4 device-show） |
| 選択期間の URL 反映（R3-5, should） | モック R24「`<a href="?period=">`＋URL状態保持」 | Unknown→design（`hx-push-url` 採否。spec-init のフラグメント方式と統合） |

### R4 温度・湿度 SVG グラフ（サーバー側生成）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| 24h 生データ（昇順） | `ListRecentSensorReadings(deviceID, recordedAt>=)` ASC | ✅ |
| 7d/30d 日次集計（avg/max/min/count） | `ListDailySensorAggregates(deviceID, recordedAt>=)` | ✅ |
| nullable 集計値の安全変換 | `pgconv.NumericToFloat(n)` | ✅ |
| **SVG 自作生成ロジック**（線描画・スケーリング・軸ラベル・凡例） | **前例ゼロ**（service は alert_evaluator のみ） | **Missing（最大の新規能力）** |
| SVG 配置先（service / chart パッケージ / view） | structure.md「service 層の線引き未定（初 HTMX 後に確定）」 | Constraint / **要設計判断** |
| 空データ時の空グラフ（「データはまだありません」） | — | **Missing** |

### R5 最新計測データテーブル（固定10件）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| **最新10件・降順クエリ** | **存在しない**（`ListRecentSensorReadings`=時刻以降・昇順／`GetLatestSensorReading`=LIMIT 1） | **Missing（新規 sqlc クエリ + `make sqlc` 再生成）** |
| LatestReadingsTable component templ | モック table パターン | **Missing** |
| 計測日時「YYYY-MM-DD HH:MM」/ 温湿度 小数2桁 | `formatReadingText`（小数2桁）パターン、絶対時刻は R2 と同じ Missing | 一部 ✅ / 一部 **Missing** |
| 期間切替に非連動（swap 対象に含めない） | — | Constraint→design（id を chart-area swap 範囲外に配置） |

### R6 削除（確認モーダル → 論理削除 → リダイレクト）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| 論理削除クエリ | `SoftDeleteDevice(ctx,id)` | ✅ |
| 所有者認可 | `authz.RequireDeviceOwner` + `renderDeviceOwnerError` | ✅ 再利用 |
| Delete ハンドラ | — | **Missing**（`DeviceHandler.Delete` 新規） |
| 確認モーダル（Alpine.js x-show） | モック `delete-modal` パターン | Constraint（モック写経）+ **Missing** templ |
| HX-Redirect /dashboard（HTMX）/ 303（非HTMX） | **HX-Redirect 前例なし**（初使用） | Unknown→design（HTMX実装ガイド §9 / §11 / §24） |
| 非HTMX DELETE の `_method` 上書き | `middleware.MethodOverride`（合成済み） | ✅ |
| DELETE への CSRF 適用 | gorilla/csrf（Web グループ）+ htmx X-CSRF-Token | ✅ 配線済み |

### R7 所有者認可・アクセス制御（BOLA 防止）
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| 未認証→ログイン誘導 | `middleware.RequireAuth()` | ✅ 再利用 |
| 非所有/不在→404・列挙防止 | `RequireDeviceOwner`→`pgx.ErrNoRows`/`ErrNotOwner`→`renderDeviceOwnerError`（404/403） | ✅ 完全再利用 |

### R8 入力バリデーション・エラーハンドリング
| 必要能力 | 既存資産 | 状態 |
|---|---|---|
| device ID 非数値の拒否 | S4 は **404** で写像（`device.go:111-114`）。**R8.1 は 400 を要求** | Constraint / **要設計判断（400 vs 既存404 の統一）** |
| period 不正の拒否（400/422） | binding バリデーション前例 | ✅ パターン流用 |
| 想定外DBエラー→内部非露出 | `renderError(c,500)` | ✅ |
| 全文言 日本語 | 既存方針 | ✅ |

---

## 2. 実装アプローチ オプション

### Option A: 既存 DeviceHandler を device.go 内で拡張
`device.go` に `Show/Chart/Delete` を追記し、`DeviceRepo` を拡張。SVG も handler パッケージ内に。
- ✅ `DeviceHandler`・`DeviceRepo`・`renderDeviceOwnerError`・描画ヘルパを最大限再利用、デバイス系ルートが単一ハンドラに集約
- ❌ `device.go`（現289行）が肥大化。SVG ロジック同居で凝集が崩れ、coding-style（ファイル<800・関数<50）に圧迫

### Option B: 新ファイル/新パッケージで分離
新 handler ファイル + SVG 専用パッケージ + 新 templ 群。`DeviceHandler` とは別構造体も検討。
- ✅ 関心分離が明快、SVG を単体テスト容易な純粋ユニットに隔離
- ❌ ファイル増。別構造体にすると `DeviceRepo`/認可写像の再利用がしにくくなる

### Option C: ハイブリッド（推奨）
1. **`DeviceHandler` を流用**しつつ `Show/Chart/Delete` は**新ファイル `internal/handler/device_show.go`（同 package・同 struct）** に置く（`dashboard.go` が `auth.go` から分割されているのと同じ手法で `device.go` 肥大化を回避）。
2. **`DeviceRepo` interface を拡張**（`ListRecentSensorReadings`・`ListDailySensorAggregates`・新「最新10件」クエリ・`SoftDeleteDevice` を追加。`repository.Querier` が満たすため main.go 配線は無改修）。
3. **SVG 生成は専用の純粋ユニットに隔離**（配置は design 決定: spec-init は `service/device_service.go`、ただし SVG は表示ロジックで業務ロジックではないため `internal/chart` 等の専用パッケージも有力）。`strings.Builder` で XML 直描画・外部ライブラリ非依存（spec-init 軽量自作方針）。
4. **新 templ**: `page/DeviceShow.templ` + `component/{DeviceChartArea,DeviceInfoPanel,LatestReadingsTable}.templ`（モック `device-show.html` を写経）。
5. **新 sqlc クエリ**（最新10件降順）+ `make sqlc`。**絶対時刻整形**を `timefmt` に追加。**フラグメント描画ヘルパ**を追加。
6. **main.go に 3 ルート配線**（GET `/devices/:device`、GET `/devices/:device/chart`、DELETE `/devices/:device`）。

- ✅ 既存パターン最大活用 + SVG を隔離して単体テスト容易 + `device.go` 非肥大化
- ❌ 計画が最も多段（クエリ追加・パッケージ判断・初 HTMX/HX-Redirect の確立を同時進行）

---

## 3. Effort & Risk

- **Effort: M（3〜7日）** — 新規ピースが多い（SVG 自作・初 HTMX 部分更新・新 templ 4本・新クエリ・絶対時刻整形・3ルート配線）が、SVG 以外は確立パターン上に乗る。
- **Risk: Medium** — 主因は (1) SVG 自作生成（前例ゼロ・視覚仕様未確定だが純粋関数で隔離・テスト可能）、(2) 初の HTMX 部分更新 / HX-Redirect / フルフラグメント swap の確立。緩和材料: HTMX/CSRF 基盤は配線済み、所有者認可・描画・error 写像は完全再利用。

---

## 4. design フェーズへの申し送り

### 推奨アプローチ
- **Option C（ハイブリッド）**。`device_show.go` 分割 + `DeviceRepo` 拡張 + SVG 隔離 + モック写経 templ + 新クエリ + 3ルート。
- **Foundation 的タスク**として「最新10件クエリ追加 + `make sqlc`」「絶対時刻整形 helper」「フラグメント描画 helper」を先頭付近に置く。

### 主要な設計判断（design で確定すべき点）
1. **SVG 生成の配置**: `internal/service/device_service.go`（spec-init 案）か、表示専用の `internal/chart` 等の新パッケージか。structure.md の「service 層の線引きは初 HTMX 後に確定」に対する**本セッションが最初の判断機会**。依存方向（view→service/repository 禁止、handler が組み立て）を順守。
2. **device ID 非数値の HTTP ステータス**: R8.1 = **400** だが既存 S4 は **404**。全デバイス系ルートで統一する（要件側 or 実装側のどちらに寄せるか design で確定）。
3. **期間切替の URL 反映（R3-5 should）**: `hx-push-url` 採否。モック R24（`<a href="?period=">`＋URL 状態保持）と spec-init のフラグメント swap（`hx-get /chart` → `#device-chart-area`）の統合方式。Show ハンドラが `?period` を読むか。
4. **DELETE 応答分岐**: `HX-Request` 有無で `HX-Redirect: /dashboard`（HTMX）/ `303 SeeOther`（非HTMX `_method`）を分岐（初の HX-Redirect 使用）。
5. **テーブルを swap 範囲外に保つ id 設計**: `#device-chart-area` の innerHTML swap に `latest-readings-table` を**含めない**配置（HTMX実装ガイド §3 id 一覧 / §4 device-show）。

### Research Needed（design で詰める不確実点）
- **SVG 視覚仕様**: 温度/湿度の線色・フォント・寸法（viewBox）・縦軸スケーリング（auto min/max）・24h 線グラフ vs 7d/30d の max/min 表現（帯 or 2線）・軸ラベル書式。出典: `システム構成図.md`「サーバサイド SVG グラフ自作」。
- **日次集計のタイムゾーン**: `DATE(recorded_at)` の日境界（UTC か JST か）。timestamptz 格納のため日付グルーピング結果に影響。
- **最新10件クエリの命名/形**: `ListLatestSensorReadings`（`WHERE device_id=$1 ORDER BY recorded_at DESC LIMIT 10`）等。既存 `ListRecentSensorReadings`（時刻以降・昇順）と紛らわしいため命名注意。
- **アクティブ往復の具体 markup**: フルフラグメント swap での `.active` 付与方式（HTMX実装ガイド §10-D）。

### 参照（design 着手時）
- `2cc_sdd/HTMX実装ガイド(動的).md` §3.3 id一覧・§4 device-show・§9 HX-Redirect・§10-D フルフラグメント swap・§11 削除確認・§24 Alpine.js モーダル
- `システム構成図.md`「サーバサイド SVG グラフ自作」
- `docs/database_snapshot/table_definitions.md`（devices / sensor_readings）
- `mocks/html/device-show.html` + `mocks/html/style.css`（写経元）
- 既存パターン: `internal/handler/dashboard.go`（page handler 整形）・`device.go`（認可写像・View 組立）・`authz/ownership.go`

---

# Design Discovery & Synthesis (light, extension)

> design フェーズの追加調査と合成所見。gap 分析（上記）を踏まえ、HTMX実装ガイド §3.3/§4 device-show/§9/§10-D/§43、システム構成図.md（SVG self-built）、既存 handler/dashboard.go・device.go・auth.go の写経で確定した設計判断を記録する。
## 確定した設計判断（Synthesis）

1. **SVG 配置 = 新規 `internal/chart` 純粋パッケージ**（spec-init の `service/device_service.go` 案から変更）。理由: SVG 生成は表示ロジックであり業務ロジックではない（`service/alert_evaluator.go` のような業務判定とは別物）。`timefmt`・`pgconv` と同じ「stdlib のみ依存の純粋ユーティリティ」パターンに合わせ、gin/DB/templ 非依存・table-driven で単体テスト可能にする。`service` 層は真の業務ロジック用に温存（structure.md の「線引き未定」に対し、本画面は service 不要＝handler が query 組立＋chart 呼出＋view 構築を担うと結論）。
2. **device ID 非数値 = 400**（R8.1 準拠。HTMX実装ガイド §9 の Delete 例も `c.String(400,...)`）。S4 の Show/Edit は 404 を採用しており**画面間で不一致が残る**が、本 spec は R8.1 に従い 400 とする（S4 の 404 統一は本 spec のスコープ外＝別途検討）。
3. **期間 URL 反映（R3-5）= `hx-push-url` をフルページ URL に向ける**。期間ボタンは `hx-get="/devices/{id}/chart?period={p}"`（フラグメント取得）だが `hx-push-url="/devices/{id}?period={p}"`（**フルページ URL** を push）とする。フラグメント URL を push するとリロード時に部分 HTML だけ返る不具合になるため。`Show` ハンドラは任意の `?period`（oneof 24h/7d/30d・既定 24h・不正は 400）を読んで初期描画する。これでモック R24（URL 状態保持）と spec-init（/chart フラグメント方式）を矛盾なく統合。
4. **DELETE 応答分岐**: `c.GetHeader("HX-Request") != ""` → `HX-Redirect: /dashboard` + 200／非 HTMX（フォーム `_method=delete`→MethodOverride）→ `c.Redirect(303, "/dashboard")`（HTMX実装ガイド §9 のパターンを踏襲）。
5. **削除確認モーダルの Alpine スコープ**: `App.templ` の body は `x-data="{ navOpen:false }"` 固定のため、DeviceShow 側で**ネスト `x-data="{ deleteModalOpen:false }"` の div** を設け、その**内側**に削除ボタン（`@click="deleteModalOpen=true"`）とモーダル（`x-show="deleteModalOpen"`）を**両方**置く（§62「x-data スコープ外モーダルの Alpine エラー」回避）。確認ボタンは `hx-delete="/devices/{id}"`（§24 推奨）。
6. **最新10件クエリ新設**: `ListLatestSensorReadings(deviceID) → ORDER BY recorded_at DESC LIMIT 10`。既存 `ListRecentSensorReadings`（時刻以降・昇順）と紛らわしいため `Latest` で明確化。`db/queries/sensor_readings.sql` 追記 → `make sqlc` 再生成。
7. **絶対時刻整形を `timefmt` に追加**: `DateTimeJP(t)`→"2006-01-02 15:04:05"（最終通信）、`DateTimeMinuteJP(t)`→"2006-01-02 15:04"（テーブル）。dashboard が `RelativeJP` を使うのと同じく view-pure 整形を timefmt に集約。
8. **フラグメント描画ヘルパ**: 既存 `renderPage`（フルページ）に加え、HTMX フラグメント用に `renderComponent(c, comp)`（status 200・`comp.Render`）を handler パッケージに追加（Chart のレイアウト無し返却用）。
9. **Generalization/Simplification**: 温度・湿度グラフは同一の線グラフ描画関数（系列色・単位・スケールをパラメータ化）で共通化し、24h（生データ1系列）/ 7d・30d（日次 max・min の2系列）を同一 `chart.LineChartSVG` で表現。service 層・独自 Repository interface は新設せず（speculative abstraction 回避）、既存 `repository.Querier`（DB ポート）と `DeviceRepo` 拡張で完結。

## Open Questions / Risks（design 内で対処）
- **日次集計のタイムゾーン**: 既存 `ListDailySensorAggregates` の `DATE(recorded_at)` は DB セッション TZ 依存。JST 日境界を期待する場合は接続 TZ=Asia/Tokyo を前提とする（本 spec ではクエリの TZ 挙動を変更しない＝Out of Boundary。要 TZ 確認を申し送り）。
- **SVG 視覚仕様**: 寸法・線色・軸ラベル書式は他資料に詳細が無いため design 内で具体化（`internal/chart` の単体テストで構造要素を固定）。実装時のピクセル計算には裁量を残す。
