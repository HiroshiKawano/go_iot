package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/desktop"
)

// startServer は確定済み listener で Serve を開始し、実ポートをログ出力後に
// 既定ブラウザで localhost URL を開く (R5.1/5.4)。実ポートが起動ログに出ること、
// ブラウザが実ポートの URL で開かれることを openURL スタブで検証する。
func TestStartServer_LogsPortAndOpensBrowser(t *testing.T) {
	ln, port, err := desktop.Listen(0) // 空きポートを採番
	if err != nil {
		t.Fatalf("desktop.Listen: %v", err)
	}
	srv := &http.Server{Handler: http.NotFoundHandler()}
	t.Cleanup(func() { _ = srv.Close() })

	var gotURL string
	openURL := func(u string) error {
		gotURL = u
		return nil
	}

	var logBuf bytes.Buffer
	origOut := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(origOut) })

	errCh := startServer(srv, ln, port, openURL)
	if errCh == nil {
		t.Fatal("startServer は errCh を返すべき")
	}

	wantURL := fmt.Sprintf("http://localhost:%d", port)
	if gotURL != wantURL {
		t.Errorf("ブラウザ URL = %q, want %q", gotURL, wantURL)
	}
	if !strings.Contains(logBuf.String(), strconv.Itoa(port)) {
		t.Errorf("実ポート %d が起動ログに出ていない: %q", port, logBuf.String())
	}
}

// ブラウザ起動失敗は非致命: サーバ起動 (errCh) は継続する (R5.1)。
func TestStartServer_BrowserFailureIsNonFatal(t *testing.T) {
	ln, port, err := desktop.Listen(0)
	if err != nil {
		t.Fatalf("desktop.Listen: %v", err)
	}
	srv := &http.Server{Handler: http.NotFoundHandler()}
	t.Cleanup(func() { _ = srv.Close() })

	openURL := func(string) error { return errors.New("ブラウザが見つからない") }

	errCh := startServer(srv, ln, port, openURL)
	if errCh == nil {
		t.Fatal("startServer は errCh を返すべき")
	}

	// ブラウザ失敗後もサーバが応答する (= 継続起動している) ことを実証する。
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", port))
	if err != nil {
		t.Fatalf("ブラウザ失敗後もサーバは応答すべき (非致命): %v", err)
	}
	_ = resp.Body.Close()

	// ブラウザ失敗が致命化して errCh へ流れていないことを確認する。
	select {
	case err := <-errCh:
		t.Errorf("ブラウザ失敗は非致命のはずだが errCh にエラーが流れた: %v", err)
	default:
	}
}
