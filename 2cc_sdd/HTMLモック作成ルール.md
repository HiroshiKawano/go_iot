# HTMLモック作成ルール

HTMLモックから templ + HTMX 変換をスムーズに行うためのルール集です。
R01〜R27（R01-2、R16-2、R22-2含む）は基本ルール、TS01〜TS07はTom Select標準化に伴う補足ルール、TL01〜TL14は templ 変換時の構造・クラス統一ルールです。

本プロジェクトは **Go 1.26 + Gin + templ + HTMX + Alpine.js**（スタイルは CSS フレームワークを使わない素のモダンCSS）構成です。モックHTMLは後で templ コンポーネント（`internal/view/{layout,component,page}`）に組み込まれ、HTMX 属性（`hx-get` / `hx-post` 等）はバックエンド側（Go）で付与されます。
関連文書: 画面設計書(静的).md（画面定義）/ HTMX実装ガイド(動的).md（動的振る舞いの設計）/ システム構成図.md（ルーティング・エラー方針）。

> **CSS方針の唯一の正は `.kiro/steering/tech.md`「CSS方針」**です。外部CSSフレームワーク（Lism CSS を含む）は再導入しません。スタイルは `mocks/html/style.css` に自前で定義したトークン（`--space-*` / `--fs-*` / `--radius` / `--shadow-*` / `--color-*` 等）と、自前の `@layer reset, base, components, utilities;` カスケードで統一します。

**重要度レベル:**
- **【ERROR】** — templ + HTMX 変換時に構造の組み直しが必要になる。必ず修正すること。
- **【WARNING】** — 変換は可能だがバックエンド側で追加作業が発生する。修正を推奨。

---

## R01: id属性を使用しない 【ERROR】

id属性は templ 変換時にバックエンド側が付与する（HTMX の `hx-target` 等の差し替えターゲット用）。モック段階でidが存在すると、バックエンド側のid付与と衝突する。

※ R01-2 に定める label 連携用の例外を除く。

**チェック条件:**
- HTML要素に `id="..."` 属性が存在しないこと（R01-2 の例外を除く）

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

## R01-2: radio / checkbox の label 連携 【WARNING】

`input type="radio"` および `input type="checkbox"` には、対応する `label` を必ず配置すること。
原則として label が input を内包する形（内包型）を採用する。

デザイン上 label が input を内包できない場合に限り、`for`/`id` による関連付け（関連付け型）を使用してよい（R01 の例外）。

**チェック条件:**
- すべての radio / checkbox に対応する label が存在すること
- 原則: label 要素が input を内包していること（内包型）
- 例外: label の `for` 属性と input の `id` 属性が一致していること（関連付け型）
- 関連付け型の id は label 連携専用とし、JS制御・CSS指定・テスト識別子への流用を禁止する
- 関連付け型の id の命名規則: `field_項目名_値`（例: `field_is_active_true`。スネークケース統一、R07準拠）

**正しい例（内包型・推奨）:**
```html
<label>
  <input type="radio" name="is_active" value="1"> 稼働中
</label>
<label>
  <input type="checkbox" name="remember" value="1"> ログイン状態を保持
</label>
```

**正しい例（関連付け型・内包できない場合のみ）:**
```html
<input type="radio" name="is_active" value="1" id="field_is_active_true">
<span class="note">※ 補足テキスト</span>
<label for="field_is_active_true">稼働中</label>
```

※ 現行モックでは全件が内包型で実装されており、関連付け型の実需は確認されていない。
関連付け型は将来デザイン上内包できないケースが発生した場合の備えとして用意している。

---

## R02: CSSでIDセレクタ（#）を使用しない 【ERROR】

R01の通りモック段階ではid属性を原則使用しないため、CSSでもIDセレクタは使用できない。

※ R01-2 の例外で label 連携用の id が存在する場合でも、R01-2 にて CSS指定への流用は禁止されているため、本ルールに影響しない。

**チェック条件:**
- 外部CSSファイル内に `#` で始まるセレクタが存在しないこと

※ `<style>` タグ自体がインラインスタイル禁止ルールにより使用不可のため、チェック対象は外部CSSファイル（`style.css`）のみ。

**違反例:**
```css
#sensor-table { border: 1px solid #ccc; }
```

**正しい例:**
```css
.sensor-table { border: 1px solid #ccc; }
```

※ CSSの色コード（例: `#ccc`, `#ff0000`）はIDセレクタではないので違反ではない。

---

## R03: テーブル・一覧などの動的コンテンツはラッパー要素で囲む 【ERROR】

バックエンド側ではラッパー要素を HTMX の差し替えターゲットとして使用する。
テーブルやカード一覧を直接配置すると、差し替え用の「器」がないため構造の組み直しが必要になる。

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

## R04: テーブルは thead と tbody を分離する 【ERROR】

バックエンド側ではtheadを固定したままtbody部分のみを差し替えるパターンを多用する。
thead/tbodyが分かれていないと構造の組み直しが必要になる。

**チェック条件:**
- すべての `<table>` 要素が `<thead>` と `<tbody>` の両方を子要素として持つこと
- ヘッダー行（`<th>` を含む行）は `<thead>` 内に配置されていること
- データ行（`<td>` を含む行）は `<tbody>` 内に配置されていること

**違反例:**
```html
<table>
  <tr><th>計測日時</th><th>温度</th></tr>
  <tr><td>2026-02-19 14:30</td><td>28.50℃</td></tr>
</table>
```

**正しい例:**
```html
<table>
  <thead>
    <tr><th>計測日時</th><th>温度</th></tr>
  </thead>
  <tbody>
    <tr><td>2026-02-19 14:30</td><td>28.50℃</td></tr>
    <tr><td>2026-02-19 14:25</td><td>28.30℃</td></tr>
  </tbody>
</table>
```

---

## R05: フォーム要素は `<form>` タグで囲む 【ERROR】

バックエンド側ではフォーム送信時にセキュリティトークン（CSRF）等を埋め込む必要がある。
`<form>` タグがないと構造の組み直しが必要になる。

**チェック条件:**
- `<input>`（`type="hidden"` を除く）、`<select>`、`<textarea>` 要素が `<form>` 要素の子孫であること
- `<form>` 要素に `action` 属性が存在すること（値は `#` や仮のURLで可）

※ フィルター用の `<select>` も `<form>` 内に配置してよい。
バックエンド側でHTMX属性（`hx-get`, `hx-trigger="change"` 等）に変換するため、モック段階では通常のフォーム要素として扱う。

※ **`<form>` のネスト禁止（HTML仕様）:** 1つの `<form>` の内部に別の `<form>` を配置してはならない。HTMLの仕様で `<form>` のネストは禁止されており、ブラウザは内側の `<form>` を無視する。メインフォームの中にファイルアップロード用フォームを配置するとアップロードが動作しない。ファイルアップロードエリアはメインフォームの外に配置するか、templ 変換時に `<form>` タグを使わず JavaScript（fetch + FormData）で処理する（HTMX実装ガイド(動的).md §25 参照）。

※ **モーダル内のデータ送信フォーム:** `<form>` は `modal-body` と `modal-footer` の両方を包含する位置に配置すること（R06・R27準拠）。`<button type="submit">` が `<form>` の外にあるとフォーム送信が機能しない。CSSレイアウト用のクラスは `<form>` ではなく内部の `<div>` に付与し、フォーム構造とレイアウト構造を分離すること。

**正しい例（モーダル内データ送信フォーム）:**
```html
<div class="modal-content">
  <header class="modal-header">...</header>

  <form method="POST" action="#">
    <div class="modal-body">
      <div class="form-group">  <!-- レイアウト用クラスはdivに -->
        <input type="text" name="device_name">
        <span class="error-message"></span>
      </div>
    </div>
    <footer class="modal-footer">
      <a href="#" class="btn btn-secondary">キャンセル</a>
      <button type="submit" class="btn btn-primary">登録</button>
    </footer>
  </form>
</div>
```

---

## R06: ボタンの type 属性を必ず明示する 【ERROR】

フォームの送信動作を伴うボタンは `<button type="submit">` を使用する。
`<a>` タグを `onclick` 等で疑似的に送信ボタンとして使用することは禁止。
フォーム送信を伴わないボタン（モーダルを閉じる、Alpine.jsでUIを操作する等）には `<button type="button">` を明示すること。
`<button>` のデフォルトは `submit` のため、type省略はフォーム内で意図しないフォーム送信が発生する。

※ 画面遷移のみを行うリンク（「詳細を見る」「キャンセル」など）は `<a>` タグで問題ありません。送信動作を伴うかどうかが判断基準です。

**チェック条件:**
- `<form>` 内で送信を行うボタンが `<button type="submit">` であること
- フォーム送信を伴わないボタンが `<button type="button">` であること
- `<button>` に `type` 属性が省略されていないこと
- `<a>` タグに `onclick` 等を付けてフォーム送信を行っていないこと

**違反例:**
```html
<form action="#">
  <input type="text" name="keyword">
  <a href="#" class="btn" onclick="submit()">検索</a>
  <button class="btn-cancel">キャンセル</button>  <!-- type省略: submit扱いになる -->
</form>
```

**正しい例:**
```html
<form action="#">
  <input type="text" name="keyword">
  <button type="submit" class="btn">検索</button>
  <button type="button" class="btn-cancel">キャンセル</button>
</form>
```

---

## R07: フォーム要素に name 属性を付ける（スネークケース） 【ERROR】

バックエンド側はname属性でフォームの値を受け取る（Gin の `c.PostForm("name")`、または Go 構造体の `form:"..."` タグ + `ShouldBind`）。HTML の `<form>` 要素と、Go 構造体フィールドに付ける `form:"..."` タグは別物である点に注意。name属性がないとバックエンド側で全要素に属性を追加する作業が発生する。

**チェック条件:**
- `<form>` 内のすべての `<input>`（`type="submit"` を除く）、`<select>`、`<textarea>` に `name` 属性が存在すること
- name属性の値がスネークケース（小文字英数字とアンダースコアのみ、例: `mac_address`）であること
- 以下のパターンは違反: キャメルケース（`macAddress`）、ケバブケース（`mac-address`）、大文字混在（`Mac_Address`）
- 複数選択の `<select multiple>` では `name` 属性に `[]` を付与すること（例: `name="device_ids[]"`）

**違反例:**
```html
<input type="text">
<input type="date" name="measurementDate">
<input type="number" name="Sensor-Value">
<select name="device_ids" multiple>...</select>  <!-- []がない -->
```

**正しい例:**
```html
<input type="text" name="device_name">
<input type="date" name="measurement_date">
<input type="number" name="sensor_value">
<select name="device_ids[]" multiple>...</select>
```

---

## R08: select / radio / checkbox に value 属性を付ける 【ERROR】

value属性がないとバックエンド側で正しい値を受け取れない。

**チェック条件:**
- `<select>` 内のすべての `<option>` に `value` 属性が存在すること
- `<input type="radio">` および `<input type="checkbox">` に `value` 属性が存在すること
- value属性の値が英数字・スネークケース、または DB の Enum 格納値（比較演算子 `>` `<` `>=` `<=` 等の記号）であること（日本語テキストは不可）

**違反例:**
```html
<select name="metric">
  <option>温度</option>
  <option>湿度</option>
</select>
<label><input type="radio" name="is_active"> 稼働中</label>
```

