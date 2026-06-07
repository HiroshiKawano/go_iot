# ギャップ分析 — device-create-edit

> 既存コードベース（S1 web-foundation-auth・dashboard・基盤）と本要件の差分を分析し、設計フェーズの実装戦略に資する。
> 結論先出し: **S1 が想定した前提成果物はほぼ全て実在し、本機能は既存パターンの写経で実装できる**。新規性は (1) MAC の形式検証＋大文字正規化＋一意性、(2) デバイス専用バリデーションメッセージ、(3) 2 ルート追加と PUT 配線のみ。

## 1. 現状調査（既存資産・規約）

### 1.1 S1 前提成果物 — すべて実在（要件の「S1 完了前提」は妥当）

| 前提（要件・spec-init が依存） | 実体 | 状態 |
|---|---|---|
| Session 認証ガード | `middleware.RequireAuth()`（未認証 → `302 /login`・`c.Abort()`） | ✅ 実在 |
| user_id 解決 | `auth.UserID(c) int64`（`SessionLoad` が gin ctx へ橋渡し）／`auth.SetUserID` | ✅ 実在 |
| MethodOverride | `middleware.MethodOverride`（`_method` を `strings.ToUpper` → PUT/PATCH/DELETE、`gin.Engine` の外側で適用） | ✅ 実在（モックの小文字 `value="put"` がそのまま機能） |
| CSRF | `middleware.CSRF(cfg)`（gorilla/csrf・Web グループ限定・フォーム値 `gorilla.csrf.Token` とヘッダ `X-CSRF-Token` 双方検証・dev は `PlaintextHTTPRequest`） | ✅ 実在 |
| 所有者認可（BOLA 防止） | `authz.RequireDeviceOwner(ctx, DeviceGetter, deviceID, userID) (repository.Device, error)`。sentinel: `ErrUnauthenticated`→401 / `pgx.ErrNoRows`→404 / `ErrNotOwner`→403。**成功時は device を返す**（編集フォームの既存値ロードに再利用可） | ✅ 実在 |
| バリデーション表示方針 | `translateValidationErrors(err) map[string]string` + `fieldKey` + `validationMessage`（共有バッグ無し・templ へ明示引数渡し） | ⚠️ パターンは実在だがメッセージはデバイス非対応（後述ギャップ2） |
| App レイアウト | `layout.App(AppLayoutData{Title,UserName,CSRFToken,CSSURL,Flash})`（ヘッダ＋サイドバ＋`#main-content`＋csrf meta） | ✅ 実在 |
| CSS 単一ソース配信 | `view.MountStatic`・`view.CSSURL()`・`internal/view/static.go`（go:embed）。必要クラス（`card-narrow`/`form-group`/`error-message`/`form-help`/`required-mark`/`radio-group`/`form-actions`/`btn*`）は `mocks/html/style.css` に全て実在 | ✅ 実在（**CSS Foundation タスク不要**） |

### 1.2 確立済みハンドラ規約（写経対象＝auth.go / dashboard.go）

- **ハンドラ struct**: `AuthHandler{ Repo <最小consumer interface>, SM *scs.SessionManager }`。`Repo` は `repository.Querier` が満たす最小 interface（`AuthRepo` 等）として handler 側に定義。`main.go` は `repository.New(pool)` をそのまま注入。
- **フォーム処理フロー**（`LoginPost`/`RegisterPost`）:
  1. `var form xxxForm; c.ShouldBind(&form)`（`form:` + `binding:` タグ）
  2. 失敗 → `translateValidationErrors(err)` で `Errors map[string]string` を作り、**同一ページを `renderPage(c, http.StatusOK, ...)` で再描画**（＝ **200**。422 は HTMX フォーム専用＝§7）。入力値は View に詰めて復元。
  3. 業務エラー（重複等）→ 当該 field に日本語メッセージを入れて再描画。
  4. 成功 → `c.Redirect(http.StatusSeeOther, "/...")`（**303**）。
  5. 内部エラー → `renderError(c, http.StatusInternalServerError)`（500・機密非漏洩）。
