package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureMethod は ServeHTTP 時の r.Method を記録するハンドラを返す。
func captureMethod(dst *string) http.Handler {
	return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		*dst = r.Method
	})
}

func postForm(h http.Handler, body string) {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMethodOverride(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"DELETE に上書き", "_method=DELETE", http.MethodDelete},
		{"PUT に上書き", "_method=PUT", http.MethodPut},
		{"PATCH に上書き", "_method=PATCH", http.MethodPatch},
		{"小文字も大文字化して上書き", "_method=delete", http.MethodDelete},
		{"_method 無しは POST のまま", "name=foo", http.MethodPost},
		{"GET への上書きは許可せず POST のまま", "_method=GET", http.MethodPost},
		{"未知の値は POST のまま", "_method=FOO", http.MethodPost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := MethodOverride(captureMethod(&got))
			postForm(h, tt.body)
			if got != tt.want {
				t.Errorf("r.Method = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMethodOverride_GETはフォームを解析せず素通し(t *testing.T) {
	var got string
	h := MethodOverride(captureMethod(&got))
	req := httptest.NewRequest(http.MethodGet, "/x?_method=DELETE", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got != http.MethodGet {
		t.Errorf("GET は上書きされないはず: got %q", got)
	}
}
