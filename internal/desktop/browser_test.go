package desktop

import (
	"errors"
	"io"
	"testing"

	"github.com/cli/browser"
)

// OpenBrowser は GUI ログ汚染防止のため browser の出力を io.Discard に向け、
// 指定 URL を既定ブラウザ起動関数へ委譲する (Decision 3)。
// 実ブラウザを起動しないよう、委譲先 (browserOpener) をスタブに差し替えて検証する。
func TestOpenBrowser_DiscardsOutputAndDelegates(t *testing.T) {
	orig := browserOpener
	t.Cleanup(func() { browserOpener = orig })

	var gotURL string
	browserOpener = func(u string) error {
		gotURL = u
		return nil
	}

	const url = "http://localhost:8080"
	if err := OpenBrowser(url); err != nil {
		t.Fatalf("OpenBrowser() エラー: %v", err)
	}
	if gotURL != url {
		t.Errorf("委譲 URL = %q, want %q", gotURL, url)
	}
	if browser.Stdout != io.Discard {
		t.Error("browser.Stdout を io.Discard にすべき (GUI ログ汚染防止)")
	}
	if browser.Stderr != io.Discard {
		t.Error("browser.Stderr を io.Discard にすべき (GUI ログ汚染防止)")
	}
}

// ブラウザ起動失敗時は error を返し、呼び出し側が非致命として継続できるようにする (R5.1)。
func TestOpenBrowser_ReturnsErrorOnFailure(t *testing.T) {
	orig := browserOpener
	t.Cleanup(func() { browserOpener = orig })

	browserOpener = func(string) error { return errors.New("ブラウザが見つからない") }

	if err := OpenBrowser("http://localhost:8080"); err == nil {
		t.Error("ブラウザ起動失敗時は error を返すべき (呼び出し側が非致命処理できる)")
	}
}
