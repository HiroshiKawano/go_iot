# Requirements Document

## Project Description (Input)

> 出典: `2cc_sdd/spec-init-prompts/session-03-dashboard.md`（セッション3）
> 推奨 feature-name: dashboard / 前提セッション: S1（基盤＋認証）。表示する未対応アラートデータは S2 が生成するが seed でも代替可
> 設計フェーズで参照:
> - 画面設計書(静的).md 行205-264（ダッシュボード）、行72-103（共通レイアウト・ナビ）
> - HTMX実装ガイド(動的).md §3.2（行1255-1263, id一覧）、§4 dashboard（行1428-1442）、§5（行1587-1616, OOB）、§6（行1619-1630, HTMX未使用）、§7（行1633-1699, バリデーション）
> - DB設計書.md devices/sensor_readings/alert_histories テーブル定義、sqlc リレーション方針
> - mocks/html/dashboard.html
> - 実装現状サマリ.md（コンテキスト）

### 機能概要

農場運営者がシステムに最初にアクセスする画面。現状、デバイスのステータス（稼働状態・最新測定値・通信状態）とアラート発生状況（未対応アラート一覧）を一覧で確認し、詳細画面への入口となる。本セッションで、認証済みユーザーが `/dashboard` へ GET で到達したとき、登録済みデバイス一覧（名前・設置場所・稼働状態・最新温度・最新湿度・最終通信時刻）と未対応アラート一覧を templ でレンダリングし、デバイス登録・詳細画面へのリンクを提供する機能を実装する。

### 背景・現状

- バックエンド API・DB・sqlc クエリはほぼ完成（実装現状サマリ参照）。
- templ 画面層・Web UI ハンドラは全面未着手。`internal/view/{layout,component,page}` は3ディレクトリとも空。
- htmlモック 9 画面の内、dashboard.html は完成済み（id は templ 側で付与）。
- 共通レイアウト（ヘッダ + サイドバー + 認証メニュー）は S1 で実装予定（`internal/view/layout/App.templ`）。本セッションはこのレイアウトを継承する。
- S1 は GET /dashboard ハンドラと `Dashboard.templ` を「認証後に到達できる最小プレースホルダ」として用意済み。本セッションはルートを新規追加するのではなく、その**ハンドラ実装と templ を正規版に置き換える**。
- デバイス履歴・測定値・アラート詳細画面の実装は S4・S5・S6・S7・S8 で担当。本セッションはリンク先のハンドラ/templ は実装しない。

### このセッションのスコープ（実装対象）

#### ハンドラ
- **GET /dashboard**（認証済みユーザーのみ）
  - ロール: ログイン済みユーザーのデバイス一覧・最新センサー値・未対応アラートを取得し、templ でレンダリング。
  - 返却: `internal/view/page/Dashboard.templ`（フルページ）。
  - 使用 sqlc クエリ:
    - `ListDevicesByUser(ctx, userID)` → Device 配列
    - `GetLatestSensorReading(ctx, deviceID)` → 直近1件の温度・湿度・記録時刻
    - `ListUnnotifiedAlertHistoriesWithDevice(ctx, userID)` → デバイス名を含む未通知アラート一覧

#### templ コンポーネント・ページ
- **`internal/view/page/Dashboard.templ`**
  - フルページ（共通レイアウト `App.templ` を継承）。
  - props: `devices []domain.Device`, `readings map[int64]domain.SensorReadingRow`, `unnotifiedAlerts []domain.UnnotifiedAlertRow`（domain は repository 生成構造体）
  - 構成:
    1. 共通レイアウト（S1 実装予定）
    2. ページタイトル「ダッシュボード」＋「+ デバイス登録」リンク（`/devices/create` へ遷移）
    3. 未対応アラートセクション（id: `unhandled-alert-banner`、見出し・リスト・0件時メッセージ）
    4. デバイス一覧セクション（id: `device-grid`、カードグリッド・0件時メッセージ）

