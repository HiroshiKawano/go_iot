package handler

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"
)

// fieldKey は binding 構造体の Go フィールド名を、templ/フォームのキー名へ変換する。
func fieldKey(structField string) string {
	switch structField {
	case "Name":
		return "name"
	case "Email":
		return "email"
	case "Password":
		return "password"
	case "PasswordConfirmation":
		return "password_confirmation"
	default:
		return strings.ToLower(structField)
	}
}

// validationMessage は フォームキー と バリデーションタグ から日本語エラーメッセージを返す。
func validationMessage(field, tag string) string {
	switch field {
	case "name":
		switch tag {
		case "required":
			return "ユーザー名を入力してください"
		case "max":
			return "255文字以内で入力してください"
		}
	case "email":
		switch tag {
		case "required":
			return "メールアドレスを入力してください"
		case "email":
			return "メールアドレス形式で入力してください"
		}
	case "password":
		switch tag {
		case "required":
			return "パスワードを入力してください"
		case "min":
			return "8文字以上で入力してください"
		}
	case "password_confirmation":
		switch tag {
		case "required":
			return "確認用パスワードを入力してください"
		case "eqfield":
			return "パスワードが一致しません"
		}
	}
	return "入力内容を確認してください"
}

// translateValidationErrors は binding 失敗エラーを field → 日本語メッセージの map に変換する。
// validator.ValidationErrors 以外のエラーはフォーム全体の汎用メッセージとして "form" キーに入れる。
func translateValidationErrors(err error) map[string]string {
	out := make(map[string]string)
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			key := fieldKey(fe.Field())
			out[key] = validationMessage(key, fe.Tag())
		}
		return out
	}
	out["form"] = "入力内容を確認してください"
	return out
}
