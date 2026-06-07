package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

// --- Task 1.1: MAC 正規化・形式検証・型変換ヘルパ ---

func TestNormalizeMac(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"小文字は大文字化", "aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF"},
		{"大文字小文字混在も大文字化", "Aa:bB:Cc:dD:Ee:fF", "AA:BB:CC:DD:EE:FF"},
		{"前後空白は除去", "  aa:bb:cc:dd:ee:ff  ", "AA:BB:CC:DD:EE:FF"},
		{"前後タブ・改行も除去", "\taa:bb:cc:dd:ee:ff\n", "AA:BB:CC:DD:EE:FF"},
		{"既に大文字ならそのまま", "AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF"},
		{"空文字はそのまま空", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMac(tt.in); got != tt.want {
				t.Errorf("normalizeMac(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsValidMacFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"正規の大文字MACは有効", "AA:BB:CC:DD:EE:FF", true},
		{"小文字MACも形式としては有効", "aa:bb:cc:dd:ee:ff", true},
		{"数字混じりも有効", "01:23:45:67:89:AB", true},
		{"組数不足(5組)は無効", "AA:BB:CC:DD:EE", false},
		{"組数過多(7組)は無効", "AA:BB:CC:DD:EE:FF:00", false},
		{"各組1桁は無効", "A:B:C:D:E:F", false},
		{"区切りがハイフンは無効", "AA-BB-CC-DD-EE-FF", false},
		{"区切り無しは無効", "AABBCCDDEEFF", false},
		{"16進外の文字は無効", "GG:HH:II:JJ:KK:LL", false},
		{"前後空白付きは無効(正規化前提)", " AA:BB:CC:DD:EE:FF ", false},
		{"空文字は無効", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidMacFormat(tt.in); got != tt.want {
				t.Errorf("isValidMacFormat(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeThenValidate(t *testing.T) {
	// 設計の不変条件: normalizeMac → isValidMacFormat の順で、
	// 小文字・前後空白付きの入力も正規化後は有効と判定される。
	raw := "  aa:bb:cc:dd:ee:ff  "
	norm := normalizeMac(raw)
	if !isValidMacFormat(norm) {
		t.Errorf("正規化後 %q は有効であるべき", norm)
	}
}

func TestLocationPtr(t *testing.T) {
	t.Run("空文字はnil(未設定)", func(t *testing.T) {
		if got := locationPtr(""); got != nil {
			t.Errorf("locationPtr(\"\") = %v, want nil", got)
		}
	})
	t.Run("値があれば非nilで同値を指す", func(t *testing.T) {
		got := locationPtr("第1ハウス")
		if got == nil {
			t.Fatal("locationPtr(\"第1ハウス\") = nil, want 非nil")
		}
		if *got != "第1ハウス" {
			t.Errorf("*locationPtr = %q, want %q", *got, "第1ハウス")
		}
	})
}

func TestParseIsActive(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1", true},
		{"0", false},
	}
	for _, tt := range tests {
		t.Run("in="+tt.in, func(t *testing.T) {
			if got := parseIsActive(tt.in); got != tt.want {
				t.Errorf("parseIsActive(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- Task 1.2: バインド規則 (deviceForm) と項目別日本語エラー変換 ---

// validDeviceForm は有効データのベース。mut で 1 フィールドだけ壊して table-driven に使う。
func validDeviceForm(mut func(*deviceForm)) deviceForm {
	f := deviceForm{
		Name:       "温室センサー",
		MacAddress: "AA:BB:CC:DD:EE:FF",
		Location:   "第1ハウス",
		IsActive:   "1",
	}
	if mut != nil {
		mut(&f)
	}
	return f
}

// bindDeviceForm は gin の ShouldBind と同じく binding タグで検証し、
// 失敗エラー (validator.ValidationErrors) を返す。gin は SetTagName("binding") を
// 行うため、テストでも同設定で忠実再現する。
func bindDeviceForm(f deviceForm) error {
	v := validator.New()
	v.SetTagName("binding")
	return v.Struct(f)
}

func TestDeviceForm_有効データは通過(t *testing.T) {
	if err := bindDeviceForm(validDeviceForm(nil)); err != nil {
		t.Fatalf("有効データで検証失敗: %v", err)
	}
}

// TestToDeviceFieldErrors_各タグが対応する日本語になる は
// required/max/oneof 各失敗が項目別の日本語メッセージに変換されることを確認する。
func TestToDeviceFieldErrors_各タグが対応する日本語になる(t *testing.T) {
	long := strings.Repeat("あ", 256) // 255 超 (validator の max はルート数で判定)
	tests := []struct {
		name    string
		mut     func(*deviceForm)
		wantKey string
		wantMsg string
	}{
		{"name 空は必須", func(f *deviceForm) { f.Name = "" }, "name", "デバイス名を入力してください"},
		{"name 256文字は上限超過", func(f *deviceForm) { f.Name = long }, "name", "デバイス名は255文字以内で入力してください"},
		{"mac_address 空は必須", func(f *deviceForm) { f.MacAddress = "" }, "mac_address", "MACアドレスを入力してください"},
		{"location 256文字は上限超過", func(f *deviceForm) { f.Location = long }, "location", "設置場所は255文字以内で入力してください"},
		{"is_active 空は必須", func(f *deviceForm) { f.IsActive = "" }, "is_active", "ステータスを選択してください"},
		{"is_active 範囲外はoneof", func(f *deviceForm) { f.IsActive = "2" }, "is_active", "ステータスが不正です"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := bindDeviceForm(validDeviceForm(tt.mut))
			if err == nil {
				t.Fatal("検証は失敗するはずが通過した")
			}
			got := toDeviceFieldErrors(err)
			if got[tt.wantKey] != tt.wantMsg {
				t.Errorf("toDeviceFieldErrors()[%q] = %q, want %q", tt.wantKey, got[tt.wantKey], tt.wantMsg)
			}
		})
	}
}

// TestToDeviceFieldErrors_認証フォームと文言が混同されない は
// "name" キーがデバイス文脈 (ユーザー名ではない) であることを確認する (タスク 1.2)。
func TestToDeviceFieldErrors_認証フォームと文言が混同されない(t *testing.T) {
	err := bindDeviceForm(validDeviceForm(func(f *deviceForm) { f.Name = "" }))
	got := toDeviceFieldErrors(err)
	if got["name"] == "ユーザー名を入力してください" {
		t.Errorf("name メッセージが認証フォームの文言と混同されている: %q", got["name"])
	}
	if !strings.Contains(got["name"], "デバイス名") {
		t.Errorf("name メッセージはデバイス文脈であるべき: %q", got["name"])
	}
}

// TestToDeviceFieldErrors_複数項目同時失敗は各項目分そろう は
// 同時に複数フィールドが失敗したとき、各キーにメッセージが揃うことを確認する (R5.6)。
func TestToDeviceFieldErrors_複数項目同時失敗は各項目分そろう(t *testing.T) {
	err := bindDeviceForm(validDeviceForm(func(f *deviceForm) {
		f.Name = ""
		f.MacAddress = ""
		f.IsActive = ""
	}))
	if err == nil {
		t.Fatal("複数項目失敗で検証は失敗するはず")
	}
	got := toDeviceFieldErrors(err)
	for _, key := range []string{"name", "mac_address", "is_active"} {
		if got[key] == "" {
			t.Errorf("キー %q のエラーメッセージが欠落: %v", key, got)
		}
	}
}

// TestToDeviceFieldErrors_検証以外のエラーはフォーム全体メッセージ は
// validator.ValidationErrors でないエラーが "form" キーの汎用メッセージになることを確認する。
func TestToDeviceFieldErrors_検証以外のエラーはフォーム全体メッセージ(t *testing.T) {
	got := toDeviceFieldErrors(errors.New("予期せぬエラー"))
	if got["form"] == "" {
		t.Errorf("検証以外のエラーは form キーに汎用メッセージを入れるべき: %v", got)
	}
}

func TestDeviceValidationMessage(t *testing.T) {
	tests := []struct {
		field, tag, want string
	}{
		{"name", "required", "デバイス名を入力してください"},
		{"name", "max", "デバイス名は255文字以内で入力してください"},
		{"mac_address", "required", "MACアドレスを入力してください"},
		{"location", "max", "設置場所は255文字以内で入力してください"},
		{"is_active", "required", "ステータスを選択してください"},
		{"is_active", "oneof", "ステータスが不正です"},
		{"name", "unknown_tag", "入力内容を確認してください"}, // フォールバック
	}
	for _, tt := range tests {
		if got := deviceValidationMessage(tt.field, tt.tag); got != tt.want {
			t.Errorf("deviceValidationMessage(%q, %q) = %q, want %q", tt.field, tt.tag, got, tt.want)
		}
	}
}

func TestDeviceFieldKey(t *testing.T) {
	tests := []struct {
		structField, want string
	}{
		{"Name", "name"},
		{"MacAddress", "mac_address"},
		{"Location", "location"},
		{"IsActive", "is_active"},
		{"Unknown", "unknown"}, // フォールバック: 未知フィールドは小文字化
	}
	for _, tt := range tests {
		if got := deviceFieldKey(tt.structField); got != tt.want {
			t.Errorf("deviceFieldKey(%q) = %q, want %q", tt.structField, got, tt.want)
		}
	}
}
