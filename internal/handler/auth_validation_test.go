package handler

import "testing"

func TestValidationMessage(t *testing.T) {
	tests := []struct {
		field, tag, want string
	}{
		{"name", "required", "ユーザー名を入力してください"},
		{"name", "max", "255文字以内で入力してください"},
		{"email", "required", "メールアドレスを入力してください"},
		{"email", "email", "メールアドレス形式で入力してください"},
		{"password", "required", "パスワードを入力してください"},
		{"password", "min", "8文字以上で入力してください"},
		{"password_confirmation", "required", "確認用パスワードを入力してください"},
		{"password_confirmation", "eqfield", "パスワードが一致しません"},
		{"email", "unknown_tag", "入力内容を確認してください"}, // フォールバック
	}
	for _, tt := range tests {
		if got := validationMessage(tt.field, tt.tag); got != tt.want {
			t.Errorf("validationMessage(%q, %q) = %q, want %q", tt.field, tt.tag, got, tt.want)
		}
	}
}

func TestFieldKey(t *testing.T) {
	tests := []struct {
		structField, want string
	}{
		{"Name", "name"},
		{"Email", "email"},
		{"Password", "password"},
		{"PasswordConfirmation", "password_confirmation"},
	}
	for _, tt := range tests {
		if got := fieldKey(tt.structField); got != tt.want {
			t.Errorf("fieldKey(%q) = %q, want %q", tt.structField, got, tt.want)
		}
	}
}
