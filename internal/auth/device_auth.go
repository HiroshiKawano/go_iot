// Package auth は認証関連のミドルウェアを提供する。
// device_auth.go はデバイスAPI用の Bearer トークン認証を実装する。
// (Web UI 用 Session 認証は将来 session_auth.go を追加する予定)
package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HiroshiKawano/go_iot/internal/infra/token"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

const (
	ctxKeyUserID        = "device_auth_user_id"
	ctxKeyDeviceTokenID = "device_auth_token_id"
)

// DeviceAuthConfig は DeviceAuth ミドルウェアの依存を保持する。
type DeviceAuthConfig struct {
	Repo *repository.Queries
}

// DeviceAuth は Authorization: Bearer <token> を検証し、成功した場合に
// user_id / token_id を Echo コンテキストに格納する。
//
// エラーの HTTP ステータス:
//   - 401: ヘッダ欠如 / Bearer 以外 / トークン不一致 / 期限切れ
//   - 500: DB 参照エラー
func DeviceAuth(cfg DeviceAuthConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing Authorization header")
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(authHeader, prefix) {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid Authorization scheme (Bearer required)")
			}
			plaintext := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
			if plaintext == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "empty Bearer token")
			}

			hash := token.Hash(plaintext)
			tok, err := cfg.Repo.GetDeviceTokenByHash(c.Request().Context(), hash)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "token lookup failed")
			}

			// last_used_at の更新は認証の必須要件ではないためエラーは無視
			_ = cfg.Repo.UpdateDeviceTokenLastUsed(c.Request().Context(), tok.ID)

			c.Set(ctxKeyUserID, tok.UserID)
			c.Set(ctxKeyDeviceTokenID, tok.ID)
			return next(c)
		}
	}
}

// UserID は DeviceAuth 成功後の Echo コンテキストから user_id を取得する。
func UserID(c echo.Context) int64 {
	v, _ := c.Get(ctxKeyUserID).(int64)
	return v
}

// DeviceTokenID は DeviceAuth 成功後の Echo コンテキストから token_id を取得する。
func DeviceTokenID(c echo.Context) int64 {
	v, _ := c.Get(ctxKeyDeviceTokenID).(int64)
	return v
}
