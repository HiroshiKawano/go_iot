package config

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // round-trip 検証用に database/sql ドライバ "sqlite" を登録
)

// fileURI は Windows バックスラッシュ生パスを含むパスを forward-slash 化した file: URI へ
// 変換する (Decision 5)。OS に依存せず変換できることを table-driven で検証する。
func TestFileURI(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "Windows バックスラッシュパスを forward-slash 化し先頭スラッシュを付与",
			path: `C:\Users\foo\go_iot\app.db`,
			want: "file:/C:/Users/foo/go_iot/app.db",
		},
		{
			name: "非 Windows 絶対パスはそのまま file: URI 化",
			path: "/home/u/.config/go_iot/app.db",
			want: "file:/home/u/.config/go_iot/app.db",
		},
		{
			name: "パス中のスペースは URI エンコードする",
			path: "/home/u/my data/go_iot/app.db",
			want: "file:/home/u/my%20data/go_iot/app.db",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileURI(tt.path); got != tt.want {
				t.Errorf("fileURI(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// env 全未設定でも Load がハードフェイルせず成功し、DatabaseURL が forward-slash file: URI を返す。
func TestLoad_DefaultDatabaseURL(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("env 全未設定で Load() が失敗してはならない: %v", err)
	}
	if !strings.HasPrefix(cfg.DatabaseURL, "file:/") {
		t.Errorf("DatabaseURL = %q, forward-slash file: URI であるべき", cfg.DatabaseURL)
	}
	if strings.Contains(cfg.DatabaseURL, `\`) {
		t.Errorf("DatabaseURL = %q, バックスラッシュを含んではならない", cfg.DatabaseURL)
	}
	if !strings.HasSuffix(cfg.DatabaseURL, "app.db") {
		t.Errorf("DatabaseURL = %q, 既定は app.db で終わるべき", cfg.DatabaseURL)
	}
	if !strings.Contains(cfg.DatabaseURL, "go_iot") {
		t.Errorf("DatabaseURL = %q, アプリデータ(go_iot)配下を指すべき", cfg.DatabaseURL)
	}
}

// DATABASE_URL の明示指定は既定より最優先される。
func TestLoad_DatabaseURLOverride(t *testing.T) {
	root := t.TempDir()
	const explicit = "file:/explicit/path/custom.db"
	t.Setenv("DATABASE_URL", explicit)
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() エラー: %v", err)
	}
	if cfg.DatabaseURL != explicit {
		t.Errorf("DatabaseURL = %q, 指定値 %q が最優先されるべき", cfg.DatabaseURL, explicit)
	}
}

// forward-slash file: URI から構築した DSN が、CWD ではなく意図した絶対パスへ DB を開く (Decision 5)。
func TestFileURI_OpensAtAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "app.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("親ディレクトリ作成: %v", err)
	}

	db, err := sql.Open("sqlite", fileURI(dbPath))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Exec で接続を実体化し、ファイルを実際に作成させる (sql.Open は遅延接続)。
	if _, err := db.Exec("CREATE TABLE t (x INTEGER)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB ファイルが意図した絶対パス %q に作成されていない: %v", dbPath, err)
	}
}
