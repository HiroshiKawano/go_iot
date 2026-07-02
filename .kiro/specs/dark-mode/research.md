# Gap Analysis: dark-mode

> 生成: 2026-07-02 `/kiro-validate-gap`
> 一次情報: `2cc_sdd/ダークモード調査.md`（v2.1・全項目裏取り済み）。本分析は同資料の載荷事実を実コードで**差分検証**し、要件（requirements.md 全 8 要件）→既存資産→ギャップの対応付けと実装アプローチ評価を行ったもの。

## 1. 現状調査（差分検証結果）

調査資料の主要クレームを実コードで再確認した。**全件一致**（乖離なし）。

| 検証項目 | 結果 | 根拠 |
|---|---|---|
| ダークモード実装ゼロ | ✅ 一致 | 正本 CSS に `prefers-color-scheme` / `data-theme` / `color-scheme` / `accent-color` の宣言なし（grep 0 件） |
| `:root` トークン集中定義 | ✅ 一致 | `mocks/html/style.css:60-102`。UI 基調色 9 種＋データ意味色 5 種（`--color-vpd/dewpoint/gdd/trend/heat`）＋空間/フォント/影/レイアウト |
| トークン誤用バグ 2 件 | ✅ 一致 | `.error-toast` が `color: var(--color-surface)`（L318）、`.badge-caution` が `color: var(--color-text)`（L669） |
| `#fff` 直書き（on-accent 対象） | ✅ 一致 | `.badge-good/bad`・`.badge-trend-up/down/flat` 等で `color:#fff` 直書きを確認（`.btn-*` も同様） |
| Tom Select `.ts-dropdown` 未上書き | ✅ 一致 | 既存上書きは `.ts-control` 寸法系＋`z-index` のみ（L688-706）。ドロップダウンの色は CDN 既定（白）のまま |
| `[data-echarts]` コンテナ 12 個 | ✅ 一致 | device-show 11＋analysis-trend 1（id 全列挙で確認） |
| chart pkg クローム色未指定 | ✅ 一致 | `AxisLabel/TextStyle/SplitLine/backgroundColor` の grep 0 件 → サーバ option は現状から中立 |
| `ExtendYAxis` は 1 箇所 | ✅ 一致 | `internal/chart/echarts.go:89`（乖離率時のみ yAxis 2 本化）→ patch は軸本数可変対応が必須 |
| markArea 注入色 6 種 | ✅ 一致 | vpd 3 色・condensation・gap・trendSignificant の rgba 定数を確認 |
| EChartsInitializer 構造 | ✅ 一致 | `App.templ:126-212`。`initContainer`（parse→dispose→init→endLabel/sampling/tooltip.formatter→setOption）＋ `htmx:beforeSwap`→dispose / `afterSwap`→initScope |
| Guest.templ | ✅ 一致 | ヘッダーなし・Alpine 読込済み・csrf meta なし・HTMX なし（トグルはサーバ非依存で完結する必要 → R2-5 と整合） |
| Alpine トグル手本 | ✅ 一致 | `SiteHeader.templ:9` `.nav-toggle`（`@click`/`:aria-expanded`/`aria-label`）、設置先 `.user-menu`（L12） |
| `.u-visually-hidden` 既存 | ✅ 一致 | style.css:761 定義済み・AlertRuleRow.templ で使用実績 → トグル a11y ラベルに再利用可 |
| モック HTML 10 枚 | ✅ 一致 | login/register 含む全画面分が存在 |
| CSP なし・FOUC script 可 | ✅ 一致 | App/Guest とも `<head>` 冒頭〜`<link rel="stylesheet">` 前にインラインスクリプト挿入余地あり |

**新規の観察（調査資料に明示がなかった点）**:

- `internal/view/layout/` に **layout 直下のテストファイルは現存しない**（テストは `component/*_test.go` と handler 側）。FOUC スクリプト位置・両レイアウトのトグル存在検証は**新規のレンダリングテスト**になる（既存方式 Render→Buffer→Contains は踏襲可能）。
- App.templ の `<head>` 順序は csrf meta → title → **CSS link（L38）** → tom-select CSS → JS 群。FOUC スクリプトは CSS link より前（テーマ属性設定が最優先）に置く必要がある。
- データ意味色はトークン（`--color-vpd` 等）と chart pkg 内 rgba 定数の**2 箇所**に分かれて存在する。2 層化の際、意味色トークンは「再マップしない」側に置く設計判断が要る（R6）。

