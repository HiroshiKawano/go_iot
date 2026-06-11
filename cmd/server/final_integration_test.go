package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/migrate"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/gin-gonic/gin"
)

// dsnToPath は file: URI (forward-slash・URI エンコード済み) をファイルシステムパスへ戻す。
// darwin の "Application Support" はスペースが %20 にエンコードされるため PathUnescape で復元する。
// 注: 本テストは非 Windows CI (macOS/Linux) 前提。Windows の `file:/C:/...` 形式 (先頭 `/` 付きドライブ)
// は対象外で、その場合 os.Stat 用パスとして崩れる (config.fileURI とは非対称)。
func dsnToPath(dsn string) string {
	p := strings.TrimPrefix(dsn, "file:")
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	return p
}

// TestIntegration_ゼロ設定起動でDB自動作成しマイグレーション適用後にHTTPが応答する は、タスク7 の最終統合検証。
// env を完全に未設定相当にし、アプリデータ解決先を一時ディレクトリへリダイレクトしたうえで、本番と同じ
// 起動シーケンス (config.Load → openAndMigrate → newHTTPHandler → Serve) を組み立て、以下を end-to-end で確認する:
//   - env/.env 無しでも config 解決がハードフェイルせず成立する (R2.1)
//   - DB ファイルが CWD ではなくアプリデータ配下に自動作成される (R2.2, R2.5)
//   - 起動時にスキーマ (全6テーブル) が自動適用される (R4.1)
//   - 合成ルートが実 HTTP リクエスト (/health = DB 疎通込み) に 200 で応答する (R2.1 待受到達)
//
// 各サブシステム (appdata/config/migrate/desktop/mdns/handler) は個別テスト済みのため、本テストは
// それらが「ゼロ設定起動」として正しく合成されることの統合確認に絞る。
func TestIntegration_ゼロ設定起動でDB自動作成しマイグレーション適用後にHTTPが応答する(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevWriter := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	t.Cleanup(func() { gin.DefaultWriter = prevWriter })

	root := t.TempDir()
	// env を未設定相当にし、アプリデータ解決先を temp へリダイレクト (darwin=HOME / linux=XDG_CONFIG_HOME 両対応)。
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("APP_PORT", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))

	// 1) ゼロ設定で config 解決が成立する (必須欠如のハードフェイルが無い)。
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load (env 無し) がハードフェイルした: %v", err)
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "file:") {
		t.Fatalf("DSN が file: URI でない: %q", cfg.DatabaseURL)
	}
	if cfg.SessionSecret == "" {
		t.Fatal("CSRF 認証鍵が自動生成されていない (空)")
	}
	if cfg.AppPort != 8080 {
		t.Errorf("既定ポート = %d, want 8080", cfg.AppPort)
	}

	// 2) DB オープン直後にマイグレーションを自動適用する (本番と同じ openAndMigrate)。
	pool, err := openAndMigrate(context.Background(), cfg.DatabaseURL, migrate.Up)
	if err != nil {
		t.Fatalf("openAndMigrate (ゼロ設定 DSN): %v", err)
	}
	defer pool.Close()

	// スキーマ自動適用済み = 業務テーブル + セッションストア用テーブルが全 7 つ存在する (R4.1)。
	// sessions は migration 00007 で作成され、後続で組み立てる NewSessionManager(sqlite3store) の前提テーブル。
	for _, table := range []string{"users", "devices", "device_tokens", "sensor_readings", "alert_rules", "alert_histories", "sessions"} {
		var name string
		if err := pool.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
			t.Errorf("マイグレーション未適用 (%s テーブルが無い): %v", table, err)
		}
	}

	// 3) DB ファイルが CWD ではなくリダイレクトしたアプリデータ配下に作成されている (R2.2, R2.5)。
	// 末尾セパレータ付き prefix で兄弟ディレクトリ (root の prefix を共有する別ディレクトリ) の誤判定を避ける。
	// 配下判定を先に行い、保存先が誤っている場合に temp 外ファイルへ触れる前に落とす。
	dbPath := dsnToPath(cfg.DatabaseURL)
	if !strings.HasPrefix(dbPath, root+string(os.PathSeparator)) {
		t.Fatalf("DB ファイルがアプリデータ配下 (%s) でなく %s に作成された (ゼロ設定の保存先誤り)", root, dbPath)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB ファイルが解決パス %s に作成されていない: %v", dbPath, err)
	}

	// 4) 本番と同じ合成ルートを組み立て、実 HTTP リクエストに応答することを確認する。
	q := repository.New(pool)
	sm := auth.NewSessionManager(pool, cfg)
	// NewSessionManager が起動する sqlite3store cleanup goroutine を停止しリークを防ぐ。
	// defer で登録し (LIFO により) defer pool.Close より先に止め、Close 後に DELETE が走るのを避ける。
	// Store 実装が変わって停止漏れが起きたら検知できるよう、type assertion 失敗は致命にする。
	store, ok := sm.Store.(*sqlite3store.SQLite3Store)
	if !ok {
		t.Fatalf("sm.Store が *sqlite3store.SQLite3Store でない: %T (cleanup 停止漏れの恐れ)", sm.Store)
	}
	defer store.StopCleanup()
	handler := newHTTPHandler(cfg, sm, q, pool.PingContext)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /health = %d, want 200 (DB 疎通込み health。ゼロ設定起動でサーバが応答していない) body=%s", resp.StatusCode, body)
	}
}