**正しい例:**
```html
<select name="metric">
  <option value="">選択してください</option>
  <option value="temperature">温度</option>
  <option value="humidity">湿度</option>
</select>
<label><input type="radio" name="is_active" value="1"> 稼働中</label>
```

※ value は DB の格納値・Enum 値（`temperature` / `humidity`、`>` / `<` / `>=` / `<=` 等）に合わせること（DB設計書.md の Enum定義参照）。

---

## R09: データ送信フォームの各項目にエラー表示エリアを配置する 【WARNING】

バックエンド側ではバリデーションエラー（go-playground/validator）を各入力項目の直後に表示する。
エラー表示エリアがないとバックエンド側で各項目にタグを追加する作業が発生する。

**対象:** データ送信フォーム（登録・編集・ログイン等、サーバーにデータを送信するフォーム）の入力項目。

**対象外:** フィルター・検索フォーム（一覧画面の絞り込み・検索等、データの表示条件を変えるだけのフォーム）の入力項目にはエラー表示エリアは不要。

**チェック条件:**
- データ送信フォーム内の `<input>`、`<select>`、`<textarea>` の直後（兄弟要素として）にエラーメッセージ用の空要素が配置されていること
- エラーメッセージ用要素は `<span>` または `<p>` で、class名に `error` を含むこと（本プロジェクトは `error-message`）
- 内容は空でよい

**違反例:**
```html
<!-- データ送信フォーム: error spanがない -->
<div class="form-group">
  <label>メールアドレス</label>
  <input type="email" name="email">
</div>
```

**正しい例:**
```html
<!-- データ送信フォーム: error spanあり -->
<div class="form-group">
  <label>メールアドレス</label>
  <input type="email" name="email">
  <span class="error-message"></span>
</div>

<!-- Tom Select 適用の select でも同様 -->
<div class="form-group">
  <label>デバイス</label>
  <select name="device_id" class="js-tom-select">
    <option value="">選択してください</option>
    <option value="1">ハウスA温湿度計</option>
    <option value="2">ハウスB温湿度計</option>
  </select>
  <span class="error-message"></span>
</div>

<!-- フィルター・検索フォーム: error spanは不要 -->
<form action="#">
  <select name="device_id" class="js-tom-select">
    <option value="">すべて</option>
    <option value="1">ハウスA温湿度計</option>
  </select>
  <input type="text" name="keyword" placeholder="検索...">
  <button type="submit">検索</button>
</form>
```

※ Tom Select は初期化時に元の `<select>` を非表示にし、代わりに独自のUI要素（`.ts-wrapper` > `.ts-control` 等）を生成する。
そのため、HTML上で `<select>` の直後に配置したエラーメッセージの `<span>` が、視覚的にはTom SelectのUIの下に表示されてずれる場合がある。
外部CSSファイル（`style.css`）で `.form-group .error-message` の位置を調整すること（TS03準拠）。

---

## R10: DOM直接操作のJavaScriptを使用しない 【ERROR】

vanilla JavaScript や jQuery など、DOMを直接操作するスクリプトが存在すると、templ + HTMX 変換後に動作が壊れる。
ただし Alpine.js は使用可。Alpine.jsはHTML属性ベースの宣言的記述であり、DOMを直接操作しないため templ + HTMX 構成と共存できる。
UIの開閉・タブ切替等の軽量なインタラクションにはAlpine.jsを使用してよい。

**チェック条件:**
- `<script>` タグ内に `document.getElementById`、`document.querySelector`、`document.getElementsBy*`、`$(...)` （jQuery）等のDOM操作コードが存在しないこと
- HTML要素に `onclick`、`onchange`、`onsubmit`、`onload`、`onmouseover` 等のイベントハンドラ属性が存在しないこと
- `javascript:` で始まるhref属性が存在しないこと
- jQuery（`<script src="...jquery...">` や `$(...)` 構文）が使用されていないこと
- Alpine.js の CDN読み込み・HTML属性（`x-data`、`x-show`、`@click` 等）は違反ではない

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
<script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>

<div x-data="{ open: false }">
  <button @click="open = !open" class="menu-toggle">メニュー</button>
  <nav x-show="open" class="menu-content">
    <ul>...</ul>
  </nav>
</div>
```

※ **Tom Select の例外:** Tom Select の初期化コード（`new TomSelect(...)` および `document.querySelectorAll('select.js-tom-select:not([disabled])')`）は例外として許可する。
ただし TS02 に従い `</body>` 直前の1箇所にまとめること。
Tom Select 以外の DOM 直接操作・jQuery は引き続き禁止。

---

## R11: 繰り返し要素は2〜3件のサンプルとループ範囲を明記する 【WARNING】

バックエンド側では templ の `for` ループで繰り返し要素を生成する。サンプルが1件だけだとデザイン崩れに気づけない。また、どこからどこまでがループ対象なのかが不明確だと templ 変換時に判断に迷う。

**チェック条件:**
- `<tbody>` 内の `<tr>` が2件以上存在すること
- カード一覧の繰り返し要素が2件以上存在すること
- `<select>` 内の `<option>` が2件以上存在すること（プレースホルダー `<option value="">選択してください</option>` を除く実データの選択肢）
- ループ対象の範囲がHTMLコメントで明示されていること

**違反例:**
```html
<!-- tbody: サンプルが1件のみ -->
<tbody>
  <tr>
    <td>2026-02-19 14:30</td><td>28.50℃</td>
  </tr>
</tbody>

<!-- select: サンプルが1件のみでは選択UIの幅やUXが確認できない -->
<select name="device_id" class="js-tom-select">
  <option value="1">ハウスA温湿度計</option>
</select>
```

**正しい例:**
```html
<!-- tbody: 2件以上のサンプル -->
<tbody>
  <!-- ループ開始 -->
  <tr>
    <td>2026-02-19 14:30</td><td>28.50℃</td>
  </tr>
  <tr>
    <td>2026-02-19 14:25</td><td>28.30℃</td>
  </tr>
  <!-- ループ終了 -->
</tbody>

<!-- select: 2件以上のサンプル -->
<select name="device_id" class="js-tom-select">
  <option value="">選択してください</option>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
</select>
```

※ テーブルの場合は `<tbody>` 直下の `<tr>` が自明なループ対象であればコメント省略可。
カード一覧など繰り返し単位が複数要素にまたがる場合は必ずコメントで範囲を示すこと。
※ `<select>` の場合、プレースホルダー（`value=""` の初期選択肢）はサンプル件数に含めない。バックエンド側が templ 変換時に選択肢の表示幅やUXを確認できるよう、実データの選択肢を2件以上用意すること。

---

## R12: データ0件時の「空の状態」メッセージを用意する 【WARNING】

データが0件の場合に何も表示されないとユーザーが混乱する。
バックエンド側で表示/非表示を切り替えるが、メッセージ要素がないとバックエンド側で追加が必要になる。

**チェック条件:**
- テーブルやカード一覧を含むラッパー要素内に、0件時メッセージ用の要素が存在すること
- メッセージ要素は `x-show="false"` で初期非表示になっていること
- 「ありません」「見つかりません」「0件」等の0件を示すテキストを含むこと

※ `style="display:none;"` や `hidden` 属性ではなく、Alpine.js の `x-show="false"` を使用すること（R15参照）。

**違反例:**
```html
<div class="device-grid">
  <!-- デバイスカード群 -->
</div>
```

**正しい例:**
```html
<div class="device-grid">
  <!-- デバイスカード群 -->
  <p class="empty-message" x-show="false">登録されているデバイスはありません。</p>
</div>
```

---

## R13: リソースパスは相対パス（`./` 始まり）で統一する 【WARNING】

モックHTMLはすべて `mocks/html/` にフラットに配置され、CSS等のリソースも同階層にある（`style.css` を同階層に置く）。リソースパスは同階層相対パス（`./` 始まり）で統一する。ブラウザでHTMLを直接開いてプレビューできるようにするためである。
ファイル名のみの相対パスやルート絶対パス・ローカルの絶対パスが混在すると、プレビュー不可や変換漏れが発生する。

**チェック条件:**
- `<link>` の `href` 属性（CSS読み込み）が `./` で始まる相対パスであること（例: `./style.css`）
- `<img>` の `src` 属性が `./` で始まる相対パスであること
- ルート絶対パス（`/css/style.css` 等）が存在しないこと
- ドライブレター（`C:\` 等）やユーザーディレクトリ（`/Users/` 等）のローカルパスが存在しないこと

※ 外部CDN（`https://` で始まるURL）は本ルールの対象外。Alpine.js・Tom Select等のCDN読み込み（例: `https://cdn.jsdelivr.net/npm/...`）は許可される。

**違反例:**
```html
<link rel="stylesheet" href="style.css">
<link rel="stylesheet" href="/css/style.css">
<img src="C:\Users\xxx\Desktop\logo.png" alt="ロゴ">
```

**正しい例:**
```html
<link rel="stylesheet" href="./style.css">
<img src="./images/logo.png" alt="農業IoTロゴ">
```

※ templ 変換時、バックエンド側で `public/` 配下を起点とした配信パスに置き換える。モック段階では同階層相対（`./`）で統一しておけばよい。

---

## R14: モーダルは `</body>` 直前にまとめて配置する 【WARNING】

バックエンド側ではモーダルの中身を動的に差し替えるため、モーダル要素がページ中に散在していると管理が困難になる。

**チェック条件:**
- `modal` を含むclass名を持つ要素が、他のコンテンツ要素より後ろ（`</body>` の直前付近）に配置されていること
- モーダル要素が `x-show="false"` で初期非表示になっていること
- モーダルがコンテンツ領域の途中に挿入されていないこと

※ `style="display:none;"` や `hidden` 属性ではなく、Alpine.js の `x-show="false"` を使用すること（R15参照）。
※ templ 変換時は、参照する Alpine 変数が同一の `x-data` スコープ内に収まるよう配置すること（TL06 参照）。

**違反例:**
```html
<div class="content">
  <div class="modal-overlay">...</div>
  <p>本文...</p>
  <div class="modal-overlay">...</div>
</div>
```

**正しい例:**
```html
<div class="content">
  <p>本文...</p>
</div>

<!-- モーダル群 -->
<div class="modal-overlay" x-show="false">
  <div class="modal-content">...</div>
</div>
```

※ **Tom Select との注意点:** モーダル内に `<select class="js-tom-select">` を配置する場合、モーダルが `x-show="false"`（= `display:none`）で非表示の状態ではブラウザが要素の幅を計算できないため、Tom Select が初期化されると幅が0で描画される等の不具合が発生する可能性がある。
この問題は templ 変換時にバックエンド側がモーダル表示イベントで Tom Select を再初期化する対応で解決する（HTMX実装ガイド(動的).md §16 / C12 参照）。モック段階ではこの問題を認識した上で、通常通り `js-tom-select` クラスを付与してよい。

---

## R15: インラインスタイル（style属性）と `<style>` タグを使用しない 【ERROR】

`style` 属性は templ 変換後に Alpine.js や HTMX で制御するため、モック段階でインラインスタイルが混在すると管理が困難になる。
表示/非表示の制御には Alpine.js の `x-show="false"` を使用すること。

また、`<style>` タグによるページ内CSS定義も禁止。スタイルはすべて外部CSSファイル（`style.css`）に記述すること。
`<style>` タグが各HTMLファイルに散在すると、templ レイアウト統合時にスタイルの重複・競合が発生する。

