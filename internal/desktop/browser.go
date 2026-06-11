package desktop

import (
	"io"

	"github.com/cli/browser"
)

// browserOpener は既定ブラウザで URL を開く関数。差し替え可能にしてテストで実ブラウザ起動を避ける。
// 既定は github.com/cli/browser.OpenURL (pure-Go・OS 別 exec・組込ランタイム不要の通常ブラウザ)。
var browserOpener = browser.OpenURL

// OpenBrowser は既定ブラウザで url を開く (Decision 3)。
//
// 組込ブラウザランタイムを必要としない通常ブラウザを起動する (R5.2)。GUI ビルドでは子プロセスの
// 標準出力/エラーがログを汚染しうるため io.Discard へ捨てる。起動失敗は致命でなく、error を返すのみで
// 呼び出し側 (cmd/server) はログ提示して継続する (R5.1)。
func OpenBrowser(url string) error {
	browser.Stdout = io.Discard
	browser.Stderr = io.Discard
	return browserOpener(url)
}
