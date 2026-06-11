// Package appdata は OS 別のアプリデータディレクトリを解決し、無ければ作成する
// 単一解決源を提供する。DB ファイル (app.db)・ログ (app.log)・CSRF 認証鍵ファイルの
// 配置先をここに集約し、別ディレクトリへ分散しないことを保証する (design Decision 1)。
//
// 解決規則:
//   - Windows  : %LOCALAPPDATA%\go_iot (LOCALAPPDATA 環境変数。os.UserConfigDir は
//     Roaming %APPDATA% を返すため使わない)
//   - 非 Windows: os.UserConfigDir()/go_iot
//   - フォールバック: 上記が取得不可なら実行ファイル隣の go_iot-data
//     (GUI ダブルクリック起動では CWD が予測不能なため CWD 相対にしない)
//
// 書込制限されうる Program Files 配下等を既定にしないため、ユーザープロファイル配下の
// LOCALAPPDATA / UserConfigDir を一次解決源とする。
package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// appName はアプリデータディレクトリ名。
const appName = "go_iot"

// Dir はアプリデータディレクトリの絶対パスを返し、無ければ作成する。
// 同一プロセス内では同一パスを返す (副作用は mkdir のみ)。
func Dir() (string, error) {
	return resolveDir(runtime.GOOS, os.Getenv("LOCALAPPDATA"), os.UserConfigDir, os.Executable)
}

// Path は Dir 配下のファイルパスを返す（例: Path("app.db")）。
// 返却前に Dir がディレクトリを作成するため、親ディレクトリは存在する。
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// resolveDir は baseDir で決定したディレクトリを冪等作成して返す。
// 依存 (goos / localAppData / userConfigDir / executable) を引数化してテスト可能にする。
func resolveDir(goos, localAppData string, userConfigDir, executable func() (string, error)) (string, error) {
	dir, err := baseDir(goos, localAppData, userConfigDir, executable)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("アプリデータディレクトリ作成: %w", err)
	}
	return dir, nil
}

// baseDir は OS 種別・環境・実行ファイルパスからデータディレクトリを決定する純粋ロジック
// (副作用なし)。解決順は Windows=LOCALAPPDATA、非 Windows=UserConfigDir、最終手段=実行ファイル隣。
func baseDir(goos, localAppData string, userConfigDir, executable func() (string, error)) (string, error) {
	if goos == "windows" {
		if localAppData != "" {
			return filepath.Join(localAppData, appName), nil
		}
		// Windows で LOCALAPPDATA が空 (ほぼ発生しない) はフォールバックへ。
		// UserConfigDir は Roaming %APPDATA% を返すため使わない。
		return fallbackDir(executable)
	}
	if dir, err := userConfigDir(); err == nil {
		return filepath.Join(dir, appName), nil
	}
	return fallbackDir(executable)
}

// fallbackDir は実行ファイル隣の go_iot-data を返す (CWD 相対にしない)。
func fallbackDir(executable func() (string, error)) (string, error) {
	exe, err := executable()
	if err != nil {
		return "", fmt.Errorf("実行ファイルパス取得: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "go_iot-data"), nil
}
