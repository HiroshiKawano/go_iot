# セッション7 spec-init プロンプト: アラートルール管理（インラインCRUD）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: alert-rules
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: S1（基盤）。判定の実行はS2だがルールのCRUDは独立
> 設計フェーズで参照: 画面設計書(静的).md §7（行467-521）・§フォーム復元方針（行594-602）、HTMX実装ガイド(動的).md §3.6（行1304-1316）・§4 alert-rules（行1529-1562）・§7 バリデーション（行1650-1704）・§8 CSRF（行1900-1929）・§11 削除確認（行2351-2371）・§12 インラインCRUD（行2373-2452）・§16 Tom Select（行2905-2945）、DB設計書.md alert_rules（行450-480）・Metric Enum（行168-210）・ComparisonOperator Enum（行221-289）、mocks/html/alert-rules.html、other/templ実装仕様書.md

--- spec-init 本文 ここから ---

## 機能概要

農場運営者はデバイスごとにアラートルールを設定して、センサー異常値を自動検知できるようにしたい。現状ではバックエンド（DB・sqlcクエリ）は完成しているが、ルール管理画面（追加・編集・削除・有効/無効切替）がない。本セッションでは、GET /alerts/rules 画面でインラインHTMXを活用し、フォーム・一覧・ルールCRUDをすべてサーバサイドtempl + sqlc で実装する。

## 背景・現状

### 既実装（S1基盤で用意済み）
- PostgreSQL テーブル alert_rules（id/device_id/metric/operator/threshold/is_enabled/created_at/updated_at/deleted_at）、全34 sqlc クエリ
- domain.Metric（温度/湿度）と domain.ComparisonOperator（>/</>=/<= を記号で DB 値として格納）、Label()/Unit() メソッド完成
- scs セッションミドルウェア導入予定（session_auth.go 未作成）
- CSRF ミドルウェア採用方針（ライブラリ未確定）、MethodOverride ミドルウェア採用方針（方式未確定）

### このセッション前の状態
- Web UI 層（templ ファイル、HTMX属性、scs セッション）全0件
- internal/{auth/session_auth.go, middleware/, view/} は空ディレクトリ
- mocks/html/alert-rules.html は静的モック完成済み

### このセッションの位置づけ
- アラートルール表示・CRUD の最初の Web UI セッション
- 判定実行（S2）と履歴表示（S8）には依存しない独立スコープ
- テーブル内インラインフォーム型CRUDパターンをS2・S8へ展開するプロトタイプ

## このセッションのスコープ（実装対象）

### ルート・エンドポイント

- **GET /alerts/rules?device_id={N}**
  → ハンドラ役割: デバイス一覧を SelectOption に、指定デバイスのルール一覧を取得・描画
  → 返却: page.AlertRules（共通レイアウト App.templ 内に全体を埋め込み）
  → テンプレート: `internal/view/page/AlertRules.templ`
  → 使用 sqlc: ListDevicesByUser, ListAlertRulesByDevice

- **POST /alerts/rules**
  → ハンドラ役割: フォーム値のバリデーション→新規ルール INSERT→成功時は更新後の一覧 Fragment + 空フォーム返却
  → フォーム項目: device_id（hidden）, metric（select, required oneof=temperature humidity）, operator（select, required oneof=> < >= <=）, threshold（number step=0.01, required）
  → 返却: component.AlertRuleSection（追加フォーム空状態 + 更新後の一覧）、422 時はフォームコンポーネントにエラー埋め込み
  → テンプレート: `internal/view/component/AlertRuleSection.templ`（呼び出し時に component.AlertRuleForm（nil）+ component.AlertRuleList を内包）
  → 使用 sqlc: CreateAlertRule, ListAlertRulesByDevice

- **GET /alerts/rules/{rule}/edit**
  → ハンドラ役割: 指定ルール ID の既存値を読み込み、フォームに埋め込んで差し替え
  → 返却: component.AlertRuleForm（editingRule=既存ルール, 送信先 PUT /alerts/rules/{rule}, submit ボタン文言「更新」）
  → テンプレート: `internal/view/component/AlertRuleForm.templ`（引数 editingRule が nil なら追加、指定値なら編集）
  → 使用 sqlc: GetAlertRule

