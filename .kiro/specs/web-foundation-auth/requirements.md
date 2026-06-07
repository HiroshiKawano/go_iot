# Requirements Document

## Project Description (Input)

**S1 アプリ基盤＋認証（Walking Skeleton）** — 元プロンプト: `@2cc_sdd/spec-init-prompts/session-01-web-foundation-auth.md`

> 設計フェーズで参照: 実装現状サマリ §2/§5-5/§9、画面設計書(静的) 行72-202、HTMX実装ガイド §3.1/§3.8・§4 login/register・§8/§9、HTMX実装ガイド §1（templ コンポーネント分割アプローチ）、DB設計書 users テーブル
>
> 前提セッション: なし（全セッション最初の土台。このセッションなしに他のセッションは進まない）

### 機能概要

農場運営者がブラウザからシステムにアクセスする際の入口となる認証基盤とゲストレイアウトを整備する。現状、バックエンドの DB・API・CLI は完成しているが、Web UI 層（Session 認証・templ 画面・HTMX）は全面未着手である。このセッションでは、scs セッション認証とコア認証フロー（login/register/logout）を実装し、ログイン後に `/dashboard` へ到達できる最小限の Walking Skeleton を確立する。これにより、後続セッション（ダッシュボード・デバイス管理・アラート機能）が認証を前提として実装できるようになる。

### 背景・現状

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

### このセッションのスコープ（実装対象）

#### 1. scs セッション認証基盤
- **go.mod に依存追加**: `github.com/alexedwards/scs/v2` + PostgreSQL ストアモジュール
- **internal/auth/session_auth.go 新規作成**: SessionManager の初期化、SESSION_SECRET（config で検証済み）を使用、PostgreSQL へのセッション永続化を設定
  - Handler：`NewSessionManager(pool *pgxpool.Pool) *scs.SessionManager`
  - SessionLoad ミドルウェアの提供
  - `session.GetInt(ctx, "user_id") int64` などのセッション値取得ヘルパー
- `internal/middleware/session_load.go`: SessionLoad ミドルウェア。認証後ルートに全体適用する

#### 2. Web UI ルーターグループ
- **cmd/server/main.go の改修**: `/api/*` グループ（既存、DeviceAuth 適用）と分離して、`/` 非 api ルートを新設。SessionLoad + MethodOverride ミドルウェアを適用
- **ルート登録**:
  - `GET /login` → LoginHandler.Get（ログインページ表示）
  - `POST /login` → LoginHandler.Post（ログイン処理）
  - `GET /register` → RegisterHandler.Get（ユーザー登録ページ表示）
  - `POST /register` → RegisterHandler.Post（ユーザー新規登録）
  - `POST /logout` → LogoutHandler.Post（ログアウト）
  - `GET /dashboard` → DashboardHandler.Get（認証必須、プレースホルダ可）
  - `GET /` → HTTP 303 リダイレクト `/login` または認証済なら `/dashboard`

#### 3. MethodOverride ミドルウェア
- **internal/middleware/method_override.go 新規作成**: `<form>` の隠しフィールド `_method`（PUT/PATCH/DELETE）を HTTP メソッドとして解釈
  - 後続セッションの device-edit / alert-rules 更新時に使用（device-create/edit はこのセッションスコープ外）
  - HTML モック（device-create.html 等）では `_method=PUT` フォームが使われているため、基盤として用意が必須

#### 4. CSRF ミドルウェア（採用ライブラリ未確定）
- **internal/middleware/csrf.go 新規作成**: Gin の CSRF ミドルウェア（採用ライブラリは要確認事項。gorilla/csrf 等を想定）
  - Handler から CSRF トークンを取得し、templ レイアウトへ引数で渡す
  - 非 HTMX フォーム（login/register）は hidden input でトークン送信
  - HTMX リクエストは `htmx:configRequest` で `X-CSRF-Token` ヘッダーに自動付与
  - ヘッダー名や検証方式は採用ライブラリで確定

#### 5. 認証ガード
- **internal/middleware/require_auth.go 新規作成**: 認証必須ルート用ミドルウェア
  - セッションから user_id を取得。不在なら `/login` へリダイレクト（またはステータス 401 返却）

#### 6. templ レイアウト＆ページ

**6.1 ゲストレイアウト（未認証用）**
- **internal/view/layout/Guest.templ 新規作成**
  - login / register 画面共用 / ヘッダー・サイドバーなし / 中央寄せカード形式
  - CSRF トークン・HTMX 用の meta 要素は不要（ゲスト画面は HTMX を使わない）

