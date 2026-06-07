# 実装計画 — web-foundation-auth (S1)

> 逐次実行（上から1行ずつ `/tdd` で RED→GREEN→REFACTOR）。各 executable サブタスクは1 TDD サイクルで完了する単一責務・観測可能成果を持つ。
> 生成依存順を遵守: goose migration → `make db-snapshot` → templ generate（各 templ タスク内包）→ 生成物を消費する実装。新規 sqlc クエリは無し（既存 `GetUserByEmail`/`CreateUser`/`GetUser` で充足）。

- [x] 1. 基盤: 依存・スキーマ・アセット配信
- [x] 1.1 セッション・CSRF 依存の追加
  - go get で `alexedwards/scs/v2`・`alexedwards/scs/pgxstore`・`gorilla/csrf` を取得
  - 観測可能完了: go.mod/go.sum に3依存が記録され、既存コードの `go build ./...` が通る（`go mod tidy` は利用実装 2.1/2.3 後に実行し未使用削除を回避）
  - _Requirements: 1.1, 8.1_
- [x] 1.2 scs セッションテーブルの migration 追加と反映
  - `db/migrations` に sessions(token TEXT PK, data BYTEA, expiry TIMESTAMPTZ) + expiry index を goose で作成し migrate-up、`make db-snapshot` で反映
  - 観測可能完了: migrate-up 後に sessions テーブルが存在し `docs/database_snapshot/table_definitions.md` に sessions が出現
  - _Requirements: 1.1_
- [x] 1.3 静的アセット配信基盤（単一 Foundation タスク）
  - `make sync-css` で正本 `mocks/html/style.css` を `internal/view/public/css/` へ複製、`//go:embed all:public` と `StaticFS("/static")` マウント、`CSSURL()` バージョンヘルパー。htmx/alpine は CDN(SRI)
  - 観測可能完了: 起動時 `/static/css/style.css?v=...` が 200 で CSS を返し、`CSSURL()` がバージョン付き URL を返す
  - _Requirements: 10.1, 10.2_

- [x] 2. セッション認証とミドルウェア
- [x] 2.1 セッションマネージャと認証ヘルパー
  - scs を pgxstore で構築（HttpOnly/SameSite=Lax/Secure=prod/Persist=false）。ログイン（RenewToken→user_id 保存→remember 時 RememberMe）・ログアウト（Destroy）・取得ヘルパー
  - 観測可能完了: in-memory store のテストで login 後 user_id を取得でき、logout 後は 0、login で token が更新される
  - _Requirements: 1.1, 1.2, 1.3, 1.5, 2.5, 4.1_
- [x] 2.2 メソッド上書きミドルウェア（基盤）
  - http.Handler 層で POST かつ `_method∈{PUT,PATCH,DELETE}` のとき `r.Method` を書き換え（engine 外側＝ルーティング前）
  - 観測可能完了: POST+_method=DELETE が DELETE として処理され、通常 POST・GET は `r.Method` 不変、というテストが緑
  - _Requirements: 9.1, 9.2_
- [x] 2.3 CSRF ミドルウェア（web グループ限定）
  - gorilla/csrf を gin アダプタ化、authKey=sha256(SESSION_SECRET) で32バイト導出、既定ヘッダ X-CSRF-Token。ミドルウェア関数として提供（実配線は 5.1）
  - 観測可能完了: 任意長 secret から32バイト鍵を導出。使い捨てルータに適用したユニットテストでトークン無し状態変更→403・有り→通過
  - _Requirements: 1.3, 1.4, 8.1, 8.2_
- [x] 2.4 セッション橋渡しと認証ガード
  - SessionLoad（scs の user_id を `auth.SetUserID` で gin context へ）・RequireAuth（`auth.UserID<=0` で /login へ 302+Abort）
  - 観測可能完了: 未認証で保護ルート→302 /login、認証済→通過 のテストが緑
  - _Requirements: 1.2, 5.1, 5.2_

