package domain

import "fmt"

// QualityFlag は計測レコード単位の品質問題の種別を表す Enum。
// 欠測直後・固着(stuck/flatline)・物理異常・外れ値の4種で、品質純関数層（internal/chart）の
// 判定結果を表示カテゴリへ写すために使う。
//
// Metric/Crop と異なり DB には永続化しない（読み取り時計算由来の表示カテゴリ）。
// よって VARCHAR+CHECK 制約の対象外＝純 Go 列挙であり、本ファイルは fmt のみに依存する。
type QualityFlag string

const (
	QualityFlagMissing  QualityFlag = "missing"  // 欠測直後（期待間隔を超えた飛び）
	QualityFlagStuck    QualityFlag = "stuck"    // 固着（同値が一定回数以上連続）
	QualityFlagPhysical QualityFlag = "physical" // 物理異常（農学的にあり得ない値・据置故障・急変）
	QualityFlagOutlier  QualityFlag = "outlier"  // 外れ値（統計的期待からの著しい逸脱）
)

// Label は画面表示用の日本語ラベルを返す。
func (f QualityFlag) Label() string {
	switch f {
	case QualityFlagMissing:
		return "欠測直後"
	case QualityFlagStuck:
		return "固着"
	case QualityFlagPhysical:
		return "物理異常"
	case QualityFlagOutlier:
		return "外れ値"
	}
	return string(f)
}

// Valid は Enum として定義された値かを判定する。
func (f QualityFlag) Valid() bool {
	switch f {
	case QualityFlagMissing, QualityFlagStuck, QualityFlagPhysical, QualityFlagOutlier:
		return true
	}
	return false
}

// ParseQualityFlag は文字列から QualityFlag への変換を試み、不正値ならエラーを返す。
func ParseQualityFlag(s string) (QualityFlag, error) {
	f := QualityFlag(s)
	if !f.Valid() {
		return "", fmt.Errorf("invalid quality flag: %q", s)
	}
	return f, nil
}

// AllQualityFlags は定義済み QualityFlag の全列挙（判定順＝表示順）。
func AllQualityFlags() []QualityFlag {
	return []QualityFlag{QualityFlagMissing, QualityFlagStuck, QualityFlagPhysical, QualityFlagOutlier}
}

// QualityLevel は期間/デバイスの総合品質を表す信号色レベルの Enum。
// 信頼(緑)・注意(黄)・不良(赤)の3段階で、欠測率・外れ値率・固着有無・間隔一貫性を合成した
// 総合バッジの色を決める。QualityFlag 同様 DB 非永続の純 Go 列挙。
type QualityLevel string

const (
	QualityLevelGood    QualityLevel = "good"    // 信頼（緑）
	QualityLevelCaution QualityLevel = "caution" // 注意（黄）
	QualityLevelBad     QualityLevel = "bad"     // 不良（赤）
)

// Label は画面表示用の日本語ラベルを返す。
func (l QualityLevel) Label() string {
	switch l {
	case QualityLevelGood:
		return "信頼"
	case QualityLevelCaution:
		return "注意"
	case QualityLevelBad:
		return "不良"
	}
	return string(l)
}

// Valid は Enum として定義された値かを判定する。
func (l QualityLevel) Valid() bool {
	switch l {
	case QualityLevelGood, QualityLevelCaution, QualityLevelBad:
		return true
	}
	return false
}

// BadgeClass はバッジ信号色の CSS クラス名（dot なし）を返す。
// @layer components の .badge-good / .badge-caution / .badge-bad variant と 1:1 対応し、
// templ では class="badge {BadgeClass()}" の形で消費する。
func (l QualityLevel) BadgeClass() string {
	switch l {
	case QualityLevelGood:
		return "badge-good"
	case QualityLevelCaution:
		return "badge-caution"
	case QualityLevelBad:
		return "badge-bad"
	}
	return ""
}

// ParseQualityLevel は文字列から QualityLevel への変換を試み、不正値ならエラーを返す。
func ParseQualityLevel(s string) (QualityLevel, error) {
	l := QualityLevel(s)
	if !l.Valid() {
		return "", fmt.Errorf("invalid quality level: %q", s)
	}
	return l, nil
}

// AllQualityLevels は定義済み QualityLevel の全列挙（良→悪の順）。
func AllQualityLevels() []QualityLevel {
	return []QualityLevel{QualityLevelGood, QualityLevelCaution, QualityLevelBad}
}
