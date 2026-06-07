package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/alexedwards/scs/v2"
)

// loadedCtx は in-memory の scs セッションを載せた context を返すテストヘルパー。
func loadedCtx(t *testing.T, sm *scs.SessionManager) context.Context {
	t.Helper()
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("session load: %v", err)
	}
	return ctx
}

func TestLogin_セッションにuser_idを保存しトークンを発行(t *testing.T) {
	sm := scs.New()
	ctx := loadedCtx(t, sm)

	if err := Login(ctx, sm, 42, false); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := UserIDFromSession(ctx, sm); got != 42 {
		t.Errorf("UserIDFromSession after login: got %d, want 42", got)
	}
	if sm.Token(ctx) == "" {
		t.Error("Login 後にセッショントークンが発行されていない (RenewToken 未実行)")
	}
}

func TestLogin_rememberを指定してもuser_idが保存される(t *testing.T) {
	sm := scs.New()
	ctx := loadedCtx(t, sm)

	if err := Login(ctx, sm, 7, true); err != nil {
		t.Fatalf("Login(remember=true): %v", err)
	}
	if got := UserIDFromSession(ctx, sm); got != 7 {
		t.Errorf("UserIDFromSession: got %d, want 7", got)
	}
}

func TestLogout_セッションを破棄してuser_idが0になる(t *testing.T) {
	sm := scs.New()
	ctx := loadedCtx(t, sm)
	if err := Login(ctx, sm, 42, false); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if err := Logout(ctx, sm); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if got := UserIDFromSession(ctx, sm); got != 0 {
		t.Errorf("UserIDFromSession after logout: got %d, want 0", got)
	}
}

func TestUserIDFromSession_未設定なら0(t *testing.T) {
	sm := scs.New()
	ctx := loadedCtx(t, sm)
	if got := UserIDFromSession(ctx, sm); got != 0 {
		t.Errorf("UserIDFromSession unset: got %d, want 0", got)
	}
}

func TestApplySessionPolicy_本番はSecureCookie(t *testing.T) {
	sm := scs.New()
	applySessionPolicy(sm, &config.Config{AppEnv: "production"})

	if !sm.Cookie.Secure {
		t.Error("production では Cookie.Secure=true であるべき")
	}
	if !sm.Cookie.HttpOnly {
		t.Error("Cookie.HttpOnly=true であるべき")
	}
	if sm.Cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("Cookie.SameSite = %v, want Lax", sm.Cookie.SameSite)
	}
	if sm.Cookie.Persist {
		t.Error("Cookie.Persist=false であるべき (remember 時のみ永続化)")
	}
}

func TestApplySessionPolicy_開発はSecure無効(t *testing.T) {
	sm := scs.New()
	applySessionPolicy(sm, &config.Config{AppEnv: "development"})

	if sm.Cookie.Secure {
		t.Error("development では Cookie.Secure=false であるべき (HTTP localhost)")
	}
}
