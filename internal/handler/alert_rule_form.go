// alert_rule_form.go はアラートルール追加・更新フォームの入力検証・型変換を担う。
// alert_rule.go (HTTP 境界) から検証責務を分離する。threshold は string で受け
// 「未入力(空)=エラー / "0"=妥当」を両立する (float64+required はゼロ値 0 を未入力と
// 誤判定し 温度>0℃ 等の妥当値を弾くため。design「threshold バインド方針」)。
// binding の required/oneof エラーと threshold のパース/範囲エラーを統合し、
// 複数項目の同時エラーを全て載せる (要件 8.5)。メッセージは device_form.go と別系統。
package handler

import (
	"errors"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// alertRuleForm は追加/更新が c.ShouldBind で受ける binding 構造体。
// metric/operator は CHECK 許容値に一致する oneof のみ受理 (不正値で DB CHECK 違反に至らせない)。
// threshold は string で受け空文字を required で弾き、数値変換・範囲は handler 内で検証する。
// device_id は追加時のみ使用 (更新は path の rule から所属デバイスを引く)。
type alertRuleForm struct {
	DeviceID  int64  `form:"device_id"`
	Metric    string `form:"metric"    binding:"required,oneof=temperature humidity"`
	Operator  string `form:"operator"  binding:"required,oneof=> < >= <="`
	Threshold string `form:"threshold" binding:"required"`
}

// thresholdMax は numeric(5,2) の保存可能上限 (|x| ≤ 999.99)。
// パース後に範囲検査し DB の numeric overflow(500) を未然に防ぐ。
const thresholdMax = 999.99

var (
	errThresholdNotNumeric = errors.New("alert rule: threshold not numeric")
	errThresholdOutOfRange = errors.New("alert rule: threshold out of range")
)

// parseThreshold は閾値文字列を numeric(5,2) 保存可能な float へ変換する。
// 空文字は呼び出し側が binding required で先に弾く前提 (本関数では非数値扱い)。
// 数値変換失敗→errThresholdNotNumeric、範囲外(|x|>999.99)→errThresholdOutOfRange。
func parseThreshold(s string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, errThresholdNotNumeric
	}
	if f < -thresholdMax || f > thresholdMax {
		return 0, errThresholdOutOfRange
	}
	return f, nil
}

// validateAlertRuleForm は ShouldBind 結果 (form, bindErr) を受け、保存用 threshold と
// 項目別エラー map を返す。binding エラー (required/oneof) と threshold のパース/範囲エラーを
// 統合し、複数項目の同時エラーを全て載せる (要件 8.5)。errs が空なら検証通過。
func validateAlertRuleForm(form alertRuleForm, bindErr error) (float64, map[string]string) {
	errs := map[string]string{}
	if bindErr != nil {
		errs = toAlertRuleFieldErrors(bindErr)
	}
	var threshold float64
	// threshold が required を通過 (非空) なら数値・範囲も検証する。
	// 空 (required エラー済み) は parse せず重複メッセージを避ける。
	if _, has := errs["threshold"]; !has {
		f, perr := parseThreshold(form.Threshold)
		switch {
		case errors.Is(perr, errThresholdNotNumeric):
			errs["threshold"] = "閾値は数値で入力してください"
		case errors.Is(perr, errThresholdOutOfRange):
			errs["threshold"] = "閾値は -999.99 〜 999.99 の範囲で入力してください"
		default:
			threshold = f
		}
	}
	return threshold, errs
}

// alertRuleFieldKey は alertRuleForm の Go フィールド名をフォームキー名へ変換する。
// gin の validator は fe.Field() で Go フィールド名を返すため、それをフォーム名へ写す。
func alertRuleFieldKey(structField string) string {
	switch structField {
	case "Metric":
		return "metric"
	case "Operator":
		return "operator"
	case "Threshold":
		return "threshold"
	case "DeviceID":
		return "device_id"
	default:
		return strings.ToLower(structField)
	}
}

// alertRuleValidationMessage は フォームキー と バリデーションタグ から日本語メッセージを返す。
func alertRuleValidationMessage(field, tag string) string {
	switch field {
	case "metric":
		switch tag {
		case "required":
			return "指標を選択してください"
		case "oneof":
			return "指標の値が不正です"
		}
	case "operator":
		switch tag {
		case "required":
			return "条件を選択してください"
		case "oneof":
			return "条件の値が不正です"
		}
	case "threshold":
		switch tag {
		case "required":
			return "閾値を入力してください"
		}
	}
	return "入力内容を確認してください"
}

// toAlertRuleFieldErrors は binding 失敗エラーを field → 日本語メッセージの map に変換する。
// validator.ValidationErrors 以外のエラーはフォーム全体の汎用メッセージとして "form" キーに入れる。
func toAlertRuleFieldErrors(err error) map[string]string {
	out := make(map[string]string)
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			key := alertRuleFieldKey(fe.Field())
			out[key] = alertRuleValidationMessage(key, fe.Tag())
		}
		return out
	}
	out["form"] = "入力内容を確認してください"
	return out
}