- **共通ヘルパ**: `renderPage(c, status, templ.Component)` / `renderError(c, status)`（handler パッケージ内・auth.go 末尾）。
- **View 構造体**: `page.*View`（CSSURL/CSRFToken/各 field 値/`Errors`）。認証後画面は `layout.AppLayoutData` を内包（dashboard 方式）。
- **templ フォーム**（Login.templ）: `<input type="hidden" name="gorilla.csrf.Token" value={ v.CSRFToken }/>` + 各 field `value={...}` 復元 + `<span class="error-message">{ v.Errors["xxx"] }</span>`。

### 1.3 ルーティング配線（cmd/server/main.go）

- 合成順（外→内）: `middleware.MethodOverride( sm.LoadAndSave( engine ) )`。
- Web グループ: `web := engine.Group("/", middleware.SessionLoad(sm), middleware.CSRF(cfg))`。認証必須ルートは **per-route で `middleware.RequireAuth()` を前置**（例: `web.GET("/dashboard", middleware.RequireAuth(), authH.Dashboard)`）。
- ハンドラ生成: `authH := &handler.AuthHandler{Repo: q, SM: sm}` のように合成ルートで生成。

### 1.4 repository（sqlc 生成・権威）

```go
type Device struct { ID int64; UserID int64; Name string; MacAddress string;
    Location *string; IsActive bool; LastCommunicatedAt/CreatedAt/UpdatedAt/DeletedAt pgtype.Timestamptz }
type CreateDeviceParams struct { UserID int64; Name string; MacAddress string; Location *string; IsActive bool }
type UpdateDeviceParams struct { ID int64; Name string; MacAddress string; Location *string; IsActive bool }
// Querier:
GetDevice(ctx, id int64) (Device, error)                       // WHERE id=$1 AND deleted_at IS NULL
GetDeviceByMacAddress(ctx, macAddress string) (Device, error)  // WHERE mac_address=$1 AND deleted_at IS NULL
CreateDevice(ctx, CreateDeviceParams) (Device, error)          // RETURNING *
UpdateDevice(ctx, UpdateDeviceParams) (Device, error)          // WHERE id=$1 AND deleted_at IS NULL, RETURNING *
```

- `Location` は **`*string`**（NULL = nil）。`IsActive` は **`bool`**。
- `GetDeviceByMacAddress` の絞り込みは `deleted_at IS NULL` のみ（`is_active` 不問）＝ **要件 R6 の「削除以外の全デバイス対象」と完全一致**。DB 部分 UNIQUE 索引 `devices_mac_address_unique_active`（`WHERE deleted_at IS NULL`）も同一基準。
- DB CHECK `devices_mac_address_format` = `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`（大小混在許容・索引は btree で大小区別）。

## 2. 要件 → 資産マップ（ギャップ tag）

| 要件 | 必要技術要素 | 既存資産 | ギャップ |
|---|---|---|---|
| R1 登録フォーム表示 | GET ハンドラ + page.DeviceCreate templ + App レイアウト | renderPage / App / CSS | **Missing**: `DeviceCreate.templ`・`DeviceForm.templ`・View・ハンドラ |
| R2 登録実行・303 | POST ハンドラ + ShouldBind + CreateDevice + 所有者=uid | ShouldBind 規約 / CreateDevice / auth.UserID | **Missing**: `DeviceHandler.Create`／**Constraint**: Location ""→nil・is_active "1"/"0"→bool 変換 |
| R3 編集フォーム表示・既存値復元・404 | GET ハンドラ + RequireDeviceOwner（device 返却を再利用）+ templ | authz.RequireDeviceOwner / templ 規約 | **Missing**: `DeviceEdit.templ`・`ShowEditForm` |
| R4 更新実行・303・404 | PUT ハンドラ（MethodOverride 経由）+ UpdateDevice | MethodOverride / UpdateDevice | **Missing**: `Update`／**Constraint**: PUT ルート配線 |
| R5 項目別バリデーション・入力値復元 | binding タグ + 日本語メッセージ + Errors map | translateValidationErrors **パターン** | **Missing**: デバイス専用メッセージ（"name" が auth と衝突・後述） |
| R6 MAC 形式・大文字正規化・一意性・自己除外 | regexp + ToUpper + GetDeviceByMacAddress | GetDeviceByMacAddress（基準一致） | **Missing**: 形式/正規化/一意の handler ロジック（binding タグだけでは不可） |
| R7 認証・所有者認可・所有者=session | RequireAuth + RequireDeviceOwner + uid 由来所有者 | 全て実在 | ギャップ無し（適用のみ） |
| R8 共通フォーム・キャンセル・日本語 | 共有 DeviceForm templ | templ 規約・モック | **Missing**: `DeviceForm.templ`（登録/編集共有） |