**チェック条件:**
- HTML要素に `style="..."` 属性が存在しないこと
- `<style>` タグが存在しないこと（スタイルはすべて外部CSSファイルに記述）
- 表示/非表示の初期状態制御に `x-show="false"` を使用していること

**違反例:**
```html
<div style="display:none;">非表示コンテンツ</div>
<p style="color: red;">エラー</p>

<style>
  .custom-table { border: 1px solid #ccc; }
</style>
```

**正しい例:**
```html
<!-- Alpine.js で表示/非表示を制御 -->
<div x-show="false">非表示コンテンツ</div>

<!-- スタイルは外部CSSファイル（style.css）に記述 -->
```

※ `hidden` 属性ではなく `x-show="false"` を使用する。
Alpine.jsがページ全体で読み込まれている前提のため、`x-show` で統一することで表示制御の仕組みが一元化される。
※ スペース・フォントサイズ・角丸・影・境界色などは `style.css` の自前トークン（`--space-1`〜`--space-10`、`--fs-sm`/`--fs-base`/`--fs-lg`/`--fs-xl`/`--fs-2xl`、`--radius`、`--shadow-sm`/`--shadow-md`、`--color-border` 等）を優先利用する。ハードコード値は使わない。コンポーネント固有のスタイルは `@layer components` に追記する（CSS方針の正は `.kiro/steering/tech.md`「CSS方針」を参照）。

---

## R16: カスタムドロップダウン禁止、すべての `<select>` に Tom Select を適用する 【ERROR】

カスタムドロップダウン（`<nav>` + `<button>` + Alpine.js による開閉制御）は原則禁止。
すべてのドロップダウンはネイティブの `<select>` 要素で実装し、Tom Select を適用すること。

※ R22-2 に定める少数固定選択の例外を除く。
※ R16-2 に定めるヘッダーユーザーメニューの例外を除く。

Tom Select はネイティブ `<select>` を起点に検索可能なUIを生成するバニラJSライブラリである。
jQuery 不要で templ + HTMX + Alpine.js 構成と共存でき、以下の利点がある：

- `<select>` を起点とするため、Tom Select の JS/CSS が読み込まれなくてもネイティブ `<select>` として機能する
- HTMX変換時に `hx-trigger="change"` で動作する（Tom Select が元の `<select>` の `change` イベントを発火する）
- `name` 属性で選択値が自動送信され、フォーム連携が容易
- 検索・絞り込み機能がネイティブに備わり、選択肢が多い場合のUXが向上する
- Alpine.js による開閉状態管理（`menuOpen` 等）が不要になり、templ 変換がシンプルになる
- ネイティブ `<select>` がベースのためキーボード操作・スクリーンリーダー等のアクセシビリティが確保される

**チェック条件:**
- すべてのドロップダウンが `<select>` + `<option>` で実装されていること（R16-2 の例外を除く）
- すべての `<select>`（`disabled` 属性付き、および R22-2 の対象を除く）に Tom Select マーカークラス `js-tom-select` が付与されていること
- `<option>` に `value` 属性が付与されていること（R08準拠）
- `<nav>` + `<button>` + Alpine.js（`x-show` + `@click`）によるカスタムドロップダウンが存在しないこと（R16-2 の例外を除く）
- `disabled` 属性付きの `<select>` には `js-tom-select` クラスを付与しないこと（TS02参照）

**違反例:**
```html
<!-- カスタムドロップダウン: 原則禁止 -->
<div x-data="{ menuOpen: false }">
  <button type="button" @click="menuOpen = !menuOpen">
    ハウスA温湿度計 ▼
  </button>
  <nav x-show="menuOpen" @click.outside="menuOpen = false">
    <button type="button" class="dropdown-item">ハウスA温湿度計</button>
    <button type="button" class="dropdown-item">ハウスB温湿度計</button>
  </nav>
</div>

<!-- マーカークラスなしのselect: Tom Selectが適用されない -->
<select name="device_id">
  <option value="">選択してください</option>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
</select>
```

**正しい例:**
```html
<!-- Tom Select を適用するselect -->
<select name="device_id" class="js-tom-select">
  <option value="">選択してください</option>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
</select>

<!-- 複数選択 -->
<select name="device_ids[]" class="js-tom-select" multiple>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
  <option value="3">育苗ハウス温湿度計</option>
</select>
```

---

## R16-2: ヘッダーユーザーメニューは Alpine.js カスタムドロップダウンを許可する 【WARNING】

ヘッダーのユーザーメニュー（ユーザー名 + 個人設定・ログアウト等のアクション一覧）は、R16 の例外として Alpine.js によるカスタムドロップダウンを許可する。

**理由:**
- ユーザーメニューはユーザー名/アイコンをトリガーとするUIパターンであり、`<select>` + Tom Select ではアクションリンク表示ができない
- 選択肢がフォームデータの送信ではなくアクション実行（画面遷移・ログアウト等）であるため、`name`/`value` によるフォーム連携が不要
- templ 変換時にバックエンド側が Go のセッション認証（scs）で取得したユーザー情報に置き換えるため、モック段階の構造は変換時に再構成される

**チェック条件:**
- 本例外の適用はヘッダー内（`.site-header`）のユーザーメニューのみに限定すること
- ユーザー名テキスト（`.user-name`）が表示されていること
- ドロップダウンの開閉は Alpine.js（`x-data` + `x-show` + `@click`）で制御すること
- ドロップダウン項目はアクションに応じて `<a href>`（画面遷移）または `<form>`（ログアウト等の送信）で実装すること
- 外側クリックで閉じる制御（`@click.outside`）を含むこと

**正しい例:**
```html
<div class="user-menu" x-data="{ userMenuOpen: false }">
  <button type="button" class="user-name" @click="userMenuOpen = !userMenuOpen">
    テストユーザー ▼
  </button>
  <nav class="user-dropdown" x-show="userMenuOpen" @click.outside="userMenuOpen = false" x-cloak>
    <a href="/settings" class="dropdown-item">個人設定</a>
    <form method="POST" action="/logout" class="logout-form">
      <button type="submit" class="dropdown-item">ログアウト</button>
    </form>
  </nav>
</div>
```

※ 画面設計書(静的).md の共通レイアウトでは「ユーザー名 + ログアウト」のシンプルな構成。ドロップダウンにまとめず横並びで配置する場合も、ログアウトは `<form method="POST">`（`.logout-form`）で実装すること（GETでのログアウトは不可）。
※ ヘッダー以外のコンテンツ領域で同様のアクションメニューが必要な場合は、本例外の適用外とし、R16に従って `<select>` + Tom Select で実装すること。

---

## R17: 検索・フィルターフォームは `method="GET"` を使用する 【ERROR】

HTMX変換時、検索・フィルター操作はGETリクエストで行う。
`method="POST"` を指定すると `hx-boost` によるブラウザ履歴・戻るボタンが正しく動作しない。
また、検索条件がURLパラメータに含まれないためブックマークやリンク共有ができなくなる。

**チェック条件:**
- 検索フォーム・フィルターフォームの `<form>` タグに `method="GET"` が指定されている、または `method` 属性が省略されていること（HTMLのデフォルトはGET）
- `method="POST"` はデータ送信フォーム（登録・編集・削除等、サーバーのデータを変更する操作）にのみ使用すること

**違反例:**
```html
<!-- 検索フォームにPOSTを使用 -->
<form method="POST" action="#" class="filter-form">
  <input type="date" name="from">
  <button type="submit">検索</button>
</form>
```

**正しい例:**
```html
<!-- method省略（デフォルトGET）またはGETを明示 -->
<form action="#" class="filter-form">
  <input type="date" name="from">
  <input type="date" name="to">
  <button type="submit">検索</button>
</form>
```

---

## R18: 選択式の入力は `<input>` + ボタンではなく `<select>` + Tom Select を使用する 【ERROR】

選択肢から値を選ぶフィールドを `<input type="text">` + `<button>▼</button>` で疑似的に実装してはならない。
HTMX変換時に `hx-trigger="change"` で値の変更を検知できず、余分なJavaScriptが必要になる。

表示件数切替（「20件表示 ▼」等）も同様に `<select>` で実装すること。
HTMX変換時に `hx-get` + `hx-trigger="change"` だけで動作し、追加のJavaScriptが不要になる。

**チェック条件:**
- 選択肢から選ぶフィールドは `<select class="js-tom-select">` + `<option>` で実装されていること
- `<input>` の直後に `▼` ボタンがある疑似ドロップダウンパターンが存在しないこと
- 表示件数切替が `<select>` + `<option>` で実装されていること
- `<span>` + `<button>▼</button>` による疑似セレクターが存在しないこと

**違反例:**
```html
<!-- input + ▼ボタンによる疑似ドロップダウン -->
<div class="form-group">
  <label class="form-label">比較条件</label>
  <input type="text" name="operator" placeholder="比較条件">
  <button type="button" class="btn-dropdown-sm">&#9660;</button>
</div>

<!-- span + ボタンによる表示件数切替 -->
<div class="sub-filter-right">
  <span class="table-info">20件表示</span>
  <button type="button" class="btn btn-icon-sm">&#9660;</button>
</div>
```

**正しい例:**
```html
<!-- 比較演算子は4値固定 → R22-2 によりネイティブselect。value は DB 格納値の記号をそのまま使う（R08・TL05） -->
<div class="form-group">
  <label class="form-label">比較条件</label>
  <select name="operator">
    <option value="">選択してください</option>
    <option value=">">より大きい</option>
    <option value="<">より小さい</option>
    <option value=">=">以上</option>
    <option value="<=">以下</option>
  </select>
</div>

<!-- ネイティブselectによる表示件数切替（R22-2: Tom Select不要） -->
<div class="sub-filter-right">
  <select name="per_page" class="form-select-sm">
    <option value="10">10件表示</option>
    <option value="20" selected>20件表示</option>
    <option value="50">50件表示</option>
  </select>
</div>
```

※ R16がAlpine.js（`x-show` + `@click`）によるカスタムドロップダウンを禁止するのに対し、本ルールは `<input>` + `<button>` や `<span>` + `<button>` による疑似ドロップダウンを禁止する。いずれも `<select>` に統一する（Tom Select の適用は R16・R22-2 に従う。上の比較演算子のように少数固定選択は R22-2 でネイティブ select 許可、DB由来で件数が増える選択肢は Tom Select を適用する）。

---

## R19: ページネーションは `<a>` タグで実装する 【ERROR】

HTMX変換時に `hx-boost` でページネーションをAjax化する。
`hx-boost` は `<a>` タグの `href` 属性を利用するため、`<button>` では動作しない。
また `<a href>` により、JavaScript無効環境でも通常のページ遷移として機能する。

**チェック条件:**
- ページネーションのリンク（ページ番号、前へ、次へ、最初、最後）が `<a>` タグで実装されていること
- 各 `<a>` タグに `href` 属性が存在すること（値は `?page=N` 形式のダミーURLで可）
- `<button>` でページネーションが実装されていないこと
- 無効状態のリンク（現在ページ、範囲外等）は `<span>` または `aria-disabled="true"` 付きの `<a>` で表現すること

**違反例:**
```html
<div class="pagination">
  <button type="button" class="btn-page" disabled>&lt;&lt;</button>
  <button type="button" class="btn-page active">1</button>
  <button type="button" class="btn-page">2</button>
  <button type="button" class="btn-page">&gt;&gt;</button>
</div>
```