- **PUT /alerts/rules/{rule}**
  → ハンドラ役割: フォーム値のバリデーション→既存ルール UPDATE→成功時は更新後の一覧 Fragment + 空フォーム返却
  → フォーム項目: metric, operator, threshold（POST と同じ）
  → 返却: component.AlertRuleSection（空フォーム + 更新後の一覧）、422 時はフォームコンポーネントにエラー埋め込み
  → テンプレート: `internal/view/component/AlertRuleSection.templ`
  → 使用 sqlc: UpdateAlertRule, ListAlertRulesByDevice

- **PATCH /alerts/rules/{rule}/toggle**
  → ハンドラ役割: is_enabled を反転、当該行 is_enabled/表示テキストを再描画
  → 返却: component.AlertRuleRow（差し替え対象 id=alert-rule-row-{rule}、outerHTML）
  → テンプレート: `internal/view/component/AlertRuleRow.templ`
  → 使用 sqlc: ToggleAlertRule, GetAlertRule

- **DELETE /alerts/rules/{rule}**
  → ハンドラ役割: 論理削除 SoftDeleteAlertRule、残りルール一覧を再描画
  → 返却: component.AlertRuleList（差し替え対象 id=alert-rule-list、innerHTML）
  → テンプレート: `internal/view/component/AlertRuleList.templ`
  → 使用 sqlc: SoftDeleteAlertRule, ListAlertRulesByDevice

### フォーム・バリデーション

| 項目 | name | type | Validator タグ（Gin binding） | 説明 |
|------|------|------|-------|------|
| 指標 | metric | select | `required,oneof=temperature humidity` | 温度/湿度。DB 値で指定 |
| 条件 | operator | select | `required,oneof=> < >= <=` | より大きい/より小さい/以上/以下。DB 格納値（記号） |
| 閾値 | threshold | number step=0.01 | `required` | 小数第2位まで。pgtype.Numeric で DB 型対応 |

エラーレスポンス（422）: templ は errors map[string]string（項目→メッセージ）を受け取り、各フォーム項目の下に .error-message で描画。

### HTMX 操作仕様

| 操作 | トリガー | リクエスト | ターゲットid | swap | レスポンス |
|------|---------|----------|------------|------|-----------|
| デバイス切替 | デバイス選択 change | GET /alerts/rules?device_id={N} | alert-rule-section | innerHTML | 対象デバイスの追加フォーム + ルール一覧 Fragment |
| ルール追加 | フォーム submit | POST /alerts/rules | alert-rule-section | innerHTML | 一覧へ追加・空フォームに戻す Fragment |
| 編集フォーム読込 | [編集]クリック | GET /alerts/rules/{rule}/edit | alert-rule-form | outerHTML | 既存値埋め込み・送信先PUT・ボタン文言「更新」 |
| ルール更新 | フォーム submit | PUT /alerts/rules/{rule} | alert-rule-section | innerHTML | 一覧へ反映・空フォームに戻す Fragment |
| 有効/無効切替 | チェックボックス change | PATCH /alerts/rules/{rule}/toggle | alert-rule-row-{rule} | outerHTML | 当該行のみ is_enabled 反転・表示更新 |
| ルール削除 | [削除]クリック（確認後） | DELETE /alerts/rules/{rule} | alert-rule-list | innerHTML | 削除行除いた一覧 Fragment |

### DOM id（templ で付与）

| id | 要素 | 用途 |
|:---|:-----|:-----|
| alert-rule-section | フォーム + リスト親コンテナ | デバイス切替・追加・更新時に全体差し替え |
| alert-rule-form | form.rule-form | [編集]で既存値入りに outerHTML 差し替え |
| alert-rule-list | table tbody | 削除後に rows を差し替え |
| alert-rule-row-{rule} | tr（各ルール） | 有効切替で当該行のみ outerHTML 差し替え。{rule} は rule.ID |

### 新規作成ファイル・コンポーネント

#### templ コンポーネント
- `internal/view/page/AlertRules.templ` — フルページ（デバイス選択form + alert-rule-section）
- `internal/view/component/AlertRuleSection.templ` — alert-rule-section 領域（フォーム + リスト）
- `internal/view/component/AlertRuleForm.templ` — フォーム（引数 editingRule、nil なら追加）
- `internal/view/component/AlertRuleList.templ` — ルール一覧テーブル tbody wrapper
- `internal/view/component/AlertRuleRow.templ` — ルール1行（tr）

