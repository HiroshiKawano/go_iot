package main

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// moduleRoot は go.mod が在るリポジトリルートを返す。テストの作業ディレクトリ (cmd/server) から
// 上方向に go.mod を探索する。配布整備テスト群がリポジトリ直下のファイル (Makefile/README 等) を
// 参照するために使う。
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("作業ディレクトリ取得: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod が見つからない (リポジトリルートを特定できない)")
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("%s の読み込み: %v", name, err)
	}
	return string(b)
}

// --- 6.1 docker / PostgreSQL 撤去 ---

// TestPackaging_コンテナ構成ファイルが存在しない は、docker-compose 等のコンテナ構成ファイルが
// リポジトリに残っていないことを固定する (R10.1)。
func TestPackaging_コンテナ構成ファイルが存在しない(t *testing.T) {
	root := moduleRoot(t)
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml", "Dockerfile"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			t.Errorf("%s が残っている (R10.1: コンテナ構成ファイルを含まないこと)", name)
		}
	}
}

// TestPackaging_Makefileにdocker起動停止ターゲットが無い は、Makefile から docker compose 依存と
// up/down (DB サーバ起動停止) ターゲットが撤去されていることを固定する (R10.2)。
func TestPackaging_Makefileにdocker起動停止ターゲットが無い(t *testing.T) {
	mk := readRepoFile(t, moduleRoot(t), "Makefile")
	for _, s := range []string{"docker compose", "docker-compose"} {
		if strings.Contains(mk, s) {
			t.Errorf("Makefile に %q が残っている (docker 依存撤去)", s)
		}
	}
	// 行頭のターゲット定義 (up: / down:) が無いこと。行頭アンカーで setup:/cleanup: 等の誤検知を避ける。
	if regexp.MustCompile(`(?m)^(up|down):`).MatchString(mk) {
		t.Error("Makefile に up:/down: ターゲットが残っている (DB サーバ起動停止は不要)")
	}
}

// TestPackaging_バイナリにpostgresドライバが含まれない は、cmd/server の実ビルドグラフ (go list -deps)
// に PostgreSQL ドライバ (pgx / lib/pq) が含まれず、SQLite ドライバ (modernc) が含まれることを固定する。
// go.mod に indirect で pgx が残っても、実バイナリに含まれなければ起動経路は PostgreSQL 非依存 (R10.2)。
func TestPackaging_バイナリにpostgresドライバが含まれない(t *testing.T) {
	root := moduleRoot(t)
	cmd := exec.Command("go", "list", "-deps", "./cmd/server")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./cmd/server: %v\n%s", err, out)
	}
	// go list -deps は 1 行 1 パッケージを出力する。行単位の接頭辞一致で判定し、部分文字列の誤検知を避ける。
	deps := string(out)
	hasSQLite := false
	for _, line := range strings.Split(strings.TrimSpace(deps), "\n") {
		for _, drv := range []string{"github.com/jackc/pgx", "github.com/lib/pq"} {
			if line == drv || strings.HasPrefix(line, drv+"/") {
				t.Errorf("cmd/server バイナリ依存に %q が含まれる (PostgreSQL 非依存に反する)", line)
			}
		}
		if line == "modernc.org/sqlite" || strings.HasPrefix(line, "modernc.org/sqlite/") {
			hasSQLite = true
		}
	}
	if !hasSQLite {
		t.Error("cmd/server バイナリ依存に modernc.org/sqlite が無い (SQLite ドライバが組み込まれていない)")
	}
}

// --- 6.2 Windows 単一 .exe クロスビルド + Version 注入 ---

// TestPackaging_MakefileのWindowsビルドにVersion注入がある は、build-windows / build-windows-gui の
// 両ターゲットに internal/view.Version への ldflags 注入が入っていることを固定する (6.2)。
func TestPackaging_MakefileのWindowsビルドにVersion注入がある(t *testing.T) {
	mk := readRepoFile(t, moduleRoot(t), "Makefile")
	const inject = "-X github.com/HiroshiKawano/go_iot/internal/view.Version="
	if n := strings.Count(mk, inject); n < 2 {
		t.Errorf("Version 注入が %d 箇所 (build-windows と build-windows-gui の両方=2 箇所必要): %q", n, inject)
	}
}

