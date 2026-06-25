// session_auth.go は Web UI 用の Session 認証 (scs + PostgreSQL ストア) を提供する。
// device_auth.go (デバイス Bearer 認証) と対になる authN 実装で、
// ログイン後の user_id は Gin コンテキストへは middleware.SessionLoad が橋渡しし、
// ダウンストリームは認証方式に依らず UserID(c) で取得する。
package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sessionKeyUserID はセッションに格納するログインユーザー ID のキー。
const sessionKeyUserID = "user_id"

// sessionKeyFlash は遷移先で 1 回だけ表示するフラッシュメッセージのキー。
const sessionKeyFlash = "flash"

// NewSessionManager は scs の SessionManager を PostgreSQL ストア (pgxstore) で構築する。
// セッションテーブル sessions は migration で作成済みであることを前提とする。
//
// scs は不透明なランダムトークンを cookie に用い、cookie 自体は署名しない
// (SESSION_SECRET は scs では使用しない。CSRF 側で利用する)。
func NewSessionManager(pool *pgxpool.Pool, cfg *config.Config) *scs.SessionManager {
	sm := scs.New()
	sm.Store = pgxstore.New(pool)
	applySessionPolicy(sm, cfg)
	return sm
}

// applySessionPolicy は cookie / 有効期限のセキュリティ方針を適用する。
// 本番 (production) のみ Secure cookie を有効化する (HTTPS 前提)。
func applySessionPolicy(sm *scs.SessionManager, cfg *config.Config) {
	sm.Lifetime = 24 * time.Hour
	sm.Cookie.Name = "session"
	sm.Cookie.Path = "/"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Persist = false // 既定はブラウザセッション限り。remember 指定時のみ永続化する
	sm.Cookie.Secure = cfg.AppEnv == "production"
}

// Login はログイン成功時のセッション確立を行う。
// セッション固定攻撃を防ぐためトークンを再生成してから user_id を格納し、
// remember 指定時はブラウザセッションを越えて cookie を永続化する。
func Login(ctx context.Context, sm *scs.SessionManager, userID int64, remember bool) error {
	if err := sm.RenewToken(ctx); err != nil {
		return fmt.Errorf("renew session token: %w", err)
	}
	sm.Put(ctx, sessionKeyUserID, userID)
	if remember {
		sm.RememberMe(ctx, true)
	}
	return nil
}

// Logout はセッションを破棄する (値の全消去 + 次回 commit で cookie 削除)。
func Logout(ctx context.Context, sm *scs.SessionManager) error {
	if err := sm.Destroy(ctx); err != nil {
		return fmt.Errorf("destroy session: %w", err)
	}
	return nil
}

// UserIDFromSession はセッションからログインユーザー ID を取得する。
// 未ログイン (未設定) の場合は 0 を返す。
func UserIDFromSession(ctx context.Context, sm *scs.SessionManager) int64 {
	return sm.GetInt64(ctx, sessionKeyUserID)
}

// PutFlash は遷移先で 1 回だけ表示するフラッシュメッセージを session に格納する。
// HX-Redirect / c.Redirect のいずれの遷移でも、遷移「前」に呼ぶことで遷移先の
// PopFlash で読み出せる。HX-Redirect は本文なし応答だが、scs の LoadAndSave が
// 応答時に store へ確定するため遷移先リクエストへ運ばれる
// (詳細は 2cc_sdd/HTMX実装ガイド(動的).md §9.1)。
func PutFlash(ctx context.Context, sm *scs.SessionManager, msg string) {
	sm.Put(ctx, sessionKeyFlash, msg)
}

// PopFlash はフラッシュメッセージを読み取り、同時に session から削除する (1 回限り表示)。
// メッセージが無い場合は空文字を返す。
func PopFlash(ctx context.Context, sm *scs.SessionManager) string {
	return sm.PopString(ctx, sessionKeyFlash)
}