#### ミドルウェア
- `internal/middleware/method_override.go` — DELETE/PATCH を POST の _method hidden で受ける（採用方式は要確認）
- 概要: templ では PUT/PATCH/DELETE を直接送信できないため、HTML フォーム側で hidden _method + MethodOverride ミドルウェアで対応

#### ハンドラ（既存 handler/ にファイル追加、または新規ファイル）
- alert_rule_handler.go に GetList / Add / GetEdit / Update / Toggle / Delete メソッド群

### DB 操作・sqlc クエリ

使用 sqlc クエリ名:
- `ListDevicesByUser(ctx, userID)` — デバイス選択肢取得（フロントページ初期表示）
- `ListAlertRulesByDevice(ctx, deviceID)` — 指定デバイスのルール一覧（ページ表示・追加後・削除後）
- `GetAlertRule(ctx, ruleID)` — 指定ルール取得（編集フォーム値復元）
- `CreateAlertRule(ctx, params)` — 新規ルール INSERT
- `UpdateAlertRule(ctx, ruleID, params)` — ルール UPDATE
- `ToggleAlertRule(ctx, ruleID)` — is_enabled 反転（PATCH）
- `SoftDeleteAlertRule(ctx, ruleID)` — 論理削除（deleted_at を UPDATE）

### デバイス選択（Tom Select 適用）

- DOM: デバイス選択 `<select name="device_id" class="js-tom-select">` をラッパーDiv `#device-select-wrapper` で囲む
- HTMX属性: `hx-get="/alerts/rules?device_id={N}"` + `hx-target="#alert-rule-section"` + `hx-trigger="change"`
- レスポンス: Tom Select の swap 後 再初期化（TS-1 ラッパーDiv方式により select タグ全体を返却）

### 削除確認方針（§11）

- 削除ボタンに `hx-confirm="このルールを削除しますか?"` 属性を付与
- 確認「はい」時のみ DELETE リクエスト送信
- JavaScript による複雑なモーダルは使わない

### フォーム項目の表示・ラベル

- metric / operator の表示: domain.Metric.Label() / domain.ComparisonOperator.Label() で「温度」「より大きい」等に変換
- threshold 単位: domain.Metric.Unit() で「℃」「%」を付与
- 一覧が0件時: .empty-message で「このデバイスにはアラートルールが設定されていません」表示

### CSRF 対応（§8）

- 採用: Gin CSRF ミドルウェア（ライブラリは要確認）+ htmx:configRequest グローバルハンドラ
- App.templ <head> に `<meta name="csrf-token" content={csrfToken}>` を埋め込み
- <body> 末尾に htmx:configRequest で X-CSRF-Token ヘッダーを自動付与する JS
- 通常フォーム（非HTMX）では HTML hidden input で個別にトークン送信（device-create/edit等で例示）

### エラーハンドリング

- バリデーションエラー（422）: フォームコンポーネントを component.AlertRuleForm(editingRule, form, errs) で再描画、各項目に .error-message を表示
- DB エラー・不正 ID（400/404/500）: 簡潔なテキストレスポンス（例: `c.String(http.StatusInternalServerError, "ルール更新に失敗しました")` ）
- エラーハンドリングは § のパターン §7 に従う

## スコープ外（このセッションでやらないこと）

- **アラート判定の実行（S2）**: ComparisonOperator.Evaluate() でルール判定→alert_histories 記録は別セッション
- **アラート履歴表示（S8）**: alert-history.html の画面実装は別セッション
- **デバイス登録・編集（S4）／デバイス詳細（S5）**: device-create/edit・device-show の画面実装は別セッション
- **セッション認証層完成化（S1+）**: session_auth.go の CSRF・セッション設定は基盤セッション（完全な実装は後で拡張可）
- **ダッシュボード（S3）**: dashboard.html の画面実装は別セッション

## 技術制約・準拠事項

