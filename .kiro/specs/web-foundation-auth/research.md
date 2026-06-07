# ギャップ分析 — web-foundation-auth (S1)

> `/kiro-validate-gap` 生成。既存コードベースと要件のギャップを分析し、design フェーズの実装戦略・要研究事項を提示する。
> 調査方法: 4領域を並行サブエージェント調査（既存コード拡張点 / scs統合 / CSRF・メソッド上書き / HTMXガイド・モック）。事実は file:line・出典URL付き。
> 結論ではなく情報と選択肢を提示する（最終決定は design フェーズ）。

## 1. 分析サマリ

- **全体像**: バックエンド（DB・デバイスAPI・CLI・sqlc 全37クエリ・config・pgxpool）は完成。Web UI 層（session 認証・templ・middleware）は**全面未着手**。既存の認証ミドルウェア（`device_auth.go`）・ハンドラ（`sensor_api.go`）・テスト（モック注入）が**そのまま手本になる**ため、新規実装は確立済みパターンの横展開が中心。
- **最大の発見（要件への影響あり）**: **scs はセッション cookie を署名しない**（不透明なランダムトークンを使用）。つまり `SESSION_SECRET` は scs では消費されない。一方 **gorilla/csrf は 32バイトの認証鍵を要求する**ため、`SESSION_SECRET`（config が本番32文字以上を検証済み）は **CSRF トークン保護鍵として自然に転用できる**。→ R1.3/R1.4 の「秘密鍵でセッションを保護」は design で「秘密鍵で CSRF を保護＋セッションは不透明トークン＋cookie 属性で保護」と再解釈するのが妥当。
- **新規スキーマが必要**: scs の PostgreSQL ストア（pgxstore）は `sessions` テーブル（token/data/expiry）の **手動 DDL が必須**（自動生成しない）。→ goose migration 追加 + `make db-snapshot` 再生成が Foundation タスク。
- **Gin 固有の制約**: Gin は**ミドルウェア実行前に HTTP メソッドでルーティング**するため、ミドルウェアで `_method` を書き換えてもルートは変わらない。メソッド上書きは **`gin.Engine` を包む http.Handler 層**（ルーティング前）で行う必要がある。scs の `LoadAndSave` も同じ http.Handler 層ミドルウェアなので、両者を `gin.Engine` の外側で合成できる。
- **推奨アプローチ**: 全領域 **Option B（新規コンポーネント作成）** を基本とし、`device_auth.go` 等のパターンを踏襲。Effort 合計 **L（1〜2週間）**、Risk **Medium**（新規ライブラリ scs/csrf 統合と Gin の http.Handler 層合成が新パターンだが、出典・手本が明確）。

---

## 2. Requirement → Asset マップ（ギャップタグ: ✅実在 / 🟡要新規 / 🔵要決定 / ⚠️制約）

