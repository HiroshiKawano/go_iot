package auth

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/alexedwards/scs/sqlite3store"
)

// createSessionsTable は sqlite3store 要求スキーマ (db/migrations/00007 と同一) の sessions を作る。
// :memory: は接続ごとに別 DB となり再起動跨ぎの永続検証が成立しないため、必ずファイル DB に作る。
func createSessionsTable(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	if _, err := sqlDB.Exec(`CREATE TABLE sessions (token TEXT PRIMARY KEY, data BLOB NOT NULL, expiry REAL NOT NULL)`); err != nil {
		t.Fatalf("create sessions: %v", err)
	}
	if _, err := sqlDB.Exec(`CREATE INDEX sessions_expiry_idx ON sessions (expiry)`); err != nil {
		t.Fatalf("create index: %v", err)
	}
}

// TestSessionManager_再起動跨ぎで管理を作り直しても既存トークンでユーザーを読み戻せる は、
// 同一 DB ファイルに対し「別プロセス相当」= 別 *sql.DB プール + 別 SessionManager を生成しても、
// 1 回目で発行した cookie トークンで user_id が読み戻せることを検証する回帰テスト (R3.2)。
//
// これは「再起動後のログイン維持」の実現主体が SESSION_SECRET (CSRF 認証鍵) ではなく
// DB セッションストア + DB ファイルの永続であること (design の用語明確化) を固定する。
// scs は cookie を署名せず不透明トークンを DB キーに用いるため、管理インスタンスや鍵を
// 共有しない 2 つ目の SessionManager でも、同じ DB を指していればセッションを読み戻せる。
//
// 回帰検知力 (偽陽性でないこと): 起動1で sm1.Commit を経ず (= sessions へ INSERT されず) に
// db2 で同トークンを Load すると UserIDFromSession は 0 を返し本テストは落ちる。すなわち本アサートは
// 「DB ファイルへ実際に永続された行を別接続が読めた」ことだけを true にする。
func TestSessionManager_再起動跨ぎで管理を作り直しても既存トークンでユーザーを読み戻せる(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "restart_session.sqlite")

	// --- 起動 1 回目: プール + セッション管理を生成し、ログインを実 DB ファイルへ Commit ---
	db1, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("起動1: NewPool: %v", err)
	}
	createSessionsTable(t, db1)

	sm1 := NewSessionManager(db1, &config.Config{AppEnv: "development"})
	ctx1, err := sm1.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("起動1: session load: %v", err)
	}
	// remember=true でブラウザを越えて永続する cookie 相当 (ただし DB 永続は cookie 永続フラグと独立)。
	if err := Login(ctx1, sm1, 42, true); err != nil {
		t.Fatalf("起動1: Login: %v", err)
	}
	token, _, err := sm1.Commit(ctx1)
	if err != nil {
		t.Fatalf("起動1: Commit(実DBへ永続化): %v", err)
	}
	if token == "" {
		t.Fatal("起動1: Commit がトークンを返さない")
	}

	// プロセス終了相当: cleanup goroutine を止め、1 回目のプールを完全に閉じる。
	// WAL に書かれたセッション行は DB ファイル状態の一部として 2 回目の接続から読める。
	if store, ok := sm1.Store.(*sqlite3store.SQLite3Store); ok {
		store.StopCleanup()
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("起動1: db close: %v", err)
	}

	// --- 起動 2 回目: 同一 DB ファイルへ別プール + 別 SessionManager を生成 (再起動相当) ---
	db2, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("起動2: NewPool: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
	sm2 := NewSessionManager(db2, &config.Config{AppEnv: "development"})
	if store, ok := sm2.Store.(*sqlite3store.SQLite3Store); ok {
		t.Cleanup(store.StopCleanup)
	}

	// 1 回目に発行した cookie トークンで読み戻す → user_id が維持されている (R3.2)。
	ctx2, err := sm2.Load(context.Background(), token)
	if err != nil {
		t.Fatalf("起動2: session reload: %v", err)
	}
	if got := UserIDFromSession(ctx2, sm2); got != 42 {
		t.Errorf("再起動後の UserIDFromSession = %d, want 42 (DB セッションが再起動を跨いで維持されていない)", got)
	}
}

// TestSessionManager_存在しないトークンは再起動後も未ログイン は、再起動後に未知/期限切れ相当の
// トークンを渡しても 0 (未ログイン) へ安全に誘導され、誤ってログイン扱いにならないことを固定する。
func TestSessionManager_存在しないトークンは再起動後も未ログイン(t *testing.T) {
	dsn := "file:" + filepath.Join(t.TempDir(), "restart_session_absent.sqlite")
	db, err := infradb.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	createSessionsTable(t, db)

	sm := NewSessionManager(db, &config.Config{AppEnv: "development"})
	if store, ok := sm.Store.(*sqlite3store.SQLite3Store); ok {
		t.Cleanup(store.StopCleanup)
	}

	ctx, err := sm.Load(context.Background(), "no-such-token")
	if err != nil {
		t.Fatalf("session load(未知トークン): %v", err)
	}
	if got := UserIDFromSession(ctx, sm); got != 0 {
		t.Errorf("未知トークンの UserIDFromSession = %d, want 0 (誤ってログイン扱いになっている)", got)
	}
}
