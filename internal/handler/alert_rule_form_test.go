package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

// bindAlertRuleForm は gin の ShouldBind と同じ tag 名 (binding) で alertRuleForm を検証する。
// validator.New() の既定 tag は "validate" のため SetTagName("binding") が必須 (§25.1 の罠)。
func bindAlertRuleForm(f alertRuleForm) error {
	v := validator.New()
	v.SetTagName("binding")
	return v.Struct(f)
}

// --- parseThreshold 単体 (4.1: "0"=妥当 / 非数値 / 範囲外 / 境界) ---

func TestParseThreshold(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    float64
		wantErr error
	}{
		{"ゼロは妥当", "0", 0, nil},
		{"正の小数", "35.00", 35.00, nil},
		{"負値(霜害アラート等)", "-5.5", -5.5, nil},
		{"上限境界", "999.99", 999.99, nil},
		{"下限境界", "-999.99", -999.99, nil},
		{"前後空白を許容", " 12.5 ", 12.5, nil},
		{"非数値", "abc", 0, errThresholdNotNumeric},
		{"範囲外(正)", "1000", 0, errThresholdOutOfRange},
		{"範囲外(負)", "-1000", 0, errThresholdOutOfRange},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseThreshold(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("parseThreshold(%q) err = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseThreshold(%q) 予期せぬ err: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseThreshold(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- binding (metric/operator/threshold の required+oneof) 単体 (4.1: 許容外の指標・条件) ---

func TestAlertRuleForm_binding(t *testing.T) {
	valid := alertRuleForm{Metric: "temperature", Operator: ">", Threshold: "35.00"}
	if err := bindAlertRuleForm(valid); err != nil {
		t.Fatalf("有効フォームで検証エラー: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*alertRuleForm)
		field  string // 期待する ValidationError の Go フィールド名
		tag    string
	}{
		{"指標未選択", func(f *alertRuleForm) { f.Metric = "" }, "Metric", "required"},
		{"指標が許容外", func(f *alertRuleForm) { f.Metric = "pressure" }, "Metric", "oneof"},
		{"条件未選択", func(f *alertRuleForm) { f.Operator = "" }, "Operator", "required"},
		{"条件が許容外", func(f *alertRuleForm) { f.Operator = "==" }, "Operator", "oneof"},
		{"閾値空", func(f *alertRuleForm) { f.Threshold = "" }, "Threshold", "required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := valid
			tt.mutate(&f)
			err := bindAlertRuleForm(f)
			var ve validator.ValidationErrors
			if !errors.As(err, &ve) {
				t.Fatalf("ValidationErrors を期待: %v", err)
			}
			found := false
			for _, fe := range ve {
				if fe.Field() == tt.field && fe.Tag() == tt.tag {
					found = true
				}
			}
			if !found {
				t.Errorf("field=%q tag=%q のエラーが見つからない: %v", tt.field, tt.tag, ve)
			}
		})
	}
}

// --- validateAlertRuleForm 統合 (4.1: 0=妥当 / 複数同時 / バインドとパースの統合) ---

func TestValidateAlertRuleForm_有効入力は閾値を返しエラーなし(t *testing.T) {
	form := alertRuleForm{Metric: "temperature", Operator: ">", Threshold: "0"}
	th, errs := validateAlertRuleForm(form, bindAlertRuleForm(form))
	if len(errs) != 0 {
		t.Fatalf("0 は妥当だがエラー: %v", errs)
	}
	if th != 0 {
		t.Errorf("threshold: got %v, want 0", th)
	}
}

func TestValidateAlertRuleForm_全項目未入力で全エラーが揃う(t *testing.T) {
	form := alertRuleForm{Metric: "", Operator: "", Threshold: ""}
	_, errs := validateAlertRuleForm(form, bindAlertRuleForm(form))
	for _, key := range []string{"metric", "operator", "threshold"} {
		if errs[key] == "" {
			t.Errorf("%q のエラーが揃っていない: %v", key, errs)
		}
	}
}

func TestValidateAlertRuleForm_指標エラーと閾値非数値が同時に載る(t *testing.T) {
	// metric 未選択 (binding) と threshold 非数値 (parse) を同時に検出する (要件 8.5)。
	form := alertRuleForm{Metric: "", Operator: ">", Threshold: "abc"}
	_, errs := validateAlertRuleForm(form, bindAlertRuleForm(form))
	if errs["metric"] == "" {
		t.Errorf("metric エラーが無い: %v", errs)
	}
	if errs["threshold"] == "" {
		t.Errorf("threshold 非数値エラーが無い: %v", errs)
	}
}

func TestValidateAlertRuleForm_範囲外閾値はエラー(t *testing.T) {
	form := alertRuleForm{Metric: "temperature", Operator: ">", Threshold: "1000"}
	_, errs := validateAlertRuleForm(form, bindAlertRuleForm(form))
	if errs["threshold"] == "" {
		t.Errorf("範囲外 threshold エラーが無い: %v", errs)
	}
}

// --- メッセージ翻訳層 (§25.6/§41.5: 文言完全一致・生フィールド名を露出しない) ---
// device_form_test.go の翻訳層テストと対称。alertRuleValidationMessage / alertRuleFieldKey /
// toAlertRuleFieldErrors を直接固定し、翻訳マップへの追加漏れ・タグずれ・生フィールド名露出を回帰検出する。

func TestAlertRuleValidationMessage(t *testing.T) {
	tests := []struct {
		field, tag, want string
	}{
		{"metric", "required", "指標を選択してください"},
		{"metric", "oneof", "指標の値が不正です"},
		{"operator", "required", "条件を選択してください"},
		{"operator", "oneof", "条件の値が不正です"},
		{"threshold", "required", "閾値を入力してください"},
		{"metric", "unknown_tag", "入力内容を確認してください"},     // タグ未定義はフォールバック
		{"unknown_field", "required", "入力内容を確認してください"}, // フィールド未定義はフォールバック
	}
	for _, tt := range tests {
		if got := alertRuleValidationMessage(tt.field, tt.tag); got != tt.want {
			t.Errorf("alertRuleValidationMessage(%q, %q) = %q, want %q", tt.field, tt.tag, got, tt.want)
		}
	}
}

func TestAlertRuleFieldKey(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Metric", "metric"},
		{"Operator", "operator"},
		{"Threshold", "threshold"},
		{"DeviceID", "device_id"},
		{"Unknown", "unknown"}, // 既定は ToLower フォールバック
	}
	for _, tt := range tests {
		if got := alertRuleFieldKey(tt.in); got != tt.want {
			t.Errorf("alertRuleFieldKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestToAlertRuleFieldErrors_各タグが対応する日本語になる は
// binding 失敗を項目別の日本語メッセージ (完全一致) へ変換することを確認する。
func TestToAlertRuleFieldErrors_各タグが対応する日本語になる(t *testing.T) {
	valid := alertRuleForm{Metric: "temperature", Operator: ">", Threshold: "35.00"}
	tests := []struct {
		name    string
		mutate  func(*alertRuleForm)
		wantKey string
		wantMsg string
	}{
		{"指標未選択は必須", func(f *alertRuleForm) { f.Metric = "" }, "metric", "指標を選択してください"},
		{"指標許容外はoneof", func(f *alertRuleForm) { f.Metric = "pressure" }, "metric", "指標の値が不正です"},
		{"条件未選択は必須", func(f *alertRuleForm) { f.Operator = "" }, "operator", "条件を選択してください"},
		{"条件許容外はoneof", func(f *alertRuleForm) { f.Operator = "==" }, "operator", "条件の値が不正です"},
		{"閾値空は必須", func(f *alertRuleForm) { f.Threshold = "" }, "threshold", "閾値を入力してください"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := valid
			tt.mutate(&f)
			err := bindAlertRuleForm(f)
			if err == nil {
				t.Fatal("検証は失敗するはずが通過した")
			}
			got := toAlertRuleFieldErrors(err)
			if got[tt.wantKey] != tt.wantMsg {
				t.Errorf("toAlertRuleFieldErrors()[%q] = %q, want %q", tt.wantKey, got[tt.wantKey], tt.wantMsg)
			}
		})
	}
}

// TestToAlertRuleFieldErrors_生フィールド名を露出しない は (§25.6/§41.5)
// 422 へ渡る map のキーがフォーム名 (小文字) であり、Go フィールド名 (Metric 等) や
// validator の Namespace パス (alertRuleForm.Metric) が混ざらないこと、
// 個別文言が定義済み (フォールバック素通りでない) ことを固定する。
func TestToAlertRuleFieldErrors_生フィールド名を露出しない(t *testing.T) {
	err := bindAlertRuleForm(alertRuleForm{Metric: "", Operator: "", Threshold: ""})
	got := toAlertRuleFieldErrors(err)
	if len(got) == 0 {
		t.Fatal("全項目未入力でエラーが空")
	}
	for key, msg := range got {
		// キーはフォーム名 (metric/operator/threshold)。Namespace パス・大文字 Go 名は禁止。
		if strings.ContainsAny(key, ".[]") || key != strings.ToLower(key) {
			t.Errorf("キーに生フィールドパス/大文字が露出: %q", key)
		}
		// メッセージにも Go フィールド名・構造体名が混ざらない。
		for _, raw := range []string{"alertRuleForm", "Metric", "Operator", "Threshold"} {
			if strings.Contains(msg, raw) {
				t.Errorf("メッセージに生フィールド名 %q が露出: key=%q msg=%q", raw, key, msg)
			}
		}
		// フォールバック素通り (個別文言の定義漏れ) でないこと。
		if msg == "入力内容を確認してください" {
			t.Errorf("key=%q がフォールバック文言のまま (個別定義が漏れている)", key)
		}
	}
}

// TestToAlertRuleFieldErrors_検証以外のエラーはフォーム全体メッセージ は
// validator.ValidationErrors でないエラーが "form" キーの汎用メッセージになることを確認する。
func TestToAlertRuleFieldErrors_検証以外のエラーはフォーム全体メッセージ(t *testing.T) {
	got := toAlertRuleFieldErrors(errors.New("予期せぬエラー"))
	if got["form"] != "入力内容を確認してください" {
		t.Errorf("検証以外のエラーは form キーに汎用メッセージを入れるべき: %v", got)
	}
}