- **`internal/view/component/DeviceCard.templ`**
  - デバイス1台分のカード（`<article class="device-card">`）。
  - props: `device domain.Device`, `latestReading *domain.SensorReadingRow`（nil 可）
  - 表示項目（モック dashboard.html 行56-71, 74-89 準拠）:
    - デバイス名（`<h3>`）
    - 設置場所（「場所: ○○」）
    - 稼働状態（「状態: ● 稼働中」または「○ 停止中」、`is_active` で判定）
    - 最新温度（小数2桁 + ℃、latestReading が nil なら「計測待機中」等の代替文言）
    - 最新湿度（小数2桁 + %）
    - 最終通信（相対時間表記、例「2分前」。`last_communicated_at` から go-ago や hand-made フォーマッタで算出）
    - 「詳細を見る」ボタン（`<a href="/devices/{device.ID}">`）

- **`internal/view/component/UnhandledAlertBanner.templ`**
  - 未対応アラート一覧バナー（`<div id="unhandled-alert-banner">`）。
  - props: `alerts []domain.UnnotifiedAlertRow`
  - 表示形式（モック dashboard.html 行39-48 準拠）:
    - 見出し「⚠ 未対応アラート」
    - 各アラートをリスト表示: 「⚠ {デバイス名}: {指標}が{閾値}{条件}（{実測値}）」
    - 0件時: `.empty-message` 要素で「未対応のアラートはありません。」
    - 指標・閾値の表記: `domain.Metric.Label()` / `domain.ComparisonOperator.Label()` / `domain.Metric.Unit()` を使用

#### ミドルウェア
- SessionLoad（S1 で実装）を通す前提。GET /dashboard へのアクセスは認証済みユーザーのみ。
- 認証されていない場合は `/login` へリダイレクト。ハンドラ内で `auth.UserID(c)` で user_id を取得。

#### id 属性（templ で付与）
- `unhandled-alert-banner` — 未対応アラートバナーのラッパー（OOB 更新対象の可能性）
- `device-grid` — デバイスカード全体のグリッドコンテナ（削除後・登録後の差し替え対象）
- `device-card-{id}` — 個別デバイスカード（device.id を埋め込み、将来の個別更新用）

#### エラーハンドリング
- `ListDevicesByUser` / `GetLatestSensorReading` / `ListUnnotifiedAlertHistoriesWithDevice` が DB エラーを返した場合 → 500（Gin デフォルト）。
- デバイス0件・アラート0件は正常系として `.empty-message` で表示（エラーではない）。

#### テスト・カバレッジ
- ハンドラテスト: `TestGetDashboard`（認証済み・デバイス複数・アラート複数、デバイス0件・アラート0件、DB エラーを含む3+パターン）。
- templ は機能テスト側で画面遷移確認（model テスト不要）。
- 目標: 80% 以上。

### スコープ外（このセッションでやらないこと）

- デバイス登録画面（`/devices/create`）・詳細画面（`/devices/{device}`）の実装 → S4・S5 担当。
- センサー履歴画面（`/readings` 等）の実装 → S6 担当。
- アラートルール・履歴画面の実装 → S7・S8 担当。
- 日本語の相対時間フォーマッタ（「2分前」「5分前」）の自作判断。後述「未確定事項」参照。
- HTMX リアルタイム自動更新（`hx-trigger="every 30s"` でデバイスグリッドを定期ポーリング）は **導入しないこと**（設計書 §4 行1440 注記参照）。初期描画のみ。

### 技術制約・準拠事項

- **Gin v1.12 + templ v0.3**: ハンドラは `c.Render(http.StatusOK, templ.Renderer(page.Dashboard(...)))`。
- **sqlc 生成構造体**: `internal/repository/*` に `GetLatestSensorReadingRow` / `ListUnnotifiedAlertHistoriesWithDeviceRow` 等の行構造体が既存（実装現状サマリ参照）。domain 型マッピングは必要に応じて行う。
- **認証**: `auth.UserID(c)` で user_id を取得（S1 実装予定）。
- **sessionLoad ミドルウェア**: `/dashboard` グループ全体に自動適用（S1 で実装）。未認証時 401。
- **日本語・表示形式**:
  - 温度・湿度は小数2桁固定（`fmt.Sprintf("%.2f℃", reading.Temperature)`）。
  - アラート表記: 「⚠ {デバイス名}: {指標}が{閾値}{条件}（{実測値}）」（Metric/ComparisonOperator の Label() / Unit() を使用）。
  - 最終通信の相対時間表記（「2分前」「1時間前」「1日前」等）については後述「未確定事項」。
