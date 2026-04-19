
---

# HTMLモック作成ルール

## cc-sdd参照ガイド

本設計書をcc-sdd（詳細設計書）から参照する際に価値の高いセクションと用途を示す。

| 優先度 | セクション | cc-sddでの用途 |
|:------:|-----------|---------------|
| ★★★ | [R01: id属性を使用しない](#r01-id属性を使用しない-error) | HTMLモックにidが存在しない理由の根拠。HTMX実装ガイド(動的).mdのid属性一覧が「唯一の定義元」である理由の説明。Blade変換時にバックエンド側がidを付与する責務 |
| ★★ | [R03: ラッパー要素で囲む](#r03-テーブル一覧などの動的コンテンツはラッパー要素で囲む-error) | ラッパー要素がHTMXのFragment差し替えターゲット（hx-target）になる前提の根拠。モックに既存のラッパーを利用して良い |
| ★★ | [R04: theadとtbodyを分離する](#r04-テーブルは-thead-と-tbody-を分離する-error) | theadを固定したままtbodyのみをBladeループ・Fragment差し替えするパターンの実現前提の根拠 |
| ★★ | [R09: エラー表示エリアを配置する](#r09-フォーム各項目の直後にエラー表示エリアを配置する-warning) | バリデーションエラー用の空spanがHTMLモックに既存であることの確認根拠。Blade側でエラーテキストを出力するだけでよく、要素追加は不要 |
| ★★ | [R10: DOM直接操作JavaScript禁止・Alpine.js可](#r10-dom直接操作のjavascriptを使用しない-error) | Alpine.jsの使用可否と適用範囲の根拠。UIの開閉・タブ切替等はAlpine.jsで対応し、JavaScriptフレームワーク不使用方針を確認 |
| ★ | [R12: 0件時メッセージを用意する](#r12-データ0件時の空の状態メッセージを用意する-warning) | 空状態メッセージ要素がHTMLモックに非表示で既存であることの確認根拠。Bladeで表示切替のみ実装すれば良い |

> ※ cc-sddのBlade実装・Fragment設計を記述する際は、まずR01でidがモックに存在しないこと、R03でラッパー要素が存在することを確認すること。

### 次回プロジェクトでの記載チェックリスト

HTMLモック作成ルールを新規作成する際に以下が揃っているか確認する：

- [ ] HTMLモックにid属性を付与しない旨を明記（id付与はBladeテンプレート変換時にバックエンド担当が行う）
- [ ] 動的コンテンツ（テーブル・カード一覧）のラッパー要素配置ルールを記載（HTMXのFragment差し替えターゲットになる）
- [ ] theadとtbodyの分離ルールを記載（tbody差し替えパターンの実現前提）
- [ ] フォームの各入力項目直後にエラー表示エリア（空のspan等）を用意する旨を記載
- [ ] DOM直接操作JavaScriptの使用禁止とAlpine.jsの可否・使用範囲を明記
- [ ] データ0件時の空状態メッセージ要素を初期非表示で配置する旨を記載

---

### R01: id属性を使用しない [ERROR]

id属性はBlade変換時にバックエンド側が付与する。モック段階でidが存在すると、バックエンド側のid付与と衝突する。

**チェック条件:**
- HTML要素に `id="..."` 属性が存在しないこと

**違反例:**

```html
<div id="sensor-report-table">
  <table>...</table>
</div>
```

**正しい例:**

```html
<div class="sensor-report-table">
  <table>...</table>
</div>
```

---

### R02: CSSでIDセレクタ（#）を使用しない [ERROR]

R01の通りモック段階ではid属性を使用しないため、CSSでもIDセレクタは使用できない。

**チェック条件:**
- `<style>` タグ内、および外部CSSファイル内に `#` で始まるセレクタが存在しないこと

**違反例:**

```html
<style>
  #sensor-table { border: 1px solid #ccc; }
</style>
```

**正しい例:**

```html
<style>
  .sensor-table { border: 1px solid #ccc; }
</style>
```

> ※ CSSの色コード（例: `#ccc`, `#ff0000`）はIDセレクタではないので違反ではない。

---

### R03: テーブル・一覧などの動的コンテンツはラッパー要素で囲む [ERROR]

バックエンド側ではラッパー要素を差し替えターゲットとして使用する。テーブルやカード一覧を直接配置すると、差し替え用の「器」がないため構造の組み直しが必要になる。

**チェック条件:**
- `<table>` 要素が `<div>` 等のブロック要素の子要素として配置されていること
- カード一覧（繰り返し要素の集合）が `<div>` 等のブロック要素の子要素として配置されていること
- ラッパー要素はデータが0件でも存在すること（中身が空でもラッパーは残す）

**違反例:**

```html
<h2>センサー一覧</h2>
<table>...</table>
```

**正しい例:**

```html
<h2>センサー一覧</h2>
<div class="sensor-list">
  <table>...</table>
</div>
```

---

### R04: テーブルは thead と tbody を分離する [ERROR]

バックエンド側ではtheadを固定したままtbody部分のみを差し替えるパターンを多用する。thead/tbodyが分かれていないと構造の組み直しが必要になる。

**チェック条件:**
- すべての `<table>` 要素が `<thead>` と `<tbody>` の両方を子要素として持つこと
- ヘッダー行（`<th>` を含む行）は `<thead>` 内に配置されていること
- データ行（`<td>` を含む行）は `<tbody>` 内に配置されていること

**違反例:**

```html
<table>
  <tr><th>センサー名</th><th>値</th></tr>
  <tr><td>温度センサーA</td><td>25.3℃</td></tr>
</table>
```

**正しい例:**

```html
<table>
  <thead>
    <tr><th>センサー名</th><th>値</th></tr>
  </thead>
  <tbody>
    <tr><td>温度センサーA</td><td>25.3℃</td></tr>
    <tr><td>湿度センサーB</td><td>60%</td></tr>
  </tbody>
</table>
```

---

### R05: フォーム要素は `<form>` タグで囲む [ERROR]

バックエンド側ではフォーム送信時にセキュリティトークン（CSRF）を埋め込む必要がある。`<form>` タグがないと構造の組み直しが必要になる。

**チェック条件:**
- `<input>`（type="hidden" を除く）、`<select>`、`<textarea>` 要素が `<form>` 要素の子孫であること
- `<form>` 要素に `action` 属性が存在すること（値は `#` や仮のURLで可）

**違反例:**

```html
<input type="text" name="keyword">
<button type="submit">検索</button>
```

**正しい例:**

```html
<form action="#">
  <input type="text" name="keyword">
  <button type="submit">検索</button>
</form>
```

---

### R06: フォーム送信を行うボタンは `<button type="submit">` を使用する [ERROR]

フォームの送信動作を伴うボタンは、必ず `<button type="submit">` を使用してください。
`<a>` タグを onclick 等で疑似的に送信ボタンとして使用することは禁止します。

> ※ 画面遷移のみを行うリンク（「詳細を見る」「キャンセル」など）は `<a>` タグで問題ありません。送信動作を伴うかどうかが判断基準です。

**チェック条件:**
- `<form>` 内で送信を行うボタンが `<button type="submit">` であること
- `<a>` タグに `onclick` 等を付けてフォーム送信を行っていないこと

**違反例（フォーム送信をaタグで代用している）:**

```html
<form action="#">
  <input type="text" name="keyword">
  <a href="#" class="btn" onclick="submit()">検索</a>
</form>
```

**正しい例:**

```html
<form action="#">
  <input type="text" name="keyword">
  <button type="submit" class="btn">検索</button>
</form>
```

---

### R07: フォーム要素に name 属性を付ける（スネークケース） [ERROR]

バックエンド側はname属性でフォームの値を受け取る。name属性がないとバックエンド側で全要素に属性を追加する作業が発生する。

**チェック条件:**
- `<form>` 内のすべての `<input>`（type="submit" を除く）、`<select>`、`<textarea>` に `name` 属性が存在すること
- name属性の値がスネークケース（小文字英数字とアンダースコアのみ、例: `sensor_name`）であること
- 以下のパターンは違反: キャメルケース（`sensorName`）、ケバブケース（`sensor-name`）、大文字混在（`Sensor_Name`）

**違反例:**

```html
<input type="text">
<input type="date" name="measurementDate">
<input type="number" name="Sensor-Value">
```

**正しい例:**

```html
<input type="text" name="sensor_name">
<input type="date" name="measurement_date">
<input type="number" name="sensor_value">
```

---

### R08: select / radio / checkbox に value 属性を付ける [ERROR]

value属性がないとバックエンド側で正しい値を受け取れない。

**チェック条件:**
- `<select>` 内のすべての `<option>` に `value` 属性が存在すること
- `<input type="radio">` および `<input type="checkbox">` に `value` 属性が存在すること
- value属性の値が英数字またはスネークケースであること（日本語テキストは不可）

**違反例:**

```html
<select name="sensor_type">
  <option>温度</option>
  <option>湿度</option>
</select>
<label><input type="radio" name="status"> 有効</label>
```

**正しい例:**

```html
<select name="sensor_type">
  <option value="">選択してください</option>
  <option value="temperature">温度</option>
  <option value="humidity">湿度</option>
</select>
<label><input type="radio" name="status" value="active"> 有効</label>
```

---

### R09: フォーム各項目の直後にエラー表示エリアを配置する [WARNING]

バックエンド側ではバリデーションエラーを各入力項目の直後に表示する。エラー表示エリアがないとバックエンド側で各項目にタグを追加する作業が発生する。

**チェック条件:**
- `<input>`、`<select>`、`<textarea>` の直後（兄弟要素として）にエラーメッセージ用の空要素が配置されていること
- エラーメッセージ用要素は `<span>` または `<p>` で、class名に `error` を含むこと
- 内容は空でよい

**違反例:**

```html
<div class="form-group">
  <label>メールアドレス</label>
  <input type="email" name="email">
</div>
```

**正しい例:**

```html
<div class="form-group">
  <label>メールアドレス</label>
  <input type="email" name="email">
  <span class="error-message"></span>
</div>
```

---

### R10: DOM直接操作のJavaScriptを使用しない [ERROR]

vanilla JavaScript や jQuery など、DOMを直接操作するスクリプトが存在すると、Blade+HTMX変換後に動作が壊れる。

**ただし Alpine.js は使用可。** Alpine.jsはHTML属性ベースの宣言的記述であり、DOMを直接操作しないためBlade+HTMX構成と共存できる。UIの開閉・タブ切替等の軽量なインタラクションにはAlpine.jsを使用してよい。

Alpine.js参考（数時間ほどの学習だけで誰でも書ける様になります）：https://press.monaca.io/atsushi/33849

**チェック条件:**
- `<script>` タグ内に `document.getElementById`、`document.querySelector`、`document.getElementsBy*`、`$(...)` （jQuery）等のDOM操作コードが存在しないこと
- HTML要素に `onclick`、`onchange`、`onsubmit`、`onload`、`onmouseover` 等のイベントハンドラ属性が存在しないこと
- `javascript:` で始まるhref属性が存在しないこと
- jQuery（`<script src="...jquery...">` や `$(...)` 構文）が使用されていないこと
- Alpine.js の CDN読み込み（`<script src="...alpinejs...">`）は違反ではない
- Alpine.js のHTML属性（`x-data`、`x-show`、`x-on:click`、`@click`、`x-bind`、`x-text`、`x-model` 等）は違反ではない

**違反例:**

```html
<script>
  document.getElementById('menu').style.display = 'block';
</script>

<script src="https://code.jquery.com/jquery-3.7.1.min.js"></script>
<script>
  $('.menu-toggle').click(function() { $('.menu-content').toggle(); });
</script>

<button onclick="toggleMenu()">メニュー</button>

<a href="javascript:void(0)">リンク</a>
```

**正しい例:**

```html
<!-- Alpine.js CDN読み込み -->
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

<!-- Alpine.jsによるUI開閉 -->
<div x-data="{ open: false }">
  <button @click="open = !open" class="menu-toggle">メニュー</button>
  <nav x-show="open" class="menu-content">
    <ul>...</ul>
  </nav>
</div>
```

---

### R11: 繰り返し要素は2〜3件のサンプルとループ範囲を明記する [WARNING]

バックエンド側ではループ処理で繰り返し要素を生成する。サンプルが1件だけだとデザイン崩れに気づけない。

**チェック条件:**
- `<tbody>` 内の `<tr>` が2件以上存在すること
- カード一覧の繰り返し要素が2件以上存在すること

**違反例:**

```html
<tbody>
  <tr>
    <td>温度センサーA</td><td>25.3℃</td>
  </tr>
</tbody>
```

**正しい例:**

```html
<tbody>
  <tr>
    <td>温度センサーA</td><td>25.3℃</td>
  </tr>
  <tr>
    <td>湿度センサーB</td><td>60%</td>
  </tr>
</tbody>
```

---

### R12: データ0件時の「空の状態」メッセージを用意する [WARNING]

データが0件の場合に何も表示されないとユーザーが混乱する。バックエンド側で表示/非表示を切り替えるが、メッセージ要素がないとバックエンド側で追加が必要になる。

**チェック条件:**
- テーブルやカード一覧を含むラッパー要素内に、0件時メッセージ用の要素が存在すること
- メッセージ要素は `style="display:none;"` で初期非表示になっていること
- 「ありません」「見つかりません」「0件」等の0件を示すテキストを含むこと

**違反例:**

```html
<div class="sensor-list">
  <table>...</table>
</div>
```

**正しい例:**

```html
<div class="sensor-list">
  <table>...</table>
  <p class="empty-message" style="display:none;">該当するデータはありません。</p>
</div>
```

---

### R13: 画像パスはルート相対パス（/始まり）で統一する [WARNING]

バックエンド側でパスを一括変換するため、パス形式が統一されている必要がある。`../` を使った相対パスやローカルの絶対パスが混在すると変換漏れが発生する。

**チェック条件:**
- `<img>` の `src` 属性、`<link>` の `href` 属性（CSS読み込み）が `/` で始まること
- `../` で始まる相対パスが存在しないこと
- ドライブレター（`C:\` 等）やユーザーディレクトリ（`/Users/` 等）のローカルパスが存在しないこと

**違反例:**

```html
<img src="../assets/img/logo.png" alt="ロゴ">
<img src="C:\Users\xxx\Desktop\logo.png" alt="ロゴ">
```

**正しい例:**

```html
<img src="/images/logo.png" alt="ロゴ">
```

---

### R14: モーダルは `</body>` 直前にまとめて配置する [WARNING]

バックエンド側ではモーダルの中身を動的に差し替えるため、モーダル要素がページ中に散在していると管理が困難になる。

**チェック条件:**
- `modal` を含むclass名を持つ要素が、他のコンテンツ要素より後ろ（`</body>` の直前付近）に配置されていること
- モーダル要素が `style="display:none;"` で初期非表示になっていること
- モーダルがコンテンツ領域の途中に挿入されていないこと

**違反例:**

```html
<div class="content">
  <div class="modal-confirm">...</div>
  <p>本文...</p>
  <div class="modal-detail">...</div>
</div>
```

**正しい例:**

```html
<div class="content">
  <p>本文...</p>
</div>

<!-- モーダル群 -->
<div class="modal-confirm" style="display:none;">
  <div class="modal-content">...</div>
</div>
<div class="modal-detail" style="display:none;">
  <div class="modal-content">...</div>
</div>
```

---

更新日時: 2026-02-24
