// Package view は templ レイアウト・ページ・コンポーネントと、
// それらが参照する静的アセット (CSS / JS) の配信を担う。
package view

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

// publicFS は配信用の静的アセットを埋め込む。
// CSS の正本は mocks/html/style.css で、make sync-css が
// internal/view/public/css/style.css へ複製する (生成物・手編集禁止)。
// go:embed は親ディレクトリ (..) を辿れないため public は本パッケージ配下に置く。
//
//go:embed all:public
var publicFS embed.FS

// Version は配信アセットのキャッシュバスティング用バージョン。
// ビルド時に -ldflags "-X github.com/HiroshiKawano/go_iot/internal/view.Version=<sha>"
// で上書きする。未指定時は "dev"。
var Version = "dev"

// CSSURL は共通スタイルシートのバージョンクエリ付き URL を返す。
// templ レイアウトの <link href> に渡してキャッシュバスティングする。
func CSSURL() string {
	return "/static/css/style.css?v=" + Version
}

// JSURL は自サーバ配信する ECharts スクリプトのバージョンクエリ付き URL を返す。
// App.templ の <head> に <script src> として 1 回だけ渡してキャッシュバスティングする
// (外部 CDN 非依存・self-host)。実体は public/js/echarts.min.js (ECharts 5.4.3)。
func JSURL() string {
	return "/static/js/echarts.min.js?v=" + Version
}

// MountStatic は埋め込んだ public 配下を /static で配信する。
// 例: /static/css/style.css。
func MountStatic(r *gin.Engine) {
	sub, err := fs.Sub(publicFS, "public")
	if err != nil {
		// embed パスは静的に決まるため、ここに到達するのはビルド構成の不整合のみ。
		panic("view: embed public のサブツリー取得に失敗: " + err.Error())
	}
	r.StaticFS("/static", http.FS(sub))
}