| Req | 必要技術要素 | 既存資産 | ギャップ |
|---|---|---|---|
| R1 セッション認証基盤 | SessionManager・SessionLoad・user_id 格納/取得 | `device_auth.go` の Config/SetUserID/UserID パターン✅ / scs 未導入 | 🟡 `internal/auth/session_auth.go` 新規・scs/v2+pgxstore 依存追加・`sessions` テーブル migration / 🔵 scs の Gin 統合方式（http.Handler 層 vs gin middleware）/ 🔵 `SESSION_SECRET` の用途（scs は不要→CSRF 鍵へ） |
| R2 ログイン | GET/POST 表示・bcrypt 照合・session 確立・remember | `GetUserByEmail`✅ `golang.org/x/crypto`(bcrypt)✅ ハンドラ規約✅ login.html モック✅ | 🟡 `handler/auth.go` LoginHandler 新規・Guest レイアウト+Login ページ templ / 🔵 失敗時の再描画ステータス（後述 §6-5）/ 🔵 remember=Cookie.Persist+RememberMe |
| R3 ユーザー登録 | GET/POST・binding 検証・重複検出・bcrypt 生成・自動ログイン | `CreateUser`✅(Name/Email/PasswordHash) `GetUserByEmail`✅ register.html✅ | 🟡 RegisterHandler 新規・Register ページ templ・binding タグ構造体 / ⚠️ `email` UNIQUE 索引✅で重複は検出可（ただし重複は事前 GetUserByEmail 検査で扱う方針） |
| R4 ログアウト | session 破棄 | scs `Destroy()`✅(調査済) | 🟡 LogoutHandler 新規・`POST /logout` ルート |
| R5 認証ガード・ルート | RequireAuth・`/` 振り分け | `device_auth.go` の context 受け渡し手本✅ | 🟡 `middleware/require_auth.go`・`/` ハンドラ |
| R6 ダッシュボード | 認証後ページ・ユーザー名表示 | dashboard.html モック✅ `GetUser`✅ | 🟡 Dashboard ページ templ（プレースホルダ）・DashboardHandler |
| R7 共通レイアウト・部品 | Guest/App layout・SiteHeader/Sidebar/FlashMessage | view/ に layout/page/component 空ディレクトリ✅ モック構造✅ | 🟡 templ 9ファイル新規（モック写経）/ ⚠️ `id` はモックに無い→templ で付与（命名規約 §htmxガイド） |
| R8 CSRF 保護 | トークン発行/検証・meta+header・hidden input | なし（gorilla/csrf 未導入） | 🟡 `middleware/csrf.go`・CSRF ライブラリ追加 / 🔵 採用ライブラリ（gorilla/csrf vs カスタム scs-backed）/ 🔵 鍵=SESSION_SECRET |
| R9 メソッド上書き | `_method` → HTTP メソッド | なし | 🟡 `middleware/method_override.go` / ⚠️ Gin ルーティング前(http.Handler 層)で実行必須 / 🔵 採用方式（http.Handler ラッパ vs handler 内分岐） |
| R10 静的アセット配信 | CSS/JS 配信・go:embed・CSSURL | `make sync-css`✅ `public/css/style.css`✅ モック CSS✅ | 🟡 `internal/view/static.go`(go:embed)・StaticFS マウント・CSSURL ヘルパー・htmx/alpine の配置 / 🔵 htmx/alpine を CDN か public 同梱か |
| R11 セキュリティ・品質 | bcrypt・日本語UI・80%・500 | bcrypt✅ 既存テストのモック手本✅ error→HTTP 写し✅ | 🟡 各テスト新規（auth/middleware/handler）/ ✅ パターン確立済 |

---

## 3. 主要な既存資産・規約（横展開の手本）

**合成ルート** `cmd/server/main.go`: pool 生成`infradb.NewPool`(L38) → `repository.New(pool)`(L44, `*Queries`=`Querier` 実装) → `&handler.SensorAPI{Repo: q}`(L82) → `deviceAuth := auth.DeviceAuth(...)`(L83) → `apiGroup := r.Group("/api", deviceAuth)`(L85)。**Web UI グループはここに追記**（`r.Group("/", sessionLoad, csrf, ...)`）。

**認証ミドルウェアの手本** `internal/auth/device_auth.go`: `func DeviceAuth(cfg DeviceAuthConfig) gin.HandlerFunc`（L42）、最小 consumer interface `TokenRepo`（L26）、sentinel error 分岐（`pgx.ErrNoRows`）、`SetUserID(c, id)`/`UserID(c)` で context 受け渡し（L74/L88）。→ `session_auth.go` を同形で作る。

**ハンドラ規約** `internal/handler/sensor_api.go`: `type SensorAPI struct { Repo SensorRepo }`、`c.ShouldBind*` + binding タグ、`errors.Is` で error→HTTP（401/403/422/500）。テストは `fakeSensorRepo` モック + `newRouterWithUser` で `auth.SetUserID` 注入。→ `auth.go` ハンドラ・テストを同様に。

**config** `internal/config/config.go`: `Config{AppEnv, AppPort, DatabaseURL, SessionSecret}`、`Load()` が未設定を collect して一括 `fmt.Errorf`、本番のみ `SESSION_SECRET≥32`検証（L41-43）。

**repository**: `Querier`(37メソッド)、`GetUser(ctx,int64)`/`GetUserByEmail(ctx,string)`/`CreateUser(ctx,CreateUserParams{Name,Email,PasswordHash})`。`User` モデルは `EmailVerifiedAt/CreatedAt/UpdatedAt = pgtype.Timestamptz`。

**view/Makefile**: `internal/view/{layout,page,component}` は空・templ 0件・`static.go` 無し・CSSURL 無し。`make sync-css` が `mocks/html/style.css`→`internal/view/public/css/style.css` 複製、`build`/`dev` が前段で実行。

