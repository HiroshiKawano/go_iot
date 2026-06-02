# セッション4 spec-init プロンプト: デバイス登録・編集

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: device-create-edit
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1（基盤・認証・App レイアウト・MethodOverride・バリデーション表示方針）
> 設計フェーズで参照: 画面設計書(静的).md 行349-404・582-590 / HTMX実装ガイド(動的).md §3.4（行1281-1289）・§4 device-create（行1472-1488）・§4 device-edit（行1491-1505）・§7 バリデーション（行1633-1706）/ DB設計書.md「devices テーブル」（行364-388）/ mocks/html/device-create.html・device-edit.html

--- spec-init 本文 ここから ---

## 機能概要
農場運営者がデバイス（ESP32 温湿度センサー）をシステムに登録・編集する。現状、デバイス登録・編集画面の UI 層（templ・HTMX・入力値復元・バリデーション表示）は全面未着手。本セッションで、共有テンプレートコンポーネント `DeviceForm.templ` を軸に、新規登録と編集を同一フォーム構造で実装し、データベース保存・リダイレクト・エラー表示を完成させる。

## 背景・現状
バックエンド（sqlc 全34クエリ・CreateDevice・UpdateDevice・GetDevice・GetDeviceByMacAddress 等）は実装完了。Web UI 層は未着手。S1 で App レイアウト・MethodOverride ミドルウェア・バリデーション共通方針が確立。本セッションはそれを踏まえ、デバイス登録・編集フォーム専用のハンドラ・テンプレート・バリデーションロジックを構築する。

## このセッションのスコープ（実装対象）

### ルート・ハンドラ
- **GET /devices/create** → `DeviceHandler.ShowCreateForm()` → templ `page.DeviceCreate()` を返す。フォームは空、`is_active` 初期値=1（稼働中）。
- **POST /devices** → `DeviceHandler.Create()` → バリデーション → sqlc `CreateDevice(user_id, name, mac_address, location, is_active)` → 成功時 `c.Redirect(http.StatusSeeOther, "/devices/{device}")` / 失敗時 templ `page.DeviceCreate(form, errors)` を再描画。
- **GET /devices/{device}/edit** → `DeviceHandler.ShowEditForm()` → sqlc `GetDevice(id)` で既存値取得 → templ `page.DeviceEdit(device)` を返す。
- **PUT /devices/{device}** (HTML フォームから POST で受け、hidden `_method=PUT` で判定、S1 の MethodOverride ミドルウェア経由) → `DeviceHandler.Update()` → バリデーション（MAC 一意は自分自身を除外） → sqlc `UpdateDevice(id, name, mac_address, location, is_active)` → 成功時 `c.Redirect(http.StatusSeeOther, "/devices/{device}")` / 失敗時 templ `page.DeviceEdit(device, form, errors)` を再描画。

### フォーム項目・バリデーション
| 項目 | name | type | validator タグ | 制約 | 説明 |
|------|------|------|-------------|-----|------|
| デバイス名 | `name` | text | `binding:"required,max=255"` | 必須、最大255文字 | — |
| MACアドレス | `mac_address` | text | `binding:"required"` + custom validator | 必須、形式 `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$` | 登録時：全稼働中デバイスで一意（sqlc `GetDeviceByMacAddress` → `is_active=1 AND deleted_at IS NULL` で重複検査）。更新時：自分自身を除外。 |
| 設置場所 | `location` | text | `binding:"max=255"` | 任意、最大255文字 | — |
| ステータス | `is_active` | radio（値 "1" / "0"） | `binding:"required,oneof=1 0"` | 必須、初期値=1（稼働中） | "1" = 稼働中 / "0" = 停止中 |

バリデーションエラーは `map[string]string` にして templ へ明示的に引数で渡す（S1 で決められた方針、共有バッグは無し）。

### templ コンポーネント構成
- **`internal/view/page/DeviceCreate.templ`** — 登録ページ（タイトル "デバイス登録"）。`DeviceForm` を呼び出し、`action="/devices"` / `method="POST"` / `device=nil` / `errors=map` を渡す。
- **`internal/view/page/DeviceEdit.templ`** — 編集ページ（タイトル "デバイス編集: {device.name}"）。`DeviceForm` を呼び出し、`action="/devices/{device.id}"` / `method="PUT"`（hidden `_method` 経由） / `device={device}` / `errors=map` を渡す。
- **`internal/view/component/DeviceForm.templ`** — 登録・編集共有フォーム。引数：`action` (文字列)、`method` (文字列、"POST" or "PUT")、`device` (*repository.Device or nil)、`errors` (map[string]string)。フォーム `.form-group` の各項目に `value` / `checked` を条件付きで埋め込み（device nil なら空、device あれば既存値）。エラーがあれば `.error-message` 要素に項目別メッセージ表示。HTMX id: `device-form`（ターゲット、フルページ POST なので不使用だが R27 準拠で付与）。

