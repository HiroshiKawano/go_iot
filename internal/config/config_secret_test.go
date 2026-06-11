package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/appdata"
)

// setupTempAppdata は env を全クリアし、appdata の解決先を一時ディレクトリへ固定する。
func setupTempAppdata(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
}

// env 未設定なら鍵を生成・永続化し、再度 Load しても同一鍵を再利用する (3.1)。
func TestLoad_SessionSecretPersisted(t *testing.T) {
	setupTempAppdata(t)

	cfg1, err := Load()
	if err != nil {
		t.Fatalf("1 回目 Load() エラー: %v", err)
	}
	if cfg1.SessionSecret == "" {
		t.Fatal("SessionSecret が空であってはならない")
	}

	cfg2, err := Load()
	if err != nil {
		t.Fatalf("2 回目 Load() エラー: %v", err)
	}
	if cfg1.SessionSecret != cfg2.SessionSecret {
		t.Errorf("永続化された鍵が再利用されていない: 1回目=%q 2回目=%q", cfg1.SessionSecret, cfg2.SessionSecret)
	}

	// 鍵ファイルが永続化されていること
	keyPath, err := appdata.Path("session_secret")
	if err != nil {
		t.Fatalf("鍵ファイルパス解決: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("鍵ファイルが永続化されていない: %v", err)
	}
}

// SESSION_SECRET の明示指定は最優先され、鍵生成・永続化を行わない (3.3)。
func TestLoad_SessionSecretEnvOverride(t *testing.T) {
	setupTempAppdata(t)
	const explicit = "explicit-csrf-key-value-32characters!!"
	t.Setenv("SESSION_SECRET", explicit)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() エラー: %v", err)
	}
	if cfg.SessionSecret != explicit {
		t.Errorf("SessionSecret = %q, 指定値 %q が最優先されるべき", cfg.SessionSecret, explicit)
	}

	// env 指定時は鍵ファイルを生成しない
	keyPath, _ := appdata.Path("session_secret")
	if _, err := os.Stat(keyPath); err == nil {
		t.Error("env 指定時は鍵ファイルを生成すべきでない")
	}
}

// 破損 (空/長さ不足/非 base64) 鍵ファイルは警告ログを出して再生成・上書きする (3.1)。
func TestLoad_SessionSecretRegeneratesOnCorruption(t *testing.T) {
	setupTempAppdata(t)

	// 破損鍵ファイルを事前配置 (appdata.Path がディレクトリを作成する)
	keyPath, err := appdata.Path("session_secret")
	if err != nil {
		t.Fatalf("鍵ファイルパス解決: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("bad"), 0o600); err != nil {
		t.Fatalf("破損鍵ファイル配置: %v", err)
	}

	var logBuf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(orig) })

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() エラー: %v", err)
	}
	if cfg.SessionSecret == "bad" || len(cfg.SessionSecret) < 32 {
		t.Errorf("破損鍵が再生成されていない: %q", cfg.SessionSecret)
	}
	if !strings.Contains(logBuf.String(), "再生成") {
		t.Errorf("再生成の警告ログが出ていない: %q", logBuf.String())
	}

	// ファイルが新しい有効な鍵で上書きされていること
	got, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("鍵ファイル読込: %v", err)
	}
	if strings.TrimSpace(string(got)) != cfg.SessionSecret {
		t.Errorf("鍵ファイルが再生成値で上書きされていない")
	}
}

// production では鍵長検証を維持し、短い SESSION_SECRET はエラーにする。
func TestLoad_ProductionShortSecretFails(t *testing.T) {
	setupTempAppdata(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "short")

	if _, err := Load(); err == nil {
		t.Error("production で短い SESSION_SECRET は Load() がエラーを返すべき")
	}
}

// production では env 経由の鍵にも base64 妥当性検証を適用する (design 403: env/file 同一検証)。
// 32 文字以上でも base64 としてデコードできない値はエラーにする。
func TestLoad_ProductionNonBase64SecretFails(t *testing.T) {
	setupTempAppdata(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "this-string-is-long-enough-but-not-valid-base64!!")

	if _, err := Load(); err == nil {
		t.Error("production で非 base64 の SESSION_SECRET は Load() がエラーを返すべき")
	}
}

// production でも有効な base64 鍵 (32 文字以上) は受け入れる。
func TestLoad_ProductionValidBase64SecretOK(t *testing.T) {
	setupTempAppdata(t)
	t.Setenv("APP_ENV", "production")
	// 32 バイトを base64 化した 44 文字の有効な鍵
	t.Setenv("SESSION_SECRET", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	if _, err := Load(); err != nil {
		t.Errorf("production で有効な base64 鍵は受け入れるべき: %v", err)
	}
}