## 2. 要件→資産マップ（ギャップタグ付き）

タグ: **Missing**=新規に作る必要 / **Unknown**=design で要調査・確定 / **Constraint**=既存アーキテクチャからの制約

| 要件 | 既存資産（再利用可能） | ギャップ |
|---|---|---|
| **R1 初期決定・FOUC なし** | `:root` トークン集中定義・`@layer base`・CSP なし（インライン script 可）・両レイアウトの `<head>` 構造 | **Missing**: トークン 2 層化＋ダークブロック（`@media` と `[data-theme]` の両導線）／FOUC 極小スクリプト（App・Guest 両方、`<link>` 前）／保存値破損時フォールバック。**Unknown**: OS 設定ライブ変更（R1-2）のグラフ連動配線（matchMedia change → themechange の合流方式） |
| **R2 手動トグル・永続化** | Alpine 両レイアウト読込済み・`.nav-toggle` の Alpine 用法手本・`.user-menu` 設置先・`.u-visually-hidden` | **Missing**: 共通 `ThemeToggle.templ`（component 新設）／localStorage 永続＋`html[data-theme]` 更新＋`themechange` 発火／Guest への設置。**Unknown**: Guest 配置（`.guest-layout` 右上 fixed か `.guest-card` 内上部か）。**Constraint**: Guest は HTMX/csrf なし → トグルは完全クライアント完結（サーバ送信なし＝R2-5 と整合） |
| **R3 全 UI 判読性** | 大半の部品が `var(--color-*)` 参照（トークン差し替えで追従）・`.ts-control` 上書き実績 | **Missing**: セマンティック層のダーク値／生色掃討（調査資料 §3 の表＝黄バナー・プレースホルダ縞・十字線・select 矢印 SVG・影）／`.ts-dropdown` 系ダーク上書き／`color-scheme: dark`＋`accent-color`（ネイティブ UI 一括対応）。**Unknown**: パレット最終値の AA 実測 |
| **R4 コントラスト不具合修正** | 該当 2 箇所の位置・原因特定済み（L318/L669） | **Missing**: `--color-on-accent`／`--color-on-warning` トークン新設＋該当置換＋`#fff` 直書き（btn/badge/badge-trend/pagination）の寄せ |
| **R5 グラフ判読性・状態保持** | EChartsInitializer の`initContainer`（setOption 前に介入可能な単一点）／`beforeSwap`→dispose・`afterSwap`→init のライフサイクル／サーバ option が現状クローム中立（好条件）／`tooltip.formatter` 既存上書き | **Missing**: クローム patch 汎用ヘルパ（xAxis/yAxis 本数可変・visualMap・dataZoom・凡例対応）＋ ①initContainer 時マージ ②`themechange` 時 `setOption(patch)` 再適用。**Constraint**: dispose→init 禁止（ズーム位置・凡例状態保持＝R5-2）／formatter は触らず色のみマージ。**Unknown**: markArea 帯・正常帯の視認性（据置スモーク→不足時のみ単一値調整）・ツールチップ暗地化の要否 |
| **R6 意味色の両テーマ一貫性** | 意味色はトークン 5 種＋chart 内 rgba 定数に分離済み | **Constraint**: 2 層化の際に意味色を「再マップしない」層に置く／chart 定数は非変更。実機スモークで向き確認（テストでは捕捉不可＝運用期待） |
| **R7 無回帰** | ライト値は現行トークン値そのまま | **Constraint**: 2 層化は**視覚差ゼロのリファクタ**であること（パレット層へ値を移すだけ）。既存 component/handler テスト全緑維持 |
| **R8 モック整合** | モック 10 枚＋正本 style.css＋`make sync-css` 一方向同期 | **Missing**: 全モック（特に login/register）へのトグル要素反映・ダークブロックの正本反映。**Constraint**: グラフ動的描画はモック例外（器のみ） |

**複雑性シグナル**: DB・エンドポイント・スキーマ影響ゼロ（永続データ項目なし）。難所は (a) ECharts クローム patch（Go テスト外＝実機/E2E 検証のみ）、(b) パレットの AA 実測、(c) 生色掃討の網羅性（調査資料 §3 が作業リストとして完備）。

## 3. 実装アプローチ選択肢

### Option A: 全て既存ファイル内で拡張

style.css にダークブロック追記、FOUC＋トグルロジックを App/Guest 各レイアウトへ直接インライン記述、patch を EChartsInitializer IIFE 内に追記。

