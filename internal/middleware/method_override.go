// Package middleware は Web UI の横断的ミドルウェア
// (セッション読込・認証ガード・CSRF・HTTP メソッド上書き) を提供する。
package middleware

import (
	"net/http"
	"strings"
)

// allowedOverrides は隠しフィールド _method による上書きを許可する HTTP メソッド。
var allowedOverrides = map[string]bool{
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

// MethodOverride は POST フォームの隠しフィールド _method を読み取り、
// 値が PUT/PATCH/DELETE のとき r.Method を書き換える http.Handler ミドルウェア。
//
// Gin はミドルウェア実行前に HTTP メソッドでルーティングを解決するため、
// メソッド上書きは gin.Engine の外側 (ルーティング前) で適用する必要がある。
// そのため gin.HandlerFunc ではなく net/http レベルのラッパとして実装する。
//
// 注意: _method は form 値のため PostFormValue が body を解析する。
// urlencoded の場合 ParseForm の結果はキャッシュされ、後段の form binding でも読める。
func MethodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if override := strings.ToUpper(r.PostFormValue("_method")); allowedOverrides[override] {
				r.Method = override
			}
		}
		next.ServeHTTP(w, r)
	})
}
