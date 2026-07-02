# 拡張スペック E2 spec-init プロンプト: システム全体ダークモード（テーマ切替）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: dark-mode
> 位置づけ: [実装計画.md](../実装計画.md) 拡張スペック **E2**。S（画面分割）/P（分析ロードマップ）系列とは独立の、**全画面横断の UI/UX 拡張**（新規画面なし・スキーマ非変更・Go サーバコード原則非変更）。
> 前提セッション: S1（Guest/App レイアウト・SiteHeader・Alpine 配線）／ E1（ECharts 移行・EChartsInitializer）／ 全 9 画面＋login/register 実装済み
> **製品判断（2026-07-02 ユーザー確定・変更不可の前提）**: ①保存先=**localStorage のみ**（DB 保存しない） ②初回既定=**OS 追従**（`prefers-color-scheme`） ③**Guest 画面（login/register）にもトグルを置く**
> 設計フェーズで参照:
> - **調査資料（権威・全項目裏取り済み）**: [ダークモード調査.md](../ダークモード調査.md) — 生色インベントリ（§3）・トークン誤用バグ 2 件（§3 後半）・提案パレット（§5）・ECharts クローム対応方式（§6）・検証済みエビデンス一覧（付録）。**requirements〜design は本資料を一次情報として使い、再調査は差分のみでよい。**
> - CSS 正本: mocks/html/style.css（`@layer reset<base<components<utilities`・`:root` トークン集中定義・770 行）。本番 internal/view/public/css/style.css は `make sync-css` 生成物（手編集禁止・gitignore）。
> - レイアウト: internal/view/layout/App.templ（EChartsInitializer・tom-select CDN・CSP なし確認済み）／ Guest.templ（ヘッダーなし）／ internal/view/component/SiteHeader.templ（`.user-menu`・`.nav-toggle` の Alpine 手本）
> - グラフ: internal/chart/*_echarts.go（option 構築・クローム色は全チャート未指定）／ `[data-echarts]` コンテナ 12 個（device-show 11＋analysis-trend 1）
> - 規約: .kiro/steering/structure.md（CSS 単一ソース §31・独自クラスはモック先行反映）／ CLAUDE.md／ 物理規約 project_vpd_physics_convention（暖=暑/乾・寒=湿はダークでも不変）

--- spec-init 本文 ここから ---

## 機能概要

現在システム全体がライト（白背景）一色のところへ、**ダークモード**を追加する。方式は「既定 OS 追従＋手動トグルで上書き（`<html data-theme>`）＋localStorage 永続」の両立型。全 9 認証後画面＋login/register の全 UI（カード・フォーム・テーブル・バッジ・モーダル・Tom Select・ネイティブコントロール）と、**ECharts グラフ 12 コンテナのクローム（軸・凡例・ツールチップ・visualMap・dataZoom）**を両テーマで判読可能にする。色は既存の CSS カスタムプロパティ（`:root` トークン）を 2 層化（原色パレット層＋セマンティック層）して差し替え、トークンが届かない領域（CSS 内生色・CDN の Tom Select・ネイティブ UI・ECharts）を個別に対応する。**データ意味色（温度橙/湿度青/VPD緑/露点青/GDD赤/トレンド紫/暑熱ピンク）は両テーマ据置**。付随して、ダーク値投入で顕在化する**既存のトークン誤用バグ 2 件**（`.error-toast`／`.badge-caution` の文字色）を同時修正する。

## 背景・現状

- **既存ダークモード実装はゼロ**（`prefers-color-scheme`/`data-theme`/トグル UI の痕跡なし・全 grep 確認済み）。
- **追い風**: 色は `:root` トークン集中定義で、templ 内インライン色ゼロ。トークン差し替えで大半が切り替わる。Alpine.js は両レイアウト読込済み。CSP 未設定で `<head>` インラインスクリプト可。
- **障害 4 系統**（詳細は調査資料 §0/§3/§6）:
  1. CSS 内の生色（黄バナー `#fff3cd`/`#856404`・プレースホルダ縞・グラフ十字線 `#495057`・select 矢印 SVG・影）
  2. ECharts クロームが既定色（暗テキスト）依存 → ダーク背景で軸・凡例・visualMap ラベルが判読不能
  3. トークンが届かない領域: CDN の tom-select.css（`.ts-dropdown` が白ハードコード）・ネイティブ UI（date ピッカー×5・checkbox×4・radio×2 ほか）
  4. トークン誤用 2 件: `.error-toast` が `color: var(--color-surface)`、`.badge-caution` が `color: var(--color-text)` — ダーク値投入で赤地に暗文字／黄地に明文字となりコントラスト崩壊
- **運用制約**: CSS 単一ソース（正本=mocks/html/style.css → `make sync-css`）。トグル要素・新クラス・ダークブロックは**モック HTML と正本 CSS の両方へ反映必須**（モックが正本）。グラフの動的描画はモック反映例外（器のみ反映）。

## このセッションのスコープ（実装対象）

### CSS トークン 2 層化＋ダークブロック（正本 style.css・モック同時）
- `:root` を原色パレット層（`--palette-*`・テーマ不変）とセマンティック層（`--color-*`・役割名）に分離。ダーク値は `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) }` と `:root[data-theme="dark"]` の両導線からセマンティック層のみ再マップ（重複記述を避ける）。
- ダーク適用時に `color-scheme: dark` を宣言（date ピッカー・checkbox・スクロールバー等ネイティブ UI の一括ダーク化）。併せて `accent-color: var(--color-primary)` を両テーマで指定。
- 新設トークン: `--color-on-accent`（彩色地上の文字・両テーマ `#fff`）・`--color-on-warning`（warning 地上の文字・両テーマ `#212529`）。
- パレット初期値は調査資料 §5 のたたき台（bg `#121417`/surface `#1c1f24`/text `#e4e6eb` 等）。**AA コントラスト実測のうえ design で確定**。

### トークン誤用バグ修正（ダーク値投入より先に実施）
- `.error-toast`: `color: var(--color-surface)` → `var(--color-on-accent)`。
- `.badge-caution`: `color: var(--color-text)` → `var(--color-on-warning)`。
- `.btn-*`/`.badge-*`/`.pagination .current` 等の `#fff` 直書きも `--color-on-accent` へ寄せる。

### 生色の掃討（調査資料 §3 の表を作業リストとして全行処理）
- 要対応: `.alert-banner`（暗黄系へ）・`.chart-placeholder` 縞・`.ch-vline/.ch-hline`（明色へ）・select 矢印 SVG（`mask`+`currentColor` 化推奨）・`--shadow-sm/md`（ダークは濃い影 or 境界線）。
- 据置と判断済み: `.nav-backdrop`・`.modal-overlay`・`.heat-scale-bar` グラデ（データ意味色）・彩色地上の白文字。

### Tom Select ダーク上書き
- 正本 CSS に `.ts-dropdown` 系（本体・候補行・`.option.active`/`.selected`）のダーク上書きを追加。CDN ファイルは触らない。既存 `.ts-control` 上書き（トークン参照）はそのまま生きる。

### FOUC 対策＋トグル UI（App＋Guest 両方）
- 両レイアウトの `<head>`・`<link>` **前**に極小インラインスクリプト（`localStorage.theme` → `documentElement.dataset.theme` 即設定）。
- 共通コンポーネント `ThemeToggle.templ` を新設し、App は `SiteHeader.templ` の `.user-menu` 内・**Guest は独自配置（`.guest-layout` 右上 fixed か `.guest-card` 内上部・design で確定）**に設置。Alpine で `localStorage` 永続＋`html[data-theme]` 更新＋`CustomEvent('themechange')` 発火。a11y: `aria-pressed`＋`.u-visually-hidden` ラベル。
- OS ダーク時にライト固定へ戻せること（`data-theme="light"` の明示上書き）。

### ECharts クロームのテーマ patch（クライアント側・App.templ の EChartsInitializer 拡張）
- **サーバ option はテーマ非依存の中立を維持**（系列色のみ・internal/chart は原則非変更）。`registerTheme`/組込 dark テーマは不使用。
- 現在テーマのクローム色（軸ラベル/軸線/splitLine・凡例 textStyle・visualMap textStyle・dataZoom・任意でツールチップ暗地化）を option へマージする**汎用ヘルパ**を追加。**既存 option の xAxis/yAxis の本数を読んで同数分生成**する（乖離率時のみ yAxis 2 本＝`ExtendYAxis`・heatmap の category 軸・calendar の無軸を同一ヘルパで吸収）。
- 適用は **2 タイミング**: ①initContainer 時（setOption 前にマージ＝ダーク中の HTMX 期間切替 swap でも正しく着色）②`themechange` 時（既存全インスタンスへ `setOption(patch)` 再適用。**dispose→init はしない**＝dataZoom ズーム位置・凡例トグル状態を保持）。
- 既存の `tooltip.formatter` 上書き（App.templ）には触れない（色プロパティのみマージ）。
- markArea 注入色 6 種（VPD 3 帯/結露帯/欠測帯/有意区間帯・rgba 10〜12%）と正常帯（Opacity 0.15）は**据置のまま実機スモーク**し、不足時のみ「両テーマで成立する単一値」へ微調整（テーマ別分岐は最終手段）。

### モック反映（単一ソース規約）
- トグル要素を全モック HTML（特に login.html/register.html）へ、ダークブロック・新クラスを正本 style.css へ反映。`make sync-css` で本番へ同期。

## スコープ外（このセッションでやらないこと）

- **テーマの DB 保存・ユーザー設定画面**（製品判断①で localStorage 確定。将来の DB 昇格は非破壊で可能）。
- ECharts の `registerTheme`/組込 dark テーマ採用・サーバ側 option へのテーマ別色分岐（クローム patch 方式で統一）。
- **データ意味色（系列色・暑熱スケール・バッジ信号色）のテーマ別変更**（両テーマ据置。沈む色の微調整は実機スモーク後の個別判断）。
- tom-select CSS の self-host 化・CSP ヘッダ導入・Alpine/htmx のバージョン変更。
- 新規エンドポイント・スキーマ変更・マイグレーション（一切なし）。
- グラフの業務要件・URL・期間切替 UX の変更（無回帰）。

## 技術制約・準拠事項

- **CSS 単一ソース**: 正本=mocks/html/style.css のみ手編集。`make sync-css` → go:embed 配信（`?v=Version` でキャッシュバスティング実効・確認済み）。`@layer` 構造（reset<base<components<utilities）を維持し、ダークブロックは base 層のトークン再宣言で行う。
- **独自クラス新設はモック先行反映が条件**（structure.md §31）。トグルの新クラス・ダーク差分はモックへ先行反映。
- **templ**: ThemeToggle は component 配下・既存 Alpine 用法（`.nav-toggle` 手本）踏襲。FOUC スクリプトは `<link>` より前。
- **物理規約**: 暖色=暑/乾・寒色=湿の向きはダークでも不変（project_vpd_physics_convention）。**テストは仕様前提を符号化するため向きの崩れを捕捉できない → 実機スモーク必須**。
- **言語**: 日本語コメント・エラー・コミット。
- **TDD 80% 以上**: templ レンダリング（トグル要素の存在・`aria-pressed`・FOUC スクリプトの位置・App/Guest 両方）を既存 component/handler テスト方式で。ECharts patch は Go テスト外＝E2E（Playwright）またはブラウザ実機で検証。

## 受け入れ基準（概略）

1. **OS 追従**: 未選択時、OS のライト/ダーク設定どおりに表示される（`prefers-color-scheme`）。
2. **手動トグル**: 全認証後画面＋login/register にトグルがあり、切替が即時反映・`localStorage` で再訪時も維持・OS ダーク時のライト固定も可能。
3. **FOUC なし**: ダーク保存状態での再読込時に白い画面が一瞬も出ない。
4. **UI 網羅**: 全 9 画面＋login/register の全部品（カード/フォーム/テーブル/バッジ/モーダル/ページネーション/Tom Select ドロップダウン/date ピッカー等ネイティブ UI）が両テーマで判読可能（本文 AA 目安）。
5. **トークン誤用修正**: `.error-toast`（赤地）・`.badge-caution`（黄地）の文字が両テーマで読める。
6. **グラフ**: 12 コンテナ全てで軸・凡例・visualMap・dataZoom が両テーマ判読可能。テーマ切替後も dataZoom ズーム位置・凡例トグル状態が保持される。ダーク中の期間切替（HTMX swap）でも新グラフが正しく着色される。系列色は両テーマ同一。
7. **モック同期**: モック HTML（ブラウザ直開き）でもダーク表示・トグル要素が確認でき、`make sync-css` 後の本番 CSS と一致する。
8. **無回帰**: ライトテーマの見た目が従来と同等（トークン 2 層化はリファクタであり視覚差ゼロ）。
9. **テスト 80% 以上**＋実機スモーク（両テーマ×全画面・意味色の向き確認）。

## 未確定事項・要確認（design フェーズで確定）

- ダークパレット最終値（調査資料 §5 たたき台の AA コントラスト実測・実機スモークで確定）。
- Guest トグルの配置（`.guest-layout` 右上 fixed か `.guest-card` 内上部か）。
- ツールチップの暗地化（既定の白地でも読めるため任意・やるなら patch に含める）。
- markArea 帯・正常帯の不透明度調整の要否（据置スモーク後に判断）。
- App.templ/static.go の ECharts バージョンコメント表記（5.4.3→5.4.4）の同時修正（軽微・任意）。

--- spec-init 本文 ここまで ---
