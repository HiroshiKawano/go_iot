package view

import (
	"os"
	"strings"
	"testing"
)

// css_theme_guard_test.go は正本 CSS (mocks/html/style.css) の機械検査を行う。
// 配信物 (internal/view/public/css/style.css) は make sync-css の複製先であり
// 手編集禁止のため、検査対象は常に正本を直接読む。

const canonicalCSSPath = "../../mocks/html/style.css"

func loadCanonicalCSS(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(canonicalCSSPath)
	if err != nil {
		t.Fatalf("正本CSSの読込に失敗: %v", err)
	}
	return string(b)
}

// --- 極小 CSS ブロックパーサ ---
// @layer/@media 等のネストしたルールグループを再帰的に分解し、セレクタ (または
// at-rule prelude) と本体 (宣言 or 子ブロック) を持つ木構造にする。
// 文字列リテラル内の '{'/'}' は本 CSS では使用されない (data URI に含まれる文字は
// '{'/'}' を含まない) ため、素朴な波括弧対応付けで十分。

type cssBlock struct {
	Selector string
	Body     string // 子ブロックを持たない場合は宣言テキスト、持つ場合は生の内側テキスト
	Children []cssBlock
}

func stripCSSComments(css string) string {
	var b strings.Builder
	for i := 0; i < len(css); {
		if strings.HasPrefix(css[i:], "/*") {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				break
			}
			i += 2 + end + 2
			continue
		}
		b.WriteByte(css[i])
		i++
	}
	return b.String()
}

func parseCSSBlocks(css string) []cssBlock {
	var blocks []cssBlock
	i, n := 0, len(css)
	for i < n {
		for i < n && isCSSSpace(css[i]) {
			i++
		}
		if i >= n {
			break
		}
		openRel := strings.IndexByte(css[i:], '{')
		if openRel < 0 {
			break
		}
		selector := strings.TrimSpace(css[i : i+openRel])
		bodyStart := i + openRel + 1
		depth := 1
		j := bodyStart
		for j < n && depth > 0 {
			switch css[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			j++
		}
		bodyEnd := j - 1
		if bodyEnd < bodyStart {
			bodyEnd = bodyStart
		}
		body := css[bodyStart:bodyEnd]
		block := cssBlock{Selector: selector, Body: body}
		if strings.Contains(body, "{") {
			block.Children = parseCSSBlocks(body)
		}
		blocks = append(blocks, block)
		i = j
	}
	return blocks
}

func isCSSSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// allLeafBlocks は木構造から子を持たない (=通常の宣言ブロックの) ノードのみを集める。
func allLeafBlocks(blocks []cssBlock) []cssBlock {
	var out []cssBlock
	for _, b := range blocks {
		if len(b.Children) == 0 {
			out = append(out, b)
		} else {
			out = append(out, allLeafBlocks(b.Children)...)
		}
	}
	return out
}

// findBlocks は述語に一致するノードを木全体 (グループ・リーフ問わず) から集める。
func findBlocks(blocks []cssBlock, pred func(cssBlock) bool) []cssBlock {
	var out []cssBlock
	for _, b := range blocks {
		if pred(b) {
			out = append(out, b)
		}
		if len(b.Children) > 0 {
			out = append(out, findBlocks(b.Children, pred)...)
		}
	}
	return out
}

// splitDeclarations は宣言ブロック本体を ';' で分割する。ただし url()/var()/rgba() 等の
// 関数の括弧内、および引用符文字列内の ';' では分割しない (data URI の "image/svg+xml;charset=..."
// のような値を1つの宣言として正しく扱うため)。
func splitDeclarations(body string) []string {
	var stmts []string
	depth := 0
	inQuote := false
	var quoteChar byte
	start := 0
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case inQuote:
			if c == quoteChar {
				inQuote = false
			}
		case c == '\'' || c == '"':
			inQuote = true
			quoteChar = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case c == ';' && depth == 0:
			stmts = append(stmts, body[start:i])
			start = i + 1
		}
	}
	if start < len(body) {
		stmts = append(stmts, body[start:])
	}
	return stmts
}

