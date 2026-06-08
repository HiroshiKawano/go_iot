package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test_NewPool_SQLite接続でWALとPRAGMAが実効しPingが成功する は、Task 2.1 の観測可能完了条件
// 「接続生成のスモークテストで WAL が実効し PingContext が成功する」を実機検証する。
// SQLite 単一 writer 前提の接続層が modernc.org/sqlite + database/sql で正しく構築され、
// journal_mode=WAL / busy_timeout / foreign_keys=ON の PRAGMA が各接続へ適用されることを確認する。
//
// 現状 pool.go が *pgxpool.Pool を返す間は PingContext/QueryRow(database/sql の API)が
// コンパイルできず RED となる。
func Test_NewPool_SQLite接続でWALとPRAGMAが実効しPingが成功する(t *testing.T) {
	// WAL は実ファイルが必要(:memory: は WAL 不可)。一時ディレクトリで隔離する。
	dbPath := filepath.Join(t.TempDir(), "smoke.sqlite")
	dsn := "file:" + dbPath

	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		t.Fatalf("PingContext: %v", err)
	}

	// WAL が実効していること(journal_mode=WAL は DB ファイル属性として永続化される)。
	var journalMode string
	if err := pool.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}

	// busy_timeout が適用されていること(scs cleanup × ESP32 INSERT の SQLITE_BUSY を待機吸収)。
	var busyTimeout int
	if err := pool.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	// foreign_keys が ON であること。
	var foreignKeys int
	if err := pool.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ON)", foreignKeys)
	}

	// SQLite 単一 writer 前提で接続数を絞っていること(現 pgx の MaxConns=10 は不適切)。
	if got := pool.Stats().MaxOpenConnections; got <= 0 || got > 8 {
		t.Errorf("MaxOpenConnections = %d, want 単一writer前提の控えめな値(1..8)", got)
	}
}

// Test_withPragmas_既存クエリ有無で正しく連結する は、DSN への PRAGMA 付与が
// '?' の有無に応じて '?'/'&' を正しく選ぶことを検証する。
func Test_withPragmas_既存クエリ有無で正しく連結する(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want []string // 含むべき部分文字列
	}{
		{"クエリなし", "file:test.sqlite", []string{"file:test.sqlite?", "_pragma=journal_mode(WAL)", "_pragma=busy_timeout(5000)", "_pragma=foreign_keys(1)"}},
		{"既存クエリあり", "file:test.sqlite?cache=shared", []string{"file:test.sqlite?cache=shared&", "_pragma=journal_mode(WAL)"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := withPragmas(c.dsn)
			for _, sub := range c.want {
				if !strings.Contains(got, sub) {
					t.Errorf("withPragmas(%q) = %q, want contains %q", c.dsn, got, sub)
				}
			}
		})
	}
}

// Test_NewPool_全接続にPRAGMAが適用される は、PRAGMA が単一接続だけでなく SetMaxOpenConns 分の
// 全接続へ適用されることを保証する(design: 接続を絞っても全接続へ確実適用)。
// modernc の更新等で DSN _pragma 方式がデグレしても検知できる回帰ガード。
func Test_NewPool_全接続にPRAGMAが適用される(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "pragma_all.sqlite")
	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	// MaxOpenConns 分の接続を同時に掴み、新規接続を強制的にオープンさせる。
	const n = 4
	conns := make([]*sql.Conn, 0, n)
	for i := 0; i < n; i++ {
		c, err := pool.Conn(context.Background())
		if err != nil {
			t.Fatalf("Conn[%d]: %v", i, err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		var fk, bt int
		if err := c.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatalf("conn[%d] PRAGMA foreign_keys: %v", i, err)
		}
		if err := c.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&bt); err != nil {
			t.Fatalf("conn[%d] PRAGMA busy_timeout: %v", i, err)
		}
		if fk != 1 || bt != 5000 {
			t.Errorf("conn[%d]: foreign_keys=%d busy_timeout=%d, want 1/5000 (この接続に PRAGMA 未適用)", i, fk, bt)
		}
		c.Close()
	}
}

// Test_NewPool_並行書込と読取がSQLITE_BUSYで失敗しない は、design/research R-3 の最大運用リスク
// (scs cleanup × ESP32 INSERT × Web UI 読取の競合)を busy_timeout が吸収することを回帰固定する。
// MaxOpenConns を超える goroutine から並行 INSERT/SELECT を投げ、SQLITE_BUSY 由来の失敗が出ないことを確認。
func Test_NewPool_並行書込と読取がSQLITE_BUSYで失敗しない(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "busy.sqlite")
	pool, err := NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, v INTEGER NOT NULL)`); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}

	const writers, perWriter, readers = 8, 20, 4
	var wg sync.WaitGroup
	var failures int64

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				if _, err := pool.Exec(`INSERT INTO t (v) VALUES (?)`, base+i); err != nil {
					atomic.AddInt64(&failures, 1)
				}
			}
		}(w * perWriter)
	}
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				var cnt int
				if err := pool.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&cnt); err != nil {
					atomic.AddInt64(&failures, 1)
				}
			}
		}()
	}
	wg.Wait()

	if failures != 0 {
		t.Errorf("並行アクセスで %d 件失敗(busy_timeout が SQLITE_BUSY を吸収できていない)", failures)
	}
	// 全 INSERT が投入されたこと(書込のシリアライズが正しく機能)。
	var total int
	if err := pool.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&total); err != nil {
		t.Fatalf("最終 COUNT: %v", err)
	}
	if want := writers * perWriter; total != want {
		t.Errorf("投入行数 = %d, want %d", total, want)
	}
}