- [x] 3. 共通レイアウトと画面（templ）
  - ※各サブタスクは templ generate で `*_templ.go` を生成しコンパイル可能にする
- [x] 3.1 Guest/App レイアウト
  - Guest（中央カード・ヘッダ/サイドバーなし）、App（header+sidebar+main#main-content+#flash-message+csrf meta+htmx:configRequest+CSSURL link）。共通 `AppLayoutData`
  - 観測可能完了: templ generate 成功、App 出力 HTML が `#main-content`・`#flash-message`・`meta[name=csrf-token]` を含む
  - _Requirements: 7.1, 7.2, 8.4, 10.1, 10.2_
- [x] 3.2 再利用コンポーネント
  - SiteHeader（ロゴ+ユーザー名+Alpine メニュー+ログアウトフォーム）・Sidebar（固定ナビ）・FlashMessage（#flash-message）
  - 観測可能完了: SiteHeader 出力にユーザー名と /logout フォーム、FlashMessage が `#flash-message` を描画
  - _Requirements: 7.3, 7.4_
- [x] 3.3 ログイン/登録ページ
  - mocks の login.html/register.html を写経。各 `.error-message` にエラー表示、入力値再表示、hidden csrf。独自クラス新設禁止
  - 観測可能完了: errs 指定時に該当 `.error-message` に日本語が描画され、送信済み入力値が再表示され、各フォームに hidden CSRF input（gorilla.csrf.Token）が描画される
  - _Requirements: 2.1, 2.4, 3.1, 3.3, 3.4, 3.5, 3.6, 3.7, 8.3, 11.2_
- [x] 3.4 ダッシュボードページ
  - dashboard.html 写経。App 内にユーザー名表示と、デバイス一覧/未対応アラートの空状態プレースホルダ（実装は S3）
  - 観測可能完了: App レイアウト内にログイン中ユーザー名とプレースホルダが描画される
  - _Requirements: 6.1, 6.2_

- [x] 4. 認証ハンドラ
- [x] 4.1 バリデーション日本語変換
  - go-playground/validator の FieldError を field+tag で日本語へ（email/min/eqfield/max/required）。map[string]string で返す
  - 観測可能完了: 各 field+tag→期待日本語メッセージのテーブルテストが緑
  - _Requirements: 2.4, 3.3, 3.4, 3.5, 3.6, 11.2_
  - _Boundary: AuthHandler_
- [x] 4.2 ログインハンドラ
  - GET /login（認証済は /dashboard）・POST /login（ShouldBind→GetUserByEmail→bcrypt 照合→RenewToken+user_id 保存+remember）。失敗（不在/不一致）は共通メッセージで 200 再描画、バリデーション失敗も 200
  - 観測可能完了: 正資格で 303 /dashboard かつ session に user_id、失敗で 200+共通メッセージ（列挙防止）
  - _Requirements: 2.1, 2.2, 2.3, 2.5, 2.6, 11.1_
- [x] 4.3 登録ハンドラ
  - GET /register・POST /register（ShouldBind→GetUserByEmail で重複→bcrypt 生成→CreateUser→自動ログイン→303）。email 重複/検証失敗は 200 再描画。DB 失敗は 500
  - 観測可能完了: 妥当入力で CreateUser 呼出→自動ログイン→303 /dashboard、重複で 200+「このメールアドレスは既に登録されています」、email_verified_at は NULL
  - _Requirements: 3.1, 3.2, 3.7, 3.8, 3.9, 11.1, 11.4_
- [x] 4.4 ログアウト/ダッシュボード/ルートハンドラ
  - POST /logout（Destroy→303 /login）・GET /dashboard（RequireAuth・GetUser でユーザー名）・GET /（認証で /dashboard、未認証で /login へ 302）
  - 観測可能完了: ログアウト後 /dashboard が 302 /login、認証済 /dashboard でユーザー名表示、/ が認証状態で振り分け
  - _Requirements: 4.1, 4.2, 5.1, 5.3, 5.4, 6.1_