### 2.1 重点ギャップ詳細

- **ギャップ2（バリデーションメッセージ衝突）**: `validationMessage("name","required")` は現状 **「ユーザー名を入力してください」**（auth 文脈）。デバイスは **「デバイス名を入力してください」** が必要で、`fieldKey`/`validationMessage` の共有マップでは "name" キーが衝突する。→ spec-init の「`toFieldErrors()` は S1 で確立」は **パターンのみ正しく、メッセージは未対応**。**デバイス専用トランスレータ**（`device_form.go` に `toFieldErrors()` + デバイス用メッセージ）を新設するのが正。フィールド: `name`/`mac_address`/`location`/`is_active`。
- **ギャップ（App レイアウトの UserName）**: App レイアウトはヘッダにログイン中ユーザー名を表示する。よって登録/編集フォーム表示・**バリデーションエラー再描画のたびに `GetUser(uid)` が必要**（dashboard.go と同じ）。`DeviceHandler` の consumer interface に `GetUser` を含める。**spec-init プロンプトはこの取得に言及していない**（設計で明示すること）。
- **ギャップ（CSRF hidden field）**: 本フォームは通常フォーム（非 HTMX）のため、HTMX のヘッダ注入（App レイアウトの `htmx:configRequest`）は効かない。**`DeviceForm.templ` に `<input type="hidden" name="gorilla.csrf.Token" value={ v.CSRFToken }/>` を必須追加**（Login.templ と同形）。モック `device-create.html`/`device-edit.html` には CSRF 隠しフィールドが無いため、templ 写経時に付与する。
- **Constraint（型変換）**: `is_active` は radio 文字列 "1"/"0"。binding は `binding:"required,oneof=1 0"` の string 受けにし、保存時 `== "1"` で bool 化。再描画は選択状態（"1"/"0"）を View に保持。`location` 空文字 → `*string` nil 変換。
- **Constraint（再描画ステータス）**: バリデーションエラー再描画は **200**（auth.go の Login/Register と同一）。422 は使わない（§7 で HTMX フォーム専用）。

## 3. 実装アプローチ案

### Option A: 既存ファイル拡張（auth.go / auth_validation.go に相乗り）
- 内容: デバイスのハンドラ・バリデーションを既存 auth 系ファイルに追記。
- ❌ `auth.go` は認証専門で肥大化・凝集低下。❌ `validationMessage` の "name" 衝突で分岐が汚れる。❌ structure.md「機能/ドメインで整理・200-400行」に反する。
- **不採用**。

### Option B: 新規ファイル（既存パターンを写経）★推奨
- 新設: `internal/handler/device.go`（`DeviceHandler{Repo, [SM不要]}` + `ShowCreateForm/Create/ShowEditForm/Update`）、`internal/handler/device_form.go`（binding struct `CreateUpdateDeviceRequest` + `toFieldErrors()` + デバイス用メッセージ + MAC 正規化/形式/一意ヘルパ）、`internal/view/page/DeviceCreate.templ`・`DeviceEdit.templ`、`internal/view/component/DeviceForm.templ`、`page/views.go` に View 構造体追加。
- 統合: `main.go` で `deviceH := &handler.DeviceHandler{Repo: q}` を生成し web グループへ 4 ルート追加（各 `RequireAuth()` 前置）。共有ヘルパ `renderPage`/`renderError`/`authz.RequireDeviceOwner` をそのまま再利用。
- ✅ spec-init のファイル計画・structure.md の3層 view 規約・既存ハンドラ規約に整合。✅ 単体テスト容易（Querier モック）。
- **推奨**。実体は「新規ファイル＋最大限の既存資産再利用」なので下記 C と連続。

