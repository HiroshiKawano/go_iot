# 実装計画 — device-create-edit

> 前提（S1 で確立済み・本 spec では再実装しない）: 共通レイアウト `App`、認証ガード `RequireAuth`（未認証→302 `/login`）、session（`auth.UserID`）、CSRF（`middleware.CSRF`・hidden `gorilla.csrf.Token`）、MethodOverride（`_method`→PUT）、所有者認可 `authz.RequireDeviceOwner`、CSS 単一ソース配信（`view.CSSURL`/`static.go`・必要クラスは `mocks/html/style.css` に実在）。`devices` テーブル・sqlc クエリ（`CreateDevice`/`UpdateDevice`/`GetDevice`/`GetDeviceByMacAddress`）も既存。新規 `.templ` は実装時に `make templ` で生成する。
>
> 実装は番号の昇順に1行ずつ `/tdd`（RED→GREEN→REFACTOR）で進める（逐次・`(P)` なし）。

- [x] 1. Foundation: デバイスフォームの検証・変換ヘルパ
- [x] 1.1 MAC 正規化・形式検証と入力値の型変換ヘルパ
  - MAC を前後空白除去のうえ大文字へ正規化する関数、正規化後の値が形式 `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$` に一致するか判定する関数を実装
  - 設置場所の空文字を「未設定」へ、ステータス文字列 "1"/"0" を稼働状態（真偽）へ変換するヘルパを実装
  - table-driven 単体テストで、小文字/混在/前後空白の MAC が大文字 trim され、桁不足・区切り違いが不正判定されること、空 location が未設定・"1"/"0" が真偽へ変換されることを確認
  - _Requirements: 6.1, 6.2, 6.3, 2.3_
  - _Boundary: device_form.go_

- [x] 1.2 入力バインド構造と項目別日本語エラーへの変換
  - name（必須・255）・mac_address（必須）・location（255）・is_active（必須・"1"/"0" のみ）のバインド規則を定義
  - バリデーション失敗を項目名→日本語メッセージの対応へ変換する関数を、デバイス文脈の文言（例「デバイス名を入力してください」）で実装（認証フォームの文言と混同しない）
  - 単体テストで required/max/oneof 各失敗が対応する日本語になること、複数項目同時失敗が各項目分そろうことを確認
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.6, 8.4_
  - _Boundary: device_form.go_

- [x] 2. Core: 登録・編集で共有するフォーム View 層（templ）
- [x] 2.1 共有フォームコンポーネントと描画パラメータ
  - 登録/編集が共有する描画パラメータ（送信先・編集フラグ・各入力値・ステータス選択・キャンセル先・項目別エラー・CSRF トークン）を定義
  - モック device-create/edit.html を写経した共有フォームを実装。先頭に CSRF 隠しフィールド、編集時は method override 用の隠しフィールド、各入力に入力値復元（value/checked）、項目直後にエラー表示、独自CSSクラスは新設しない
  - `make templ` 生成後、描画結果に CSRF 隠しフィールド・入力値復元・項目別エラー要素が含まれることを Render 検証で確認
  - _Requirements: 1.2, 1.4, 3.1, 3.2, 3.3, 5.5, 8.1_
  - _Boundary: DeviceForm.templ, DeviceFormView_

- [x] 2.2 登録・編集ページのラッパ
  - 共通レイアウトで囲み、登録は見出し「デバイス登録」・送信先 `/devices`・キャンセル `/dashboard`、編集は見出し「デバイス編集: {名前}」・送信先 `/devices/{id}`（method override で PUT）・キャンセル `/devices/{id}` を構成
  - 共有フォームコンポーネントを呼び出すのみで、フォーム本体を複製しない
  - Render 検証で、登録ページが POST フォーム（method override 隠しフィールドなし）、編集ページが PUT 用隠しフィールド付きで描画されることを確認
  - _Requirements: 1.3, 3.3, 8.2, 8.3_
  - _Boundary: DeviceCreate.templ, DeviceEdit.templ_