- ✅ 新規ファイル最小・配線変更なし
- ❌ トグルのマークアップ＋Alpine ロジックが App/Guest に**二重記述**（保守点 2 箇所・乖離リスク）
- ❌ requirements の「共通コンポーネント化」示唆（調査資料 §4-4 確定事項）に反する

### Option B: テーマ機構を新規モジュールに分離

`ThemeToggle.templ` 新設＋テーマ JS を独立アセット（`public/js/theme.js` 等）として切り出し。

- ✅ 責務分離が最も明瞭・テーマ JS の単体管理
- ❌ 本プロジェクトの script 運用は「templ 内インライン」が確立パターン（EChartsInitializer 等）で、**独立 JS アセットは前例なし**（配信配線・キャッシュバスティングの追加設計が必要）
- ❌ FOUC スクリプトはどのみち `<head>` インライン必須で、分離してもファイルが増えるだけ

### Option C: ハイブリッド（調査資料 §4/§7 の想定と一致）★推奨

- **新設**: `ThemeToggle.templ`（component 配下・Alpine ロジック内包・App/Guest で共有）
- **拡張**: 正本 style.css（2 層トークン＋ダークブロック＋生色掃討＋ts-dropdown）／App・Guest の `<head>`（FOUC 極小インライン script）／EChartsInitializer（クローム patch ヘルパ＋themechange 購読）／SiteHeader（`.user-menu` へトグル設置）／モック 10 枚
- ✅ 既存パターン（component 分割・インライン script・CSS 単一ソース）に全て整合
- ✅ トグルの保守点が 1 箇所・レイアウト変更は最小差分
- ❌ 変更ファイル数は最多（ただし各差分は小さい）

## 4. 工数・リスク評価

- **工数: M（3〜7 日）** — 変更は「広く浅い」。CSS 掃討（§3 表の全行処理）＋トグル/FOUC＋ECharts patch＋モック 10 枚反映＋テスト＋両テーマ×全画面スモークの合算。
- **リスク: Medium** — 根拠: (a) ECharts クローム patch は Go テスト外（実機/E2E のみ）で軸本数可変・2 タイミング適用の考慮点が多い、(b) パレット AA 実測が未実施（design で確定）、(c) それ以外は確立パターンの延長で Low 相当。アーキテクチャ変更・未知技術なし。

## 5. design フェーズへの推奨事項

**推奨アプローチ**: Option C（ハイブリッド）。調査資料 §4（方式 C）・§7（作業ステップ）をそのまま設計骨格に使える。

**design で確定すべき事項（Research Needed）**:

1. **ダークパレット最終値** — 調査資料 §5 たたき台の AA（4.5:1）実測。組合せ: text/bg・text/surface・muted/surface・on-accent/primary・on-accent/danger・on-warning/warning・アラートバナー暗黄系。
2. **Guest トグル配置** — `.guest-layout` 右上 fixed か `.guest-card` 内上部か（モック login/register への反映形も同時確定）。
3. **OS 設定ライブ追従の配線（R1-2）** — `matchMedia('(prefers-color-scheme: dark)')` の change と手動選択の優先解決、および `themechange` への合流（グラフのクローム追従）。
4. **クローム patch の対象プロパティ表** — 軸ラベル/軸線/splitLine・legend textStyle・visualMap textStyle・dataZoom（slider は trend-chart のみ）・calendar 枠線/月ラベル・（任意）tooltip 暗地化。xAxis/yAxis 本数可変（`ExtendYAxis`・heatmap category 軸・calendar 無軸）の生成規則。
5. **影のダーク表現** — 濃影（例 `rgba(0,0,0,.4)`）か境界線表現か。
6. **select 矢印 SVG** — `mask`+`currentColor` 化の採否（対象ブラウザ互換）。
7. **markArea 帯・正常帯** — 据置スモーク後の単一値調整の判断基準。
8. **テスト設計** — layout 直下にテスト前例がない点を踏まえた FOUC script 位置・トグル存在・`aria-pressed` の検証方式（既存 Render→Buffer→Contains 踏襲）。ECharts patch は E2E/実機スモークに切り分け。
9. （軽微・任意）ECharts バージョンコメント 5.4.3→5.4.4 の表記修正。