- [x] 5. 合成ルート統合
- [x] 5.1 main.go 配線と起動方式変更（統合タスク）
  - SessionManager 構築、Web UI グループ（SessionLoad+CSRF）に login/register/logout/dashboard(RequireAuth)/ルート登録、MountStatic、http.Handler 合成 `MethodOverride(LoadAndSave(engine))`、`r.Run`→`http.Server` へ変更。既存 /api は維持
  - 観測可能完了: 起動後 /login GET=200、/dashboard 未認証=302、/static CSS=200、/api/sensor-data（既存）が従来通り動作
  - _Boundary: cmd/server/main.go_
  - _Depends: 1.3, 2.1, 2.2, 2.3, 2.4, 4.4_
  - _Requirements: 5.1, 5.3, 5.4, 8.1, 9.2, 10.1_

- [x] 6. 検証
- [x] 6.1 認証フロー統合テスト
  - httptest + `repository.Querier` モック + scs in-memory で login（成功/不在/不一致/形式不正）・register（成功/重複/8字未満/不一致）・logout・guard・CSRF（無/有）・/ 振り分けを網羅。templ 描画（#main-content/#flash-message/.error-message）もアサート
  - 観測可能完了: 上記フローの go test が全て緑
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.2, 3.5, 3.6, 3.7, 4.1, 4.2, 5.1, 5.2, 6.1, 8.1, 8.2, 11.3, 11.4_
- [x] 6.2 カバレッジ確認
  - `go test ./... -cover` で auth/middleware/handler が 80% 以上
  - 観測可能完了: 対象パッケージのカバレッジが 80% 以上
  - _Requirements: 11.3_

---

## 要件カバレッジ（全44 ID）

| Req | タスク | Req | タスク |
|---|---|---|---|
| 1.1 | 1.1, 1.2, 2.1 | 5.1 | 4.4, 5.1, 6.1 |
| 1.2 | 2.1, 2.4 | 5.2 | 2.4, 6.1 |
| 1.3 | 2.1, 2.3 | 5.3 | 4.4, 5.1 |
| 1.4 | 2.3 | 5.4 | 4.4, 5.1 |
| 1.5 | 2.1 | 6.1 | 3.4, 4.4 |
| 2.1 | 3.3, 4.2, 6.1 | 6.2 | 3.4 |
| 2.2 | 4.2, 6.1 | 7.1 | 3.1 |
| 2.3 | 4.2, 6.1 | 7.2 | 3.1 |
| 2.4 | 3.3, 4.1, 6.1 | 7.3 | 3.2 |
| 2.5 | 2.1, 4.2 | 7.4 | 3.2 |
| 2.6 | 4.2 | 8.1 | 1.1, 2.3, 5.1, 6.1 |
| 3.1 | 3.3, 4.3 | 8.2 | 2.3, 6.1 |
| 3.2 | 4.3, 6.1 | 8.3 | 3.3 |
| 3.3 | 3.3, 4.1 | 8.4 | 3.1 |
| 3.4 | 3.3, 4.1 | 9.1 | 2.2 |
| 3.5 | 3.3, 4.1, 6.1 | 9.2 | 2.2, 5.1 |
| 3.6 | 3.3, 4.1, 6.1 | 10.1 | 1.3, 3.1, 5.1 |
| 3.7 | 3.3, 4.3, 6.1 | 10.2 | 1.3, 3.1 |
| 3.8 | 4.3 | 11.1 | 4.2, 4.3 |
| 3.9 | 4.3 | 11.2 | 3.3, 4.1 |
| 4.1 | 4.4 | 11.3 | 6.1, 6.2 |
| 4.2 | 4.4 | 11.4 | 4.3, 6.1 |