// parseAllDecls は宣言ブロック本体を "プロパティ名 -> 値" のマップへ変換する。
// 同名プロパティが複数回現れる場合は後勝ち (CSS のカスケードと同じ)。
func parseAllDecls(body string) map[string]string {
	decls := map[string]string{}
	for _, stmt := range splitDeclarations(body) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		idx := strings.IndexByte(stmt, ':')
		if idx < 0 {
			continue
		}
		prop := strings.TrimSpace(stmt[:idx])
		val := strings.TrimSpace(stmt[idx+1:])
		decls[prop] = val
	}
	return decls
}

// declValue は宣言ブロック本体から指定プロパティの値を1つ取り出す。
// マップキーの完全一致で引くため "background" と "background-image" のような
// プレフィックス関係にあるプロパティを取り違えない。
func declValue(body, prop string) (string, bool) {
	v, ok := parseAllDecls(body)[prop]
	return v, ok
}

// mergedDeclsForSelector は target セレクタを含む全リーフルール (複合セレクタの
// カンマ列挙を含む) の宣言をソース順にマージする (後勝ち=カスケード相当)。
// 同一セレクタが複数ルールに分割定義される場合 (例: 共通ブロック + 専用ブロック)
// の実効値を正しく求めるために使う。
func mergedDeclsForSelector(leaves []cssBlock, target string) map[string]string {
	merged := map[string]string{}
	for _, rule := range leaves {
		if !ruleMatchesSelector(rule.Selector, target) {
			continue
		}
		for k, v := range parseAllDecls(rule.Body) {
			merged[k] = v
		}
	}
	return merged
}

// customPropertyNames は宣言ブロック本体に含まれる "--xxx:" 形式のカスタムプロパティ名を集める。
func customPropertyNames(body string) map[string]bool {
	names := map[string]bool{}
	for prop := range parseAllDecls(body) {
		if strings.HasPrefix(prop, "--") {
			names[prop] = true
		}
	}
	return names
}

func isNeutralBackgroundValue(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.Contains(v, "var(--color-bg)") ||
		strings.Contains(v, "var(--color-surface)") ||
		v == "none" || v == "transparent" || v == ""
}

// ============================================================
// タスク 1.1: on-color トークン新設・コントラスト不具合2件の修正
// ============================================================

// 禁止パターン「彩色 background と color: var(--color-surface)/var(--color-text) の同居」がゼロであること。
// (このパターンは --color-surface/--color-text がテーマで反転するのに対し、彩色 background は
// テーマ不変のため、ダークテーマで文字が読めなくなる不具合を生む。)
func TestCSSGuard_NoColoredBackgroundWithThemeFlippingTextColor(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	var violations []string
	for _, rule := range allLeafBlocks(blocks) {
		bg, hasBG := declValue(rule.Body, "background")
		col, hasCol := declValue(rule.Body, "color")
		if !hasBG || !hasCol || isNeutralBackgroundValue(bg) {
			continue
		}
		if col == "var(--color-surface)" || col == "var(--color-text)" {
			violations = append(violations, rule.Selector)
		}
	}
	if len(violations) != 0 {
		t.Errorf("彩色backgroundとcolor:var(--color-surface)/var(--color-text)が同居する禁止パターンが残存: %v", violations)
	}
}

// --color-on-accent / --color-on-warning がテーマ不変トークンとして :root に存在すること。
func TestCSSGuard_OnColorTokensExist(t *testing.T) {
	css := loadCanonicalCSS(t)
	for _, tok := range []string{"--color-on-accent", "--color-on-warning"} {
		if !strings.Contains(css, tok+":") {
			t.Errorf("トークン %s が :root に存在しない", tok)
		}
	}
}

// .error-toast は on-accent、.badge-caution は on-warning を color に参照すること。
func TestCSSGuard_ErrorToastAndBadgeCautionUseOnColorTokens(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	cases := []struct {
		selector string
		want     string
	}{
		{".error-toast", "var(--color-on-accent)"},
		{".badge-caution", "var(--color-on-warning)"},
	}
	for _, c := range cases {
		found := false
		for _, rule := range leaves {
			if !ruleMatchesSelector(rule.Selector, c.selector) {
				continue
			}
			found = true
			col, ok := declValue(rule.Body, "color")
			if !ok || col != c.want {
				t.Errorf("%s の color は %s を参照すべきだが %q (存在=%v)", c.selector, c.want, col, ok)
			}
		}
		if !found {
			t.Errorf("セレクタ %s が正本CSSに見つからない", c.selector)
		}
	}
}

