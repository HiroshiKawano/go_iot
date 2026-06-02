# セッション1 spec-init プロンプト: アプリ基盤＋認証（Walking Skeleton）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: web-foundation-auth
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: なし（全セッション最初の土台。このセッションなしに他のセッションは進まない）
> 設計フェーズで参照: 実装現状サマリ §2/§5-5/§9、画面設計書(静的) 行72-202、HTMX実装ガイド §3.1/§3.8・§4 login/register・§8/§9、templ実装仕様書 全体、DB設計書 users テーブル

--- spec-init 本文 ここから ---

## 機能概要

農場運営者がブラウザからシステムにアクセスする際の入口となる認証基盤とゲストレイアウトを整備する。現状、バックエンドの DB・API・CLI は完成しているが、Web UI 層（Session 認証・templ 画面・HTMX）は全面未着手である。このセッションでは、scs セッション認証とコア認証フロー（login/register/logout）を実装し、ログイン後に `/dashboard` へ到達できる最小限の Walking Skeleton を確立する。これにより、後続セッション（ダッシュボード・デバイス管理・アラート機能）が認証を前提として実装できるようになる。

## 背景・現状

**実装現状（実装現状サマリ §1 / §5-5 / §9 より）:**
- バックエンド API 層（6テーブル・sqlc 全37クエリ・デバイス Bearer 認証・POST /api/sensor-data）はほぼ実装完了。
- `SESSION_SECRET` は `config.Load()` で検証済みだが、`main.go` では未使用のままである（Web UI 未実装のため）。
- `cmd/server/main.go` はプレースホルダ `/` で固定文字列を返すだけ。Web UI ルーターグループが存在しない。
- `internal/view/`、`internal/middleware/`、`internal/auth/session_auth.go` は全て空もしくは未作成。
- templ ファイル（`.templ`）は0件。ただし `mocks/html/` に login.html / register.html など9画面のモックが完成済み。
- 自前 style.css（素のモダンCSS。`:root` トークン + 自前 `@layer reset, base, components, utilities`）は `mocks/html/` に配置済みで、共通デザインシステムとして利用可能（CSSフレームワーク非依存。CSS方針は `.kiro/steering/tech.md` 参照）。
- 既存 sqlc クエリ（`GetUserByEmail`、`CreateUser`、`GetUser`）は揃い、bcrypt パスワード照合は seed で実績がある。

**このセッションの位置づけ:**
全セッションの土台。Web UI 全9画面の前提となる認証ミドルウェア、共通レイアウト、ゲスト向けレイアウト、認証フロー（login/register/logout）を整備する。

## このセッションのスコープ（実装対象）

### 1. scs セッション認証基盤

- **go.mod に依存追加**: `github.com/alexedwards/scs/v2` + PostgreSQL ストアモジュール
- **internal/auth/session_auth.go 新規作成**: SessionManager の初期化、SESSION_SECRET（config で検証済み）を使用、PostgreSQL へのセッション永続化を設定
  - Handler：`NewSessionManager(pool *pgxpool.Pool) *scs.SessionManager`
  - SessionLoad ミドルウェアの提供
  - `session.GetInt(ctx, "user_id") int64` など のセッション値取得ヘルパー
- `internal/middleware/session_load.go`: SessionLoad ミドルウェア。認証後ルートに全体適用する

### 2. Web UI ルーターグループ

- **cmd/server/main.go の改修**: `/api/*` グループ（既存、DeviceAuth 適用）と分離して、`/` 非 api ルートを新設。SessionLoad + MethodOverride ミドルウェアを適用
- **ルート登録**:
  - `GET /login` → LogginHandler.Get（ログインページ表示）
  - `POST /login` → LoginHandler.Post（ログイン処理）
  - `GET /register` → RegisterHandler.Get（ユーザー登録ページ表示）
  - `POST /register` → RegisterHandler.Post（ユーザー新規登録）
  - `POST /logout` → LogoutHandler.Post（ログアウト）
  - `GET /dashboard` → DashboardHandler.Get（認証必須、プレースホルダ可）
  - `GET /` → HTTP 303 リダイレクト `/login` または認証済なら `/dashboard`