- [x] 3. Core: フォーム表示ハンドラ（GET）
- [x] 3.1 デバイス登録フォーム表示
  - 認証済みユーザーのユーザー名でレイアウトを組み立て、空フォーム（ステータス初期=稼働中）を 200 で返す
  - httptest+gin と Querier モックで、GET `/devices/create` が 200・空フォーム・稼働中の選択・CSRF 隠しフィールドを返すことを確認
  - _Requirements: 1.1, 1.2, 1.3, 1.4_
  - _Boundary: DeviceHandler_

- [x] 3.2 デバイス編集フォーム表示と所有者認可
  - URL のデバイス ID を数値化し、所有者認可で対象を取得（本人所有のみ）、既存値を埋めたフォームを 200 で返す
  - 存在しない/論理削除済み/非数値 ID は 404、他ユーザー所有は 403 を返す
  - httptest+gin と Querier モックで、所有者一致時の既存値復元、ErrNoRows→404、非所有→403 を確認
  - _Requirements: 3.1, 3.2, 3.4, 7.2_
  - _Boundary: DeviceHandler, internal/authz_

- [x] 4. Core: 登録・更新ハンドラ（POST/PUT）
- [x] 4.1 デバイス登録の実行
  - バインド→項目検証→MAC 正規化・形式検証→MAC 一意検査（削除以外の全デバイス対象）→ログイン中ユーザーを所有者として作成→303 で詳細へリダイレクト
  - 検証失敗（各項目・MAC 形式・MAC 重複）は作成せず 200 で入力値復元付き再描画、内部エラーは 500
  - httptest+gin と Querier モックで、正常時に正規化 MAC・所有者=ログインユーザー・空 location が未設定で作成され 303 になること、各失敗が 200 で該当項目エラーになることを確認
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 5.5, 6.4, 7.3_
  - _Boundary: DeviceHandler_

- [x] 4.2 デバイス更新の実行
  - 所有者認可→バインド→項目検証→MAC 正規化・形式→MAC 一意検査（自分自身を除外、自身の現在値は許可）→更新→303 で詳細へリダイレクト
  - 存在しない/論理削除済みは 404、検証失敗は 200 で再描画、内部エラーは 500
  - httptest+gin と Querier モックで、method override 経由の PUT が正常時 303、他デバイスと MAC 重複時 200 エラー、自身の MAC 据置は許可、不在時 404 を確認
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 6.5, 6.6, 7.2_
  - _Boundary: DeviceHandler, internal/authz_

- [x] 5. Integration: ルート配線と通し検証（main.go）
  - 合成ルートで DeviceHandler を生成し、Web グループ（session+CSRF）に各 RequireAuth 前置で 4 ルート（GET `/devices/create`・POST `/devices`・GET `/devices/{device}/edit`・PUT `/devices/{device}`）を登録
  - 静的経路 `/devices/create` とパラメータ経路 `/devices/{device}` が共存してルータが panic しないことを起動で確認
  - ミドルウェア通し（httptest）で、未認証アクセスが 302 `/login`、CSRF トークンを GET で取得し POST/PUT へ往復すると通過、トークン欠落で 403、編集フォームの隠し `_method` で PUT ルートへ解決されることを確認
  - _Requirements: 7.1_
  - _Depends: 3.1, 3.2, 4.1, 4.2_
  - _Boundary: cmd/server/main.go_

- [x] 6. Validation: カバレッジとエッジケースの通し確認
  - 全テスト通過と、デバイス登録/編集のハンドラ・フォーム・ヘルパのカバレッジ80%以上を確認
  - 大文字小文字違いの MAC（例 `aa:..` と `AA:..`）が同一とみなされ重複登録が防止されること、複数項目同時エラーが各項目に表示されることを通し確認
  - 既存の認証・ダッシュボードのテストが引き続き通過する（回帰なし）ことを確認
  - _Requirements: 5.6, 6.3, 8.1_