- **イミュータブル方針**: templ コンポーネントに渡す props は読み取り専用。コンポーネント内で加工・キャッシュは不可。
- **エラーハンドリング**: HTML レンダリングエラー（templ 構文 panic）は Gin Recovery で 500。ユーザーには詳細を表示しない。
- **テスト**: Go Teststand（Gin mockwriter + repository mock / stub）で記述。統合テスト（実 DB）は cc-sdd では不追跡だが、80% カバレッジ前提。

### 受け入れ基準（概略）

1. **GET /dashboard へのアクセス（認証済みユーザー）で Dashboard.templ が正しくレンダリングされる**
   - ステータスコード 200。
   - HTML 構造が モック dashboard.html と一致（共通レイアウト除く）。
   - id 属性（`unhandled-alert-banner` / `device-grid` / `device-card-{id}`）が配置。

2. **デバイス一覧が表示される**
   - ListDevicesByUser で取得したデバイスが複数表示。
   - 各カードに名前・場所・稼働状態・最新温度・最新湿度・最終通信が表示。
   - 最新センサー値は GetLatestSensorReading で取得したもの。
   - 0件時は `.empty-message` で「登録されたデバイスはありません。」。

3. **未対応アラート一覧が表示される**
   - ListUnnotifiedAlertHistoriesWithDevice で取得したアラートが表示。
   - 表記: 「⚠ {デバイス名}: {指標}が{閾値}{条件}（{実測値}）」。
   - 0件時は `.empty-message` で「未対応のアラートはありません。」。

4. **リンク遷移**
   - 「+ デバイス登録」ボタン → `/devices/create`（S4 で未実装でも href 配置）。
   - 「詳細を見る」ボタン → `/devices/{device.id}`（S5 で未実装でも href 配置）。

5. **ハンドラテスト（80% カバレッジ）**
   - `TestGetDashboard_Success_WithDevicesAndAlerts`（正常系）。
   - `TestGetDashboard_Success_NoDevices`（デバイス0件）。
   - `TestGetDashboard_Success_NoAlerts`（アラート0件）。
   - `TestGetDashboard_Unauthenticated`（認証なしで 401 または login へリダイレクト）。
   - `TestGetDashboard_DBError`（ListDevicesByUser エラーで 500）。

6. **templ コンポーネントの正確性**
   - Dashboard / UnhandledAlertBanner / DeviceCard が個別に機能テスト可能（分割設計）。
   - props を正しく渡せば期待値通りにレンダリング（単体テストはなくても e2e で検証）。

7. **CSSクラス・要素構造**
   - 全体グリッドの class は `device-grid`（自前 CSS の `@layer components` に定義。`grid-template-columns: repeat(auto-fill, minmax(...))` でカード自己レスポンシブ。CSS方針は `.kiro/steering/tech.md` 参照）。
   - 各カードの class は `device-card`。
   - 相対時間・数値フォーマットのクラス（`status-active`/`reading-value` 等）が モック に準拠。

### 未確定事項・要確認（requirements/design フェーズで決定）

1. **相対時間フォーマッタの選択**
   - モック では「2分前」「5分前」「1時間前」等が表示される（`last_communicated_at` との差分）。
   - ハンドラ内で手作りするか、`github.com/relaysh/go-ago` 等のライブラリを使うか要確認。
   - 決定後、go.mod に追加し、templ props に `relativeTime string` で渡すか、templ 内で計算するか決める。

2. **デバイス0件のデフォルトメッセージ**
   - モック 行92 の text は「登録されたデバイスはありません。上の「デバイス登録」ボタンから追加してください。」。
   - 本セッションで全文を採用するか、簡略版（「登録されたデバイスはありません。」）にするか確認。

3. **最新センサー値がない場合の表示**
   - デバイスは存在するが計測データがまだない（GetLatestSensorReading が nil）場合、カード上にどう表示するか。
   - モック では全デバイスに値があるため未定義。「計測待機中」「ー」「なし」等の方針を決める。

4. **unhandled-alert-banner と device-grid の HTMX 部分更新タイミング**
   - 設計書 §5 (OOB) では `/devices/{device}/toggle`（稼働状態切替）や `/alerts/check`（しきい値チェック実行）でこれらを OOB 更新する想定。
   - 本セッションでは「初期描画のみ」だが、S4（デバイス削除）・S7（アラート確認）で該当エンドポイント実装時に OOB テンプレート関数（`UnhandledAlertBannerOOB` 等）を分割するか、同一関数で `hx-swap-oob="true"` を条件付きするか、の方針決定は実装時に。