### 3. MethodOverride ミドルウェア

- **internal/middleware/method_override.go 新規作成**: `<form>` の隠しフィールド `_method`（PUT/PATCH/DELETE）を HTTP メソッドとして解釈
  - 後続セッションの device-edit / alert-rules 更新時に使用（device-create/edit はこのセッションスコープ外）
  - HTML モック（device-create.html 等）では `_method=PUT` フォームが使われているため、基盤として用意が必須

### 4. CSRF ミドルウェア（採用ライブラリ未確定）

- **internal/middleware/csrf.go 新規作成**: Gin の CSRF ミドルウェア（採用ライブラリは要確認事項。gorilla/csrf 等を想定）
  - Handler から CSRF トークンを取得し、templ レイアウトへ引数で渡す
  - 非 HTMX フォーム（login/register）は hidden input でトークン送信
  - HTMX リクエストは `htmx:configRequest` で `X-CSRF-Token` ヘッダーに自動付与
  - ヘッダー名や検証方式は採用ライブラリで確定

### 5. 認証ガード

- **internal/middleware/require_auth.go 新規作成**: 認証必須ルート用ミドルウェア
  - セッションから user_id を取得。不在なら `/login` へリダイレクト（またはステータス 401 返却）

### 6. templ レイアウト＆ページ

#### 6.1 ゲストレイアウト（未認証用）
- **internal/view/layout/Guest.templ 新規作成**
  - login / register 画面共用
  - ヘッダー・サイドバーなし
  - 中央寄せカード形式
  - CSRF トークン・HTMX 用の meta 要素は不要（ゲスト画面は HTMX を使わない）

#### 6.2 App レイアウト（認証後）
- **internal/view/layout/App.templ 新規作成**
  - ヘッダー（site-header コンポーネント）、サイドバー、メインコンテンツ領域
  - `<meta name="csrf-token">` を含む（Handler から csrfToken 引数で受け取る）
  - HTMX 用 `htmx:configRequest` スクリプト（X-CSRF-Token ヘッダー自動付与）
  - id: `main-content`（HTMX hx-boost のターゲット）、`flash-message`（共通通知領域）
  - Alpine.js スクリプト（ユーザーメニュー開閉用）

#### 6.3 ページコンポーネント
- **internal/view/page/Login.templ**: ログインフォーム（email, password, remember checkbox）
- **internal/view/page/Register.templ**: ユーザー登録フォーム（name, email, password, password_confirmation）
- **internal/view/page/Dashboard.templ**: 認証後の最小プレースホルダ（ユーザー名表示、「デバイス一覧」へのリンク等）

#### 6.4 共通コンポーネント
- **internal/view/component/SiteHeader.templ**: ロゴ + ユーザー名 + ユーザーメニュー（Alpine.js ローカル状態で開閉）、ログアウトボタン
- **internal/view/component/Sidebar.templ**: ナビゲーション（ダッシュボード・アラートルール・アラート履歴）
  - 現在ページのハイライト表示は後続セッションで実装（このセッションでは固定 HTML でよい）
- **internal/view/component/FlashMessage.templ**: エラー・成功通知の表示領域（id: `flash-message`）

### 7. 認証ハンドラ

#### 7.1 Login ハンドラ（internal/handler/auth.go）

**GET /login**
- templ.Login() を Guest.templ で描画（セッション user_id なければ許可）
- フォーム項目: email / password / remember

**POST /login**
- `c.ShouldBind` で email / password / remember を取得
- バリデーション: email 必須・形式、password 必須
- `repo.GetUserByEmail(ctx, email)` でユーザー検索
  - 不在 → バリデーションエラー（「メールアドレスまたはパスワードが間違っています」）を map[string]string で templ へ引数で渡し、ページ再描画
  - 存在 → `bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password))` で照合
    - 不一致 → 同じエラー
    - 一致 → セッションに user_id を格納。remember チェック時は `session.Put(ctx, "user_id", user.ID)` + `manager.Commit(ctx)` で永続化。303 See Other で `/dashboard` へリダイレクト