**正しい例:**
```html
<div class="pagination">
  <span class="btn-page disabled">&lt;&lt;</span>
  <span class="btn-page active">1</span>
  <a href="?page=2" class="btn-page">2</a>
  <a href="?page=3" class="btn-page">3</a>
  <a href="?page=2" class="btn-page">&gt;&gt;</a>
</div>
```

※ センサーデータ履歴・アラート履歴は20件/ページ（画面設計書(静的).md 参照）。

---

## R20: ソート可能なテーブルヘッダーは `<a>` タグでリンクする 【WARNING】

HTMX変換時に `hx-boost` でソートをAjax化する。
ソート可能なヘッダーセル内にリンクがないとバックエンド側で構造の追加が必要になる。

**チェック条件:**
- ソート可能な `<th>` 内のテキストとソートアイコンが `<a>` タグで囲まれていること
- `<a>` タグに `href` 属性が存在すること（値は `?sort=column_name&order=asc` 形式のダミーURLで可）
- ソートアイコン（↕ 等）がリンク内に含まれていること

**違反例:**
```html
<thead>
  <tr>
    <th class="col-recorded-at">計測日時 <span class="sort-icon">↕</span></th>
    <th class="col-temperature">温度 <span class="sort-icon">↕</span></th>
    <th class="col-humidity">湿度 <span class="sort-icon">↕</span></th>
  </tr>
</thead>
```

**正しい例:**
```html
<thead>
  <tr>
    <th class="col-recorded-at">
      <a href="?sort=recorded_at&order=desc">計測日時 <span class="sort-icon">↕</span></a>
    </th>
    <th class="col-temperature">
      <a href="?sort=temperature&order=asc">温度 <span class="sort-icon">↕</span></a>
    </th>
    <th class="col-humidity">
      <a href="?sort=humidity&order=asc">湿度 <span class="sort-icon">↕</span></a>
    </th>
  </tr>
</thead>
```

※ ソート不要なカラム（操作ボタン列等）はリンク不要。ソートアイコンが付いているカラムのみが対象。

---

## R21: 画面遷移は `<a href>` タグで実装する（`<button>` + `location.href` 禁止） 【ERROR】

HTMX変換時に `hx-boost` で画面遷移をAjax化する。
`hx-boost` は `<a>` タグの `href` 属性を利用するため、`<button @click="location.href='...'">` では動作しない。
また `<a href>` により、JavaScript無効環境でも通常のページ遷移として機能する（プログレッシブ・エンハンスメント）。

**チェック条件:**
- 画面遷移を行うすべての要素が `<a>` タグで実装されていること
- 各 `<a>` タグに `href` 属性が存在すること（値は仮のURLで可）
- `<button @click="location.href='...'">` や `<button @click="window.location='...'">` によるページ遷移が存在しないこと
- Alpine.js の `@click` でページ遷移を行うパターンが存在しないこと

**違反例:**
```html
<!-- buttonでページ遷移 -->
<button type="button" @click="location.href='/devices/1'">詳細を見る</button>
<button type="button" class="btn" @click="window.location='/dashboard'">ダッシュボード</button>
```

**正しい例:**
```html
<!-- aタグでページ遷移 -->
<a href="/devices/1" class="btn">詳細を見る</a>
<a href="/dashboard" class="btn">ダッシュボード</a>
```

※ R19（ページネーション）・R20（ソートヘッダー）と同じ原則。`hx-boost` の恩恵を受けるには `<a href>` が必須。
ボタンのスタイルが必要な場合は `<a>` にボタン用のクラス（`btn` 等）を付与すればよい。

---

## R22: ファイルアップロードエリアには `<input type="file">` を必ず含める 【ERROR】

HTMX変換時にファイルアップロードは `hx-encoding="multipart/form-data"` でHTMXが処理する。
ドラッグ＆ドロップ用のUIエリアだけでは `<input type="file">` がないためHTMXによるファイル送信ができない。

> ※ 本プロジェクトの現行画面（画面設計書(静的).md）にはファイルアップロード画面は含まれないが、将来 CSV取込等を追加する場合の汎用ルールとして記載する。

**チェック条件:**
- ファイルアップロードを行うエリア（「ファイルをドラッグ」等のテキストを含むエリア）内に `<input type="file">` が存在すること
- `<input type="file">` に `name` 属性が付与されていること（R07準拠、スネークケース）
- 複数ファイルアップロードが想定される場合は `multiple` 属性が付与されていること
- `<input type="file">` は `<form>` の子孫であること（R05準拠）

**違反例:**
```html
<!-- input type="file" がないドラッグ＆ドロップエリア -->
<div class="file-upload-area">
  <p>ファイルをドラッグ＆ドロップまたはクリックしてアップロード</p>
</div>
```

**正しい例:**
```html
<!-- input type="file" を含むアップロードエリア -->
<div class="file-upload-area">
  <p>ファイルをドラッグ＆ドロップまたはクリックしてアップロード</p>
  <input type="file" name="upload_file" multiple>
</div>
```

※ `<input type="file">` の見た目はCSSで調整可能（非表示にしてエリア全体をクリック可能にする等）。
モック段階ではデフォルトの表示で問題ない。バックエンド側でHTMX属性（`hx-post`, `hx-encoding` 等）を付与する。

※ **メインフォームとのネストに注意:** ファイルアップロードエリアがメインフォームの中にある場合、templ 変換時にformネスト禁止の制約に抵触する（R05補足参照）。モック段階では問題にならないが、templ 変換時に `<form>` タグを使わない実装（fetch + FormData）に変換する必要がある（HTMX実装ガイド(動的).md §25 参照）。

---

## R22-2: 少数固定選択はネイティブ select を許可する 【WARNING】

以下に該当する `<select>` は、Tom Select（`js-tom-select` クラス）を適用せず、ネイティブ `<select>` のまま使用してよい（R16 の例外）。

**ネイティブ select 許可の対象（限定列挙）:**
- はい/いいえ（boolean型）
- 稼働中/停止中（status型）
- 表示件数切替（per_page）
- 年/月/日の数値選択
- 比較演算子（operator: より大きい/より小さい/以上/以下 — 4値固定）
- 計測指標（metric: 温度/湿度 — 2値固定）
- DB由来でなく、選択肢が少数かつ固定のもの

上記以外（デバイス一覧など、DB由来で件数が増える選択肢）はすべて Tom Select を適用する（R16準拠）。**迷った場合は Tom Select を適用する。**

**チェック条件:**
- `js-tom-select` クラスのない `<select>`（`disabled` を除く）が、上記の許可対象に該当すること
- 許可対象外の `<select>` に `js-tom-select` クラスがない場合は R16 違反
- ネイティブ select にも R05（form内配置）、R07（name属性）、R08（value属性）、R09（データ送信フォームではエラー表示エリア）、R11（サンプル2件以上）は適用される

**正しい例:**
```html
<!-- 計測指標: 2値固定 → ネイティブselect（Tom Select不要） -->
<select name="metric">
  <option value="">選択してください</option>
  <option value="temperature">温度</option>
  <option value="humidity">湿度</option>
</select>

<!-- 表示件数切替: ネイティブselect（Tom Select不要） -->
<select name="per_page" class="form-select-sm">
  <option value="20" selected>20件表示</option>
  <option value="50">50件表示</option>
  <option value="100">100件表示</option>
</select>

<!-- DB由来の選択肢（件数が増える）: Tom Select必須（R16準拠） -->
<select name="device_id" class="js-tom-select">
  <option value="">選択してください</option>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
</select>
```

※ ネイティブ select でも `<select>` + `<option>` の基本構造は同じため、将来 Tom Select が必要になった場合は `js-tom-select` クラスを追加するだけで移行できる。
※ ネイティブ select はブラウザ標準のUIで表示され、検索・絞り込み機能はない。選択肢が将来的に増える可能性がある場合は、最初から Tom Select を適用しておくことを推奨する。

---

## R23: 日付入力には `<input type="date">` を使用する 【WARNING】

HTMX変換時に `hx-trigger="change"` で日付変更を検知する。
`<input type="text" placeholder="yyyy/MM/dd">` + カレンダーボタンではブラウザネイティブの日付ピッカーが動作せず、JavaScript製の日付ピッカーが必要になる。
`<input type="date">` を使用すればブラウザネイティブの日付ピッカーが利用でき、追加のJavaScriptが不要になる。

**チェック条件:**
- 日付入力フィールドが `<input type="date">` で実装されていること
- `<input type="text">` + カレンダーアイコンボタンによる疑似日付ピッカーが存在しないこと
- `placeholder="yyyy/MM/dd"` や `placeholder="年/月/日"` 等の日付フォーマットを示すプレースホルダーを持つ `<input type="text">` が存在しないこと
- `name` 属性が付与されていること（R07準拠、スネークケース）

**違反例:**
```html
<!-- text入力 + カレンダーボタンによる疑似日付ピッカー -->
<div class="form-group">
  <label class="form-label">開始日</label>
  <input type="text" name="from" placeholder="yyyy/MM/dd">
  <button type="button" class="btn-calendar">&#x1F4C5;</button>
</div>
```

**正しい例:**
```html
<!-- ネイティブ日付ピッカー -->
<div class="form-group">
  <label class="form-label">開始日</label>
  <input type="date" name="from">
</div>
```

※ ブラウザによって日付ピッカーのUIが若干異なるが、モック段階では問題ない。
templ 変換時にバックエンド側（Go）でフォーマット（`2006-01-02` 等）を制御するため、プレースホルダーは不要。

---

## R24: ステータスフィルタ・タブ切替は `<a href>` で実装する 【ERROR】

ステータスフィルタ（稼働中/停止中、未通知/通知済等）やサブタブ切替は `GET /{base}?status={value}` のサーバーリクエストとして実装する。
`<button @click="activeFilter = '...'">` でAlpine.jsのローカル状態を切り替える実装では、以下の問題が発生する：

1. **hx-boost不可**: `<a href>` でないと `hx-boost="true"` が適用できない
2. **URL状態なし**: フィルタ状態がURLに反映されず、ブラウザの戻るボタンで前のフィルタ状態に戻れない
3. **ブックマーク不可**: 特定のフィルタ状態をブックマークしたりURLを共有できない
4. **サーバー通信なし**: Alpine.jsのローカル状態変更はサーバーにリクエストを送らず、実際のフィルタリング結果を取得できない

**チェック条件:**
- ステータスフィルタボタンが `<a href="?status=...">` で実装されていること
- サブタブ切替が `<a href="?...">` で実装されていること
- `<button @click="activeFilter = '...'">` 等のAlpine.jsによるローカル状態切替が使用されていないこと
- アクティブ状態の視覚的区別はCSSクラス（`active`等）で表現すること

**違反例:**
```html
<!-- Alpine.jsでローカル状態を切り替えるステータスフィルタ -->
<div x-data="{ activeFilter: 'all' }" class="filter-buttons">
  <button type="button" class="filter-btn"
    :class="{ 'active': activeFilter === 'all' }"
    @click="activeFilter = 'all'">
    全件
  </button>
  <button type="button" class="filter-btn"
    :class="{ 'active': activeFilter === 'unnotified' }"
    @click="activeFilter = 'unnotified'">
    未通知
  </button>
</div>
```

**正しい例:**
```html
<!-- アンカーリンクによるステータスフィルタ -->
<div class="filter-buttons">
  <a href="?status=all" class="filter-btn active">全件</a>
  <a href="?status=unnotified" class="filter-btn">未通知</a>
  <a href="?status=notified" class="filter-btn">通知済</a>
</div>
```