// ruleMatchesSelector はカンマ区切りの複合セレクタの中に target が含まれるかを判定する。
func ruleMatchesSelector(selector, target string) bool {
	for _, part := range strings.Split(selector, ",") {
		if strings.TrimSpace(part) == target {
			return true
		}
	}
	return false
}

func normalizeSelector(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// findLeafBySelector は木構造全体から、子を持たない (=宣言ブロックの) ノードのうち
// セレクタが target と一致する最初の1件を返す。
func findLeafBySelector(blocks []cssBlock, target string) (cssBlock, bool) {
	want := normalizeSelector(target)
	matches := findBlocks(blocks, func(b cssBlock) bool {
		return len(b.Children) == 0 && normalizeSelector(b.Selector) == want
	})
	if len(matches) == 0 {
		return cssBlock{}, false
	}
	return matches[0], true
}

const (
	darkMediaSelector = `:root:not([data-theme="light"])`
	darkAttrSelector  = `:root[data-theme="dark"]`
	lightRootSelector = `:root`
)

// ============================================================
// タスク 1.2: ダーク2導線のセマンティック再マップ
// ============================================================

// @media (prefers-color-scheme: dark) 導線と [data-theme="dark"] 導線は
// 同一集合のカスタムプロパティ名を再マップすること (値はパレット変数参照で単一ソース化)。
func TestCSSGuard_DarkWiresRemapSameVariableSet(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	mediaWire, ok := findLeafBySelector(blocks, darkMediaSelector)
	if !ok {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", darkMediaSelector)
	}
	attrWire, ok := findLeafBySelector(blocks, darkAttrSelector)
	if !ok {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", darkAttrSelector)
	}

	mediaNames := customPropertyNames(mediaWire.Body)
	attrNames := customPropertyNames(attrWire.Body)
	if len(mediaNames) == 0 {
		t.Fatalf("%q の再マップ変数が0件", darkMediaSelector)
	}
	for name := range mediaNames {
		if !attrNames[name] {
			t.Errorf("%q には %s があるが %q に無い", darkMediaSelector, name, darkAttrSelector)
		}
	}
	for name := range attrNames {
		if !mediaNames[name] {
			t.Errorf("%q には %s があるが %q に無い", darkAttrSelector, name, darkMediaSelector)
		}
	}
}

// 基調系7トークン (bg/surface/text/muted/border/shadow-sm/shadow-md) が両導線に含まれること。
func TestCSSGuard_DarkWiresContainBaseTokens(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	baseTokens := []string{
		"--color-bg", "--color-surface", "--color-text",
		"--color-muted", "--color-border", "--shadow-sm", "--shadow-md",
	}
	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		names := customPropertyNames(wire.Body)
		for _, tok := range baseTokens {
			if !names[tok] {
				t.Errorf("%q に %s の再マップが無い", sel, tok)
			}
		}
	}
}

// 彩色トークン(primary/primary-dark/danger/warning)とデータ意味色5種は再マップ対象に含まれないこと
// (両テーマ据置・研究フェーズの実測判断)。
func TestCSSGuard_DarkWiresExcludeSemanticColorTokens(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	excluded := []string{
		"--color-primary", "--color-primary-dark", "--color-danger", "--color-warning",
		"--color-vpd", "--color-dewpoint", "--color-gdd", "--color-trend", "--color-heat",
	}
	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		names := customPropertyNames(wire.Body)
		for _, tok := range excluded {
			if names[tok] {
				t.Errorf("%q に彩色/意味色トークン %s が再マップされている (両テーマ据置のはず)", sel, tok)
			}
		}
	}
}

// color-scheme / accent-color の宣言が両テーマに存在すること
// (ライト側 :root は light + accent-color、ダーク両導線は dark)。
func TestCSSGuard_ColorSchemeAndAccentColorDeclared(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	lightRoot, ok := findLeafBySelector(blocks, lightRootSelector)
	if !ok {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", lightRootSelector)
	}
	if v, ok := declValue(lightRoot.Body, "color-scheme"); !ok || v != "light" {
		t.Errorf(":root の color-scheme は light であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := declValue(lightRoot.Body, "accent-color"); !ok || v != "var(--color-primary)" {
		t.Errorf(":root の accent-color は var(--color-primary) であるべきだが %q (存在=%v)", v, ok)
	}

	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		if v, ok := declValue(wire.Body, "color-scheme"); !ok || v != "dark" {
			t.Errorf("%q の color-scheme は dark であるべきだが %q (存在=%v)", sel, v, ok)
		}
	}
}