#### 7.2 Register ハンドラ（internal/handler/auth.go）

**GET /register**
- templ.Register() を Guest.templ で描画

**POST /register**
- `c.ShouldBind` で name / email / password / password_confirmation を取得
- バリデーション（binding タグで実装）:
  - `name`: `binding:"required,max=255"`
  - `email`: `binding:"required,email"`
  - `password`: `binding:"required,min=8"`
  - `password_confirmation`: `binding:"required,eqfield=Password"`
- `repo.GetUserByEmail(ctx, email)` で重複チェック
  - 既存 → バリデーションエラー（「このメールアドレスは既に登録されています」）
- `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)` でパスワードハッシュ化
- `repo.CreateUser(ctx, repository.CreateUserParams{Name, Email, PasswordHash})` で新規作成
- 作成後、自動ログイン（セッションに user_id を格納）
- 303 See Other で `/dashboard` へリダイレクト

#### 7.3 Logout ハンドラ（internal/handler/auth.go）

**POST /logout**
- セッション破棄: `session.Remove(ctx, "user_id")` + `manager.Commit(ctx)`
- 303 See Other で `/login` へリダイレクト

### 8. 静的アセット配信

- **public/css/style.css**: mocks/html/style.css から移植（素のモダンCSS。`:root` トークン + 自前 `@layer` を内包。これ単体で全画面のスタイルが完結し、外部CSSフレームワークは配信しない。CSS方針は `.kiro/steering/tech.md` 参照）
- **public/js/htmx.min.js**: CDN or ローカル配置（具体方式は要確認）
- **public/js/alpine.min.js**: 同上

### 9. 使用 sqlc クエリ

- `GetUserByEmail(ctx context.Context, email string) (User, error)`: ログイン時検索
- `CreateUser(ctx, params CreateUserParams{Name, Email, PasswordHash}) (User, error)`: 登録時作成
- `GetUser(ctx, id int64) (User, error)`: セッション取得時の user_id から情報取得（後続セッション用）

### 10. 新規作成ファイル一覧

```
internal/
├── auth/
│   └── session_auth.go              # SessionManager 初期化・ヘルパー
├── middleware/
│   ├── session_load.go              # SessionLoad ミドルウェア
│   ├── method_override.go           # _method フォームフィールド処理
│   ├── csrf.go                      # CSRF トークン発行・検証
│   └── require_auth.go              # 認証必須ガード
├── handler/
│   └── auth.go                      # LoginHandler / RegisterHandler / LogoutHandler
├── view/
│   ├── layout/
│   │   ├── App.templ                # 認証後レイアウト
│   │   └── Guest.templ              # ゲストレイアウト
│   ├── page/
│   │   ├── Login.templ              # ログイン
│   │   ├── Register.templ           # 登録
│   │   └── Dashboard.templ          # ダッシュボード（プレースホルダ）
│   └── component/
│       ├── SiteHeader.templ         # ヘッダー
│       ├── Sidebar.templ            # サイドバー
│       └── FlashMessage.templ       # 通知領域

cmd/
└── server/
    └── main.go                      # ルーター改修（Web UI グループ追加）

public/
├── css/
│   └── style.css                    # 素のモダンCSS（:root トークン + 自前 @layer。CSSフレームワーク非依存）
└── js/
    ├── htmx.min.js
    └── alpine.min.js
```

## スコープ外（このセッションでやらないこと）

- **ダッシュボード機能（S3）**: デバイス一覧・未対応アラート表示は最小プレースホルダのみ。正規実装は別セッション。
- **デバイス管理（S4）**: 登録・編集・詳細・削除はこのセッションでは実装しない。MethodOverride ミドルウェアは基盤として用意するが、使用は後続セッションから。
- **アラート機能（S5-8）**: ルール管理・履歴・判定ロジックはこのセッションスコープ外。
- **アセット最適化**: バージョンクエリ・minify・バンドル最適化は後から。このセッションでは public/ へのコピー配信に留める。
- **エラーハンドリング詳細**: 本セッションは認証成功・失敗の2系統に留める。複雑な例外系（DB接続失敗等）は最小限の 500 エラーページで統一。

