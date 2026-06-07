package middleware

import (
	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

// SessionLoad は scs セッションのログインユーザー ID を Gin コンテキストへ橋渡しする。
// これにより device 認証 (Bearer) と session 認証 (cookie) の双方が、
// ダウンストリームで auth.UserID(c) により統一的に user_id を参照できる。
//
// 前提: scs の LoadAndSave が http.Handler 層で既にセッションを読み込んでいること
// (本ミドルウェアはロード済みセッションの値を Gin コンテキストへ写すだけ)。
func SessionLoad(sm *scs.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uid := auth.UserIDFromSession(c.Request.Context(), sm); uid > 0 {
			auth.SetUserID(c, uid)
		}
		c.Next()
	}
}