**6.2 App レイアウト（認証後）**
- **internal/view/layout/App.templ 新規作成**
  - ヘッダー（site-header コンポーネント）、サイドバー、メインコンテンツ領域
  - `<meta name="csrf-token">` を含む（Handler から csrfToken 引数で受け取る）
  - HTMX 用 `htmx:configRequest` スクリプト（X-CSRF-Token ヘッダー自動付与）
  - id: `main-content`（HTMX hx-boost のターゲット）、`flash-message`（共通通知領域）
  - Alpine.js スクリプト（ユーザーメニュー開閉用）

**6.3 ページコンポーネント**
- **internal/view/page/Login.templ**: ログインフォーム（email, password, remember checkbox）
- **internal/view/page/Register.templ**: ユーザー登録フォーム（name, email, password, password_confirmation）
- **internal/view/page/Dashboard.templ**: 認証後の最小プレースホルダ（ユーザー名表示、「デバイス一覧」へのリンク等）

**6.4 共通コンポーネント**
- **internal/view/component/SiteHeader.templ**: ロゴ + ユーザー名 + ユーザーメニュー（Alpine.js ローカル状態で開閉）、ログアウトボタン
- **internal/view/component/Sidebar.templ**: ナビゲーション（ダッシュボード・アラートルール・アラート履歴）。現在ページのハイライトは後続セッション（このセッションでは固定 HTML でよい）
- **internal/view/component/FlashMessage.templ**: エラー・成功通知の表示領域（id: `flash-message`）

#### 7. 認証ハンドラ（internal/handler/auth.go）

**7.1 Login**
- **GET /login**: templ.Login() を Guest.templ で描画（セッション user_id なければ許可）。フォーム項目: email / password / remember
- **POST /login**:
  - `c.ShouldBind` で email / password / remember を取得。バリデーション: email 必須・形式、password 必須
  - `repo.GetUserByEmail(ctx, email)` でユーザー検索
    - 不在 → バリデーションエラー（「メールアドレスまたはパスワードが間違っています」）を map[string]string で templ へ渡しページ再描画
    - 存在 → `bcrypt.CompareHashAndPassword(...)` で照合。不一致 → 同じエラー / 一致 → セッションに user_id 格納（remember 時は永続化）、303 See Other で `/dashboard`

**7.2 Register**
- **GET /register**: templ.Register() を Guest.templ で描画
- **POST /register**:
  - `c.ShouldBind` で name / email / password / password_confirmation を取得。binding タグ: `name:"required,max=255"` / `email:"required,email"` / `password:"required,min=8"` / `password_confirmation:"required,eqfield=Password"`
  - `repo.GetUserByEmail` で重複チェック（既存 → 「このメールアドレスは既に登録されています」）
  - `bcrypt.GenerateFromPassword(...)` でハッシュ化 → `repo.CreateUser(...)` → 自動ログイン → 303 で `/dashboard`

**7.3 Logout**
- **POST /logout**: セッション破棄（`session.Remove` + `manager.Commit`）→ 303 で `/login`

#### 8. 静的アセット配信
- **public/css/style.css**: mocks/html/style.css から移植（素のモダンCSS。CSSフレームワーク非配信。CSS方針は `.kiro/steering/tech.md`）
- **public/js/htmx.min.js** / **public/js/alpine.min.js**: CDN or ローカル配置（具体方式は要確認）

#### 9. 使用 sqlc クエリ
- `GetUserByEmail(ctx, email string) (User, error)`: ログイン時検索
- `CreateUser(ctx, params CreateUserParams{Name, Email, PasswordHash}) (User, error)`: 登録時作成
- `GetUser(ctx, id int64) (User, error)`: セッション user_id から情報取得（後続セッション用）