## 技術制約・準拠事項

### 技術スタック
- Go 1.26 + Gin v1.12 + templ v0.3 + HTMX + Alpine.js + 素のモダンCSS（CSSフレームワーク非依存。CSS方針は `.kiro/steering/tech.md` 参照） + sqlc v1.30 + pgx/v5 + scs v2（新規）
- PostgreSQL 16（既存）

### 認証・セッション
- **scs（alexedwards/scs/v2）** + PostgreSQL ストア: セッション管理（SESSION_SECRET は config で 32文字以上を検証）
- **bcrypt**: パスワードハッシュ化・照合（seed で実績）
- **CSRF**: ミドルウェア採用ライブラリは要確認。ただし以下の実装パターンは言語非依存で流用可:
  - 非 HTMX フォーム: hidden input でトークン送信
  - HTMX リクエスト: `htmx:configRequest` で `X-CSRF-Token` ヘッダーに自動付与

### templ 実装
- **HTML モック → templ 移植**: mocks/html/login.html / register.html → internal/view 配下に変換（id ゼロ件なので templ 変換時に付与不要）
- **共通レイアウト設計**: Guest.templ（login/register）と App.templ（認証後）の2系統。他セッションの9画面すべてが App.templ を継承。
- **コンポーネント分割**: SiteHeader / Sidebar / FlashMessage を再利用可能な部品として独立させる（後続セッションで各画面が同じコンポーネント関数を呼ぶ）。
- **エラー表示**: Go では Laravel の共有 errors バッグは無く、エラーは `map[string]string` として templ へ明示的に引数で渡す（フォーム再描画時に各項目の `.error-message` に表示）。

### バリデーション
- **binding タグ**: Gin の `binding:"required,email,max=255"` 等で実装（go-playground/validator）
- **カスタムバリデーション**: password_confirmation は `binding:"eqfield=Password"`
- **エラーメッセージ**: 日本語で templ へ渡す

### エラーハンドリング
- **バリデーション失敗**: ステータス 400 + フォーム再描画（エラー表示）
- **データベース失敗**: ステータス 500 + 簡潔なエラーメッセージ
- **メールアドレス重複**: バリデーション扱い（「このメールアドレスは既に登録されています」）
- **パスワード不一致**: 「メールアドレスまたはパスワードが間違っています」で統一（ユーザー列挙攻撃防止）

### 日本語化
- ボタン: 「ログイン」「登録」「ログアウト」
- プレースホルダ: 「メールアドレス」「パスワード」「ユーザー名」
- エラー: 「メールアドレス形式で入力してください」「8文字以上で入力してください」等
- フォーム項目名も全て日本語表示

### テスト・品質基準
- **テストカバレッジ**: 80% 以上（TDD 方針）
- **対象**: auth ハンドラ（login POST 成功・失敗・email 重複）、middleware（SessionLoad・RequireAuth）、component（テンプレート生成）
- **repo モック**: sqlc の `emit_interface=true` を活かし、Repository インターフェースをモック化して、DB 接続なしでハンドラテスト（種子データ使用。詳細は後続セッションで整備）

### 参照ドキュメント
- 実装現状サマリ.md §2/§5-5/§9: 現在のアーキ・実装状況・次のステップ確認
- 画面設計書(静的).md 行72-202: 共通レイアウト・login・register の要件定義
- HTMX実装ガイド(動的).md §3.1/§3.8・§4 login/register・§8 CSRF・§9 HX-Redirect: 動的振る舞い・バリデーション再表示・リダイレクト処理
- templ実装仕様書.md: templ コンポーネント分割・Handler からの呼び出しパターン・OOB Swap（この sesssion では使わないが仕様把握）
- DB設計書.md users テーブル: ID / name / email / password_hash / email_verified_at / created_at / updated_at・制約・sqlc 方針

## 受け入れ基準（概略）

このセッション完了の判定基準（概略。詳細は design フェーズで確定）:

