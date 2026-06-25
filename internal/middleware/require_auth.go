package middleware

import (
	"net/http"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/gin-gonic/gin"
)

// RequireAuth は認証必須ルート用のガード。
// 未認証 (auth.UserID(c) <= 0) の場合、通常ナビゲーションは /login へリダイレクトし、
// HTMX リクエスト (HX-Request ヘッダ有) は本文なし 401 を返して処理を中断する。
// user_id は先行する SessionLoad が橋渡しした値を参照する。
//
// HTMX 経路を 302 のままにすると、htmx が 302 を辿りログイン画面の HTML を部分領域へ
// swap してしまい画面が壊れる。本文なし 401 にすることで、フロント (App.templ の
// htmx:responseError 401 分岐) がトースト通知＋/login への遷移を担う
// (詳細は 2cc_sdd/HTMX実装ガイド(動的).md §14 補完①)。
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.UserID(c) <= 0 {
			if c.GetHeader("HX-Request") != "" {
				c.Status(http.StatusUnauthorized) // 本文なし 401 (フロントが toast + /login 遷移)
				c.Abort()
				return
			}
			c.Redirect(http.StatusFound, "/login") // 通常ナビゲーションは従来どおり 302
			c.Abort()
			return
		}
		c.Next()
	}
}