**設計時の遵守制約（Boundary Commitments 候補）**: CSS 単一ソース（正本のみ手編集・`@layer base` でトークン再宣言）／意味色トークン・chart 定数は非変更／dispose→init 禁止（状態保持）／サーバ option 中立維持／モック先行反映（structure.md §31）。

---

# Design Discovery & Decisions: dark-mode（設計フェーズ）

> 生成: 2026-07-02 `/kiro-spec-design`。上記ギャップ分析の続き。Discovery 種別: **Extension（light discovery）**。

## Summary

- **Feature**: `dark-mode`
- **Discovery Scope**: Extension（既存全画面への横断 UI 拡張・新規依存ライブラリなし）
- **Key Findings**:
  - **AA コントラスト実測により調査資料 §5 の「彩色トークン微明化」案を棄却**。primary を `#2fbe54` に明るくすると白文字ボタンが 2.44:1（ライト現状 3.13:1 より悪化）。**primary/primary-dark/danger/warning はライト値のまま両テーマ据置**が最適解（下記 Decision 参照）。
  - 再マップ対象のセマンティックトークンは **bg / surface / text / muted / border / shadow-sm / shadow-md の 7 種＋表示切替ユーティリティ 2 種**に縮小。基調 4 色（text 13.23:1・muted 6.26:1 等）は §5 たたき台のまま AA 合格。
  - Guest トグル配置は `.guest-layout` 右上 **fixed** に決定（カードのフォーム構造を崩さず、App ヘッダー右上のトグル位置と視覚的に一貫）。

## Research Log

### AA コントラスト実測（WCAG 相対輝度・自前計算）

- **Context**: 要件 3.2（本文 AA 4.5:1 目安）・4.1/4.2（エラートースト/注意バッジ判読）・7.1（ライト無回帰）。調査資料 §5 は「数値未計測」と明記。
- **Sources Consulted**: WCAG 2.x 相対輝度式で全組合せを計算（python）。
- **Findings**（抜粋・比率は計算値）:
  - ダーク基調: text `#e4e6eb`/bg `#121417`=**14.78**・text/surface `#1c1f24`=**13.23**・muted `#9aa0a6`/surface=**6.26**・banner `#ffd43b`/`#3a2f00`=**9.29** → 全て AA 合格（たたき台のまま採用可）。
  - 彩色微明化案の劣化: 白文字 on `#2fbe54`=**2.44**（✗・ライト現状 on `#28a745`=3.13 より悪化）／白文字 on `#e5484d`=**3.91**（ライト現状 on `#dc3545`=**4.53 AA** から降格）。
  - 彩色据置案の成立: `#28a745` on ダーク surface=**5.27 AA**・`#dc3545` on surface=**3.65**（非テキスト 3:1 合格）・`#ffc107` on surface=**10.14**・白文字 on hover `#218838`=**4.52 AA**。
  - パリティ確認: border `#2e3238`/surface=1.28（ライト現状 `#dedfe3`/白=1.33 と同水準＝装飾境界の設計どおり）・プレースホルダ縞 1.11（ライトも同様の微差縞＝据置妥当）。
- **Implications**: ダークブロックの再マップは基調系のみでよく、彩色系はパレット層（テーマ不変）に置ける。error-toast は据置により両テーマ AA を満たす。

### HTMX 実装ガイド該当節（§31 系・§40-B）

- **Context**: トグル UI という新規要素・新規クラスの追加が単一ソース規約に抵触しないか。
- **Sources Consulted**: `2cc_sdd/HTMX実装ガイド(動的).md` §31（クラス体系統一）・§31.2（器の変更はモック正本へ先行反映）・§40-B（CSS 単一ソース運用・本番ファイル直接編集禁止）。
- **Findings**: 器（要素）の追加は「モック HTML を先に書き換え → templ を写経」の順。CSS 追記は正本 `mocks/html/style.css` の `@layer components`（部品）/`@layer base`（トークン）へ。CSS 変更があるため `make sync-css` 必須。
- **Implications**: タスク順序は「正本 CSS＋モック反映 → templ 写経」が正。ThemeToggle の新クラス `.theme-toggle` はモック先行で正当化される。

### テストガイダンス該当節（Testing Strategy の根拠）

