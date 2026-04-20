package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// newRouter は SensorAPI.Create を単独で呼ぶルータを組み立てる。
// 認証は通した前提でテストするため DeviceAuth は挟まない。
func newRouter(h *SensorAPI) *gin.Engine {
	r := gin.New()
	r.POST("/api/sensor-data", h.Create)
	return r
}

func TestSensorAPI_Create_InvalidJSONSyntax(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader("{invalid-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

func TestSensorAPI_Create_TypeMismatchReturns400(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	// device_id に文字列を入れて UnmarshalTypeError を誘発
	body := `{"device_id":"not-a-number","temperature":20,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestSensorAPI_Create_ValidationFailureReturns422(t *testing.T) {
	h := &SensorAPI{}
	r := newRouter(h)

	cases := []struct {
		name string
		body string
	}{
		{
			name: "temperature below lower bound",
			body: `{"device_id":1,"temperature":-100,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "temperature above upper bound",
			body: `{"device_id":1,"temperature":200,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "humidity above 100",
			body: `{"device_id":1,"temperature":20,"humidity":150,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "device_id missing",
			body: `{"temperature":20,"humidity":50,"recorded_at":"2026-04-21T00:00:00Z"}`,
		},
		{
			name: "recorded_at missing",
			body: `{"device_id":1,"temperature":20,"humidity":50}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/sensor-data", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if got, want := w.Code, http.StatusUnprocessableEntity; got != want {
				t.Errorf("status code: got %d, want %d, body=%s", got, want, w.Body.String())
			}
		})
	}
}