※ R19（ページネーション）・R20（ソートヘッダー）と同じ理由（hx-boost適用、URL状態保持）だが、対象がフィルタ操作である点が異なる。
HTMX変換時にバックエンド側が `hx-boost="true"` を付与し、`?status=` パラメータでサーバーサイドフィルタリングを行う。
`active` クラスの付与は templ 側で `if currentStatus == "unnotified" { active }` のように制御する（Go の `c.Query("status")` で受け取った値を templ コンポーネントに渡す）。

---

## R25: ダウンロードボタンは `<a href>` で実装する 【ERROR】

HTMXはサーバーからのレスポンスをHTMLとしてDOM操作（innerHTML/outerHTML等）することを前提としている。
ファイルダウンロード（`Content-Disposition: attachment`）はバイナリレスポンスであり、HTMXでは処理できない。
そのため、ダウンロード操作は通常のブラウザナビゲーション（`<a href>`）で実装する必要がある。

**チェック条件:**
- ダウンロードボタン（CSV出力等）が `<a href="...">` で実装されていること
- `<button type="button">` でダウンロード操作が実装されていないこと
- `download` 属性は任意（バックエンド側でContent-Dispositionヘッダーを設定するため）

**違反例:**
```html
<!-- buttonによるダウンロード操作 -->
<button type="button" class="btn btn-primary">センサーデータCSV出力</button>
```

**正しい例:**
```html
<!-- アンカーリンクによるダウンロード操作 -->
<a href="/devices/1/readings/export" class="btn btn-primary">センサーデータCSV出力</a>
```

※ `<a>` に `class="btn btn-primary"` を付与すればボタンと同じ見た目になる。
href値はモック段階ではダミー（`#` や `/placeholder`）でもよい。
`hx-boost="true"` の影響を受けないよう、templ 変換時にバックエンド側で `hx-boost="false"` を明示的に付与する。

---

## R26: `<img>` タグに `alt` 属性を必ず付与する 【WARNING】

templ 変換後もアクセシビリティとSEOを確保するため、すべての `<img>` タグに `alt` 属性を付与する。
`alt` 属性がないとスクリーンリーダーがファイル名を読み上げてしまい、UXが低下する。

**チェック条件:**
- すべての `<img>` タグに `alt` 属性が存在すること
- 意味のある画像には内容を説明する `alt` テキストを設定すること（例: `alt="農業IoTロゴ"`）
- 装飾目的のみの画像は空文字 `alt=""` を許可する

**違反例:**
```html
<img src="./images/logo.png">
<img src="./images/icon-search.png">
```

**正しい例:**
```html
<img src="./images/logo.png" alt="農業IoTロゴ">
<img src="./images/icon-search.png" alt="検索">
<img src="./images/divider.png" alt="">  <!-- 装飾画像 -->
```

---

## R27: 編集画面のフォーム構造 【ERROR】

編集画面（デバイス登録・編集等）は1カラムのフォーム構造を基本とする。`<main>` のクラスとフォーム内構造を正しく使い分けること。
誤ったクラスを指定すると、スクロール不可・アクションバー消失等の表示崩れが発生する。

`<main>` に `main-content` を指定し、内側を `main-inner` でラップする。フォーム全体が `main-content` の `overflow-y: auto` でスクロールされる。
登録・更新・キャンセル等のアクションボタンは `<form>` の末尾（`</form>` の直前）に `form-actions` として配置する。

```html
<main class="main-content">
  <div class="main-inner">
    <form method="POST" action="#">
      <!-- フォームフィールド群 -->
      <div class="form-group">
        <label class="form-label">デバイス名 <span class="required-mark">＊</span></label>
        <input type="text" name="name" value="ハウスA温湿度計">
        <span class="error-message"></span>
      </div>
      <div class="form-group">
        <label class="form-label">MACアドレス <span class="required-mark">＊</span></label>
        <input type="text" name="mac_address" value="AA:BB:CC:DD:EE:FF">
        <span class="error-message"></span>
      </div>

      <!-- アクションボタン（form末尾） -->
      <div class="form-actions">
        <a href="#" class="btn btn-secondary">キャンセル</a>
        <button type="submit" class="btn btn-primary">登録</button>
      </div>
    </form>
  </div>
</main>
```

**チェック条件:**
- アクションボタン群（`form-actions`）が `<form>` の末尾（`</form>` の直前）に配置されていること
- `form-actions` が `<form>` の外に配置されていないこと（フォーム送信にアクション内の値が含まれなくなる）
- 送信ボタンが `<button type="submit">`、キャンセルが `<a href>` であること（R06準拠）
- 各入力項目が `form-group` でラップされ、直後に `error-message` 要素を持つこと（R09準拠）

**よくある誤り:**
- アクションボタンを `<form>` の外（`main-inner` の外）に配置 → 送信時にフッター内の入力値が含まれない／レイアウトがずれる
- 登録と編集で異なるフォーム構造を使う → 登録(`device-create.html`)と編集(`device-edit.html`)は同じフォーム構造にし、編集時はダミーの既存値を `value`/`selected` で埋め込む（画面設計書(静的).md 参照）

---

# Tom Select 補足ルール（TS01〜TS07）

R16の改訂に伴い、Tom Select を標準利用するための補足ルールを定める。
本プロジェクトは素のモダンCSSを基盤とするため、Tom Select のカスタムスタイルは自前スタイル（`style.css` の `@layer components`）と競合しないよう外部CSS（`style.css`）に記述する（TS03・TS07参照）。

---

## TS01: Tom Select の CDN読み込みは `<head>` 内に固定する 【ERROR】

Tom Select の CSS と JS は `<head>` 内で CDN から読み込むこと。
バージョンは統一し、プロジェクト内で複数バージョンが混在しないようにする。

**チェック条件:**
- Tom Select の CSS（`tom-select.css`）が `<head>` 内の `<link>` で読み込まれていること
- Tom Select の JS（`tom-select.complete.min.js`）が `<head>` 内の `<script>` で読み込まれていること
- バージョンがプロジェクト内で統一されていること（推奨: `2.3.1`）
- `<body>` 内での Tom Select ライブラリ読み込みが存在しないこと

**正しい例:**
```html
<head>
  <meta charset="UTF-8">
  <link rel="stylesheet" href="./style.css">
  <!-- Tom Select -->
  <link href="https://cdn.jsdelivr.net/npm/tom-select@2.3.1/dist/css/tom-select.css" rel="stylesheet">
  <script src="https://cdn.jsdelivr.net/npm/tom-select@2.3.1/dist/js/tom-select.complete.min.js"></script>
</head>
```

※ R13（`./` 相対パス）の対象外。外部CDNは `https://` で始まるURLのため R13 に抵触しない。
※ R15（`<style>` タグ禁止）の対象外。外部CSSファイルの `<link>` 読み込みは R15 に抵触しない。

---

## TS02: 初期化スクリプトは共通の1箇所にまとめる 【ERROR】

Tom Select の初期化コードは `</body>` 直前に1つの `<script>` ブロックとしてまとめること。
各 `<select>` の近くにインラインで `<script>` を散在させると、templ 変換時に初期化コードの移行漏れが発生する。

初期化はマーカークラス `js-tom-select` に対する一括初期化パターンで行うこと。
個別要素ごとの初期化コードは禁止。

**チェック条件:**
- Tom Select の初期化コード（`new TomSelect(...)`）が `</body>` 直前の `<script>` ブロック内に1箇所のみ存在すること
- `document.querySelectorAll('select.js-tom-select:not([disabled])')` による一括初期化であること
- HTML中に Tom Select 初期化用の `<script>` が複数箇所に存在しないこと
- `disabled` 属性付きの `<select>` は初期化から除外すること

**違反例:**
```html
<!-- 各selectの近くにインラインで初期化: 禁止 -->
<select name="device_id" class="js-tom-select">...</select>
<script>new TomSelect('.js-tom-select', { ... });</script>

<select name="metric" class="js-tom-select">...</select>
<script>new TomSelect('select[name="metric"]', { ... });</script>
```

**正しい例:**
```html
<select name="device_id" class="js-tom-select">...</select>
<!-- （中略） -->
<select name="metric" class="js-tom-select">...</select>

<!-- Tom Select 初期化（</body>直前に1箇所のみ） -->
<script>
document.querySelectorAll('select.js-tom-select:not([disabled])').forEach(function(el) {
    new TomSelect(el, {
        allowEmptyOption: true,
        dropdownParent: 'body', // 親要素のoverflow:hiddenによるクリッピング防止
        plugins: el.multiple
            ? ['remove_button', 'dropdown_input']
            : ['dropdown_input'], // 検索窓をドロップダウン内に移動（Select2と同様の操作感）
    });
});
</script>
```

※ `allowEmptyOption: true` は TS04（プレースホルダー用の空 `<option>`）と連携して動作する。この設定がないと `value=""` の `<option>` が Tom Select のドロップダウンに表示されず、プレースホルダーとして機能しない。
※ `plugins` に指定しているのは `tom-select.complete.min.js`（フルバンドル版）に同梱されている標準プラグイン。`remove_button` は複数選択時に選択済みアイテムの横に「×」ボタンを表示する。`dropdown_input` は検索窓をドロップダウン内に移動し、Select2と同様の操作感を実現する（TL12参照）。
※ **TS02が定めるのは初期化の構造パターンであり、使用するプラグインの種類およびオプションの追加を制限するものではない。** `tom-select.complete.min.js` に同梱されている標準プラグインは用途に応じて自由に追加してよい。`dropdownParent`・`maxOptions` 等のTom Selectオプションも用途に応じて自由に追加してよい。
※ `disabled` 属性付きの `<select>` は `:not([disabled])` で初期化対象から除外する。disabled な select にTom Select を適用するとネイティブの disabled 表示と異なるUIになり、混乱の原因となる。

※ **z-indexの注意:** `dropdownParent: 'body'` を使用するとドロップダウンが `body` 直下に配置されるため、ヘッダーバー（`.site-header`）の背後に隠れる場合がある。外部CSS（`style.css`）に `.ts-dropdown { z-index: 200 !important; }` をグローバルルールとして定義し、すべてのドロップダウンがヘッダーより上に表示されるようにすること。

※ **`remove_button` プラグインのコンテキスト判定:** `remove_button` プラグインは multiple 選択時のみ使用すること。single select に `remove_button` を適用すると、空option（「すべて」等）がタグ風に `すべて X` と表示され、意図しないUIになる。templ 変換時も同じ判定ロジックを維持すること（HTMX実装ガイド(動的).md §27 参照）。

| コンテキスト | plugins |
|---|---|
| ヘッダーの select | `[]` |
| 検索・フィルターフォーム内の single select | `[]` |
| 編集フォームの single select（`.form-group` 内） | `[]` |
| multiple select | `['remove_button', 'dropdown_input']` |
| その他の single select | `['dropdown_input']` |

**templ 変換時の注意事項:** HTMX実装ガイド(動的).md の「C12: Tom Select + HTMX ライフサイクル管理」「§16 Tom Select 障害パターン集」を参照。初期化の移行先、htmx swap との共存、モーダル内の幅0問題、Alpine.js との状態同期の各項目をコード例付きで詳述している。

---

## TS03: Tom Select のカスタムスタイルは外部CSSファイルに記述する 【ERROR】

Tom Select の見た目を調整するカスタムCSSは外部CSSファイル（`style.css`）に記述すること。
`<style>` タグでのページ内定義は R15 に従い禁止。