- **Context**: templ レンダリング検証と ECharts patch 検証の切り分け。
- **Sources Consulted**: `2cc_sdd/テストガイダンス集.md` Go テーマ別索引 → §4（templ Render→Buffer→Contains）・§56.1（クライアント供給 ECharts option JSON の構造アサート）・§5.6.AA（ECharts 実機スモーク定石・markArea/ドメイン意味の実機確認）・§5.6.AB（フレッシュバイナリ・別ポート起動・data-* マーカー生存法）・§61.4（HTMX swap 後の再 init/connect・凡例トグルのライブ内省）。
- **Findings**: Go テストは templ 出力の文字列アサートで FOUC script 位置・トグル存在・aria 属性を固定できる。クロームpatch・状態保持・OS 追従は chrome-devtools 実機スモーク（§5.6.AA の定石＝再ビルド・別ポート・emulate）で検証する。
- **Implications**: Testing Strategy を「Go テスト（templ/CSS 機械検査）」と「実機スモーク（patch/FOUC/状態保持）」の 2 層に分割。

### DB スキーマ現状（形式確認）

- **Context**: design が扱うデータ項目の実在確認（プロジェクト必須手順）。
- **Sources Consulted**: `docs/database_snapshot/table_definitions.md`（7 テーブル）。
- **Findings**: 本機能の永続データは localStorage の `theme` キーのみで、**DB テーブル・カラムには一切非接触**（製品判断①）。
- **Implications**: Data Models は State Management（クライアント側）のみ記述。migration なし。

## Architecture Pattern Evaluation

| Option | Description | Strengths | Risks / Limitations | Notes |
|--------|-------------|-----------|---------------------|-------|
| 2 層トークン＋両導線ダークブロック（採用） | パレット層（テーマ不変値）＋セマンティック層（役割名）。ダークは `@media(prefers-color-scheme:dark){:root:not([data-theme=light])}` と `:root[data-theme=dark]` の 2 ブロックからセマンティック層のみ再マップ | 値の単一ソース化・手動上書きと OS 追従の両立・全ブラウザで安全 | 再マップ行が 2 ブロックに重複（値はパレット参照で一元） | spec-init 本文で確定済みの方式 |
| CSS `light-dark()` 関数 | `color-scheme` 連動で 1 行 2 値宣言 | 重複ゼロ・最新のベストプラクティス | Baseline 2024（Chrome123+/Safari17.5+/FF120+）。非対応ブラウザで**プロパティ無効＝配色全損** | 棄却: 互換リスクが利得に見合わず、確定方式からの逸脱 |
| ECharts `registerTheme`/組込 dark | テーマ機構で一括着色 | 公式機構 | dispose→re-init が必要になり dataZoom/凡例状態が飛ぶ（R5-2 違反）・変更面が拡大 | 調査 v2 で棄却済み（クローム patch 方式を採用） |

## Design Decisions

### Decision: 彩色トークン（primary/primary-dark/danger/warning）は両テーマ据置

- **Context**: 調査資料 §5 たたき台は primary `#2fbe54`・danger `#e5484d`・warning `#f0b429` への微明化を提案していた。
- **Alternatives Considered**:
  1. たたき台どおり微明化 — ダーク面上の彩色文字は読みやすくなる
  2. ライト値のまま据置 — 彩色地＋白文字の現状コントラストを両テーマで維持
- **Selected Approach**: 据置（2）。再マップはセマンティック基調系 7 トークン＋表示切替 2 トークンのみ。
- **Rationale**: 実測で微明化は白文字ボタン 2.44:1（現状 3.13 より悪化）・エラートースト AA 喪失（4.53→3.91）。据置なら彩色地上の文字は両テーマ完全同値＝R4/R7 を同時充足し、ダーク面上の彩色も primary 5.27 AA・warning 10.14 AA・danger 3.65（非テキスト3:1）で成立。
- **Trade-offs**: danger をダーク面上の**本文級テキスト**に使う箇所があれば 4.5 未達だが、danger の用途は彩色地・バッジ・境界であり本文用途はない（grep 確認済みの用途一覧に基づく）。
- **Follow-up**: 実機スモークで danger 系の視認を目視確認（不足時のみ danger の前景用途に限り明化検討）。

### Decision: トグルのアイコン切替は CSS 表示トークンで行う（Alpine 非依存）

- **Context**: モック HTML は静的（Alpine なし）だが R8-1 でトグル要素の両テーマ表示確認が必要。
- **Alternatives Considered**: 1. Alpine `x-text` でアイコン切替（モックでは切替不能） 2. 月/日 2 スパン＋`--display-when-dark/light` トークンで CSS 切替
- **Selected Approach**: 2。`:root` に `--display-when-dark: none; --display-when-light: inline` を定義しダークブロックで反転。アイコン span がこれを `display` に参照。
- **Rationale**: モック直開き（JS なし・OS 追従）でもアイコンがテーマに追従し、実装とモックの見た目が一致（単一ソース規約）。Alpine は `aria-pressed` と永続化のみ担当に縮小。
- **Trade-offs**: 再マップ行が 2 行増える（軽微）。
- **Follow-up**: なし。

