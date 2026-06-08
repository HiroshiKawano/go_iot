// Package applog はアプリケーションログの出力先設定を担う。
//
// Windows の GUI ビルド (`-ldflags "-H windowsgui"`) ではコンソールウィンドウが無く、
// 標準出力/標準エラーへの書き込みが失われる。そのためビルド時に Mode="file" を注入し
// (`-ldflags "-X .../internal/applog.Mode=file"`)、ログを %LOCALAPPDATA%\go_iot\app.log
// 等のファイルへローテーション付きで書き出す。
//
// 本パッケージは DB 層から独立しており、SQLite 移行 (S9) とは無関係に利用できる。
package applog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Mode はビルド時に -ldflags "-X .../internal/applog.Mode=file" で上書きされる出力モード。
// 既定 "console" では標準出力へ、"file" では DefaultPath() のファイルへ出力する。
var Mode = "console"

// Destination はログ出力先のファイルパスを解決する。
// 空文字を返した場合は標準出力を意味する。
// 優先順位: envLogFile (明示指定) > mode=="file" のとき defaultPath() > "" (stdout)。
func Destination(mode, envLogFile string, defaultPath func() string) string {
	if envLogFile != "" {
		return envLogFile
	}
	if mode == "file" {
		return defaultPath()
	}
	return ""
}

// DefaultPath は既定のログファイルパスを返す。
// Windows では %LOCALAPPDATA%\go_iot\app.log、それ以外は os.UserConfigDir 配下。
// いずれも取得できない場合はカレント配下の相対パスにフォールバックする。
func DefaultPath() string {
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "go_iot", "app.log")
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "go_iot", "app.log")
	}
	return filepath.Join("go_iot-data", "app.log")
}

// Setup はログ出力先を構築する。
// path が空なら標準出力を返し close は no-op。path 指定時は親ディレクトリを作成し、
// ローテーション付き (lumberjack) のファイルへ書き出す io.Writer と close 関数を返す。
//
// 圃場のノートPC で長期間稼働するため、サイズ上限・世代数・保持日数を設定して
// ログが無制限に肥大化しないようにする。
func Setup(path string) (io.Writer, func() error, error) {
	if path == "" {
		return os.Stdout, func() error { return nil }, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, nil, fmt.Errorf("ログディレクトリ作成: %w", err)
	}

	lj := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    10, // MB: この大きさを超えるとローテーション
		MaxBackups: 5,  // 世代: 古いログは5つまで保持
		MaxAge:     90, // 日: 90日より古いログは削除
		Compress:   true,
	}
	return lj, lj.Close, nil
}
