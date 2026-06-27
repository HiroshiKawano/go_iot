# Requirements Document

## Project Description (Input)

認証後レイアウトの左サイドバーを「文脈に応じて変わる動的ナビゲーション」に進化させる機能（device-context-nav）。

### 誰の・どんな課題か
自分のデバイスを運用する利用者（眞境名さん／テストユーザー）。あるデバイスの**詳細画面**と、その**センサーデータ履歴**を行き来したいが、現状サイドバーには両者への直接リンクが無く、毎回ダッシュボードへ戻る・「もっと見る」リンクを辿る・ブラウザの戻る、に頼っている。選択中デバイスの「詳細↔履歴」の往復が手間で、現在どの画面に居るかもサイドバー上で分からない（active がダッシュボード固定）。

### 現状
- サイドバーは `internal/view/component/Sidebar.templ` で**引数なしの固定HTML**。リンクは 🏠ダッシュボード / 🔔アラートルール / 🕐アラート履歴 の3つで、`active` クラスはダッシュボードにハードコード。
- Sidebar.templ のコメントに「現在ページのハイライト(active)は後続セッションで動的化する想定」と明記されており、本機能はその予定されていた動的化を回収する。
- サイドバーは認証後レイアウト `layout.App` が `@component.Sidebar()` で**全7画面**（dashboard / device-show / readings / device-create / device-edit / alert-rules / alert-history）に描画し、各**モック正本** `mocks/html/*.html`（全7ファイル）にも同一構造が存在する。

### 何を変えるか（提案する振る舞い）
左サイドメニューを文脈連動の動的ナビにする：
- 🏠 **ダッシュボード**（常時表示・ダッシュボードで active）。ダッシュボードにはセンサー一覧があり、センサーを選択するとそのデバイス詳細へ移動する。
- 📟 **デバイス詳細**（**デバイス選択中のみ登場**・`/devices/{id}` へリンク・デバイス詳細画面で active）。
- 📈 **センサーデータ履歴**（**デバイス選択中のみ登場**・`/devices/{id}/readings` へリンク・履歴画面で active）。
- 🔔 **アラートルール**（常時表示）。
- 🕐 **アラート履歴**（常時表示）。

導線: ダッシュボード（センサー一覧）→ センサー選択 → デバイス詳細（サイドに「センサーデータ履歴」が登場）→ 履歴（サイドに「デバイス詳細」が登場）で、**選択中デバイスの詳細↔履歴を相互に往復**できる。デバイス文脈に居ないページ（dashboard / alerts）ではデバイス詳細・センサーデータ履歴の2リンクは**非表示**。

### スコープ（確定済みのユーザー判断）
本フェーズの範囲＝「文脈リンク（デバイス詳細／センサーデータ履歴）の追加」＋「**全メニュー項目の active ハイライトを現在ページ連動で動的化**」の両方を一括で行う（Sidebar.templ が予告していた active 動的化もここで回収）。

### 技術的所在（現状コードの参照点・実現方式は design で確定）
- 共有レイアウトデータ `layout.AppLayoutData`（`internal/view/layout/App.templ` 内に定義・Title/UserName/CSRFToken/CSSURL/Flash）に、**現在のナビ文脈**（現在デバイスID の有無＋現在ページ識別子＝active 判定用）を渡すフィールド追加が要る見込み。
- `Sidebar()`：引数なし → ナビ文脈を受け取る形へ。文脈リンクの条件描画＋active の動的付与。独自CSSクラスは新設しない（既存 `.sidebar`/`.active`/`nav li` を流用）。
- App を描画する全ハンドラ（dashboard / device-show / readings / device-create / device-edit / alert-rules / alert-history）がナビ文脈を埋める。大半はデバイス文脈なし（2リンク非表示）。`device-show`（`DeviceHandler.Show`）と `readings`（`ReadingsHandler.Index`）が選択中デバイスID を設定する。
- **モック正本**：`mocks/html/*.html` 全7ファイルのサイドバーを、その画面のナビ状態（active 位置・device 文脈の2リンク有無）に合わせて更新（templ 写経の正本・デグレ防止）。CSS 追加が要る場合のみ `mocks/html/style.css` の `@layer components` へ最小追記し `make sync-css`。

### 境界（Out of Boundary 候補・design で確定）
- 各画面のコンテンツ本体（グラフ／表／フォーム）は変更しない（サイドバーのナビ構造のみ）。
- 新規ルート・新規クエリ・スキーマ変更は伴わない（既存ルート `/devices/{id}`・`/devices/{id}/readings`・`/dashboard`・`/alerts/*` を消費するのみ）。
- モバイルドロワー開閉（Alpine `navOpen`）の挙動は現状維持。
- 所有者認可・各画面の既存振る舞いは消費のみ・無回帰維持。

### 非機能・整合
読み取り側 UI のみ。全7画面で無回帰（既存リンク先・レイアウト・HTMX 部分更新を壊さない）。テストは layout/component の templ Render テスト（render→strings.Contains）と、device-show/readings ハンドラがナビ文脈を埋めることの httptest を想定。

## Requirements
<!-- Will be generated in /kiro-spec-requirements phase -->