**チェック条件:**
- `.ts-wrapper`、`.ts-control`、`.ts-dropdown` 等の Tom Select 関連セレクタが `<style>` タグ内に存在しないこと
- Tom Select のカスタムスタイルが外部CSSファイル（`style.css`）に記述されていること

**違反例:**
```html
<!-- <style>タグ内にTom Select用CSS: R15違反 -->
<style>
  .ts-wrapper.single .ts-control {
      border-radius: 8px;
  }
</style>
```

**正しい例:**
```css
/* style.css 内に記述（角丸・余白等は自前トークンを利用） */
.ts-wrapper.single .ts-control {
    border-radius: var(--radius);
}
```

※ **モーダル内のz-index対応:** モーダル内で Tom Select を使用する場合、Tom Select のドロップダウン（`.ts-dropdown`）がモーダルの背面に隠れることがある。以下のようにz-indexを外部CSSで調整すること。

```css
/* style.css 内に記述 */
/* モーダル内の Tom Select ドロップダウンがモーダルより前面に表示されるようにする */
.modal-overlay .ts-dropdown {
    z-index: 10060;  /* モーダルのz-indexより大きい値 */
}
```

---

## TS04: 単一選択ではプレースホルダー用の空 `<option>` を先頭に置く 【WARNING】

単一選択の Tom Select では、プレースホルダーテキスト（「選択してください」等）を表示するために
`value=""` の `<option>` を先頭に配置すること。
R08（value属性必須）にも準拠し、Tom Select なしでも同じ挙動になる。

**チェック条件:**
- 単一選択（`multiple` なし）の `js-tom-select` 付き `<select>` の先頭 `<option>` が `value=""` であること（下記の対象外ケースを除く）
- プレースホルダーテキストが設定されていること

**対象外（プレースホルダー不要なケース）:** 以下に該当する `<select>` は本ルールのチェック条件の対象外とする。
- 表示件数切替など、必ずいずれかの値が選択される `<select>` ではプレースホルダー用の空 `<option>` は不要。`selected` 属性でデフォルト値を指定すること。※ 表示件数切替は R22-2 によりネイティブ select を使用するため、本ルール（TS04）の適用対象外となる
- ユーザーメニュー等のアクション選択（R16-2 のヘッダーユーザーメニュー例を参照）

**正しい例:**
```html
<!-- 通常の選択フィールド: プレースホルダーあり -->
<select name="device_id" class="js-tom-select">
  <option value="">選択してください</option>
  <option value="1">ハウスA温湿度計</option>
  <option value="2">ハウスB温湿度計</option>
</select>

<!-- 表示件数切替: R22-2 によりネイティブselect（Tom Select不要・プレースホルダー不要） -->
<select name="per_page" class="form-select-sm">
  <option value="20" selected>20件表示</option>
  <option value="50">50件表示</option>
  <option value="100">100件表示</option>
</select>
```

※ TS02 の初期化コードで `allowEmptyOption: true` を設定することで、プレースホルダー用の空 `<option>` が Tom Select に正しく認識される。

---

## TS05: Tom Select 適用時も `<select>` の既存ルールをすべて遵守する 【ERROR】

Tom Select を適用する `<select>` は通常の `<select>` と同じルールに従うこと。
Tom Select の使用は「見た目と検索機能の拡張」であり、HTML構造の要件を免除するものではない。

**遵守すべき既存ルール:**
- R05: `<form>` タグ内に配置
- R07: `name` 属性をスネークケースで付与（複数選択は `[]` を付与）
- R08: `<option>` に `value` 属性を付与（英数字またはスネークケース）
- R09: データ送信フォームではエラー表示エリアを `<select>` の直後に配置
- R11: `<option>` のサンプルを2件以上用意
- R17: 検索・フィルターフォームは `method="GET"`

※ Tom Select は元の `<select>` を非表示にして独自のUI要素に置き換えるため（R09の注記参照）、R09 のエラー表示 `<span>` の視覚的位置がずれる場合がある。
`.form-group` 等のラッパー要素内でエラー表示位置を制御するCSSを外部CSSファイル（`style.css`）に記述すること（TS03準拠）。

---

## TS06: `<input>` + ボタンによる疑似セレクトも Tom Select で置き換える 【ERROR】

R18（`<input>` + ボタン禁止）と同様の趣旨だが、Tom Select 導入に伴い明示する。
`<input type="text">` + `<button>▼</button>` による疑似ドロップダウンは禁止。
すべて `<select>` に統一すること（Tom Select の適用は R16・R22-2 に従う）。

**チェック条件:**
- `<input>` の直後に `▼` ボタンがある疑似ドロップダウンパターンが存在しないこと
- `<span>` + `<button>▼</button>` による疑似セレクターが存在しないこと
- 上記パターンはすべて `<select>` に置き換えられていること（Tom Select の適用は R16・R22-2 に従う）

※ R18 の内容と重複するが、Tom Select 標準化に伴い「何に置き換えるか」を明示するために併記する。

---

## TS07: Tom Select 適用時のCSS競合は「親セレクタ経由パターン」で解決する 【ERROR】

既存のカスタムスタイルが付いた `<select>` に Tom Select を適用すると、Tom Select が生成する `.ts-wrapper` > `.ts-control` 等の要素と元のCSSが競合し、見た目が崩れる（余分なpadding、背景色の二重適用、▼ボタンの消失、浮いた表示等）。`<select>` に付与した自前ユーティリティクラス（`.u-*`）やフォームスタイルクラスが `.ts-wrapper` に継承されて崩れることもある。

この問題は **親セレクタ経由パターン**（崩れている `<select>` の親要素にスコープしたCSSを `style.css` に追加する）で解決すること。

**親セレクタ経由パターンの構成（4段階）:**

```css
/* 1. .ts-wrapper のリセット（継承スタイルを確実に打ち消す） */
.対象の親セレクタ .ts-wrapper.single {
  position: relative;
  padding: 0;
  border: none;
  background: none;
  box-shadow: none;
  min-height: 0;
  border-radius: var(--radius);
  overflow: hidden;       /* 内部要素を角丸でクリップ */
}

/* 2. .ts-control に元selectと同じ見た目を適用 */
.対象の親セレクタ .ts-wrapper.single .ts-control {
  background-color: var(--surface, #fff);
  border: 1px solid var(--color-border);
  border-radius: var(--radius) 0 0 var(--radius);  /* 左だけ角丸 */
  padding: 0 var(--space-2) !important;
  min-height: 32px;
  line-height: 32px;
  border-right: none;
  box-shadow: none;
}

/* 3. .ts-control のデフォルト ::after を非表示 */
.対象の親セレクタ .ts-wrapper.single .ts-control::after {
  display: none;
}

/* 4. .ts-wrapper の ::after で▼ボタンを配置 */
.対象の親セレクタ .ts-wrapper.single::after {
  content: '';
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 32px;
  background: var(--color-primary) url("data:image/svg+xml;...") center/14px no-repeat;
  border-radius: 0 var(--radius) var(--radius) 0;  /* 右だけ角丸 */
  pointer-events: none;
  z-index: 10;
}
```

**重要: 親セレクタ要件**

Tom Select のCSS制御は**親要素セレクタを経由してのみ機能する**。`<select>` に自前ユーティリティクラス（`.u-*`）やフォームスタイルクラスを直接付与した状態で Tom Select を適用すると、それらの padding/border が `.ts-wrapper` に継承され、見た目が崩れる。

```html
<!-- ❌ 禁止: フォームスタイルクラスと js-tom-select の直接併用（崩れる） -->
<select name="device_id" class="form-control-styled js-tom-select">...</select>

<!-- ✅ 正しい: 親要素セレクタ経由で制御 -->
<div class="form-group">
  <select name="device_id" class="js-tom-select">...</select>
</div>
```

**⚠️ よくあるハマりパターン: クラス継承による二重スタイル**

Tom Select は初期化時に元の `<select>` のクラスを `.ts-wrapper` にコピーする。そのため `<select class="form-control form-select-sm js-tom-select">` のように書くと、`.form-control` 等の border/padding/height が `.ts-wrapper` にそのまま適用され、内側の `.ts-control` のスタイルと二重になり、枠とテキストの位置がずれる。

**対処法:** 4段階パターンの第1段階（`.ts-wrapper` のリセット）で `border: none; padding: 0; min-height: 0; background: transparent; box-shadow: none;` を必ず指定し、継承されたスタイルを確実にリセットすること。

**適用手順:**

1. 崩れている `<select>` の親要素（`.form-group`、`.filter-form` 等）を特定する
2. 上記4段階のCSSを「対象の親セレクタ」を置き換えて `style.css` に追加する
3. 高さ・フォントサイズ・ボーダー色は元の `<select>` のスタイル（自前トークン `--fs-*`・`--color-border` 等）に合わせて調整する
4. `<select>` にフォームスタイルクラスが直接付与されていないことを確認する

**チェック条件:**
- Tom Select を適用した `<select>` が、元のネイティブ `<select>` と同等の見た目で表示されていること
- `.ts-wrapper` に余分なpadding・border・box-shadowが残っていないこと
- ▼ボタンが右端に表示され、角丸が正しく適用されていること
- ドロップダウンメニューが正しい幅・位置で表示されること
- フォームスタイルクラスと `js-tom-select` の直接併用が存在しないこと

※ 新しい画面で Tom Select の見た目が崩れた場合は、`style.css` の参照実装をコピーし、親セレクタを対象要素に合わせて調整すること。詳細は HTMX実装ガイド(動的).md §16（Tom Select 障害パターン集）を参照。

---

# 付録

## 既存ルールと Tom Select の関係整理

| 既存ルール | Tom Select との関係 | 対応 |
|-----------|-------------------|------|
| R01（id属性禁止） | Tom Select は内部で id を生成する | **抵触しない**。Tom Select が生成する内部 id はモック HTML に記述するものではない（R01-2 の label 連携用 id とも無関係） |
| R05, R07, R08, R11 | `<select>` の構造ルール | **そのまま適用**（TS05） |
| R09（エラー表示エリア） | Tom Select がDOMを変更しエラー表示位置がずれる | **CSSで対応**。外部CSS（`style.css`）で位置調整（TS03・TS05） |
| R10（DOM直接操作禁止） | `new TomSelect()` は DOM を操作する | **例外として許可**。TS02 の一括初期化パターンに限定。使用するプラグインおよびオプションはTS02の制限を受けない |
| R13（`./` 相対パス） | Tom Select の CDN は外部URL | **抵触しない**。`https://` で始まるURLは対象外（TS01） |
| R14（モーダル配置） | モーダル非表示時に初期化すると幅が壊れる | **templ 変換時に対応**。モーダル表示時に再初期化（HTMX実装ガイド §16 / C12） |
| R15（`<style>` タグ禁止） | Tom Select の CSS が必要 | **抵触しない**。CDN `<link>` 読み込みは R15 対象外（TS01）。カスタムCSSは `style.css` に記述（TS03・TS07） |
| R16（カスタムドロップダウン禁止） | **改訂**：Tom Select を標準化 | `<select>` + Tom Select に統一。ただし R22-2 の少数固定選択は例外 |
| R18（疑似セレクト禁止） | 置き換え先が明確になった | `<select>` に置き換える。Tom Select の適用は R16・R22-2 に従う（TS06 で補足） |
| R22-2（少数固定選択） | R16 の例外を限定列挙 | boolean型・status型・per_page・年月日・metric・operator・DB由来でない固定選択はネイティブ select を許可 |