---

## 4. scs（セッション）統合の所見　出典: github.com/alexedwards/scs v2.9.0, /pgxstore

- **依存追加**: `github.com/alexedwards/scs/v2` + `github.com/alexedwards/scs/pgxstore`（pgx/v5 既存）。
- **ストア**: `pgxstore.New(pool)`（*pgxpool.Pool を直接受ける）。**`sessions` テーブルの DDL は手動**:
  ```sql
  CREATE TABLE sessions (token TEXT PRIMARY KEY, data BYTEA NOT NULL, expiry TIMESTAMPTZ NOT NULL);
  CREATE INDEX sessions_expiry_idx ON sessions (expiry);
  ```
  → **goose migration（`db/migrations/00007_create_sessions.sql`）を追加**し `make db-snapshot` 再生成。クリーンアップは goroutine が自動（既定5分間隔・`NewWithCleanupInterval` で調整）。
- **Gin 統合（2案）**:
  - **(A) http.Handler 層で `gin.Engine` を包む（推奨・標準）**: `srv := &http.Server{Handler: sessionManager.LoadAndSave(engine)}`。`LoadAndSave` が request context に session を載せた状態で gin に渡るため、ハンドラの `c.Request.Context()` でそのまま読める。commit/cookie 書込も `LoadAndSave` が担う。`r.Run()` をやめ `http.Server` を使う小変更が必要。
  - **(B) カスタム gin ミドルウェア**: `Load` → `c.Request = c.Request.WithContext(ctx)` → `c.Next()` → commit。実装/テストの手間が増えるが gin 内に閉じる。
- **値**: `Put(ctx,"user_id",int64)` / `GetInt64(ctx,"user_id")`。
- **remember me**: `Cookie.Persist=false` を既定にし、ログイン成功かつチェック時のみ `RememberMe(ctx,true)`（Expires 付与）。
- **セッション固定対策**: ログイン時 `RenewToken(ctx)`（user_id 格納の直前）、ログアウト時 `Destroy(ctx)`。
- **⚠️ SESSION_SECRET**: scs は cookie を**署名せず**、32バイト乱数トークンを使うため **SESSION_SECRET を消費しない**。→ §6-1 の決定事項参照。

---

## 5. CSRF・メソッド上書きの所見　出典: gorilla/csrf, justinas/nosurf, gin-gonic#3826, htmx.org

**CSRF ライブラリ候補**:
| 候補 | scs 互換 | header(X-CSRF-Token) | 保守 | 備考 |
|---|---|---|---|---|
| **gorilla/csrf** | ✅ 独立(自前cookie) | ✅ form+header両対応 | ✅ 2025活発 | **32バイト authKey 必須**→SESSION_SECRET 転用可。net/http→Gin アダプタ要 |
| justinas/nosurf | ✅ 独立(cookie) | ✅ 両対応 | ✅ v1.2.0 | cookie ベース |
| utrack/gin-csrf | ❌ gin-contrib/sessions 依存 | ✅ | △ | **scs と二重セッション競合→不採用** |
| カスタム(scs-backed) | ✅ 最良 | ✅ 自由 | 自前 | scs セッションにトークン格納し検証。保守/テスト責任は自前 |

- **推奨**: **gorilla/csrf**（SESSION_SECRET を authKey に転用でき要件 R1.4 と整合）。net/http ミドルウェアなので scs と同様に **http.Handler 層で合成**（`csrf.Protect(key)(handler)`）。カスタム scs-backed も有力（依存最小だが実装責任大）。
- **HTMX パターン**（ガイド §8 と一致）: App.templ `<head>` に `<meta name="csrf-token" content={ csrfToken }>`、`<body>` 末尾に `htmx:configRequest` で `event.detail.headers['X-CSRF-Token']` 付与。非HTMXフォーム（login/register）は hidden input。ヘッダ名 `X-CSRF-Token`。

