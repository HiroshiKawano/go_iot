package appdata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// baseDir は副作用なしのパス決定ロジック。OS 種別ごとの解決規則とフォールバックを
// table-driven で検証する (実 OS に依存せず windows / 非 windows / フォールバックの
// 3 経路をすべて通すため、依存を引数で注入する)。
func TestBaseDir(t *testing.T) {
	okUserConfig := func(p string) func() (string, error) {
		return func() (string, error) { return p, nil }
	}
	errFn := func() (string, error) { return "", errors.New("取得不可") }
	okExe := func(p string) func() (string, error) {
		return func() (string, error) { return p, nil }
	}

	tests := []struct {
		name          string
		goos          string
		localAppData  string
		userConfigDir func() (string, error)
		executable    func() (string, error)
		want          string
		wantErr       bool
	}{
		{
			name:         "Windows は LOCALAPPDATA 配下",
			goos:         "windows",
			localAppData: filepath.FromSlash("/tmp/local"),
			want:         filepath.Join(filepath.FromSlash("/tmp/local"), "go_iot"),
		},
		{
			name:          "非 Windows(linux) は UserConfigDir 配下",
			goos:          "linux",
			userConfigDir: okUserConfig(filepath.FromSlash("/home/u/.config")),
			want:          filepath.Join(filepath.FromSlash("/home/u/.config"), "go_iot"),
		},
		{
			name:          "非 Windows(darwin) は UserConfigDir 配下",
			goos:          "darwin",
			userConfigDir: okUserConfig(filepath.FromSlash("/home/u/Library/Application Support")),
			want:          filepath.Join(filepath.FromSlash("/home/u/Library/Application Support"), "go_iot"),
		},
		{
			name:         "Windows で LOCALAPPDATA 空はフォールバック(実行ファイル隣 go_iot-data)",
			goos:         "windows",
			localAppData: "",
			executable:   okExe(filepath.FromSlash("/opt/app/go_iot.exe")),
			want:         filepath.Join(filepath.FromSlash("/opt/app"), "go_iot-data"),
		},
		{
			name:          "非 Windows で UserConfigDir 取得不可はフォールバック(実行ファイル隣)",
			goos:          "linux",
			userConfigDir: errFn,
			executable:    okExe(filepath.FromSlash("/opt/app/go_iot")),
			want:          filepath.Join(filepath.FromSlash("/opt/app"), "go_iot-data"),
		},
		{
			name:          "解決源がすべて取得不可ならエラー",
			goos:          "linux",
			userConfigDir: errFn,
			executable:    errFn,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := baseDir(tt.goos, tt.localAppData, tt.userConfigDir, tt.executable)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("baseDir() エラーを期待したが nil (got=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("baseDir() 予期せぬエラー: %v", err)
			}
			if got != tt.want {
				t.Errorf("baseDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

// resolveDir はパス決定に加えてディレクトリを冪等作成する。LOCALAPPDATA を一時ディレクトリへ
// 差し替えた経路 (Windows 相当) を含め、各経路で当該ディレクトリ配下を返し実際に作成されることを検証。
func TestResolveDir_CreatesDirectory(t *testing.T) {
	okFn := func(p string) func() (string, error) {
		return func() (string, error) { return p, nil }
	}
	errFn := func() (string, error) { return "", errors.New("取得不可") }

	t.Run("Windows: LOCALAPPDATA 配下を作成", func(t *testing.T) {
		root := t.TempDir()
		got, err := resolveDir("windows", root, errFn, errFn)
		if err != nil {
			t.Fatalf("resolveDir() エラー: %v", err)
		}
		want := filepath.Join(root, "go_iot")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		assertDirExists(t, got)
	})

	t.Run("非 Windows: UserConfigDir 配下を作成", func(t *testing.T) {
		root := t.TempDir()
		got, err := resolveDir("linux", "", okFn(root), errFn)
		if err != nil {
			t.Fatalf("resolveDir() エラー: %v", err)
		}
		want := filepath.Join(root, "go_iot")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		assertDirExists(t, got)
	})

	t.Run("フォールバック: 実行ファイル隣 go_iot-data を作成", func(t *testing.T) {
		root := t.TempDir()
		exe := filepath.Join(root, "go_iot")
		got, err := resolveDir("linux", "", errFn, okFn(exe))
		if err != nil {
			t.Fatalf("resolveDir() エラー: %v", err)
		}
		want := filepath.Join(root, "go_iot-data")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		assertDirExists(t, got)
	})
}

// Dir / Path の公開 API が実環境で解決・作成し、同一ディレクトリ配下のファイルパスを返すこと。
func TestDirAndPath(t *testing.T) {
	root := t.TempDir()
	// 非 Windows 実機 (darwin/linux) では UserConfigDir 経由。HOME / XDG を差し替えて
	// テンポラリ配下へ解決させる。LOCALAPPDATA は GOOS!=windows では参照されないが念のため空に。
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() エラー: %v", err)
	}
	if filepath.Base(dir) != "go_iot" {
		t.Errorf("Dir() = %q, 末尾要素は go_iot であるべき", dir)
	}
	assertDirExists(t, dir)

	p, err := Path("app.db")
	if err != nil {
		t.Fatalf("Path() エラー: %v", err)
	}
	if want := filepath.Join(dir, "app.db"); p != want {
		t.Errorf("Path(\"app.db\") = %q, want %q", p, want)
	}
	if filepath.Dir(p) != dir {
		t.Errorf("Path の親 %q は Dir %q と一致すべき", filepath.Dir(p), dir)
	}
}

func assertDirExists(t *testing.T, dir string) {
	t.Helper()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("ディレクトリが作成されていない: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q はディレクトリではない", dir)
	}
}