---

# templ 変換時の注意事項（TL01〜）

モック → templ 変換時に頻発する構造・クラス・スコープの問題パターン。Go + Gin + templ + HTMX + Alpine.js 構成に固有の注意点をまとめる。
（本セクションは Laravel/Blade 前提の旧ルール集を本プロジェクト向けに再編したもの。Blade 固有・別ドメイン固有のルールは末尾の「削除した別プロジェクト固有ルール」を参照）

---

## TL01: ボタンには必ず `btn` ベースクラスを含める 【ERROR】

ボタンの色・形状クラス（`btn-primary`, `btn-secondary`, `btn-danger`, `btn-small` 等）は `btn` ベースクラスと併用すること。`style.css` は `.btn` に `border-radius`, `padding`, `font-size` 等のベーススタイルを定義しているため、`btn` なしでは角張ったボタンになる。

**違反例:**
```html
<!-- btn ベースクラスがない → ベーススタイルが適用されない -->
<button type="button" class="btn-secondary">キャンセル</button>
<button type="submit" class="btn-primary">登録</button>
```

**正しい例:**
```html
<button type="button" class="btn btn-secondary">キャンセル</button>
<button type="submit" class="btn btn-primary">登録</button>
```

※ `<a>` をボタン表示にする場合も同様（`<a href="#" class="btn btn-secondary">`）。

---

## TL02: Alpine.js の `x-data` に配列・オブジェクトを渡す時は安全な値渡しを使う 【ERROR】

templ で Alpine の `x-data` 属性にサーバーデータ（スライス/マップ）を渡す場合、属性値に JSON 文字列を直接連結すると、属性のクォートやエスケープと衝突して Alpine の初期化に失敗しやすい。JSON は `<script type="application/json">` に出力し、Alpine 側は `JSON.parse` で読み込むのが堅牢。

**templ実装側のパターン（推奨）:**
```html
<!-- サーバーが JSON を script タグに出力（templ 側で値を JSON エンコードして埋め込む） -->
<script type="application/json" data-devices>[{"id":1,"name":"ハウスA温湿度計"}]</script>

<div x-data="{ items: JSON.parse(document.querySelector('[data-devices]').textContent), selected: null }">
    ...
</div>
```

**モック側で注意すること:**
- モック段階では `x-data="{ items: [], selected: null }"` のように**空配列・サンプル値**を Alpine 式として静的に記述する（テンプレート段階では空でよい）
- 実データの注入は templ 側で行う前提のため、モックに巨大なJSONをベタ書きしない
- 参照する変数のスコープは TL06（モーダル等を同一 `x-data` スコープ内に置く）と整合させる

関連: HTMX実装ガイド(動的).md §10-E（`<script type="application/json">` で値を渡す方針）・§32（同節は Laravel の `@json()`/`Js::from()` で記述されているが、本プロジェクトでは上記の script タグ + `JSON.parse` 方式に読み替える）

---

## TL03: `<fieldset disabled>` を disabled 制御に使用する場合は CSS リセットが必要 【WARNING】

HTML の `<fieldset disabled>` は配下の全フォーム要素を一括無効化できる便利な手段だが、ブラウザデフォルトで枠線（border）・余白（padding/margin）が付与される。モック段階では見えにくいが、disabled 制御に `<fieldset>` を使用すると意図しない枠が表示される。

**チェック条件:**
- `<fieldset>` を使用する場合、CSS で枠線・余白をリセットすること

**正しい例（CSS でリセット）:**
```css
/* style.css */
fieldset {
  border: none;
  padding: 0;
  margin: 0;
  min-inline-size: 0;
}
```

関連: HTMX実装ガイド(動的).md §53（fieldset のブラウザデフォルトスタイルリセット）

---

## TL04: 全角/半角文字の統一ルール 【WARNING】

モックと templ 変換後で全角/半角が混在するとUIの不一致が発生する。以下の文字は全角に統一すること。

**チェック条件:**
- 確認ダイアログの疑問符は全角 `？` を使用すること（半角 `?` は禁止）
- 必須注記のアスタリスクは全角 `＊` を使用すること（例: `＊は必須入力です`）
- 必須マークは全角 `＊` を使用すること（例: `<span class="required-mark">＊</span>`）
- ページタイトルの区切りを使う場合は全角 `＞` を使用すること（例: `デバイス管理 ＞ 新規登録`）

**違反例:**
```
このデバイスを削除しますか?          ← 半角?
*は必須入力です                      ← 半角*
```

**正しい例:**
```
このデバイスを削除しますか？          ← 全角？
＊は必須入力です                      ← 全角＊
```

---

## TL05: モックのHTMLエンティティは templ 化時に UTF-8 文字へ変換する 【ERROR】

**問題:** モックHTMLで使用する `&#176;`（°）や `&#8597;`（↕）等のHTMLエンティティは、HTML直書き部分では正しくレンダリングされる。しかし templ 化時に Go 文字列リテラル内に `"&#176;"` のまま埋め込み templ の `{ }`（自動HTMLエスケープ）で出力すると、`&amp;#176;` に二重変換され、画面にリテラル文字が表示される。

**ルール:**

| 場所 | HTMLエンティティ | 対応 |
|------|--------------|------|
| HTML直書き部分（templ 式の外） | `&#8597;` 等 | そのまま使用可 |
| templ `{ }` で出力する Go 文字列 | `"&#176;"` | **禁止** → `"℃"` 等の UTF-8 文字に変換 |

**チェック対象エンティティ（本ドメインで頻出）:**
- `℃`（温度の単位。`&#8451;` をベタ書きせず文字で）
- `%`（湿度の単位）
- `&#8597;` → `↕`（ソートアイコン）
- `&#9654;` → `▶`（展開ボタン）
- `&#9660;` → `▼`（折りたたみ・select の矢印）

**注意:** ソートアイコン（`↕`）や展開ボタン（`▶`/`▼`）はHTML直書き部分や Alpine の `x-text` 内で使われることが多く、その場合はそのままで問題ない。Go 文字列に埋め込んで templ で出力する場合のみ注意が必要。

関連: HTMX実装ガイド(動的).md §59（`{{ }}` と HTMLエンティティの二重エスケープ問題。templ の `{ }` も同様に自動エスケープする）

---

## TL06: モーダルは templ 化時に Alpine.js の `x-data` スコープ内に配置する 【ERROR】

**問題:** モックHTMLのR14「モーダルは `</body>` 直前に配置」に従うと、templ 化時にページコンポーネントの `x-data` スコープの**外側**にモーダルが出てしまう場合がある。Alpine.js の `x-show` で参照する変数がスコープ外になり、コンソールエラー（`Alpine Expression Error: ... is not defined`）が発生する。

**ルール:** templ 化時のモーダル配置は、`x-show`/`@click` で参照する変数を定義した `x-data` の**スコープ内**に収める。

```
モックHTML:
  <body x-data="{ confirmDeleteOpen: false }">  ← body全体がスコープ
    ...main content...
    <div class="modal-overlay" x-show="confirmDeleteOpen">...</div>  ← スコープ内
  </body>

templ:
  templ DeviceShow(device Device) {
    <div x-data="{ confirmDeleteOpen: false }">  ← ページコンポーネント内のdivがスコープ
      ...main content...
      <div class="modal-overlay" x-show="confirmDeleteOpen">...</div>  ← この中に配置
    </div>
  }
```

**チェック条件:**
- モーダルの `x-show` / `@click` で参照する変数が、同一の `x-data` スコープ内に定義されていること
- ブラウザの DevTools Console に `Alpine Expression Error: ... is not defined` が出ていないこと

関連: HTMX実装ガイド(動的).md §62（`x-data` スコープ外のモーダル配置によるAlpine.jsエラー）

---

## TL07: アクションセルは `<td>` 直下ではなく内側 `<div>` で flex ラップする 【ERROR】

**問題:** 操作列の td に直接 `display: flex` を当てると、td 要素の `display` が `table-cell` から `flex` に変わり、**行内の他セルと高さが揃わない**バグが発生する（セル下に数 px の隙間ができ、背景色の境界線が見える）。

**ルール:** アクションセルの flex 配置は **`<td>` 直下ではなく内側 `<div>` に適用する**。

```html
<!-- ✅ 推奨: td は table-cell のまま、内側 div で flex -->
<td class="actions-cell">
    <div class="actions-cell-inner">
        <a href="/devices/1/edit" class="btn btn-small">編集</a>
        <button type="button" class="btn btn-small btn-danger">削除</button>
    </div>
</td>
```

```css
/* style.css */
.actions-cell { vertical-align: middle; text-align: center; }
.actions-cell-inner { display: flex; gap: var(--space-1); justify-content: center; }
```

```html
<!-- ❌ NG: td に直接 display: flex -->
<td class="actions-cell" style="display: flex; gap: 4px;">
    <a href="#">編集</a>
    <button>削除</button>
</td>
```

**適用範囲:** テーブル内のすべての `<td>` で flex / grid レイアウトを使いたい箇所（アラートルール一覧の操作列、デバイス一覧等）。

関連: HTMX実装ガイド(動的).md §12（インラインCRUD方針）、R15（インラインスタイル禁止）

---

## TL08: 保存後の遷移先をコメントで明記する 【WARNING】

**問題:** 「登録」「更新」「保存」ボタンを持つ編集画面で、保存後にどこへ遷移するかがモックに書かれていないと、templ + Handler 実装者が推測で実装する。結果として「画面コンテキストが失われる」「入力内容が反映されていないように見える」等のUX問題が発生する。

**ルール:** 保存ボタンのすぐそばに**遷移先とエラー時の挙動をコメントで明記**する。

```html
<div class="form-actions">
    <a href="/dashboard" class="btn btn-secondary">キャンセル</a>
    <!-- 「キャンセル」: 前の画面に戻る -->

    <button type="submit" class="btn btn-primary">登録</button>
    <!--
        保存成功時:
        - デバイス登録（POST /devices）→ /devices/{device}（作成したデバイスの詳細）にリダイレクト
        - デバイス編集（PUT /devices/{device}）→ /devices/{device}（更新後の詳細）にリダイレクト
          （HTMX 経由なら HX-Redirect ヘッダで遷移）
        保存失敗時:
        - 422 + 部分HTML（templ エラーコンポーネント）でフォームを再描画し、入力値を復元（TL10）
    -->
</div>
```

**ポイント:**
- **成功時の遷移先 URL**（パラメータ含む）
- **失敗時のエラー表示方法**（インライン / 部分HTML）
- **画面コンテキスト（選択中のデバイス・検索条件・期間タブ等）を維持するか**

> ※ デバイス登録・編集はHTMXを使わずフルページPOSTで実装する（画面設計書(静的).md）。HTMX リクエストのリダイレクトは `HX-Redirect` を使う（HTMX実装ガイド(動的).md §9）。

**適用範囲:** 登録・更新・削除等のアクションを持つすべての画面。

関連: 画面設計書(静的).md「フォーム送信後のリダイレクト先」、HTMX実装ガイド(動的).md §9（`HX-Redirect` 使用方針）

---

## TL09: ネイティブ `<select>` のデフォルト option は「業務的な初期値」を推奨 【WARNING】