// ============================================================
// タスク 1.3: 生色の掃討（トークン化と SVG・影のダーク対応）
// ============================================================

// 新設セマンティックトークン (banner/placeholder/chart-crosshair) が :root に存在すること。
func TestCSSGuard_NewSemanticTokensExist(t *testing.T) {
	css := loadCanonicalCSS(t)
	for _, tok := range []string{
		"--color-banner-bg", "--color-banner-text",
		"--color-placeholder-a", "--color-placeholder-b",
		"--color-chart-crosshair",
	} {
		if !strings.Contains(css, tok+":") {
			t.Errorf("トークン %s が :root に存在しない", tok)
		}
	}
}

// 新設トークンが両ダーク導線に再マップされていること。
func TestCSSGuard_NewSemanticTokensInBothDarkWires(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	newTokens := []string{
		"--color-banner-bg", "--color-banner-text",
		"--color-placeholder-a", "--color-placeholder-b",
		"--color-chart-crosshair",
	}
	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		names := customPropertyNames(wire.Body)
		for _, tok := range newTokens {
			if !names[tok] {
				t.Errorf("%q に %s の再マップが無い", sel, tok)
			}
		}
	}
}

// 対象セレクタ (.alert-banner/.chart-placeholder/.ch-vline,.ch-hline/.ch-dot) に生色が残存しないこと。
func TestCSSGuard_TargetSelectorsHaveNoRawColors(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	find := func(selector string) cssBlock {
		for _, rule := range leaves {
			if ruleMatchesSelector(rule.Selector, selector) {
				return rule
			}
		}
		t.Fatalf("セレクタ %q が正本CSSに見つからない", selector)
		return cssBlock{}
	}

	banner := find(".alert-banner")
	if bg, ok := declValue(banner.Body, "background"); !ok || bg != "var(--color-banner-bg)" {
		t.Errorf(".alert-banner の background は var(--color-banner-bg) であるべきだが %q (存在=%v)", bg, ok)
	}
	bannerH2 := find(".alert-banner h2")
	if col, ok := declValue(bannerH2.Body, "color"); !ok || col != "var(--color-banner-text)" {
		t.Errorf(".alert-banner h2 の color は var(--color-banner-text) であるべきだが %q (存在=%v)", col, ok)
	}
	bannerLi := find(".alert-banner li")
	if col, ok := declValue(bannerLi.Body, "color"); !ok || col != "var(--color-banner-text)" {
		t.Errorf(".alert-banner li の color は var(--color-banner-text) であるべきだが %q (存在=%v)", col, ok)
	}

	placeholder := find(".chart-placeholder")
	if bg, ok := declValue(placeholder.Body, "background"); !ok ||
		!strings.Contains(bg, "var(--color-placeholder-a)") || !strings.Contains(bg, "var(--color-placeholder-b)") {
		t.Errorf(".chart-placeholder の background は placeholder-a/b トークンを参照すべきだが %q (存在=%v)", bg, ok)
	}
	if strings.Contains(placeholder.Body, "#eef2f5") || strings.Contains(placeholder.Body, "#e4e9ed") {
		t.Errorf(".chart-placeholder に生色 (#eef2f5/#e4e9ed) が残存している")
	}

	crosshair := find(".ch-vline")
	if !ruleMatchesSelector(crosshair.Selector, ".ch-vline") || !ruleMatchesSelector(crosshair.Selector, ".ch-hline") {
		t.Fatalf(".ch-vline, .ch-hline の複合セレクタが想定と異なる: %q", crosshair.Selector)
	}
	if stroke, ok := declValue(crosshair.Body, "stroke"); !ok || stroke != "var(--color-chart-crosshair)" {
		t.Errorf(".ch-vline,.ch-hline の stroke は var(--color-chart-crosshair) であるべきだが %q (存在=%v)", stroke, ok)
	}

	dot := find(".ch-dot")
	if stroke, ok := declValue(dot.Body, "stroke"); !ok || stroke != "var(--color-surface)" {
		t.Errorf(".ch-dot の stroke は var(--color-surface) であるべきだが %q (存在=%v)", stroke, ok)
	}
}

