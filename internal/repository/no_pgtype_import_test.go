package repository

import (
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// Test_repositoryはpgtypeとpgxをimportしない は、SQLite 移行の Foundation ゲート
// (タスク1.2)の観測可能完了条件「再生成された internal/repository に pgtype が
// 一切残らない」を AST 走査で回帰固定する。
//
// sqlc を engine=sqlite で再生成すると、生成型は pgtype.Numeric/Timestamptz/Date
// から database/sql 系(float64/time.Time/sql.Null*/json.RawMessage)へ移行する。
// 本テストはテストファイルを除く全 .go の直接 import を検査し、pgtype/pgx 依存が
// 残っていないことを保証する(engine=postgresql のままだと RED)。
func Test_repositoryはpgtypeとpgxをimportしない(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}

	forbidden := []string{"pgtype", "jackc/pgx"}
	for _, pkg := range pkgs {
		for fname, file := range pkg.Files {
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				for _, f := range forbidden {
					if strings.Contains(path, f) {
						t.Errorf("%s が禁止 import %q を含む (SQLite 移行未完: pgtype/pgx 残存)", fname, path)
					}
				}
			}
		}
	}
}
