package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
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

// --- DB 経路 (トークン照合) のテスト ---

// fakeTokenRepo は TokenRepo の最小モック。
type fakeTokenRepo struct {
	tok    repository.DeviceToken
	getErr error
}

func (f fakeTokenRepo) GetDeviceTokenByHash(_ context.Context, _ string) (repository.DeviceToken, error) {
	return f.tok, f.getErr
}

func (f fakeTokenRepo) UpdateDeviceTokenLastUsed(_ context.Context, _ int64) error {
	return nil
}

// newRouterRepo は指定した Repo で DeviceAuth を適用し、成功時に user_id を JSON で返す。
func newRouterRepo(repo TokenRepo) *gin.Engine {
	r := gin.New()
	r.GET("/protected", DeviceAuth(DeviceAuthConfig{Repo: repo}), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": UserID(c)})
	})
	return r
}

func getProtected(r *gin.Engine, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestDeviceAuth_有効なトークンは200でuser_idを格納(t *testing.T) {
	repo := fakeTokenRepo{tok: repository.DeviceToken{ID: 3, UserID: 42}}
	w := getProtected(newRouterRepo(repo), "valid-token")

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"user_id":42`) {
		t.Errorf("user_id が格納されていない: body=%s", w.Body.String())
	}
}

func TestDeviceAuth_不正トークンは401(t *testing.T) {
	repo := fakeTokenRepo{getErr: pgx.ErrNoRows}
	w := getProtected(newRouterRepo(repo), "bad-token")

	if got, want := w.Code, http.StatusUnauthorized; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}

func TestDeviceAuth_DB参照エラーは500(t *testing.T) {
	repo := fakeTokenRepo{getErr: errors.New("db down")}
	w := getProtected(newRouterRepo(repo), "some-token")

	if got, want := w.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status: got %d, want %d, body=%s", got, want, w.Body.String())
	}
}
