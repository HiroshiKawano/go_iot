// Package auth は認証関連のミドルウェアを提供する。
// device_auth.go はデバイスAPI用の Bearer トークン認証を実装する。
// (Web UI 用 Session 認証は将来 session_auth.go を追加する予定)
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/HiroshiKawano/go_iot/internal/infra/token"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

const (
	ctxKeyUserID        = "device_auth_user_id"
	ctxKeyDeviceTokenID = "device_auth_token_id"
)

// TokenRepo は DeviceAuth が必要とする最小の DB ポート (consumer interface)。
// repository.Querier / *repository.Queries はこれを満たす。最小メソッドに限定することで
// テスト時のモックを小さく保つ (DIP / アーキ決定の「consumer 最小 interface」)。
type TokenRepo interface {
	GetDeviceTokenByHash(ctx context.Context, tokenHash string) (repository.DeviceToken, error)
	UpdateDeviceTokenLastUsed(ctx context.Context, id int64) error
}

// DeviceAuthConfig は DeviceAuth ミドルウェアの依存を保持する。
type DeviceAuthConfig struct {
	Repo TokenRepo
}

// DeviceAuth は Authorization: Bearer <token> を検証し、成功した場合に
// user_id / token_id を Gin コンテキストに格納する。
//
// エラーの HTTP ステータス:
//   - 401: ヘッダ欠如 / Bearer 以外 / トークン不一致 / 期限切れ
//   - 500: DB 参照エラー
func DeviceAuth(cfg DeviceAuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "missing Authorization header"})
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid Authorization scheme (Bearer required)"})
			return
		}
		plaintext := strings.TrimSpace(strings.TrimPrefix(authHeader, prefix))
		if plaintext == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "empty Bearer token"})
			return
		}

		hash := token.Hash(plaintext)
		tok, err := cfg.Repo.GetDeviceTokenByHash(c.Request.Context(), hash)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid or expired token"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "token lookup failed"})
			return
		}

		// last_used_at の更新は認証の必須要件ではないためエラーは無視
		_ = cfg.Repo.UpdateDeviceTokenLastUsed(c.Request.Context(), tok.ID)

		SetUserID(c, tok.UserID)
		c.Set(ctxKeyDeviceTokenID, tok.ID)
		c.Next()
	}
}

// SetUserID は認証成功後の user_id を Gin コンテキストへ格納する。
// DeviceAuth(本パッケージ) と将来の Session 認証が共通で使い、ダウンストリームは
// 認証方式に依らず UserID(c) で取得できる。テストの認可経路セットアップにも使う。
func SetUserID(c *gin.Context, id int64) {
	c.Set(ctxKeyUserID, id)
}

// UserID は DeviceAuth 成功後の Gin コンテキストから user_id を取得する。
func UserID(c *gin.Context) int64 {
	v, _ := c.Get(ctxKeyUserID)
	id, _ := v.(int64)
	return id
}

// DeviceTokenID は DeviceAuth 成功後の Gin コンテキストから token_id を取得する。
func DeviceTokenID(c *gin.Context) int64 {
	v, _ := c.Get(ctxKeyDeviceTokenID)
	id, _ := v.(int64)
	return id
}