// TestPackaging_Windowsクロスビルドが単一PE32exeをcgoなしで生成する は、macOS/Linux 上の
// CGO_ENABLED=0 クロスビルドが console / GUI 両方の単一 .exe を生成し、いずれも PE32+ (amd64) で
// サブシステムが期待どおり (console=CUI / GUI=GUI) であることを debug/pe で検証する (R1.1, R1.2, R1.3, R9.1)。
// cgo を要するパッケージが混入すると CGO_ENABLED=0 でビルドが失敗するため、成功自体が pure-Go の証跡となる。
//
// 前提: internal/migrate/migrations は gitignore された go:embed 生成物のため、本テスト (内部で
// go build ./cmd/server を実行) は make sync-migrations 済み = make test 経由を前提とする。素の
// `go test ./...` をクリーン clone で叩くと、embed 対象欠如で internal/migrate がそもそも
// コンパイル不能になる (本テスト固有ではなくパッケージ全体の前提)。
func TestPackaging_Windowsクロスビルドが単一PE32exeをcgoなしで生成する(t *testing.T) {
	if testing.Short() {
		t.Skip("クロスビルドは時間がかかるため -short ではスキップ")
	}
	root := moduleRoot(t)
	dir := t.TempDir()

	cases := []struct {
		name          string
		gui           bool
		wantSubsystem uint16
	}{
		{name: "console", gui: false, wantSubsystem: pe.IMAGE_SUBSYSTEM_WINDOWS_CUI},
		{name: "gui", gui: true, wantSubsystem: pe.IMAGE_SUBSYSTEM_WINDOWS_GUI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exePath := filepath.Join(dir, tc.name+".exe")
			ldflags := "-s -w"
			if tc.gui {
				ldflags += " -H windowsgui"
			}
			cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", exePath, "./cmd/server")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=windows", "GOARCH=amd64")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Windows %s クロスビルド失敗 (cgo 由来リンクエラーの疑い): %v\n%s", tc.name, err, out)
			}

			// 単一ファイル (追加同梱物不要) として生成されている。
			info, err := os.Stat(exePath)
			if err != nil {
				t.Fatalf("生成物 stat: %v", err)
			}
			if info.Size() == 0 {
				t.Fatal("生成された .exe が空")
			}

			f, err := pe.Open(exePath)
			if err != nil {
				t.Fatalf("PE として開けない: %v", err)
			}
			defer f.Close()

			if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
				t.Errorf("machine = %#x, want AMD64 (%#x)", f.Machine, pe.IMAGE_FILE_MACHINE_AMD64)
			}
			oh, ok := f.OptionalHeader.(*pe.OptionalHeader64)
			if !ok {
				t.Fatalf("OptionalHeader が 64bit でない (PE32+ ではない): %T", f.OptionalHeader)
			}
			if oh.Subsystem != tc.wantSubsystem {
				t.Errorf("subsystem = %d, want %d (%s)", oh.Subsystem, tc.wantSubsystem, tc.name)
			}
		})
	}
}

// --- 6.3 配布文書を SQLite / デスクトップ起動へ更新 ---

// TestPackaging_READMEにdocker_postgres前提が無くSQLite手順がある は、README から docker/PostgreSQL の
// 前提記述 (コンテナ起動コマンド・PG ポート・Docker Desktop 要件) が消え、SQLite ベースのデスクトップ
// 起動手順 (.exe) が記載されていることを固定する (R10.3)。語「PostgreSQL」自体は移行履歴の言及を許容するため、
// 前提を一意に示すマーカーのみを禁止語とする。
func TestPackaging_READMEにdocker_postgres前提が無くSQLite手順がある(t *testing.T) {
	readme := readRepoFile(t, moduleRoot(t), "README.md")
	// PG ポート前提は文脈付きマーカー (":5432" / "5432:5432") で検出する。裸の "5432" は無関係な
	// 数値 (バイト数等) に偶発一致してブリットルになるため使わない。
	for _, s := range []string{"docker compose", "docker-compose", "Docker Desktop", ":5432", "5432:5432"} {
		if strings.Contains(readme, s) {
			t.Errorf("README に前提記述 %q が残っている (R10.3)", s)
		}
	}
	for _, s := range []string{"SQLite", ".exe"} {
		if !strings.Contains(readme, s) {
			t.Errorf("README に %q が無い (SQLite デスクトップ起動手順が必要・R10.3)", s)
		}
	}
}

// TestPackaging_envExampleがSQLite前提でenv任意を明記 は、設定例から PostgreSQL 接続が消え、SQLite file:
// DSN と「env は任意 (未設定でも起動)」の旨が記載されていることを固定する (R10.3)。
func TestPackaging_envExampleがSQLite前提でenv任意を明記(t *testing.T) {
	env := readRepoFile(t, moduleRoot(t), ".env.example")
	if strings.Contains(env, "postgres") {
		t.Error(".env.example に postgres 接続が残っている (R10.3)")
	}
	if !strings.Contains(env, "file:") {
		t.Error(".env.example が SQLite file: DSN になっていない (R10.3)")
	}
	if !strings.Contains(env, "任意") && !strings.Contains(env, "未設定") && !strings.Contains(env, "省略") {
		t.Error(".env.example に env が任意 (未設定でも起動) である旨の記載が無い (R2.1/R10.3)")
	}
}