**問題:** TS04 は Tom Select 向けの「プレースホルダー用空 option」ルールだが、少数固定選択の**ネイティブ select（R22-2 適用）で、画面コンテキストから自動判定できる初期値があるケース**では、「選択してください」空 option は不要なことが多い。逆に空 option があると、実装者が「required バリデーションで空を弾くべきか」の判断に迷う。

**ルール:** 少数固定選択のネイティブ `<select>`（R22-2）で、**画面コンテキストから業務的に初期値が決まる**場合、モック段階で**初期値が selected な option を先頭**に配置する。コメントで「どの条件で初期値が決まるか」を明記。

```html
<!-- ✅ 推奨: 期間選択 — デフォルトは24時間 -->
<select name="period">
    <!-- デフォルトは直近24時間表示 -->
    <option value="24h" selected>24時間</option>
    <option value="7d">7日間</option>
    <option value="30d">30日間</option>
</select>

<!-- △ 可: 業務的な初期値が決まらない場合は空 option を先頭に -->
<select name="metric">
    <option value="" selected>選択してください</option>
    <option value="temperature">温度</option>
    <option value="humidity">湿度</option>
    <!-- バリデーション: required（空選択は弾く） -->
</select>
```

**ポイント:**
- **業務的に初期値が決まる場合**: 空 option 不要、該当 option に `selected`
- **ユーザーが明示的に選ぶ必要がある場合**: 空 option を先頭に（`required` で空を弾く）

関連: R22-2（少数固定選択）、TS04（Tom Select の空 option）、R09（バリデーションエラー表示）

---

## TL10: フォーム再表示時は入力値を保持する 【ERROR】

**問題:** Go + Gin には Laravel の `old()` のような暗黙の入力値保持機構はない。バリデーションエラー時に Handler が何も渡さずにフォームを再描画すると、入力値が初期値（空 / DB 保存値）に戻る。

**ルール:** データ送信フォーム（R09 対象）のバリデーションエラー時は、Handler が `ShouldBind` で受け取ったリクエスト構造体の値を templ フォームコンポーネントに渡して再描画し、入力値を復元する。モック段階では `value="(初期値)"` / `selected` / `checked` のように「初期値があれば表示される」構造になっていれば、templ 側で渡された値にバインドできる。

**templ + Handler 実装側のパターン:**
```go
// Handler: バリデーション失敗時は受け取った値をそのまま渡して再描画
func (h *DeviceHandler) Create(c *gin.Context) {
    var req CreateDeviceRequest
    // フォーム送信(application/x-www-form-urlencoded)は ShouldBind、JSON送信(hx-post でJSON)は ShouldBindJSON
    if err := c.ShouldBind(&req); err != nil {
        // req（ユーザー入力値）と errs を templ に渡して再描画 → 入力値が保持される
        // validationErrors: validator のエラーを map[string]string(項目名→メッセージ)に変換する自作ヘルパー（実装側で用意）
        c.Status(http.StatusUnprocessableEntity)
        page.DeviceForm(req, validationErrors(err)).Render(c.Request.Context(), c.Writer)
        return
    }
    // ...
}
```
```go
// templ: 渡された値を value/selected にバインド
templ DeviceForm(form CreateDeviceRequest, errs map[string]string) {
    <input type="text" name="name" value={ form.Name }>
    <span class="error-message">{ errs["name"] }</span>
}
```

**モック側で注意すること:**
- 値が入る想定の項目は、`value="..."` または `selected` / `checked` を付けて「初期値を持つ構造」を明示する

**除外:**
- パスワード系（`type="password"`）は**意図的にクリア**するのが普通なので対象外
- CSRFトークン、`<input type="hidden" name="_method">` 等のフレームワーク側管理項目

関連: R09（エラー表示エリア）、R07（name 属性）、HTMX実装ガイド(動的).md §7（バリデーションエラー表示方針）

---

## TL11: エラー表示の配置 — フルページは共通レイアウト、HTMX fragment は fragment 内に含める 【ERROR】

**問題:** Go + templ には Laravel の共有 `$errors` バッグのような暗黙機構はない。エラーは Handler から templ コンポーネントへ**明示的に渡す**必要がある。配置を誤ると、HTMX 部分更新時にエラーが表示されない、またはフルページで重複表示される。

**ルール:** システム構成図.md「エラーレスポンス方針」に従い、リクエスト種別でエラー表示先を分ける。

| リクエスト種別 | エラー表示先 |
|---|---|
| フルページ（初回ロード・通常フォーム送信）| 共通レイアウト or ページ先頭の1箇所に集約（重複配置しない）|
| HTMX 部分更新（4xx を返す）| 返却する**部分HTML（fragment）内**にエラー表示要素を含め、エラーデータを明示的に渡す（layout 側のバナーは swap されないため）|

**モック側で注意すること:**
- データ送信フォームは各項目の直後に `error-message` 要素を置く（R09）
- 一覧画面で HTMX 検索を想定する場合、部分更新で返す fragment の先頭に `<div class="alert-banner" x-show="false"><!-- エラー表示 --></div>` 等のプレースホルダを配置する
- 編集画面モックの冒頭に独立した「エラー表示エリア」を重複して設けない（共通レイアウト側で表示）

関連: システム構成図.md「エラーレスポンス方針」、HTMX実装ガイド(動的).md §7（バリデーションエラー表示方針）、R09

---

## TL12: ヘッダー外の single select には Tom Select の `dropdown_input` プラグインを使用する 【ERROR】

**問題:** Tom Select をプラグインなしで初期化すると、control 要素そのものがテキスト入力可能になり、以下の不具合が発生する：

1. **Safari 固有**: control input のイベント順序バグにより、選択値がフォーカスアウトまで表示に反映されない
2. **全ブラウザ共通**: 選択後の表示文字列が編集可能になり、Backspace で削除できてしまう

**ルール:** Tom Select の一括初期化（TS02）で、**ヘッダー内 select 以外のすべての single select** に `plugins: ['dropdown_input']` を適用する。これにより検索 input がドロップダウン内に配置され、control 要素は表示専用（編集不可）になる。

```js
// ✅ 正解: ヘッダー以外は dropdown_input を使用
var isHeaderSelect = !!el.closest('.site-header');
var useDropdownInput = el.multiple || !isHeaderSelect;

new TomSelect(el, {
    allowEmptyOption: true,
    dropdownParent: 'body',
    plugins: el.multiple
        ? ['remove_button', 'dropdown_input']
        : (useDropdownInput ? ['dropdown_input'] : []),
});
```

**モック側で注意すること:** モック HTML は Tom Select 適用前提（R16 / TS01〜TS07）なので、モック段階での対応は不要。「検索フォーム内の select で、ユーザーがタイプして絞り込みたい」意図がある場合は、検索 input がドロップダウン内に表示される挙動で問題ないかをモックレビュー時に確認する。

関連: R16（Tom Select 全適用）、TS02（初期化スクリプト統一）、HTMX実装ガイド(動的).md §16 / §27

---

## TL13: ヘッダーバーに `overflow: hidden` を指定しない 【ERROR】

**問題:** ヘッダーバー（`.site-header`）内に `position: absolute` で表示されるドロップダウン（ユーザーメニュー等を Alpine.js の `x-show` で開閉）は、親の `.site-header` に `overflow: hidden` が指定されていると**ヘッダー領域外にはみ出した部分が切り取られて非表示**になる。結果、ドロップダウンそのものが見えなくなる／クリック不能になる。

**ルール:** `.site-header` には `overflow: hidden` を指定しない。デフォルト値の `visible` のままにする。ヘッダー内コンテンツのはみ出し対策は、**個別要素側**で `max-width` + `text-overflow: ellipsis` 等で対応する。

```css
/* ❌ 違反: ドロップダウンメニューが切り取られる */
.site-header { overflow: hidden; }

/* ✅ 正解: デフォルトの visible を維持。はみ出し対策は個別要素で */
.user-name {
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
```

関連: R16-2（ヘッダーユーザーメニューの Alpine.js ドロップダウン許可）

---

## TL14: 初期表示（クエリパラメータなし）ではバリデーションをスキップする 【WARNING】

**問題:** 検索条件を required で設定している一覧画面で、URL 直アクセス時に**まだ何も検索していないのにエラーメッセージが表示**されると、ユーザーに「操作ミスをした」錯覚を与える。

**ルール:** 検索フォームの Handler で、**クエリパラメータが1つもない場合は「初期表示」と判定してバリデーションをスキップ**し、空の一覧を返す。`?device_id=` のようにキーだけあって値が空のケースも初期表示とみなす。クエリパラメータに実値がある場合（ユーザーが検索ボタンを押した、または URL 直指定で条件付きアクセスした）は従来通りバリデーションを実行。

```go
// Gin Handler
func (h *ReadingHandler) Index(c *gin.Context) {
    // クエリが皆無、または必須検索条件(例: device_id)が空文字の場合を初期表示とみなす
    isInitialAccess := len(c.Request.URL.Query()) == 0 || c.Query("device_id") == ""

    if !isInitialAccess {
        if err := validateSearch(c); err != nil {
            // エラー表示（TL11 パターン）
            // ...
            return
        }
    }
    // 初期表示は空一覧、検索時は結果を返す
    // ...
}
```

**モック側で注意すること:**
- モック HTML で**初期表示状態と検索結果ありの状態**を別々に用意する
- 初期表示モックには**エラーバナーを出さない**（「該当するデータはありません」等の空メッセージは可、R12 参照）
- 検索結果ありのモックには実データサンプル（R11 参照）

**適用範囲:** 検索条件に必須項目を課している一覧画面（例: センサーデータ履歴の期間フィルタ、アラート履歴のフィルタ）。

関連: R12（空メッセージ）、TL11（エラー表示の配置）、HTMX実装ガイド(動的).md §7

---

## 本プロジェクトで不採用とした別プロジェクト固有ルール

本ルール集は、元となった Laravel + Blade 前提のルール集（BL01〜BL40）を本プロジェクト（Go + Gin + templ + 農業IoT ドメイン）向けに再編したものである。以下は**本プロジェクトに該当しないため不採用**とした（旧 BL ルールから削除）。将来該当する画面・機能を追加する場合は、HTMX実装ガイド(動的).md の対応節を参照すること。

- **Blade 固有のクラス体系**（`mf-*` モーダルフォーム、`work-info-panel-*` 右カラムパネル、`ledger-list-container` 一覧コンテナ、`detail-columns`/`footer-bar-right` 2カラム編集、`accordion` コンポーネント等）— 本プロジェクトは素のモダンCSS（自前 `style.css` のトークン＋`@layer`）を使用するため不採用
- **2カラム編集画面・多段ステップフォーム**（S005型 営業→見積→結果、ステップ別 disabled 制御、ステップタブ配色）— 本プロジェクトの編集画面は1カラム（R27）のみ
- **別ドメイン固有の用語・データ**（営業案件・発注者情報・住所ブロック・担当支社・見積金額・台帳ダウンロード等）
- **Blade/HTMX 専用のモーダル基盤**（`#modal-content` 重複、`hx-push-url` 継承、`hx-on::after-swap` のオープンイベント、`view()->share('errors')`、`$dispatch` イベントリスナー等）— 本プロジェクトのモーダル/HTMX 方針は HTMX実装ガイド(動的).md §11・§16・§40 等で別途定義
- **列カスタマイズ（☰）・部分一致検索ラベル・CSS二重管理（backend ↔ local_mock）等** — 本プロジェクトに該当機能がない、または運用形態が異なる（CSSは自前 `style.css` 1本で一元管理）

---

更新日時: 2026-06-01