## Introduction

農場運営者がログイン後に最初に到達する画面として、本人が所有するデバイスの状態（稼働状態・最新の温湿度・通信の新しさ）と、未対応アラートの発生状況を 1 画面で一覧把握できるダッシュボードを提供する。本仕様は、認証済みユーザーが `/dashboard` を GET したときのフルページ初期描画と、デバイス登録・各デバイス詳細への遷移導線までを対象とする。S1（web-foundation-auth）が用意した最小プレースホルダのダッシュボードを、正規のデータ表示版へ置き換える。

## Boundary Context

- **In scope（本機能が所有する振る舞い）**
  - 認証済みユーザーによる `/dashboard` の GET フルページ初期描画。
  - 本人所有デバイスの一覧表示（名前・設置場所・稼働状態・最新温度・最新湿度・最終通信）と 0 件時の空メッセージ。
  - 計測データ未受信・通信実績なしといったデータ欠損時のカード表示。
  - 未対応アラートの一覧表示と 0 件時の空メッセージ。
  - 「デバイス登録」「各デバイス詳細」への遷移導線（href の配置）。
- **Out of scope（本機能が所有しない振る舞い）**
  - デバイス登録画面（`/devices/create`）・デバイス詳細画面（`/devices/{id}`）・センサー履歴画面・アラートルール/履歴画面の実装。本機能はリンク（遷移先 URL）を配置するのみで、遷移先の表示は各別セッションが所有する。
  - デバイスグリッドやアラートバナーの定期ポーリングによる自動更新（初期描画のみ）。
  - 操作（稼働状態切替・アラート確認・デバイス削除等）に伴う部分更新（OOB）での再描画。本機能は初期描画のみを対象とし、部分更新は当該操作を実装する別セッションが所有する。
- **Adjacent expectations（隣接仕様への期待）**
  - 共通レイアウト（ヘッダー・サイドバー・ユーザーメニュー・ログアウト）と認証ガード（未認証時の `/login` リダイレクト・本人ユーザー ID の解決）は web-foundation-auth（S1）が所有する。本機能はこれらを前提として描画し、再実装しない。
  - 未対応アラートの元データは alert-evaluation（S2）が受信時判定で生成する。本機能は当該データを読み取って表示するのみで、生成・判定は行わない（開発時は seed データで代替可）。
- **データ対応（UI 概念 → 現状スキーマ。`docs/database_snapshot/` を正とする）**
  - 「設置場所」= `devices.location`（未設定可）／「稼働状態」= `devices.is_active`（true=稼働中／false=停止中）／「最終通信」= `devices.last_communicated_at`（未設定可）。
  - 「最新温度／最新湿度」= 当該デバイスの最新 `sensor_readings.temperature` / `humidity`（いずれも小数 2 桁精度）。
  - 「未対応アラート」= 当該ユーザー所有デバイスに紐づく、まだ通知が送られていないアラート履歴（`alert_histories.is_notified = false`）。
  - 「指標／比較条件」の許容値はスキーマの CHECK 制約に従う（指標: `temperature` / `humidity`、比較演算子: `>` / `<` / `>=` / `<=`）。

## Requirements

### Requirement 1: ダッシュボードへの認証付きアクセス

**Objective:** 農場運営者として、ログイン後に自分のデバイスとアラート状況を 1 画面で確認したい。そのために、認証済みのときだけダッシュボードに到達できるようにしたい。

#### Acceptance Criteria

1. When 認証済みユーザーが `/dashboard` を GET したとき、the Dashboard 画面 shall ステータス 200 で、共通レイアウト内にダッシュボード本体を含むフルページの HTML を返す。
2. If 未認証ユーザーが `/dashboard` を GET したとき、then the システム shall ステータス 302 で `/login` へリダイレクトする（認証ガードは web-foundation-auth が所有する）。
3. The Dashboard 画面 shall ログイン中の本人が所有するデバイス・最新計測値・未対応アラートのみを表示し、他ユーザーが所有するデータを表示しない。

### Requirement 2: デバイス一覧の表示

**Objective:** 農場運営者として、登録済みデバイスを一覧で把握したい。そのために、各デバイスの基本情報をカード形式で見られるようにしたい。