### ハンドラ実装サイズ・構成
- **`internal/handler/device.go`** — `DeviceHandler` struct（DI で repository・logger を受け持つ）。メソッド：`ShowCreateForm()`・`Create()`・`ShowEditForm()`・`Update()`。400行前後。
- **`internal/handler/device_form.go`** (共通) — `CreateUpdateDeviceRequest` struct（binding タグ）と `toFieldErrors()` ヘルパー（validator.ValidationErrors → map[string]string に変換、S1 で確立）。
- MAC 形式検証・MAC 一意制約チェックのロジック：validator カスタムルール or Handler 内で `regexp.Compile()` + `GetDeviceByMacAddress()` で実行。

### キャンセル動作
- 登録画面：`<a href="/dashboard" class="btn btn-secondary">` （ダッシュボードに戻る）
- 編集画面：`<a href="/devices/{device}" class="btn btn-secondary">` （デバイス詳細に戻る）

### 使用 sqlc クエリ
- `CreateDevice(user_id, name, mac_address, location, is_active)` — デバイス新規作成、RETURNING *
- `UpdateDevice(id, name, mac_address, location, is_active)` — デバイス更新、RETURNING *
- `GetDevice(id)` — デバイス詳細取得（編集画面の既存値ロード）
- `GetDeviceByMacAddress(mac_address)` — MAC 一意制約チェック

### 新規作成ファイル・ディレクトリ
```
internal/
  handler/
    device.go                  （DeviceHandler + ShowCreateForm/Create/ShowEditForm/Update）
    device_form.go             （CreateUpdateDeviceRequest + toFieldErrors）
  view/
    page/
      DeviceCreate.templ       （登録ページ）
      DeviceEdit.templ         （編集ページ）
    component/
      DeviceForm.templ         （共有フォーム、R27 準拠）
```

## スコープ外（このセッションでやらないこと）
- デバイス詳細表示・グラフ表示（S5 device-detail）
- デバイス一覧・ダッシュボード（S3 dashboard）
- デバイス削除（S5 device-detail）
- scs Session の実装仕様決定（S1 で完了を想定）
- MethodOverride ミドルウェア・CSRF 対応の実装詳細（S1 で完了を想定）
- アラートルール関連（S7 alert-rules）

## 技術制約・準拠事項
- **Gin フレームワーク**: `c.ShouldBind()` で form データをバインド、`binding` タグで検証（`github.com/go-playground/validator/v10` 内蔵）
- **templ**: コンポーネント関数（`DeviceForm.templ`）は `templ.Component` インターフェース。page（`DeviceCreate.templ`・`DeviceEdit.templ`）は `internal/view/layout/App.templ` レイアウトで囲む（S1 準拠）。
- **sqlc**: 返却型は `*repository.Device`。nullable 値は `pgtype.Timestamptz` など型で表現。
- **scs**: Session の実装方式は S1 で決定済みを前提。本セッション中は `*http.Request` 内の user_id 取得を実装（session ミドルウェア経由）。
- **入力値復元**: フォーム再描画時、form struct の値を templ 引数として渡し、`value` / `checked` で HTML に埋め込む（TL10・ HTMLモック作成ルール.md）。
- **エラーメッセージ**: `map[string]string`（項目名 → メッセージ）を templ へ明示的に引数で渡す（共有バッグ無し）。項目ごとのエラーは `.error-message` 要素にインライン表示。§7（HTMX実装ガイド行1633-1706）準拠。
- **バリデーションタグ**: Go の binding タグを正確に記述。`required`・`max`・`oneof` など。
- **日本語**: エラーメッセージ・コメント・コミットは日本語。コード識別子のみ英語可。
- **TDD**: テストカバレッジ80%以上。ハンドラ・バリデーション・フォーム復元・リダイレクト・sqlc クエリ呼び出しの各ユースケース。

## 受け入れ基準（概略）
1. **ルート動作**: GET /devices/create → 空フォーム表示 / POST /devices（成功）→ /devices/{id} へ 303 リダイレクト。
2. **バリデーション**: name（required, max=255）・mac_address（required, 形式, 一意）・location（max=255）・is_active（required, oneof）が正確に検証される。
3. **入力値復元**: 登録・編集画面でバリデーションエラーが発生した場合、入力値が templ フォームに復元されて再描画される。
4. **編集画面**: GET /devices/{device}/edit で既存値が `value` / `checked` で埋め込まれて表示される。
5. **PUT メソッド対応**: 編集フォーム送信時、hidden `_method=PUT` + MethodOverride ミドルウェア（S1）でルートが正確に PUT として処理される。
6. **MAC 一意制約**: 登録時は全稼働中デバイスで MAC が重複していないか確認。更新時は自分自身を除いて確認。重複時は `mac_address` 項目のエラーが表示される。
7. **キャンセル**: 登録 → /dashboard、編集 → /devices/{device} へ遷移する。
8. **テスト**: ハンドラ・バリデーション・テンプレート実装が TDD で80%以上カバレッジ。

## 未確定事項・要確認（あれば）
- **MethodOverride ミドルウェア**: S1 で採用ライブラリ・実装方式（hidden `_method` 値の大文字小文字、HTML フォーム form method 属性の形式）を確認。
- **CSRF トークン生成・検証**: S1 で採用ライブラリ・meta / header / form hidden のどれを使うか確認。
- **scs セッションストア実装**: S1 で PostgreSQL ストア設定が完了していることを前提。本セッション中はセッションから user_id を取得するだけ。

--- spec-init 本文 ここまで ---