**メソッド上書き（⚠️ Gin の本質的制約）**:
- Gin は**ミドルウェア前にメソッドでルート決定**するため、`engine.Use()` 内で `c.Request.Method` を書き換えても routing は変わらない（POST のまま PUT/DELETE ハンドラに届かない／404）。
- **解決策（2案）**:
  - **(A) http.Handler 層ラッパ（ルーティング前に書換・推奨）**: `gin.Engine` を包む http ミドルウェアで、POST かつ `_method∈{PUT,PATCH,DELETE}` のとき `r.Method` を書き換えてから `engine.ServeHTTP`。scs `LoadAndSave`・gorilla/csrf と**同じ http.Handler 層で合成**できる（`methodOverride(csrf(scs.LoadAndSave(engine)))`）。⚠️ `_method` は form 値なので `r.ParseForm()` が必要＝body 消費に注意（urlencoded は ParseForm がキャッシュするため後段の `ShouldBind` でも読める）。
  - **(B) handler 内分岐**: POST ルートで受け `c.PostForm("_method")` を見て分岐。RESTful な PUT/DELETE ルートにならない。
- 本セッションでは**基盤提供のみ**（実利用は S4/S7）。(A) を用意しておくと後続が `engine.PUT/DELETE` をそのまま書ける。

---

## 6. design フェーズへの決定事項・要研究（Research Needed）

1. **🔵 SESSION_SECRET の用途確定**: scs は不要。**gorilla/csrf の 32バイト authKey に転用**するのが自然（config の既存検証 R1.4 をそのまま活かせる）。採用時は R1.3 を「秘密鍵で CSRF を保護、セッションは不透明トークン+cookie 属性(HttpOnly/SameSite/Secure)で保護」と design で明文化。カスタム CSRF を選ぶ場合の鍵用途も要確定。
2. **🔵 CSRF ライブラリ**: gorilla/csrf（推奨・SESSION_SECRET 転用）か、カスタム scs-backed（依存最小）か。HTMX header + form 両対応・scs 非競合が条件（utrack/gin-csrf は不採用）。
3. **🔵 scs の Gin 統合方式**: http.Handler 層で `LoadAndSave` が `gin.Engine` を包む（A・推奨）か gin ミドルウェアアダプタ（B）か。A なら `main.go` で `r.Run()`→`http.Server` へ小変更。
4. **🔵 メソッド上書き方式**: http.Handler 層ラッパ（A・推奨、`ParseForm` body 注意）か handler 内分岐（B）か。
5. **🔵 バリデーション失敗時の再描画ステータス**: spec-init 本文は「400 + 再描画」、HTMXガイド §7 は非HTMXゲストフォームを「**200 で再描画 + errors 引数**」とする。**齟齬**。要件 R2.4/R3.x は「再表示」のみ規定（ステータス非依存）なので**要件矛盾ではない**が、design で 200 か 422/400 を確定（ガイド準拠なら 200）。
6. **🟡 sessions テーブル migration**: `db/migrations/00007_create_sessions.sql` 追加＋`make db-snapshot` 再生成（Foundation タスク）。
7. **🟡 アセット配信パイプライン（Foundation・steering 必須）**: `internal/view/static.go`（`//go:embed all:public`）→ `r.StaticFS("/static", http.FS(...))` → `/static/css/style.css?v=Version`、`CSSURL()` ヘルパー、`make sync-css`。**htmx.min.js / alpine.min.js を CDN か public 同梱か**を確定（本番再現性・オフライン考慮なら同梱）。
8. **🔵 依存追加**: `alexedwards/scs/v2`・`alexedwards/scs/pgxstore`・（CSRF 採用ライブラリ）。bcrypt は `golang.org/x/crypto` 既存。`make tidy`。
9. **🟡 templ 9ファイルはモック写経**: `id` はモックに無い→命名規約（ケバブ id ↔ PascalCase 関数）で付与。独自 CSS クラス新設禁止（steering §CSS）。`css` スコープスタイル式禁止。
10. **🔵 ログイン失敗の HTTP 表現**: ユーザー列挙防止の共通メッセージ（R2.3）。redirect せず同一ページ再描画（ガイド §7）。

---

## 7. 実装アプローチ（A/B/C）と Effort・Risk

