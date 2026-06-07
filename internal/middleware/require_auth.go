package middleware

import (
	"net/http"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/gin-gonic/gin"
)

// RequireAuth は認証必須ルート用のガード。
// 未認証 (auth.UserID(c) <= 0) の場合は /login へリダイレクトして処理を中断する。
// user_id は先行する SessionLoad が橋渡しした値を参照する。
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.UserID(c) <= 0 {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}