// select 矢印は生の stroke='%236c757d' 直書きを廃し、テーマに追従する CSS カスタム
// プロパティ (両ダーク導線で異なる値に再マップ) 経由で描画すること。
// (mask-image を <select> 本体に直接適用すると要素全体がマスクされ枠線/文字が消えるため
//
//	実装は background-image + var() トークン切替方式を採る。実機検証で確認済み。)
func TestCSSGuard_SelectArrowUsesThemeAwareToken(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	const target = ".form-group select"
	merged := mergedDeclsForSelector(leaves, target)
	if len(merged) == 0 {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", target)
	}
	for _, rule := range leaves {
		if ruleMatchesSelector(rule.Selector, target) && strings.Contains(rule.Body, "6c757d") {
			t.Errorf("%s に生色 (6c757d) の直書きが残存している", target)
		}
	}
	bgImage, ok := merged["background-image"]
	if !ok || !strings.HasPrefix(bgImage, "var(--") {
		t.Fatalf("%s の background-image はトークン参照であるべきだが %q (存在=%v)", target, bgImage, ok)
	}
	tokenName := strings.TrimSuffix(strings.TrimPrefix(bgImage, "var("), ")")

	lightRoot, ok := findLeafBySelector(blocks, lightRootSelector)
	if !ok {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", lightRootSelector)
	}
	lightVal, ok := declValue(lightRoot.Body, tokenName)
	if !ok {
		t.Fatalf(":root にトークン %s のライト既定値が無い", tokenName)
	}

	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		darkVal, ok := declValue(wire.Body, tokenName)
		if !ok {
			t.Errorf("%q にトークン %s のダーク再マップが無い", sel, tokenName)
			continue
		}
		if darkVal == lightVal {
			t.Errorf("%q の %s がライト値と同一 (テーマに追従していない)", sel, tokenName)
		}
	}
}

// ============================================================
// タスク 1.4: Tom Select ドロップダウンとトグル部品のスタイル
// ============================================================

// .ts-dropdown が既存の z-index 上書き (TS02) に加え、トークン参照の配色上書きを持つこと
// (CDN既定は白背景固定のため、ダーク時に白いドロップダウンが残らないようにする)。
// 既存の z-index !important も維持されていること (回帰防止)。
func TestCSSGuard_TSDropdownHasTokenizedColors(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	merged := mergedDeclsForSelector(leaves, ".ts-dropdown")
	if len(merged) == 0 {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", ".ts-dropdown")
	}
	if v, ok := merged["z-index"]; !ok || !strings.Contains(v, "200") {
		t.Errorf(".ts-dropdown の z-index 上書き (TS02) が失われている: %q (存在=%v)", v, ok)
	}
	for _, prop := range []string{"background", "color", "border-color"} {
		v, ok := merged[prop]
		if !ok || !strings.HasPrefix(v, "var(--color-") {
			t.Errorf(".ts-dropdown の %s はトークン参照であるべきだが %q (存在=%v)", prop, v, ok)
		}
	}
}

// .ts-control (Tom Select の可視コントロール本体) も .ts-dropdown と同じ理由 (CDN
// tom-select.css が本CSSより後読み込みで同一詳細度は後勝ち) で background/color/border-color が
// CDN既定 (白背景・濃色文字固定) に負ける。ライトでは偶然近い見た目になり気付きにくいが、
// ダークでは白いコントロールが残る (実機確認で発見・2026-07-02)。!important で明示上書きすること。
func TestCSSGuard_TSControlHasImportantTokenizedColors(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	const target = ".form-group .ts-wrapper.single .ts-control"
	merged := mergedDeclsForSelector(leaves, target)
	if len(merged) == 0 {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", target)
	}
	for _, prop := range []string{"background", "color", "border-color"} {
		v, ok := merged[prop]
		if !ok || !strings.HasPrefix(v, "var(--color-") || !strings.Contains(v, "!important") {
			t.Errorf("%s の %s はトークン参照 + !important であるべきだが %q (存在=%v)", target, prop, v, ok)
		}
	}
}

