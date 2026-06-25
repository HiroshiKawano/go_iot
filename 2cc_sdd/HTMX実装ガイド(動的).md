# HTMX実装ガイド(動的)

---

## cc-sdd参照ガイド

本設計書をcc-sdd（詳細設計書）から参照する際に価値の高いセクションと用途を示す。

| 優先度 | セクション | cc-sddでの用途 |
|:------:|-----------|---------------|
| ★★★ | [モックHTML → templ+HTMX 変換ルール](#2-モックhtml--templhtmx-変換ルール) | HTMLモックのAlpine.js制御（モーダル等）や`<a href>`リンク（フィルタ・ソート・ページネーション等）をHTMX属性に変換する際の唯一のルール定義。変換パターンを誤ると全画面のモーダル・検索・フィルタが機能しない |
| ★★★ | [templ コンポーネント分割アプローチ](#templ-コンポーネント分割アプローチ) | Handler が部分更新領域の templ コンポーネントを直接返却するパターンの実装根拠。フルページ templ とサブコンポーネントの分割方式。誤ったパターンを選ぶと templ ファイル構成・Handler 構造・テストがすべて変わる |
| ★★★ | [コンポーネント命名規約](#コンポーネント命名規約) | コンポーネント名・`hx-target` id・Handler の返却対象を統一するルール。名前不一致による実装ミスを防ぐ唯一の根拠 |
| ★★★ | [id属性一覧](#3-id-属性一覧) | Handler（部分返却対象）・templ（コンポーネント定義）・HTMX属性（hx-target指定）の唯一の定義元。HTMLモックにはidが原則含まれないため（R01。R01-2のlabel連携用 `field_*` idを除く）この一覧が必須 |
| ★★★ | [画面別HTMX操作仕様](#4-画面別-htmx-操作仕様) | 各画面のトリガー・URL・ターゲットのプロジェクト固有仕様。ルーティング・Handler実装設計の根拠 |
| ★★★ | [バリデーションエラー表示方針](#7-バリデーションエラー表示方針) | HTMXフォームは Handler 内バリデーション+422+部分コンポーネント返却、通常フォームはリダイレクト+errors引数渡しの使い分け根拠。422のswap設定方式。誤ると全フォームのエラー表示が機能しない |
| ★★★ | [CSRF対応方針](#8-csrf対応方針) | グローバルmeta tag + `htmx:configRequest` 方式の実装根拠。未設定だと全ミューテーションリクエストが403になる。**§8-A に Gin 実装の確定（gorilla/csrf 採用・Web グループ限定/API 除外・SESSION_SECRET→authKey・ミドルウェア合成順）と「gorilla/csrf は既定で HTTPS 前提 → 開発で全 POST が 403」の落とし穴を記載** |
| ★★ | [OOB同時更新エンドポイント一覧](#5-oob-同時更新エンドポイント一覧) | 1リクエストで複数コンポーネントを返却するエンドポイントの定義。単一コンポーネント設計にすると更新されない要素が生じる |
| ★★ | [インラインCRUD方針](#12-インラインcrud方針) | アラートルールのインライン追加・編集・削除パターン。alert-rules で必須 |
| ★★ | [HX-Redirect使用方針](#9-hx-redirect-使用方針) | 削除後のページ遷移実装根拠。`c.Redirect()` と `HX-Redirect` の使い分け |
| ★ | [ページネーションのHTMX統合方針](#10-ページネーションの-htmx-統合方針) | `hx-boost` + `hx-target` によるページネーション部分更新パターン |
| ★★ | [hx-push-url適用方針](#hx-push-url-適用方針) | 検索・フィルタ・ソートでURLを更新するか否かの統一ルール。未定義だとブックマーク・戻るボタンの挙動がバラつく |
| ★ | [削除確認方針](#11-削除確認方針) | `hx-confirm` 使用方針の統一根拠 |
| ★★ | [ネットワークエラー・タイムアウト対応方針](#14-ネットワークエラータイムアウト対応方針) | エラー時のグローバルトースト通知・セッション切れ対応。未設定だとエラー時に無反応 |
| ★★ | [確認モーダルパターン](#c09-確認モーダル保存確認-alpinejs--フォーム送信で実装する) | デバイス削除確認・保存確認モーダル。削除確認（`hx-confirm`）とは異なるパターン |
| ★ | [モーダル連鎖パターン](#c10-モーダル連鎖モーダルから別モーダルを開く) | 連鎖モーダルの「中身差し替え」方式 |
| ★ | [ローディングインジケータ](#c11-ローディングインジケータ) | `hx-indicator` の使用方針と配置パターン |
| ★★★ | [Tom Select + HTMX ライフサイクル管理](#c12-tom-select--htmx-ライフサイクル管理) | Tom Select の初期化方式・グローバルハンドラ・Alpine.js状態同期の**基盤設計**。障害パターンの詳細は §16 |
| ★★★ | [Tom Select 障害パターン集（TS）](#16-tom-select-障害パターン集ts) | Tom Select と HTMX/Alpine.js/CSS の干渉で発生する**具体的な障害と回避策**（TS-1〜TS-8）。新パターン発見時に連番追記 |
| ★ | [複数インラインCRUD領域の共存](#15-複数インラインcrud領域の同一画面共存) | 1画面に複数のインラインCRUD領域がある場合の設計注意 |

> ※ cc-sddのHandler実装・templ設計・ルーティングを記述する際は、まず本書の変換ルール・id属性一覧・画面別操作仕様・バリデーションエラー方針・CSRF対応を確認すること。

### 次回プロジェクトでの記載チェックリスト

HTMX実装ガイド(動的)を新規作成する際に以下が揃っているか確認する：

- [ ] モックHTMLからtempl+HTMXへの変換ルール（Alpine.js → HTMX 置換パターン）を記載
- [ ] 部分コンポーネントをどう分割するか（フルページ templ とサブコンポーネントの関係）を明記（Handler・templ ファイル構成・テスト構造が変わる）
- [ ] コンポーネント名とhx-target idの対応ルール（命名規約）を記載
- [ ] 全画面のid属性一覧（Handler・templ・HTMX属性の「共通語彙」）を記載。HTMLモックにはidが原則含まれないため必須（R01。R01-2のlabel連携用 `field_*` idを除く）
- [ ] 画面ごとのHTMX操作仕様（メソッド・URL・ターゲット・トリガー）を網羅
- [ ] 1リクエストで複数要素を更新するOOBエンドポイントを明記（複数コンポーネント返却箇所）
- [ ] HTMX未使用の画面・操作を明記（誤ってHTMXを適用させないため）
- [ ] HTMXフォームのバリデーションエラー返却方式（422+部分コンポーネント vs リダイレクト）を明記
- [ ] 422レスポンスのswap設定方式（`responseHandling`）を明記
- [ ] HTMXフォームのバリデーション方式（Handler 内 `ShouldBind` の `binding` タグ）を明記
- [ ] CSRF対応方式（グローバルmeta tag方式 vs フォームごとトークン埋め込み方式）を明記
- [ ] `hx-push-url` の適用方針（検索・フィルタ・ソートでURL更新するか）を明記
- [ ] `HX-Redirect` を使う操作とリダイレクト先を記載
- [ ] 削除操作の確認方法（`hx-confirm` / Alpine.jsモーダル等）を明記
- [ ] インラインCRUD（行追加・編集・削除）のHTMXパターンを記載
- [ ] 確認モーダル（保存確認）のパターンを記載（`hx-confirm` との使い分け）
- [ ] モーダル連鎖（モーダル内から別モーダルを開く）のパターンを記載
- [ ] ローディングインジケータ（`hx-indicator`）の方針を記載
- [ ] ネットワークエラー・タイムアウト・セッション切れ時のグローバルエラー通知方式を記載
- [ ] 複数インラインCRUD領域が同一画面に共存する場合の設計注意事項を記載
- [ ] DOM操作ライブラリ（Tom Select等）とHTMXのswapライフサイクル管理方式を記載

---

## 1. HTMX 実装規約

> **cc-sdd への価値:**
> templ には「部分コンポーネントを別ファイルに分離する」設計と「フルページ templ がサブコンポーネントを呼び出し、HTMX 時は Handler がサブコンポーネントを直接レンダリングする」設計の2種類がある。本プロジェクトは後者を採用しているが、どちらを選ぶかは設計上の判断であり、他のドキュメントに記述がない。誤ったパターンで設計すると、Handler の実装方式・templ ファイル数・テストの構造がすべて変わるため、spec-design の段階で必ず参照する必要がある。

### templ コンポーネント分割アプローチ

部分更新領域は **専用の templ コンポーネント関数** に切り出す。フルページの templ コンポーネントがその部分コンポーネントを呼び出してページ全体を描画する。HTMX リクエスト時は Handler が部分コンポーネントを直接 `Render(c.Request.Context(), c.Writer)` で返却し、レイアウト（ヘッダー・サイドバー）を含めない。Blade の `@fragment` / `@endfragment` 構文は使わず、**コンポーネント関数の分割**で同じ効果を得る。

**templ コンポーネント側:**

```templ
// internal/view/page/DeviceReadings.templ
// フルページ: レイアウト + 部分コンポーネント呼び出し
templ DeviceReadingsPage(device Device, readings []Reading, filters ReadingFilters) {
	@layout.App(device.Name + " - 計測履歴") {
		<div id="device-readings-list">
			@DeviceReadingsList(readings, filters)
		</div>
	}
}

// 部分コンポーネント: HTMX 時はこの関数だけを直接レンダリングする
templ DeviceReadingsList(readings []Reading, filters ReadingFilters) {
	<table class="data-table">
		<thead>
			<tr><th>計測日時</th><th>温度</th><th>湿度</th></tr>
		</thead>
		<tbody>
			for _, r := range readings {
				<tr>
					<td>{ r.MeasuredAt.Format("2006-01-02 15:04") }</td>
					<td class="reading-value">{ fmt.Sprintf("%.1f℃", r.Temperature) }</td>
					<td class="reading-value">{ fmt.Sprintf("%.1f%%", r.Humidity) }</td>
				</tr>
			}
		</tbody>
	</table>
	@component.Pagination(filters.Page, filters.TotalPages, "#device-readings-list")
}
```

**Handler 側:**

```go
// HX-Request ヘッダーの有無で返却するコンポーネントを切り替える
func (h *ReadingHandler) Index(c *gin.Context) {
	device, readings, filters := h.load(c) // データ取得・フィルタ組み立て

	if c.GetHeader("HX-Request") != "" {
		// 部分コンポーネントのみ返す（レイアウトは含めない）
		_ = page.DeviceReadingsList(readings, filters).
			Render(c.Request.Context(), c.Writer)
		return
	}
	// 通常リクエスト → フルページを返す
	_ = page.DeviceReadingsPage(device, readings, filters).
		Render(c.Request.Context(), c.Writer)
}
```

> **共通化:** `HX-Request` 判定とコンポーネント切り替えは毎回書くと冗長になるため、Handler ヘルパーへ集約する（§56 参照）。ヘルパーは「HTMX なら部分コンポーネント、そうでなければフルページ」を1関数で受け取り、`Render` まで行う。
>
> **422 ステータスとの併用:** バリデーションエラー時など 422 を返したい場合は、`Render` の前に `c.Status(http.StatusUnprocessableEntity)` を呼んでから部分コンポーネントをレンダリングする（§7 参照）。Laravel の `fragment()->withStatus()` のようなチェーン制約は無く、ステータスとレンダリングを独立に制御できる。

**テスト容易性のための指針** 📅 2026-04-22 — 確立:

Go では Handler が直接 `http.ResponseWriter` に書き込むため、テストは `httptest.NewRecorder()` でレスポンス本文を検証する。「サーバー側で組み立てた変数」を直接 assert する Laravel の `assertViewHas()` のような仕組みは無いため、**レスポンス HTML に対する文字列アサーションで検証**する。

```go
// 非 HTMX: フルページが返る → <html> を含むことを検証
// HTMX:    部分コンポーネントが返る → <html> を含まず、部分 HTML 断片を含むことを検証
req := httptest.NewRequest(http.MethodGet, "/devices/1/readings", nil)
req.Header.Set("HX-Request", "true")
w := httptest.NewRecorder()
router.ServeHTTP(w, req)

body := w.Body.String()
assert.NotContains(t, body, "</html>")          // HTMX 時はレイアウト無し
assert.Contains(t, body, `id="device-readings-list"` == false) // 部分のみ
assert.Contains(t, body, "計測日時")               // 部分コンポーネントの中身
```

**使い分け指針**:
- 一覧画面で `perPage` / `totalCount` / 対象デバイス名等の **サーバー側変数を検証したい** → レンダリング結果の HTML に対する `assert.Contains` で間接検証する（Go には View 変数の直接 assert は無い）
- templ 構造のみ assert で十分な画面 → 同様に HTML 文字列アサーションで足りる
- 共通化のため、`HX-Request` 判定とコンポーネント切り替えは Handler ヘルパー（§56）に統一する

**複数コンポーネントの同時返却（OOB Swap 用）:**

```go
// 1リクエストで一覧 + サマリーを返す（サマリー側は OOB 属性付きコンポーネント）
if c.GetHeader("HX-Request") != "" {
	_ = page.DeviceReadingsList(readings, filters).Render(c.Request.Context(), c.Writer)
	_ = component.ReadingSummaryOOB(summary).Render(c.Request.Context(), c.Writer) // hx-swap-oob="true"
	return
}
```

> templ は同一 `Writer` へ複数コンポーネントを順に書き込めるため、メインコンポーネントの直後に OOB コンポーネントを `Render` するだけで複数領域の同時更新が成立する。

### templ レイアウトのテストとパッケージ分離 📅 2026-06-07 — S1 web-foundation-auth で確立

**`{ children... }` を取るレイアウトを Go テストで描画する:** children は関数引数ではなく context 経由で渡す。`templ.WithChildren(ctx, child)` を使う（引数に追加してはならない）。children を取らないコンポーネント/ページは `comp.Render(ctx, &buf)` で直接描画する。

```go
import "github.com/a-h/templ"

// children を取る Guest レイアウトのテスト
var buf bytes.Buffer
ctx := templ.WithChildren(context.Background(), templ.Raw("<p>子要素</p>"))
_ = layout.Guest("タイトル", cssURL).Render(ctx, &buf)
// buf.String() に対して文字列アサーション（#main-content / .error-message 等）

// children を取らないページ/コンポーネント
var buf2 bytes.Buffer
_ = page.LoginPage(page.LoginView{ /* ... */ }).Render(context.Background(), &buf2)
```

**import 循環の回避（binding 構造体と View 構造体の置き場所）:** フォームの binding 構造体（`form:"..." binding:"..."` タグ付き）は **handler パッケージ**に置く。一方、templ ページが描画に使う View 構造体（`LoginView` / `RegisterView` 等）は **view（page）パッケージ**に置く。handler が view を import して描画するため、View 構造体を handler 側に置くと `view → handler` の循環依存になる。handler が binding 構造体から View 構造体へ詰め替える。

```
handler ──import──▶ view/page（LoginView, RegisterView をここに定義）
   └ loginForm / registerForm（binding 構造体）は handler に定義し、View へ詰め替える
```

**import 循環の第2のケース（page ↔ component）— 部品が表示用 DTO を引数に取るとき 📅 2026-06-07 — S3 dashboard で確立:** ページ templ（`page.DashboardPage`）がカード/バナー部品（`@component.DeviceCard` / `@component.UnhandledAlertBanner`）を描画すると `page → component` の import が生じる。このとき部品関数が「カード1件分の表示用 DTO」を引数に取り、その DTO を **page パッケージに置く**と `component → page` も生じ、**page ↔ component の循環**になる（前項の handler↔view とは別経路の循環）。回避策: **1件分の表示用 DTO（`DashboardDevice` / `DashboardAlert` 等）は部品と同じ component パッケージに置く**。ページ全体の集約 DTO（`DashboardView`）は page に残し、`Devices []component.DashboardDevice` のように参照して `page → component` の一方向に保つ。

```
page ──import──▶ component（DeviceCard / UnhandledAlertBanner と、その引数 DashboardDevice / DashboardAlert を同居）
  └ DashboardView（ページ集約 DTO）は page に定義し、Devices []component.DashboardDevice を持つ
```

> 一般則: **「部品が引数に取る表示用 DTO は、その部品と同じ（または下位の）パッケージに置く」**。view 内の依存は常に `page → component → layout` の一方向に保つ。設計段階で「部品に渡す型をどのパッケージに置くか」を決めておかないと、実装時に循環でビルド不能になる。

**import 循環の第3のケース（Layout を内包する View を component が描画するとき）📅 2026-06-08 — device-create-edit(S4) で確立:** 登録/編集で共有するフォーム部品（`@component.DeviceForm`）のように、**1つの View 構造体を component の templ 関数が引数に取る**場合、その構造体に `layout.AppLayoutData` を内包させてはならない。`layout` パッケージは `component` を import する（`App.templ` が `@component.SiteHeader` / `@component.Sidebar` / `@component.FlashMessage` を描画する）ため、`component` 側の型が `layout.AppLayoutData` を持つと `component → layout → component` の循環になり**ビルド不能**になる（第2のケースの page↔component とは別経路）。

回避策: **View を 2 つに分割する**。Layout を内包するページ集約 View は **page パッケージ**に、フォーム本体の描画パラメータ（Layout 抜き）は **component パッケージ**に置き、page が component の型を `Form` フィールドとして内包する。これは第2のケースで `page.DashboardView` が `[]component.DashboardDevice` を持つのと同型。

```go
// internal/view/page/views.go — ページ集約 View（Layout を持つ）
type DeviceFormView struct {
    Layout     layout.AppLayoutData     // App レイアウト（Title/UserName/CSRFToken/CSSURL）
    DeviceName string                   // 編集見出し「デバイス編集: {DeviceName}」用
    Form       component.DeviceFormView // 共有フォーム描画パラメータ（Layout を内包しない）
}

// internal/view/component/views.go — フォーム本体の描画パラメータ（Layout を持たない）
type DeviceFormView struct {
    CSRFToken  string            // hidden gorilla.csrf.Token 用
    Action     string            // 送信先 "/devices"（登録）/ "/devices/{id}"（編集）
    IsEdit     bool              // true で hidden _method=put を出しボタンを「更新」にする
    CancelURL  string            // キャンセル先 "/dashboard" / "/devices/{id}"
    Name, MacAddress, Location string // 入力値復元
    IsActive   string            // "1"/"0" の radio checked 復元用
    Errors     map[string]string // field → 日本語メッセージ
}
```

ページ templ は `@layout.App(v.Layout){ … @component.DeviceForm(v.Form) }`、フォーム部品は `templ DeviceForm(d component.DeviceFormView)` と、それぞれ自層の View を受け取る。

> 注意: design.md が「**単一の View に Layout を内包して登録/編集で共有する**」と1つの構造体で指定していても、その View を component の templ 関数が引数に取る設計だと上記の循環でビルドできない。設計どおりに 1 構造体で書けないため、**実装時に page 集約 View と component フォーム View へ分割するのは必須の補正**であり、設計逸脱ではない（design レビュー時に「component が引数に取る View へ Layout を内包していないか」を確認すると手戻りを防げる）。

### templ 条件付き boolean 属性と入力値復元のテスト検証 📅 2026-06-08 — device-create-edit(S4) で確立

radio/checkbox の選択状態復元は templ の**条件付き属性記法 `属性?={ 条件式 }`** で行う。条件が `true` のときだけ属性を出力する（templ の boolean 属性糖衣構文）。`generate` 後の出力は `if 条件 { 書き込み " checked" }` に展開されるため、`value="1"` の直後に空白付きで ` checked` が連結される。

```templ
<div class="radio-group">
  <label><input type="radio" name="is_active" value="1" checked?={ v.IsActive == "1" }/> 稼働中</label>
  <label><input type="radio" name="is_active" value="0" checked?={ v.IsActive == "0" }/> 停止中</label>
</div>
```

**レンダリング結果（生成物の挙動）:**
- 条件 `true`: `<input ... value="1" checked>`（`checked` が空白付きで連結）
- 条件 `false`: `<input ... value="0">`（`checked` 属性そのものが出力されない）

text input の `value={ v.X }` は静的な属性式扱いのため、**空文字列でも `value=""` を出力する**（属性は常に存在）。したがって初期表示・空フォーム・入力値復元のいずれも `value="…"` の文字列マッチで検証できる。

**テスト検証パターン:** 選択状態の復元は `strings.Contains` ベースで固定できる。「選択されている」は `value="1" checked` の存在、「選択されていない」は `value="0" checked` の**不在**で表現する（`checked` 属性が出ないことを利用）。

```go
func TestDeviceForm_稼働中はvalue1がchecked(t *testing.T) {
    v := baseDeviceFormView()
    v.IsActive = "1"
    html := render(t, DeviceForm(v))

    assertContains(t, html, `value="1" checked`)            // 稼働中が選択
    if strings.Contains(html, `value="0" checked`) {        // 停止中は非選択（checked が無い）
        t.Errorf("稼働中(=1)なのに停止中(value=0)が checked になっている:\n%s", html)
    }
}
```

空フォーム（`v.Name = ""` 等）も `value=""` の出力を `assertContains` で検証できる。

**参考実装:** `internal/view/component/DeviceForm.templ`（`checked?={ v.IsActive == "1" }`）と `internal/view/component/device_form_test.go` の `TestDeviceForm_稼働中はvalue1がchecked` / `TestDeviceForm_停止中はvalue0がchecked`。

### templ ファイル（`.templ`）内の import の落とし穴 📅 2026-06-07 — S3 dashboard で確立

**`.templ` 内で `github.com/a-h/templ` を手動 import してはならない。** `templ generate` が生成する `*_templ.go` は `templ` パッケージを自動 import するため、`.templ` 側にも `import "github.com/a-h/templ"` を書くと生成物で **`templ redeclared in this block`** と **`"github.com/a-h/templ" imported and not used`** のビルドエラーになる。テンプレ式で `templ.URL(...)` / `templ.SafeURL(...)`（動的 `href` の安全化）を使う場合も **import は不要**（自動 import 経由で解決される）。`.templ` の import ブロックには自前で使うパッケージ（`strconv` 等）だけを書く。動的属性は **`id` は文字列連結、`href` は `templ.URL` / `templ.SafeURL`** で組み立てる。

```templ
// internal/view/component/DeviceCard.templ
package component

import "strconv" // ← a-h/templ は書かない（生成物が自動 import するため重複でビルド不可）

templ DeviceCard(d DashboardDevice) {
	// 動的 id は文字列連結、動的 href は templ.URL（どちらも a-h/templ の import 不要で使える）
	<article id={ "device-card-" + strconv.FormatInt(d.ID, 10) } class="device-card">
		<a href={ templ.URL("/devices/" + strconv.FormatInt(d.ID, 10)) }>詳細を見る</a>
	</article>
}
```

---

### コンポーネント命名規約

> **cc-sdd への価値:**
> 部分コンポーネント名はターゲット id と整合させるプロジェクト規約がある。規約がないと cc-sdd が Handler・templ・HTMX 属性でそれぞれ異なる名前を生成し、実装時に不整合が発生する。

**規則:** 部分コンポーネントが配置されるコンテナの `id` と、コンポーネントが表す論理名を対応させる。**ただしモーダルは例外**（下記参照）。

| 例 | HTMX 属性 | コンポーネント関数 |
|---|----------|------------|
| 計測履歴テーブル | `hx-target="#device-readings-list"` | `DeviceReadingsList(...)` |
| アラート履歴テーブル | `hx-target="#alert-history-list"` | `AlertHistoryList(...)` |
| アラートルール一覧 | `hx-target="#alert-rules"` | `AlertRulesList(...)` |

**モーダルの例外規則:** モーダルはすべて共通の `#modal-content` をターゲットとする（C10参照）。ターゲットが同一のため、コンポーネント名はモーダルの**種類を識別する名前**を付ける。

| 例 | HTMX 属性 | コンポーネント関数 |
|---|----------|------------|
| デバイス削除確認モーダル | `hx-target="#modal-content"` | `DeviceDeleteConfirmModal(...)` |
| アラートルール削除確認モーダル | `hx-target="#modal-content"` | `AlertRuleDeleteConfirmModal(...)` |
| 計測値詳細モーダル | `hx-target="#modal-content"` | `ReadingDetailModal(...)` |

---

### コンポーネントの2つの配置パターン

> **cc-sdd への価値:**
> メインコンポーネントと OOB コンポーネントでは templ 内での配置・属性の付け方が逆になる。間違えると部分更新時の返却範囲が変わるか、フルページ表示時に OOB 属性が誤動作する。どちらのパターンを使うかは id 属性一覧と OOB エンドポイント一覧（セクション 5）で定義している。

**パターン 1: メインコンポーネント（innerHTML swap 用）**

ターゲット要素（id 付きコンテナ）の **内側** に呼び出しを配置する。部分コンポーネントの中身だけが HTMX レスポンスとして返される。

```templ
<div id="device-readings-list">
	@DeviceReadingsList(readings, filters)
	// HTMX 時はこのコンポーネントだけを Render する → コンテナの innerHTML を差し替え
</div>
```

**パターン 2: OOB コンポーネント（outerHTML swap 用）**

コンポーネントのルート要素自身に `id` と `hx-swap-oob="true"` を持たせ、要素 **全体** を OOB で差し替える。

```templ
templ ReadingSummaryOOB(summary Summary) {
	<div id="reading-summary" hx-swap-oob="true">
		// 要素全体が OOB で差し替えられる
	</div>
}
```

> ※ `hx-swap-oob` 属性はフルページ表示時にはブラウザに無視されるため、常に付与して問題ない。

---

### hx-target とコンポーネント返却範囲の整合性 【重要】

> **cc-sdd への価値:**
> HTMX 時の Handler は常に部分コンポーネント**全体**を返却する。`hx-target` がコンポーネントの**部分要素**（例: 結果テーブルの `<div>` のみ）を指す場合、コンポーネント全体の HTML が部分要素の innerHTML にネストされ、壊れたDOM（二重ヘッダー・二重フッター・ネストされた `x-data`）が生成される。

**規則:** `hx-target` は、Handler が返却する**部分コンポーネント全体の直接のコンテナ要素**を指定すること。コンポーネントの内部要素をターゲットにしてはならない。

| パターン | 正誤 | 理由 |
|---------|------|------|
| `hx-target="#modal-content"` + `ReadingDetailModal(...)` | ✅ 正 | コンポーネント全体が `#modal-content` に配置される |
| `hx-target="#device-readings-list"` + `DeviceReadingsList(...)` | ✅ 正 | コンポーネント返却範囲とターゲットidが一致（命名規約） |
| `hx-target="#readings-result"` + `ReadingDetailModal(...)` | ❌ 誤 | コンポーネント全体（ヘッダー+検索+結果+フッター）が結果エリア内にネストされる |

**誤ったパターンの症状:**
- HTMLの二重構造（ヘッダーが2つ表示される等）
- Alpine.js `x-data` のネスト（内側と外側で状態が分離）
- `assertContains` テストでは検出不可（テキストは存在するためパスする）

**対応策:**
- **A案: hx-target をコンポーネントコンテナに統一** — 同一コンポーネント内の異なるセクションだけを更新したい場合は、`hx-target` をコンポーネントのコンテナに統一し、サーバーから毎回コンポーネント全体を返却する。検索条件エリアの入力値・Tom Select状態が毎回リセットされるデメリットがある
- **B案: サブコンポーネント分離** — 対象セクション専用のサブコンポーネントを定義し、Handler で返却コンポーネントを切り替える。Handler の分岐が増えるデメリットがある
- **C案: hx-select でレスポンスから部分抽出（推奨）** — `hx-target` と同じセレクタを `hx-select` にも指定する。サーバーはコンポーネント全体を返すが、クライアント側で `hx-select` に一致する要素のみを抽出してswapする。Handler変更不要で、検索条件エリアの状態もHTMX管理外のため保持される

```html
<!-- ✅ C案: hx-select でコンポーネントから結果エリアのみ抽出 -->
<button hx-get="/devices/1/readings"
        hx-target="#readings-result"
        hx-select="#readings-result"
        hx-push-url="false"
        hx-indicator="">
    検索
</button>
```

> **C案の注意点:** C02（`<tbody>` の差し替え）で禁止している `hx-select` は「フルページHTMLからの抽出」を禁止している。C案はフルページではなく**部分コンポーネントのレスポンスからの抽出**であり、レスポンスサイズはコンポーネント範囲に限定されるため許容される。

> 📅 追記: 2026-03-27 — 検索モーダル設計で検出。検索ボタンの `hx-target` が結果エリアのみを指しており、コンポーネント全体がネストされる潜在バグを確認。
> 📅 追記: 2026-04-10 — 画面確認テストで実際に二重表示バグが発生。C案（`hx-select`）で修正。

---

### 共通コンポーネント使用時の `target` 必須ルール 【重要】

> **cc-sdd への価値:**
> 検索エリア・表示件数セレクタ・ページネーションの各共通コンポーネントは常に `hx-get` または `hx-boost` を内包する **HTMX コンポーネント**。`target` 引数を省略すると `hx-target=""` となり、HTMX 仕様上「トリガー要素自身」（= フォーム）を innerHTML 置換する挙動になる。Handler が部分コンポーネントではなくフルページを返すと、**フォーム内にヘッダー・サイドバー込みのフルページが挿入され、画面が入れ子表示**になる。前節の「hx-target の内部要素誤指定」とは別ケースで、**target 省略 + 部分コンポーネント未分割**の組み合わせで発生する。

**症状:**
- 一覧画面で検索・per_page 切替・ページネーション等を実行すると、検索フォームの中にもう一枚のヘッダー・サイドバー・検索フォーム・一覧テーブルが描画される
- DevTools 上で `document.querySelectorAll('.site-header').length` が 2 以上になる

**対応策（他画面と統一したコンポーネント分割パターン）:**

```templ
// 1. 検索エリアコンポーネントに target 必須
@component.FilterForm(component.FilterFormProps{
	Action: "/devices/1/readings",
	Target: "#device-readings-list",
	Title:  "計測履歴の絞り込み",
}) {
	// フィルタ入力欄
}

// 2. 一覧エリアを id コンテナ + 部分コンポーネントで囲む
<div id="device-readings-list">
	@DeviceReadingsList(readings, filters)
}

// DeviceReadingsList の内部で件数セレクタ・ページネーションにも同じ target を渡す
templ DeviceReadingsList(readings []Reading, filters ReadingFilters) {
	<div class="table-wrapper">...</div>
	@component.PerPageSelector(filters.PerPage, "#device-readings-list")
	@component.Pagination(filters.Page, filters.TotalPages, "#device-readings-list")
}
```

```go
// 3. Handler で HX-Request 判定して部分コンポーネントを返却
func (h *ReadingHandler) Index(c *gin.Context) {
	device, readings, filters := h.load(c)
	if c.GetHeader("HX-Request") != "" {
		_ = page.DeviceReadingsList(readings, filters).Render(c.Request.Context(), c.Writer)
		return
	}
	_ = page.DeviceReadingsPage(device, readings, filters).Render(c.Request.Context(), c.Writer)
}
```

**チェックリスト:**
- [ ] 検索エリアコンポーネント使用箇所すべてに `Target: "#xxx-list"` が指定されている
- [ ] 表示件数セレクタとページネーションコンポーネントにも同じ target を渡している
- [ ] 対応する `<div id="xxx-list"> @XxxList(...) </div>` のコンテナと部分コンポーネントが存在する
- [ ] Handler が末尾で `HX-Request` を判定し、HTMX 時は部分コンポーネントを `Render` している

> 📅 追記: 2026-04-17 — 一覧画面の画面確認テストで入れ子表示バグが発生。当該画面のみ `Target` 未指定 + 部分コンポーネント未分割で実装されていた（他の一覧画面は全て統一パターン）。設計書の「SSR フルページ再描画（HTMX 不使用）」記述を優先して実装したが、共通コンポーネントが常時 HTMX を発火するため矛盾が発生。**設計書に「HTMX 不使用」と書かれていても、検索エリア・件数セレクタ・ページネーションの共通コンポーネントを使う限り HTMX パターンで実装する**のが原則。

---

### hx-swap デフォルト動作

- HTMX の既定の swap は `innerHTML`。本システムもこれを標準とする
- `hx-swap` 属性を明示する必要があるのは `outerHTML` を使う場合のみ
- OOB Swap（`hx-swap-oob="true"`）は常に `outerHTML` で動作する（id が一致する要素全体を差し替え）

---

### hx-push-url 適用方針

> **cc-sdd への価値:**
> `hx-push-url` はブラウザのURLを更新し、戻るボタン・ブックマーク・URLコピーを正しく機能させる。適用範囲が未定義だと開発者ごとに判断がバラつく。

**適用ルール:**

| 操作 | `hx-push-url` | 理由 |
|------|:-------------:|------|
| 計測履歴の絞り込みフォーム送信 | `true` | 絞り込み結果をURLで共有・ブックマーク可能にする |
| デバイス稼働状態フィルタ切替 | `true` | フィルタ状態をURLに反映する |
| ソート切替 | `true` | ソート順をURLに反映する |
| 期間切替（24h/3d/7d/30d） | `true` | 期間設定をURLに反映する |
| ページネーション | 不要 | `hx-boost` が自動的にURLを更新する |
| 表示件数切替 | `true` | 件数設定をURLに反映する |
| モーダル表示 | 不要 | モーダルはオーバーレイであり、独立URLを持たない |
| インラインCRUD | 不要 | データ操作でありURL変更は不適切 |
| 削除 | 不要 | 一覧コンポーネント返却のみ |

> **原則:** 一覧画面の表示状態（絞り込み条件・フィルタ・ソート・期間・ページ・件数）はURLに反映する。データ操作（作成・更新・削除）およびモーダル表示ではURLを変更しない。

---

## 2. モックHTML → templ+HTMX 変換ルール

> **cc-sdd への価値:**
> HTMLモック（`doc/HTMLモック作成ルール.md` の R01〜R27 に準拠）を templ+HTMX に変換する際の統一ルール。モック段階では Alpine.js でモーダル開閉を、`<a href>` でフィルタ・ページネーション・ソートを実現しているが、templ 変換時に HTMX に置換すべき箇所と Alpine.js のまま維持すべき箇所の判断基準がここに定義されている。このルールなしに変換すると、開発者ごとに方式がバラバラになる。

### 判断基準: HTMX化する操作 vs Alpine.js維持する操作

| 操作 | 変換方針 | 理由 |
|------|---------|------|
| モーダル表示（詳細・選択・確認） | **HTMX化**（C01） | モーダル内容はサーバーから動的取得が必要 |
| テーブル検索 | **HTMX化**（C03） | 検索フォーム（R17: GET）に `hx-get` + `hx-target` を付与 |
| ステータスフィルタ・タブ切替 | **HTMX化**（C04） | モックの `<a href>`（R24準拠）に `hx-boost` を適用 |
| ソートヘッダー | **HTMX化**（hx-boost） | モックの `<a href>`（R20準拠）に `hx-boost` を適用 |
| ページネーション | **HTMX化**（C05） | モックの `<a href>`（R19準拠）に `hx-boost` を適用 |
| 表示件数切替 | **HTMX化** | モックの `<select>`（R22-2準拠、ネイティブselect＝Tom Select不要）に `hx-trigger="change"` を適用 |
| 削除操作 | **HTMX化**（C06） | サーバーサイドでの削除処理が必要 |
| インラインCRUD（行追加・編集・削除） | **HTMX化** | サーバーサイドでのデータ操作が必要 |
| ファイルアップロード | **HTMX化** | モックの `<input type="file">`（R22準拠）に `hx-encoding` を適用 |
| 画面遷移リンク | **変換不要**（hx-boost自動適用） | モックの `<a href>`（R21準拠）がレイアウトの `hx-boost` で自動HTMX化 |
| ダウンロード | **HTMX除外** | モックの `<a href>`（R25準拠）に templ 側で `hx-boost="false"` を付与。ファイルDLはHTMX非対応 |
| ドロップダウン・選択式入力 | **`<select>` で実装**（R16・R18・R22-2参照） | カスタムドロップダウン・疑似ドロップダウン禁止。`<select>` + `hx-trigger="change"` でHTMX化。R22-2限定列挙対象（boolean型・status型・表示件数・年月日・DB由来でない少数固定）はネイティブselect許可、それ以外はTom Select適用（R16） |
| Tom Select ライフサイクル | **グローバル管理**（C12） | swap時の破棄/再初期化、モーダル表示後の初期化を App.templ で一括管理 |
| セクション折り畳み（アコーディオン） | **Alpine.js維持**（C07） | 純粋なUI操作、サーバー通信不要 |
| サイドバー開閉 | **Alpine.js維持**（C08） | 純粋なUI操作、サーバー通信不要 |
| 検索条件の折り畳み | **Alpine.js維持**（C07） | 純粋なUI操作、サーバー通信不要 |

---

### C01: Alpine.js モーダル制御 → hx-get に置換する

HTMLモックでは Alpine.js の `@click` + `x-show` でモーダルの開閉を制御しているが、templ 変換時はモーダル内容をサーバーから動的に取得する `hx-get` に置換する。

**モック（変換前）:**

```html
<div x-data="{ showDeleteModal: false, selectedDevice: null }">
  <!-- デバイスカード -->
  <div class="device-card" @click="selectedDevice = 'ハウスA温湿度計'; showDeleteModal = true">
    <span class="device-info">ハウスA温湿度計</span>
  </div>

  <!-- モーダル（静的な仮データ） -->
  <div class="modal-overlay" x-show="showDeleteModal">
    <div class="modal-content">
      <h3 x-text="selectedDevice"></h3>
      <p>このデバイスを削除しますか？</p>
      <button type="button" @click="showDeleteModal = false">閉じる</button>
    </div>
  </div>
</div>
```

**templ+HTMX（変換後）:**

部分更新領域（モーダル内容）は別の templ コンポーネント関数に分割し、`hx-get` のレスポンスとして Handler がそのコンポーネントを直接描画する。

```templ
// internal/view/component/device_list.templ
// デバイスカード: クリックで削除確認モーダル内容をサーバーから取得
templ DeviceCard(d store.Device) {
	<div
		class="device-card"
		hx-get={ fmt.Sprintf("/devices/%d/delete-confirm", d.ID) }
		hx-target="#modal-content"
		hx-trigger="click"
	>
		<span class="device-info">{ d.Name }</span>
	</div>
}
```

```templ
// internal/view/component/modal.templ
// モーダル枠（Alpine.js は開閉制御のみに使用）
templ ModalShell() {
	<div
		x-data="{ open: false }"
		@modal-open.window="open = true"
		@keydown.escape.window="open = false"
	>
		<div class="modal-overlay" x-show="open" @click="open = false">
			<div class="modal-content" id="modal-content" @click.stop>
				<!-- hx-get のレスポンスがここに差し込まれる -->
			</div>
		</div>
	</div>
}
```

**Handler:**

フルページ templ がサブコンポーネントを呼び、HTMX リクエスト時は Handler がサブコンポーネントを直接 `.Render(c.Request.Context(), c.Writer)` する。

```go
// internal/handler/device_handler.go
func (h *DeviceHandler) DeleteConfirm(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "不正なデバイスIDです")
		return
	}

	device, err := h.repo.GetDevice(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "デバイスが見つかりません")
		return
	}

	if c.GetHeader("HX-Request") != "" {
		// HTMX 時はモーダル内容コンポーネントのみを描画
		_ = component.DeleteConfirmModal(device).Render(c.Request.Context(), c.Writer)
		return
	}
	// 通常アクセス時はフルページを描画
	_ = page.DeviceShow(device).Render(c.Request.Context(), c.Writer)
}
```

**変換ポイント:**

| モックの要素 | templ+HTMXでの対応 |
|------------|------------------|
| `@click="showDeleteModal = true"` | `hx-get="/devices/{device}/delete-confirm"` + `hx-target="#modal-content"` |
| `x-show="showDeleteModal"` | `x-show="open"` + `@modal-open.window="open = true"` |
| モーダル内の静的データ | サーバーから返却される templ コンポーネント（`DeleteConfirmModal`） |
| `@click="showDeleteModal = false"` | `@click="open = false"` |

> **モーダル開閉に Alpine.js を残す理由:** HTMX は DOM の差し込みを担当するが、モーダルの表示/非表示アニメーション制御はクライアントサイドの責務。Alpine.js のイベントバス（`@modal-open.window`）で HTMX 完了後にモーダルを開く。
>
> **モーダル内の Tom Select:** モーダルのコンポーネント内に `<select class="js-tom-select">` が含まれる場合、`x-show` が `true` になった後に Tom Select を初期化する必要がある（C12 セクション3参照）。

**HTMXイベントによるモーダル表示トリガー:**

```html
<!-- hx-get 完了後にカスタムイベントを発火してモーダルを開く -->
<div
  class="device-card"
  hx-get="/devices/1/delete-confirm"
  hx-target="#modal-content"
  hx-trigger="click"
  hx-on::after-swap="$dispatch('modal-open')"
>
```

> ※ `hx-on::after-swap` は HTMX が DOM を差し替えた後に発火するイベント。`$dispatch` は Alpine.js のグローバルイベント発火ヘルパー。これらのクライアント側 JS はフレームワーク非依存なのでそのまま流用できる。

---

### C02: `<tbody>` は templ コンポーネントで差し替える（hx-select は使わない）

テーブルの `<tbody>` 部分を更新する際、サーバーからフルページHTMLを返して `hx-select` で `<tbody>` だけを抽出するパターンは**使用しない**。代わりに、Handler が必要な範囲のみの templ コンポーネントを直接描画して返却する。

**使用しないパターン（hx-select）:**

```html
<!-- ❌ hx-select でフルページから tbody を抽出 -->
<form hx-get="/devices/1/readings" hx-target="#readings-tbody" hx-select="#readings-tbody">
```

> ❌ サーバーがフルページHTML（レイアウト込み）を返し、クライアント側で `<tbody id="readings-tbody">` だけを抽出する方式。レスポンスサイズが無駄に大きく、サーバーサイドで不要なデータ取得・ビュー構築が発生する。

**採用パターン（templ コンポーネント分割）:**

部分更新領域を別の templ コンポーネント関数に分割し、フルページ templ がそれを呼ぶ。HTMX 時は Handler がそのコンポーネントを直接描画する。

```templ
// internal/view/component/device_readings_list.templ
// 計測一覧の部分更新領域（thead + tbody + pagination をまとめて1コンポーネント化）
templ DeviceReadingsList(readings []store.Reading, page Pagination) {
	<div id="device-readings-list">
		<div class="table-wrapper">
			<table class="data-table">
				<thead>
					<tr>
						<th>計測日時</th>
						<th>項目</th>
						<th>計測値</th>
						<th>単位</th>
					</tr>
				</thead>
				<tbody>
					if len(readings) == 0 {
						<tr>
							<td colspan="4">
								<p class="empty-message">該当する計測データはありません。</p>
							</td>
						</tr>
					} else {
						for _, r := range readings {
							<tr class="readings-row">
								<td>{ r.MeasuredAt.Format("2006-01-02 15:04") }</td>
								<td>{ metricLabel(r.Metric) }</td>
								<td class="reading-value">{ fmt.Sprintf("%.1f", r.Value) }</td>
								<td>{ metricUnit(r.Metric) }</td>
							</tr>
						}
					}
				</tbody>
			</table>
		</div>

		// ページネーション（自前ページネーションコンポーネント）
		<div hx-boost="true" hx-target="#device-readings-list" hx-swap="innerHTML">
			@Pager(page)
		</div>
	</div>
}
```

```html
<!-- 検索フォーム -->
<form
  hx-get="/devices/1/readings"
  hx-target="#device-readings-list"
  hx-trigger="submit"
>
```

**Handler:**

```go
// internal/handler/reading_handler.go
func (h *ReadingHandler) Index(c *gin.Context) {
	deviceID, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "不正なデバイスIDです")
		return
	}

	var q ReadingQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.String(http.StatusBadRequest, "検索条件が不正です")
		return
	}
	q.Normalize() // metric / period / page のデフォルト補完

	// sqlc 生成クエリ: LIMIT/OFFSET で 20 件ページング
	readings, total, err := h.repo.ListReadings(c.Request.Context(), store.ListReadingsParams{
		DeviceID: deviceID,
		Metric:   q.Metric,
		Period:   q.Period,
		Limit:    q.PerPage,
		Offset:   (q.Page - 1) * q.PerPage,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "計測データの取得に失敗しました")
		return
	}
	pg := NewPagination(total, q.Page, q.PerPage)

	if c.GetHeader("HX-Request") != "" {
		// HTMX 時は一覧コンポーネントのみを描画
		_ = component.DeviceReadingsList(readings, pg).Render(c.Request.Context(), c.Writer)
		return
	}
	// 通常アクセス時はフルページを描画
	_ = page.Readings(deviceID, readings, pg, q).Render(c.Request.Context(), c.Writer)
}
```

> **コンポーネント範囲に thead を含める理由:** ソート時に thead のソートインジケータ（⇅）が変わる可能性があるため、thead + tbody + pagination をまとめて1つのコンポーネントとする。0件時のメッセージ表示もコンポーネント内に含まれるため、条件分岐の整合性が保たれる。
>
> **Tom Select との共存:** コンポーネント内にフィルタ用 `<select class="js-tom-select">` が含まれる場合、swap 時に Tom Select インスタンスの破棄→再初期化が必要。C12 のグローバルライフサイクル管理で自動対応される。

---

### C03: 検索フォーム → hx-get に変換する

**モック（変換前）:**

```html
<form action="#" class="filter-form">
  <input type="text" name="keyword" placeholder="デバイス名で検索">
  <select name="state" class="js-tom-select">
    <option value="">すべて</option>
    <option value="active">稼働中</option>
    <option value="stopped">停止中</option>
  </select>
  <button type="submit" class="btn btn-primary">検索</button>
</form>
```

**templ+HTMX（変換後）:**

```templ
templ DeviceFilterForm(q DeviceQuery) {
	<form
		class="filter-form"
		hx-get="/dashboard"
		hx-target="#device-grid"
		hx-trigger="submit"
		hx-push-url="true"
	>
		<input type="text" name="keyword" placeholder="デバイス名で検索" value={ q.Keyword }/>
		<select name="state" class="js-tom-select">
			<option value="">すべて</option>
			<option value="active" selected?={ q.State == "active" }>稼働中</option>
			<option value="stopped" selected?={ q.State == "stopped" }>停止中</option>
		</select>
		<button type="submit" class="btn btn-primary">検索</button>
	</form>
}
```

**変換ポイント:**

| モックの属性 | templ+HTMXでの対応 |
|------------|------------------|
| `action="#"` | `hx-get="/dashboard"` に置換（`action` は削除） |
| — | `hx-target="#device-grid"` を追加 |
| — | `hx-trigger="submit"` を追加 |
| — | `hx-push-url="true"` を追加（ブラウザURLを更新、ブックマーク・戻るボタン対応） |

> **検索は GET メソッド:** 検索・フィルタ操作はデータを変更しないため `hx-get` を使用する。一覧画面のルートはすべて `GET` で定義する。

> **`hx-include` は form 外の要素にのみ使う:** `<form hx-get="...">` 内の `<input>` / `<select>` / `<textarea>` は HTMX が自動的にシリアライズしてリクエストに含める。`hx-include` は**フォーム外**の要素（例: ヘッダーのコンボボックス、hidden input）をリクエストに追加する場合にのみ使用する。form 内の要素に `hx-include` を指定するのは冗長であり、混乱の原因になる。

---

### C04: ステータスフィルタタブ → hx-boost で変換する

デバイスの稼働状態（稼働中/停止中）でダッシュボードのデバイス一覧を絞り込むタブ。

**モック（変換前）— R24準拠:**

```html
<div class="period-selector">
  <a href="?state=all" class="period-btn status-active">全件(12)</a>
  <a href="?state=active" class="period-btn">稼働中(9)</a>
  <a href="?state=stopped" class="period-btn">停止中(3)</a>
</div>
```

**templ+HTMX（変換後）:**

```templ
templ DeviceStateFilter(current string, counts map[string]int) {
	<div
		class="period-selector"
		hx-boost="true"
		hx-target="#device-grid"
		hx-push-url="true"
	>
		<a
			href="?state=all"
			class={ "period-btn", templ.KV("status-active", current == "all") }
		>{ fmt.Sprintf("全件(%d)", counts["all"]) }</a>
		<a
			href="?state=active"
			class={ "period-btn", templ.KV("status-active", current == "active") }
		>{ fmt.Sprintf("稼働中(%d)", counts["active"]) }</a>
		<a
			href="?state=stopped"
			class={ "period-btn", templ.KV("status-active", current == "stopped") }
		>{ fmt.Sprintf("停止中(%d)", counts["stopped"]) }</a>
	</div>
}
```

**変換ポイント:**

| モックの属性 | templ+HTMXでの対応 |
|------------|------------------|
| `<a href="?state=...">` | そのまま維持。コンテナに `hx-boost="true"` + `hx-target` + `hx-push-url="true"` を追加 |
| `class="status-active"`（静的） | サーバーサイドで `current` と比較して動的に `status-active` クラスを出力（`templ.KV`） |
| 件数テキスト（静的） | サーバーサイドの `counts` マップで動的に出力 |

> **R24準拠により Alpine.js の削除が不要:** HTMLモック作成ルール R24 により、モック段階で稼働状態フィルタ・タブ切替が `<a href>` で実装されている。templ 変換時はコンテナに `hx-boost` を付与するだけで HTMX 化が完了し、Alpine.js の状態管理コード（`x-data`、`@click`、`:class`）を削除する手間がない。
>
> **フィルタタブのアクティブ状態をサーバーサイドで管理する理由:** フィルタ切替はサーバーへのリクエストを伴うため、サーバーが「どのフィルタが選択されているか」を知っている。フィルタタブ自体も部分更新コンポーネントに含め、レスポンスで正しいアクティブ状態を返す。

---

### C05: ページネーション → hx-boost で変換する

モックのページネーションは静的リンク（または仮のHTMLマークアップ）で表現されている。templ 変換時は sqlc の LIMIT/OFFSET で取得したページ情報をもとに、自前のページネーション templ コンポーネントへ `hx-boost` を適用する（Laravel paginator のようなヘルパーは無いので自作する）。

```templ
// internal/view/component/pager.templ
// 自前ページネーションコンポーネント（1ページ20件）
templ Pager(p Pagination) {
	<nav class="pagination" hx-boost="true" hx-target="#device-readings-list" hx-swap="innerHTML">
		if p.HasPrev {
			<a href={ templ.SafeURL(p.PrevURL) } class="btn btn-small btn-secondary">前へ</a>
		}
		<span>{ fmt.Sprintf("%d / %d ページ", p.Current, p.Last) }</span>
		if p.HasNext {
			<a href={ templ.SafeURL(p.NextURL) } class="btn btn-small btn-secondary">次へ</a>
		}
	</nav>
}
```

> `hx-boost` はコンテナ内のリンクを自動的に HTMX リクエストに変換する。`hx-target` を明示しないと `<body>` 全体が対象になるため必ず指定する。

> **表示件数切替（R22-2 ネイティブselect）の HTMX 化:**
> 表示件数切替はページネーションと密接に関連する操作だが、`hx-boost`（リンク用）ではなく `hx-trigger="change"` で HTMX 化する。R22-2 により Tom Select は不要（少数固定選択）。
>
> ```templ
> // 表示件数切替: 部分更新コンポーネントの外に配置（swap 時に DOM 状態を保持するため）
> templ PerPageSelect(perPage int) {
> 	<select
> 		name="per_page"
> 		class="btn btn-small"
> 		hx-get="/devices/1/readings"
> 		hx-target="#device-readings-list"
> 		hx-trigger="change"
> 		hx-push-url="true"
> 		hx-include="[name='keyword'],[name='metric']"
> 	>
> 		<option value="20" selected?={ perPage == 20 }>20件表示</option>
> 		<option value="50" selected?={ perPage == 50 }>50件表示</option>
> 		<option value="100" selected?={ perPage == 100 }>100件表示</option>
> 	</select>
> }
> ```
>
> **ポイント:**
> - **部分更新コンポーネントの外に配置:** 表示件数 select は一覧コンポーネント（`#device-readings-list`）の外に配置する。コンポーネント内に配置すると、swap 時に select の DOM 自体が差し替えられ、ユーザーの選択状態が失われる
> - **`hx-include`:** select が検索フォーム外の独立要素のため、現在の検索条件を引き継ぐには `hx-include` で検索フォーム内のフィルタ値を明示的に含める
> - **Tom Select ライフサイクル管理は不要:** ネイティブ select（R22-2）のため、C12 の swap 時破棄/再初期化の対象外。ブラウザ標準の `change` イベントで `hx-trigger="change"` が直接動作する
> - **`selected` 属性のサーバーサイド出力:** ページネーションや検索で一覧コンポーネントが更新されても、コンポーネント外の select は変化しないため、初回ロード時の `selected`（`selected?={ ... }`）を Handler が渡す値で正しく設定すれば状態は保持される

---

### C06: 削除確認モーダル → hx-delete + hx-confirm に変換する

**モック（変換前）:**

```html
<div x-data="{ showDeleteModal: false, selectedDevice: null }">
  <button type="button" class="btn btn-danger btn-small" @click="selectedDevice = 'ハウスA温湿度計'; showDeleteModal = true">削除</button>

  <div class="modal-overlay" x-show="showDeleteModal">
    <div class="modal-content">
      <p><span x-text="selectedDevice"></span>を削除しますか？</p>
      <button type="button" class="btn btn-secondary" @click="showDeleteModal = false">キャンセル</button>
      <button type="button" class="btn btn-danger" @click="showDeleteModal = false">削除する</button>
    </div>
  </div>
</div>
```

**templ+HTMX（変換後）:**

```templ
templ DeviceDeleteButton(d store.Device) {
	<button
		type="button"
		class="btn btn-danger btn-small"
		hx-delete={ fmt.Sprintf("/devices/%d", d.ID) }
		hx-confirm={ fmt.Sprintf("「%s」を削除しますか？", d.Name) }
		hx-target="#device-grid"
	>
		削除
	</button>
}
```

**変換ポイント:**
- Alpine.js のモーダル開閉ロジック全体を `hx-confirm` 属性1つに置換
- 削除確認はブラウザのネイティブ確認ダイアログを使用（`hx-confirm`）
- Alpine.js のモーダルHTML・状態変数はすべて削除

> **hx-confirm を採用する理由:** 削除確認は「はい/いいえ」の二択であり、カスタムモーダルUIが不要。ブラウザネイティブダイアログの方がアクセシビリティが高く、実装が簡潔。

---

### C07: セクション折り畳み・検索条件折り畳み → Alpine.js のまま維持する

以下のUI操作はサーバー通信を伴わないため、Alpine.jsの `x-show` / `@click` をそのまま維持する。

**そのまま維持する例:**

```html
<!-- セクション折り畳み（デバイス編集 /devices/{device}/edit） -->
<div x-data="{ basic: true, threshold: true, notify: true }">
  <h3 @click="basic = !basic">基本情報</h3>
  <div x-show="basic">
    <!-- デバイス名・設置場所などの入力欄 -->
  </div>
</div>

<!-- 検索条件の折り畳み（アラート履歴・計測値一覧などの一覧画面） -->
<div x-data="{ searchOpen: true }">
  <button type="button" @click="searchOpen = !searchOpen">検索条件</button>
  <div x-show="searchOpen">
    <form hx-get="/alerts/history" hx-target="#alert-history-list">...</form>
  </div>
</div>
```

> **templ 変換時の注意:** Alpine.jsの `x-data` スコープはHTMXのswap対象外の要素に配置すること。swap対象内に `x-data` があると、swap後にAlpine.jsの状態がリセットされる。templ では `x-data` を持つ外側のラッパー要素を、HTMX が差し替える内側のコンポーネント（例: `AlertHistoryList`）の**外**に配置する。

---

### C08: サイドバー → Alpine.js のまま維持する

サイドバーの開閉はサーバー通信を伴わないため、Alpine.jsで制御し続ける。

```html
<!-- サイドバー開閉（全画面共通） -->
<div x-data="{ sidebarOpen: true }">
  <button type="button" @click="sidebarOpen = !sidebarOpen">メニュー</button>
  <aside class="sidebar" x-show="sidebarOpen">...</aside>
  <main class="main-content">...</main>
</div>
```

> **ドロップダウンメニューについて:** HTMLモック作成ルールR16により、カスタムドロップダウン（Alpine.js `x-show` + `@click` による開閉制御）は用途を問わず禁止。すべて `<select>` 要素で実装する。R22-2の限定列挙対象（boolean型・status型・表示件数・年月日・DB由来でない少数固定）はネイティブselect許可、それ以外はTom Select適用（R16）。判定に迷う場合はTom Selectを適用する（R22-2の原則）。templ 変換時は `hx-trigger="change"` でHTMX化するか、ユーザーメニュー（`.user-menu` / `.user-name`）など認証連携が必要な箇所は適切な templ コンポーネントに置換する。

---

### C09: 確認モーダル（保存確認）→ Alpine.js + フォーム送信で実装する

保存前に入力内容の確認サマリーを表示する目的のモーダルである。削除確認（C06: `hx-confirm`）とは異なり、確認内容のカスタム表示が必要なため Alpine.js で実装する。本プロジェクトではデバイス登録（/devices/create）・編集（/devices/{device}/edit）の保存確認に用いる。

**templ + HTMX（実装パターン）:**

```html
<!-- 保存ボタンクリックで確認モーダルを表示 -->
<div x-data="{ confirmOpen: false }">
  <form id="device-form" method="POST" action="/devices">
    <!-- CSRF トークンは Handler が meta タグ経由で渡す。htmx:configRequest で送信（§9参照） -->
    <input type="text" name="name" x-ref="name">
    <input type="text" name="location" x-ref="location">

    <!-- 確認モーダルを開く（フォーム送信はしない） -->
    <button type="button" class="btn btn-primary" @click="confirmOpen = true">保存</button>
  </form>

  <!-- 確認モーダル -->
  <div class="modal-overlay" x-show="confirmOpen" @click="confirmOpen = false">
    <div class="modal-content" @click.stop>
      <h3>入力内容の確認</h3>
      <dl>
        <dt>デバイス名</dt>
        <dd x-text="$refs.name.value"></dd>
        <dt>設置場所</dt>
        <dd x-text="$refs.location.value"></dd>
      </dl>
      <div class="form-actions">
        <button type="button" class="btn btn-secondary" @click="confirmOpen = false">戻る</button>
        <!-- 確認後にフォームを送信 -->
        <button type="button" class="btn btn-primary"
                @click="document.getElementById('device-form').submit()">
          保存する
        </button>
      </div>
    </div>
  </div>
</div>
```

templ コンポーネント側では、この HTML 構造を `templ` 関数（例: `component.DeviceConfirmModal`）として表現する。Alpine.js の属性（`x-data` / `x-show` / `x-text` / `@click`）はそのまま templ の属性として記述できる。

**変換ポイント:**

| 判断基準 | パターン |
|---------|---------|
| 削除確認（はい/いいえの二択） | C06: `hx-confirm`（ブラウザネイティブ） |
| 保存確認（入力内容のサマリー表示） | C09: Alpine.jsモーダル（カスタムUI） |

> **確認モーダルをHTMX化しない理由:** 確認サマリーの内容はすべてクライアント側のフォーム入力値であり、サーバーから取得する必要がない。Alpine.jsの `$refs` でフォーム値を参照し、サーバー通信なしで表示する。

**対象画面:**

| 画面 | モーダル | 確認内容 |
|------|---------|---------|
| /devices/create | デバイス登録確認 | 入力したデバイス情報のサマリー |
| /devices/{device}/edit | デバイス更新確認 | 編集したデバイス情報のサマリー |

---

### C10: モーダル連鎖（モーダルから別モーダルを開く）

アラートルール設定（/alerts/rules）では、対象デバイス選択モーダルから新規デバイス登録モーダルを開く連鎖パターンがありうる。

**実装方針:** モーダル連鎖は「単一モーダル領域の中身を差し替える」方式で実装する。複数モーダルの重ね表示はしない。

```html
<!-- 共通モーダルコンテナ（1つだけ） -->
<div x-data="{ open: false }"
     @modal-open.window="open = true"
     @modal-close.window="open = false"
     @keydown.escape.window="open = false">
  <div class="modal-overlay" x-show="open" @click="open = false">
    <div class="modal-content" id="modal-content" @click.stop>
      <!-- すべてのモーダル内容がここに差し込まれる -->
    </div>
  </div>
</div>

<!-- 対象デバイス選択モーダルを開く -->
<button type="button" class="btn btn-secondary"
        hx-get="/alerts/rules/select-device"
        hx-target="#modal-content"
        hx-on::after-swap="$dispatch('modal-open')">
  デバイス選択
</button>
```

連鎖先のモーダル内容は、それぞれ独立した templ コンポーネント関数として実装し、Handler が `HX-Request` 時に直接 `.Render(c.Request.Context(), c.Writer)` で返す。

```go
// 対象デバイス選択モーダルの templ コンポーネント
// component.SelectDeviceModal(devices []repository.Device) templ.Component
templ SelectDeviceModal(devices []repository.Device) {
	<div>
		<h3>対象デバイス選択</h3>
		<table class="data-table">
			for _, d := range devices {
				<tr>
					<td>{ d.Name }</td>
				</tr>
			}
		</table>
		<!-- 同じモーダル領域の中身を新規デバイス登録フォームに差し替える -->
		<button type="button" class="btn btn-primary"
				hx-get="/devices/create"
				hx-target="#modal-content">
			新規登録
		</button>
	</div>
}
```

```go
// 新規デバイス登録フォームの templ コンポーネント
// 登録完了後に元の選択モーダルに戻る
templ AddDeviceForm(errors map[string]string) {
	<div>
		<h3>デバイス新規登録</h3>
		<form hx-post="/devices"
			  hx-target="#modal-content">
			<div class="form-group">
				<input type="text" name="name">
				if msg, ok := errors["name"]; ok {
					<p class="error-message">{ msg }</p>
				}
			</div>
			<div class="form-actions">
				<button type="submit" class="btn btn-primary">登録</button>
				<!-- キャンセルでデバイス選択モーダルに戻る -->
				<button type="button" class="btn btn-secondary"
						hx-get="/alerts/rules/select-device"
						hx-target="#modal-content">
					キャンセル
				</button>
			</div>
		</form>
	</div>
}
```

> Go ではバリデーションエラーは暗黙の共有バッグではなく、`errors map[string]string`（項目→メッセージ）を templ コンポーネントへ引数で明示的に渡して再描画する。Laravel の `$errors` 共有バッグ・`ShareErrorsFromSession` は存在しない。

**ルール:**
- モーダルコンテナは1つの `#modal-content` のみ使用する（重ね表示しない）
- 連鎖先のモーダルは同じ `#modal-content` の中身を `hx-get` で差し替える
- 「戻る」操作も `hx-get` で元のモーダル内容を再取得する
- モーダルの開閉状態（`open`）はコンテナ側のAlpine.jsで管理するため、中身の差し替えでは影響しない

---

### C11: ローディングインジケータ

HTMX リクエスト中のローディング状態を表示する。

**採用方針:** `hx-indicator` 属性と `.htmx-indicator` CSSクラスを使用する。

```css
/* App.templ から読み込む CSS（public/style.css） */
.htmx-indicator {
    display: none;
}
.htmx-request .htmx-indicator,
.htmx-request.htmx-indicator {
    display: inline-block;
}
```

```html
<!-- 検索フォームのローディング（アラート履歴） -->
<form class="filter-form"
      hx-get="/alerts/history"
      hx-target="#alert-history-list"
      hx-indicator="#search-spinner">
  <input type="text" name="keyword">
  <button type="submit" class="btn btn-primary">検索</button>
  <span id="search-spinner" class="htmx-indicator">検索中...</span>
</form>
```

```go
// テーブル行クリック（モーダル表示）のローディング — templ コンポーネント内
for _, r := range readings {
	<tr hx-get={ fmt.Sprintf("/devices/%d/readings/%d", deviceID, r.ID) }
		hx-target="#modal-content"
		hx-trigger="click"
		hx-indicator="closest tr"
		hx-on::after-swap="$dispatch('modal-open')">
		<!-- hx-indicator="closest tr" → クリックした行に .htmx-request クラスが付与される -->
		<td>{ r.RecordedAt }</td>
		<td class="reading-value">{ fmt.Sprintf("%.1f", r.Value) }</td>
	</tr>
}
```

**ルール:**
- 検索・フィルタ操作にはスピナーテキスト（「検索中...」等）を表示する
- テーブル行クリックでは行自体にCSSクラスを付与し、行の視覚的変化で応答中を示す
- データ量の多い計測値取得（30d 期間など）には進捗テキスト（「読み込み中...」）を表示する

---

### C12: Tom Select + HTMX ライフサイクル管理

HTMLモック作成ルール TS02「templ 変換時の注意事項」を具体化した、Tom Select と HTMX の共存パターン。

- **TS02 注意事項1** は初期化スクリプトの移行先を「`x-init` または共通レイアウト（App.templ）」と概説している
- **本セクション** では HTMX swap との競合を考慮し、以下のように具体化：
  - **方式A**（共通レイアウト `App.templ` の `<body>` 末尾）を推奨 → swap 対象を含むすべてのスコープで使用
  - **方式B**（`x-init`）は swap 対象外のスコープに限定

> **cc-sdd への価値:**
> Tom Select は DOM を直接操作するライブラリであり、HTMX の swap（DOM差し替え）と競合する。swap 後に Tom Select のインスタンスが残存すると、メモリリーク・イベントハンドラの重複・UIの破壊が発生する。本セクションのライフサイクル管理なしに Tom Select 付きの部分更新コンポーネントを swap すると、一覧画面の Tom Select 適用 `<select>` が壊れる（R22-2のネイティブselectは Tom Select インスタンスを持たないため影響なし）。

#### 1. 初期化の移行先

モックの `</body>` 直前にある一括初期化スクリプト（TS02）は、templ 変換時に以下のいずれかに移行する。初期化スクリプト自体はフレームワーク非依存であり、内容はそのまま流用する（配置先のみ App.templ に読み替える）。

**方式A: 共通レイアウト `App.templ` の `<body>` 末尾に配置（推奨）**

```html
<!-- internal/view/layout/App.templ の </body> 直前 -->
<script>
function initTomSelect(root) {
    (root || document).querySelectorAll('select.js-tom-select:not([disabled]):not(.tomselected)').forEach(function(el) {
        new TomSelect(el, {
            allowEmptyOption: true,
            dropdownParent: 'body',
            plugins: el.multiple
                ? ['remove_button', 'dropdown_input']
                : ['dropdown_input'],
            onChange: function(value) {
                el.dispatchEvent(new Event('change', { bubbles: true }));
            }
        });
    });
}
// 初回ロード時
initTomSelect();
</script>
```

> `.tomselected` は Tom Select が初期化済みの `<select>` に自動付与するクラス。`:not(.tomselected)` で二重初期化を防ぐ。
>
> `onChange` で `change` イベントを明示的に発火する理由: HTMX の `hx-trigger="change"` が Tom Select 適用済みのフィルタ用 `<select>`（デバイス絞り込み・計測種別フィルタ等）で確実に動作するようにするため（詳細は§4参照）。ネイティブ select（R22-2限定列挙対象: boolean型・表示件数・期間 24h/3d/7d/30d 等）はブラウザが `change` イベントを直接発火するため本対応は不要。

**方式B: Alpine.js `x-init` でスコープ内を初期化**

```html
<div x-data x-init="initTomSelect($el)">
    <select name="device_id" class="js-tom-select">...</select>
</div>
```

> 方式B は swap 対象外のスコープ（検索フォーム等）に適する。swap 対象内では方式A + ライフサイクル管理（下記§2）を使用する。
>
> **方式B を swap 対象内で使うべきでない理由:**
>
> 1. swap で DOM が差し替わると、**§2 のグローバル `htmx:afterSwap` ハンドラ**が `initTomSelect(target)` を呼び、方式A の汎用 `onChange`（`change` イベント発火のみ）で初期化する
> 2. 同時に、新しい要素に対して **Alpine.js の `x-init`** も発火し、独自の `onChange`（Alpine 状態更新を含む）で初期化しようとする
> 3. この2つの**実行順序は不定**のため、どちらが先に `new TomSelect()` を呼ぶか保証できない → 二重初期化や `onChange` の上書き競合が発生しうる
>
> swap 対象内で Alpine 状態同期が必要な場合は §4 の注意事項を参照。

#### 2. HTMX swap との共存（ライフサイクル管理）

`hx-swap` 対象の部分更新コンポーネント内に `<select class="js-tom-select">` が含まれる場合、swap 前に既存インスタンスを破棄し、swap 後に再初期化する必要がある。以下のスクリプトはフレームワーク非依存であり、そのまま流用する。

```html
<!-- internal/view/layout/App.templ の </body> 末尾 -->
<script>
// swap 前: 対象領域内の Tom Select インスタンスを破棄
document.addEventListener('htmx:beforeSwap', function(event) {
    var target = event.detail.target;
    if (target) {
        target.querySelectorAll('select.js-tom-select.tomselected').forEach(function(el) {
            if (el.tomselect) {
                el.tomselect.destroy();
            }
        });
    }
});

// swap 後: 差し込まれた領域内の Tom Select を初期化
document.addEventListener('htmx:afterSwap', function(event) {
    var target = event.detail.target;
    if (target) {
        initTomSelect(target);
    }
});
</script>
```

**影響範囲:** 以下のすべての部分更新 swap で自動的に適用される。
- 一覧画面の検索・フィルタ・ページネーション（部分更新コンポーネント内に `<select class="js-tom-select">` フィルタがある場合）
- インラインCRUD の行追加・編集（コンポーネント内に `<select class="js-tom-select">` がある場合）
- モーダル内容の差し替え（C01, C10）

> **OOB swap の注意:** `htmx:beforeSwap` / `htmx:afterSwap` はメイン swap ターゲットに対して発火する。OOB コンポーネント（`hx-swap-oob="true"`）で差し替えられる要素内に `<select class="js-tom-select">` が含まれる場合、上記ハンドラの `event.detail.target` はメインターゲットを指すため、OOB 先の Tom Select は自動で destroy/再初期化されない。本プロジェクトでは OOB は集計エリア（`.summary-box` / `.summary-grid`）等に限定使用しており、OOB コンポーネント内に `<select>` を含めない方針とする。やむを得ず OOB 内に `<select>` を配置する場合は、`htmx:oobAfterSwap` イベントで個別に `initTomSelect` を呼ぶこと。

> **障害パターンの詳細は §16 を参照。** 本セクション（C12）はライフサイクル管理の基盤設計のみを記載する。HTMX swap・Alpine.js・モーダル表示タイミングとの干渉で発生する具体的な障害パターンと対策は **§16 Tom Select 障害パターン集（TS）** に集約されている。

#### 3. Alpine.js との状態同期

Tom Select は内部で独自の UI を構築するため、Alpine.js の `x-model` やリアクティブバインディングが直接機能しない。`onChange` コールバックで Alpine 側の状態を更新する。

> **R22-2 ネイティブ select の場合:** ネイティブ select（R22-2: boolean型・表示件数・期間 24h/3d/7d/30d 等）は独自UIを構築しないため、Alpine.js の `x-model` が直接機能する。以下の `onChange` 手動同期パターンは Tom Select 適用 select 専用であり、ネイティブ select には不要。
>
> ```html
> <!-- ネイティブ select（R22-2）は x-model で直接同期可能 -->
> <div x-data="{ period: '24h' }">
>     <select name="period" class="form-control" x-model="period">
>         <option value="24h">24時間</option>
>         <option value="7d">7日間</option>
>         <option value="30d">30日間</option>
>     </select>
> </div>
> ```

```html
<!-- Alpine.js の状態と Tom Select を同期する例 -->
<div x-data="{ selectedDevice: '' }">
    <select name="device_id" class="js-tom-select"
            x-ref="deviceSelect"
            x-init="
                new TomSelect($refs.deviceSelect, {
                    allowEmptyOption: true,
                    dropdownParent: 'body',
                    onChange: function(value) {
                        selectedDevice = value;
                        $refs.deviceSelect.dispatchEvent(new Event('change', { bubbles: true }));
                    }
                });
            ">
        <option value="">選択してください</option>
        <option value="1">ハウスA温湿度計</option>
        <option value="2">ハウスB温湿度計</option>
    </select>
</div>
```

> **`dispatchEvent(new Event('change'))` の目的:** Tom Select v2.x は内部の `updateOriginalInput()` でネイティブの `change` イベントを発火するが、`onChange` コールバックが先に実行されるため、タイミングによっては HTMX の `hx-trigger="change"` が反応しないケースがありうる。`onChange` 内での明示的な `dispatchEvent` により、値の変更直後に確実に `change` が発火する。結果として同一操作で `change` が2回発火しうるが、HTMX はデフォルトで同一要素からの連続リクエストを最新のもので置換するため（キューイング）、二重リクエストの実害はない。Tom Select 適用済みのフィルタ用 `<select>`（デバイス絞り込み等）で `hx-trigger="change"` を使用する場合に必須。ネイティブ select（R22-2限定列挙対象: boolean型・表示件数・期間等）はブラウザ標準の `change` イベントで動作するため、本対応の対象外。

> **通常の一括初期化（方式A）の場合:** §1 の `initTomSelect` 関数に `onChange` が組み込み済みのため、追加対応は不要。方式B（`x-init` で個別初期化）を使用する場合のみ、上記のように `onChange` を明示的に設定すること。
>
> **swap 対象内で Alpine 状態同期が必要な場合:** 上記の `x-init` パターンは swap 対象外での使用を想定している。swap 対象内に配置すると、swap 後に §2 のグローバルハンドラが `initTomSelect(target)` で汎用 `onChange`（`change` イベント発火のみ）で再初期化するため、`selectedDevice = value` 等のカスタムロジックが失われる。swap 対象内で Alpine 状態同期が必要な場合は、`htmx:afterSwap` イベント内で `initTomSelect` の代わりにカスタム初期化を行うか、対象の `<select>` を swap 範囲外（検索フォーム等）に配置すること。

#### 4. CDN → ローカル配信

モック段階では CDN（TS01）を使用するが、本番環境では Tom Select の CSS/JS を `public/` 配下に配置し、`go:embed` で静的アセットとして配信すること。バージョン更新時のキャッシュ無効化はバージョンクエリ（例: `?v=2.3.1`）で行う。

```html
<!-- internal/view/layout/App.templ の <head> 内 -->
<link rel="stylesheet" href="/static/vendor/tom-select.css?v=2.3.1">
<script src="/static/vendor/tom-select.js?v=2.3.1" defer></script>
```

```go
// internal/view/static.go — public/ を go:embed で配信
//go:embed all:public
var staticFS embed.FS
// r.StaticFS("/static", http.FS(sub)) 等で Gin にマウントする
```

> CDN を本番で使用するリスク: CDN障害時にすべての `<select>` が機能しなくなる、CORS・CSP設定の複雑化、バージョン固定の不確実性。`go:embed` で同梱すればバイナリ単体で完結し、これらのリスクを回避できる。

---

## 3. id 属性一覧

HTMLモックは id を一切持たない（HTMLモック作成ルール.md R01）。HTMX の差し替えターゲット（`hx-target` / `hx-swap-oob`）に使う id は、**templ 変換時にバックエンド（Go）側で付与する**。本節は、その付与すべき id を画面ごとに一覧化したものである。各 id は HTMX の部分更新領域（templ サブコンポーネントが描画する DOM の外枠）に対応する。

**前提・命名規約:**

- id はスタイリング目的では使わない。クラス（`style.css` 実クラス）はモック側で付与済みなので、id は HTMX の差し替えターゲット専用とする。
- 命名はケバブケース（小文字・ハイフン区切り）で統一する。同一画面内でユニークであればよいが、画面横断でも衝突しないよう機能名を含める（例: `device-readings-list`）。
- ラッパー要素は0件でも残す（HTMLモック作成ルール.md R03）。`hx-target` が指す器は常に存在させる。
- テーブルは `thead` を固定したまま `tbody` のみ差し替えるパターンを多用するため、差し替え単位が `tbody` の場合は `tbody` 側に id を付ける（HTMLモック作成ルール.md R04）。
- `hx-swap-oob`（Out-of-Band Swap）で別領域を同時更新する場合（例: アラートルール追加後にルール一覧とフォームを同時に書き換える）も、対象領域に id が必要になる。

---

### 3.1 ゲスト画面（login / register）

| 画面 | URL | id | 対象要素 | 用途（HTMX） |
|------|-----|----|---------|-------------|
| ログイン | `/login` | `login-form` | ログイン `<form>` | バリデーション失敗時に `hx-target` でフォーム全体（エラー表示含む）を 422 + 部分HTML で差し替え |
| ユーザー登録 | `/register` | `register-form` | 登録 `<form>` | 同上（登録フォームの再描画） |

> ※ login / register は Guest.templ レイアウト。HTMX を使わず通常 POST でも実装可だが、インラインバリデーション表示を行う場合に上記 id を使う。エラーは共有バッグではなく、各項目直後の `.error-message` へ Handler から明示的に渡す（共有 errors バッグは Go には無い）。

---

### 3.2 ダッシュボード（dashboard）

| URL | id | 対象要素 | 用途（HTMX） |
|-----|----|---------|-------------|
| `/dashboard` | `device-grid` | `.device-grid`（デバイスカードのラッパー） | デバイス削除後・登録後にカードグリッドを差し替え／OOB 更新。0件時は `.empty-message` を内包 |
| | `unhandled-alert-banner` | `.summary-box` 等で構成する未対応アラートバナーのラッパー | デバイス削除・アラート解消後に未対応アラート一覧を再描画（OOB Swap 候補） |

> ※ デバイスカードは `.device-card` の繰り返し。個別カードを差し替える必要がある場合は、カードごとに `device-card-{id}`（例: `device-card-1`）を付与する。`{id}` はデバイスの主キー。

---

### 3.3 デバイス詳細（device-show）

| URL | id | 対象要素 | 用途（HTMX） |
|-----|----|---------|-------------|
| `/devices/{device}` | `delete-device-modal` | `.modal-overlay`（削除確認モーダル） | 削除ボタン押下時の確認モーダル。Alpine.js の `x-show` で開閉し、`x-data` スコープ内に配置（TL06） |
| | `device-chart-area` | 温度グラフ + 湿度グラフ + 期間セレクタを内包するラッパー | 期間選択（24h/3d/7d/30d）切替の `hx-target`。この領域全体を差し替え、期間ボタンの active 状態もサーバー側往復（§4 device-show / §10-D） |
| | `temperature-chart` | 温度グラフ領域（`.chart-placeholder` のラッパー、`device-chart-area` 内） | 期間切替時にサーバ生成 SVG を差し替え |
| | `humidity-chart` | 湿度グラフ領域（`.chart-placeholder` のラッパー、`device-chart-area` 内） | 同上（湿度グラフの差し替え） |
| | `latest-readings-table` | 最新計測データテーブルの `tbody`（`.data-table` 内） | 固定10件・ページネーションなし。**グラフの期間切替には連動しない**（常に最新10件）。自動更新（ポーリング）を導入する場合の差し替えターゲット |

> ※ 期間選択ボタン（`.period-btn` / `.period-selector`）は `?period=24h` 等を送る。期間切替時は `device-chart-area`（温度・湿度グラフ + 期間セレクタを内包）を1リクエストで差し替える（§4 device-show）。最新計測テーブル（`latest-readings-table`）は期間に連動しないため差し替え対象に含めない。
> ※ 削除確認モーダルの本文（デバイス名等）を動的に差し込む場合は、モーダル内の本文ラッパーに `delete-device-modal-body` を追加する。

---

### 3.4 デバイス登録 / 編集（device-create / device-edit）

| 画面 | URL | id | 対象要素 | 用途（HTMX） |
|------|-----|----|---------|-------------|
| デバイス登録 | `/devices/create` | `device-form` | デバイス `<form>`（`.form-group` 群 + `.form-actions`） | バリデーション失敗時に 422 + 部分HTML でフォームを差し替え、入力値を復元（TL10） |
| デバイス編集 | `/devices/{device}/edit` | `device-form` | 同上（同一フォーム構造を再利用） | 同上 |

> ※ デバイス登録・編集はHTMXを使わずフルページ POST で実装する（画面設計書(静的).md）のが基本方針。`device-form` id は、インラインバリデーションを HTMX で行う場合のターゲットとして用意する。登録と編集はフォーム構造が同一のため id 名も共通でよい（画面ごとにユニークであれば衝突しない）。

---

### 3.5 センサーデータ履歴（readings）

| URL | id | 対象要素 | 用途（HTMX） |
|-----|----|---------|-------------|
| `/devices/{device}/readings` | `device-readings-list` | 集計 + データ一覧テーブル + ページネーションを内包するフィルタ結果領域（`.summary-grid` / `.data-table` / `.pagination` を含む） | フィルタ検索（期間 from/to）・ページ送り時にこの領域全体を差し替え（§4 readings）。`thead` は固定 |
| | `readings-pagination` | `.pagination`（ページネーションのラッパー） | 一覧と分けて個別 OOB 更新する場合の細分 id（通常は `device-readings-list` に内包） |
| | `readings-summary` | `.summary-grid` / `.summary-box`（集計情報のラッパー） | 同上（通常は `device-readings-list` に内包） |

> ※ フィルタフォーム（`.filter-form`）は `method="GET"`（HTMLモック作成ルール.md R17）。`hx-get` + `hx-target="#device-readings-list"` でフィルタ結果領域（集計 + 一覧 + ページネーション）をまとめて差し替える。集計・ページネーションを個別 OOB で更新したい場合のみ `readings-summary` / `readings-pagination` を使う。20件/ページ（画面設計書(静的).md）。

---

### 3.6 アラートルール管理（alert-rules）

| URL | id | 対象要素 | 用途（HTMX） |
|-----|----|---------|-------------|
| `/alerts/rules` | `alert-rule-section` | 追加/編集フォーム + ルール一覧を内包するコンテナ | デバイス切替・ルール追加/更新時にこの領域全体を差し替え（§4 alert-rules）。内部は `alert-rule-form` + `alert-rule-list` で構成 |
| | `alert-rule-form` | ルール追加/編集フォーム（`.rule-form`） | `[編集]` 押下時に既存値をロードしたフォームへ `outerHTML` で差し替え（追加/編集を同一要素で兼用）。更新完了後は空の追加フォームに戻す |
| | `alert-rule-list` | ルール一覧の **`div` ラッパー**（中に `table.data-table` か空状態 `p.empty-message` を出し分け。`tbody` ではない） | ルール削除後に一覧を差し替え（最後の 1 件削除で空状態へ遷移するため `div`。§12 / §56.4 参照） |
| | `alert-rule-row-{rule}` | ルール一覧の各行 `<tr>` | 有効/無効切替（PATCH）時に当該行のみ `outerHTML` で差し替え。`{rule}` はルールの主キー |

> ※ 追加フォームと編集フォームは同一要素を再利用する（画面設計書(静的).md「アラートルール編集時のフォーム復元方針」、Alpine.js は使わない）。`[編集]` は `hx-get` で `alert-rule-form` を既存値入りに差し替え、送信先 URL と HTTP メソッド（PUT）も切り替える。ルール追加・更新後は `alert-rule-section` 全体（空に戻したフォーム + 更新後の一覧）を差し替える（§4 のテーブル全体返却パターン）。
> ※ 操作列（`[編集]` `[削除]`）の flex 配置は `<td>` 直下ではなく内側 `<div>` に当てる（TL07）。
> ※ デバイス選択（`.js-tom-select`）は Tom Select 適用。デバイス切替時は `alert-rule-section` をターゲットに対象デバイスのフォーム + ルール一覧へ差し替える。有効/無効切替は行単位で `alert-rule-row-{rule}` を差し替える。

---

### 3.7 アラート履歴（alert-history）

| URL | id | 対象要素 | 用途（HTMX） |
|-----|----|---------|-------------|
| `/alerts/history` | `alert-history-list` | **結果領域全体のラッパー `div`**（内側に `.error-message` / `table.data-table`（`thead`+`tbody`）or 空状態 `p.empty-message` / `nav.pagination` を内包） | フィルタ検索（デバイス・期間）・ページ送り時に結果領域全体を `innerHTML` で差し替え |

> ⚠️ **実装で確定（2026-06-08・OOB 不採用）**: 旧版はページネーションを別 id `alert-history-pagination` として `hx-swap-oob` で同時更新し、`alert-history-list` は `tbody` のみを指す想定だった。しかし**実装は OOB を使わず**、ページャを結果領域 fragment `#alert-history-list`（一覧テーブル＋空状態＋ページャを内包する `div`）の**内側に置き、単一 innerHTML swap** で一覧とページャを同時更新する方式に確定した（§5「OOB 最小限」＋兄弟 readings 流儀に統一。詳細 §64）。したがって `alert-history-pagination` / `alert-history-filter` の独立 id・OOB swap は**使わない**。
> ※ フィルタフォーム（`.filter-form`）は `method="GET"`（R17）で結果領域の**外**に置く（swap 対象外＝入力保持・Tom Select 再初期化不要）。デバイス選択（`.js-tom-select`）は Tom Select 適用。`hx-get` + `hx-target="#alert-history-list"` + `hx-swap="innerHTML"` + `hx-push-url="true"` で履歴を更新する。20件/ページ（画面設計書(静的).md）。
> ※ 通知状態（済/未）の絞り込みをステータスフィルタとして設ける場合は、`<a href="?status=notified">` 等の `.period-btn` 相当リンクで実装し（R24）、ターゲットは `alert-history-list`。

---

### 3.8 共通レイアウト（App.templ）

| id | 対象要素 | 用途（HTMX） |
|----|---------|-------------|
| `main-content` | `<main class="main-content">`（`.main-inner` を内包） | `hx-boost` でのページ遷移時に、サイドバー・ヘッダーを保持したままメインコンテンツのみ差し替えるターゲット（採用する場合） |
| `flash-message` | フラッシュメッセージ表示領域（`.error-message` 等） | 操作完了通知・エラーバナーを OOB Swap で表示する共通領域。フルページ時のエラーはここ1箇所に集約（TL11） |

> ※ `hx-boost` を全面採用する場合、リンク・フォームのデフォルトターゲットを `#main-content` に統一できる。ただしサイドバーのアクティブ表示（現在ページのハイライト）は `hx-boost` 時に手動更新が必要になる点に注意。
> ※ ヘッダーのユーザーメニュー（`.user-menu`）は Alpine.js のローカル状態（`x-data`）で開閉するため id は不要（R16-2）。

---

### 3.9 id 命名一覧（まとめ）

templ 変換時にバックエンドが付与する id の一覧。重複なく付与すること。

| id | 画面 | 差し替え方式 |
|----|------|------------|
| `login-form` | login | `hx-target`（フォーム再描画） |
| `register-form` | register | `hx-target`（フォーム再描画） |
| `device-grid` | dashboard | `hx-target` / OOB |
| `unhandled-alert-banner` | dashboard | OOB |
| `device-card-{id}` | dashboard | `hx-target`（個別カード、任意） |
| `delete-device-modal` | device-show | Alpine `x-show`（モーダル） |
| `device-chart-area` | device-show | `hx-target`（期間切替、グラフ+期間セレクタ内包） |
| `temperature-chart` | device-show | `device-chart-area` 内 |
| `humidity-chart` | device-show | `device-chart-area` 内 |
| `latest-readings-table` | device-show | `hx-target`（tbody、期間非連動） |
| `device-form` | device-create / device-edit | `hx-target`（フォーム再描画） |
| `device-readings-list` | readings | `hx-target`（集計+一覧+ページネーション領域） |
| `readings-pagination` | readings | OOB（細分・任意） |
| `readings-summary` | readings | OOB（細分・任意） |
| `alert-rule-section` | alert-rules | `hx-target`（フォーム+一覧コンテナ） |
| `alert-rule-form` | alert-rules | `hx-target` / `outerHTML`（追加/編集兼用） |
| `alert-rule-list` | alert-rules | `hx-target`（div ラッパー、削除時。`table`/空状態 `p` を内包） |
| `alert-rule-row-{rule}` | alert-rules | `outerHTML`（有効切替の行） |
| `alert-history-list` | alert-history | `hx-target`（結果領域全体のラッパー `div`・一覧＋空状態＋ページャを内包・`innerHTML`。OOB 不採用 §64） |
| `main-content` | 共通レイアウト | `hx-boost` ターゲット（任意） |
| `flash-message` | 共通レイアウト | OOB（共通通知） |

## 4. 画面別 HTMX 操作仕様

> **cc-sdd への価値:**
> 「何をトリガーに」「どの URL へリクエストし」「どの id 要素を更新するか」の組み合わせはプロジェクト固有の設計決定であり、他のドキュメントから導出できない。本セクションは確定スタックの 9 画面（login / register / dashboard / device-show / device-create / device-edit / readings / alert-rules / alert-history）について、各操作を HTMX 化するか、フルページ遷移にするか、Alpine.js でクライアント完結させるかを定義する。

### 設計原則

本プロジェクトの HTMX 化方針は以下の 3 種に分類する。各画面の操作表ではこの分類を「方式」列で示す。

| 方式 | 用途 | 実装 |
|------|------|------|
| **HTMX** | 部分更新（一覧の差し替え・グラフ差し替え・インライン CRUD） | `hx-get` / `hx-post` / `hx-put` / `hx-delete` + `hx-target` + `hx-swap`。Handler は templ の sub-component を `.Render(c.Request.Context(), c.Writer)` で返す（§56 ヘルパー） |
| **フルページ** | 認証系・デバイス登録/編集の保存（ページ全体遷移を伴う） | 通常の `<form method="POST">` 送信。Handler は `c.Redirect(http.StatusSeeOther, "/...")` でリダイレクト |
| **Alpine.js** | サーバー往復不要の UI 開閉（モーダル表示/非表示・ヘッダーメニュー） | `x-data` / `x-show` / `@click`。サーバーへリクエストしない |

> **HX-Request 判定:** Handler は `c.GetHeader("HX-Request") != ""` でフルページ要求か部分更新要求かを分岐する。共通化は §56 の Handler ヘルパーを使う。
> **id 命名:** `hx-target` が参照する Fragment id は §3「id 属性一覧」で定義する。本セクションの表に登場する id（`device-readings-list`、`alert-rule-list`、`alert-history-list`、`temperature-chart`、`humidity-chart` 等）はモック HTML には存在せず、templ 変換時に付与する。

---

### login（ログイン） — フルページのみ

> **URL:** `/login` ／ **レイアウト:** `internal/view/layout/Guest.templ`
> **この画面は HTMX を使用しない。** すべて通常のフォーム送信（POST → リダイレクト）。

| 操作 | 方式 | メソッド | URL | レスポンス |
|------|------|---------|-----|----------|
| 初期表示 | フルページ | GET | `/login` | `internal/view/page/Login.templ`（ゲストレイアウト） |
| ログイン | フルページ | POST | `/login` | 成功: `c.Redirect(303, "/dashboard")` ／ 失敗: バリデーションエラーを引数で渡して `Login.templ` を再描画（Go では errors を引数で明示的に渡す。共有バッグは無い） |
| 登録画面へ遷移 | フルページ | GET | `/register` | `<a href="/register">`（通常リンク） |

**注意:**
- 認証は scs セッション（将来 `internal/auth/session_auth.go`）。`Auth::user()` / `session()` 相当は scs の `session.GetInt(ctx, "user_id")` 等で扱う。
- フォーム値の保持・エラー表示は TL10（フォーム再表示時の入力値保持）/ TL11（フルページはレイアウト内エラー表示）に従う。

---

### register（ユーザー登録） — フルページのみ

> **URL:** `/register` ／ **レイアウト:** `internal/view/layout/Guest.templ`
> **この画面は HTMX を使用しない。**

| 操作 | 方式 | メソッド | URL | レスポンス |
|------|------|---------|-----|----------|
| 初期表示 | フルページ | GET | `/register` | `internal/view/page/Register.templ`（ゲストレイアウト） |
| 登録 | フルページ | POST | `/register` | 成功: `c.Redirect(303, "/dashboard")`（自動ログイン）／ 失敗: バリデーションエラー（`name` 必須255文字以内・`email` メール形式・`password` 8文字以上・`password_confirmation` 一致）を引数で渡して `Register.templ` を再描画 |
| ログイン画面へ遷移 | フルページ | GET | `/login` | `<a href="/login">`（通常リンク） |

**注意:**
- バリデーションは Handler 内で `c.ShouldBind`（`binding:"required,email"` 等のタグ）。`password_confirmation` の一致は `binding:"eqfield=Password"`。
- エラーは map[string]string（項目→メッセージ）へ変換し、templ コンポーネントへ明示的に引数で渡す。

---

### dashboard（ダッシュボード）

> **URL:** `/dashboard` ／ **レイアウト:** `internal/view/layout/App.templ`
> デバイス管理はダッシュボードが兼ねる。未対応アラート一覧 + デバイスカード一覧 + デバイス登録ボタンで構成。

| 操作 | 方式 | メソッド | URL | ターゲット | swap | トリガー | レスポンス |
|------|------|---------|-----|----------|------|---------|----------|
| 初期表示 | フルページ | GET | `/dashboard` | — | — | — | `internal/view/page/Dashboard.templ`（未対応アラート + `device-grid`） |
| デバイス登録画面へ遷移 | フルページ | GET | `/devices/create` | — | — | click | `<a href="/devices/create">`（`.btn-primary`「+ デバイス登録」） |
| デバイス詳細へ遷移 | フルページ | GET | `/devices/{device}` | — | — | click | `<a href="/devices/{device}">`（各 `.device-card` の「詳細を見る」） |

**注意:**
- デバイスカード（`.device-card` / `.device-grid`）と未対応アラート（`.summary-box`）は初期描画のみ。リアルタイム自動更新（ポーリング）は MVP では実装しない。将来必要になった場合は `hx-trigger="every 30s"` でデバイスグリッド領域（`#device-grid`）を `hx-get="/dashboard/devices"` で差し替える方式を検討する（現時点では未確定。導入しないこと）。
- デバイス 0 件時は `.empty-message`、未対応アラート 0 件時も同様の空メッセージを表示する（R12）。

---

### device-show（デバイス詳細）

> **URL:** `/devices/{device}` ／ **レイアウト:** `internal/view/layout/App.templ`
> デバイス情報パネル + 期間切替（24h/3d/7d/30d）+ 温度グラフ + 湿度グラフ + 最新計測データ（10件固定）で構成。**期間切替とグラフ差し替えが本画面の HTMX 中核。**

| 操作 | 方式 | メソッド | URL | ターゲット | swap | トリガー | レスポンス |
|------|------|---------|-----|----------|------|---------|----------|
| 初期表示 | フルページ | GET | `/devices/{device}` | — | — | — | `DeviceShow.templ`（デフォルト期間 24h でグラフ + 最新計測10件） |
| 期間切替（24h / 3d / 7d / 30d） | HTMX | GET | `/devices/{device}/chart?period={24h\|3d\|7d\|30d}` | `#device-chart-area` | innerHTML | click | グラフ領域 Fragment（温度グラフ + 湿度グラフ SVG を再生成）+ 期間ボタンの active 状態 |
| 編集画面へ遷移 | フルページ | GET | `/devices/{device}/edit` | — | — | click | `<a href="/devices/{device}/edit">`（`.btn`「編集」） |
| 削除 | HTMX | DELETE | `/devices/{device}` | — | — | click（確認後） | 成功時 `c.Header("HX-Redirect", "/dashboard")`（§9。一覧に戻す） |
| 「もっと見る →」遷移 | フルページ | GET | `/devices/{device}/readings` | — | — | click | `<a href="/devices/{device}/readings">` |

**期間切替（24h/3d/7d/30d）の実装:**
- 期間ボタン（`.period-btn` / `.period-selector`）は `<a>` ではなく `<button type="button">` で配置し、`hx-get="/devices/{device}/chart?period=24h"` + `hx-target="#device-chart-area"` + `hx-swap="innerHTML"` を付与する。
- レスポンスは温度グラフ（`#temperature-chart`）と湿度グラフ（`#humidity-chart`）を含むグラフ領域コンポーネント（`internal/view/component/DeviceChartArea.templ`）。SVG はサーバーサイドで自作生成し（システム構成図.md）、`@templ.Raw(svgString)` で埋め込む。
- 24h は `sensor_readings` 生データ、3d/7d/30d（24h以外の複数日）は日次集計クエリ（`GROUP BY DATE(recorded_at)`、DB設計書.md）を sqlc クエリで取得する。
- アクティブな期間ボタンの `.period-btn` に active 状態（`active` 等）を付ける必要があるため、グラフ領域 Fragment 内に期間セレクタも含めて差し替える（フルフラグメント swap で選択状態をサーバー側往復、§10-D）。
- 最新計測データ（10件固定）はグラフ期間に連動しないため差し替え対象に含めない。

**削除の実装:**
- 「削除」ボタンは Alpine.js モーダル（デバイス削除確認モーダル、`.modal-overlay` / `.modal-content`）で確認 UI を出し、確認ボタンが `hx-delete="/devices/{device}"` を発火する方式を推奨する（§24 の「Alpine.js モーダル + DELETE パターン」）。
- 簡易実装として `hx-delete` + `hx-confirm="このデバイスを削除しますか?"`（ブラウザ標準ダイアログ）でもよい（§11 削除確認方針）。本プロジェクトのモックは確認ダイアログ前提でボタンのみ配置（画面設計書(静的).md）。
- 削除成功後は一覧（ダッシュボード）へ戻すため `HX-Redirect: /dashboard` を返す。devices は論理削除（`deleted_at`、DB設計書.md）。

---

### device-create（デバイス登録） — フォーム保存はフルページ

> **URL:** `/devices/create` ／ **レイアウト:** `internal/view/layout/App.templ`
> 1 カラムフォーム（R27）。`name` / `mac_address` / `location` / `is_active`。**保存は HTMX を使わずフルページ POST**（画面設計書(静的).md）。

| 操作 | 方式 | メソッド | URL | レスポンス |
|------|------|---------|-----|----------|
| 初期表示 | フルページ | GET | `/devices/create` | `DeviceForm.templ`（空フォーム、`is_active` 初期値=稼働中） |
| 登録 | フルページ | POST | `/devices` | 成功: `c.Redirect(303, "/devices/{device}")`（作成したデバイス詳細へ）／ 失敗: バリデーションエラーを引数で渡して `DeviceForm.templ` を再描画 |
| キャンセル | フルページ | GET | （前画面） | `<a class="btn btn-secondary">`（前の画面に戻る。遷移元は基本ダッシュボード） |

**注意:**
- バリデーション（Handler 内 `c.ShouldBind`）: `name` `binding:"required,max=255"`、`mac_address` `binding:"required"` + MAC 形式（`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`。カスタムバリデータ or 正規表現タグ）、`location` `binding:"max=255"`、`is_active` はラジオ（稼働中/停止中）。
- MAC アドレスは稼働中デバイスで一意（`devices_mac_address_unique_active`、DB設計書.md）。重複時は項目別エラーを返す。
- エラーは map[string]string にして templ へ明示的に引数で渡す（共有バッグは無い）。入力値の保持は TL10。
- 登録と編集は同一フォーム構造のコンポーネント（`DeviceForm.templ`）を共有し、引数で `action` URL・メソッド・既存値を切り替える（R27）。

---

### device-edit（デバイス編集） — フォーム保存はフルページ

> **URL:** `/devices/{device}/edit` ／ **レイアウト:** `internal/view/layout/App.templ`
> device-create と同一フォーム構造。既存値を埋め込んで表示。**更新は HTMX を使わずフルページ POST**（HTML は PUT を直接送れないため `_method` オーバーライド or POST で受ける。本プロジェクトのルートは PUT 相当）。

| 操作 | 方式 | メソッド | URL | レスポンス |
|------|------|---------|-----|----------|
| 初期表示 | フルページ | GET | `/devices/{device}/edit` | `DeviceForm.templ`（既存値を `value` / `checked` で埋め込み） |
| 更新 | フルページ | PUT | `/devices/{device}` | 成功: `c.Redirect(303, "/devices/{device}")`（更新後の詳細へ）／ 失敗: バリデーションエラーを引数で渡して `DeviceForm.templ` を再描画 |
| キャンセル | フルページ | GET | `/devices/{device}` | `<a class="btn btn-secondary">`（詳細に戻る） |

**注意:**
- バリデーションは device-create と同一。MAC 一意制約チェックは自分自身を除外する（更新対象 id を除く）。
- PUT は HTML フォームから直接送れないため、フォームに `<input type="hidden" name="_method" value="PUT">` を置き、Gin 側で method override ミドルウェアを使うか、ルートを POST で受け Handler 内で更新する（採用方式は要確認）。

---

### readings（センサーデータ履歴）

> **URL:** `/devices/{device}/readings` ／ **レイアウト:** `internal/view/layout/App.templ`
> フィルター（開始日 / 終了日）+ 集計情報（平均/最高/最低 × 温度/湿度）+ データ一覧（20件/ページ）+ ページネーション。検索・ページネーションを HTMX 化する。

| 操作 | 方式 | メソッド | URL | ターゲット | swap | トリガー | レスポンス |
|------|------|---------|-----|----------|------|---------|----------|
| 初期表示 | フルページ | GET | `/devices/{device}/readings` | — | — | — | `Readings.templ`（集計 + 一覧1ページ目） |
| 期間検索 | HTMX | GET | `/devices/{device}/readings?from={日付}&to={日付}` | `#device-readings-list` | innerHTML | submit | 集計 + 一覧 Fragment（`DeviceReadingsList.templ`） |
| ページネーション | HTMX | GET | `/devices/{device}/readings?from=...&to=...&page={N}` | `#device-readings-list` | innerHTML | click（hx-boost） | 同上 Fragment |

**注意:**
- フィルターフォームは `method="GET"`（R17）。検索条件を URL クエリに載せ、ブックマーク・戻るボタンを機能させる。`hx-push-url="true"` でブラウザ履歴に反映する。
- ページネーション（`.pagination`）は `<a href="?page=N">`（R19）。一覧領域 `#device-readings-list` を `hx-boost` 経由で部分差し替えするため、ページネーション領域も Fragment 内に含めて返す。
- ページング 20 件/ページ。sqlc クエリ `ListSensorReadingsPaginated`（`LIMIT 20 OFFSET ...`、画面設計書(静的).md / DB設計書.md）。ページ番号は **1 始まり**（`page=1` が先頭）、`OFFSET = (page - 1) * 20`。`page` 未指定・`page < 1` は 1 に正規化する。自前ページネーション templ コンポーネントで総ページ数を計算する（Laravel paginator は無い）。
- 集計情報（`.summary-box` / `.summary-grid`）は検索条件に連動するため、一覧と同じ Fragment で同時に差し替える。
- クエリパラメータは Handler で `c.Query("from")` / `c.Query("to")` / `c.Query("page")` で受ける。日付未指定（初期表示）はバリデーションをスキップ（TL14）し全期間最新順で表示。
- **日付フィルタが任意（必須でない）の場合**は §7.2 の「全パラメータ無→一括スキップ」ではなく、`from`/`to` を**各々独立**に扱う。空文字は「境界なし」（遠過去/遠未来センチネル）へ写し、値があるものだけ `time.ParseInLocation("2006-01-02", v, jst)` で解釈する。`ParseInLocation` は**形式不正だけでなく暦日範囲外（`2026-13-01` / `2026-04-31` / `2026-02-30`）も error で弾く**（正規化しない）ため、形式＋暦日妥当性の検証はこの 1 呼び出しで足り、別途の「月ごとの日数」チェックは不要。形式不正のフィールドだけ `errors[...]` にメッセージを積み、その場合は件数/一覧/集計クエリを呼ばず空一覧＋インラインエラーで 200 を返す（§7.1）。`from > to` のような意味的に空の指定は**形式は妥当なので通常検索を実行**し、`recorded_at BETWEEN from AND to` が自然に 0 件を返すに任せる（特別なエラーにしない）。
  - **DB 非依存テストの定石:** 暦日範囲外（`2026-04-31` 等）を渡して `errors["from"]` が立つことと、形式エラー時に Querier モックの件数/一覧/集計が**呼ばれない**こと（`*Called == false`）を固定する。`from > to` は逆に**呼ばれる**ことと 0 件描画（`.empty-message`）を固定し、形式エラー経路と区別する。
- データ 0 件時は `.empty-message` を表示（R12）。

---

### alert-rules（アラートルール管理）

> **URL:** `/alerts/rules` ／ **レイアウト:** `internal/view/layout/App.templ`
> デバイス選択 + ルール追加/編集フォーム（同一要素を再利用）+ ルール一覧（有効切替・編集・削除）。**追加/編集/削除/有効切替はすべてインライン HTMX。Alpine.js は使用しない**（画面設計書(静的).md）。

| 操作 | 方式 | メソッド | URL | ターゲット | swap | トリガー | レスポンス |
|------|------|---------|-----|----------|------|---------|----------|
| 初期表示 | フルページ | GET | `/alerts/rules?device_id={N}` | — | — | — | `AlertRules.templ`（デバイス選択 + 空の追加フォーム + 一覧） |
| デバイス切替 | HTMX | GET | `/alerts/rules?device_id={N}` | `#alert-rule-section` | innerHTML | change | 選択デバイスの追加フォーム + ルール一覧 Fragment |
| ルール追加 | HTMX | POST | `/alerts/rules` | `#alert-rule-section` | innerHTML | submit | 一覧へ追加した状態 + 空フォームに戻した Fragment |
| 編集フォーム読込 | HTMX | GET | `/alerts/rules/{rule}/edit` | `#alert-rule-form` | outerHTML | click | 既存値を埋めた編集フォーム（送信先 URL とメソッドを PUT に切替） |
| ルール更新 | HTMX | PUT | `/alerts/rules/{rule}` | `#alert-rule-section` | innerHTML | submit | 一覧へ反映 + 空の追加フォームに戻した Fragment |
| 有効切替（インライン） | HTMX | PATCH | `/alerts/rules/{rule}/toggle` | `#alert-rule-row-{rule}` | outerHTML | change | 当該行のみ差し替え（`is_enabled` 反転後の行） |
| ルール削除 | HTMX | DELETE | `/alerts/rules/{rule}` | `#alert-rule-list` | innerHTML | click（確認後） | 当該行を除いたルール一覧 Fragment |

**追加/編集フォーム再利用の実装:**
- 追加フォームと編集フォームは同一の templ コンポーネント（`internal/view/component/AlertRuleForm.templ`）を再利用し、別ページ・モーダルは使わない（画面設計書(静的).md）。
- 引数 `editingRule *Rule`（nil なら追加モード）で、`action` URL（追加 `/alerts/rules` POST ／ 編集 `/alerts/rules/{rule}` PUT）と各入力値を切り替える。
- `[編集]` クリックで `hx-get="/alerts/rules/{rule}/edit"` がフォーム領域（`#alert-rule-form`）を `outerHTML` で差し替え、既存値ロード + 送信メソッドを PUT に変更する。
- 更新完了後は空の追加フォームに戻る（追加・更新の成功レスポンスに空フォームを含めて返す）。
- フォーム項目: `metric`（select、温度/湿度。`binding:"required,oneof=temperature humidity"`）、`operator`（select、より大きい `>` / より小さい `<` / 以上 `>=` / 以下 `<=`。少数固定のため R22-2 でネイティブ select 可。value は DB 格納値の記号、`binding:"required,oneof=> < >= <="`）、`threshold`（number step=0.01、`binding:"required"`）。
- デバイス選択は DB 由来で件数が増えるため Tom Select を適用（R16 / R18）。

**有効切替（インライン）の実装:**
- 一覧各行のチェックボックス（`.rule-list` 内）に `hx-patch="/alerts/rules/{rule}/toggle"` + `hx-target="#alert-rule-row-{rule}"` + `hx-trigger="change"` を付与し、当該行のみ `outerHTML` で差し替える。
- フォーム全体の再送信は不要。`is_enabled`（DB: boolean、DB設計書.md）を反転して保存し、更新後の行を返す。

**削除の実装:**
- `[削除]` ボタンは `hx-delete="/alerts/rules/{rule}"` + `hx-confirm="このルールを削除しますか?"`（§11）。一覧領域 `#alert-rule-list` を差し替える。alert_rules は論理削除（`deleted_at`、DB設計書.md）。

**注意:**
- ルール一覧（`.rule-list` / `.rule-list-actions`）と追加/編集フォーム（`.rule-form`）はラベル表示に domain.Metric / domain.ComparisonOperator の `Label()` / `Unit()` を使う（例: 温度 + より大きい + 35.00 + ℃ → 「温度より大きい35.00℃」）。
- ルール 0 件時は `.empty-message` を表示（R12）。

---

### alert-history（アラート履歴）

> **URL:** `/alerts/history` ／ **レイアウト:** `internal/view/layout/App.templ`
> フィルター（デバイス / 開始日 / 終了日）+ 履歴一覧（20件/ページ、発火日時・デバイス・指標・ルール条件・実測値・通知状態）+ ページネーション。検索・ページネーションを HTMX 化する。

| 操作 | 方式 | メソッド | URL | ターゲット | swap | トリガー | レスポンス |
|------|------|---------|-----|----------|------|---------|----------|
| 初期表示 | フルページ | GET | `/alerts/history` | — | — | — | `AlertHistory.templ`（一覧1ページ目） |
| 検索（デバイス・期間） | HTMX | GET | `/alerts/history?device_id={N}&from=...&to=...` | `#alert-history-list` | innerHTML | submit | 履歴一覧 Fragment（`AlertHistoryList.templ`） |
| ページネーション | HTMX | GET | `/alerts/history?device_id=...&from=...&to=...&page={N}` | `#alert-history-list` | innerHTML | click（hx-boost） | 同上 Fragment |

**注意:**
- フィルターフォームは `method="GET"`（R17）。`hx-push-url="true"` で URL に検索条件を反映。
- ページネーション（`.pagination`）は `<a href="?page=N">`（R19）+ `hx-boost`。一覧領域 `#alert-history-list` を部分差し替え。
- ページング 20 件/ページ。sqlc クエリ `ListAlertHistoriesPaginated`（`LIMIT 20 OFFSET ...`、画面設計書(静的).md / DB設計書.md）。
- デバイスフィルタ（`device_id`）の select は「全デバイス」+ 各デバイス。DB 由来件数のため Tom Select を適用（R16）。
- 表示: ルール条件は演算子+閾値+単位（例: `>35.00℃`）、実測値は小数2桁+単位（例: `38.50℃`）、通知状態（`is_notified`、DB設計書.md）は 済 / 未。
- クエリパラメータは Handler で `c.Query(...)` で受ける。未指定（初期表示）はバリデーションをスキップ（TL14）し全件最新順。
- 履歴 0 件時は `.empty-message` を表示（R12）。

---

## 5. OOB 同時更新エンドポイント一覧

> **cc-sdd への価値:**
> 1リクエストで物理的に離れた複数要素を同時更新する場合、templ では「メイン更新領域」と「OOB 更新領域」をそれぞれ別の templ コンポーネント関数に分割し、Handler が両方を `.Render(c.Request.Context(), c.Writer)` で続けて書き出す。OOB 側のコンポーネントには `hx-swap-oob="true"` を付与したルート要素を持たせる。

| エンドポイント | メイン更新領域 | OOB 更新領域 |
|-------------|--------------|-------------|
| `/alerts/check`（しきい値チェック実行） | `AlertHistoryList`（→ `#alert-history-list`） | `UnreadAlertBanner`（→ `#unread-alert-banner`、未通知件数バナー）に `hx-swap-oob="true"` |
| `/devices/{device}/toggle`（稼働状態の切替） | `DeviceCard`（→ `#device-card-{id}`） | `UnreadAlertBanner`（→ `#unread-alert-banner`）に `hx-swap-oob="true"` |

> ダッシュボードでアラート確認や稼働状態の切替を行うと、対象デバイスカード（メイン領域）の更新と同時に、ヘッダー付近の「未通知アラート」バナー（離れた位置の OOB 領域）も同一レスポンスで差し替える、という想定。

> **本プロジェクトでは OOB 使用は最小限とする。** 多くの画面はメイン更新領域 1 つで十分であり、フィルタの選択状態（期間ボタン等）もメイン領域内に含めることで対応する。OOB が必要になるのは、物理的に離れた 2 つの要素を同時更新する必要がある場合のみ。

> **Go/templ での OOB 実装イメージ:**
> ```go
> // alert_handler.go: メイン + OOB を続けて書き出す
> func (h *AlertHandler) Check(c *gin.Context) {
>     // ... しきい値チェックして history, unread を取得 ...
>     if c.GetHeader("HX-Request") != "" {
>         // メイン領域
>         component.AlertHistoryList(history).Render(c.Request.Context(), c.Writer)
>         // OOB 領域（ルート要素に hx-swap-oob="true" を持つコンポーネント）
>         component.UnreadAlertBannerOOB(unread).Render(c.Request.Context(), c.Writer)
>         return
>     }
>     page.AlertHistory(history, unread).Render(c.Request.Context(), c.Writer)
> }
> ```

---

## 6. HTMX 未使用の画面・操作

> **cc-sdd への価値:**
> 誤って HTMX を適用させないための明示的な記載。

| 画面・操作 | 理由 |
|----------|------|
| login / register（ゲスト系） | 認証フローはフルページ遷移が前提。HTMX 部分更新は不適切。フルページ POST → リダイレクト |
| dashboard（ダッシュボード） | 表示中心。デバイス状態の能動更新は最小限であり、基本はフルページ表示 |
| device-create（`/devices/create`）/ device-edit（`/devices/{device}/edit`） メインフォーム送信 | デバイスの新規登録・更新はフルページ遷移（POST → リダイレクト）。バリデーションエラーは再描画した templ フォームコンポーネントに `errors` を引数で渡して表示（§7） |
| CSV / 帳票などのファイルダウンロード操作（将来想定） | ファイルダウンロードは HTMX 非対応。通常の `<a href="...">` リンクまたはフォーム送信で処理 |

---

## 7. バリデーションエラー表示方針

> **cc-sdd への価値:**
> HTMX フォームでバリデーションエラー（422）が発生した場合の返却形式はプロジェクト固有の設計決定。

### フォーム種別による方針の違い

| フォーム種別 | エラー返却方式 |
|------------|-------------|
| **HTMX フォーム**（インライン CRUD、モーダル内登録、アラートルールのインライン追加 等） | フォームコンポーネントを **422 ステータス** で返却 |
| **通常フォーム**（device-create / device-edit のメインフォーム、login / register） | フルページ POST → **リダイレクト**（`c.Redirect(http.StatusSeeOther, "/...")`）。エラー時は同じページを再描画 |

### HTMX フォームのエラー返却パターン

**採用方針:** Handler 内で `c.ShouldBind`（または `c.ShouldBindJSON`）を使い、`binding` タグでバリデーションする。Laravel の FormRequest 相当の仕組みは Go には無いため、バリデーション結果は Handler 内で制御する。

> Go では Laravel の暗黙の `$errors` 共有バッグ・`view()->share('errors')`・`ViewErrorBag`・`ShareErrorsFromSession` は **存在しない**。バリデーションエラーは `map[string]string`（項目名 → メッセージ）へ変換し、templ コンポーネントへ **明示的に引数で渡して再描画**する。

```go
// AlertRuleHandler.Add（アラートルールのインライン追加）
// AlertRuleForm は struct で、binding タグで必須・範囲を表現する。
type AlertRuleForm struct {
    Metric    string  `form:"metric" binding:"required,oneof=temperature humidity"`
    Operator  string  `form:"operator" binding:"required,oneof=> < >= <="`
    Threshold float64 `form:"threshold" binding:"required"`
}

func (h *AlertRuleHandler) Add(c *gin.Context) {
    deviceID := c.Param("device")

    var form AlertRuleForm
    if err := c.ShouldBind(&form); err != nil {
        // binding エラーを map[string]string（項目→メッセージ）へ変換
        errs := toFieldErrors(err)

        if c.GetHeader("HX-Request") != "" {
            // HTMX 時: フォームコンポーネントを 422 で返却（errors を明示的に引数で渡す）
            c.Status(http.StatusUnprocessableEntity)
            component.AlertRuleForm(deviceID, form, errs).
                Render(c.Request.Context(), c.Writer)
            return
        }

        // 通常時: 同じ編集画面を errors 付きで再描画（リダイレクトせず 200 で再表示）
        c.Status(http.StatusUnprocessableEntity)
        page.AlertRules(deviceID, form, errs).
            Render(c.Request.Context(), c.Writer)
        return
    }

    // バリデーション成功後の処理（sqlc の repository で INSERT 等）...
}
```

> `toFieldErrors(err)` は `validator.ValidationErrors` を走査し、`map[string]string{ "Threshold": "しきい値を入力してください。" }` のような項目別メッセージを組み立てるヘルパー（実装は Handler ヘルパー §56 と同様に共通化する）。

> **⚠️ 数値必須フィールドの罠（`0` が妥当な値のとき）📅 2026-06-08 — alert-rules で確立:** 上の `Threshold float64 binding:"required"` は **「`0` が妥当な値ではない」項目に限る**。`float64` + `required` は Go のゼロ値で未入力を判定するため、**`threshold=0` を「未入力」と誤判定して 422 で弾く**。`0` 自体が妥当（例: 霜害アラートの `温度 > 0℃`）の項目では、この例をそのままコピーしてはいけない。
>
> **対策: フォーム DTO を `string` で受け、空文字のみ `required` で弾き、数値変換と範囲を Handler 内で検証する。** これで「未入力=422 / `0`=妥当」を両立し、`numeric(5,2)` 範囲（`|x|≤999.99`）外を弾いて DB の numeric overflow（500）も未然に防ぐ。
>
> ```go
> type alertRuleForm struct {
>     Metric    string `form:"metric"    binding:"required,oneof=temperature humidity"`
>     Operator  string `form:"operator"  binding:"required,oneof=> < >= <="`
>     Threshold string `form:"threshold" binding:"required"` // 空文字のみ required で弾く（float64 でなく string）
> }
>
> func parseThreshold(s string) (float64, error) {
>     f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
>     if err != nil {
>         return 0, errThresholdNotNumeric // → "閾値は数値で入力してください"
>     }
>     if f < -999.99 || f > 999.99 {       // numeric(5,2) の保存可能上限
>         return 0, errThresholdOutOfRange // → "閾値は -999.99〜999.99 の範囲で入力してください"
>     }
>     return f, nil // "0" は妥当値として受理
> }
> ```
>
> **binding エラーと Handler 内エラーは 1 つの map に統合して同時表示する。** `c.ShouldBind` は最初のエラーで止まらず全フィールドを検証し、かつ検証可否に関わらず form 値を bind する。したがって `toFieldErrors(bindErr)` で map 化した**後**、`threshold` が `required`（非空）を通過していれば `parseThreshold` を実行して同じ map へ threshold エラーを追記できる。これで「指標未選択（binding 由来）＋ 閾値が非数値（Handler 由来）」が 1 レスポンスで同時に項目別表示される（要件「複数項目同時エラー」型）。実装参考: `internal/handler/alert_rule_form.go`。テストガイダンス集 §41.4 の `*float64`（`nil`/`0.0`）方式も同じ「ゼロ値と未送信の区別」を解くが、範囲検証と項目別メッセージを Handler に集約したい場合は string 受けが素直。

### 422 レスポンスの swap 設定

**採用方針:** 共通レイアウト `internal/view/layout/App.templ` 内で `htmx.config.responseHandling` を設定し、422 をスワップ対象に含める。この JS はフレームワーク非依存なのでそのまま流用する。**さらに `[45]..` には必ず `error: true` を付与する**（理由は下記の警告）。

```html
<!-- App.templ の <body> 末尾（CSRF 設定と同じ箇所） -->
<script>
    htmx.config.responseHandling = [
        {code: "204", swap: false},
        {code: "[23]..", swap: true},
        {code: "422", swap: true},                  // バリデーション: 本文（インラインエラー）を表示
        {code: "[45]..", swap: false, error: true}  // その他の 4xx/5xx: 本文置換せず error 扱い（= htmx:responseError 発火）
    ];
</script>
```

> ⚠️ **`error: true` は必須（別プロジェクト〈Laravel・帳票管理〉の e2e で発見＝KDCS_PRJ-265 バグ⑦の教訓・📅 2026-06-25 移植）。**
> htmx 2.x では `htmx:responseError` の発火条件は「マッチした `responseHandling` エントリの `error` が真であること」。
> `htmx.config.responseHandling` を**全置換**すると、htmx 既定値 `{code:'[45]..', swap:false, error:true}` の **`error:true` を書き落としやすい**。
> 書き落とすと **401/403/404/5xx すべてで `htmx:responseError` が発火せず**、§14 のグローバルエラーハンドラ（トースト通知）が**完全に死にコード化**する（エラーが無言になり、検索などの HTMX 操作後も古い内容が画面に残る）。
> `swap: false` は維持するため本文置換は起きず、**エラーイベントの発火だけ**が復活する。個別 code を並べず `[45]..` 1 本で error 扱いにし、ステータス別の出し分けは §14 のハンドラ側で `status` 判定する。
>
> ✅ **2026-06-25 適用済み**: `internal/view/layout/App.templ` に `error: true` を追加し templ 再生成済み。回帰防止に `internal/view/layout/layout_test.go` で `{code: "[45]..", swap: false, error: true}` の存在をアサート。なお本プロジェクトは楽観ロック（§46 不採用）を使わないため、別プロジェクト版にある `{code:"409", swap:true}` は追加しない。

> エラーメッセージは独立した id 要素ではなく、**フォームコンポーネント内にインライン（`.error-message` クラス）で含める**。templ では `errors` 引数を受け取り、該当項目があれば描画する。

### GET 検索画面のバリデーションエラー方針

一覧画面の検索フォーム（GET）で必須パラメータが未指定/不正な場合、**リダイレクトせず同画面に 200 + インラインエラー + 空一覧で応答**するパターン。CRUD フォームのエラー返却（§7 冒頭）とは方針が異なる。

**応答マトリクス**:

| リクエスト種別 | 応答 |
|--------------|------|
| HTML（通常ブラウザ）/ HTMX `HX-Request`（`Accept` は HTML） | **HTTP 200 + インラインエラー表示 + 一覧を空で返す** |
| JSON（`Accept: application/json`） | HTTP 422 + バリデーションエラー JSON |

**実装パターン**（Handler 側で `c.ShouldBindQuery` し、`Accept` ヘッダーで分岐）:

```go
// ReadingsForm: GET 検索条件（期間など）。binding タグで必須を表現。
type ReadingsForm struct {
    Period string `form:"period" binding:"required,oneof=24h 3d 7d 30d"`
}

func (h *ReadingsHandler) Index(c *gin.Context) {
    deviceID := c.Param("device")

    var form ReadingsForm
    if err := c.ShouldBindQuery(&form); err != nil {
        errs := toFieldErrors(err) // map[string]string

        // JSON クライアントには 422 + エラー JSON
        if strings.Contains(c.GetHeader("Accept"), "application/json") {
            c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errs})
            return
        }

        // HTML / HTMX には 200 + インラインエラー + 空一覧
        // errors は templ コンポーネントへ明示的に引数で渡す（共有バッグは無い）
        if c.GetHeader("HX-Request") != "" {
            component.DeviceReadingsList(deviceID, nil, errs).
                Render(c.Request.Context(), c.Writer)
            return
        }
        page.Readings(deviceID, nil, errs).
            Render(c.Request.Context(), c.Writer)
        return
    }

    // 通常フロー（計測値の検索・集計の実行）
}
```

#### 7.1 errors は templ コンポーネントへ引数で明示的に渡す（Go に共有バッグは無い）

**前提:** Go には Laravel の `$errors` 共有バッグ・`view()->share('errors', $bag)`・`ShareErrorsFromSession` ミドルウェア・`ViewErrorBag` のような **暗黙のエラー共有機構は一切存在しない**。

**方針:** バリデーションエラー（`map[string]string`）は、エラーを表示したい templ コンポーネント（フォーム本体・一覧の上部・エラーバナー等）へ **すべて引数として明示的に渡す**。フルページの場合もフラグメントの場合も同様に、エラーを描画する各コンポーネントが `errors` を引数で受け取る。

```go
if err := c.ShouldBindQuery(&form); err != nil {
    errs := toFieldErrors(err) // map[string]string

    // フルページ: ページコンポーネントへ errors を引数で渡す
    // HTMX:       フラグメント用コンポーネントへ errors を引数で渡す
    if c.GetHeader("HX-Request") != "" {
        component.DeviceReadingsList(deviceID, nil, errs).
            Render(c.Request.Context(), c.Writer)
        return
    }
    page.Readings(deviceID, nil, errs).
        Render(c.Request.Context(), c.Writer)
    return
}
```

```templ
// internal/view/component/error_messages.templ
// errors を引数で受け取り、1 件以上あれば .error-message として描画する。
templ ErrorMessages(errors map[string]string) {
    if len(errors) > 0 {
        <div class="error-message">
            for _, msg := range errors {
                <p>{ msg }</p>
            }
        </div>
    }
}
```

> **⚠ マップを `for _, msg := range errors` で描画すると並び順が非決定になる。** Go のマップ反復順はランダム化されるため、エラーが 2 件以上のとき `<p>` の順序が毎回入れ替わる。影響: (1) レンダリング結果が不安定で snapshot/diff が割れる、(2) 文字列順に依存するテストが**不定期に**落ちる、(3) ユーザーに見せたい順序（開始日→終了日 等）を保てない。**エラー 1 件のテストでは再現せず複数エラー時のみ揺れる**ため気付きにくい。上記 `ErrorMessages` は「順不同で全件出す」最簡形であり、順序を保証したい場面では下記いずれかにする:
>
> - **キー集合が既知（特定の検索画面など）:** マップを range せず、**キーを明示した `if` 分岐**で順に描画する。例（センサーデータ履歴 `DeviceReadingsList`）は `if errors["from"] != ""` → `if errors["to"] != ""` の順で from→to を保証する:
>
> ```templ
>     if len(v.Errors) > 0 {
>         <div class="error-message">
>             if v.Errors["from"] != "" {
>                 <p>{ v.Errors["from"] }</p>
>             }
>             if v.Errors["to"] != "" {
>                 <p>{ v.Errors["to"] }</p>
>             }
>         </div>
>     }
> ```
>
> - **キーが可変（汎用フォーム）:** Handler 側で `sort.Strings` 等で順序を固定した `[]string`（または `[]struct{ Field, Msg string }`）へ詰め替えて templ へ渡し、templ はそのスライスを range する（マップを直接 range しない）。
> - なお項目別フォーム（`AddDeviceForm` の `if msg, ok := errors["name"]; ok` を各項目直後に置く方式）は、各エラーが項目の固定位置に出るため**元々決定的**。非決定になるのは「全件をまとめて出す」`ErrorMessages` 型の集約描画だけ。
>
> **テスト含意:** 集約エラー描画は、**2 件以上のエラーを同時に立てたケース**を必ず 1 本入れ、各メッセージが `strings.Contains` で出ること＋（順序を保証する設計なら）先後関係を `strings.Index` で固定する。1 件だけのテストでは反復順バグを取り逃す。

**Laravel との差分（考え方の翻訳）**:

| 観点 | Laravel | Go / Gin / templ |
|------|---------|------------------|
| エラーの伝播 | `view()->share('errors', $bag)` で shared レジストリ経由（暗黙） | コンポーネント引数で明示的に渡す（暗黙の共有は無い） |
| フラグメント内コンポーネントへの到達 | shared 変数経由でしか解決できない罠あり | フラグメント用コンポーネントの引数に渡せば確実に届く |
| 必須対応 | `view()->share` の呼び忘れで HTMX 時に届かない | 引数渡し忘れがコンパイル/レビューで気付きやすい |

> フラグメント応答時にもエラーが見えるよう、**フラグメント用コンポーネント自身が `ErrorMessages` を内側に持つ**設計とする（フルページ用レイアウトのエラーバナーはフラグメント応答に含まれないため）。

#### 7.2 初期表示（クエリパラメータなし）ではバリデーションをスキップする

**問題:** 検索条件に必須（`binding:"required"`）を課している画面で、URL 直アクセス時にユーザーが何も操作していないのにエラーメッセージが表示されると、「操作ミスをした」錯覚を与える。

**対策:** Handler の検索アクションで、**クエリパラメータが 1 つもない場合は「初期表示」と判定してバリデーションをスキップ**し、空一覧を返す。クエリパラメータが 1 つでもあれば（ユーザーが検索ボタンを押した、または URL 直指定で条件付きアクセスした）従来通りバリデーションを実行する。

```go
func (h *ReadingsHandler) Index(c *gin.Context) {
    deviceID := c.Param("device")

    // クエリパラメータが一切無ければ初期表示扱い
    isInitialAccess := len(c.Request.URL.Query()) == 0

    if !isInitialAccess {
        var form ReadingsForm
        if err := c.ShouldBindQuery(&form); err != nil {
            errs := toFieldErrors(err)

            if strings.Contains(c.GetHeader("Accept"), "application/json") {
                c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": errs})
                return
            }
            // errors を引数で明示的に渡して再描画（§7.1 参照）
            if c.GetHeader("HX-Request") != "" {
                component.DeviceReadingsList(deviceID, nil, errs).
                    Render(c.Request.Context(), c.Writer)
                return
            }
            page.Readings(deviceID, nil, errs).
                Render(c.Request.Context(), c.Writer)
            return
        }
    }

    if isInitialAccess {
        // 初期表示: バリデーションをスキップし、空一覧を返す（errors は空 map）
        if c.GetHeader("HX-Request") != "" {
            component.DeviceReadingsList(deviceID, nil, nil).
                Render(c.Request.Context(), c.Writer)
            return
        }
        page.Readings(deviceID, nil, nil).
            Render(c.Request.Context(), c.Writer)
        return
    }

    // 通常フロー（計測値の検索・集計の実行）
}
```

**「初期表示」判定ロジックの選択基準**:

| 判定式 | 判定される動作 | 採用理由 |
|---|---|---|
| `len(c.Request.URL.Query()) == 0` | URL 直指定で `?` が一切無いときのみ初期表示扱い | ✅ 最もシンプルで誤動作が少ない |
| 特定キーの有無で判定（`c.Query("period") == "" 等を列挙`） | 特定のキーが無ければ初期表示扱い | - 検索系キーを列挙する必要があり保守性低 |
| 必須フィールド単体の空判定 | 必須フィールドが空なら初期表示扱い | ❌ 「検索ボタンを押して必須フィールド空」ケースでエラーが出なくなる |

**適用範囲**: 検索条件に必須（`binding:"required"`）を課している一覧画面（readings の期間など）。必須がない一覧画面は対象外（すべての検索条件が任意なら、このスキップ判定は不要）。

**templ 側のエラー表示**:

HTMX フラグメント応答時にもエラーが見えるよう、フラグメント用コンポーネント（例: `DeviceReadingsList`）の**内側**に `ErrorMessages` を配置する。フルページ用レイアウトのエラーバナーはフラグメント応答に含まれないため、フラグメント内の表示が必須。

```templ
// internal/view/component/device_readings_list.templ
templ DeviceReadingsList(deviceID string, readings []Reading, errors map[string]string) {
    <div id="device-readings-list">
        @ErrorMessages(errors)
        if len(readings) == 0 {
            <p class="empty-message">計測値がありません。</p>
        } else {
            <table class="data-table">
                <!-- 計測値一覧本体 -->
            </table>
        }
    </div>
}
```

**CRUD フォームとの違い**:

| ケース | 方針 | 参照 |
|------|------|------|
| HTMX CRUD フォーム（インライン・モーダル） | HTMX: 422 + フォームコンポーネント返却 / 非 HTMX: 200 で再描画 | §7 「HTMX フォームのエラー返却パターン」 |
| 通常フォーム（非 HTMX、device-create / device-edit / login / register 等） | 成功時は 303 リダイレクト（PRG）、失敗時は同ページを errors 付きで再描画 | §7 冒頭 |
| **GET 検索画面（一覧系）** | **HTML/HTMX: 200 + インライン errors / JSON: 422** | **本小節** |

**なぜリダイレクトではなく 200 か**:

- 検索画面はブラウザの戻る操作や URL 共有で状態を復元できる必要があり、リダイレクトは URL 履歴を汚す
- 検索欄の既入力値を保ったままエラーを表示するには、同じビューを 200 で再描画する方が自然
- HTMX 経由（`HX-Request` ヘッダ付き、`Accept` は HTML）の場合、`Accept` に `application/json` を含まないため HTML ブランチに入る。フラグメント返却と組み合わせると、検索ボタン押下で対象領域だけがエラー文言を含む空一覧に差し替わる

### 反復項目（配列フォーム）の検証エラーは生フィールドパスを露出させず日本語ラベルで返す

> 📅 2026-06-25 — 別プロジェクト（Laravel・帳票管理）の e2e で、複数行フォームの 422 フラグメントに生パス（`rows.0.work_idは必須です。`）が露出した事象（E2Eバグ③ / KDCS_PRJ-262）を移植。本プロジェクトは現状この種の反復フォームを持たないが、将来 1 画面で同種行を複数追加・一括編集する CRUD（例: アラート条件の複数行一括編集）を実装する際の指針。

§7 冒頭の手動バリデーション（Handler 内で `map[string]string`〈フィールド→日本語メッセージ〉を組み、templ の `errors` 引数へ渡す）を**反復行へ拡張すると、エラーキーが `rows.0.work_id` のような添字付き生パスになりやすい**。これをそのままメッセージに混ぜると、422 フラグメントに生パスが露出する。

**対策（Laravel の中央 `lang` 相当が Go には無いことが要点）:**
- 日本語ラベルは**画面ローカル**に持つ。Laravel は中央 `lang/ja/validation.php` の `attributes` で解決するが、Go にこの仕組みは無いので、**そのフォーム専用の「フィールドキー → 日本語ラベル」マップを Handler（またはその画面の view パッケージ）に閉じて持つ**。`rows.*` は画面ごとに別物を指すため中央辞書に置くと衝突する、という Laravel 側の教訓はそのまま当てはまる。
- メッセージは「ラベル＋定型文」で組み（`業種区分は必須です。` のように）、templ 側は受け取った日本語メッセージをそのまま描くだけにする。
- 同一項目を区分コードごとに繰り返す場合（役職別など）は、コード→ラベルの対応を回して `errs["rows."+i+".user_id"] = label+"を指定してください。"` のように**コード別文言**を生成する。

```go
// 画面ローカルのラベル辞書（中央に置かない）
var workRowLabels = map[string]string{"work_id": "作業", "staff_type": "担当区分", "amount": "金額"}

for i, row := range rows {
    if row.WorkID == 0 {
        errs[fmt.Sprintf("rows.%d.work_id", i)] = workRowLabels["work_id"] + "は必須です。"
    }
}
// → 「作業は必須です。」のように画面ラベルで返り、生パス rows.0.work_id は露出しない
```

**テスト**: 422 フラグメントの本文に生パス（`rows.0.` 等）が**含まれないこと**と、日本語ラベル文言が**含まれること**の両方をアサートする（§56.4 の 422 フラグメント検証と同型）。

### 複数項目にまたがる必須要件は「フィールド名キー」でなく「セクションキー」で表示する

> 📅 2026-06-25 — 別プロジェクト（Laravel・帳票管理）の通しテストで、「A・B の両方が必須」という要件メッセージが片方のフィールド横にだけ出て分かりにくかった事象（通しテスト バグ③ / KDCS_PRJ-265-3）を移植。

1 つの要件が**複数フィールドにまたがる**（例: 「アラート通知を有効にするにはメール宛先と閾値の両方が必要」）場合、エラーキーに**特定フィールド名**を使うと、共通フォーム部品が `errors[name]` でそのフィールド直下にだけメッセージと赤枠（`.input-error` 等）を出す。結果、片方が埋まっているときに**埋まっている側へメッセージが出る**・赤枠が誤誘導する、という分かりにくさが起きる。

**対策:** エラーキーを**フィールド非依存のセクションキー**（例 `notify_section`）にし、**該当セクション（パネル / フォームグループ）の最上段**にまとめて表示する。

```go
// Handler: フィールド名でなくセクションキーで積む
errs["notify_section"] = "通知を有効にするにはメール宛先と閾値の両方を入力してください。"
```

```templ
// セクション先頭にセクションレベルで表示（個別フィールド直下には出さない）
if msg, ok := errors["notify_section"]; ok {
    <div class="error-message" role="alert">{ msg }</div>
}
// 各フィールドの * マークと文言で「どの項目が必要か」を示す
```

**ポイント:**
- フィールド名キーをやめると、`errors["mail_to"]` 等の赤枠・固定表示が発火しなくなり、特定フィールドへの誤誘導が消える。
- 折り畳みセクション内に置くなら、エラー時は自動展開する（`open` 条件にエラー有無を OR）。既定折り畳みのまま差し戻すとメッセージが死角に入る。
- ページ上部の集約バナーにも全エラーを出している場合、セクション表示と二重に出るのは正常（上部＝概況／セクション＝該当箇所への誘導）。出現箇所を「フィールド直下→セクション」へ移しただけで総数は不変。

---

## 8. CSRF 対応方針

> **cc-sdd への価値:**
> CSRF 保護は POST / PUT / DELETE / PATCH すべてのミューテーションに適用する想定。HTMX でこれらを使う際のトークン送信方法はプロジェクト固有の選択であり、未記録だと全ミューテーションリクエストが拒否される。

**採用方針:** **Gin の CSRF ミドルウェア（採用ライブラリは要確認）** でトークンを発行し、共通レイアウト `App.templ` の `<head>` に `<meta name="csrf-token">`（Handler が templ にトークンを引数で渡す）を配置する。`htmx:configRequest` イベントでグローバルに `X-CSRF-Token` ヘッダーをセットする。**HTMX フォームでは個別にトークン用 hidden input を書く必要はない。**

> **通常フォーム（非 HTMX）について:** device-create / device-edit のメインフォームや login / register など、HTMX を使わない通常の `<form>` 送信では、フォーム内に CSRF トークンの hidden input を含める（採用ライブラリの作法に従う）。上記のグローバル設定（meta + ヘッダー送信）は HTMX リクエストにのみ適用される。

```templ
// App.templ の <head> 内（csrfToken は Handler から引数で受け取る）
templ App(csrfToken string) {
    <head>
        <meta name="csrf-token" content={ csrfToken }/>
        <!-- ... -->
    </head>
    <!-- ... -->
}
```

```html
<!-- App.templ の <body> 末尾。この JS はフレームワーク非依存なのでそのまま流用 -->
<script>
    document.addEventListener('htmx:configRequest', function(event) {
        event.detail.headers['X-CSRF-Token'] =
            document.querySelector('meta[name="csrf-token"]').content;
    });
</script>
```

> Gin の CSRF ミドルウェアが `X-CSRF-Token` ヘッダーを検証する。グローバル設定により HTMX フォームにトークン用 hidden input を書く必要はない。**採用ライブラリ・ヘッダー名・hidden フィールド名・開発環境での注意点・ミドルウェア合成順は §8-A で確定済み。**

### 8-A. Gin 実装の確定（gorilla/csrf）📅 2026-06-07 — S1 web-foundation-auth で確立

S1 で CSRF を実装し、§8 の「要確認」を以下に確定した。**後続セッションはこの実装を踏襲する。**

**採用ライブラリ: `github.com/gorilla/csrf` v1.7+。** gin 専用の `utrack/gin-csrf` は `gin-contrib/sessions` 依存で scs と二重セッション化するため不採用。`justinas/nosurf` 等でも可だが、フォーム値とヘッダの双方からトークンを読める gorilla/csrf を採用した。

**(1) Web ルートグループ限定で適用し、デバイス取込 API は CSRF 対象外にする**

gorilla/csrf（net/http ミドルウェア）を gin に適応し、**Web ルートグループにのみ**適用する。`/api`（Bearer・機械間）に適用すると POST 取込が 403 で止まるため**必ず除外**する。

```go
// internal/middleware/csrf.go — gorilla/csrf を gin に適応（Web グループ限定）
func CSRF(cfg *config.Config) gin.HandlerFunc {
    isProd := cfg.AppEnv == "production"
    protect := csrf.Protect(
        csrfAuthKey(cfg.SessionSecret), // (2) 参照
        csrf.Secure(isProd),            // 開発(HTTP)は Secure cookie 無効
        csrf.Path("/"),
        csrf.SameSite(csrf.SameSiteLaxMode),
    )
    return func(c *gin.Context) {
        var passed bool
        wrapped := protect(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
            passed = true
            c.Request = r // csrf トークンを載せた context を後段ハンドラへ伝播
            c.Next()
        }))
        req := c.Request
        if !isProd {
            req = csrf.PlaintextHTTPRequest(req) // ★ (3) 開発(HTTP)の Origin/Referer 強制を回避
        }
        wrapped.ServeHTTP(c.Writer, req)
        if !passed {
            c.Abort() // 403 は gorilla/csrf が書き込み済み
        }
    }
}
```

```go
// cmd/server/main.go — CSRF は Web グループのみ。/api は DeviceAuth のみで CSRF 非適用。
web := engine.Group("/", middleware.SessionLoad(sm), middleware.CSRF(cfg))
api := engine.Group("/api", deviceAuth) // CSRF 対象外
```

**(2) authKey は `SESSION_SECRET` から SHA-256 で 32 バイト導出する**

gorilla/csrf は 32 バイトの authKey を要求する。一方 **scs はセッション cookie を署名せず**（不透明なランダムトークンを使う）`SESSION_SECRET` を消費しない。そこで `SESSION_SECRET` を CSRF の authKey に転用する。SHA-256 で畳み込めば任意長（開発で 32 文字未満でも）安全に 32 バイトになる。

```go
func csrfAuthKey(secret string) []byte {
    sum := sha256.Sum256([]byte(secret))
    return sum[:]
}
```

**(3) ⚠️ 落とし穴: gorilla/csrf は既定でリクエストを HTTPS とみなす**

gorilla/csrf v1.7.x は状態変更リクエストに対し **Origin ヘッダの同一オリジン検証**（Origin が無ければ **Referer 必須**・cleartext な http referer は拒否）を強制し、内部で `requestURL.Scheme = "https"` と仮定する。このため **開発環境（HTTP localhost）では全 POST が 403（`"referer not supplied"`）になる**。本番以外で `csrf.PlaintextHTTPRequest(r)` を適用して回避する（上記コードの ★）。本番（HTTPS）はブラウザが送る Origin/Referer により同一オリジン検証が機能する。TLS 終端プロキシ構成でホストが異なる場合は `csrf.TrustedOrigins([]string{...})` を追加する。

**(4) hidden フィールド名 / ヘッダ名（gorilla 既定）**

- **非 HTMX フォーム**（login / register / logout 等）: `<input type="hidden" name="gorilla.csrf.Token" value={ csrfToken }/>`（gorilla 既定のフィールド名）。Handler は `csrf.Token(c.Request)` でトークンを取得して templ へ引数で渡す。
- **HTMX**: §8 の `<meta name="csrf-token">` + `htmx:configRequest`（`X-CSRF-Token` ヘッダ）。gorilla 既定の検証ヘッダが `X-CSRF-Token` なのでそのまま整合する（追加設定不要）。

**(5) ミドルウェア合成順（重要）**

Gin は**ミドルウェア実行前に HTTP メソッドでルーティングを解決する**。そのため `_method` によるメソッド上書き（§3/§4）と scs `LoadAndSave` は `gin.Engine` の**外側（http.Handler 層）**で合成する必要がある（engine 内の gin ミドルウェアでは間に合わない）。CSRF は逆に「`/api` を除外したい」ため engine 内の Web グループ限定 gin ミドルウェアにする。

```go
// cmd/server/main.go — 合成順（外側 → 内側）
//   MethodOverride(http層) → scs.LoadAndSave(http層) → gin.Engine
//     └ engine 内の Web グループ: SessionLoad → CSRF → ハンドラ
return middleware.MethodOverride(sm.LoadAndSave(engine))
```

| 関心事 | 配置層 | 理由 |
|---|---|---|
| メソッド上書き（`_method`） | http.Handler 層（engine 外） | ルーティング前に `r.Method` を書き換える必要がある |
| セッション load/save（scs `LoadAndSave`） | http.Handler 層（engine 外） | net/http ミドルウェア。context 伝播と commit を engine 全体に被せる |
| CSRF（gorilla/csrf アダプタ） | gin ミドルウェア（Web グループ限定） | `/api`（Bearer）を除外するため |
| SessionLoad（user_id を gin context へ橋渡し）/ RequireAuth | gin ミドルウェア | Web グループ・保護ルートに付与 |

> **メソッド上書きの注意:** `_method` は form 値のため `r.PostFormValue("_method")` が body を解析する。urlencoded では `ParseForm` の結果がキャッシュされ、後段の `ShouldBind`（form binding）でも読めるため二重解析の問題は起きない。

---

## 9. `HX-Redirect` 使用方針

> **cc-sdd への価値:**
> HTMX リクエストに対して通常のリダイレクト（`c.Redirect`）は効かない（HTMX は HTML を swap するため）。`HX-Redirect` レスポンスヘッダーを使う必要があるが、どの操作で使うかはプロジェクト固有の設計決定。

```go
// デバイス編集画面からの削除（例: DeviceHandler.Delete）
func (h *DeviceHandler) Delete(c *gin.Context) {
    deviceID, err := strconv.ParseInt(c.Param("device"), 10, 64)
    if err != nil {
        c.String(http.StatusBadRequest, "不正なデバイスIDです")
        return
    }

    if err := h.repo.DeleteDevice(c.Request.Context(), deviceID); err != nil {
        c.String(http.StatusInternalServerError, "デバイスの削除に失敗しました")
        return
    }

    if c.GetHeader("HX-Request") != "" {
        // 削除後は編集画面が存在しなくなるためダッシュボードへ遷移
        c.Header("HX-Redirect", "/dashboard")
        c.Status(http.StatusOK)
        return
    }
    // 通常フォーム送信時は 303 See Other で PRG
    c.Redirect(http.StatusSeeOther, "/dashboard")
}
```

**本プロジェクトでの使用箇所:**

| 操作 | リダイレクト先 | 理由 |
|------|-------------|------|
| 一覧（履歴・計測値）からの削除 | 同一覧画面 | 一覧領域のサブコンポーネントを再描画して返却できるため `HX-Redirect` は不要。サブコンポーネント返却を推奨 |
| デバイス編集画面からの削除 | `/dashboard` | 編集画面が存在しなくなるためリダイレクトが必要 |

> **原則:** 一覧画面での削除は `HX-Redirect` を使わず、削除後の一覧サブコンポーネント（削除された行を除いた最新テーブル）を返却する方が自然。`HX-Redirect` はページ全体の遷移が必要な場合のみ使用する。

> ⚠️ **`HX-Redirect` は `htmx:responseError` を抑制する（別プロジェクト〈Laravel〉KDCS_PRJ-265 バグ⑦の設計判断・📅 2026-06-25 移植）。**
> htmx は `HX-Redirect` / `HX-Refresh` ヘッダを `responseHandling`（swap/error 判定）より**前**に処理し、即 `location.href` で遷移して return する（htmx 2.x `handleAjaxResponse`）。
> そのため `HX-Redirect` を付けたレスポンスでは `htmx:responseError` が**発火せず**、§14 のトーストは出ない。
> 「トーストで理由を見せてから遷移したい」ケース（例: 401 セッションタイムアウト）は、`HX-Redirect` を使わず、フロントの `htmx:responseError` 側でトースト表示＋`setTimeout` 遷移を行う（§14）。逆に「即時に確実な遷移」だけで良ければ `HX-Redirect` が最も堅牢。

### 9.1 `HX-Redirect` 後に成功メッセージを出すには「遷移前に session へ flash」する

> 📅 2026-06-25 — 別プロジェクト〈Laravel〉KDCS_PRJ-265 バグ⑥を Go/scs 向けに翻訳。保存後に別画面（一覧・ダッシュボード等）へ `HX-Redirect` しつつ「保存しました」を出したいケースの定石。

`HX-Redirect` を返すレスポンスは**本文なしの 200（または 204）であり、リダイレクトレスポンスではない**。`HX-Redirect` はブラウザのフルナビゲーション（`location.href` による新規 GET）を発火させるため、成功／失敗メッセージを遷移先で出すには、**レスポンスを返す前に session へ値を flash しておく**。

Go（scs）では flash は専用 API ではなく「`Put` して、遷移先で `PopString`（読み取り即削除）」で実現する。scs の `LoadAndSave`（本プロジェクトは `SessionLoad`）が応答時に store へ確定するため、200/204 + `HX-Redirect` でも値は遷移先へ運ばれる。`internal/auth/session_auth.go` の既存ヘルパ（`Login` / `Logout` / `UserIDFromSession`）に倣い、**`auth.PutFlash(ctx, sm, msg)` / `auth.PopFlash(ctx, sm)` を追加して session キーを散らさない**（現状未実装。反映時に追加する）。

```go
// 保存ハンドラ: HX-Redirect の前に flash を Put する（HTMX・通常 POST どちらの経路でも遷移前に）
func (h *DeviceHandler) Update(c *gin.Context) {
    // ... 保存処理 ...
    auth.PutFlash(c.Request.Context(), h.sm, "デバイスを保存しました。") // ← 遷移前に flash（内部は sm.Put）

    if c.GetHeader("HX-Request") != "" {
        c.Header("HX-Redirect", "/dashboard")
        c.Status(http.StatusOK) // 本文なし。ヘッダに ->with 相当は載らないので flash で運ぶ
        return
    }
    c.Redirect(http.StatusSeeOther, "/dashboard") // 通常 POST も同じ flash を読む
}

// 遷移先（/dashboard 等）ハンドラ: PopString で 1 回だけ読み出し、共通バナーへ渡す
flash := auth.PopFlash(c.Request.Context(), h.sm) // 内部は sm.PopString（読み取り後に自動削除）
// → App.templ 共通バナーに flash を渡して表示
```

> **注意:** `HX-Redirect` のヘッダだけを載せて flash を忘れると、遷移はするが成功メッセージは出ない。`c.Redirect`（通常）と `HX-Redirect`（HTMX）の**どちらの経路でも遷移前に同じ flash を Put** しておけば、どちらで送信されても表示される。要点は「遷移が `c.Redirect` か `HX-Redirect` か」に関わらず **flash の Put を遷移前に必ず行う**こと。なお保存後に同じ画面へ留まる（遷移しない）場合は、flash ではなく描画する view に直接メッセージを渡せばよい。

---

## 10. ページネーションの HTMX 統合方針

> **cc-sdd への価値:**
> Go では Laravel の `$paginator->links()` のようなページャ生成機構は無い。sqlc の `LIMIT`/`OFFSET` クエリで 1 ページ分を取得し、ページ番号・総件数から自前のページネーション templ コンポーネントを描画する。HTMX で部分更新するにはページリンクに `hx-get` / `hx-target` / `hx-swap` を明示する。

**採用方針:** sqlc 生成クエリで `LIMIT 20 OFFSET ?` を発行し、`COUNT(*)` で総件数を取得。`Pagination` templ コンポーネントが現在ページ・総ページ数を受け取り、各リンクに `hx-get` を設定して一覧領域だけを差し替える。Laravel paginator は存在しないため移植しない。

**ページング件数:** 1 ページ 20 件。

```sql
-- internal/repository/queries/readings.sql（sqlc）
-- name: ListReadingsByDevice :many
SELECT id, device_id, metric, value, measured_at
FROM readings
WHERE device_id = ?
ORDER BY measured_at DESC
LIMIT ? OFFSET ?;

-- name: CountReadingsByDevice :one
SELECT COUNT(*) FROM readings WHERE device_id = ?;
```

```go
// internal/handler/reading_handler.go
const perPage = 20

func (h *ReadingHandler) List(c *gin.Context) {
    deviceID, _ := strconv.ParseInt(c.Param("device"), 10, 64)
    page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
    if page < 1 {
        page = 1
    }
    offset := int64((page - 1) * perPage)

    rows, err := h.repo.ListReadingsByDevice(c.Request.Context(), repository.ListReadingsByDeviceParams{
        DeviceID: deviceID,
        Limit:    perPage,
        Offset:   offset,
    })
    if err != nil {
        c.String(http.StatusInternalServerError, "計測値の取得に失敗しました")
        return
    }
    total, _ := h.repo.CountReadingsByDevice(c.Request.Context(), deviceID)
    lastPage := int((total + perPage - 1) / perPage)

    // HTMX 時は一覧サブコンポーネントのみ、通常時はフルページ
    if c.GetHeader("HX-Request") != "" {
        component.ReadingsList(deviceID, rows, page, lastPage).
            Render(c.Request.Context(), c.Writer)
        return
    }
    page.Readings(deviceID, rows, page, lastPage).
        Render(c.Request.Context(), c.Writer)
}
```

```templ
// internal/view/component/pagination.templ
// 自前ページネーション。各リンクは一覧領域を hx-get で差し替える。
templ Pagination(baseURL string, currentPage, lastPage int, targetID string) {
    if lastPage > 1 {
        <div class="pagination">
            if currentPage > 1 {
                <a class="period-btn"
                   hx-get={ fmt.Sprintf("%s?page=%d", baseURL, currentPage-1) }
                   hx-target={ "#" + targetID }
                   hx-swap="innerHTML">前へ</a>
            }
            <span>{ fmt.Sprint(currentPage) } / { fmt.Sprint(lastPage) }</span>
            if currentPage < lastPage {
                <a class="period-btn"
                   hx-get={ fmt.Sprintf("%s?page=%d", baseURL, currentPage+1) }
                   hx-target={ "#" + targetID }
                   hx-swap="innerHTML">次へ</a>
            }
        </div>
    }
}
```

> ⚠️ **上記 `Pagination` は「前へ / 次へ」のみ・ターゲット id 固定**。番号リンク（1,2,3…）を要するモック（例 alert-history）には不足する。**番号付きページャは番号リンク配列を持つ別部品**として実装する（§64.2「番号付きページネーション部品」参照）。既存 `Pagination` を汎用化すると兄弟画面（readings）へ回帰リスクがあるため、新規部品にして影響を局所化する。

**本プロジェクトでのページネーション使用箇所:**

| 画面 | ページネーション対象 | ターゲット id | 形式 |
|------|-----------------|------------|------|
| 計測値一覧 `/devices/{device}/readings` | センサー計測（reading） | `#device-readings-list` | 前/次のみ（`Pagination`） |
| アラート履歴 `/alerts/history` | アラート通知（alert） | `#alert-history-list` | 番号付き（`AlertHistoryPagination` §64.2） |

### 10-B: 選択モーダル内のページネーション手動構築

一覧画面ではサブコンポーネント差し替えで十分だが、**選択モーダル（サブコンポーネント返却）** では以下の制約がある:

1. モーダル内のページリンクは、ターゲット（`#modal-content`）とフラグメント境界を明示しないとフルページ遷移に戻りがち
2. ページ遷移時に検索パラメータ・選択状態を `hx-vals` / `hx-include` で明示的に含める必要がある

**対策:** `pagination` > `period-btn` の CSS パターンで手動構築し、各ページリンクに `hx-get` + `hx-target="#modal-content"` + `hx-vals` を設定する。本プロジェクトでは選択モーダルの利用が限定的なため、必要になった時点で軽量に組む。

```templ
// 選択モーダル内のページネーション例（汎用形）
templ ModalPagination(baseURL string, currentPage, lastPage int, params map[string]any) {
    if lastPage > 1 {
        <div class="pagination">
            if currentPage > 1 {
                <a class="period-btn"
                   hx-get={ baseURL }
                   hx-target="#modal-content"
                   hx-vals={ hxValsJSON(params, currentPage-1) }>&lsaquo;</a>
            } else {
                <span class="period-btn disabled">&lsaquo;</span>
            }
            // ページ番号は省略
        </div>
    }
}
```

**ポイント:**
- `hx-vals` に渡す値は `encoding/json` で安全に JSON 化する（属性へ生 JSON を連結しない）。ヘルパー `hxValsJSON` が検索パラメータ・表示件数・選択状態（`selected_device_id` 等）・遷移先ページ番号をまとめて `json.Marshal` する。
- `selected_device_id` を含めることで、ページ遷移後も選択行のハイライトが維持される。

---

## 10-C: Alpine.js 値渡しの JSON 化パターン 【セキュリティ】

Alpine.js の `x-data` や `@click` で Go 側の値を JavaScript に渡す際、文字列を素朴に連結すると**XSS 脆弱性を残す**。templ では値を `encoding/json` で `json.Marshal` し、`templ.JSONString` 経由で属性に埋め込むことで、JSON エンコード + 属性エスケープを同時に行う。

```templ
// NG: 文字列連結は引用符・タグ混入を防げない
<button @click={ "selectedRow = { id: " + fmt.Sprint(device.ID) + ", name: '" + device.Name + "' }" }>選択</button>

// OK: templ.JSONString で JSON 化 + エスケープ
<button @click={ "selectedDevice = " + templ.JSONString(map[string]any{
    "id":   device.ID,
    "name": device.Name,
}) }>選択</button>
```

`templ.JSONString` は内部で `json.Marshal` を行い、templ の属性式は出力を自動的に HTML 属性エスケープする。`<script>` タグや引用符を含む文字列も安全に渡せる。`x-data` の初期値設定にも使用可能:

```templ
templ DeviceSelector(selectedID *int64, selectedName string) {
    <div x-data={ "{ selectedDevice: " + selectedDeviceInit(selectedID, selectedName) + " }" }>
        // ...
    </div>
}

// selectedID が nil なら null、あれば JSON オブジェクトを返す
func selectedDeviceInit(id *int64, name string) string {
    if id == nil {
        return "null"
    }
    return templ.JSONString(map[string]any{"id": *id, "name": name})
}
```

> **セキュリティ原則:** Alpine.js へ渡す動的値は必ず `encoding/json` 経由で生成する。文字列連結や独自エスケープ関数（旧 Laravel コードの `addslashes()` 相当）は HTML エンティティ・タグ閉じを処理できないため使わない。

---

## 10-D: フルサブコンポーネント swap 時の選択状態サーバーサイド往復

検索・ソート・ページネーション・タブ切替でモーダルやパネルのサブコンポーネント全体を swap する場合、Alpine.js の `x-data` は毎回サーバーから再初期化される。選択行を保持するには、**hidden input でサーバーに往復させる**。

```templ
// hidden input: Alpine.js の選択値を常に HTMX リクエストに含める
<input type="hidden" name="selected_device_id" x-bind:value="selectedDevice?.id || ''">

// 検索ボタン: hx-include に selected_device_id を含める
<button hx-get="/devices/select" hx-target="#modal-content"
        hx-include="[name='keyword'], [name='selected_device_id']">検索</button>

// タブ変更時も選択を維持したいなら hx-include に含める
<select hx-get="/devices/select" hx-target="#modal-content"
        hx-include="[name='tab']">...</select>
```

**Handler 側:** `selected_device_id` を受け取り、該当デバイスが現在の閲覧コンテキスト（ログインユーザーの所有デバイス等）に属するか検証してから templ に渡す。

```go
func (h *DeviceHandler) Select(c *gin.Context) {
    selectedID, _ := strconv.ParseInt(c.Query("selected_device_id"), 10, 64)

    var selectedName string
    if selectedID > 0 {
        d, err := h.repo.GetDevice(c.Request.Context(), selectedID)
        // 安全策: 他ユーザーのデバイスや存在しない ID はクリア
        if err != nil || d.OwnerID != currentUserID(c) {
            selectedID = 0
        } else {
            selectedName = d.Name
        }
    }
    // selectedID / selectedName を選択モーダルの templ コンポーネントへ明示的に渡す
    component.DeviceSelectModal(selectedID, selectedName /* , ... */).
        Render(c.Request.Context(), c.Writer)
}
```

> **ポイント:** templ には共有エラーバッグや暗黙の state 引き継ぎが無い。選択状態は hidden input → クエリ/フォーム → Handler → templ 引数の経路で明示的に往復させる。

---

## 10-E: `<script type="application/json">` には `json.Marshal` を使う

JavaScript から動的に読み取りたいデータを、templ 上の `<script type="application/json">` タグに埋め込む。Go では `encoding/json` の `json.Marshal` でシリアライズした **純 JSON 文字列** を `@templ.Raw` で出力し、Alpine.js / 素の JS は `JSON.parse` で読む。属性へ生 JSON を連結する方式（§10-C の `x-data` 直書き）とは目的が異なるので使い分ける。

> **重要:** この方式は姉妹文書「HTMLモック作成ルール.md」の **TS** 系（テンプレート構造ルール）と整合する。データ供給は属性連結ではなく専用の JSON script タグに集約する。

### 問題（やってはいけない）

`x-data` などの**式として評価される文脈向けの値**（`JSON.parse('...')` のような JS 式を吐く形式）を、そのまま `<script type="application/json">` の中身に流し込むと、`textContent` を `JSON.parse()` する側で先頭の `JSON.parse(` が構文エラーになり catch で空配列に落ちる。症状として「JS ロジックは動いているのに値が反映されない」現象が出る。`<script type="application/json">` の中身は**純 JSON でなければならない**。

### 対策: `json.Marshal` の結果をそのまま埋め込む

```templ
// internal/view/component/readings_chart.templ
// グラフ描画用の計測値を純 JSON で供給する
templ ReadingsChartData(rows []ReadingPoint) {
    <script type="application/json" id="readings-chart-data">
        @templ.Raw(marshalReadings(rows))
    </script>
}
```

```go
// internal/view/component/readings_chart_helper.go
type ReadingPoint struct {
    MeasuredAt string  `json:"measured_at"`
    Metric     string  `json:"metric"`     // temperature / humidity
    Value      float64 `json:"value"`
}

// json.Marshal で純 JSON を生成する。
// HTML 特殊文字（< > & 等）は encoding/json が \uXXXX へエスケープするため
// </script> 混入による XSS は起きない（json.Marshal のデフォルト挙動）。
func marshalReadings(rows []ReadingPoint) string {
    b, err := json.Marshal(rows)
    if err != nil {
        return "[]"
    }
    return string(b)
}
```

**XSS 対策の要点（Go の場合）:**
- `encoding/json` は標準で `<`, `>`, `&` を `<` `>` `&` にエスケープする（`SetEscapeHTML(true)` がデフォルト）。これにより `</script>` のタグ閉じ混入を防げる。`json.Marshal` をそのまま使えば追加対策は基本不要。
- 日本語は `\uXXXX` でエスケープされても `JSON.parse` で正しく復元されるため可読性以外の実害はない。生の日本語で出したい場合は `json.Encoder` + `SetEscapeHTML(true)` を維持したまま扱う（HTML エスケープは外さない）。

### 使い分けルール

| 文脈 | 推奨 | 理由 |
|------|------|------|
| Alpine.js `x-data` / `@click` 属性 | **`templ.JSONString`**（§10-C） | 属性式として評価され、属性エスケープもかかる |
| HTMX `hx-vals` | **`encoding/json`** で生成した文字列 | 属性へ純 JSON を渡す（生連結しない） |
| **`<script type="application/json">` の中身** | **`json.Marshal` + `@templ.Raw`** | 純 JSON が必要。`JSON.parse` で読む |

### 検出方法（テスト）

ハンドラの出力 HTML を文字列で検査する。

```go
// 計測値データの script タグが存在し、中身が純 JSON であること
body := w.Body.String()
assert.Contains(t, body, `id="readings-chart-data"`)
// JSON.parse( で始まっていないこと（= JS 式が紛れていない）
assert.NotContains(t, body, "JSON.parse(")

// パースできることも確認
start := strings.Index(body, `id="readings-chart-data">`) + len(`id="readings-chart-data">`)
var points []ReadingPoint
assert.NoError(t, json.Unmarshal([]byte(extractScriptBody(body, start)), &points))
```

JS 側は `JSON.parse(document.getElementById('readings-chart-data').textContent)` が例外を投げないことを実機確認する。

---

## 10-F: セッションベースのトグル状態管理パターン

一覧画面で「→ 追加 / ← 削除」のように、ユーザーがクリックごとに選択集合を積み上げる UI のサーバーサイド状態管理パターン。例として「監視対象に追加したデバイス」パネルで使う。状態は **scs セッション**（`internal/auth/session_auth.go` 経由で取得する `*scs.SessionManager`）に保持する。

### 設計方針

- **セッションキーに ID 配列を保持**（scs の `session.Get` / `session.Put`）
- **初回アクセス時のみ自動抽出ロジックで初期化**（例: ログインユーザーが所有する稼働中デバイスを自動 pick）
- **以降は 追加/削除トグルのエンドポイント** で配列を更新
- レスポンスは右パネルのサブコンポーネント HTML + `HX-Trigger` で他要素に同期

### Handler 実装

```go
const watchedDeviceIDsKey = "watched_device_ids"

// セッションから監視対象デバイス ID 配列を解決する。
// 初回は自動抽出した ID でセッションを初期化する。
func (h *WatchHandler) resolveWatchedIDs(c *gin.Context) []int64 {
    ctx := c.Request.Context()
    if !h.session.Exists(ctx, watchedDeviceIDsKey) {
        autoIDs := h.buildAutoWatchedIDs(ctx) // 稼働中デバイスを自動 pick
        h.session.Put(ctx, watchedDeviceIDsKey, autoIDs)
        return autoIDs
    }
    ids, _ := h.session.Get(ctx, watchedDeviceIDsKey).([]int64)
    return ids
}

// 監視対象のトグル。配列を更新して右パネルのサブコンポーネントを返す。
func (h *WatchHandler) Toggle(c *gin.Context) {
    deviceID, err := strconv.ParseInt(c.Param("device"), 10, 64)
    if err != nil {
        c.String(http.StatusBadRequest, "不正なデバイスIDです")
        return
    }
    ctx := c.Request.Context()
    ids := h.resolveWatchedIDs(c)

    if contains(ids, deviceID) {
        ids = remove(ids, deviceID) // 新しいスライスを返す（破壊的変更を避ける）
    } else {
        ids = append(append([]int64{}, ids...), deviceID)
    }
    h.session.Put(ctx, watchedDeviceIDsKey, ids)

    // HX-Trigger で左一覧の行色・アイコンを一斉同期
    payload, _ := json.Marshal(map[string]any{
        "watched-changed": map[string]any{"ids": ids},
    })
    c.Header("HX-Trigger", string(payload))
    component.WatchedPanel(ids /* , ... */).Render(ctx, c.Writer)
}
```

### templ / Alpine.js 実装

```templ
templ DeviceList(devices []Device, watchedIDs []int64) {
    <div x-data={ "{ watchedIds: " + templ.JSONString(watchedIDs) + ", " +
        "isWatched(id) { return this.watchedIds.map(v => Number(v)).includes(Number(id)); } }" }
        @watched-changed.window="watchedIds = ($event.detail.ids ?? []).map(v => Number(v))">
        for _, d := range devices {
            <tr x-bind:class={ "isWatched(" + fmt.Sprint(d.ID) + ") ? 'status-active' : ''" }>
                // ...
                <td>
                    <button class="btn btn-small"
                        hx-post={ fmt.Sprintf("/devices/%d/watch-toggle", d.ID) }
                        hx-target="#watched-panel"
                        hx-swap="outerHTML">
                        <span x-show={ "!isWatched(" + fmt.Sprint(d.ID) + ")" }>→</span>
                        <span x-show={ "isWatched(" + fmt.Sprint(d.ID) + ")" } x-cloak>✓</span>
                    </button>
                </td>
            </tr>
        }
    </div>
    // Toggle から返されるサブコンポーネントで置換される
    <div id="watched-panel"></div>
}
```

### 特性

| 項目 | 挙動 |
|------|------|
| スコープ | scs セッション単位（ログアウトで消失） |
| 永続化 | なし（必要なら中間テーブル `watched_devices` に移行） |
| OOB swap | 不要（HX-Trigger で Alpine.js 同期） |
| 一斉同期 | 左一覧 N 行の表示状態が 1 イベントで更新 |

### 「初期化のタイミング」の選択

- **初回アクセスで自動抽出**（上記例）: 既存状態の再現性あり、ログアウトで消える
- **都度再計算**: 自動抽出 + セッション追加の合算表示も可能（ただし「取り消し」が効かないので非推奨）
- **毎回空から**: ユーザーが明示的に追加のみ

> **補足:** このパターンは「片方の HTMX レスポンスで多数の UI 要素（一覧の行色・アイコン切替）を `HX-Trigger` だけで一斉更新する」多要素状態同期の例。単一イベント受信のみで足りる場合は OOB swap も `HX-Trigger` も使わず、対象領域の直接 swap で十分。

---

## 10-G: `@templ.Raw` に流してよいのは自前生成文字列だけ（SVG/自前 HTML・XML の安全条件） 📅 2026-06-08 — device-detail で確立

> **cc-sdd への価値:**
> §10-E は `<script type="application/json">` 向けに `json.Marshal` + `@templ.Raw` を扱うが、安全性は `encoding/json` の HTML エスケープ依存。SVG など**自前生成の HTML/XML 文字列**を `@templ.Raw` で埋め込む場合は安全条件が別になる。device-show の温度/湿度グラフ（サーバー生成 SVG）に適用。

### 原則

`@templ.Raw` は出力を一切エスケープしない。流してよいのは**自分がコードで組み立てた文字列だけ**であり、**ユーザー入力（DB 値・クエリ・フォーム）を 1 バイトも Raw に入れない**。これを「気をつける」運用ではなく**層分離で構造的に担保**する。

### 確立した3条件（SVG 生成パッケージ）

1. **生成パッケージを最下流の純粋ユーティリティにする。** `internal/chart` は stdlib（`strings.Builder`/`fmt`/`strconv`）のみに依存し、`gin`・DB・`templ`・`pgtype` を import しない（structure.md 最下流）。入力は整形済みの `float64` 点列（`chart.Point{Label, Y}`）に限定し、**`pgtype` 変換（`pgconv.NumericToFloat` 等）は handler の責務**とする。これにより「外部入力の生値」がそもそもパッケージ境界を越えない。

2. **出力は外部入力を含まない安全な自前生成文字列のみ。** a11y 用 `<title>` や凡例名など、引数由来の文字列をマークアップに組む箇所は、外部入力を想定しない設計でも**防御的に XML 最小エスケープ（`&` `<` `>`）してから**組む。

   ```go
   // internal/chart/svg.go
   func esc(s string) string { return xmlEscaper.Replace(s) }
   var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
   // 例: fmt.Fprintf(b, `<title>%s</title>`, esc(title))
   ```

3. **テーブル・情報パネルのユーザー値は `@templ.Raw` を通さない。** 計測値テーブルやデバイス情報の DB 由来文字列は templ 既定の `{ }` エスケープ経路に乗せる。`@templ.Raw` を使うのは**自前生成 SVG だけ**に閉じる。

   ```templ
   // OK: SVG（自前生成）だけ Raw
   <div id="temperature-chart" class="chart-wrapper">
       @templ.Raw(v.TemperatureSVG)
   </div>
   // ユーザー値は { } 既定エスケープ（Raw を使わない）
   <td>{ r.RecordedAt }</td>
   <td>{ r.Temp }</td>
   ```

### 使い分け（§10-E / §59 との関係）

| 対象 | 手段 | 安全の根拠 |
|------|------|-----------|
| `<script type="application/json">` の中身 | `json.Marshal` + `@templ.Raw`（§10-E） | `encoding/json` が `< > &` をエスケープ |
| **自前生成 SVG / HTML・XML 文字列** | **生成パッケージ + `@templ.Raw`（本節）** | **外部入力を境界で遮断 + 引数文字列は防御的 XML エスケープ** |
| ユーザー値（計測値・デバイス名等） | templ 既定 `{ }`（§59） | `{ }` が常に HTML エスケープ。Raw 直書きは非推奨 |

> §59 は `@templ.Raw(value + "...")` を「XSS リスクあり・非推奨」とユーザー値側で禁じる。本節はその裏返しで、**Raw が許容される積極的条件**（= 自前生成のみ・層分離で外部入力を遮断）を定める。

### 検出方法（テスト）

```go
// chart パッケージ: title に < > & を渡しても XML エスケープされること
got := chart.LineChartSVG("a<b&c", "℃", series)
if !strings.Contains(got, "a&lt;b&amp;c") {
    t.Errorf("title が XML エスケープされていない: %s", got)
}
// component: SVG が生のまま埋め込まれること（@templ.Raw 経路）
if !strings.Contains(body, "<svg") {
    t.Error("SVG が埋め込まれていない")
}
```

## 11. 削除確認方針

> **cc-sdd への価値:**
> 削除操作の確認方法はプロジェクト固有の設計決定。Alpine.js モーダルか `hx-confirm` かで templ テンプレートの構造が変わる。

**採用方針:** `hx-confirm` 属性（HTMX 組み込み）を使用する。簡易な削除確認では Alpine.js カスタムモーダルは使わない。デバイス削除のような重い操作は `.modal-overlay` + `.modal-content` の削除確認モーダルを別途用意する（§後述のインライン編集と同様、画面設計に従う）。

```templ
<button type="button"
  hx-delete={ fmt.Sprintf("/alerts/rules/%d", rule.ID) }
  hx-target="#alert-rule-list"
  hx-confirm={ fmt.Sprintf("「%s」のアラートルールを削除しますか？", rule.Name) }>
  削除
</button>
```

**ルール:**
- 確認メッセージは日本語とし、削除対象の名前を含める。
- 一覧画面・編集画面・インライン CRUD すべての削除操作に適用する。

---

## 12. インライン CRUD 方針

> **cc-sdd への価値:**
> アラートルール設定 `/alerts/rules` でのインライン追加・編集・削除の HTMX パターン。ルールの追加・編集・削除エンドポイントと対応する。

### 行追加パターン

```templ
// 追加フォーム: テーブル下部に配置
<form class="rule-form"
      hx-post="/alerts/rules"
      hx-target="#alert-rule-list"
      hx-swap="innerHTML">
  <select name="metric">
    <option value="temperature">温度（℃）</option>
    <option value="humidity">湿度（%）</option>
  </select>
  <select name="operator">
    <option value=">">&gt;</option>
    <option value="<">&lt;</option>
    <option value=">=">&gt;=</option>
    <option value="<=">&lt;=</option>
  </select>
  <input type="number" name="threshold" step="0.1">
  <button type="submit" class="btn btn-primary btn-small">追加</button>
</form>

// 一覧テーブル（サブコンポーネントとして分離）
templ AlertRuleList(rules []AlertRule) {
  <div id="alert-rule-list" class="rule-list">
    <table class="data-table">
      <thead>...</thead>
      <tbody>
        for _, rule := range rules {
          <tr>
            <td>{ metricLabel(rule.Metric) }</td>
            <td>{ rule.Operator }</td>
            <td>{ fmt.Sprintf("%.1f", rule.Threshold) }</td>
            <td>
              <button type="button" class="btn btn-danger btn-small"
                      hx-delete={ fmt.Sprintf("/alerts/rules/%d", rule.ID) }
                      hx-target="#alert-rule-list"
                      hx-confirm="このアラートルールを削除しますか？">削除</button>
            </td>
          </tr>
        }
      </tbody>
    </table>
  </div>
}
```

`/alerts/rules` のフルページ templ が `AlertRuleList` を呼び、HTMX 時は Handler が `AlertRuleList` を直接 `.Render(c.Request.Context(), c.Writer)` する。

### 行編集パターン

```templ
// 編集モードの行
<tr>
  <td colspan="4">
    <form class="rule-form"
          hx-put={ fmt.Sprintf("/alerts/rules/%d", rule.ID) }
          hx-target="#alert-rule-list"
          hx-swap="innerHTML">
      <select name="metric">
        <option value="temperature" selected?={ rule.Metric == "temperature" }>温度（℃）</option>
        <option value="humidity" selected?={ rule.Metric == "humidity" }>湿度（%）</option>
      </select>
      <input type="number" name="threshold" step="0.1" value={ fmt.Sprintf("%.1f", rule.Threshold) }>
      <button type="submit" class="btn btn-primary btn-small">保存</button>
      <button type="button" class="btn btn-secondary btn-small"
              hx-get="/alerts/rules"
              hx-target="#alert-rule-list">キャンセル</button>
    </form>
  </td>
</tr>
```

**Handler のレスポンス:** 追加・編集・削除いずれの場合も、更新後の一覧テーブル全体（`AlertRuleList` サブコンポーネント）を返却する。個別行の差し替えではなくテーブル全体を返すことで、件数表示等の整合性を保つ。

> **更新（PUT）では「フォームに無いフィールド」を現在 DB 値から読み戻して保全する 📅 2026-06-08 — alert-rules で確立。** 上の行編集フォームは `metric` / `threshold` のみで `is_enabled` を持たない（有効/無効切替は別エンドポイント PATCH `/toggle` で行単位 swap）。このとき更新 Handler がナイーブに `UpdateAlertRuleParams{... IsEnabled: form.IsEnabled}` と書くと、未送信の bool が**ゼロ値 `false`** で渡り、別経路で有効化したルールが編集保存のたびに無効へ巻き戻る（silent regression）。認可で取得済みの現在レコード値をそのまま書き戻す:
>
> ```go
> // rule は RequireAlertRuleOwner が返す現在レコード
> _, err := h.Repo.UpdateAlertRule(ctx, repository.UpdateAlertRuleParams{
>     ID: rule.ID, Metric: form.Metric, Operator: form.Operator,
>     Threshold: pgconv.Numeric2(threshold),
>     IsEnabled: rule.IsEnabled, // ← フォーム外。現在値を保全し toggle の結果を壊さない
> })
> ```
>
> テスト（必須）: 現在 `is_enabled=true` のルールを `is_enabled` を**送らずに** PUT し、`UpdateAlertRuleParams.IsEnabled == true` が維持されることを assert する（フォームに含めると保全を検証できない）。

> **enum 相当の `<select>` 候補は domain 列挙からループ生成する（oneof / DB CHECK と単一ソース化）。** 上の例は `<option>` をハードコードしているが、`metric` / `operator` のような enum（DB は varchar+CHECK・Go は `internal/domain` 型）は binding の `oneof`・DB CHECK・select 候補が同じ許容集合であり、3 箇所にコピペすると enum 追加時に取り残しが出る。`domain.AllMetrics()` / `domain.AllComparisonOperators()` をループし、`value={ string(m) }`（DB 格納値=`oneof` 許容値）・表示は `m.Label()`・選択復元は `selected?={ v.Metric == string(m) }` とする:
>
> ```templ
> <select name="metric">
>   <option value="">選択してください</option>
>   for _, m := range domain.AllMetrics() {
>     <option value={ string(m) } selected?={ v.Metric == string(m) }>{ m.Label() }</option>
>   }
> </select>
> ```

### 行単位 swap パターン（outerHTML swap）

行数が多いテーブルや、行ごとに独立した操作（インライン編集 → 保存/キャンセル、削除）が必要な場合は、`outerHTML` swap で行単位の差し替えを行う。

**適用条件:**
- 行番号や合計値など、他の行に依存する集計表示がない（または集計をクライアントサイドで更新する）場合
- 行ごとに 3 モード切替（readonly / edit / input）が必要な場合

**templ テンプレート構造（3 モードを単一コンポーネントで切替）:**

```templ
// internal/view/component/alert_rule_row.templ
// mode で readonly / edit / input を切替
templ AlertRuleRow(mode string, rule AlertRule) {
    switch mode {
        case "input":
            <tr id="alert-rule-row-new">
                // 新規入力行: metric/operator/threshold 入力
            </tr>
        case "edit":
            <tr id={ fmt.Sprintf("alert-rule-row-%d", rule.ID) }>
                // 編集行: 既存値セット済み + 保存/キャンセル
                <td>
                    <button class="btn btn-primary btn-small"
                            hx-put={ fmt.Sprintf("/alerts/rules/%d", rule.ID) }
                            hx-target={ fmt.Sprintf("#alert-rule-row-%d", rule.ID) }
                            hx-swap="outerHTML"
                            hx-include={ fmt.Sprintf("#alert-rule-row-%d", rule.ID) }>保存</button>
                    <button class="btn btn-secondary btn-small"
                            hx-get={ fmt.Sprintf("/alerts/rules/%d/edit", rule.ID) }
                            hx-target={ fmt.Sprintf("#alert-rule-row-%d", rule.ID) }
                            hx-swap="outerHTML">キャンセル</button>
                </td>
            </tr>
        default:
            <tr id={ fmt.Sprintf("alert-rule-row-%d", rule.ID) }>
                // 読取専用行: 表示のみ + 編集/削除ボタン
                <td>
                    <button class="btn btn-small"
                            hx-get={ fmt.Sprintf("/alerts/rules/%d/edit?mode=edit", rule.ID) }
                            hx-target={ fmt.Sprintf("#alert-rule-row-%d", rule.ID) }
                            hx-swap="outerHTML">編集</button>
                    <button class="btn btn-danger btn-small"
                            hx-delete={ fmt.Sprintf("/alerts/rules/%d", rule.ID) }
                            hx-target={ fmt.Sprintf("#alert-rule-row-%d", rule.ID) }
                            hx-swap="outerHTML"
                            hx-confirm="このアラートルールを削除しますか？">削除</button>
                </td>
            </tr>
    }
}
```

**Handler パターン:**

```go
// GET /alerts/rules/:rule/edit — mode クエリで readonly / edit を切替
func (h *AlertRuleHandler) Edit(c *gin.Context) {
    ruleID, _ := strconv.ParseInt(c.Param("rule"), 10, 64)
    rule, err := h.repo.GetAlertRule(c.Request.Context(), ruleID)
    if err != nil {
        c.String(http.StatusNotFound, "アラートルールが見つかりません")
        return
    }
    mode := "readonly"
    if c.Query("mode") == "edit" {
        mode = "edit"
    }
    component.AlertRuleRow(mode, rule).Render(c.Request.Context(), c.Writer)
}

// PUT /alerts/rules/:rule — バリデーション → 保存 → readonly 行を返却
func (h *AlertRuleHandler) Update(c *gin.Context) {
    ruleID, _ := strconv.ParseInt(c.Param("rule"), 10, 64)

    var in AlertRuleInput
    if err := c.ShouldBind(&in); err != nil {
        // バリデーションエラーは map に変換し、edit モードの行を再描画して返す
        // （templ には共有エラーバッグが無いため引数で明示的に渡す）
        errs := bindErrorsToMap(err)
        component.AlertRuleRowWithErrors("edit", rebuildRule(ruleID, in), errs).
            Render(c.Request.Context(), c.Writer)
        return
    }
    rule, err := h.repo.UpdateAlertRule(c.Request.Context(), /* ... */)
    if err != nil {
        c.String(http.StatusInternalServerError, "アラートルールの更新に失敗しました")
        return
    }
    component.AlertRuleRow("readonly", rule).Render(c.Request.Context(), c.Writer)
}

// DELETE /alerts/rules/:rule — 削除 → 空レスポンス（outerHTML swap で行が消滅）
func (h *AlertRuleHandler) Delete(c *gin.Context) {
    ruleID, _ := strconv.ParseInt(c.Param("rule"), 10, 64)
    if err := h.repo.DeleteAlertRule(c.Request.Context(), ruleID); err != nil {
        c.String(http.StatusInternalServerError, "アラートルールの削除に失敗しました")
        return
    }
    c.String(http.StatusOK, "")
}
```

**テーブル全体返却パターンとの使い分け:**

| 観点 | テーブル全体返却 | 行単位差し替え |
|------|------------------|----------------|
| 集計行の整合性 | 自動維持 | クライアント側で更新が必要 |
| レスポンスサイズ | 大（行数に比例） | 小（1 行分のみ） |
| Tom Select 再初期化 | 全行で発火 | 対象行のみ |
| templ コンポーネント | 一覧サブコンポーネント 1 つ | 行コンポーネント 1 つ（3 モード） |
| 適用イメージ | 件数が少ないアラートルール一覧 | ルールが多く行ごとに独立編集する場合 |

### Alpine.js `editing` フラグによるクライアントサイド編集モード切替

フォーム送信時に親レコードと一括保存したい行集合では、**削除のみ HTMX 即時反映、追加・編集は Alpine.js 配列管理 → フォーム一括保存**のハイブリッドパターンを採る。

**仕組み:** 各行に `editing` プロパティを持たせ、`x-if` で表示モード（readonly span）と編集モード（input）を切り替える。新規追加行は `editing: true` で生成。

```templ
// インライン編集テーブル + Alpine.js x-for
<div x-data={ "{ rules: " + templ.JSONString(initRules) + " }" }>
  <table class="data-table" id="alert-rules-table">
    <thead>
      <tr>
        <th>計測項目</th><th>条件</th><th>しきい値</th><th>操作</th>
      </tr>
    </thead>
    <tbody>
      <template x-for="(row, index) in rules" :key="index">
        <tr>
          <td>
            // 編集モード: select を表示
            <template x-if="row.editing">
              <select x-model="row.metric">
                <option value="temperature">温度（℃）</option>
                <option value="humidity">湿度（%）</option>
              </select>
            </template>
            // 表示モード: readonly span
            <template x-if="!row.editing">
              <span x-text="row.metric === 'temperature' ? '温度（℃）' : '湿度（%）'"></span>
            </template>
          </td>
          <td>
            <template x-if="row.editing">
              <input type="number" step="0.1" x-model="row.threshold">
            </template>
            <template x-if="!row.editing">
              <span x-text="Number(row.threshold || 0).toLocaleString()"></span>
            </template>
          </td>
          <td>
            // 編集/確定トグル
            <button type="button" class="btn btn-small"
                    @click="row.editing = !row.editing">
              <span x-text="row.editing ? '✓' : '編集'"></span>
            </button>
            // DB 保存済み行: HTMX 即時削除
            <button type="button" class="btn btn-danger btn-small"
                    x-show="row.id"
                    @click="if(confirm('削除しますか?')) { rules.splice(index, 1) }"
                    x-bind:hx-delete="'/alerts/rules/' + row.id">✕</button>
            // 未保存行: 配列から除去のみ
            <button type="button" class="btn btn-danger btn-small"
                    x-show="!row.id"
                    @click="rules.splice(index, 1)">✕</button>
          </td>
        </tr>
      </template>
    </tbody>
  </table>
  // 追加ボタン: editing: true で新規行を配列に追加
  <button type="button" class="btn btn-secondary btn-small"
          @click="rules.push({id: null, metric: 'temperature', operator: '>', threshold: '', editing: true})">
    ルールを追加
  </button>

  // 一括保存用 JSON hidden input
  <input type="hidden" name="rules_json" x-bind:value="JSON.stringify(rules)">
</div>
```

**HTMX 即時保存型との使い分け:**

| 観点 | HTMX 即時保存型 | Alpine.js 一括保存型 |
|------|-----------------|----------------------|
| 保存タイミング | 行ごとに即時 DB 反映 | フォーム「保存」ボタンで一括 |
| 編集 UI | HTMX でサブコンポーネントを差し替え | `editing` フラグで `x-if` 切替 |
| 削除 | HTMX DELETE → サブコンポーネント更新 | HTMX DELETE + `splice()` |
| 用途 | 独立性が高い行 | 親レコードと一体保存が必要な行 |

**注意事項:**
- `x-if` 内の `<select>` に Tom Select を適用する場合、`editing` が `true` になったタイミングで `$nextTick(() => initTomSelect())` を呼ぶ必要がある（§16 TS-1 参照）。
- 削除ボタンは `row.id` の有無で分岐する。DB 保存済み行（`id` あり）は `hx-delete` で即時削除、未保存行（`id` なし）は `splice` のみ。
- Handler 側で `rules_json` を `c.ShouldBind` の対象フィールドとして受け取り、`encoding/json` でデコードしてから一括保存する。

### 補足: `<td>` に `display: flex` を直接当てると高さが揃わない

`.actions-cell { display: flex }` のような CSS を `<td>` 要素に直接適用すると、ブラウザの `table-cell` レイアウトと衝突してセル下端が他のセルと揃わなくなる（セル下に謎の隙間・ボーダーが見える）。

**NG:**
```css
.actions-cell {
    display: flex;      /* td が table-cell でなくなる */
    justify-content: center;
    gap: 4px;
}
```

**OK: td は table-cell 維持、内側 div で flex**
```templ
<td class="actions-cell">
    <div class="actions-cell-inner">
        <button class="btn btn-small">編集</button>
        <button class="btn btn-danger btn-small">削除</button>
    </div>
</td>
```
```css
.actions-cell {
    display: table-cell;       /* ブラウザデフォルト維持 */
    vertical-align: middle;
    text-align: center;
}
.actions-cell-inner {
    display: flex;
    gap: 4px;
    justify-content: center;
}
```

**縦並びボタン（保存/キャンセル等）の場合:**
```css
.actions-cell-stacked {
    display: flex;
    flex-direction: column;
    gap: 4px;
    align-items: stretch;
}
.actions-cell-stacked > .btn {
    width: 100%;
    min-width: 0;
    padding: 3px 2px;
    font-size: 10px;
    white-space: nowrap;
}
```
※ ボタン文字が収まらない狭い列では、`font-size` を小さくしつつ `white-space: nowrap` で改行阻止。

**チェック方法:** ブラウザ DevTools で td 要素の `getBoundingClientRect().bottom` を隣接セルと比較。差が 1px 以上なら flex の衝突を疑う。

### 補足: outerHTML swap で `scope` 子セレクタを使って CSS 波及を防ぐ

インライン編集テーブルの td padding 調整などで、親テーブルの展開セルと子テーブルのセルが同じセレクタでマッチしてしまうケースがある。子孫セレクタより **直接子セレクタ `>`** でスコープを絞るのが安全。

**NG（子テーブル内の td にも波及する）:**
```css
.readings-table-wrapper tbody tr td[colspan] {
    padding: 0 !important;
}
```
→ サブテーブル内の `colspan` セルにも効いてしまい、意図しない padding 0 を強制。

**OK（親テーブルの td のみ対象）:**
```css
.readings-table-wrapper > .data-table > tbody > tr > td[colspan] {
    padding: 0 !important;
}
```

---

## 13. ファイルアップロード方針

> **現状の該当画面なし注記:**
> 本プロジェクトの確定9画面（login, register, dashboard, device-show, device-create, device-edit, readings, alert-rules, alert-history）にはファイルアップロードを伴う画面は存在しない。本セクションは将来 HTMX 経由のファイルアップロードが必要になった場合に備えた汎用ガイドとして残す。

> **cc-sdd への価値:**
> HTMX 経由のファイルアップロードを行う場合の標準パターン。`multipart/form-data` のエンコーディング指定と、アップロード後の部分更新領域の更新方法を定義する。

**採用方針:** `hx-encoding="multipart/form-data"` を使用する。

```html
<form hx-post="/devices/{{ deviceID }}/files/upload"
      hx-target="#device-file-list"
      hx-encoding="multipart/form-data">
  <input type="file" name="file">
  <button type="submit" class="btn btn-primary">アップロード</button>
</form>

<div id="device-file-list">
  <!-- templ では DeviceFileList コンポーネントとして分離する（後述） -->
</div>
```

templ では `@fragment` 構文を使わず、部分更新領域を別の templ コンポーネント関数に分割する。フルページ用 templ がこの sub-component を呼び、HTMX 時は Handler が sub-component を直接レンダリングする。

```templ
// internal/view/component/device_file_list.templ
// デバイス添付ファイル一覧（部分更新領域）
templ DeviceFileList(deviceID int64, files []FileItem) {
	<div id="device-file-list">
		<ul>
			for _, f := range files {
				<li>
					{ f.OriginalName }
					<button
						type="button"
						class="btn btn-danger btn-small"
						hx-delete={ fmt.Sprintf("/devices/%d/files/%d", deviceID, f.ID) }
						hx-target="#device-file-list"
						hx-confirm={ fmt.Sprintf("「%s」を削除しますか？", f.OriginalName) }
					>
						削除
					</button>
				</li>
			}
		</ul>
	</div>
}
```

**Handler:**

Gin Handler では `c.FormFile` でアップロードファイルを受け取る。バリデーションは Handler 内で実施し、エラー時もアップロード後も同じ `DeviceFileList` コンポーネントを直接レンダリングして部分更新領域だけを返す。

```go
// ファイルアップロード（HTMX 部分更新）
func (h *DeviceHandler) UploadFile(c *gin.Context) {
	deviceID, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "不正なデバイスIDです")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusUnprocessableEntity, "ファイルが選択されていません")
		return
	}
	// サイズ上限（例: 10MB）を Handler 内で検証する。FormRequest 相当は無い
	const maxSize = 10 << 20
	if fileHeader.Size > maxSize {
		c.Status(http.StatusUnprocessableEntity)
		// バリデーションエラーでも一覧コンポーネントを返し、部分更新領域を維持する
		files, _ := h.repo.ListFiles(c.Request.Context(), deviceID)
		component.DeviceFileList(deviceID, files).Render(c.Request.Context(), c.Writer)
		return
	}

	// ファイル保存処理...
	if err := h.repo.SaveFile(c.Request.Context(), deviceID, fileHeader); err != nil {
		c.String(http.StatusInternalServerError, "ファイルの保存に失敗しました")
		return
	}

	files, err := h.repo.ListFiles(c.Request.Context(), deviceID)
	if err != nil {
		c.String(http.StatusInternalServerError, "ファイル一覧の取得に失敗しました")
		return
	}
	// 更新後の一覧コンポーネントを直接レンダリング
	component.DeviceFileList(deviceID, files).Render(c.Request.Context(), c.Writer)
}
```

---

## 14. ネットワークエラー・タイムアウト対応方針

> **cc-sdd への価値:**
> HTMX リクエストが失敗した場合（ネットワーク断、サーバーエラー、タイムアウト、セッション切れ）のユーザー体験はプロジェクト固有の設計決定。未定義だとエラー時に無反応になり、ユーザーが操作を繰り返す原因になる。

**採用方針:** `htmx:responseError` / `htmx:sendError` イベントでグローバルにエラー通知（トースト）を表示する。通知用のトースト領域とイベントハンドラは `internal/view/layout/App.templ` の `<body>` 末尾に配置する。

> ⚠️ **前提（§7 と必ずセット）:** このハンドラが機能するのは §7 の `responseHandling` で `[45]..` に **`error: true`** が付いている場合のみ。付いていないと 4xx/5xx で `htmx:responseError` が**発火せず、本ハンドラは一切動かない**（＝無言失敗。別プロジェクト〈Laravel〉KDCS_PRJ-265 バグ⑦の真因）。✅ `error: true` は §7 で適用済み（2026-06-25）。

> 📅 2026-06-25 適用済み（`App.templ` + `.error-toast` CSS）。トーストは **Alpine 非依存の純 JS** で実装する（Alpine 未ロード時もエラー表示できる）。クラスは**インライン検証エラーの `.error-message` とは別物の `.error-toast`** を使う（テストが `.error-message` を「フィールド検証エラーの有無」マーカーに使うため、レイアウト常設トーストが同クラスだと誤検知する。`.error-toast` は `mocks/html/style.css` に追加し `make sync-css`）。

```html
<!-- internal/view/layout/App.templ の <body> 末尾 -->
<div id="global-error-toast" class="error-toast" role="alert" style="display:none;">
  <span id="global-error-toast-text"></span>
</div>

<script>
    (function () {
        var toast = document.getElementById('global-error-toast');
        var msgEl = document.getElementById('global-error-toast-text');
        var hideTimer = null;
        function showError(message) {
            if (!toast || !msgEl) return;
            msgEl.textContent = message;
            toast.style.display = '';
            if (hideTimer) clearTimeout(hideTimer);
            hideTimer = setTimeout(function () { toast.style.display = 'none'; }, 5000);
        }
        document.addEventListener('htmx:responseError', function (event) {
            var status = event.detail.xhr ? event.detail.xhr.status : 0;
            // 401（セッションタイムアウト）: トースト通知後、ログイン画面へ遷移する。
            // 古い内容が残ったまま無言でログアウト状態になるのを防ぐ。
            if (status === 401) {
                if (window.__sessionExpiredHandled) return;   // 多重発火抑止
                window.__sessionExpiredHandled = true;
                showError('セッションが切れました。ログイン画面へ移動します。');
                setTimeout(function () { window.location.href = '/login'; }, 1500);
                return;
            }
            var message = '';
            if (status === 403) message = 'アクセス権限がありません。';                 // 認可拒否（CSRF は 419 で分離＝補完②）
            else if (status === 404) message = '対象が見つかりません。';
            else if (status === 419) message = 'セッションが切れました。ページを再読み込みしてください。'; // CSRF/セッション（補完②）
            else if (status >= 500) message = 'サーバーエラーが発生しました。しばらく後に再度お試しください。';
            if (message) showError(message);
        });
        // ネットワークエラー（接続失敗・タイムアウト）
        document.addEventListener('htmx:sendError', function () {
            showError('ネットワークエラーが発生しました。接続を確認してください。');
        });
    })();
</script>
```

**サーバ側の補完①（401 セッションタイムアウト・HTMX 経路）✅ 2026-06-25 適用済み:** 旧 `internal/middleware/require_auth.go` は未認証時に **HX-Request の有無に関わらず一律 302 リダイレクト**していた。HTMX 経路では 302 がフォローされ**ログイン画面の HTML が部分領域に swap されて壊れる**ため、**HX-Request 有のときは本文なし 401 を返す**ように分岐した（上のフロント 401 分岐がトースト＋`/login` 遷移を担当）。非 HTMX は従来どおり 302。テストは `TestRequireAuth_未認証のHTMXは本文なし401`。

```go
// internal/middleware/require_auth.go（適用済み）
func RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        if auth.UserID(c) <= 0 {
            if c.GetHeader("HX-Request") != "" {
                c.Status(http.StatusUnauthorized) // 本文なし 401（フロントが toast + /login 遷移）
                c.Abort()
                return
            }
            c.Redirect(http.StatusFound, "/login") // 通常ナビゲーションは従来どおり
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**サーバ側の補完②（CSRF/セッション切れ — gorilla/csrf の 403 を 419 へ分離）✅ 2026-06-25 適用済み:** 別プロジェクト〈Laravel〉は CSRF/セッション切れを **419** で返すが、本プロジェクトの `gorilla/csrf` は**既定で 403** を返す（`internal/middleware/csrf.go`、KDCS_PRJ-273 バグ③ に相当）。一方 **403 は所有者外アクセス（BOLA）拒否でも使う**（`renderError(c, http.StatusForbidden)`）ため、**両者が 403 に混在するとフロントで「再読み込みすべき CSRF 失敗」と「権限なし」を区別できない**。そこで `gorilla/csrf` に `csrf.ErrorHandler` を設定し、CSRF 失敗を **419（`middleware.StatusCSRFExpired`）** で返して認可 403 と分離した。要件 8.2（「拒否し状態変更しない」）はステータスを規定しないため、403→419 はこの要件の範囲内（design/tasks のステータス記述と CSRF 系テスト 6 本を 419 へ更新済み）。

```go
// internal/middleware/csrf.go（適用済み: CSRF 失敗を 419 にして認可 403 と分離）
const StatusCSRFExpired = 419 // Page Expired（HTTP 標準外・Laravel 由来の慣習値）

protect := csrf.Protect(
    csrfAuthKey(cfg.SessionSecret),
    csrf.Secure(isProd), csrf.Path("/"), csrf.SameSite(csrf.SameSiteLaxMode),
    csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("HX-Request") != "" {
            w.WriteHeader(StatusCSRFExpired) // HTMX: 本文なし。フロントの 419 分岐がトースト表示
            return
        }
        // 非 HTMX フルページ送信: 素の 403 本文でなくログイン誘導を 419 で返す
        w.WriteHeader(StatusCSRFExpired)
        _, _ = w.Write([]byte("セッションが切れました。ログインし直してください。"))
    })),
)
```

> 補足: `419`（Page Expired）は HTTP 標準コードではなく Laravel 由来の慣習値。標準コードで揃えたい場合は 403 のまま `X-CSRF-Failed: 1` 等のカスタムヘッダで識別する代替もあったが、Laravel の番号体系に寄せ可読性を優先して 419 を採用した。要点は「CSRF 失敗」と「認可拒否」を**フロントが区別できる識別子を 1 つ用意する**こと。/api（Bearer・機械間）は CSRF 非適用のため本分岐の対象外（DeviceAuth が 401 を返す）。

**ルール:**
- エラー通知はトースト形式（画面上部に一時表示、5 秒で自動消去）。スタイルは `.error-toast` クラスを利用する（インライン検証エラーの `.error-message` とは別物）。
- **401（セッション切れ）はトースト通知後 `/login` へ自動遷移**（多重発火は `window.__sessionExpiredHandled` で抑止）。サーバは HTMX 経路で本文なし 401 を返す（補完①）。
- 403（認可拒否）はトーストのみで自動遷移しない（CSRF と混在しうるため。分離する場合は補完②）。
- 419/404/5xx は各メッセージのトーストを表示（本文 swap はしない）。
- 422（バリデーション）は §7 の方針で swap 処理されるため、このハンドラでは対象外（`responseHandling` で `swap: true`）。
- 自動リトライは行わない（データ重複のリスクを回避）。

---

## 15. 複数インラインCRUD領域の同一画面共存

> **cc-sdd への価値:**
> 1画面に複数のインラインCRUD領域が存在する場合の設計上の注意事項。本プロジェクトでは alert-rules 画面（`/alerts/rules`）でアラートルールの一覧追加・削除を扱う等、将来的に複数領域が同居しうる構成への指針として残す。

**ルール:**

1. **各領域は独立した部分更新コンポーネント・独立したエンドポイントで操作する。** 1つのインライン操作が別領域のコンポーネントに影響を与えてはならない。

2. **メインフォーム送信とインラインCRUDは分離する。** メインフォーム（例: デバイス基本情報の保存 POST `/devices/{device}`）は通常フォーム送信でフルページ遷移（`c.Redirect(http.StatusSeeOther, ...)`）。インラインCRUD（例: アラートルール追加 POST `/alerts/rules`）は HTMX で該当領域のみ更新する。

3. **インラインCRUDで追加・編集したデータはサーバー側DBに即時保存する。** メインフォームの「保存」ボタンを押す前でも、インラインCRUDの操作結果は DB 反映済みである。

```
[デバイス編集画面 /devices/{device}/edit のレイアウト例]
┌────────────────────────────────────────────┐
│ メインフォーム（通常送信 POST /devices/{device}）       │
│ ┌──────────────┐                            │
│ │ 基本情報入力     │（デバイス名・設置場所など）        │
│ └──────────────┘                            │
│                                             │
│ ┌──────────────┐ ← HTMX #alert-rule-list    │
│ │ アラートルール一覧 │   POST   /alerts/rules            │
│ │ （インラインCRUD） │   DELETE /alerts/rules/{rule}     │
│ └──────────────┘                            │
│                                             │
│ [保存ボタン] ← 通常フォーム送信                  │
└────────────────────────────────────────────┘
```

> **設計のポイント:**
> - 各領域は専用の templ コンポーネント（例: `AlertRuleList`）に分割し、対応する HTMX エンドポイントだけがそのコンポーネントを再描画する。
> - 同一画面に2領域以上を置く場合でも、各領域の `id`（`#alert-rule-list` 等）とエンドポイントを1対1で対応させ、相互干渉を避ける。

---

## 16. Tom Select 障害パターン集（TS）

> **⚠️ Tom Select は本プロジェクト唯一の命令的DOM操作ライブラリである。**
> Alpine.js（宣言的HTML属性）・HTMX（宣言的サーバー通信）と異なり、Tom Select は生のJavaScript でDOMを直接操作する（`<select>` を非表示にし独自UIを構築する）。このため、HTMX swap・Alpine.js ライフサイクル・CSS表示タイミングの**全てと干渉しうる**。
>
> 新しい Tom Select + HTMX の組み合わせパターンを実装する際は、本セクションの既知パターン（TS-1〜）を必ず確認すること。新たに発見された障害パターンは連番（TS-N）で追記する。
>
> **基盤設計:** C12（Tom Select + HTMX ライフサイクル管理）に初期化方式・グローバルハンドラ・Alpine.js 状態同期の基盤設計を記載。本セクションは**具体的な障害とその回避策**に特化する。
>
> Tom Select 初期化 JS（`initTomSelect()` 等）はフレームワーク非依存のため、配置先を `App.templ` に読み替えてそのまま流用する。

---

### TS-1: HTMX swap 後の破棄・再初期化が発火しない（`querySelectorAll` の制約）

**症状:** カスケード連動で `hx-target` が `<select>` 要素自体を指す場合、swap 後に Tom Select の UI が古い選択肢のまま更新されない。

**原因:** C12 §2 のグローバルハンドラは `target.querySelectorAll('.tomselected')` で破棄対象を検索するが、`querySelectorAll` は**子孫要素のみ**を検索し、`target` 要素自体は含まない。`hx-target="#metric-select"` のように `<select>` を直接ターゲットにすると、`beforeSwap` の `destroy()` も `afterSwap` の `initTomSelect()` も発火しない。

**対策: ラッパーDiv方式**

`<select>` を `<div>` で囲み、`hx-target` をラッパーDiv に設定する。レスポンスは `<option>` のみではなく `<select>` タグ全体を返却する。

```html
<!-- ✅ ラッパーDiv方式 -->
<select name="device_id" class="js-tom-select"
        hx-get="/devices/metrics"
        hx-target="#metric-wrapper"
        hx-swap="innerHTML"
        hx-trigger="change">
    ...
</select>

<div id="metric-wrapper">
    <!-- hx-target はここ。レスポンスは <select> タグ全体 -->
    <select name="metric" class="js-tom-select">
        <option value="">全て</option>
        ...
    </select>
</div>
```

```html
<!-- ❌ 誤ったパターン: <select> を直接ターゲットにする -->
<select name="device_id" class="js-tom-select"
        hx-get="/devices/metrics"
        hx-target="#metric-select"
        hx-swap="innerHTML"
        hx-trigger="change">
    ...
</select>

<select name="metric" id="metric-select" class="js-tom-select">
    <!-- destroy() も initTomSelect() も発火しない -->
</select>
```

**サーバー側レスポンス:**

templ コンポーネントで `<select>` タグ全体を返す（部分更新領域を担うサブコンポーネントとして分離し、Handler が直接レンダリングする）。

```templ
// ✅ <select> タグ全体を返却するコンポーネント
templ MetricSelect(options []MetricOption) {
	<select name="metric" class="js-tom-select">
		<option value="">全て</option>
		for _, o := range options {
			<option value={ o.Value }>{ o.Label }</option>
		}
	</select>
}
```

```templ
// ❌ <option> のみの返却（Tom Select 付き <select> には不適）
templ MetricOptionsOnly(options []MetricOption) {
	<option value="">全て</option>
	for _, o := range options {
		<option value={ o.Value }>{ o.Label }</option>
	}
}
```

> **例外:** Tom Select を適用しない素の `<select>`（ネイティブ select、本プロジェクトでは表示件数や boolean 型の選択等）は `querySelectorAll` の制約に影響されないため、`<option>` のみの返却で動作する。

---

### TS-2: モーダル `display:none` 時の幅0初期化

**症状:** モーダル内の Tom Select コンボが横幅0で描画される（クリックしても何も表示されない、または極端に細い UI になる）。

**原因:** C12 §2 のグローバル `htmx:afterSwap` ハンドラは swap 完了直後に `initTomSelect(target)` を呼ぶ。しかしモーダルは swap 時点ではまだ `x-show="false"`（`display:none`）のため、ブラウザが要素の幅を計算できず 0 幅で初期化される。その後 `@modal-open` で再度 `initTomSelect` を呼んでも、初回の初期化で `.tomselected` クラスが既に付与されており、`:not(.tomselected)` セレクタに合致せずスキップされる。

**対策:** モーダル表示時に `$nextTick` で DOM 更新後（`display:block` 後）に `destroy()` → 再初期化する。

```html
<!-- internal/view/layout/App.templ のモーダルコンテナで対応済み -->
<div x-data="{ open: false }"
     @modal-open.window="open = true; $nextTick(() => {
         $refs.modalContent.querySelectorAll('select.js-tom-select.tomselected').forEach(function(el) {
             if (el.tomselect) el.tomselect.destroy();
         });
         initTomSelect($refs.modalContent);
     })">
```

**フロー:**

1. `hx-get` → swap 完了 → `htmx:afterSwap` → `initTomSelect()` → **0 幅で初期化**
2. `$dispatch('modal-open')` → `open = true` → Alpine DOM 更新予約
3. `$nextTick` → `display:block` 適用後 → `destroy()` で 0 幅インスタンス破棄 → `initTomSelect()` で**正しい幅で再初期化**

> `sync()` はオプション変更の反映用であり、幅の再計算には効かない。`destroy()` → `new TomSelect()` が必要。

---

### TS-3: `.clear()` によるカスケード HTMX リクエストの誤発火

**症状:** 検索クリアボタン押下時に、デバイス選択コンボの `.clear()` がメトリック選択カスケードの HTMX リクエストを意図せず発火し、クリア→検索の自動再検索と競合する。

**原因:** Tom Select の `.clear()` はデフォルトで `onChange` コールバックを発火する。C12 §3 の `onChange` 内で `el.dispatchEvent(new Event('change', { bubbles: true }))` を実行しているため、`change` イベント → `hx-trigger="change"` → カスケード HTMX リクエストが連鎖する。

**対策:** `.clear(true)` でサイレントモードを使用する。引数 `true` は `onChange` コールバックの発火を抑制する。

```javascript
// ❌ .clear() → onChange 発火 → change イベント → HTMX カスケードリクエスト
el.tomselect.clear();

// ✅ .clear(true) → onChange 抑制 → カスケード防止
el.tomselect.clear(true);  // silent mode
```

**使用場面:** 検索フォームのクリアボタンで、カスケード元コンボ（デバイス選択等）とカスケード先コンボ（メトリック選択等）を同時にリセットした後、検索ボタンの `.click()` で HTMX リクエストを1回だけ発火させる場合。

```javascript
// クリアボタンの onclick ハンドラ例
function clearSearchForm(formScope) {
    // (1) テキスト入力の初期化
    formScope.querySelectorAll('input[type=text]').forEach(el => el.value = '');
    // (2) チェックボックスの初期化
    formScope.querySelectorAll('input[type=checkbox]').forEach(el => el.checked = false);
    // (3) Tom Select のサイレントクリア
    formScope.querySelectorAll('select.tomselected').forEach(function(el) {
        if (el.tomselect) el.tomselect.clear(true);
    });
    // (4) 検索ボタンの click シミュレートで HTMX リクエスト発火
    document.querySelector('.filter-form button[type=submit]').click();
}
```

**共通の検索クリア処理:**

検索フォーム（`.filter-form`）の検索クリアボタンに、`form.reset()` 後の Tom Select リセット処理を組み込む。カスケード連動を持たない検索フォーム（readings 画面のデバイス・期間フィルタ等）では、個別のクリアハンドラ実装は不要。

```html
<!-- filter-form 内 — 検索クリアボタン -->
<button type="button" class="btn btn-secondary"
    @click="
        const form = $el.closest('form');
        form.reset();
        form.querySelectorAll('select.js-tom-select').forEach(el => {
            if (el.tomselect) el.tomselect.setValue('');
        });
        htmx.trigger(form, 'submit');
    ">
    検索クリア
</button>
```

> `setValue('')` は `onChange` を発火するが、直後に `htmx.trigger(form, 'submit')` でフォーム全体を送信するため、カスケードの個別リクエストは HTMX の同一要素リクエスト置換により実害はない。カスケード連動を持つ検索フォーム（デバイス→メトリックのカスケード等）で個別リクエストの抑制が必要な場合は、上記の `clear(true)` サイレントモードを使用すること。

---

### TS-4: OOB swap 先の Tom Select が破棄・再初期化されない

**症状:** OOB（`hx-swap-oob="true"`）で差し替えられた領域内の Tom Select が古いインスタンスのまま残る。

**原因:** `htmx:beforeSwap` / `htmx:afterSwap` の `event.detail.target` はメイン swap ターゲットを指す。OOB で差し替えられる別の DOM 領域は対象外。

**対策（方針）:** 本プロジェクトでは OOB は集計エリア（サマリーボックス）・タブナビゲーション等に限定使用（§5参照）し、OOB 領域内に `<select class="js-tom-select">` を含めない。やむを得ず配置する場合は `htmx:oobAfterSwap` イベントで個別に `initTomSelect` を呼ぶ。

```javascript
// OOB 内に Tom Select が必要な場合のみ
document.addEventListener('htmx:oobAfterSwap', function(event) {
    initTomSelect(event.detail.target);
});
```

---

### TS-5: Alpine.js `x-model` が Tom Select に直接機能しない

**症状:** Tom Select 適用済みの `<select>` に `x-model` を設定しても、Tom Select の UI 操作が Alpine 変数に反映されない。

**原因:** Tom Select は元の `<select>` を非表示にし独自 UI を構築する。Alpine.js の `x-model` は元の `<select>` の `change` イベントを監視するが、Tom Select の UI 操作はネイティブ `change` を直接発火しない。

**対策:** `onChange` コールバック内で Alpine 変数を手動更新し、`dispatchEvent(new Event('change'))` でネイティブイベントも発火する。詳細は C12 §3 参照。

> **例外:** ネイティブ select（boolean 型・表示件数等）は Tom Select を適用しないため、`x-model` が直接機能する。

### TS-6: Tom Select の CSS クラス `tom-select` と `js-tom-select` の混在

**症状:** `<select class="form-select tom-select">` と記述した要素で Tom Select が初期化されない。モーダル表示後もプレーンな `<select>` のまま表示される。

**原因:** C12 §1 の `initTomSelect()` は `select.js-tom-select:not([disabled]):not(.tomselected)` セレクタで初期化対象を検索する。`tom-select` クラスはこのセレクタに一致しないため、初期化がスキップされる。

**回避策:**

```html
<!-- ❌ 誤: initTomSelect() に検出されない -->
<select name="metric" class="form-select tom-select">

<!-- ✅ 正: initTomSelect() のセレクタに一致 -->
<select name="metric" class="form-select js-tom-select">
```

Tom Select を適用する `<select>` には必ず `js-tom-select` クラスを使用すること。`tom-select` は CSS フレームワーク由来のクラス名と紛らわしいため使用しない。

**影響範囲:** HTMX swap 後の再初期化（C12 §2 `htmx:afterSwap` ハンドラ）とモーダル初回表示時の初期化（`@modal-open.window` → `initTomSelect($refs.modalContent)`）の両方が影響を受ける。

### TS-7: カスケードエンドポイントの HTML 属性が画面固有である問題

**症状:** デバイス選択カスケードのエンドポイントを別画面から共用したところ、フォーム送信時にカスケード先項目が「必須エラー」になる。

**原因:** エンドポイントが返却する `<select>` の HTML 属性が呼び出し元画面に固有の値でハードコードされている。

| 属性 | エンドポイントA の返却値 | 呼び出し元画面B が期待する値 |
|------|--------------------------|-----------------------------|
| `name` | `metric` | `target_metric` |
| `class` | `form-control js-tom-select` | `form-select js-tom-select` |
| `id` | （なし） | `metric-select` |
| デフォルト option | `全て` | `選択してください` |

`name` 属性の不一致が致命的。サーバーバリデーション（`binding` タグ）は `target_metric` を期待するが、エンドポイント返却の `name="metric"` ではリクエストパラメータ名が異なり、必須チェックに引っかかる。

**対策:** カスケードエンドポイントを画面間で共用する場合は、属性の一致を事前に確認する。不一致がある場合は画面専用の軽量エンドポイントを新設する（クエリロジックの共通化は repository パッケージのメソッド抽出で対応可能）。

---

### TS-8: Alpine.js スコープチェーンと共通フォームモーダルの dirty check

**症状:** 共通のフォームモーダルコンポーネント（`internal/view/component` の `FormModal` 相当）を使用する画面で、フォーム入力後にキャンセルボタンを押しても確認ダイアログが表示されない。

**原因:** Alpine.js のスコープチェーンは**内側→外側のみ**解決される。フォームモーダルの DOM 構造は以下のようになっている:

```
<div class="modal-content">
  ├─ modal-header（x-data の外）
  │    └─ close ボタン（×）           ← ❌ どの x-data にもアクセス不可
  └─ <div class="modal-body">
       └─ <div x-data="{ submitting, dirty }">   ← form-modal の x-data
            └─ <form>
                 ├─ { children }
                 │    └─ <div x-data="{ ... }">   ← 呼び出し側の x-data
                 │         └─ @input="dirty = true"
                 └─ form-actions
                      └─ cancel ボタン            ← ✅ form-modal の dirty にアクセス可能
```

- **cancel ボタン**（form-actions 内）: form-modal の `x-data` スコープ内にあるため、`dirty` にアクセス**可能** ✅
- **close ボタン（×）**（modal-header 内）: ベースモーダルの header 領域で描画され、`x-data` の**外側**にあるためアクセス**不可** ❌
- **呼び出し側の x-data** → form-modal の x-data: 子から親へのスコープチェーンで `dirty = true` を更新**可能** ✅
- **form-modal の cancel** → 呼び出し側の x-data: 親から子へは**アクセス不可** ❌

**対策（dirtyMessage 引数パターン）:**

1. フォームモーダルコンポーネントに `dirtyMessage` 引数（文字列、デフォルト空）を追加する
2. form-modal の `x-data` に `dirty: false` を含める（常に定義。既存利用箇所に影響なし）
3. cancel ボタンの `@click` に dirty check 式を条件付きで出力する:
   - `dirtyMessage` 指定時: `(!dirty || confirm(dirtyMessage)) && $dispatch('modal-close')`
   - `dirtyMessage` 未指定時: `$dispatch('modal-close')` （従来通り）
4. 呼び出し側の x-data から `dirty` を**削除**する（`@input="dirty = true"` はスコープチェーンで form-modal の `dirty` を更新する）
5. close ボタン（×）は x-data 外のため dirty check 不可。標準の「強制閉じ」として許容する

templ では引数として `dirtyMessage` を渡し、モーダル本体を children として受け取る（デバイス削除確認モーダルなどの呼び出し例）。

```templ
// 呼び出し側: dirtyMessage を指定してデバイス編集モーダルを表示
@component.FormModal(component.FormModalProps{
	Title:          "デバイス編集",
	Action:         fmt.Sprintf("/devices/%d", deviceID),
	SubmitLabel:    "更新",
	ConfirmMessage: "入力内容を更新しますか？",
	DirtyMessage:   "入力内容が破棄されます。よろしいですか？",
}) {
	// dirty は定義しない → @input="dirty = true" は form-modal の dirty を更新する
	<div x-data="{ thresholdEdited: false }" @input="dirty = true">
		<!-- フォームフィールド（デバイス名・しきい値など） -->
	</div>
}
```

> **注意:** 独自の破棄確認（`confirmDiscard()` 等）パターンは form-modal を使わない独自モーダル構造でのみ有効。共通フォームモーダル利用時は本パターン（`dirtyMessage` 引数）を使用すること。

### TS-9: innerHTML swap 後の Tom Select ネイティブ select 表示残り

**症状:** デバイス→メトリックのカスケードで `hx-swap="innerHTML"` により `#metric-wrapper` の中身を差し替えた後、ネイティブの `<select>` と Tom Select の `.ts-wrapper` が**両方表示**される（二重表示）。

**原因:** Tom Select は初期化時にネイティブ select を非表示にするが、`tabindex="-1"` を設定するのみで `display: none` を明示しない。Tom Select CSS の `select.ts-hidden-accessible` クラスが適用されない場合（HTMX swap 後の再初期化など）、ネイティブ select が `display: inline-block` のまま残る。

**対策:**

```css
/* Tom Select 初期化済み select の非表示（安全ネット） */
select.tomselected,
select.ts-hidden-accessible {
  display: none !important;
}
select[tabindex="-1"].js-tom-select {
  display: none !important;
}
```

**補足:**
- `hx-on::before-swap` / `hx-on::after-swap` をトリガー要素に追加する方法は、グローバルの `htmx:beforeSwap` / `htmx:afterSwap` ハンドラー（§C12）と二重実行になるため**使用しない**
- グローバルハンドラーのみに任せ、CSS で安全ネットを敷くのが正しいパターン

---

### TS-10: `fieldset[disabled]` 内の Tom Select が操作可能なまま残る

**症状:** `<fieldset disabled>` で囲んだセクション内のネイティブ `<input>` / `<select>` は自動的に disabled になるが、Tom Select で初期化済みの `<select>` は**操作可能なまま**残る。閲覧専用にしたいセクションでメトリックやオペレーターの Tom Select が選択できてしまう。

**原因:** `<fieldset disabled>` はネイティブフォーム要素に直接 `disabled` 状態を適用するが、Tom Select は元の `<select>` を非表示にして独自の DOM（`.ts-wrapper`）を構築するため、fieldset の disabled 属性が Tom Select の独自 UI に伝播しない。

**対策:** `initTomSelect()` で `fieldset[disabled]` 内の select を検知し、初期化直後に `ts.disable()` を呼ぶ。

```javascript
function initTomSelect(scope) {
    var root = scope || document;
    root.querySelectorAll('select.js-tom-select:not([disabled]):not(.tomselected)').forEach(function (el) {
        var isInsideDisabledFieldset = !!el.closest('fieldset[disabled]');
        el.classList.add('tomselected');
        // ... 通常の初期化 ...
        var ts = new TomSelect(el, { /* options */ });
        if (isInsideDisabledFieldset) {
            ts.disable();
        }
    });
}
```

**CSS 補足:** disabled 状態の Tom Select をグレー表示にする。

```css
fieldset[disabled] .ts-wrapper.disabled,
fieldset[disabled] .ts-wrapper.disabled .ts-control {
    background-color: #f0f0f0 !important;
    color: #999 !important;
}
```

**影響範囲:** `<fieldset disabled>` で入力制御を行う編集画面（device-edit, alert-rules 等）。

---

### TS-11: `fieldset[disabled]` 内の `<button>` がクリック不可になる

**症状:** `<fieldset disabled>` 内のアコーディオン開閉ボタン（`<button type="button">`）がクリックに反応せず、セクションを展開できない。

**原因:** HTML仕様により、`<fieldset disabled>` 内の `<button>` 要素は自動的に disabled になる。アコーディオンの開閉ボタンはフォーム入力ではなく UI 操作だが、`<button>` として実装すると fieldset の disabled 制御の影響を受ける。

**対策:** アコーディオンのセクションヘッダーを `<button>` ではなく `<div role="button">` で実装する。

```html
<!-- NG: fieldset[disabled]内でクリック不可 -->
<button type="button" class="section-header" @click="open = !open">

<!-- OK: fieldset[disabled]内でもクリック可能 -->
<div class="section-header" @click="open = !open" role="button" tabindex="0" style="cursor: pointer;">
```

**影響範囲:** アコーディオンコンポーネント。`<fieldset disabled>` と組み合わせて使用する編集画面（device-edit, alert-rules 等）。

---

### TS-12: `htmx:afterSwap` の `event.detail.target` が `outerHTML` swap 後に DOM から detach される

**症状:** `hx-swap="outerHTML"` で `<tr>` を差し替える操作（例: アラートルールのインライン編集モード切替）の直後、挿入された新 tr 内の `<select class="js-tom-select">` が Tom Select 化されない。ネイティブ select が `display:inline-block` のまま狭い幅で表示されて親セルを突き抜ける。

**原因:** HTMX の `outerHTML` swap は、対象要素（古い `<tr>`）を DOM から detach して新しい `<tr>` を同じ位置に挿入する。`htmx:afterSwap` ハンドラで受け取る `event.detail.target` は **detach 済みの古い `<tr>`** を指しており、`target.querySelectorAll(...)` は常に空集合を返す。結果として `initTomSelect(target)` 内の `root.querySelectorAll('select.js-tom-select:not(.tomselected)')` がマッチせず初期化がスキップされる。

TS-1（`querySelectorAll` が target 自体を含まない）と**別の問題**である点に注意: TS-1 は「target が単体 select の場合に `querySelectorAll` が子孫を検索するため漏れる」、TS-12 は「target が tr/div 等のラッパーでも detach 済みで DOM に無いため何もヒットしない」。

**対策:** `htmx:afterSwap` で `document.body.contains(target)` を確認し、DOM から外れていたら `document` をフォールバック対象にする。

```javascript
document.body.addEventListener('htmx:afterSwap', function (event) {
    var target = event.detail.target;
    // outerHTML swap 等で target が DOM から外れている場合は document 全体を再初期化対象に
    var initTarget = (!target || !document.body.contains(target)) ? document : target;
    initTomSelect(initTarget);
    // 元の target 参照が必要な分岐（例: #modal-content の自動オープン）は target 変数を維持する
    if (target && target.id === 'modal-content') {
        // ...
    }
});
```

**テスト観点:** `tomselect` プロパティの存在と `.ts-wrapper` 要素の生成をブラウザ DevTools で実測する（§5.6 のガイダンス集参照）。

```javascript
// 合格例
const row = document.querySelector('.data-table tr[id^="rule-row-"]');
const select = row?.querySelector('select[name="metric"]');
// ✅ swap 後でも tomselect が付いていること
console.assert(select.classList.contains('tomselected'));
console.assert(!!select.tomselect);
```

**影響範囲:**
- `hx-swap="outerHTML"` で tr/div を差し替える全インライン編集画面（アラートルールのインライン編集行など）
- Tom Select 以外にも、swap 後 DOM に何らかの後処理（イベントバインド等）をかける必要があるケース全般

**関連:** TS-1, TS-9, §12（インラインCRUD方針）, §9（HX-Redirect）

---

## 付録: App.templ 共通レイアウト配置要素一覧

> **目的:** 本ガイドの各セクションで「`internal/view/layout/App.templ` に配置」と記述されている要素を一覧化し、アプリケーションシェル実装時の漏れを防ぐ。`App.templ` はダッシュボード・デバイス詳細・計測一覧・アラート設定/履歴など認証後の全画面が共有する共通レイアウト。ログイン/登録のゲスト画面は `Guest.templ` を使う。

### `<head>` 内

| 要素 | 根拠セクション | 用途 |
|------|-------------|------|
| `<meta name="csrf-token" content={ csrfToken }>` | §8 CSRF対応 | HTMX リクエストへの自動CSRF付与（トークンは Handler が templ 引数で渡す） |
| `<link rel="stylesheet" href={ assetURL("css/style.css") }>` | 既存資産 | 全画面共通スタイル（`public/` を `go:embed`、バージョンクエリでキャッシュバスティング） |
| Tom Select CSS（CDN） | C12 | Tom Select のスタイル |

### `<body>` 内（HTML構造）

| 要素 | 根拠セクション | 用途 |
|------|-------------|------|
| `@layout.SiteHeader(...)`（`.site-header`） | 既存資産 | ヘッダーバー（タイトル・`.user-menu`・`.user-name`・ログアウトフォーム） |
| `@layout.Sidebar(...)`（`.sidebar`） | 既存資産 | サイドバーナビゲーション |
| メインコンテンツ領域（`.main-content` / `.main-inner`） | — | 各画面のメインコンテンツ領域（children として差し込む） |
| グローバルエラートースト `#global-error-toast` | §14 ネットワークエラー | エラー時のユーザー通知（Alpine.jsコンポーネント） |
| フラッシュメッセージ領域 | — | `.error-message` / `.empty-message` 等の通知表示。Go ではセッションフラッシュ（scs、将来 `internal/auth/session_auth.go`）の値を Handler が templ 引数で渡す。Laravel の暗黙の共有エラーバッグは無いため、表示する値は明示的に引数で渡す |
| 共通モーダルコンテナ `#modal-content` | C01 モーダル, C12§3 | 全画面共通のモーダル表示領域（`.modal-overlay` / `.modal-content`） |

### `</body>` 直前スクリプト（配置順序）

| 順序 | 要素 | 根拠セクション | 用途 |
|:----:|------|-------------|------|
| ① | HTMX 2.x CDN `<script>` | — | HTMX ライブラリ読み込み |
| ② | Tom Select CDN `<script>` | C12 | Tom Select ライブラリ読み込み |
| ③ | Alpine.js CDN `<script defer>` | 既存資産 | Alpine.js ライブラリ読み込み |
| ④ | `htmx.config.responseHandling` | §7 バリデーション | 422をswap対象に含める設定 |
| ⑤ | `htmx:configRequest` ハンドラ | §8 CSRF | X-CSRF-Token ヘッダー自動付与 |
| ⑥ | `initTomSelect()` 関数定義 + 初回実行 | C12§1 | Tom Select の初期化 |
| ⑦ | `htmx:beforeSwap` ハンドラ | C12§2 | swap前のTom Selectインスタンス破棄 |
| ⑧ | `htmx:afterSwap` ハンドラ | C12§2 | swap後のTom Select再初期化 |
| ⑨ | `htmx:responseError` / `htmx:sendError` ハンドラ | §14 ネットワークエラー | グローバルエラートースト表示 |

> これらのスクリプトはフレームワーク非依存のため、モック（姉妹文書のスクリプト群）からそのまま `App.templ` の末尾に移植してよい。templ では `<script>` ブロックをコンポーネント末尾に直接記述する。画面固有JSが必要な場合は、その画面の templ コンポーネント内に `<script>` を置く（Laravel の `@stack('scripts')` 相当の機構は無い）。

### CSRF meta タグの渡し方（Go/Gin）

CSRF トークンは Gin の CSRF ミドルウェア（採用ライブラリは要確認）が生成する。Handler でトークンを取得し、`App.templ` の引数として渡して `<meta>` に埋め込む。HTMX 側はこの meta を読んで `htmx:configRequest` で `X-CSRF-Token` ヘッダーを付与する。

```go
// Handler: CSRF トークンを templ に渡す
func (h *DashboardHandler) Show(c *gin.Context) {
    csrfToken := getCSRFToken(c) // CSRF ミドルウェアからトークン取得（採用ライブラリは要確認）
    layout.App(layout.AppProps{
        Title:     "ダッシュボード",
        CSRFToken: csrfToken,
        User:      currentUser(c),
    }, page.Dashboard(...)).Render(c.Request.Context(), c.Writer)
}
```

### 共通モーダルコンテナの実装（C01 + C12§3 統合）

```templ
// internal/view/layout/App.templ — 共通モーダルコンテナ
templ ModalContainer() {
	<div
		x-data="{ open: false }"
		@modal-open.window="open = true; $nextTick(() => {
			$refs.modalContent.querySelectorAll('select.js-tom-select.tomselected').forEach(function(el) {
				if (el.tomselect) el.tomselect.destroy();
			});
			initTomSelect($refs.modalContent);
		})"
		@modal-close.window="open = false"
		@keydown.escape.window="if (open) { open = false; }"
	>
		<div class="modal-overlay" x-show="open" @click="open = false" x-cloak>
			<div class="modal-content" id="modal-content" x-ref="modalContent" @click.stop>
				<!-- hx-get のレスポンス（templ サブコンポーネント）がここに差し込まれる -->
			</div>
		</div>
	</div>
}
```

> **呼び出し側の実装パターン:** 一覧行から詳細モーダルを開く例。
>
> ```html
> <tr
> 	hx-get={ fmt.Sprintf("/devices/%d", device.ID) }
> 	hx-target="#modal-content"
> 	hx-trigger="click"
> 	hx-on::after-swap="$dispatch('modal-open')"
> >
> ```

---

## 19. `hx-vals='js:{...}'` による Alpine.js 変数の動的送信パターン

> 📅 2026-04-02 — アラート設定モーダルで確立。

**課題:** モーダルを開く際に、クライアント側の Alpine.js 配列（例: 選択済みアラートルール一覧）をサーバーに送信したいが、`hx-get` の URL パラメータでは JSON 配列が肥大化する。

**解決策:** `hx-post` + `hx-vals='js:{...}'` で Alpine.js 変数を動的に評価し、POST ボディに含める。

```html
<button
	type="button"
	class="btn btn-primary btn-small"
	hx-post={ fmt.Sprintf("/devices/%d/alert-rules/preview", device.ID) }
	hx-vals='js:{"alert_rules_json": JSON.stringify(alertRules), "metric": document.querySelector("[name=metric]").value}'
	hx-target="#modal-content"
	hx-on::after-swap="$dispatch('modal-open')"
>ルール確認</button>
```

**ポイント:**
- `js:` プレフィックスでJavaScript式を評価する（HTMX公式機能）
- Alpine.js の `x-data` 変数は `hx-vals` の `js:` 式からアクセス可能（同一スコープ内）
- DOM要素の値は `document.querySelector()` で動的取得可能
- サーバー側（Gin Handler）では `c.PostForm("alert_rules_json")` で受け取り、`json.Unmarshal` でデコードする

**注意:** POST メソッドだがデータ変更を行わない読み取り操作の場合もある（モーダル用の templ サブコンポーネント返却等）。意味的には GET だが、JSON 配列のサイズ制約のため POST を使用する。

**⚠️ `hx-vals='js:{}'` が発火しないケース:**

`hx-vals='js:{...}'` 内で Alpine.js スコープ変数（例: `alertRules`）を参照する場合、HTMX の `js:` 式評価タイミングと Alpine.js スコープの初期化タイミングが噛み合わず、リクエスト自体が発火しないケースが報告されている。

**回避策:** `hx-post` + `hx-vals` の代わりに、Alpine.js の `@click` で `htmx.ajax()` を直接呼ぶ。

```html
<!-- NG: hx-vals='js:{}'が発火しない場合がある -->
<button
	hx-post={ fmt.Sprintf("/devices/%d/alert-rules/preview", device.ID) }
	hx-vals='js:{"alert_rules_json": JSON.stringify(alertRules)}'
	...
>...</button>

<!-- OK: Alpine.js @click で htmx.ajax() を直接呼ぶ -->
<button
	@click={ fmt.Sprintf(`htmx.ajax('POST', '/devices/%d/alert-rules/preview', {
		target: '#modal-content', swap: 'innerHTML',
		values: { alert_rules_json: JSON.stringify(alertRules) }
	}).then(() => window.dispatchEvent(new CustomEvent('modal-open')))`, device.ID) }
>ルール確認</button>
```

> **テンプレ注意:** templ では JS 文字列内に `{ ... }` を直接書くとテンプレ式と衝突するため、上記のように `fmt.Sprintf` 等で文字列を組み立てて属性に渡すか、`<script>` ブロックへ切り出して関数呼び出し形にするのが安全。

---

## 20. データ送信モーダル vs 単純モーダルの使い分け

> 📅 2026-04-02 — アラート設定モーダルで確立。

### 判断基準

本プロジェクトのモーダルは `.modal-overlay` / `.modal-content` を共通の外枠とし、中身（templ サブコンポーネント）が用途で分かれる。役割は次の2系統で考える。

| 条件 | モーダルの種類 |
|------|------------------|
| モーダル内のフォームをサーバーに送信（POST/PUT）してDB保存する | **データ送信モーダル**（フォーム + `hx-post`/`hx-put`） |
| モーダルは編集・確認UIのみ提供し、`$dispatch` で親画面にデータを返す（DB保存しない） | **単純モーダル**（`$dispatch` でイベント返却） |

### データ送信モーダルが不適合なケース

データ送信モーダルは送信先URLが固定（フォームの `hx-post`/`hx-put` にハードコード）になりがちで、以下のケースでは使えない。

- **クライアント側で編集 → 親画面の Alpine.js 配列に反映するだけ**のケース。例: モーダル内でアラートルール（しきい値・演算子）を編集し、「更新」ボタンで `$dispatch('alert-rules-updated', { rules })` を発火して親画面（アラート設定画面）の Alpine.js 配列を置換する。DB 保存は親画面のフォーム送信で一括実行する。

### 単純モーダルでの実装パターン

```templ
// internal/view/component/AlertRulesEditModal.templ
templ AlertRulesEditModal(rulesJSON string) {
	<div
		x-data={ fmt.Sprintf("{ rules: JSON.parse(%q), validate() { /* ... */ return true } }", rulesJSON) }
	>
		<!-- 編集UI（ルール行の追加・削除・しきい値入力など） -->
		<div class="form-actions">
			<button
				type="button"
				class="btn btn-secondary"
				@click="$dispatch('modal-close')"
			>キャンセル</button>
			<button
				type="button"
				class="btn btn-primary"
				@click="if(validate()) { $dispatch('alert-rules-updated', { rules }); $dispatch('modal-close'); }"
			>更新</button>
		</div>
	</div>
}
```

**親画面のイベントリスナー（アラート設定画面）:**

```html
<div
	x-data={ fmt.Sprintf("{ alertRules: JSON.parse(%q) }", alertRulesJSON) }
	@alert-rules-updated.window="alertRules = $event.detail.rules"
>
```

> JSON を Alpine に渡す際は、属性へ生 JSON を連結せず `<script type="application/json">` から読むか、上記のように `%q` で安全にクォートした文字列を `JSON.parse` する（§10-E/§32 参照）。

---

## 21. ラジオボタンのクリア操作（Alpine.js）

> 📅 2026-04-02 — アラートルール編集モーダルで確立。

標準 HTML ラジオボタンには選択解除機能がない。排他選択 + クリア操作が必要な場合、ネイティブ `<input type="radio">` の `name` グループではなく、Alpine.js で状態管理する。例として、複数アラートルール行のうち「有効化する1行のみ」を排他選択し、再クリックで解除する `is_active` フラグの制御を示す。

```html
<template x-for="(rule, index) in rules" :key="index">
	<td>
		<input
			type="radio"
			:checked="rule.is_active"
			:disabled="!isActivationEnabled"
			@click.prevent="selectActive(index)"
		>
	</td>
</template>
```

```javascript
// x-data 内メソッド
selectActive(index) {
	if (this.rules[index].is_active) {
		// 同一ラジオ再クリック → クリア（選択解除）
		this.rules[index].is_active = false;
	} else {
		// 排他選択: 全行OFF → 対象行ON
		this.rules.forEach(r => r.is_active = false);
		this.rules[index].is_active = true;
	}
}
```

**ポイント:**
- `@click.prevent` でネイティブラジオの動作を抑止し、Alpine.js で制御する
- `:checked` でAlpine.js状態をバインド（`x-model` ではなく `:checked` を使用）
- `:disabled="!isActivationEnabled"` で他項目（例: 計測メトリックの選択有無）との連動制御が可能

---

## 23. `$dispatch` イベント命名規約

> 📅 2026-04-03 — モーダル系コンポーネントの既存実装パターンを体系化。

### 命名パターン

Alpine.js の `$dispatch()` で発行するカスタムイベントは以下の規約に従う。

| 種別 | 命名パターン | 例 |
|------|------------|-----|
| モーダル開閉 | `modal-open` / `modal-close` | 全モーダル共通。画面固有にしない |
| 選択完了 | `{機能}-selected` | `device-selected`、`metric-selected` |
| データ更新通知 | `{機能}-updated` | `alert-rules-updated` |

### ペイロード規約

```javascript
// 選択系イベントのペイロード: camelCase でキーを統一
$dispatch('device-selected', {
	id: selected,            // 選択対象のデバイスID（number）
	deviceName: selectedName // 表示用名称（例: "ハウスA温湿度計"）
})

// データ更新系: 配列やオブジェクトを渡す
$dispatch('alert-rules-updated', { rules: [...] })
```

### 注意事項

- **`modal-close` は共通イベント名**: 複数モーダルで同じ名前を使う。親画面のモーダルコンテナが `@modal-close.window` でリスンして閉じる
- **選択イベントは機能固有**: `device-selected` と `metric-selected` のように機能名で区別する。汎用名（`item-selected` 等）は使わない
- **ペイロードのキー名は camelCase**: JavaScript の慣習に従う。snake_case は使わない
- **親画面でのリスン**: `@{イベント名}.window="..."` で受信する（Alpine.js の `$dispatch` は DOM ツリーを伝播するため `.window` 修飾子が必要）

---

## 24. Alpine.js モーダル + fetch DELETE + htmx.trigger パターン

> 📅 2026-04-04 — デバイス一覧の削除機能で確立。§11・C06 の `hx-confirm` 方針に対する**例外パターン**。

### 背景: hx-confirm では対応できないケース

§11 と C06 では「削除確認は `hx-confirm` 属性を使い、Alpine.js カスタムモーダルは使わない」と方針を定めている。しかし以下のケースでは `hx-confirm` が不適切。

- **モーダル内にデバイス名・注意書き等の詳細情報を表示する必要がある**場合（ブラウザネイティブダイアログでは装飾不可）
- **削除対象を動的に切り替える**場合（`selectedDevice` を Alpine.js で管理し、モーダル内に表示）

デバイス削除確認モーダルは「デバイス名の表示」「※この操作は取り消せません（計測データも削除されます）」の注意書きが必要なため、`hx-confirm` ではなく Alpine.js モーダルを採用する。

### 実装パターン

**templ 側（Alpine.js モーダル + fetch DELETE）:**

```templ
// 削除ボタン: Alpine.jsで対象を設定しモーダル表示
templ DeviceDeleteButton(device repository.Device) {
	<button
		class="btn btn-danger btn-small"
		@click.stop={ fmt.Sprintf("selectedDevice = { id: %d, name: %q }; showDeleteModal = true", device.ID, device.Name) }
	>削除</button>
}

// 削除確認モーダル
templ DeviceDeleteModal() {
	<div class="modal-overlay" x-show="showDeleteModal" x-cloak>
		<div class="modal-content">
			<p>デバイス名: <span x-text="selectedDevice?.name"></span></p>
			<p class="form-help">※この操作は取り消せません（計測データも削除されます）</p>
			<div class="form-actions">
				<button class="btn btn-secondary" @click="showDeleteModal = false">キャンセル</button>
				<button
					class="btn btn-danger"
					@click="
						fetch('/devices/' + selectedDevice?.id, {
							method: 'DELETE',
							headers: {
								'X-CSRF-Token': document.querySelector('meta[name=csrf-token]')?.content,
								'HX-Request': 'true',
								'Accept': 'text/html'
							}
						}).then(response => {
							if (response.ok) {
								showDeleteModal = false;
								selectedDevice = null;
								htmx.trigger(document.body, 'device-list-refresh');
							}
						})
					"
				>削除する</button>
			</div>
		</div>
	</div>
}
```

```html
<!-- リスト更新トリガー受信 -->
<div
	id="device-grid"
	class="device-grid"
	hx-get="/devices"
	hx-trigger="device-list-refresh from:body"
	hx-target="#device-grid"
	hx-swap="innerHTML"
>
	<!-- .device-card の一覧 -->
</div>
```

**Handler 側（Gin、HX-Trigger 複数イベント返却）:**

```go
func (h *DeviceHandler) Destroy(c *gin.Context) {
	deviceID, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "不正なデバイスIDです")
		return
	}

	// トランザクションで子（計測・アラートルール）→親（デバイス）の順に削除
	if err := h.repo.DeleteDeviceCascade(c.Request.Context(), deviceID); err != nil {
		c.String(http.StatusInternalServerError, "デバイスの削除に失敗しました")
		return
	}

	if c.GetHeader("HX-Request") != "" {
		// 複数イベントをまとめて発火: モーダルを閉じて一覧を再取得
		c.Header("HX-Trigger", `{"modal-close":"","device-list-refresh":""}`)
		c.Status(http.StatusNoContent)
		return
	}

	c.Redirect(http.StatusSeeOther, "/devices")
}
```

### hx-confirm との使い分け

| 判断基準 | hx-confirm（§11・C06） | fetch + htmx.trigger（本§24） |
|---------|----------------------|-------------------------------|
| 確認内容 | 「〇〇を削除しますか？」のみ | デバイス名・注意書き等の詳細表示が必要 |
| UIカスタマイズ | 不要（ブラウザネイティブ） | 必要（プロジェクト固有のデザイン） |
| 動的データ | 不要 or 文字列埋め込みで十分 | Alpine.js `selectedDevice` で動的管理 |
| 推奨度 | **デフォルト** | 上記条件を満たす場合のみ |

### 注意事項

- fetch の `headers` に `'HX-Request': 'true'` を含めないと、Handler 側の HTMX/非HTMX 分岐（`c.GetHeader("HX-Request")`）が正しく動作しない
- `htmx.trigger(document.body, 'event-name')` は HTMX のグローバルAPIで、`hx-trigger="event-name from:body"` でリスンする
- CSRF トークンは `<meta name="csrf-token">` から取得する（Handler が `App.templ` 経由で埋め込んだもの）

**適用対象:** デバイス一覧の削除。将来、詳細な確認UIが必要な削除操作（アラートルールの削除等）がある場合も本パターンを適用。

---

## 25. form ネスト禁止と fetch による代替アップロード

> **cc-sdd への価値:**
> HTMLの `<form>` ネスト禁止制約への対応パターン。メインフォームの内部にアップロード用フォームを配置する画面（例: デバイス編集画面で設定ファイルやキャリブレーション CSV を添付するケース）で必須。姉妹文書「HTMLモック作成ルール.md」の R05・R22 と整合させること。

### 問題

HTMLの仕様で `<form>` 要素のネストは禁止されている。ブラウザは内側の `<form>` を無視するため、`new FormData(form)` が `TypeError: parameter 1 is not of type 'HTMLFormElement'` で失敗する。

```html
<!-- ❌ NG: formネスト — ブラウザが内側のformを無視する -->
<form method="POST" action="/devices/1/edit">
	<!-- CSRF hidden input -->
	<!-- メインフォームの中身 -->
	<form hx-post="/devices/1/upload" hx-encoding="multipart/form-data">
		<input type="file" name="file">  <!-- このformは無効 -->
	</form>
</form>
```

### 解決策: data属性 + fetch + FormData手動構築

`<form>` タグを使わず、`<input type="file">` の `data-*` 属性にURL・CSRFトークンを保持し、`FormData` を手動構築して `fetch` で送信する。

```templ
// ✅ OK: formタグなし — data属性でURL・トークンを保持
templ DeviceFileInput(uid, uploadURL, csrfToken string) {
	<input
		type="file"
		name="file"
		id={ uid + "-input" }
		data-upload-url={ uploadURL }
		data-csrf-token={ csrfToken }
		style="position:absolute;width:0;height:0;overflow:hidden;opacity:0"
	>
}
```

```javascript
// FormData を手動構築（formタグ不要）
function upload(file) {
	var formData = new FormData();
	formData.append('file', file);

	fetch(input.dataset.uploadUrl, {
		method: 'POST',
		body: formData,
		headers: {
			'X-CSRF-Token': input.dataset.csrfToken,
			'HX-Request': 'true',  // Handler の HTMX 分岐で必要
		},
	})
	.then(function(res) { /* tbody.innerHTML で差し替え */ });
}
```

### 注意事項

- `HX-Request: true` ヘッダーを付けないと、Handler の HTMX/非HTMX 分岐（`c.GetHeader("HX-Request")`）が正しく動作しない。フルページ templ ではなくサブコンポーネントを返したい場合は必須
- アップロード成功後のDOM更新は、Alpine.js コンポーネントを破壊しないよう `<tbody>` の `innerHTML` だけを差し替える（親の `x-data` div を差し替えない）
- `<input type="file">` を `display:none` にするとブラウザによっては `.click()` が無効になる。代わりに `position:absolute; width:0; height:0; overflow:hidden; opacity:0` を使用する
- CSRFトークンは `data-csrf-token` 属性に Handler が渡した値を埋め込む（CSRF ミドルウェアの採用ライブラリは要確認）

---

## 26. Alpine.js ネストスコープの $refs 制約

> **cc-sdd への価値:**
> templ コンポーネントが親画面の `x-data` 内に展開される場合に発生する Alpine.js の `$refs` 参照不能問題。ファイルアップロード・モーダル等、再利用コンポーネント全般に適用。

### 問題

Alpine.js の `$refs` は**自身の `x-data` スコープ内の `x-ref` のみ**を参照できる。ネストされた `x-data` の `$refs` からは親の `x-ref` も兄弟の `x-ref` も見えない。

```html
<!-- 親コンポーネント -->
<div x-data="{ ... }">
	<!-- 子コンポーネント（templ コンポーネント展開後） -->
	<div x-data="{ fn() { this.$refs.myRef } }">
		<!-- ❌ $refs.myRef は undefined — 親スコープの x-ref は見えない -->
	</div>
	<input x-ref="myRef">
</div>
```

### 解決策

#### A. `$el.querySelector()` パターン（同一 x-data 内で完結する場合）

```javascript
x-data="{
	getInput() { return this.$el.querySelector('input[type=file]'); }
}"
```

#### B. ピュアJS パターン（推奨 — Alpine.js 依存を完全排除）

Alpine.js に依存しないピュアJSで実装する。一意のIDを振り、`document.getElementById()` で直接参照する。templ では Handler 側で生成した一意IDを引数で受け取る。

```templ
templ FileUpload(uid string) {
	<div id={ uid }>
		<input type="file" id={ uid + "-input" }>
		<button id={ uid + "-btn" } class="btn btn-secondary btn-small" type="button">ファイル選択</button>
		<span id={ uid + "-status" }>選択されていません</span>
	</div>
	<script>
		(function() {
			var input = document.getElementById({ templ.JSONString(uid + "-input") });
			var btn = document.getElementById({ templ.JSONString(uid + "-btn") });
			btn.addEventListener('click', function(e) {
				e.stopPropagation();
				input.click();
			});
		})();
	</script>
}
```

> 一意ID（`uid`）は Handler 側で `crypto/rand` 等を使って生成し、templ 引数で渡す（例: `"fu-" + randomString(8)`）。同一画面に複数インスタンスが存在しても衝突しない。

### 判断基準

| 状況 | 推奨パターン |
|------|-------------|
| コンポーネントが必ず他の `x-data` 内に展開される | B（ピュアJS） |
| コンポーネントが独立して使われる | A（`$el.querySelector`）でも可 |
| 複数インスタンスが同一画面に存在しうる | B（ユニークID必須） |

---

## 27. Tom Select 初期化の plugins コンテキスト判定

> **cc-sdd への価値:**
> Tom Select の `remove_button` プラグインが検索フォームの single select で「すべて X」表示を引き起こす問題の防止パターン。デバイス一覧・アラート履歴等の検索/フィルタフォーム（`.filter-form`）に適用。Tom Select は本プロジェクトでも採用。姉妹文書「HTMLモック作成ルール.md」の TS02・TL12 と整合させること。

### 問題

`initTomSelect()` が全 `<select>` に `plugins: ['dropdown_input', 'remove_button']` を一律適用すると、検索/フィルタフォームの single select で空option（「すべて」等）がタグ風に `すべて X` と表示される。

### 解決策

モックの `tom-select-init.js`（姉妹文書 TS02）と同じコンテキスト判定ロジックを `App.templ` の `initTomSelect()` に適用する。

```javascript
function initTomSelect(scope) {
	var root = scope || document;
	root.querySelectorAll('select.js-tom-select:not([disabled]):not(.tomselected)')
		.forEach(function (el) {
			el.classList.add('tomselected');
			var isHeaderSelect = !!el.closest('.user-menu');
			var isFilterFormSingle = el.closest('.filter-form') && !el.multiple;
			var useDropdownInput = el.multiple
				|| (!isHeaderSelect && !isFilterFormSingle);

			new TomSelect(el, {
				allowEmptyOption: true,
				dropdownParent: 'body',
				plugins: el.multiple
					? ['remove_button', 'dropdown_input']
					: (useDropdownInput ? ['dropdown_input'] : []),
			});
		});
}
```

| コンテキスト | plugins |
|---|---|
| ヘッダーの select（`.user-menu`） | `[]` |
| フィルタフォーム内の single select（`.filter-form`） | `[]` |
| multiple select | `['remove_button', 'dropdown_input']` |
| その他の single select | `['dropdown_input']` |

> このスクリプトはフレームワーク非依存のため、モック（姉妹文書 TS02・TL12）からそのまま `App.templ` 末尾の `<script>` ブロックへ移植してよい。

---

## 29. フルスクリーンモーダルの flex column 全階層伝播

> **cc-sdd への価値:**
> デバイス削除確認モーダルや一覧選択モーダル等、フルスクリーン表示のモーダルでフッター（キャンセル・確定ボタン）が画面外に隠れる問題の防止パターン。全フルスクリーンモーダルに適用する汎用 CSS。

### 問題

`.modal-overlay .modal-content` に `max-height` と `overflow: auto` を設定しても、コンテンツが長いとフッターがスクロール外に隠れる。ヘッダー・フッター固定＋ボディのみスクロールが実現できない。

### 原因

モーダルの DOM 構造は複数階層にネストしている：

```
.modal-overlay                  ← ①
  .modal-content                ← ②
    #modal-content              ← ③（HTMX がフラグメントを挿入する領域）
      div[x-data]               ← ④
        .modal-head             ← ヘッダー
        .modal-body             ← ボディ
        .modal-foot             ← フッター
```

flex column で「ボディだけスクロール」を実現するには **①から④まで全階層**に高さ制約を伝播させる必要がある。途中の階層（③や④）で `display: flex; flex-direction: column` が抜けると、その子要素は flex 制約を受けずコンテンツの自然な高さで描画される。

### 解決策

```css
/* ①: オーバーレイ自体を flex 配置 */
.modal-overlay.fullscreen {
  display: flex;
  align-items: flex-start;
  justify-content: center;
}

/* ②: 高さ固定 + flex column + overflow 制限 */
.modal-overlay.fullscreen .modal-content {
  height: 96vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* ③④: flex 伝播（全中間層に必要） */
.modal-overlay.fullscreen #modal-content,
.modal-overlay.fullscreen #modal-content > div {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;  /* ← これがないと flex child が縮まない */
}

/* ヘッダー・フッター固定 */
.modal-overlay.fullscreen .modal-head { flex-shrink: 0; }
.modal-overlay.fullscreen .modal-foot { flex-shrink: 0; }

/* ボディのみスクロール */
.modal-overlay.fullscreen .modal-body {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}
```

### 注意事項

- `max-height` ではなく `height` を使用すること。`max-height` では中間層の flex 計算が正しく動作しない場合がある。
- `min-height: 0` は必須。flexbox のデフォルト `min-height: auto` では子要素が縮まない。
- このルールはユーティリティクラスでは表現しきれない多階層伝播なので、`style.css` の `@layer components` に汎用ルールとして 1 度だけ定義し、全フルスクリーンモーダルで使い回す。

---

## 30. `hx-on::after-swap` 内での `$dispatch` 不動作とグローバルハンドラ代替

> **cc-sdd への価値:**
> HTMX の `hx-on::after-swap` 属性内で Alpine.js の `$dispatch()` が動作しない問題の回避パターン。モーダル表示を伴う HTMX ボタン全般に適用。

### 問題

```html
<!-- ❌ $dispatch は HTMX の hx-on:: コンテキストでは未定義 -->
<button hx-get="/devices/1/readings"
        hx-target="#modal-content"
        hx-on::after-swap="$dispatch('modal-open-fullscreen')">計測値を表示</button>
```

`hx-on::after-swap` 内の JavaScript は HTMX のコンテキストで実行され、Alpine.js のマジックプロパティ（`$dispatch` 等）は使用できない。

### 解決策

個別ボタンの `hx-on::after-swap` に依存せず、`internal/view/layout/App.templ` に埋め込んだ `htmx:afterSwap` グローバルハンドラで `#modal-content` への swap を検知してモーダルを自動オープンする。

```javascript
// App.templ — htmx:afterSwap グローバルハンドラ
document.body.addEventListener('htmx:afterSwap', function (event) {
    var target = event.detail.target;
    if (target) {
        initTomSelect(target);
    }
    // #modal-content への swap 時はモーダルを自動で開く
    if (target && target.id === 'modal-content') {
        window.dispatchEvent(new CustomEvent('modal-open-fullscreen'));
    }
});
```

これにより、各ボタンは `hx-on::after-swap` を記述する必要がなくなる（記述しても害はないが不要）。

### 注意事項

- `window.dispatchEvent(new CustomEvent(...))` を使用すること（Alpine.js の `$dispatch` ではない）。
- モーダルを開かない HTMX 操作（例: 計測値一覧の差し替え）は `#modal-content` 以外をターゲットにすること。
- この JS は HTMX/Alpine いずれもフレームワーク非依存なので、Laravel 版からそのまま流用できる。配置先のみ `App.templ` に読み替える。

---

## 31. templ + 自前 CSS のクラス体系統一ルール

> **cc-sdd への価値:**
> モック HTML と templ コンポーネントで CSS クラス体系が異なると、同じ `style.css` を使用していてもスタイルが適用されない。全画面の templ 実装時に適用。

### ルール

templ コンポーネントを実装する際は、モック HTML（姉妹文書「HTML モック作成ルール.md」の R01〜R27 で定義）で使用されている CSS クラスを**そのまま使用**すること。独自クラスを新設しない。確定済みの実クラス（`.btn` `.btn-primary` `.card` `.data-table` `.device-card` `.device-grid` `.form-group` `.modal-content` `.modal-overlay` `.period-btn` `.rule-form` 等）の語彙から外れたクラス名を作らない。

### よくある違反パターン

| モックのクラス | 誤って使われるクラス | 影響 |
|-------------|----------------|------|
| `form-group` + `form-help` | `form-row` / `field-help` | フォーム行のレイアウト・補足テキストのスタイルが崩れる |
| `form-actions` | `actions-right` | フォーム下部ボタン群の配置が効かない |
| `btn btn-secondary` | `btn-secondary`（`btn` なし） | `border-radius` 等のベーススタイルが適用されない |
| `card` + `page-header` | `panel` / `header-block` | カード枠・見出し帯のスタイルが適用されない |
| `filter-form` | `search-row` | 絞り込みフォームのラベル・入力欄の横並びレイアウトが崩れる |

### チェック方法

templ コンポーネント作成後、対応するモック HTML の同一箇所と以下を比較すること：
1. 親要素のクラス名が一致しているか
2. ボタンに `btn` ベースクラスが含まれているか
3. Tom Select 対象の `<select>` に `js-tom-select` クラスがあるか

> 自前トークン（`var(--space-4)` 等）や `@layer utilities` のユーティリティクラス（`.u-mb-4` 等）でスペーシングや配色を委譲する場合も、上記の構造クラス（`.card` `.form-group` 等）は維持し、ユーティリティを併用する。CSS方針は .kiro/steering/tech.md 参照。CSS ファイル自体の単一ソース運用（正本=`mocks/html/style.css`・本番は生成物）は §40-B「単一ソース運用」を参照。

### 31.1 写経の境界 — handler 合成の動的文字列はサンプル文言の逐語写経対象ではない 📅 2026-06-07 — S3 dashboard で確立

モック HTML の**構造・CSS クラス・静的な固定文言**（ページ見出し「ダッシュボード」、空メッセージ「登録されたデバイスはありません。…」等）は写経の対象。一方、templ が `{ 変数 }` で描画する **handler 合成の動的文字列**（例: 未対応アラートの「{デバイス名}: {指標}が{閾値}{単位}を超えました（{実測値}{単位}）」）は、モックに置かれたサンプル値を**逐語コピーする対象ではない**。合成書式は設計（`composeAlertMessage` の仕様）が正であり、モックは「見た目の一例」にすぎない。

両者が食い違う場合（例: モックは半角括弧 `(38.50℃)`、設計の合成書式は全角 `（38.50℃）`）は、**設計の合成書式を正準**とし、モック側のサンプル文言を設計に合わせて更新して単一ソースを保つ（モックの構造・クラスは引き続き正）。テストは合成関数の出力を、更新後のモック文言・設計例と完全一致で固定する。「モックと一致」の語に引きずられて handler 合成文字列までモックの見た目へ寄せない（写経対象は静的 HTML の構造・クラス・固定文言まで）。

---

## 32. `x-data` 属性内への JSON 受け渡し（script タグ + JSON.parse 方式）

> **cc-sdd への価値:**
> `x-data` 内に生 JSON を埋め込むと HTML 属性値のクォートと衝突し、Alpine.js が初期化に失敗してボタンが動作しなくなる問題の防止パターン。`x-data` に JSON 配列/オブジェクトを渡す全画面に適用。姉妹文書「HTML モック作成ルール.md」TL02 と対応する。

### 問題

templ の `{ jsonString }` でエスケープなしの生 JSON を `x-data="..."` 属性に直接埋め込むと、JSON 内のダブルクォートが属性のダブルクォートと衝突し、Alpine.js のパースエラー（`Unexpected token`）が発生する。

```templ
// ❌ NG: 生 JSON のダブルクォートが x-data="" と衝突
<div x-data={ "{ items: " + readingsJSON + " }" }>
```

シングルクォート `x-data='...'` に変更しても、JSON の値にシングルクォートが含まれる場合に同じ問題が発生する。

### 解決策: `<script type="application/json">` + `JSON.parse`

JSON データは属性に連結せず、`<script type="application/json">` ブロックに出力し、Alpine 側は `JSON.parse` で読み込む。templ は `<script type="application/json">` の中身を `@templ.JSONScript` あるいは `{ jsonString }` で安全に出力できる（HTML エスケープは script 文脈に応じて行う）。

```templ
templ ReadingsChart(deviceID int, readingsJSON string) {
	<div id={ fmt.Sprintf("readings-%d", deviceID) }
		x-data="{ readings: JSON.parse($refs.data.textContent) }">
		<script type="application/json" x-ref="data">
			@templ.Raw(readingsJSON)
		</script>
		<canvas class="chart-placeholder"></canvas>
	</div>
}
```

Handler 側で JSON 文字列を生成して渡す。

```go
// Handler: 計測値スライスを JSON 化して templ へ渡す
b, err := json.Marshal(readings)
if err != nil {
	c.String(http.StatusInternalServerError, "計測値のシリアライズに失敗しました")
	return
}
component.ReadingsChart(deviceID, string(b)).Render(c.Request.Context(), c.Writer)
```

### ルール

| データ型 | 推奨方法 | 例 |
|---------|---------|---|
| JSON 配列・オブジェクト | `<script type="application/json">` + `JSON.parse` | 計測値配列、アラートルール一覧 |
| 単純な文字列 | `x-data` 内に `{ value }` を引用符付きで埋め込む | `currentPeriod: '24h'` |
| boolean | Go 側で `"true"`/`"false"` 文字列を渡す | `open: false` |
| 数値・null | Go 側で文字列化して渡す（未設定は `null`） | `deviceId: 1` |

### 注意事項

- 生 JSON を `x-data` 等の属性値に直接連結しないこと。属性衝突の温床になる。
- script タグ方式なら配列・ネストオブジェクト・日本語ラベルを含む値でも安全に渡せる。
- Go では Laravel の `@json()` / `Js::from()` のようなヘルパーは無いため、Handler で `json.Marshal` した結果を templ コンポーネントへ**引数で明示的に渡す**。

---

## 39. 絞り込みフォーム内ボタンの `hx-push-url` 継承問題

> **cc-sdd への価値:**
> 絞り込みフォーム（`.filter-form`）内に配置したモーダル呼び出しボタンが、フォームの `hx-push-url="true"` を継承し、URL が変わってフルページ遷移してしまう問題の再発防止。

### 問題

計測値一覧やアラート履歴の絞り込みフォーム `<form>` は `hx-push-url="true"` を持つ（絞り込み条件を URL に反映するため）。内部に `hx-get` ボタンを配置すると、HTMX の属性継承によりボタンも `hx-push-url="true"` になり、モーダル用 HTML をページ全体として読み込んでしまう。

### 解決策

絞り込みフォーム内のモーダル呼び出しボタンには `hx-push-url="false"` と `hx-indicator=""` を明示的に指定する。

```templ
{{-- ❌ 親 form の hx-push-url を継承して URL が変わる --}}
<button type="button" class="btn btn-secondary"
	hx-get="/devices/1/edit" hx-target="#modal-content" hx-swap="innerHTML">編集</button>

{{-- ✅ hx-push-url と hx-indicator を上書き --}}
<button type="button" class="btn btn-secondary"
	hx-get="/devices/1/edit" hx-target="#modal-content" hx-swap="innerHTML"
	hx-push-url="false" hx-indicator="">編集</button>
```

### HTMX 属性継承の注意

HTMX は以下の属性を親要素から自動継承する: `hx-target`, `hx-swap`, `hx-push-url`, `hx-indicator`, `hx-confirm`, `hx-boost` 等。子要素で異なる挙動が必要な場合は明示的に上書きすること。

---

## 40. モーダル呼び出しの `hx-target` は `#modal-content` を使用する

> **cc-sdd への価値:**
> `.modal-overlay`（コンテナ）をターゲットにするとモーダルが自動オープンしない問題の再発防止。

### 問題

グローバルの `htmx:afterSwap` ハンドラ（§30）は `target.id === 'modal-content'` の場合のみ `modal-open-fullscreen` イベントを発火する。コンテナ要素をターゲットにすると、スワップは成功するがモーダルが開かない。

### ルール

モーダルを HTMX で開くボタンは **常に `hx-target="#modal-content"`** を使用する。

```templ
{{-- ❌ モーダルが開かない --}}
<button hx-get="/devices/1/edit" hx-target=".modal-overlay" hx-swap="innerHTML">編集</button>

{{-- ✅ グローバルハンドラでモーダルが自動オープン --}}
<button hx-get="/devices/1/edit" hx-target="#modal-content" hx-swap="innerHTML">編集</button>
```

### DOM 構造

```
.modal-overlay            ← Alpine.js x-data でモーダル開閉を制御
  .modal-content          ← フルスクリーンモーダルのフレーム
    #modal-content        ← ★ここに HTMX でフラグメントを挿入する
```

`#modal-content` の `<div id="modal-content">` は `App.templ`（共通レイアウト）に 1 つだけ配置する。個別画面の templ に同じ id の div を追加してはならない（id 重複で HTMX がレイアウト外の空 div にコンテンツを挿入し、白い画面になる）。

---

## 41. モーダル選択結果のイベントリスナー実装パターン

> **cc-sdd への価値:**
> モーダルで値を選択しても呼び出し元に反映されない問題の再発防止。汎用的な Alpine イベント連携パターン。

### 問題

デバイス選択モーダル（例: アラートルール作成時の対象デバイス指定）は `$dispatch('device-selected', { id, deviceName })` でイベントを発火するが、呼び出し元のフォームにリスナーがないと選択結果が無視される。

### 解決策

呼び出し元のフォームグループに `x-data` + `@{event-name}.window` リスナーを追加し、表示用テキストと hidden input の両方を更新する。

```templ
<div class="form-group"
	x-data="{ deviceName: '' }"
	@device-selected.window="
		deviceName = $event.detail.deviceName;
		$refs.deviceId.value = $event.detail.id;
	">
	<label class="form-label">対象デバイス</label>
	<div class="device-select-row">
		{{-- 表示用（readonly） --}}
		<input type="text" name="device_name"
			class="form-control" readonly
			:value="deviceName">
		{{-- 送信用（hidden） --}}
		<input type="hidden" name="device_id"
			x-ref="deviceId">
		{{-- モーダル呼び出し --}}
		<button type="button" class="btn btn-secondary"
			hx-get="/devices/select" hx-target="#modal-content" hx-swap="innerHTML"
			hx-push-url="false" hx-indicator="">指定</button>
	</div>
</div>
```

選択モーダル側（フラグメント）は確定ボタンで `device-selected` イベントを `window` に発火する。

```templ
<button type="button" class="btn btn-primary"
	@click="$dispatch('device-selected', { id: 1, deviceName: 'ハウスA温湿度計' })">
	このデバイスを選択
</button>
```

### チェックリスト

モーダル選択ボタンを設置する際は以下を確認:
1. ボタンに `hx-target="#modal-content"` が指定されている（§40）
2. ボタンに `hx-push-url="false"` が指定されている（§39）
3. 呼び出し元に `@{event-name}.window` リスナーがある
4. リスナー内で表示用 input **と** hidden input の両方を更新している

---

## 42. サイドバー項目の `path` 未設定によるリンク無効化

> **cc-sdd への価値:**
> サイドバー定義に遷移先パスを設定し忘れると、サイドバー項目がクリックしても何も起きない `<button>` になる問題の再発防止。

### 問題

サイドバー項目のレンダリングは、各項目の `path` が空の場合 `<a>` ではなく `<button>` を出力する設計になりやすい。`path` 未設定の項目はクリックしても遷移しない。

### ルール

新しいサイドバー項目を追加する際は、遷移先 `path` を必ず設定する。templ のサイドバーコンポーネントは項目構造体を受け取り、`Path` が空でなければ `<a href={ item.Path }>` を、空なら `<button>` をレンダリングする。Handler 側で `path` を埋め忘れないこと。

```go
// ❌ Path 未設定 → <button> になりリンク不能
{Key: "alert_rules", Label: "アラートルール"},

// ✅ Path 設定済み → <a href="/alerts/rules"> になりリンク有効
{Key: "alert_rules", Label: "アラートルール", Path: "/alerts/rules"},
```

### 現在のサイドバー構成

| key | label | path |
|-----|-------|------|
| dashboard | ダッシュボード | `/dashboard` |
| alert_rules | アラートルール | `/alerts/rules` |
| alert_history | アラート履歴 | `/alerts/history` |

---

---

## 43. タブ切替（期間切替）によるサーバー側条件分岐パターン

> **cc-sdd への価値:**
> デバイス詳細画面（`/devices/{device}`）の計測値グラフで期間（24h / 3d / 7d / 30d）を切り替える際の Handler 設計・HTMX 統合パターン。

### パターン

`?period=24h` / `?period=7d` / `?period=30d` のようなクエリパラメータで表示期間を切り替え、Handler 側で条件分岐して異なる集計データを返却する。HX-Request 時はグラフ領域のサブコンポーネントのみを差し替える。

```go
// Handler: 期間切替
func (h *DeviceHandler) Readings(c *gin.Context) {
	deviceID := /* パスパラメータから取得 */

	period := c.DefaultQuery("period", "24h")
	var since time.Time
	switch period {
	case "7d":
		since = time.Now().AddDate(0, 0, -7)
	case "30d":
		since = time.Now().AddDate(0, 0, -30)
	default:
		period = "24h"
		since = time.Now().Add(-24 * time.Hour)
	}

	readings, err := h.repo.ListReadingsSince(c.Request.Context(), repository.ListReadingsSinceParams{
		DeviceID: int32(deviceID),
		Since:    since,
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "計測値の取得に失敗しました")
		return
	}

	b, _ := json.Marshal(readings)

	// HX-Request 時はグラフ領域のサブコンポーネントだけを差し替える
	if c.GetHeader("HX-Request") != "" {
		component.ReadingsChart(deviceID, period, string(b)).
			Render(c.Request.Context(), c.Writer)
		return
	}
	// 通常アクセス時はフルページを返す
	page.DeviceShow(deviceID, period, string(b)).
		Render(c.Request.Context(), c.Writer)
}
```

```templ
{{-- ReadingsChart サブコンポーネント（期間ボタン + グラフ） --}}
templ ReadingsChart(deviceID int, period string, readingsJSON string) {
	<div class="period-selector">
		for _, p := range []string{"24h", "7d", "30d"} {
			<button
				class={ "period-btn", templ.KV("active", p == period) }
				hx-get={ fmt.Sprintf("/devices/%d/readings?period=%s", deviceID, p) }
				hx-target="#readings-chart"
				hx-swap="outerHTML"
				hx-push-url="true">
				{ p }
			</button>
		}
	</div>
	<div id="readings-chart" x-data="{ readings: JSON.parse($refs.data.textContent) }">
		<script type="application/json" x-ref="data">
			@templ.Raw(readingsJSON)
		</script>
		<canvas class="chart-placeholder"></canvas>
	</div>
}
```

### 設計判断ポイント

| 項目 | 判断基準 |
|------|---------|
| サーバー側 vs クライアント側 | 同一データセットの単純な表示切替ならクライアント側（Alpine.js）でも可。期間ごとに異なる集計クエリ（24h/3d/7d/30d で粒度が変わる）が必要ならサーバー側 |
| URL 反映 | 期間ボタンに `hx-push-url="true"` を付け、リロード・共有時に期間が復元されるようにする |
| 差し替え範囲 | 期間ボタンを含めて差し替えると active 状態も同時更新できるため、ボタン＋グラフを 1 つのサブコンポーネントにまとめ `hx-swap="outerHTML"` で置換する |
| active 表現 | 状態クラス `active` を `templ.KV` で付与する（class のみでスタイリングし、id はスタイルに使わない） |

### 補足: フラグメント取得を専用ルートに分離したら `hx-get` と `hx-push-url` を別 URL にする 📅 2026-06-08 — device-detail で確立

> 本節冒頭の例は **1 本のルートを `HX-Request` ヘッダで分岐**（HTMX→フラグメント / 通常→フルページ）し、`hx-get` と同じ URL を `hx-push-url="true"` で履歴に積む。この方式ならリロード時も同 URL がフルページを返すため問題ない。一方 device-detail のように **フラグメント取得を専用ルート（レイアウト無しの部分 HTML だけを返す `/devices/{id}/chart`）に分離**した場合は、その書き方をそのまま使うと壊れる。

`hx-push-url="true"` は **`hx-get` の URL をそのまま履歴に積む**。フラグメント専用ルートを `hx-get` に指定したまま `="true"` で push すると、その部分 HTML 専用 URL が履歴に入り、リロード・URL 共有・ブックマークからの直アクセスでブラウザに**レイアウト無しの部分 HTML だけ**が返って画面が崩れる。

対策は **2 系統の URL を用意し、push 側だけフルページ URL に向ける**こと:

- `hx-get` = フラグメント取得 URL（`/devices/{id}/chart?period=…`、レイアウト無し）
- `hx-push-url` = ブックマーク可能なフルページ URL（`/devices/{id}?period=…`、文字列で明示。`"true"` ではない）

```templ
for _, p := range chartPeriods {
    <button
        type="button"
        class={ "period-btn", templ.KV("active", p.Value == v.Period) }
        hx-get={ fmt.Sprintf("/devices/%d/chart?period=%s", v.DeviceID, p.Value) }
        hx-target="#device-chart-area"
        hx-swap="innerHTML"
        hx-push-url={ fmt.Sprintf("/devices/%d?period=%s", v.DeviceID, p.Value) }
    >{ p.Label }</button>
}
```

**前提（必須）:** push したフルページ URL が直アクセスでも成立するよう、フルページ Handler（`Show`）側も任意の `?period` を読んで初期描画できること。device-detail の実装では:

- `Show`（フルページ）: `period := c.Query("period")` を任意で受け、未指定は既定 `24h`、不正値は 400（`isValidPeriod` で 24h/3d/7d/30d を検証）。
- `Chart`（フラグメント専用）: `ShouldBindQuery` + `binding:"required,oneof=24h 3d 7d 30d"` で必須・列挙検証（不正/未指定→400）。

> **使い分け:** 単一ルート + `HX-Request` 分岐なら `hx-push-url="true"` でよい（同 URL が両方を返す）。フラグメント取得を別ルートに分離した場合のみ、push 先をフルページ URL に明示する。

## 22-B. モーダル内の hx-target 使い分け（全体差替え vs 部分差替え）

> 📅 デバイス削除確認モーダル・センサー選択モーダルの操作設計で確立。

### パターン

モーダル内で HTMX リクエストを送信する際、操作の種類によって `hx-target` を使い分ける。

| 操作 | hx-target | 理由 |
|------|-----------|------|
| デバイス切替（対象セレクター） | `#modal-content` | 関連マスターデータ（センサー一覧・期間プリセット等）を再取得する必要があるため、モーダル全体を差替え |
| 検索・ソート・ページ切替 | `#device-readings-list` | 結果テーブルのみ更新。検索条件エリアの入力値・Tom Select の状態を保持 |
| 表示件数変更 | `#device-readings-list` | 検索と同様、結果テーブルのみ更新 |

### templ 実装

部分更新領域は `device-readings-list` のような独立した templ コンポーネント関数に分割し、HTMX 時は Handler がそのコンポーネントを直接 `Render` する（フルページ templ も同じコンポーネントを呼ぶ）。

```templ
// デバイス切替: モーダル全体を差替え
<select name="device_id" class="js-tom-select"
        hx-get="/devices/select"
        hx-target="#modal-content"
        hx-include="[name='target_field']">
    for _, d := range devices {
        <option value={ strconv.FormatInt(d.ID, 10) }>{ d.Name }</option>
    }
</select>

// 検索ボタン: 結果エリアのみ差替え
<button type="button" class="btn btn-secondary"
        hx-get="/devices/select"
        hx-target="#device-readings-list"
        hx-include=".filter-form input, .filter-form select, [name='device_id'], [name='target_field']">
    検索
</button>

// ソート列ヘッダー: 結果エリアのみ差替え
<th hx-get="/devices/select"
    hx-target="#device-readings-list"
    hx-vals={ `{"sort":"measured_at","order":"desc"}` }
    hx-include="[name='device_id'], [name='target_field'], [name='per_page']">
    計測日時
</th>
```

### 補足

- この使い分けは §40-B（CSS キャッシュバスティングとは別テーマ）および §50 の form-modal ルールと整合する。フォーム送信でモーダル構造を返す場合は `#modal-content` を差替える（§50 参照）。
- 部分差替えのターゲット ID は本ドメインで自然な名前（`device-readings-list` / `alert-history-list` 等）を使う。

---

## 49. Alpine.js `x-for` 配列追加時のスクロール制御

> 📅 アラートルール登録・編集モーダルで「条件追加ボタンが効かないように見える」問題を調査した際に確立。

### 背景

Alpine.js の `x-for` で描画されるリスト（条件行等）に `rules.push(...)` で新規アイテムを追加すると、新しい行はリストの末尾に追加される。スクロール可能なコンテナ（`overflow-y: auto` のモーダル本体等）では、追加された行が画面外にあるためユーザーには「ボタンが動かない」ように見える。

### 知見

1. **`$nextTick` は `x-for` の展開完了を保証しない場合がある** — Alpine の `$nextTick` はリアクティブ更新のスケジュール後に実行されるが、`x-for` のテンプレート展開（DOM 要素生成）が完了するタイミングとは一致しないことがある。

2. **`$refs` を closure 経由で `setTimeout` に渡すと null になる** — `$refs` はプロキシオブジェクトであり、`var self = this` で保存した参照を `setTimeout` 内で読むと null になる場合がある。事前に DOM 要素を変数へキャプチャすること。

3. **`setTimeout(50ms)` + DOM キャプチャが確実な回避策**:

```javascript
addRule: function () {
  var form = this.$refs.saveForm; // DOM要素を事前キャプチャ
  this.rules.push({ id: this.nextRuleId--, metric: 'temperature', operator: '>', threshold: '' });
  setTimeout(function () {
    if (form) {
      form.scrollTop = form.scrollHeight;
    }
  }, 50);
},
```

### 注意事項

- `setTimeout` の遅延は 50ms で十分（Alpine の DOM 更新サイクルは通常数十ms以内）。
- `scrollTop = scrollHeight` は末尾への強制スクロール。特定行へ移動する場合は `scrollIntoView({ block: 'nearest', behavior: 'smooth' })` を使う。
- このスクリプトはフレームワーク非依存。配置先は `App.templ` 内の Alpine コンポーネント定義に読み替える。

**適用対象:** アラートルール登録・編集モーダル（条件行の追加処理）

---

## 50. form-modal の hx-target は `#modal-content` を使用する

> 📅 デバイス登録モーダルのテストで発見。

### 背景

form-modal コンポーネント（ヘッダー+ボディ+フッターを含む）の `<form>` に `hx-target="closest .modal-body"` を指定すると、バリデーションエラー（422 レスポンス）で返した templ コンポーネントが `.modal-body` **内部**に差し込まれる。返却フラグメントには form-modal 全体が含まれるため、モーダルが**無限にネスト**する。

### ルール

form-modal を返す送信では必ず `hx-target="#modal-content"` / `hx-swap="innerHTML"` を使う。部分差替え（検索結果エリアのみ更新等）が必要な場合は form-modal ではなく個別のコンポーネント ID を指定する。

詳細な原則は §40-B ではなく §22-B（モーダル内の hx-target 使い分け）に集約済み。本セクションはフォーム送信時の注意点のみを示す。

```templ
// ✅ 正しい: #modal-content に差し替え（モーダル構造をネストさせない）
<form hx-post={ action }
      hx-target="#modal-content"
      hx-swap="innerHTML">
```

---

## 51. Alpine.js `x-show` と CSS `display: flex !important` の競合

> 📅 削除確認ダイアログの表示制御で発見。

### 背景

Alpine.js の `x-show` はインラインスタイル `display: none` / `display: ""` で表示を制御する。CSS 側に `display: flex !important` を指定すると、`x-show="false"` 時の `display: none` が上書きされ、要素が**常に表示**されてしまう。

### ルール

`x-show` で制御する要素に `display` の `!important` を使わない。代わりに `style` 属性の値で条件分岐する。本プロジェクトでは `.modal-overlay`（削除確認モーダル等）がこの対象になる。

```css
/* ✅ x-show 互換: style属性で条件分岐 */
.modal-overlay[style*="display: none"] {
  display: none !important;
}
.modal-overlay:not([style*="display: none"]) {
  display: flex !important;
}

/* ❌ 誤り: x-show の display:none を上書きしてしまう */
.modal-overlay {
  display: flex !important;
}
```

### 補足

- この問題は `display: flex` / `display: grid` などブロック以外のレイアウトモードで発生する。
- `display: block !important` の場合は `x-show` と競合しない（Alpine が `display: none` → `display: ""` のデフォルト復帰で `block` に戻るため）。

---

## 52. `$dispatch` イベントの `detail` プロパティ名は camelCase に統一する

> 📅 センサー選択モーダルで、選択した対象が別フィールドにも反映される不具合として発見。

### 背景

Alpine.js の `$dispatch('event-name', { targetField: 'device_id' })` でカスタムイベントを発火する際、`detail` オブジェクトのプロパティ名を **camelCase と snake_case で混在**させると、リスナー側でプロパティが `undefined` になる。

### 実際の障害

```javascript
// ディスパッチ側（device-select-modal）
$dispatch('item-selected', { ...selectedRow, targetField })
// → detail.targetField = 'device_id'

// リスナー側
if ($event.detail.target_field === 'device_id')
// → detail.target_field = undefined → 条件不一致 → 値が反映されない
```

さらに、別パネルのリスナーが `targetField` フィルタなしで全イベントを受信していたため、あるフィールドの選択が無関係なフィールドにも反映された。

### ルール

1. **`$dispatch` の `detail` プロパティ名は camelCase で統一する**（`targetField`, `deviceId` 等）。
2. **リスナー側も camelCase で参照する**（`$event.detail.targetField`）。
3. **リスナーは必ず `targetField` でフィルタする** — 同一イベント名を複数コンポーネントが受信する場合、自分宛でないイベントを無視する:

```html
<!-- ✅ 正しい: targetField でフィルタ -->
<div @item-selected.window="if ($event.detail.targetField === 'device_id') { deviceId = $event.detail.id }"></div>

<!-- ❌ 誤り: 全イベントを無条件に受信 -->
<div @item-selected.window="deviceId = $event.detail.id"></div>
```

### 補足

- イベント名は kebab-case、`detail` プロパティ名は camelCase という2層の命名規約を守る。
- この Alpine スクリプトはフレームワーク非依存なので、templ の属性へそのまま記述できる（`@event.window` 形式）。

---

## 40-B. CSS キャッシュバスティング

> 📅 disabled グレーアウト CSS が反映されない問題で発覚。

### 問題

CSS を固定パスで読み込んでいる場合、ファイルを更新してもブラウザキャッシュにより古い CSS が使われ続ける。開発中に追加した CSS ルールが反映されず、原因調査に時間を浪費する。

### 対策（Go + go:embed）

Laravel の Vite / mix / `asset()` ＋ `filemtime()` は本プロジェクトに存在しない。Go では `public/` 配下を `go:embed` で埋め込み、`<link>` の `href` にバージョンクエリを付与してキャッシュを無効化する。バージョン値はビルド時に埋め込む文字列、または埋め込みファイルの更新時刻ハッシュを使う。

```go
// internal/view/asset/asset.go
package asset

// ビルド時に -ldflags で注入する（例: -X .../asset.Version=$(git rev-parse --short HEAD)）
var Version = "dev"

// CSS のバージョン付き URL を返す
func CSSURL() string {
    return "/static/style.css?v=" + Version
}
```

```templ
// App.templ — Handler から渡された cssURL を埋め込む
templ App(cssURL string, content templ.Component) {
    <head>
        <link rel="stylesheet" href={ cssURL }/>
    </head>
    // ...
}
```

### 補足

- バージョン値はリリース単位で変わればよいので、Git の short SHA をビルド時に注入するのが簡潔。
- 開発時にホットリロードしたい場合は、`go:embed` ではなく `http.FileServer(http.Dir("public"))` で配信し、ファイル更新時刻（`os.Stat` の `ModTime().Unix()`）をクエリに付ける方法でも代替できる。本番は埋め込み＋固定バージョンを推奨。

### 単一ソース運用（mocks/html/style.css が唯一の正本・canonical パス確定）

> **cc-sdd への価値:** モック↔本番の CSS 乖離を構造的に防ぎ（保守性）、実装時に CSS を再生成させない（流用）。**本節が CSS の配置・配信の canonical 定義**であり、本セクション上部のコード例（`asset.go`・`/static/style.css` 等）のパスは本節に合わせる。

- **正本は `mocks/html/style.css` の1ファイルのみ**（モック作成ルール準拠・相対 `./style.css`）。本番配信ファイルは正本から**生成**し、手で書かない（双方向同期はしない／「両方直す」運用は不要）。
- **生成**: `make sync-css` が `mocks/html/style.css` → `internal/view/public/css/style.css` を複製（`build`/`dev` の前段で自動実行・生成物は `.gitignore`）。
- **埋め込み**: `internal/view/static.go` の `//go:embed all:public`。**go:embed は親ディレクトリ（`..`）やシンボリックリンクを辿れない**ため、public は必ず embed する Go ファイルと同階層＝`internal/view/public/` に置く（リポジトリ直下の `public/` は埋め込めない）。
- **配信**: `r.StaticFS("/static", http.FS(staticFS))` → `/static/css/style.css`。
- **参照**: layout（`App.templ`）で `CSSURL()`（= `/static/css/style.css?v=<Version>`）を `<link href>` に渡す。`assetURL("css/style.css")` 表記も同義。
- **追記運用**: コンポーネント固有 CSS（`.htmx-indicator`・fullscreen-modal の `@layer components` 等）は**正本 `mocks/html/style.css` の `@layer` 内に追記**し、`make sync-css` で本番へ反映。本番ファイルへ直接追記しない。

| 役割 | パス | 編集可否 |
|---|---|---|
| 正本（編集する） | `mocks/html/style.css` | ✅ ここだけ編集 |
| 配信用（生成物） | `internal/view/public/css/style.css` | ❌ 手編集禁止（`make sync-css` 生成・gitignore） |
| 埋め込み配線 | `internal/view/static.go`（`//go:embed all:public`） | 実装時に作成 |

---

## 53. `fieldset` のブラウザデフォルトスタイルリセット

> 📅 `fieldset[disabled]` 使用時にグレーの枠線が表示される問題として発覚。

### 問題

`<fieldset>` にはブラウザデフォルトの `border`・`margin`・`padding` が設定されている。グループ単位の disabled 制御で `<fieldset disabled>` を使うと、予期しないグレー枠線と余白が表示される。

### 対策

リセットを CSS に追加する:

```css
fieldset {
  border: none;
  margin: 0;
  padding: 0;
  min-inline-size: auto;
}
```

### 補足

- `min-inline-size: auto` は Firefox で fieldset がコンテンツ幅に収縮する問題を防ぐ。
- このリセットは全画面に影響するため、CSS の冒頭（リセット領域）に配置すること。
- 姉妹文書「HTMLモック作成ルール.md」TL03（共通スタイルのリセット規約）と整合させる。

---

## 54. `fieldset[disabled]` 内の入力欄グレーアウト CSS

> 📅 disabled パネルの入力欄が視覚的に入力可能に見える問題として発覚。

### 問題

ブラウザデフォルトの `disabled` スタイルはブラウザ差異が大きく、特に Chromium 系ではグレーアウトが薄い。ユーザーが入力不可エリアを入力可能と誤認する。

### 対策

明示的にグレーアウトスタイルを定義する:

```css
fieldset[disabled] input,
fieldset[disabled] select,
fieldset[disabled] textarea,
fieldset[disabled] .ts-wrapper.disabled {
  background-color: #f0f0f0 !important;
  color: #999 !important;
  cursor: not-allowed;
}

fieldset[disabled] .ts-wrapper.disabled .ts-control {
  background-color: #f0f0f0 !important;
  color: #999 !important;
}
```

### 補足

- Tom Select は `fieldset[disabled]` 内でも独自の `.ts-wrapper` を生成するため、別途セレクタが必要。
- Tom Select 初期化時に `fieldset[disabled]` 内かを検出し `ts.disable()` を呼ぶ（`App.templ` の initTomSelect 参照）。

---

## 55. Alpine.js `@click.outside` とトグルボタンの配置関係

> 📅 ユーザーメニュー（ヘッダー右上）がクリックしても即座に閉じる問題として発覚。

### 問題

`@click.outside` をメニュー div に配置すると、トグルボタンがメニューの「外」として判定される。ボタンクリック → メニュー open → 即座に `@click.outside` 発火 → メニュー close、というループが発生する。

### 対策

`@click.outside` はメニュー div ではなく、**ボタンとメニューの両方を含む親 div** に配置する:

```html
<!-- ❌ 誤り: メニューdivに@click.outside → ボタンクリックで即閉じ -->
<div style="position: relative;">
    <button @click="menuOpen = !menuOpen">メニュー</button>
    <div x-show="menuOpen" @click.outside="menuOpen = false">
        <!-- メニュー内容 -->
    </div>
</div>

<!-- ✅ 正しい: 親divに@click.outside → ボタンは「内側」扱い -->
<div style="position: relative;" @click.outside="menuOpen = false">
    <button @click="menuOpen = !menuOpen">メニュー</button>
    <div x-show="menuOpen">
        <!-- メニュー内容 -->
    </div>
</div>
```

### 補足

- 同じパターンは `.user-menu`（ユーザーメニュー）、通知ドロップダウン等にも適用される。
- この Alpine スクリプトはフレームワーク非依存なので templ にそのまま記述できる。

---

## 56. Handler 共通のフラグメント返却ヘルパー

> **cc-sdd への価値:**
> 共通の Handler ヘルパー（`internal/handler/respond.go` 等）に置く3つのメソッドは、§1.1 の「HTMX 時はサブコンポーネントを直接 Render、通常時はフルページを Render」という分岐を簡潔にラップする。新規 Handler で一貫したフラグメント返却を行うために使用する。

### 56.1 `respondWithFragment()` — 単一フラグメント返却

HTMX リクエスト時はサブコンポーネント（部分更新領域）のみ、通常リクエスト時はフルページを Render する。`HX-Request` ヘッダーの有無で分岐する（§56 の判定ロジックは §22-B と同じ）。

```go
// internal/handler/respond.go
package handler

import (
    "net/http"

    "github.com/a-h/templ"
    "github.com/gin-gonic/gin"
)

// HTMX リクエストかどうかを判定する
func isHTMX(c *gin.Context) bool {
    return c.GetHeader("HX-Request") != ""
}

// HTMX 時は fragment、通常時は page を Render する
func respondWithFragment(c *gin.Context, fragment, page templ.Component) {
    component := page
    if isHTMX(c) {
        component = fragment
    }
    if err := component.Render(c.Request.Context(), c.Writer); err != nil {
        c.AbortWithStatus(http.StatusInternalServerError)
    }
}
```

```go
// 使用例: デバイス計測一覧 Handler
func (h *DeviceHandler) Readings(c *gin.Context) {
    readings, err := h.repo.ListReadings(c.Request.Context(), deviceID)
    if err != nil {
        c.AbortWithStatus(http.StatusInternalServerError)
        return
    }
    respondWithFragment(c,
        view.DeviceReadingsList(readings), // HTMX 時: 部分更新コンポーネント
        view.ReadingsPage(device, readings), // 通常時: フルページ
    )
}
```

### 56.2 `respondWithFragments()` — 複数フラグメント同時返却

1リクエストで複数の部分更新領域を同時に差し替える。Laravel の `fragmentsIf()` のような構文は無いため、templ では「複数のサブコンポーネントを1つのラッパーコンポーネントで束ねて Render する」方式にする。OOB スワップ（§5）を使わずに、`hx-target` を2領域を包含する親要素に向ける場合に用いる。

```go
// 複数コンポーネントを順に Render する（HTMX 時のみ束ねて返す）
func respondWithFragments(c *gin.Context, page templ.Component, fragments ...templ.Component) {
    if !isHTMX(c) {
        if err := page.Render(c.Request.Context(), c.Writer); err != nil {
            c.AbortWithStatus(http.StatusInternalServerError)
        }
        return
    }
    for _, f := range fragments {
        if err := f.Render(c.Request.Context(), c.Writer); err != nil {
            c.AbortWithStatus(http.StatusInternalServerError)
            return
        }
    }
}
```

**使用場面:** デバイス切替時に、計測サマリーとアラート状態の2領域を同時に差し替えるケース。

```go
respondWithFragments(c,
    view.DeviceShowPage(device, summary, alertStatus), // 通常時
    view.ReadingSummary(summary), // HTMX 時 領域1
    view.AlertStatus(alertStatus), // HTMX 時 領域2
)
```

**templ 側の配置:** 各領域は独立した templ コンポーネント関数（`ReadingSummary` / `AlertStatus`）として定義する。`hx-target` は2領域を包含する親要素を指定する。

### 56.3 `validationErrorFragment()` — 422 + フラグメント返却

バリデーション失敗時に HTTP 422 とフォームコンポーネント（エラー表示込み）を返すヘルパー。Laravel の `View::share('errors', ...)` のような暗黙の共有バッグは Go に存在しないため、エラーは `map[string]string`（項目→メッセージ）として **templ コンポーネントへ明示的に引数で渡す**。

```go
// バリデーションエラーを 422 + フォームコンポーネントで返す
func validationErrorFragment(c *gin.Context, form templ.Component) {
    c.Status(http.StatusUnprocessableEntity)
    if err := form.Render(c.Request.Context(), c.Writer); err != nil {
        c.AbortWithStatus(http.StatusInternalServerError)
    }
}
```

```go
// 使用例: デバイス登録 Handler
func (h *DeviceHandler) Store(c *gin.Context) {
    var in CreateDeviceInput
    if err := c.ShouldBind(&in); err != nil {
        // ShouldBind のエラーを map[string]string に変換し、フォームへ明示的に渡す
        fieldErrors := toFieldErrors(err)
        validationErrorFragment(c, view.DeviceForm(in, fieldErrors))
        return
    }
    // 正常時の処理...
}
```

**ポイント:** Go では暗黙の `$errors` 共有バッグや `view()->share('errors')` は無い。エラーは常に `fieldErrors`（`map[string]string`）として明示的にコンポーネントへ渡し、`DeviceForm` 内で `if msg, ok := fieldErrors["name"]; ok { ... }` のように参照して再描画する。

> 📅 新規画面では `validationErrorFragment()` を使い、フォームコンポーネントへ `fieldErrors` を明示的に渡す方針で統一する。

### 56.4 実装で確定したヘルパー対と 422 フラグメントの流用 📅 2026-06-08 — alert-rules で確立

> §56.1〜56.3 は理念形（`respondWithFragment` 系）だが、実装は **2 関数のペア**に収束した（`internal/handler/auth.go`）。新規 Handler はこの実体に合わせる。

```go
// renderPage: フルページ専用ではなく「Content-Type html + c.Status(status) + Render」の汎用。
// status を引数に取るのでフラグメントの 422 返却にも流用できる。
func renderPage(c *gin.Context, status int, comp templ.Component) {
    c.Header("Content-Type", "text/html; charset=utf-8")
    c.Status(status)
    _ = comp.Render(c.Request.Context(), c.Writer)
}

// renderComponent: HTMX フラグメントを 200 固定で返す。status を取らないため 422 には使えない。
func renderComponent(c *gin.Context, comp templ.Component) {
    c.Header("Content-Type", "text/html; charset=utf-8")
    c.Status(http.StatusOK) // ← 200 ハードコード
    _ = comp.Render(c.Request.Context(), c.Writer)
}
```

**使い分け:** 成功時の部分返却（一覧再描画・行差し替え）は `renderComponent(c, comp)`（200 固定で十分）。**バリデーション失敗の部分返却は `renderPage(c, http.StatusUnprocessableEntity, comp)`** — `renderComponent` は 200 固定で 422 を立てられないため、status を取る `renderPage` をフラグメントにも流用する。

```go
renderComponent(c, component.AlertRuleList(toRowViews(rules)))                       // 成功=200
renderPage(c, http.StatusUnprocessableEntity, component.AlertRuleSection(section))   // 422 再描画
```

> **罠:** 「フラグメントだから `renderComponent`」と機械的に選ぶと 422 を立てられず 200 で返り、`htmx.config.responseHandling`（§7、422 を swap 対象に含める設定）の前提が崩れてエラー表示がスワップされない。**フラグメント 422 は `renderPage(c, 422, comp)`** と覚える。

## 58. `HX-Trigger` データペイロード + クライアント側 `event.detail` 受信パターン

§24 では `HX-Trigger` で空イベント（`'modal-close': ''`）を返すパターンを記載した。本節では **データペイロードを含むイベント** を返し、クライアント側でペイロードを受信して処理するパターンを解説する。

### ユースケース

サーバーで文字列を生成し、クライアントでクリップボードにコピーする（例: デバイス情報のコピー）。
`hx-swap="none"` で DOM は更新せず、`HX-Trigger` ヘッダー経由でデータだけ返す。

### Handler 側

templ は描画せず、`c.Header` で `HX-Trigger` にペイロードを載せて空ボディの 200 を返す。

```go
// hx-swap="none" のエンドポイント — DOM 更新なし、HX-Trigger でデータ返却
func (h *DeviceHandler) CopyInfo(c *gin.Context) {
    payload, err := json.Marshal(map[string]any{
        "device:copy-text": map[string]string{"text": generatedText},
    })
    if err != nil {
        c.String(http.StatusInternalServerError, "")
        return
    }
    c.Header("HX-Trigger", string(payload))
    c.String(http.StatusOK, "") // ボディは空・ステータスは 200
}
```

**ポイント:**
- ボディは空・ステータスは **200** を使う。204（No Content）だと HTMX がイベントを発火しない場合がある
- JSON のキーがイベント名、値がペイロード（任意のオブジェクト）
- 日本語をそのまま載せたい場合、`encoding/json` はデフォルトで非 ASCII を `\uXXXX` にエスケープするが、ヘッダー値としては問題なくクライアントで復元される

### templ（ボタン）側

```templ
templ DeviceCopyButton() {
    <button type="button"
        hx-post="/devices/copy-info"
        hx-swap="none"
        hx-include="[name='device_ids[]']"
    >デバイス情報をコピー</button>
}
```

CSRF トークンは §8 の `htmx:configRequest` で `X-CSRF-Token` ヘッダーに自動付与される（採用ライブラリは要確認）。

### JS（イベントリスナー）側

フレームワーク非依存のため、そのまま流用できる。配置先は `App.templ` の `<script>` ブロック。

```javascript
document.body.addEventListener('device:copy-text', function(event) {
    var text = event.detail.text;
    if (text && navigator.clipboard) {
        navigator.clipboard.writeText(text);
    }
});
```

**ポイント:**
- HTMX は `HX-Trigger` JSON の値を `event.detail` にマッピングする
- `event.detail.text` でペイロードの `text` プロパティにアクセス
- `navigator.clipboard.writeText()` は HTTPS または localhost でのみ動作

### Alpine.js 変数 → hidden input 変換（動的パラメータ送信）

`hx-include` で `[name='device_ids[]']` を含めるが、Alpine.js の `selectedDevices` 配列を hidden input に変換する必要がある:

```html
@click="$nextTick(() => {
    document.querySelectorAll('input[name=\'device_ids[]\']').forEach(el => el.remove());
    selectedDevices.forEach(d => {
        const inp = document.createElement('input');
        inp.type='hidden'; inp.name='device_ids[]'; inp.value=d;
        document.querySelector('.copy-action-btns').appendChild(inp);
    });
})"
```

§19 の `hx-vals='js:{...}'` パターンも使えるが、配列の場合は hidden input 方式の方が確実（配列の制約は §63 を参照）。

### テスト

`HX-Trigger` ヘッダーの JSON 検証は、`httptest` でレスポンスヘッダーを取り出して `json.Unmarshal` で検証する:

```go
res := w.Result()
trigger := res.Header.Get("HX-Trigger")
require.NotEmpty(t, trigger)

var payload map[string]map[string]string
require.NoError(t, json.Unmarshal([]byte(trigger), &payload))
require.Contains(t, payload, "device:copy-text")
require.Contains(t, payload["device:copy-text"]["text"], "期待する文字列")
```

---

### 58.X `HX-Trigger` で多要素の状態を OOB なしに一斉同期するパターン

単一の HTMX レスポンスで、**右パネル領域の swap + 左一覧 N 行の状態更新** を同時に行いたい場合、OOB swap を使わずに `HX-Trigger` + Alpine.js 変数で処理できる。

#### ユースケース

デバイス一覧で「お気に入り / 監視対象」のトグルを行う例:
- クリック元: 左一覧の `→` ボタン
- 右パネル: 追加・削除後の監視対象パネル templ コンポーネントに outerHTML swap
- 同時に反映: 左一覧の全 `→/✓` アイコン + ハイライトが一斉更新される

#### Handler（再掲 = §10-F と共通）

```go
func (h *DeviceHandler) ToggleWatch(c *gin.Context) {
    // ... トグル処理後、現在の監視対象 ID 一覧を取得 ...
    ids := watchedDeviceIDs // []int64

    payload, _ := json.Marshal(map[string]any{
        "device:watch-changed": map[string]any{"ids": ids},
    })
    c.Header("HX-Trigger", string(payload))

    // 右パネル templ コンポーネントを直接描画
    _ = component.WatchPanel(devices).Render(c.Request.Context(), c.Writer)
}
```

#### templ / Alpine.js

`Js::from()` は使えないため、監視対象 ID を `<script type="application/json">` に出力し、Alpine 側で `JSON.parse` して読む（§10-E/§32 と同方針。生 JSON を属性に連結しない）。

```templ
templ DeviceListWithWatch(devices []Device, watchedIDs []int64) {
    @templ.JSONScript("watched-ids-data", watchedIDs)
    <div x-data="{
        watchedIds: JSON.parse(document.getElementById('watched-ids-data').textContent),
        isWatched(id) { return this.watchedIds.map(v => Number(v)).includes(Number(id)); }
    }" @device:watch-changed.window="watchedIds = ($event.detail.ids ?? []).map(v => Number(v))">

        for _, d := range devices {
            // 1イベント受信で N 行の :class / x-show が一斉に再評価される
            <tr x-bind:class={ "isWatched(" + strconv.FormatInt(d.ID, 10) + ") ? 'status-active' : ''" }>
                <button hx-post={ "/devices/" + strconv.FormatInt(d.ID, 10) + "/watch-toggle" }
                        hx-target="#watch-panel" hx-swap="outerHTML">
                    <span x-show={ "!isWatched(" + strconv.FormatInt(d.ID, 10) + ")" }>→</span>
                    <span x-show={ "isWatched(" + strconv.FormatInt(d.ID, 10) + ")" } x-cloak>✓</span>
                </button>
            </tr>
        }
    </div>
}
```

`@templ.JSONScript`（または自前の `<script type="application/json">` 出力）で JSON をエスケープ済みのテキストノードとして埋め込み、Alpine は `JSON.parse` で読み込む。これで XSS リスクなく初期配列を渡せる。

#### なぜ OOB swap より優れるか

| 要素 | OOB swap の場合 | HX-Trigger + Alpine.js の場合 |
|------|-----------------|-------------------------------|
| 更新対象 | DOM を ID 指定で個別に outerHTML 置換 | Alpine.js の `watchedIds` 変数 1 箇所を変更 |
| レスポンスサイズ | N 行分の HTML | イベント 1 個 + 右パネル HTML のみ |
| 一覧ソート・フィルタ後 | 差分 HTML の整合性に注意 | `isWatched(id)` が各行で都度判定するので自動整合 |
| HTMX 再処理 | 各 OOB 要素で再バインド必要 | Alpine リアクティブで自動 |

#### 注意点

- **イベント名は `':'` 含む namespace 付きを推奨**（例: `device:watch-changed`）。他画面との衝突回避。
- **`event.detail` は HTMX が JSON をそのまま構造マッピング**するので、ペイロード構造は Handler 側の JSON 構造と完全一致させる。
- **`Number()` 変換を忘れない**。HTMX が渡す値は文字列／数値の混在がある（`@device:watch-changed.window="watchedIds = ($event.detail.ids ?? []).map(v => Number(v))"`）。

---

## 59. templ `{ }` と HTMLエンティティの二重エスケープ問題

> **重要** — 姉妹文書「HTMLモック作成ルール.md」TL05 を参照。

### 問題

モック HTML の `&#165;` や `&yen;` を templ テンプレートにそのまま文字列リテラルとして持ち込むと、templ の `{ }` による自動 HTML エスケープにより二重エスケープが発生する。`&` 自体が `&amp;` に変換されるため、画面に「&yen;16,000」とリテラル表示されてしまう。

templ の `{ x }` は Blade の `{{ $x }}` と同様、出力を常に自動 HTML エスケープする。したがって、文字列内に HTML エンティティ参照を含めると意図せず二重エスケープになる。

```templ
// NG: "℃" の HTML エンティティを文字列に含めると &deg;C → &amp;deg;C にエスケープされる
templ ReadingValueNg(value string) {
    <span class="reading-value">{ value + "&deg;C" }</span>
}

// OK: ℃ を直接使用（UTF-8 文字はそのまま出力され、二重エスケープにならない）
templ ReadingValueOk(value string) {
    <span class="reading-value">{ value + "℃" }</span>
}
```

### 対応ルール

| 方法 | 例 | 安全性 | 推奨 |
|------|---|--------|------|
| UTF-8 文字を直接使用 | `value + "℃"` | 安全（`{ }` エスケープ有効） | **推奨** |
| `@templ.Raw(...)` で非エスケープ出力 | `@templ.Raw(value + "&deg;C")` | XSS リスクあり | 非推奨 |
| モックの HTML エンティティをそのまま文字列にコピー | `{ value + "&deg;C" }` | 二重エスケープ | **禁止** |

### 影響を受ける文字

モック HTML で頻出する HTML エンティティと、templ で使うべき UTF-8 文字:

- `&#176;` / `&deg;` → `°`（`℃` は直接 `℃` を使う）
- `&#8593;` / `&uarr;` → `↑`（計測値の上昇トレンド表示等）
- `&#8595;` / `&darr;` → `↓`（計測値の下降トレンド表示等）
- `&#37;` → `%`（湿度の単位）

templ の `{ }` に渡す Go 文字列リテラルでは、常に UTF-8 文字を直接使用すること。HTML エンティティは templ の外（静的 HTML 直書き部分で、かつ `{ }` を通さない箇所）でのみ使用可。

---

## 60. templ コンポーネント分割と `hx-target` 用 `id` 属性の必須ラッパー

> 姉妹文書「HTMLモック作成ルール.md」TL07 を参照。

### 問題

部分更新領域を別の templ コンポーネント関数に分割する場合（§56 の方針）、その**コンポーネント関数のルート要素自体に `id` を持たせる**と、HTMX が outerHTML swap した後に `id` 要素が消え、次回の `hx-target="#xxx"` が動作しなくなる。逆に、`id` を持つラッパーが一切ないと `hx-target="#xxx"` がそもそも要素を見つけられずリクエストが失敗する。

### 正しいパターン

`id` ラッパーを **swap 対象コンポーネントの外側** に置き、内側のコンポーネントだけを差し替える。

```templ
// フルページ側: id ラッパーで部分更新コンポーネントを包む
templ ReadingsPage(device Device, readings []Reading) {
    <div id="device-readings-list">
        @ReadingsList(readings)
    </div>
}

// 部分更新コンポーネント: id を持たない
templ ReadingsList(readings []Reading) {
    <div class="readings-row">
        for _, r := range readings {
            // ...
        }
    </div>
}
```

HTMX 時は Handler が `ReadingsList(...)` を `Render` し、`hx-target="#device-readings-list"` に対して `hx-swap="innerHTML"`（デフォルト）で内側を差し替える。これなら `id` ラッパーは常に残る。

```go
func (h *ReadingHandler) List(c *gin.Context) {
    readings := h.repo.ListReadings(c.Request.Context(), deviceID)
    if c.GetHeader("HX-Request") != "" {
        _ = component.ReadingsList(readings).Render(c.Request.Context(), c.Writer)
        return
    }
    _ = page.ReadingsPage(device, readings).Render(c.Request.Context(), c.Writer)
}
```

### 誤ったパターン

```templ
// NG: 部分更新コンポーネントのルート要素自身に id → outerHTML swap 後に id が消える
templ ReadingsListNg(readings []Reading) {
    <div id="device-readings-list" class="readings-row">
        // ...
    </div>
}

// NG: id ラッパーが存在しない → hx-target="#device-readings-list" が動作しない
templ ReadingsPageNg(readings []Reading) {
    @ReadingsListNg(readings)
}
```

### チェック方法

新しく部分更新コンポーネントを追加したら、ブラウザの開発者ツールで `document.getElementById('device-readings-list')` が `null` でないことを、**初期表示時と swap 1 回後の両方**で確認する。

### 補足: 動的追加される複数パネルの id unique 化

同じ構造のカード／パネルを動的に追加できる場合（例: デバイスごとのアラートルールパネルを複数並べる）、コンポーネント単位で `id` を unique 化する。静的 `id` をそのまま繰り返し出力すると `querySelector` が最初の 1 件しかヒットしない。

```templ
// 部分コンポーネント: AlertRulePanel.templ
templ AlertRulePanel(device Device) {
    // device.ID を suffix して id を unique 化
    <div class="card" id={ fmt.Sprintf("alert-rule-panel-%d", device.ID) }>

        // 中のインライン更新領域の id もデバイスごとに unique に
        <div id={ fmt.Sprintf("alert-rule-list-%d", device.ID) } class="rule-list-actions">
            @AlertRuleList(device.Rules)
        </div>

        // hx-target にも unique id を指定
        <button hx-post="/alerts/rules"
                hx-target={ fmt.Sprintf("#alert-rule-list-%d", device.ID) }
                hx-vals={ fmt.Sprintf(`{"device_id": "%d"}`, device.ID) }>ルールを追加</button>

        // パネル自体の削除: outerHTML swap の target は wrapper の unique id
        <button hx-delete={ fmt.Sprintf("/alerts/rules/panel/%d", device.ID) }
                hx-target={ fmt.Sprintf("#alert-rule-panel-%d", device.ID) }
                hx-swap="outerHTML">削除</button>
    </div>
}
```

**ポイント:**
- コンポーネント関数名（`AlertRulePanel`）は共通でよい（1 コンポーネントを複数回レンダリングする設計）
- DOM 上の `id` はデバイス ID 等で必ず unique 化する
- 子のインライン領域（ルール一覧等）の `id` もパネル id を suffix して衝突防止
- ただし §60 本則のとおり、**部分 swap の対象になる子コンポーネント自身のルート `id`** ではなく、その外側のラッパー `id` を target にすること

---

## 61. 検索フォームのクリアにおける `<input type="date">` リセット

> 汎用挙動のメモ。

### 問題

`form.reset()` は HTML 仕様上、`<input>` の `value` **属性**（サーバーサイドで設定された初期値）に戻す。templ 側で `value={ c.Query("date_from") }` 相当の初期値を設定している場合、`reset()` はクエリパラメータの値に戻すだけで空にならない。

### 修正内容（filter-form の JS）

`form.reset()` の直後に、テキスト系・日付系の入力フィールドを明示的に空にする。フレームワーク非依存なのでそのまま流用できる。

```javascript
form.reset();
// reset() はサーバー設定の value 属性に戻すだけなので、明示的にクリア
form.querySelectorAll('input[type=date], input[type=text], input[type=search]').forEach(el => {
    el.value = '';
});
// Tom Select は別途リセット
form.querySelectorAll('select.js-tom-select').forEach(el => {
    if (el.tomselect) el.tomselect.setValue('');
});
```

### 影響範囲

`.filter-form` を使う全画面（計測履歴 `/devices/{device}/readings`、アラート履歴 `/alerts/history` 等、期間フィルタの日付フィールドを持つ画面）で有効。

---

## 62. `x-data` スコープ外のモーダル配置による Alpine.js エラー

> 姉妹文書「HTMLモック作成ルール.md」TL06 を参照。

### 問題

templ 化時にモーダルを `x-data` スコープの `</div>` 閉じタグの **外側** に配置すると、モーダル内の `x-show="deleteModalOpen"` が Alpine.js 変数を参照できずコンソールエラー（`deleteModalOpen is not defined`）になる。

モック HTML では `<body x-data="{...}">` で全体がスコープ内だが、templ 化で各ページコンポーネント内の `<div x-data="{...}">` に変わるため、モーダルの配置位置によってはスコープ外に出てしまう。

### 正しいパターン

```templ
templ DeviceShowPage(device Device) {
    <div x-data="{ deleteModalOpen: false }">

        // メインコンテンツ
        // ...

        // モーダルは x-data の閉じタグの直前（スコープ内）に配置
        <div class="modal-overlay" x-show="deleteModalOpen" x-cloak>
            <div class="modal-content">
                // デバイス削除確認モーダル
            </div>
        </div>

    </div>  // x-data スコープ終了
}
```

### 誤ったパターン

```templ
templ DeviceShowPageNg(device Device) {
    <div x-data="{ deleteModalOpen: false }">
        // ...
    </div>  // x-data スコープ終了

    // NG: スコープ外 → deleteModalOpen is not defined
    <div class="modal-overlay" x-show="deleteModalOpen" x-cloak>
        <div class="modal-content">...</div>
    </div>
}
```

### チェック方法

ブラウザの DevTools Console で `Alpine Expression Error: ... is not defined` が出ていないか確認する。モーダルが非表示（`x-show="false"`）の場合でも、ページ読み込み時に Alpine.js が式を評価するためエラーが発生する。

### HTMLモック作成ルールとの関係

モック HTML の R14「モーダルは `</body>` 直前に配置」は、templ 化時に「`x-data` スコープ内の末尾に配置」と読み替える。姉妹文書「HTMLモック作成ルール.md」TL06 を参照。

---

## 63. `hx-vals` の配列送信制約と `fetch()` + JSON 代替パターン

### 問題

Alpine.js の配列変数を `hx-vals` でバインドして HTMX POST すると、配列がフラットなフォームデータに変換される:

```
送信したい: { "device_ids": [1, 2] }
実際の送信: device_ids=1&device_ids=2  （受信側のバインドによっては最後の値のみ）
         or device_ids=1               （単一値）
```

Gin の `c.ShouldBind` で `DeviceIDs []int64 \`form:"device_ids" binding:"required"\`` のように受ける場合、送信形式によっては正しく配列にバインドされず、バリデーションエラー（400）で失敗する。

### 解決策: `fetch()` + `Content-Type: application/json`

JSON ボディで送れば配列・ネスト構造が壊れない。受信側は `c.ShouldBindJSON` で受ける。

```html
<button type="button"
    @click="
      fetch('/devices/bulk-action', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': document.querySelector('meta[name=csrf-token]').content,
          'HX-Request': 'true',
        },
        body: JSON.stringify({ device_ids: selectedDevices }),
      }).then(r => {
        const trigger = r.headers.get('HX-Trigger');
        if (trigger) {
          const data = JSON.parse(trigger);
          // HX-Trigger のデータペイロードを処理（§58）
        }
      });
    "
>一括実行</button>
```

受信側の Handler:

```go
func (h *DeviceHandler) BulkAction(c *gin.Context) {
    var req struct {
        DeviceIDs []int64 `json:"device_ids" binding:"required,min=1"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "デバイスを選択してください"})
        return
    }
    // ... req.DeviceIDs を処理 ...
}
```

**ポイント:**
- `Content-Type: application/json` で配列が正しく JSON 送信され、`c.ShouldBindJSON` で配列にバインドされる
- `X-CSRF-Token` ヘッダーは手動設定が必要（`htmx:configRequest` は HTMX 経由のリクエストにしか効かず、生の `fetch` には効かない）
- `HX-Request: true` を付けると Handler の HTMX/非 HTMX 分岐（`c.GetHeader("HX-Request")`）が正しく動作する
- レスポンスの `HX-Trigger` ヘッダーは `r.headers.get()` で取得（HTMX のイベント自動発火は効かないため、JS 側で手動処理）

### `hx-vals` が使えるケース vs 使えないケース

| データ型 | `hx-vals` | `fetch` + JSON |
|---------|-----------|----------------|
| スカラー値（`{ id: 123 }`） | ✅ 問題なし | 不要 |
| 配列（`{ device_ids: [...] }`） | ❌ 配列が壊れる | **必須** |
| ネスト構造 | ❌ フラット化される | **必須** |

### §58 との関係

§58 の `HX-Trigger` データペイロードパターンでは `hx-post` + `hx-swap="none"` を使用していたが、配列パラメータを送る場合は本セクションの `fetch()` パターンに置き換える。

### CSRF と `hx-include` の注意

§8 の `htmx:configRequest` で CSRF トークンは HTMX リクエストのヘッダー（`X-CSRF-Token`）に自動付与される。そのため **HTMX 経由のボタン** では meta タグのトークンから自動付与され、hidden input を `hx-include` する必要はない。`fetch()` を使う場合は上記のとおりヘッダーに手動設定する（CSRF ミドルウェアの採用ライブラリは要確認）。

---

---

## 付録: 本プロジェクトで不採用とした別プロジェクト固有セクション

本ガイドは、元となった Laravel + Blade 前提の別プロジェクトの実装ガイド（§1〜§67）を、本プロジェクト（Go + Gin + templ + HTMX + 農業IoT ドメイン）向けに再編したものである。以下のセクションは**本プロジェクトに該当しないため不採用**とした（元番号は欠番）。将来該当する機能を追加する場合の参考として、何を落としたかを記録する。

### 別ドメイン固有（営業案件・台帳・発注者・多段ステップ等）
- **§5.1 / §5.2**（2カラムレイアウト + 固定パネル）— 本プロジェクトの編集画面は1カラムのみ
- **§28**（combo-select コンポーネントの options 形式）— 別ドメインの Blade コンポーネント依存
- **§33**（expand-row の虫眼鏡ボタンと @click.stop）— 本プロジェクトに展開行 UI なし
- **§34**（右カラムパネル work-info-panel 構造）
- **§36**（一覧テーブルの ledger-list-container ラッパー）
- **§37**（発注者情報・住所ブロックの常時表示）
- **§38**（ステップタブの配色ルール）— 多段ステップフォームは本プロジェクトになし
- **§44**（modal-open vs modal-open-fullscreen）— 本プロジェクトのモーダル要件に対し過剰
- **§66**（複合集計の twin-query パターン）— 別ドメインの報告集計 + Eloquent 依存

### Eloquent / クエリビルダ固有（本プロジェクトは sqlc を使用）
- **§21（重複番号）**（addSelect サブクエリエイリアスでのソート）
- **§47**（Enum キャストカラムの whereIn() に Enum インスタンスを渡す）
- **§48**（削除済みモデルの Enum を getRawOriginal() で参照）
- **§57**（firstOrCreate 使用時のユニーク制約考慮）
- **§65**（一覧の totalCount を DB::query()->fromSub()->count() で取る）

### 本プロジェクトに存在しない機能
- **§17 / §18**（CSV取込 → JSON応答 → Alpine.js配列更新 / Alpine.js配列 → hidden input JSON送信）— CSV取込機能なし
- **§22（CSV取込レスポンスハンドラの未完成）** — §17 削除に伴い不要（※モーダル hx-target 使い分けの旧 §22 は「22-B」として保持）
- **§35**（ファイル削除機能のピュア JS 実装）
- **§45**（列カスタマイズ localStorage + htmx:afterSwap 復元）
- **§46**（インライン編集の楽観ロック sync_token）— 本プロジェクトの CRUD は単純で不要
- **§67**（Phase 分離画面のボタン placeholder → 実接続）

### Blade 固有の落とし穴（templ では発生しない）
- **§64**（HTMLコメント内の `<x-...>` が Blade コンポーネントとしてパースされる）— templ にこの「パースされる」問題は無い。**ただし templ はマークアップ内の HTML コメント `<!-- -->` を生成 HTML にそのまま出力する**ため、コメントに書いた語（例: `hx-target`）が否定アサート（その語を含まないことの検証）と衝突したり本番 HTML に漏れたりする。説明は templ 関数上の Go ドキュメントコメント（`// ...`）に書く（テストガイダンス集 §58.1）

> ※ 重複していた元番号（§21・§22・§40 が前後半で重複）は、保持したものを **22-B**（モーダル内 hx-target 使い分け）・**40-B**（CSS キャッシュバスティング）にリネームして番号衝突を回避した。

---

## 64. alert-history で確立した実装知見（番号付きページャ・OOB 不採用・フィルタ型テナント分離・共有ヘルパ非改変・view 写像）📅 2026-06-08 — alert-history で確立

> 「フィルタ＋一覧＋ページネーション＋HTMX 部分更新」は readings と同型課題。alert-history は readings を写経しつつ、(1) 番号付きページャ・(2) device_id を任意フィルタとして扱うテナント分離・(3) GET フィルタの from≤to 検証 で固有の判断が要った。本節はその確定事項を再利用可能な教訓として残す。

### 64.1 ページャは OOB ではなく結果領域 fragment に内包して単一 innerHTML swap（§3.7 の OOB 記述は不採用）

§3.7 の旧表は `alert-history-pagination` を `hx-swap-oob` で別途更新する想定だったが、**実装は OOB を使わない**。結果領域 fragment `#alert-history-list` のルート `div` の内側に「一覧テーブル → 空状態 → `nav.pagination`」を内包し、検索・ページ送りで**この `div` 全体を 1 回の innerHTML swap** で差し替える（readings の `DeviceReadingsList` と同型）。

**本質的教訓**: OOB は「物理的に離れた 2 要素の同時更新」専用（§5）。一覧とページャのように**隣接して同時更新する要素は同じ fragment に内包すれば OOB は不要**。OOB を避けると Handler は fragment を 1 つ Render するだけで済み、テストも「`#alert-history-list` 1 つ」を検証すればよい。`thead` も毎回再描画されるが（§3.7 旧記述の「`tbody` のみ」とは差異）コストは無視可能で、写経の一貫性を優先する。

### 64.2 番号付きページネーション部品（既存 `Pagination` の「前/次のみ」を汎用化せず新規部品にする）

既存 `Pagination`（§10）は「前へ/次へ＋ N / M 表示」かつ `hx-target` 固定で番号リンク非対応。モック（番号 1,2,3）を満たすには番号リンク配列を持つ**別部品**を新規作成する（既存を汎用化すると兄弟 readings へ回帰リスク）。

- **view モデル**: ページ番号 1 個を `PageLink{ Num int; URL string; Current bool }` とし、`PaginationView{ HasPrev, HasNext bool; PrevURL, NextURL string; Pages []PageLink }` を持つ。番号は現状 `1..Last` を全列挙（IoT 小規模・モック準拠。多数ページ時の窓化は View 契約を変えず後日対応可）。
- **templ**: `nav.pagination` に `hx-boost="true"` + `hx-target="#alert-history-list"` + `hx-swap="innerHTML"`。前へ＝`HasPrev` で `<a href={SafeURL}>← 前へ</a>` / 無効時 `<span class="disabled">← 前へ</span>`。番号＝`for p := range Pages { if p.Current { <span class="current">{Num}</span> } else { <a href={SafeURL}>{Num}</a> } }`。次へも同様。URL は Handler 生成の信頼 URL を `templ.SafeURL` で埋める。`.current` / `.disabled` はモック既存クラスで新設しない（§31）。

```templ
templ AlertHistoryPagination(v AlertHistoryPaginationView) {
    <nav class="pagination" hx-boost="true" hx-target="#alert-history-list" hx-swap="innerHTML">
        if v.HasPrev {
            <a href={ templ.SafeURL(v.PrevURL) }>← 前へ</a>
        } else {
            <span class="disabled">← 前へ</span>
        }
        for _, p := range v.Pages {
            if p.Current {
                <span class="current">{ fmt.Sprintf("%d", p.Num) }</span>
            } else {
                <a href={ templ.SafeURL(p.URL) }>{ fmt.Sprintf("%d", p.Num) }</a>
            }
        }
        if v.HasNext {
            <a href={ templ.SafeURL(v.NextURL) }>次へ →</a>
        } else {
            <span class="disabled">次へ →</span>
        }
    </nav>
}
```

- **URL 生成**: `device_id`/`from`/`to` を保持し `page` のみ差し替える純関数（空のクエリは省略・`page` は常に付与）。Handler 側で `url.Values` を `Encode()`。
- **ページャ表示条件**: `HasPagination = totalPages > 1`。**1 ページに収まる結果・0 件はページャ非表示**（R3.1「1ページに収まらないとき」/ R6.2）。readings の「常に N / M 表示」とは異なる点に注意。

### 64.3 device_id を「任意フィルタ」にする画面のテナント分離はクエリの user_id スコープに委ねる（`RequireDeviceOwner` 非経由）

readings はパス固定（`/devices/:device/readings`）で `authz.RequireDeviceOwner` により単一デバイスの所有者を強制する。alert-history は device_id を**任意フィルタ（nullable・「全デバイス」可）**として扱うため設計が異なる:

- `device_id` 空 → `nil`（全デバイス）/ 数値 → `*int64` / 非数値 → 400。
- テナント分離は `ListAlertHistoriesPaginated` / `CountAlertHistoriesInRange` の `WHERE d.user_id = $1` に**集約**し、Handler は必ず `auth.UserID(c)` を `UserID` へ渡す。`($2 IS NULL OR d.id = $2)` により全デバイス／指定デバイスを 1 クエリで両立。
- **非所有・不在の device_id を数値で渡されてもクエリスコープが空を返す**ため、「不在」と「非所有」を区別せず**列挙防止**になる（R8.3）。`authz` を経由しない。

**本質的教訓**: 「パス上のリソース＝必須・単一」なら `RequireDeviceOwner` で 404、「クエリ上のフィルタ＝任意・複数可」ならクエリの user_id スコープで空表示。**フィルタ型を 404 にしない**（存在を漏らす）。所有者境界は WHERE 句 1 箇所に集約し Handler に散らさない（structure.md ルール5）。

### 64.4 兄弟画面と共有するヘルパ（`parseDateBounds` 等）は改変せず、画面ローカルに追加検証を載せる

期間検証で、readings と共有する `parseDateBounds`（形式検証＋センチネル境界・to は end-of-day）は**確定済み**。alert-history が要する from≤to の**範囲検証**を共有関数に足すと **readings が回帰**する。そこで範囲検証は alert-history Handler 内の**ローカル純関数 `dateRangeError(from, to, fromTS, toTS)`** として追加する:

- 両指定かつ `fromTS.After(toTS)` のみエラー（`errs["to"]` に積む）。片方のみ・未指定はスキップ。`to` が end-of-day のため `from==to` は `After=false` で自然に許容。
- GET フィルタの検証失敗は **200＋インラインエラー（クエリ skip）**。ミューテーションフォームの 422（§56.4）は GET フィルタには使わない。

**本質的教訓**: **複数画面で共有する確定済みヘルパは「最小・不変」に保ち、画面固有の追加条件はその画面の Handler 内ローカル関数で重ねる**。共有関数を太らせると他画面のテスト・挙動へ波及する。

### 64.5 フィルタ select の選択肢は repository 型ではなく view 型（`component.DeviceOption`）へ Handler が写像する

デバイス選択肢は `repository.Device`（`pgtype` を含む）を view へ直接渡さず、Handler が **`component.DeviceOption{ ID, Name, Selected }`** へ写像する（兄弟 alert-rules と同型）。design.md のドラフトが `[]repository.Device` と記していても、**steering の view 純粋性（view 層に pgtype/repository 型を持ち込まない）を優先**する。

**本質的教訓**: view モデルは整形済み primitive のみ。`selected` 判定（`strconv.FormatInt(d.ID)==deviceIDStr`）も Handler で計算して `DeviceOption.Selected` に詰め、templ は `selected?={ d.Selected }` を描くだけにする。「全デバイス」option の selected は別途 `DeviceID==""` で判定（先頭 `<option value="">`）。

---

## 要確認事項（実装着手前に決定すべき項目）

本ガイドは設計書（システム構成図 / DB設計書 / 画面設計書 / HTMLモック作成ルール）に基づき翻訳したが、以下は実装方針が未確定のため断定を避けた。実装着手前に決定すること。

> **【2026-06-22 追記】下記5項目は S1〜S8 実装で全て確定・実装済み。** 実態は [実装現状サマリ.md](実装現状サマリ.md)（§2・§3）が正：① CSRF=**gorilla/csrf**（`SESSION_SECRET` から鍵導出・Web グループ限定）、② **自作 MethodOverride**（http.Handler 層で `_method` を昇格）、③ scs=**`internal/auth/session_auth.go`（scs/v2 + pgxstore）実装済み**、④ アセット=**go:embed（`internal/view/static.go` → `/static`）+ `make sync-css`**、⑤ 型名はコード（`internal/repository`・`internal/view/{component,page}`）が正。以下は翻訳時点の検討メモとして残す。

1. **CSRF 対応**: Gin の CSRF ミドルウェアの採用ライブラリ（gorilla/csrf 等）と、ヘッダー名（本書は仮に `X-CSRF-Token` で統一）。`htmx:configRequest` でのヘッダー送信パターンはライブラリ非依存で流用可。
2. **PUT / DELETE / PATCH のフォーム送信**: HTML フォームは PUT/PATCH/DELETE を直送できない。`_method` 隠しフィールド + MethodOverride ミドルウェア（システム構成図.md のルートマッピング参照）か、POST 受けで Handler 分岐かを確定する。HTMX 操作（`hx-put` 等）は直接送信可能。
3. **セッション認証**: scs（`alexedwards/scs`）+ PostgreSQL ストアは将来 `internal/auth/session_auth.go` で実装予定。`Auth::user()` 相当の取得経路は実装時に確定。
4. **アセット配信**: 本番は `go:embed` + バージョンクエリでのキャッシュバスティング、開発は FileServer 等を想定（具体方式は実装時に確定）。Tom Select は CDN（モック）→ `public/` 配下にローカル配信（本番）。
5. **コード例の型名・パッケージ名**: 本書のコード例の型名（`repository.*` / `component.*` / `page.*` / `Pagination` 構造体等）・ヘルパー名（`toFieldErrors` 等）は説明用の仮称を含む。sqlc 生成名・実パッケージ構成に合わせて実装時に統一すること。

---

更新日時: 2026-06-01（Laravel+Blade 版から Go + Gin + templ 版へ全面再編）

