package middleware

import (
	"crypto/sha256"
	"net/http"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
)

// StatusCSRFExpired は CSRF トークン検証失敗時に返すステータスコード。
// 419 (Page Expired) は HTTP 標準外 (Laravel 由来の慣習値) だが、本プロジェクトは
// BOLA 認可拒否でも 403 を使う (renderError(c, http.StatusForbidden)) ため、フロントが
// 「CSRF 失敗 (再読込/再ログイン誘導)」と「認可拒否 (誘導しない)」を区別できるよう
// CSRF 失敗を 419 へ分離する (2cc_sdd/HTMX実装ガイド(動的).md §14 補完②)。
const StatusCSRFExpired = 419

// csrfAuthKey は SESSION_SECRET から gorilla/csrf 用の 32 バイト認証鍵を導出する。
// gorilla/csrf は鍵長 32 バイトを要求するため、任意長の secret を SHA-256 で
// 固定長へ畳み込む (開発環境で 32 文字未満でも安全に動作させるため)。
func csrfAuthKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// CSRF は gorilla/csrf を Gin ミドルウェアに適応する。
// 状態変更リクエスト (POST/PUT/PATCH/DELETE) に有効な CSRF トークンを要求し、
// 欠落・不正なら 419 (StatusCSRFExpired) を返す。トークンはフォーム値 (gorilla.csrf.Token) と
// ヘッダ (X-CSRF-Token) の双方から検証される (HTMX はヘッダで送る)。
//
// デバイス取込 API (/api・Bearer) には適用しないため、Web ルートグループ限定で使う。
//
// gorilla/csrf は既定でリクエストを HTTPS とみなし Origin/Referer の同一オリジン検証を
// 行う。開発環境 (HTTP) ではこの検証が常に失敗するため、本番以外では
// PlaintextHTTPRequest でリクエストを平文 HTTP として明示し検証を緩和する。
// 本番 (HTTPS) ではブラウザが送る Origin/Referer により同一オリジン検証が機能する。
func CSRF(cfg *config.Config) gin.HandlerFunc {
	isProd := cfg.AppEnv == "production"
	protect := csrf.Protect(
		csrfAuthKey(cfg.SessionSecret),
		csrf.Secure(isProd), // 開発 (HTTP) では Secure cookie を無効化
		csrf.Path("/"),
		csrf.SameSite(csrf.SameSiteLaxMode),
		// CSRF 失敗は gorilla/csrf 既定の 403 ではなく 419 で返し、BOLA 認可拒否 (403) と分離する。
		// HTMX はフロントの 419 分岐 (App.templ) がトースト表示、非 HTMX フルページ送信は
		// ログイン誘導の本文を 419 で返す (素の 403 本文を見せない)。
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("HX-Request") != "" {
				w.WriteHeader(StatusCSRFExpired) // HTMX: 本文なし
				return
			}
			w.WriteHeader(StatusCSRFExpired)
			_, _ = w.Write([]byte("セッションが切れました。ログインし直してください。"))
		})),
	)

	return func(c *gin.Context) {
		var passed bool
		// gorilla/csrf は検証成功時のみ内側ハンドラを呼ぶ。失敗時は ErrorHandler が 419 を書く。
		wrapped := protect(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			passed = true
			c.Request = r // csrf トークンを載せた context を後段ハンドラへ伝播
			c.Next()
		}))

		req := c.Request
		if !isProd {
			req = csrf.PlaintextHTTPRequest(req) // 開発 (HTTP) の Origin/Referer 強制を回避
		}
		wrapped.ServeHTTP(c.Writer, req)
		if !passed {
			c.Abort() // 419 は ErrorHandler が書き込み済み
		}
	}
}