1. ✅ **scs セッション認証基盤が動作**: 
   - SessionManager が PostgreSQL へセッション永続化
   - session.GetInt(ctx, "user_id") でセッション値取得可
   - SESSION_SECRET 環境変数を正しく使用

2. ✅ **ログイン画面が表示・機能**:
   - GET /login で Guest.templ のログインフォーム表示
   - POST /login で email / password / remember を受け取り
   - bcrypt で照合、セッション設定、/dashboard へ 303 リダイレクト
   - バリデーション失敗時（email 形式 or 不在・パスワード不一致）はフォーム再描画でエラー表示

3. ✅ **ユーザー登録画面が表示・機能**:
   - GET /register で Guest.templ の登録フォーム表示
   - POST /register で name / email / password / password_confirmation を受け取り
   - バリデーション（name 255字・email 形式・password 8字以上・confirmation 一致）
   - CreateUser で DB 登録、自動ログイン、/dashboard へ 303 リダイレクト
   - email 重複時はフォーム再描画でエラー表示

4. ✅ **ログアウト処理が動作**:
   - POST /logout でセッション破棄
   - /login へ 303 リダイレクト

5. ✅ **認証ガード機能**:
   - 未ログイン状態で /dashboard へアクセス → /login へリダイレクト
   - ログイン後は セッション user_id から ユーザー情報を取得・表示可

6. ✅ **templ レイアウト統合**:
   - Guest.templ が login / register で使用（ヘッダー・サイドバーなし）
   - App.templ が dashboard で使用（ヘッダー・サイドバー・flash-message）
   - SiteHeader / Sidebar / FlashMessage コンポーネントが複数画面で再利用可能

7. ✅ **CSRF 対応（基盤）**:
   - CSRF ミドルウェア導入（採用ライブラリ確定後）
   - 非 HTMX フォーム（login/register）で hidden input でトークン送信
   - App.templ に `<meta name="csrf-token">` + htmx:configRequest スクリプト配置

8. ✅ **MethodOverride ミドルウェア（基盤）**:
   - internal/middleware/method_override.go で PUT/PATCH/DELETE フォーム対応
   - 後続セッション（device-edit 等）で使用可能

9. ✅ **テスト・品質**:
   - auth handler（login/register/logout）のユニットテスト・統合テスト（Repository モック）で 80% カバレッジ達成
   - middleware テスト（SessionLoad・RequireAuth）で動作検証
   - バリデーション失敗・DB 失敗の エラーパス テスト

10. ✅ **コード品質・可読性**:
    - パッケージ分割・関数命名・エラー処理が一貫して日本語コメント付き
    - 後続セッションが容易に extend 可能（config / middleware / handler インターフェース明確）

## 未確定事項・要確認（あれば）

1. **CSRF ミドルウェア採用ライブラリ** 
   - 候補: gorilla/csrf / (Gin 公式なら) / 他
   - ヘッダー名（本設計は仮に `X-CSRF-Token`）、トークン生成・検証の詳細方式
   - Handler からトークン取得ヘルパーの実装パターン

2. **PUT / DELETE / PATCH のフォーム送信方式** 
   - 採用方式: _method 隠しフィールド + MethodOverride ミドルウェア vs POST 受けで Handler 分岐か
   - 本セッションでは MethodOverride ミドルウェアを基盤として用意するが、実装時に確定

3. **セッション永続化ストア** 
   - scs のデフォルトは in-memory（開発用）
   - 本番: PostgreSQL ストアを使う（sqlc は セッションテーブルを管理するか、scs が自動生成するか）

4. **静的アセット配信方式**
   - 開発: FileServer （public/ を serve）
   - 本番: go:embed + バージョンクエリ / CDN分岐 等
   - この本セッションではシンプルに public/ FileServer でよい

5. **ユーザー新規登録時の email_verified_at**
   - 設計: email 検証が無いため、登録時に即座に設定するか、NULL のままか（後続セッション依存）
   - 本セッション: NULL で実装し、後続で必要に応じて設定ロジック追加

---

--- spec-init 本文 ここまで ---
