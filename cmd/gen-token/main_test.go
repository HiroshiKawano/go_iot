package main

import (
	"testing"
	"time"
)

// TestBuildExpiresAt は有効期間(日)→ *time.Time(NULL=無期限)写像を検証する。
// SQLite 移行で expires_at の生成型が pgtype.Timestamptz → *time.Time へ変わった追従の単体ガード。
func TestBuildExpiresAt(t *testing.T) {
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)

	t.Run("0は無期限(nil)", func(t *testing.T) {
		if got := buildExpiresAt(0, now); got != nil {
			t.Errorf("buildExpiresAt(0) = %v, want nil (無期限)", got)
		}
	})
	t.Run("負値も無期限(nil)", func(t *testing.T) {
		if got := buildExpiresAt(-5, now); got != nil {
			t.Errorf("buildExpiresAt(-5) = %v, want nil", got)
		}
	})
	t.Run("正値はnow+N日", func(t *testing.T) {
		got := buildExpiresAt(30, now)
		if got == nil {
			t.Fatal("buildExpiresAt(30) = nil, want non-nil")
		}
		want := now.Add(30 * 24 * time.Hour)
		if !got.Equal(want) {
			t.Errorf("buildExpiresAt(30) = %v, want %v", got, want)
		}
	})
}