### Option C: ハイブリッド（B＋既存資産の最大再利用）= 実際の推奨形
- B の新規ファイル構成を取りつつ、**判断ロジックは可能な限り既存に委譲**:
  - 所有者認可 → `authz.RequireDeviceOwner`（編集表示・更新で共通。成功時 device を既存値ロードに再利用）。
  - レンダリング → `renderPage`/`renderError`。
  - バリデーション骨格 → `translateValidationErrors` と同型の `errors.As(validator.ValidationErrors)` ループ（メッセージのみデバイス用に差し替え）。
  - レイアウト → `layout.AppLayoutData`（UserName は `GetUser`）。
- 新規に書くのは正味 **MAC ロジック（形式/正規化/一意/自己除外）＋型変換＋2 templ ＋ルート 4 本**のみ。

## 4. 工数・リスク

- **工数: S（1–3 日）**。理由: 既存パターンが厚く、写経主体。新規は MAC ロジック・型変換・templ 3 種・ルート 4 本に限定。CSS Foundation 不要。
- **リスク: Low**。理由: 確立済みパターンの拡張で未知技術なし。所有者認可・MethodOverride・CSRF・バリデーション規約は実装＋テスト済み。

### リスク/要確認（Research Needed・いずれも Low）
1. **Gin の静的/パラメータ経路共存**: `GET /devices/create`（静的）と `GET /devices/:device/edit`・`PUT /devices/:device`（param）を同一 `/devices/` 配下に登録する。Gin v1.12 は静的優先で共存可（現代の gin で `/users/new` + `/users/:id` は一般的）。**実装時にルート登録が panic しないかスモーク確認**（万一 panic 時は登録順や param 名の見直し）。
2. **`:device` パラメータの数値化**: `c.Param("device")` を `strconv.ParseInt` で int64 化。失敗（非数値）は 404 扱いとする方針を設計で確定。
3. **CSS クラス実在の最終確認**: `card-narrow`/`radio-group`/`required-mark`/`form-help` は style.css に存在確認済み（写経でそのまま使用・独自クラス新設禁止＝§31）。
4. **MAC 大文字正規化の一貫適用点**: 正規化は「一意検査の入力」「保存値（Create/UpdateParams.MacAddress）」「自己除外の比較」すべてで正規化後の値を使う（設計で適用箇所を明示）。

## 5. 設計フェーズへの申し送り

- **推奨アプローチ: Option C（新規ファイル＋既存資産最大再利用）**。
- **設計で確定すべき主要判断**:
  1. `DeviceHandler` の consumer interface（`DeviceRepo`）に含めるメソッド = `GetUser` + `GetDevice` + `GetDeviceByMacAddress` + `CreateDevice` + `UpdateDevice`。`authz.DeviceGetter`（`GetDevice`）を内包する形にする。
  2. binding struct `CreateUpdateDeviceRequest`（`name`/`mac_address`/`location`/`is_active` の form+binding タグ）と、MAC は binding 任せにせず **handler 内で regexp 検証＋ToUpper 正規化＋一意検査**（自己除外は更新時に「取得 device.ID == 対象 ID」で許可）。
  3. デバイス専用バリデーションメッセージ（"name"=「デバイス名…」で auth と分離）。`device_form.go` に閉じる。
  4. View 構造体: 登録/編集共有の `DeviceFormView`（action/method/各 field 値/is_active 選択状態/Errors）＋ `layout.AppLayoutData`。`DeviceForm.templ` は `action`/`method`/`device(*repository.Device or nil)`/`errors` を引数に取る共有コンポーネント（R27 準拠 id=`device-form`）。
  5. バリデーションエラー再描画 = **200**、成功 = **303 → `/devices/{id}`**、不在/論理削除 = **404**、非所有 = **403**、内部 = **500**。
  6. ルート登録（web グループ・各 RequireAuth）と `main.go` への `DeviceHandler` 生成・配線。CSRF hidden field の templ 付与。
- **テスト方針（テストガイダンス集 参照）**: Querier 手書きモックで Create/Update/GetDevice/GetDeviceByMacAddress を差し替え、httptest+gin でルート動作・303/404/403/500・バリデーション（各 field・MAC 形式/重複/正規化/自己除外）・入力値復元・templ 描画（Render→buffer→Contains）・CSRF 往復を検証。カバレッジ80%以上。
- **生成ステップ**: 新規 `.templ` は `make templ`（templ generate）で `_templ.go` を生成してからビルド/テスト。

