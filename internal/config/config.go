package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/HiroshiKawano/go_iot/internal/appdata"
)

// Config はアプリケーション全体で使用する設定値を保持する。
// 環境変数を最優先しつつ、未設定時はアプリデータ配下の既定値で成立する
// (env/.env なしのゼロ設定起動。desktop-exe-packaging)。
type Config struct {
	AppEnv        string
	AppPort       int
	DatabaseURL   string // SQLite ファイル DSN (forward-slash file: URI)。PRAGMA は接続層(infra/db)が付与
	SessionSecret string // 実体は gorilla/csrf 認証鍵 (scs セッションは不透明トークン+DBストアで非依存)
}

// Load は環境変数を読み込んで Config を構築する。
// env/.env が無くてもアプリデータ配下の既定 DB パス・自動生成鍵で成立し、
// 必須欠如によるハードフェイルは起こさない。env 指定は常に最優先。
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnvInt("APP_PORT", 8080),
	}

	dbURL, err := resolveDatabaseURL()
	if err != nil {
		return nil, fmt.Errorf("DB パス解決: %w", err)
	}
	cfg.DatabaseURL = dbURL

	secret, err := resolveSessionSecret()
	if err != nil {
		return nil, fmt.Errorf("CSRF 認証鍵解決: %w", err)
	}
	cfg.SessionSecret = secret

	// production では env/ファイル経由を問わず鍵に同一検証を適用する (design 403)。
	// base64 で 32 文字以上 (32 バイト→44 文字相当) を要求し、短い/非 base64 を弾く。
	if cfg.AppEnv == "production" && !validStoredSecret(cfg.SessionSecret) {
		return nil, errors.New("production では SESSION_SECRET を base64 形式で 32 文字以上にしてください")
	}

	return cfg, nil
}

// resolveDatabaseURL は DB 接続 DSN を確定する。
// env DATABASE_URL を最優先し、無ければアプリデータ配下 app.db の forward-slash file: URI を構築する。
func resolveDatabaseURL() (string, error) {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v, nil
	}
	dbPath, err := appdata.Path("app.db")
	if err != nil {
		return "", err
	}
	return fileURI(dbPath), nil
}

// resolveSessionSecret は CSRF 認証鍵を「env → 鍵ファイル → 生成して永続化」の順で確定する。
// env SESSION_SECRET は最優先。鍵ファイル (アプリデータ配下) が有効ならそれを再利用し、
// 破損/空/長さ不足なら警告ログを出して再生成・上書きする (単一ユーザー前提・fail fast しない)。
func resolveSessionSecret() (string, error) {
	if v := os.Getenv("SESSION_SECRET"); v != "" {
		return v, nil
	}

	keyPath, err := appdata.Path("session_secret")
	if err != nil {
		return "", fmt.Errorf("鍵ファイルパス解決: %w", err)
	}

	if data, err := os.ReadFile(keyPath); err == nil {
		secret := strings.TrimSpace(string(data))
		if validStoredSecret(secret) {
			return secret, nil
		}
		// 鍵ファイルは存在するが不正。再生成・上書きすると既発行 CSRF トークンが無効化される。
		log.Printf("警告: CSRF 認証鍵ファイル %s が不正(空/長さ不足/破損)のため再生成します。既発行の CSRF トークンは無効化されフォーム再読込が必要になります。", keyPath)
	}

	secret, err := generateSecret()
	if err != nil {
		return "", err
	}
	// base64 text で 0o600 保存 (Windows では実質 no-op、保護は %LOCALAPPDATA% の NTFS ACL に依存)。
	if err := os.WriteFile(keyPath, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("鍵ファイル保存: %w", err)
	}
	return secret, nil
}

// generateSecret は crypto/rand で 32 バイトの鍵を生成し base64 (text) で返す。
func generateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("鍵生成: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// validStoredSecret は鍵ファイルから読んだ値が再利用可能かを判定する。
// 空/長さ不足 (base64 で 32 バイト→44 文字。production 検証と整合させ 32 文字以上を要求) と、
// base64 としてデコードできない破損を弾く。
func validStoredSecret(s string) bool {
	if len(s) < 32 {
		return false
	}
	if _, err := base64.StdEncoding.DecodeString(s); err != nil {
		return false
	}
	return true
}

// fileURI はファイルパスを forward-slash 化した file: URI へ変換する。
//
// Windows のバックスラッシュ生パスを `file:` DSN にすると、SQLite URI パーサが `\` を
// 区切りとして解釈せず意図しない場所 (CWD) に DB を黙って作るため不可 (Decision 5)。
// filepath.ToSlash は実行 OS のセパレータしか変換しない (darwin では `\` を変換しない) ので、
// 実行/テスト OS に依らず Windows パスを正しく扱えるよう明示的に `\` を `/` へ置換する。
// ドライブレター付き絶対パス用に先頭へ `/` を付し、スペース等は URI エンコードする。
func fileURI(path string) string {
	slashed := strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file:" + encodePath(slashed)
}

// encodePath はパスの各セグメントを URI エンコードする (区切りの `/` は保持)。
func encodePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// IsDevelopment は開発環境かどうかを判定する。
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