### 採用方針: **Option B（新規コンポーネント作成）中心 + 既存パターン踏襲**
- **理由**: 認証は明確に独立した責務で、既存ファイルへの追記より新規ファイルが自然（device_auth.go と対の session_auth.go、middleware パッケージ新設、handler/auth.go、view/templ 群）。既存の合成ルート・ミドルウェア型・Repository interface・モックテストを**そのまま手本に横展開**できるため新規でも低リスク。
- **Option A（既存拡張）が当てはまる箇所**: `cmd/server/main.go`（合成ルートに Web UI グループと http.Handler 層合成を追記）、`go.mod`（依存追加）、`db/migrations`（migration 追加）。
- **Option C（ハイブリッド）視点**: 段階実装として ①Foundation（依存・migration・session/csrf/method-override 基盤・アセット配信・Guest/App レイアウト）→ ②認証フロー（login/register/logout/guard）→ ③dashboard プレースホルダ＋テスト、の3波。Walking Skeleton 性質上 ① を最初に薄く通すのが定石。

### Effort / Risk（領域別）
| 領域 | Effort | Risk | 根拠 |
|---|---|---|---|
| セッション基盤（scs+pgxstore+migration+Gin 合成） | M | Medium | 新ライブラリだが出典明確・device_auth 手本あり。http.Handler 層合成が新パターン |
| CSRF（ライブラリ選定+http.Handler 合成+HTMX 配線） | M | Medium | 選定要決定・net/http→Gin 合成・HTMX header 配線。OWASP/ガイドに前例 |
| メソッド上書き | S | Low | 小さな http.Handler ラッパ。基盤のみ・実利用は後続 |
| 認証ハンドラ（login/register/logout/guard） | M | Low | ハンドラ規約・sqlc・bcrypt 既存。確立パターンの適用 |
| templ レイアウト・ページ・部品（9ファイル） | M | Low–Medium | モック写経。命名規約/`@layer`/unlayered 禁止の遵守が要点 |
| アセット配信（go:embed/StaticFS/CSSURL） | S | Low | 正典 §40-B に手順確定。CDN/同梱の選択のみ |
| テスト 80%（auth/middleware/handler/templ） | M | Low | モック注入の手本（fakeRepo + SetUserID）あり |
| **合計** | **L（1–2週間）** | **Medium** | 複数新ライブラリ統合 + http.Handler 層合成 + 9 templ + テスト |

---

## 8. design フェーズ申し送り（要点）
- **推奨スタック**: scs/v2 + pgxstore（pgxpool 直結・sessions migration 追加）、CSRF=gorilla/csrf（SESSION_SECRET を authKey 転用）、メソッド上書き=http.Handler 層ラッパ。これらを `main.go` で **`methodOverride(csrf(scs.LoadAndSave(ginEngine)))`** の順に http.Handler 層で合成し、`http.Server` で起動。
- **要件の微修正候補**（design で確定）: R1.3 の「秘密鍵でセッション保護」を「秘密鍵で CSRF 保護＋セッションは不透明トークン+cookie 属性で保護」に明確化。バリデーション再描画ステータスを 200（ガイド準拠）に確定。
- **Foundation タスク必須化**: sessions migration（+db-snapshot 再生成）／アセット配信パイプライン（go:embed+StaticFS+CSSURL+sync-css）／htmx・alpine 配置方式。
- **遵守事項**: templ はモック写経・独自クラス禁止・`@layer components` 追記・`css` スコープスタイル式禁止・`id` はケバブ/関数 PascalCase 対応・依存方向（handler→Querier interface、view→domain 表示メソッドのみ）。
- **参照**: HTMX実装ガイド §2/§3/§4/§7/§8/§9/§40-B、DBスナップショット users/（新規）sessions、既存 `device_auth.go`・`sensor_api.go`(+test)・`config.go`・`main.go`。

---

## 9. 設計 synthesis（design フェーズで確定）

`/kiro-spec-design` の synthesis 3レンズ（一般化 / build-vs-adopt / 単純化）の適用結果。design.md の根拠。

### 一般化（Generalization）
- **統一認証 context**: device（Bearer）と session（Cookie）の両 authN を、既存 `auth.SetUserID/UserID(c)` の **単一 context キー**に集約。`SessionLoad` は scs セッション→gin context への橋渡しに徹し、`RequireAuth` とハンドラは authN 手段に依存せず `auth.UserID(c)` のみ参照。後続の所有者認可（`internal/authz`）もこの user_id を前提にできる。
- **共有レイアウト struct**: `AppLayoutData{Title,UserName,CSRFToken,CSSURL}` を後続全画面（S3〜S8）が継承する共通パラメータとして一般化（個別 props 階層を作らない）。