### Decision: Guest トグルは `.guest-layout` 右上 fixed

- **Context**: Guest レイアウトにヘッダーがなく独自配置が必要（要確認事項）。
- **Alternatives Considered**: 1. `.guest-card` 内上部（カードにヘッダ行を追加） 2. `.guest-layout` 右上 fixed
- **Selected Approach**: 2。ラッパクラス `.guest-theme-toggle`（`position: fixed; top/right: var(--space-4)`）で設置。
- **Rationale**: ログインカードのフォーム構造（見出し→フォーム→footer）を崩さない。App のトグル位置（ヘッダー右上 `.user-menu` 内）と視覚位置が一貫。モック反映も 2 ファイルへの追記のみ。
- **Trade-offs**: 超小型画面でカードと重なる可能性 → viewport 上端固定でカードは中央のため実害なし（スモークで確認）。
- **Follow-up**: login/register モックで狭幅表示を目視。

### Decision: クローム patch は「常時明示値」方式（ライト値=ECharts 既定同値）

- **Context**: `setOption` マージでは色の「解除」ができないため、ダーク→ライト復帰時に明示値が必要。
- **Selected Approach**: `buildChromePatch(option, isDark)` が両テーマとも明示色を返す。ライト値は ECharts 5.4.4 既定と同値を指定し視覚無変化を保証（パリティは実機スモークで確認）。適用は ①initContainer の setOption 前マージ ②`themechange` で既存インスタンスへ `setOption(patch)`。
- **Rationale**: dispose 禁止（状態保持）の下で往復切替を成立させる唯一の単純解。
- **Trade-offs**: ECharts 既定値をリテラルで持つ（バージョン更新時に追従確認が要る）→ 既定値表をコメントで根拠明記。
- **Follow-up**: ライトテーマの目視パリティ（R7-1）をスモーク項目に含める。

### Decision: OS 設定ライブ変更の追従は App の patch ブロックが所有

- **Context**: R1-2（未選択時の OS 変更追従）。CSS は `@media` で自動追従するが、ECharts クロームは JS 通知が必要。
- **Selected Approach**: App.templ の patch ブロックに `matchMedia('(prefers-color-scheme: dark)')` の change リスナを置き、`data-theme` 未設定（=OS 追従中）のときのみ `themechange` を dispatch。ThemeToggle は手動切替時に同イベントを dispatch。
- **Rationale**: グラフは App 配下にしか存在せず、Guest には購読者不要。テーマ解決の唯一の関数 `isDarkTheme()`（`data-theme` 優先→matchMedia フォールバック）を patch ブロックに集約。
- **Trade-offs**: なし（Guest は CSS のみで完結）。

## Risks & Mitigations

- ECharts 既定値リテラルの誤り（ライト視覚差）— 実機スモークのライトパリティ確認項目で捕捉。乖離時は既定値表を実測値へ修正。
- markArea 帯（rgba 10〜12%）のダーク視認不足 — 据置スモーク→不足時のみ「両テーマ成立の単一不透明度」へ微調整（テーマ別分岐は最終手段・要件 5.4）。
- `color-scheme: dark` による UA 由来の予期せぬ着色（フォーム・スクロールバー）— 全画面スモークで目視。問題部位のみ個別トークン上書き。
- モック 10 枚への要素追記漏れ — CSS 機械検査テスト＋モック目視をタスク完了条件に明記。

## References

- `2cc_sdd/ダークモード調査.md` v2.1 — 一次情報（§2 トークン・§3 生色・§5 パレット・§6 ECharts・§9 製品判断）
- `2cc_sdd/HTMX実装ガイド(動的).md` §31/§31.2/§40-B — クラス体系・モック先行反映・CSS 単一ソース
- `2cc_sdd/テストガイダンス集.md` §4・§56.1・§5.6.AA/AB・§61.4 — templ テスト定石・ECharts 実機スモーク定石
- WCAG 2.x 1.4.3（AA 4.5:1）/1.4.11（非テキスト 3:1）— コントラスト判定基準