#### Acceptance Criteria

1. While ログイン中ユーザーが本人所有のデバイスを 1 台以上持つとき、the Dashboard 画面 shall 各デバイスをカード形式で一覧表示する。
2. The 各デバイスカード shall デバイス名・設置場所・稼働状態・最新温度・最新湿度・最終通信・詳細画面への導線を表示する。
3. When デバイスが稼働中であるとき、the デバイスカード shall 稼働状態を「● 稼働中」と表示し、停止中であるときは「○ 停止中」と表示する。
4. While ログイン中ユーザーが本人所有のデバイスを 1 台も持たないとき、the Dashboard 画面 shall デバイスカードの代わりに空メッセージ「登録されたデバイスはありません。上の「デバイス登録」ボタンから追加してください。」を表示する。

### Requirement 3: 最新計測値と通信状態の表示

**Objective:** 農場運営者として、各デバイスの直近の温湿度と通信の新しさを知りたい。そのために、最新計測値と最終通信を分かりやすい表記で見られるようにしたい。

#### Acceptance Criteria

1. When デバイスに計測データが 1 件以上存在するとき、the デバイスカード shall 最新の温度を小数 2 桁＋「℃」で、最新の湿度を小数 2 桁＋「%」で表示する。
2. If デバイスに計測データが 1 件も存在しないとき、then the デバイスカード shall 温度欄・湿度欄をそれぞれ「ー」と表示する。
3. When デバイスに通信実績があるとき、the デバイスカード shall 最終通信を現在時刻との差分に基づく日本語の相対時間（例:「2分前」「1時間前」「1日前」）で表示する。
4. If デバイスが一度も通信していない（通信実績がない）とき、then the デバイスカード shall 最終通信欄を「通信実績なし」と表示する。

### Requirement 4: 未対応アラート一覧の表示

**Objective:** 農場運営者として、未対応の異常検知をダッシュボード上部で即座に把握したい。そのために、未対応アラートを一覧で見られるようにしたい。

#### Acceptance Criteria

1. When ログイン中ユーザー所有デバイスに未対応アラートが 1 件以上存在するとき、the Dashboard 画面 shall それらを未対応アラートセクションにリスト表示する。
2. The 各未対応アラート項目 shall 対象デバイス名・指標・閾値・比較条件・実測値が判別できる形式で表示する（例:「ハウスA温湿度計: 温度が35℃を超えました（38.50℃）」）。
3. While ログイン中ユーザー所有デバイスに未対応アラートが 1 件も存在しないとき、the Dashboard 画面 shall 空メッセージ「未対応のアラートはありません。」を表示する。
4. The 未対応アラート項目の指標・単位・比較条件の表記 shall システム共通の指標・比較演算子の表示ルール（指標は temperature／humidity、比較演算子は >／<／>=／<=）に従う。

### Requirement 5: 関連画面への遷移導線

**Objective:** 農場運営者として、ダッシュボードからデバイス登録や各デバイスの詳細へ移動したい。そのために、明確な遷移リンクを置きたい。

#### Acceptance Criteria

1. The Dashboard 画面 shall ページ見出し付近に「+ デバイス登録」リンク（遷移先 `/devices/create`）を常時表示する。
2. The 各デバイスカード shall 「詳細を見る」リンク（遷移先 `/devices/{デバイスID}`）を表示する。
3. Where 遷移先画面が本セッションで未実装であっても、the Dashboard 画面 shall 正しい遷移先 URL を持つリンク（href）を配置する（遷移先画面の表示は別セッションが所有する）。

### Requirement 6: エラー時の挙動と初期描画範囲

**Objective:** 農場運営者として、データ取得に失敗したときに中途半端な画面ではなく異常が分かる挙動を期待し、また 0 件状態を正常として扱ってほしい。

#### Acceptance Criteria

1. If デバイス一覧・最新計測値・未対応アラートのいずれかの取得が内部エラーで失敗したとき、then the システム shall 機密情報を含まないステータス 500 のエラー応答を返す。
2. The Dashboard 画面 shall デバイス 0 件・未対応アラート 0 件を正常系として空メッセージで表示し、エラー扱いにしない。
3. The Dashboard 画面 shall 初期表示時点のデータのみを描画し、自動ポーリングによる定期更新を行わない。
