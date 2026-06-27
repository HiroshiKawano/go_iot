// device_form.go はデバイス登録・編集フォームの入力検証・変換ロジックを担う。
// device.go (HTTP 境界) から検証・正規化の責務を分離し、認証フォーム (auth_validation.go)
// とは別系統のデバイス専用メッセージを持つ ("name" キーが「デバイス名」「ユーザー名」で
// 衝突するため)。MAC の形式検証・大文字正規化・型変換 (location/is_active) をここに閉じる。
package handler

import (
	"errors"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// deviceForm はデバイス登録・更新が c.ShouldBind で受ける binding 構造体。
// MAC の形式検証・正規化・一意検査は binding では表現できないため handler 内で行い、
// ここでは必須・文字数・ステータスの許容値のみをタグで担保する。
type deviceForm struct {
	Name       string `form:"name"        binding:"required,max=255"`
	MacAddress string `form:"mac_address" binding:"required"`           // 形式/正規化/一意は handler で
	Locality   string `form:"locality"`                                 // 沖縄の地域 (domain.Locality)。存在検証は handler で procedural に行う
	IsActive   string `form:"is_active"   binding:"required,oneof=1 0"` // "1"=稼働中 / "0"=停止中
}

// macFormat は MACアドレスの許容形式 (16進2桁をコロン区切りで6組)。
// devices テーブルの CHECK 制約 devices_mac_address_format と同一基準。
// 正規化 (normalizeMac) 後の値に対して適用する。
var macFormat = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

// normalizeMac は MACアドレスを前後空白除去のうえ大文字へ正規化する。
// 一意判定・保存・自己除外比較はすべて正規化後の値で行う (R6 AC3)。
func normalizeMac(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// isValidMacFormat は値が MACアドレス形式に一致するかを判定する。
// 桁不足・区切り違い・16進外文字・前後空白付きはいずれも false。
func isValidMacFormat(s string) bool {
	return macFormat.MatchString(s)
}

// nullableStr は任意入力文字列を保存用の *string へ変換する。
// 空文字は「未設定」とみなして nil を返す (設置場所=地域などの nullable カラム用)。
func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseIsActive は radio 値 ("1"/"0") を稼働状態 (真偽) へ変換する。
// "1"=稼働中=true、それ以外 (oneof 通過後は "0")=停止中=false。
func parseIsActive(s string) bool {
	return s == "1"
}

// deviceFieldKey は deviceForm の Go フィールド名を templ/フォームのキー名へ変換する。
// gin の validator は fe.Field() で Go フィールド名を返すため、それをフォーム名へ写す。
func deviceFieldKey(structField string) string {
	switch structField {
	case "Name":
		return "name"
	case "MacAddress":
		return "mac_address"
	case "Locality":
		return "locality"
	case "IsActive":
		return "is_active"
	default:
		return strings.ToLower(structField)
	}
}

// deviceValidationMessage は フォームキー と バリデーションタグ から日本語エラーメッセージを返す。
// 認証フォーム (auth_validation.go の validationMessage) とは別系統で、"name" は
// 「デバイス名」を指す (「ユーザー名」と混同しない)。
func deviceValidationMessage(field, tag string) string {
	switch field {
	case "name":
		switch tag {
		case "required":
			return "デバイス名を入力してください"
		case "max":
			return "デバイス名は255文字以内で入力してください"
		}
	case "mac_address":
		switch tag {
		case "required":
			return "MACアドレスを入力してください"
		}
	case "is_active":
		switch tag {
		case "required":
			return "ステータスを選択してください"
		case "oneof":
			return "ステータスが不正です"
		}
	}
	return "入力内容を確認してください"
}

// toDeviceFieldErrors は binding 失敗エラーを field → 日本語メッセージの map に変換する。
// validator.ValidationErrors 以外のエラーはフォーム全体の汎用メッセージとして "form" キーに入れる。
func toDeviceFieldErrors(err error) map[string]string {
	out := make(map[string]string)
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			key := deviceFieldKey(fe.Field())
			out[key] = deviceValidationMessage(key, fe.Tag())
		}
		return out
	}
	out["form"] = "入力内容を確認してください"
	return out
}
