package component

// seasonal_trend_view.go は統計分析ページ（長期トレンド・季節サマリ）の component 側 DTO。
// すべて整形済み primitive で保持し pgtype/repository 型を持ち込まない（view 純粋性・structure.md）。
// handler（seasonal_trend_handler）がロールアップ集約・トレンド検定・描画 option を整形して詰める。
// デバイス選択肢は既存 DeviceOption（alert_rule_view.go）を流用する。

// RollupRow はロールアップ統計サマリ表 1 行（粒度×1指標・整形済み文字列）。
// 単位は列見出し側に付くためセルは素の数値文字列（CV は無次元のため単位なし・DailyStatRow と同方針）。
// 欠測は handler が "—" を詰める（空月はそもそも行が生成されない＝0補完しない）。
type RollupRow struct {
	Bucket       string // "2024-01"（月次）/ "2024"（年次）
	Avg          string // 平均
	Max          string // 最高
	Min          string // 最低
	DiurnalRange string // 日較差ΔT（=日次ΔTの平均）
	StdDev       string // 標準偏差σ
	CV           string // 変動係数（σ/μ・無次元）
	Samples      string // サンプル数
}

// TrendBadgeView は Mann-Kendall 判定バッジ 1 件の表示データ。
// Direction（up/down/flat）は信号色 CSS（.badge-trend-*）に 1:1 対応する。
// Stage は判定の区分（一次判定（多重比較未補正）/ 補正済み（Hamed-Rao））で、一次/厳密を区別する（要件 6.3）。
type TrendBadgeView struct {
	Metric    string // 指標 "温度" / "湿度"
	Stage     string // 区分 "一次判定（多重比較未補正）" / "補正済み（Hamed-Rao）"
	Direction string // up / down / flat（.badge-trend-* に対応）
	Verdict   string // 表示文言 "有意な上昇" / "有意な下降" / "非有意"
	Slope     string // Sen の傾き "+0.03 ℃/年"
	PValue    string // p 値 "0.012"
}

// TrendSectionView は #trend-section（HTMX 部分更新単位）の表示データ。
// デバイス/期間切替時にこの領域全体を innerHTML 差し替えする（alert-rules と同型）。
// HasData=false は未選択/データ無で、EmptyMessage の案内のみ描画し検定を断定しない（要件 1.4/6.x）。
type TrendSectionView struct {
	HasData      bool
	EmptyMessage string // 未選択/データ無の案内（HasData=false 時）

	PowerNote string           // 検出力留保（"非有意≠トレンド無し" 等）・空なら非表示（要件 6.1/6.2）
	Badges    []TrendBadgeView // トレンド判定（温度/湿度 × 一次/補正済み）

	TempRows     []RollupRow // ロールアップ統計サマリ（温度）
	HumidityRows []RollupRow // ロールアップ統計サマリ（湿度）

	TrendOptionJSON   string // 長期トレンドチャート option（HTML 安全 JSON・空なら器を出さない）
	DiurnalOptionJSON string // 日較差ΔTチャート option（空なら器を出さない）
	TrendColor        string // data-color（--color-trend）
	ChartUnit         string // data-unit（"℃"）

	ClimatologyNote string // 平年比注記（要件 7.3・空なら出さない）
}