### build vs adopt
- **CSRF = adopt（gorilla/csrf）**: セキュリティ機構の自前実装はリスクが高く、battle-tested を採用。`utrack/gin-csrf` は gin-contrib/sessions 依存で scs と二重セッション化するため不採用。カスタム scs-backed トークンは依存最小だが実装/テスト責任が大きく却下（フォールバック候補としてのみ記録）。
- **セッション = adopt（scs/v2 + pgxstore）**: pgxpool 直結・goroutine クリーンアップ込みで要件を満たす。自前セッション管理は不採用。
- **メソッド上書き = build（小さな http.Handler）**: 既存ライブラリは Gin のルーティング前書換要件に合致せず、薄い自前ラッパが最小。
- **パスワード = adopt（x/crypto bcrypt・既存）**、**バリデーション = adopt（go-playground/validator・Gin binding・既存）**。

### 単純化（Simplification）
- **service 層を挟まない**: 認証フローは handler が `repository.Querier` を直接呼ぶ（Layered-lite で許容）。先回りの service interface を作らない。
- **scs Gin 統合は http.Handler 層 `LoadAndSave`（案A）に一本化**し、commit を手動制御する gin アダプタ（案B）は採用しない（実装/テストの単純化・取りこぼし防止）。
- **sessions は sqlc に載せない**: scs が直接管理するため Querier を増やさない。
- **gin SessionLoad は「橋渡し」だけに縮退**（load/commit は LoadAndSave が担当）。責務重複を排除。

### 確定した未確定事項（research §6 の回答）
1. SESSION_SECRET 用途 → **gorilla/csrf authKey に転用**（`sha256` で 32バイト化、dev でも安全）。R1.3 は「秘密鍵で CSRF 保護＋セッションは opaque トークン+Cookie 属性」と解釈。
2. CSRF ライブラリ → **gorilla/csrf**（web グループ限定 gin アダプタ・/api 除外）。
3. scs Gin 統合 → **案A（http.Handler 層 LoadAndSave）**＋`http.Server` 起動。
4. メソッド上書き → **案A（http.Handler 層ラッパ・engine 外側）**。
5. バリデーション再描画ステータス → **200 再描画**（HTMX実装ガイド §7・非HTMX ゲストフォーム準拠）。成功時のみ 303、`/` 振り分けは 302。
6. アセット: CSS=go:embed+StaticFS+CSSURL（単一ソース）、htmx/alpine=**CDN+SRI**（README 準拠・自前ホスティングは将来）。

---

## 10. 実装知見（タスク2 実装中に判明・要記憶）

- **gorilla/csrf v1.7.3 は既定でリクエストを HTTPS とみなす**（`requestURL.Scheme="https"`）。状態変更リクエストに対し **Origin ヘッダの同一オリジン検証**、Origin 不在時は **Referer 必須（http referer は拒否）** を強制する（`csrf.go` L264-312、`sameOrigin` は scheme+host 一致）。
  - **影響**: 開発環境（HTTP localhost）では Origin/Referer 検証が常に失敗し、全 POST が 403（"referer not supplied"）になる。
  - **対策（実装済）**: `middleware.CSRF` は本番以外で `csrf.PlaintextHTTPRequest(r)` を適用し、`isPlaintext=true` で scheme を http 扱いにして Origin/Referer 強制を回避する。本番（HTTPS）はブラウザの Origin/Referer で同一オリジン検証が機能する。
  - **デプロイ注意（S1 範囲外）**: 本番が TLS 終端プロキシ（Lightsail 等）の背後にある場合、Go は HTTP を受けるが gorilla は scheme を https と仮定するため、プロキシが `Host` を保持していれば同一オリジン検証は通る。ホストが異なる構成では `csrf.TrustedOrigins([]string{...})` の追加が必要になりうる。
- **scs の cookie は署名なし**（opaque ランダムトークン）。`SESSION_SECRET` は scs では未使用で、`middleware.csrfAuthKey`（`sha256(SESSION_SECRET)`）として gorilla/csrf の 32 バイト authKey に転用した（要件 1.3/1.4 を充足）。
- **テスト方針**: `NewSessionManager`（pgxstore・DB 必須）は unit では未カバー。cookie/有効期限方針は純粋関数 `applySessionPolicy` に切り出して DB 非依存でテスト（auth 88.7%）。middleware は in-memory scs + httptest で 100%。