---
作成日: 2026-06-07 / 対象 spec: device-create-edit / フェーズ: requirements 後（design 前）

---

# 設計フェーズ Discovery / Synthesis ログ（2026-06-07）

> 種別判定: **Extension（既存 S1 基盤への CRUD フォーム追加）** → light discovery。新規調査はほぼ不要（上記ギャップ分析で完了）。HTMX ガイドの該当節のみ追読。

## 追読した HTMX 実装ガイド節と得た設計制約

| 節 | 行 | 得た制約 |
|---|---|---|
| §3.4 デバイス登録/編集 id一覧 | 1333-1340 | 登録・編集とも `<form>` に **id=`device-form`**（R27。フルページ POST では未使用だが共通付与）。登録/編集はフォーム構造同一のため id 名共通でよい |
| §4 device-create | 1524-1539 | GET `/devices/create`→空フォーム（is_active=稼働中）／POST `/devices`→成功 `c.Redirect(303,"/devices/{device}")`・失敗は `DeviceForm.templ` 再描画。`DeviceForm.templ` を登録/編集で共有し `action`/method/既存値を引数で切替（R27） |
| §4 device-edit | 1543-1556 | GET `/devices/{device}/edit`→既存値埋込／PUT `/devices/{device}`（hidden `_method` + MethodOverride）→成功 303・失敗再描画。**MAC 一意は自分自身を除外** |
| §6 HTMX 未使用 | 1680 | device-create/edit メインフォームは**通常フォーム（フルページ POST→リダイレクト）**。HTMX 部分更新は使わない |
| §7 バリデーション | 1685-1706 | 通常フォーム = リダイレクト(成功)/同一ページ再描画(失敗)。**422 は HTMX フォーム専用**。エラーは `map[string]string` を templ へ明示引数渡し（共有バッグ無し） |
| §8-A (4) CSRF | 2042-2045 | **非 HTMX フォームは hidden `<input name="gorilla.csrf.Token" value={token}>` 必須**（gorilla 既定フィールド名）。Handler は `csrf.Token(c.Request)` で取得し templ へ渡す。モックには無いため templ 写経時に付与 |

> 重要な権威整合: §4 行1537・spec-init は「稼働中デバイスで一意」と緩く書くが、**要件 R6 で「削除以外の全デバイス対象（is_active 不問）」に確定**。これは実装済み `GetDeviceByMacAddress`（`WHERE deleted_at IS NULL`）・DB 部分 UNIQUE 索引（`deleted_at IS NULL`）と完全一致する。設計は R6 を正とし、ガイドの緩い表現には戻さない。MAC は大文字正規化後に一意判定・保存（R6 AC3）。

## Synthesis（3 レンズ）

1. **Generalization**: 登録(create)と編集(edit)は「デバイスフォーム」という同一能力の variant。共有 `DeviceForm.templ`（component）＋共有パラメータ struct `DeviceFormView`＋共有 binding/正規化/検証ヘルパ（`device_form.go`）で一般化し、`action`/`method`/`device`/`errors` を引数で切替（R27・§4 の方針）。page は薄いラッパ（タイトル＋App レイアウト）。
2. **Build vs Adopt**: 既存資産を最大採用 — `authz.RequireDeviceOwner`（編集表示・更新の所有者認可。成功時 device を既存値ロードへ再利用）、`renderPage`/`renderError`、`translateValidationErrors` の**骨格パターン**（メッセージのみデバイス用に新設）、`middleware`（RequireAuth/CSRF/MethodOverride/SessionLoad）、sqlc クエリ。新規ライブラリ採用なし。
3. **Simplification**: **service 層を挟まない**（handler → `repository.Querier` 直行。MAC 正規化/一意は handler 内検証ヘルパで足り、業務オーケストレーションが無い＝structure.md の隣接層スキップ許容）。MAC 形式はカスタムバリデータ登録ではなく **handler 内 regexp** で検証（ceremony 削減・テスト容易）。View は登録/編集で **単一 `DeviceFormView` を共有**（DeviceCreateView/EditView に分割しない）。