#### 10. 新規作成ファイル一覧
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
│   ├── layout/{App.templ, Guest.templ}
│   ├── page/{Login.templ, Register.templ, Dashboard.templ}
│   └── component/{SiteHeader.templ, Sidebar.templ, FlashMessage.templ}
cmd/server/main.go                   # ルーター改修（Web UI グループ追加）
public/css/style.css
public/js/{htmx.min.js, alpine.min.js}
```

### スコープ外（このセッションでやらないこと）
- **ダッシュボード機能（S3）**: デバイス一覧・未対応アラート表示は最小プレースホルダのみ。
- **デバイス管理（S4）**: 登録・編集・詳細・削除は実装しない。MethodOverride は基盤として用意するが使用は後続から。
- **アラート機能（S5-8）**: ルール管理・履歴・判定ロジックはスコープ外。
- **アセット最適化**: バージョンクエリ・minify・バンドルは後から。public/ へのコピー配信に留める。
- **エラーハンドリング詳細**: 認証成功・失敗の2系統に留める。複雑な例外系は最小限の 500 エラーページで統一。

### 技術制約・準拠事項

**技術スタック**
- Go 1.26 + Gin v1.12 + templ v0.3 + HTMX + Alpine.js + 素のモダンCSS（CSSフレームワーク非依存） + sqlc v1.30 + pgx/v5 + scs v2（新規）/ PostgreSQL 16（既存）

**認証・セッション**
- **scs（alexedwards/scs/v2）** + PostgreSQL ストア（SESSION_SECRET は config で 32文字以上を検証）
- **bcrypt**: パスワードハッシュ化・照合（seed で実績）
- **CSRF**: 採用ライブラリは要確認。非 HTMX フォームは hidden input、HTMX は `htmx:configRequest` で `X-CSRF-Token` ヘッダー付与

**templ 実装**
- HTML モック → templ 移植（mocks/html/login.html / register.html → internal/view 配下）
- 共通レイアウト: Guest.templ（login/register）と App.templ（認証後）の2系統。後続9画面は App.templ を継承
- コンポーネント分割: SiteHeader / Sidebar / FlashMessage を再利用部品として独立
- エラー表示: Go では共有 errors バッグが無いため `map[string]string` を templ へ明示的に渡す

**バリデーション**
- binding タグ（go-playground/validator）。password_confirmation は `eqfield=Password`。エラーメッセージは日本語

**エラーハンドリング**
- バリデーション失敗: 400 + フォーム再描画 / DB 失敗: 500 + 簡潔メッセージ
- email 重複: バリデーション扱い / パスワード不一致: 「メールアドレスまたはパスワードが間違っています」で統一（ユーザー列挙攻撃防止）

**日本語化**
- ボタン「ログイン」「登録」「ログアウト」、プレースホルダ「メールアドレス」「パスワード」「ユーザー名」、エラー「メールアドレス形式で入力してください」等。フォーム項目名も全て日本語表示

**テスト・品質基準**
- カバレッジ 80% 以上（TDD）。対象: auth ハンドラ（login POST 成功・失敗・email 重複）、middleware（SessionLoad・RequireAuth）、component（テンプレート生成）
- repo モック: sqlc `emit_interface=true` を活かし Repository インターフェースをモック化し DB 接続なしでハンドラテスト

**参照ドキュメント**
- 実装現状サマリ.md §2/§5-5/§9 / 画面設計書(静的).md 行72-202 / HTMX実装ガイド(動的).md §1・§3.1/§3.8・§4 login/register・§5・§8 CSRF・§9 HX-Redirect / DB設計書.md users テーブル

### 受け入れ基準（概略。詳細は design フェーズで確定）
1. scs セッション認証基盤が動作（PostgreSQL 永続化・session 値取得・SESSION_SECRET 使用）
2. ログイン画面が表示・機能（GET 表示 / POST 照合・セッション設定・303・失敗時フォーム再描画）
3. ユーザー登録画面が表示・機能（GET 表示 / POST バリデーション・CreateUser・自動ログイン・303・email 重複表示）
4. ログアウト処理が動作（セッション破棄・303 で /login）
5. 認証ガード機能（未ログインで /dashboard → /login、ログイン後は user_id から情報取得）
6. templ レイアウト統合（Guest/App の2系統、SiteHeader/Sidebar/FlashMessage 再利用）
7. CSRF 対応（基盤）（ミドルウェア導入・hidden input・meta + htmx:configRequest）
8. MethodOverride ミドルウェア（基盤）（PUT/PATCH/DELETE フォーム対応・後続で使用可能）
9. テスト・品質（auth handler 80% カバレッジ・middleware テスト・エラーパステスト）
10. コード品質・可読性（日本語コメント・後続セッションが extend 容易）

### 未確定事項・要確認
1. **CSRF ミドルウェア採用ライブラリ**（候補: gorilla/csrf 等。ヘッダー名・トークン生成検証方式・Handler 取得ヘルパー）
2. **PUT/DELETE/PATCH のフォーム送信方式**（_method 隠しフィールド + MethodOverride vs POST 受け Handler 分岐）
3. **セッション永続化ストア**（scs デフォルトは in-memory。本番 PostgreSQL ストア。テーブルは sqlc 管理か scs 自動生成か）
4. **静的アセット配信方式**（開発: FileServer / 本番: go:embed + バージョンクエリ等。本セッションは public/ FileServer）
5. **ユーザー新規登録時の email_verified_at**（email 検証無し。本セッションは NULL で実装し後続で必要に応じ設定）

## Introduction

本仕様は、バックエンド（DB・デバイス API・CLI）が完成済みの農業IoTシステムに対し、ブラウザからの利用入口となる **Web UI の認証基盤（Walking Skeleton）** を確立する。農場運営者がメールアドレスとパスワードでログイン・新規登録・ログアウトでき、認証後にアプリ共通レイアウトを備えた `/dashboard` へ到達できる最小経路を整備する。あわせて、後続の全画面（ダッシュボード・デバイス管理・アラート機能）が前提とする横断的な仕組み（セッション認証・認証ガード・共通レイアウトと再利用コンポーネント・CSRF 保護・HTTP メソッド上書き・静的アセット配信）を用意する。本セッションは全セッションの土台であり、これなしに後続セッションは進行できない。

なお、本書はユーザー／オペレーターから観測可能な振る舞いと境界（WHAT）のみを規定する。採用ライブラリ・データモデル・内部構造・配線（HOW）は design フェーズで確定する。

## Boundary Context

- **In scope（本セッションで実現する振る舞い）**:
  - セッションによるログイン状態の維持と、その維持に用いる秘密鍵の設定検証
  - ログイン（表示・認証・失敗時再表示・ログイン状態保持）
  - ユーザー登録（表示・入力検証・重複検出・自動ログイン）
  - ログアウト（セッション破棄・再ログイン要求）
  - 認証ガード（認証必須ページの未認証アクセス遮断）とルート `/` のリダイレクト
  - 認証後ダッシュボード（最小プレースホルダ）
  - ゲスト／認証後の共通レイアウトと再利用 UI コンポーネント（ヘッダー・サイドバー・通知領域）
  - 状態変更リクエストの CSRF 保護（基盤）
  - HTML フォームからの HTTP メソッド上書き（基盤）
  - 共通スタイルシートおよびクライアントスクリプトの配信
  - 認証 UI の日本語化、パスワードのハッシュ化保存、自動テスト 80% 以上
- **Out of scope（本セッションで実装しない）**:
  - ダッシュボードの正規機能（デバイス一覧・未対応アラート）＝ S3
  - デバイス管理（登録・編集・詳細・削除）＝ S4・S5。メソッド上書きは基盤提供のみ
  - アラート機能（ルール管理・履歴・判定ロジック）＝ S5–S8
  - メール確認フロー（登録時にメール検証を課さない）
  - アセット最適化（バージョンクエリ・minify・バンドル）
  - 認証成功／失敗以外の詳細なエラー系（内部失敗は簡潔な 500 応答に統一）
- **Adjacent expectations（隣接システム・後続への期待。本セッションが所有しない範囲）**:
  - 既存の `users` テーブルとユーザー検索・作成・取得のクエリに依存し、本セッションでスキーマ変更は行わない前提。
  - セッション秘密鍵の設定・強度検証は既存の環境設定機構に依存する。
  - 画面の構造・クラス・スタイルは既存の HTML モックと共通スタイルを正とし、design フェーズで反映する。
  - 後続の全画面は、本セッションが用意するアプリ共通レイアウトと再利用コンポーネントを継承する前提。
  - セッションの永続化方式（サーバ再起動をまたぐ保持の有無・保存先）、CSRF の具体方式、メソッド上書きの具体方式、アセット配信方式は design フェーズで確定する（本書は振る舞い・境界のみ規定）。

## Requirements

### Requirement 1: セッション認証基盤
**Objective:** 農場運営者として、ログイン状態がリクエストをまたいで維持されてほしい。そうすれば毎回ログインし直さずにアプリを操作できる。

#### Acceptance Criteria
1. When 認証フローでユーザーの本人確認が成立する, the Auth Foundation shall 当該ユーザーを識別する認証セッションを確立する。
2. While 有効な認証セッションが存在する, the Auth Foundation shall 後続リクエストで当該ユーザーを認証済みとして識別できるようにする。
3. The Auth Foundation shall 認証セッションの保護に、環境設定で与えられるセッション秘密鍵を使用する。
4. If セッション秘密鍵が未設定、または要求される強度（最小長）を満たさない, then the Web application shall 起動を中止し、設定不備を明示して通知する。
5. When ユーザーの認証状態が変化する（ログイン成立・ログアウト）, the Auth Foundation shall セッション識別子を再生成または破棄し、セッション固定攻撃を防止する。

### Requirement 2: ログイン
**Objective:** 登録済みの農場運営者として、メールアドレスとパスワードでログインしたい。そうすれば自分のデバイスとデータにアクセスできる。

#### Acceptance Criteria
1. When 未認証ユーザーがログインページ（`/login`）にアクセスする, the Login feature shall ゲストレイアウトで、メールアドレス・パスワード・ログイン状態保持の項目を持つログインフォームを表示する。
2. When ユーザーが正しいメールアドレスとパスワードでログインを送信する, the Login feature shall 認証セッションを確立し `/dashboard` へリダイレクトする。
3. If 入力されたメールアドレスが存在しない、または入力されたパスワードが一致しない, then the Login feature shall 「メールアドレスまたはパスワードが間違っています」という共通メッセージのみを表示してログインフォームを再表示し、どちらが誤りかを区別しない。
4. If メールアドレスが未入力もしくは形式不正、またはパスワードが未入力である, then the Login feature shall 認証処理を行わず、該当項目に日本語の入力エラーを表示してログインフォームを再表示する。
5. When ユーザーが「ログイン状態を保持する」を選択してログインに成功する, the Login feature shall 認証セッションをブラウザセッションを越えて長期間維持する。
6. While ユーザーが既に認証済みである, when ログインページにアクセスする, the Login feature shall `/dashboard` へリダイレクトする。

### Requirement 3: ユーザー登録
**Objective:** 新規の農場運営者として、アカウントを作成したい。そうすればシステムの利用を開始できる。

#### Acceptance Criteria
1. When 未認証ユーザーが登録ページ（`/register`）にアクセスする, the Register feature shall ゲストレイアウトで、ユーザー名・メールアドレス・パスワード・パスワード確認の項目を持つ登録フォームを表示する。
2. When ユーザーがすべての入力検証を満たして登録を送信する, the Register feature shall 新規ユーザーを作成し、続けて認証セッションを確立して `/dashboard` へリダイレクトする。
3. If ユーザー名が未入力、または 255 文字を超える, then the Register feature shall 該当項目に日本語の入力エラーを表示して登録フォームを再表示する。
4. If メールアドレスが未入力、または形式不正である, then the Register feature shall 該当項目に日本語の入力エラーを表示して登録フォームを再表示する。
5. If パスワードが 8 文字未満である, then the Register feature shall 該当項目に「8文字以上で入力してください」を表示して登録フォームを再表示する。
6. If パスワード確認がパスワードと一致しない, then the Register feature shall 該当項目に不一致のエラーを表示して登録フォームを再表示する。
7. If 入力されたメールアドレスが既存ユーザーと重複する, then the Register feature shall 「このメールアドレスは既に登録されています」を表示して登録フォームを再表示する。
8. The Register feature shall パスワードをハッシュ化して保存し、平文パスワードを永続化しない。
9. When 新規ユーザーを作成する, the Register feature shall メール確認フローを課さず、当該アカウントを即座に利用可能として扱う。

### Requirement 4: ログアウト
**Objective:** 認証済みの農場運営者として、ログアウトしたい。そうすれば共有端末などで自分のセッションを確実に終了できる。

#### Acceptance Criteria
1. When 認証済みユーザーがログアウトを送信する（`POST /logout`）, the Logout feature shall 認証セッションを破棄し `/login` へリダイレクトする。
2. When ログアウト後に認証必須ページへアクセスする, the Logout feature shall 当該ユーザーを未認証として扱い、再ログインを要求する。

### Requirement 5: 認証ガードとルートリダイレクト
**Objective:** システム運営者として、認証必須ページを未認証アクセスから保護したい。そうすれば他人のデータが無認証で閲覧されない。

#### Acceptance Criteria
1. If 未認証ユーザーが認証必須ページ（例: `/dashboard`）にアクセスする, then the Auth Guard shall 保護コンテンツを返さず `/login` へリダイレクトする。
2. While ユーザーが認証済みである, when 認証必須ページにアクセスする, the Auth Guard shall 当該ページの表示を許可する。
3. When 未認証ユーザーがルート（`/`）にアクセスする, the Web application shall `/login` へリダイレクトする。
4. When 認証済みユーザーがルート（`/`）にアクセスする, the Web application shall `/dashboard` へリダイレクトする。

### Requirement 6: ダッシュボード（認証後プレースホルダ）
**Objective:** 認証済みの農場運営者として、ログイン後に自分用のトップ画面へ到達したい。そうすれば認証フローが成立したことを確認できる。

#### Acceptance Criteria
1. While ユーザーが認証済みである, when ダッシュボード（`/dashboard`）にアクセスする, the Dashboard shall アプリ共通レイアウトでダッシュボードを表示し、ログイン中ユーザーの名前を表示する。
2. The Dashboard shall 後続セッションで正規実装される領域（デバイス一覧・未対応アラート）について、最小プレースホルダのみを表示する。

_Boundary:_ デバイス一覧・未対応アラートの正規実装は本セッション対象外（S3）。

### Requirement 7: 共通レイアウトと再利用 UI コンポーネント
**Objective:** 後続セッションの実装者として、ゲスト用・認証後用の共通レイアウトと再利用コンポーネントが整備されていてほしい。そうすれば各画面が一貫した外観を継承して実装できる。

#### Acceptance Criteria
1. The Guest screens (login/register) shall ヘッダー・サイドバーを持たない中央寄せのゲストレイアウトで表示される。
2. While ユーザーが認証済みである, the authenticated screens shall ヘッダー（ユーザー名・ユーザーメニュー・ログアウト操作）、サイドバー（ナビゲーション）、メインコンテンツ領域、共通通知領域を備えたアプリレイアウトで表示される。
3. The App layout shall ヘッダー・サイドバー・通知表示を再利用可能なコンポーネントとして提供し、後続画面が同一の部品を利用できるようにする。
4. When 操作の成功または失敗を利用者に伝える必要がある, the App layout shall 共通通知領域にメッセージを表示できる。

### Requirement 8: CSRF 保護（基盤）
**Objective:** システム運営者として、状態を変更するリクエストを偽造から保護したい。そうすればユーザーが意図しない操作を強要されない。

#### Acceptance Criteria
1. The Web application shall 状態を変更するリクエスト（ログイン・登録・ログアウト等の送信、および後続の更新・削除）に対して、有効な CSRF トークンを要求する。
2. If 状態変更リクエストに有効な CSRF トークンが伴わない, then the Web application shall 当該リクエストを拒否し、状態を変更しない。
3. The Guest forms (login/register) shall 送信時に CSRF トークンを含める。
4. The App layout shall 認証後画面で CSRF トークンを参照可能にし、画面内の部分更新リクエストにも当該トークンが付与されるようにする。

### Requirement 9: HTTP メソッド上書き（基盤）
**Objective:** 後続セッションの実装者として、HTML フォームから PUT/PATCH/DELETE を表現できるようにしたい。そうすれば編集・削除フォームを後続画面で実装できる。

#### Acceptance Criteria
1. When フォーム送信にメソッド上書き用の隠しフィールド（`_method`）が PUT・PATCH・DELETE のいずれかの値で含まれる, the Method Override middleware shall 当該リクエストを指定された HTTP メソッドとして処理する。
2. The Method Override middleware shall 本セッションでは基盤として提供されるに留め、ユーザー向け画面での実利用は後続セッションで行われる。

_Boundary:_ device-edit / alert-rules 等での実利用は後続セッション（S4/S7）。

### Requirement 10: 静的アセット配信
**Objective:** 農場運営者として、画面が一貫したスタイルと対話的な挙動で表示されてほしい。そうすれば各画面を快適に利用できる。

#### Acceptance Criteria
1. The Web application shall 全画面に適用される共通スタイルシートを配信する。
2. The Web application shall 画面の部分更新および軽量な UI 状態管理に必要なクライアントスクリプトを配信する。

_Boundary:_ バージョンクエリ・minify・バンドル等のアセット最適化は本セッション対象外。

### Requirement 11: セキュリティと品質に関する非機能要件
**Objective:** システム運営者として、認証機能が安全かつ日本語 UI で一貫し、自動テストで検証されていてほしい。そうすれば安心して運用・保守できる。

#### Acceptance Criteria
1. The authentication features shall パスワードをハッシュ化して保存し、平文パスワードを保持・ログ出力しない。
2. The Web UI shall ボタン・項目名・プレースホルダ・バリデーションエラーメッセージを日本語で表示する。
3. The authentication and middleware behavior shall 自動テストで検証され、テストカバレッジ 80% 以上を満たす。
4. If データベース等の内部処理が失敗する, then the Web application shall 機密情報を露出しない簡潔なエラー応答（500）を返す。