// トグル UI 部品 (.theme-toggle) がトークン参照のスタイルを持つこと。
func TestCSSGuard_ThemeToggleComponentStyled(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	merged := mergedDeclsForSelector(leaves, ".theme-toggle")
	if len(merged) == 0 {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", ".theme-toggle")
	}
	if v, ok := merged["background"]; !ok || v != "transparent" {
		t.Errorf(".theme-toggle の background は transparent であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := merged["border"]; !ok || !strings.Contains(v, "var(--color-border)") {
		t.Errorf(".theme-toggle の border は var(--color-border) を参照すべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := merged["border-radius"]; !ok || v != "var(--radius)" {
		t.Errorf(".theme-toggle の border-radius は var(--radius) であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := merged["color"]; !ok || v != "var(--color-text)" {
		t.Errorf(".theme-toggle の color は var(--color-text) であるべきだが %q (存在=%v)", v, ok)
	}
}

// Guest 用ラッパ (.guest-theme-toggle) が右上 fixed 配置であること。
func TestCSSGuard_GuestThemeToggleWrapperFixedTopRight(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)
	leaves := allLeafBlocks(blocks)

	merged := mergedDeclsForSelector(leaves, ".guest-theme-toggle")
	if len(merged) == 0 {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", ".guest-theme-toggle")
	}
	if v, ok := merged["position"]; !ok || v != "fixed" {
		t.Errorf(".guest-theme-toggle の position は fixed であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := merged["top"]; !ok || v != "var(--space-4)" {
		t.Errorf(".guest-theme-toggle の top は var(--space-4) であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := merged["right"]; !ok || v != "var(--space-4)" {
		t.Errorf(".guest-theme-toggle の right は var(--space-4) であるべきだが %q (存在=%v)", v, ok)
	}
}

// --display-when-dark/--display-when-light がライト既定 (none/inline) を持ち、
// 両ダーク導線で反転 (inline/none) すること (アイコン切替用)。
func TestCSSGuard_DisplayWhenTokensExistAndReverseInDarkWires(t *testing.T) {
	css := stripCSSComments(loadCanonicalCSS(t))
	blocks := parseCSSBlocks(css)

	lightRoot, ok := findLeafBySelector(blocks, lightRootSelector)
	if !ok {
		t.Fatalf("セレクタ %q が正本CSSに見つからない", lightRootSelector)
	}
	if v, ok := declValue(lightRoot.Body, "--display-when-dark"); !ok || v != "none" {
		t.Errorf(":root の --display-when-dark は none であるべきだが %q (存在=%v)", v, ok)
	}
	if v, ok := declValue(lightRoot.Body, "--display-when-light"); !ok || v != "inline" {
		t.Errorf(":root の --display-when-light は inline であるべきだが %q (存在=%v)", v, ok)
	}

	for _, sel := range []string{darkMediaSelector, darkAttrSelector} {
		wire, ok := findLeafBySelector(blocks, sel)
		if !ok {
			t.Fatalf("セレクタ %q が正本CSSに見つからない", sel)
		}
		if v, ok := declValue(wire.Body, "--display-when-dark"); !ok || v != "inline" {
			t.Errorf("%q の --display-when-dark は inline であるべきだが %q (存在=%v)", sel, v, ok)
		}
		if v, ok := declValue(wire.Body, "--display-when-light"); !ok || v != "none" {
			t.Errorf("%q の --display-when-light は none であるべきだが %q (存在=%v)", sel, v, ok)
		}
	}
}

// ============================================================
// タスク 1.5: モック 10 枚へのトグル要素反映
// ============================================================

// 全モック HTML (認証後8枚 + login/register) に theme-toggle 要素が存在すること
// (8.1: モック直開きでトグル要素を確認できる状態に保守される)。
func TestCSSGuard_AllMockHTMLContainThemeToggle(t *testing.T) {
	const mocksDir = "../../mocks/html"
	entries, err := os.ReadDir(mocksDir)
	if err != nil {
		t.Fatalf("モックディレクトリの読込に失敗: %v", err)
	}

	var htmlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".html") {
			htmlFiles = append(htmlFiles, e.Name())
		}
	}
	if len(htmlFiles) == 0 {
		t.Fatal("モック HTML が1件も見つからない")
	}

	for _, name := range htmlFiles {
		b, err := os.ReadFile(mocksDir + "/" + name)
		if err != nil {
			t.Fatalf("%s の読込に失敗: %v", name, err)
		}
		html := string(b)
		if !strings.Contains(html, `class="theme-toggle"`) {
			t.Errorf("%s に theme-toggle 要素が無い", name)
		}
	}
}
