package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// newRouterWithAuth は DeviceAuth を適用した最小ルータを返すヘルパー。
// Repo は nil で、ヘッダ検証のみを行う経路をテストする。
func newRouterWithAuth() *gin.Engine {
	r := gin.New()
	r.GET("/protected", DeviceAuth(DeviceAuthConfig{Repo: nil}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestDeviceAuth_MissingAuthorizationHeader(t *testing.T) {
	r := newRouterWithAuth()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestDeviceAuth_InvalidScheme(t *testing.T) {
	r := newRouterWithAuth()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestDeviceAuth_EmptyBearerToken(t *testing.T) {
	r := newRouterWithAuth()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer    ")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
}

func TestUserID_ReturnsZeroWhenUnset(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if got := UserID(c); got != 0 {
		t.Errorf("UserID without set: got %d, want 0", got)
	}
}

func TestUserID_ReturnsStoredValue(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxKeyUserID, int64(42))
	if got := UserID(c); got != 42 {
		t.Errorf("UserID: got %d, want 42", got)
	}
}

func TestDeviceTokenID_ReturnsStoredValue(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ctxKeyDeviceTokenID, int64(7))
	if got := DeviceTokenID(c); got != 7 {
		t.Errorf("DeviceTokenID: got %d, want 7", got)
	}
}