### スタック（変更なし）
- **言語**: Go 1.26 + Gin v1.12
- **DB**: PostgreSQL 16 + pgx/v5 + sqlc v1.30 + goose
- **テンプレート**: templ v0.3（a-h/templ）。Blade ではない
- **フロント動的化**: HTMX + Alpine.js（このセッションは Alpine なし）
- **バリデーション**: go-playground/validator v10 + Gin binding タグ
- **セッション**: alexedwards/scs/v2（未導入。このセッションで基本導入）

### 準拠ドキュメント
- 画面設計書(静的).md §7 アラートルール管理（レイアウト・フォーム項目・初期値・リダイレクト）
- 画面設計書(静的).md §フォーム復元方針（編集フォームへの既存値ロード）
- HTMX実装ガイド(動的).md §3.6 id一覧・§4 alert-rules 操作表・§7 バリデーション・§8 CSRF・§11 削除確認・§12 インラインCRUD・§16 Tom Select
- DB設計書.md §alert_rules テーブル・Metric/ComparisonOperator Enum
- HTMLモック作成ルール.md R01〜R27（CSS クラス・構造）
- other/templ実装仕様書.md（templ の基本記法・セキュリティ）

### 日本語・コード言語の使い分け
- 応答メッセージ・コメント・ボタンラベル・エラーメッセージ: **日本語**
- 構造体フィールド・関数名・変数名・url・id・name 属性: **英語（kebab-case / snake_case）**
- validator タグの oneof 値（metric/operator 値）: **英語（DB値に合わせて temperature/humidity/>/< 等）**

### テスト（TDD 80%以上カバレッジ）
- Handler 単体テスト: バリデーションエラー・成功系・DB エラーの 3 パターン以上
- Repository（sqlc）: 既実装なので状態確認のみ
- Service層（あれば）: アラート判定は S2 なので未対象
- 統合テスト: 所有デバイスのみアクセス可能（他ユーザーのデバイスは 403）

### イミュータブル方針
- スライス追加: `append(append([]T{}, old...), new...)` で新規スライス生成（既存スライスを変更しない）
- map 更新: 新規 map を作成（既存値の破壊的変更は避ける）
- フォーム値は struct として値渡し（ポインタ非使用、例外あり）

## 受け入れ基準（概略）

このセッション完了の判定は以下で検証:

1. **画面表示**: GET /alerts/rules?device_id=1 でデバイス選択 + 空フォーム + ルール一覧が正常表示される
2. **ルール追加**: フォーム送信（POST）→テーブルに追加され、一覧が整合し、空フォームに戻る（成功・エラーともに）
3. **ルール編集**: [編集]クリック → 既存値がフォームにロード、PUT 送信 → 一覧に反映、空フォームに戻る
4. **ルール削除**: [削除]→確認メッセージ → DELETE 送信 → ルール削除、一覧から消える
5. **有効/無効切替**: チェックボックス change → PATCH 送信 → 当該行の is_enabled 状態が反転・表示更新（他行影響なし）
6. **デバイス切替**: デバイス select change → GET /alerts/rules?device_id={N} → alert-rule-section が対象デバイスの状態に差し替わる
7. **バリデーション**: 必須項目空欄・oneof 範囲外 → 422 + フォームに .error-message 表示。フォームの他の値は保持される
8. **セッション認証**: ログインユーザーが所有するデバイスのルールのみアクセス可能。他ユーザーデバイスは 403
9. **テストカバレッジ**: Handler 単体テストで上記 1-5 の成功・エラーケース、Repository/サービス層は既実装確認のみ

## 未確定事項・要確認（要決定後に実装開始）

1. **CSRF ミドルウェアライブラリ**: gin/gin-gonic や github.com/utrack/gin-csrf 等、採用すべきライブラリの名称と import path（セッション準備フェーズで要決定）
2. **MethodOverride ミドルウェア方式**: negroni 風の自作か、既存ライブラリか（採用ライブラリ名・実装方式を要決定。本セッション中に実装予定）
3. **scs セッションストア導入**: PostgreSQL 用 `pgstore` の import path・初期化パラメータ（別セッション側で整備予定だが、このセッションで基本が必要ならその場で実装）
4. **Tom Select ライブラリロード**: CDN URL / npm package 名（mocks では CDN 使用、本実装も同じ方式を採用予定）

--- spec-init 本文 ここまで ---