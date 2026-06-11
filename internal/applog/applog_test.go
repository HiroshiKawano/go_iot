package applog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/appdata"
)

func TestDestination(t *testing.T) {
	const def = "/default/go_iot/app.log"
	defFn := func() string { return def }

	tests := []struct {
		name       string
		mode       string
		envLogFile string
		want       string
	}{
		{name: "env指定は最優先(consoleモードでも勝つ)", mode: "console", envLogFile: "/x/y.log", want: "/x/y.log"},
		{name: "env指定はfileモードでも勝つ", mode: "file", envLogFile: "/x/y.log", want: "/x/y.log"},
		{name: "fileモードかつenv空は既定パス", mode: "file", envLogFile: "", want: def},
		{name: "consoleモードかつenv空はstdout(空文字)", mode: "console", envLogFile: "", want: ""},
		{name: "未知のモードはfile扱いせずstdout", mode: "", envLogFile: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Destination(tt.mode, tt.envLogFile, defFn)
			if got != tt.want {
				t.Errorf("Destination(%q, %q) = %q, want %q", tt.mode, tt.envLogFile, got, tt.want)
			}
		})
	}
}

// DefaultPath は appdata 単一解決源へ委譲する。実機 (darwin/linux) では appdata が
// GOOS gated で LOCALAPPDATA を参照しないため、LOCALAPPDATA を設定しても委譲先 (UserConfigDir 配下)
// を返し、app.db と同一ディレクトリへ解決される (二重実装排除・Decision 1)。
func TestDefaultPath_DelegatesToAppdata(t *testing.T) {
	root := t.TempDir()
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "localappdata"))
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))

	got := DefaultPath()

	want, err := appdata.Path("app.log")
	if err != nil {
		t.Fatalf("appdata.Path: %v", err)
	}
	if got != want {
		t.Errorf("DefaultPath() = %q, appdata 委譲先 %q と一致すべき (二重実装排除)", got, want)
	}
	if filepath.Base(got) != "app.log" {
		t.Errorf("DefaultPath() = %q, 末尾は app.log であるべき", got)
	}

	// app.log と app.db が同一ディレクトリへ解決される (単一解決源)
	dbPath, err := appdata.Path("app.db")
	if err != nil {
		t.Fatalf("appdata.Path: %v", err)
	}
	if filepath.Dir(got) != filepath.Dir(dbPath) {
		t.Errorf("app.log (%q) と app.db (%q) が同一ディレクトリへ解決されるべき", got, dbPath)
	}
}

func TestDefaultPath_FallbackWhenNoLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")

	got := DefaultPath()
	if got == "" {
		t.Fatal("DefaultPath() は LOCALAPPDATA 未設定でも空であってはならない")
	}
	// app.log で終わり、go_iot を含むこと(配置ディレクトリは OS 依存なので末尾要素のみ検証)
	if filepath.Base(got) != "app.log" {
		t.Errorf("DefaultPath() = %q, 末尾は app.log であるべき", got)
	}
}

func TestSetup_Stdout(t *testing.T) {
	w, closeFn, err := Setup("")
	if err != nil {
		t.Fatalf("Setup(\"\") 予期せぬエラー: %v", err)
	}
	if w != os.Stdout {
		t.Errorf("Setup(\"\") の Writer は os.Stdout であるべき")
	}
	if err := closeFn(); err != nil {
		t.Errorf("stdout の close は no-op でエラーを返さないべき: %v", err)
	}
}

func TestSetup_File(t *testing.T) {
	// 親ディレクトリが未作成でも Setup が作成すること
	path := filepath.Join(t.TempDir(), "logs", "app.log")

	w, closeFn, err := Setup(path)
	if err != nil {
		t.Fatalf("Setup(%q) 予期せぬエラー: %v", path, err)
	}

	const line = "ログ出力テスト\n"
	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatalf("Write 失敗: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close 失敗: %v", err)
	}

	// 親ディレクトリが作成されている
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Errorf("親ディレクトリが作成されていない: %v", err)
	}
	// ファイルに書き込まれている
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ログファイル読み込み失敗: %v", err)
	}
	if string(got) != line {
		t.Errorf("ログ内容 = %q, want %q", string(got), line)
	}
}
