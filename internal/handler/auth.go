// auth.go は Web UI の認証フロー (login/register/logout/dashboard/root) を担う。
// 認証は scs セッションで行い、user_id は middleware.SessionLoad が Gin コンテキストへ
// 橋渡しした値 (auth.UserID) を参照する。templ ページを直接返す SSR ハンドラ。
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/HiroshiKawano/go_iot/internal/view/layout"
	"github.com/HiroshiKawano/go_iot/internal/view/page"
	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/csrf"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// authFailedMessage はログイン失敗時の共通メッセージ。
// 不在・不一致を区別せずユーザー列挙攻撃を防ぐ。
const authFailedMessage = "メールアドレスまたはパスワードが間違っています"

// AuthRepo は AuthHandler が必要とする最小の DB ポート。repository.Querier が満たす。
type AuthRepo interface {
	GetUserByEmail(ctx context.Context, email string) (repository.User, error)
	CreateUser(ctx context.Context, arg repository.CreateUserParams) (repository.User, error)
	GetUser(ctx context.Context, id int64) (repository.User, error)
}

// AuthHandler は認証フローのハンドラ群を提供する。
type AuthHandler struct {
	Repo AuthRepo
	SM   *scs.SessionManager
}

type loginForm struct {
	Email    string `form:"email" binding:"required,email"`
	Password string `form:"password" binding:"required"`
	Remember bool   `form:"remember"`
}

type registerForm struct {
	Name                 string `form:"name" binding:"required,max=255"`
	Email                string `form:"email" binding:"required,email"`
	Password             string `form:"password" binding:"required,min=8"`
	PasswordConfirmation string `form:"password_confirmation" binding:"required,eqfield=Password"`
}

// LoginGet はログインページを表示する。認証済みなら /dashboard へ。
func (h *AuthHandler) LoginGet(c *gin.Context) {
	if auth.UserID(c) > 0 {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	renderPage(c, http.StatusOK, page.LoginPage(page.LoginView{
		CSSURL:    view.CSSURL(),
		CSRFToken: csrf.Token(c.Request),
	}))
}

// LoginPost はメール+パスワードを照合し、成功時にセッションを確立する。
func (h *AuthHandler) LoginPost(c *gin.Context) {
	var form loginForm
	if err := c.ShouldBind(&form); err != nil {
		renderPage(c, http.StatusOK, page.LoginPage(page.LoginView{
			CSSURL:    view.CSSURL(),
			CSRFToken: csrf.Token(c.Request),
			Email:     form.Email,
			Errors:    translateValidationErrors(err),
		}))
		return
	}

	user, err := h.Repo.GetUserByEmail(c.Request.Context(), form.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			h.renderLoginFailed(c, form.Email)
			return
		}
		renderError(c, http.StatusInternalServerError)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(form.Password)) != nil {
		h.renderLoginFailed(c, form.Email)
		return
	}

	if err := auth.Login(c.Request.Context(), h.SM, user.ID, form.Remember); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (h *AuthHandler) renderLoginFailed(c *gin.Context, email string) {
	renderPage(c, http.StatusOK, page.LoginPage(page.LoginView{
		CSSURL:    view.CSSURL(),
		CSRFToken: csrf.Token(c.Request),
		Email:     email,
		Errors:    map[string]string{"form": authFailedMessage},
	}))
}

// RegisterGet はユーザー登録ページを表示する。認証済みなら /dashboard へ。
func (h *AuthHandler) RegisterGet(c *gin.Context) {
	if auth.UserID(c) > 0 {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	renderPage(c, http.StatusOK, page.RegisterPage(page.RegisterView{
		CSSURL:    view.CSSURL(),
		CSRFToken: csrf.Token(c.Request),
	}))
}

// RegisterPost は入力検証・重複確認の後にユーザーを作成し、自動ログインする。
func (h *AuthHandler) RegisterPost(c *gin.Context) {
	var form registerForm
	if err := c.ShouldBind(&form); err != nil {
		h.renderRegister(c, form, translateValidationErrors(err))
		return
	}

	// メール重複チェック (UNIQUE 索引が最終防衛線、ここでは事前検出してフォームに表示)
	if _, err := h.Repo.GetUserByEmail(c.Request.Context(), form.Email); err == nil {
		h.renderRegister(c, form, map[string]string{"email": "このメールアドレスは既に登録されています"})
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		renderError(c, http.StatusInternalServerError)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(form.Password), bcrypt.DefaultCost)
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	created, err := h.Repo.CreateUser(c.Request.Context(), repository.CreateUserParams{
		Name:         form.Name,
		Email:        form.Email,
		PasswordHash: string(hash),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	if err := auth.Login(c.Request.Context(), h.SM, created.ID, false); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (h *AuthHandler) renderRegister(c *gin.Context, form registerForm, errs map[string]string) {
	renderPage(c, http.StatusOK, page.RegisterPage(page.RegisterView{
		CSSURL:    view.CSSURL(),
		CSRFToken: csrf.Token(c.Request),
		Name:      form.Name,
		Email:     form.Email,
		Errors:    errs,
	}))
}

// Logout はセッションを破棄して /login へ戻す。
func (h *AuthHandler) Logout(c *gin.Context) {
	if err := auth.Logout(c.Request.Context(), h.SM); err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	c.Redirect(http.StatusSeeOther, "/login")
}

// Dashboard は認証後のトップ画面を表示する (RequireAuth 適用前提)。
func (h *AuthHandler) Dashboard(c *gin.Context) {
	user, err := h.Repo.GetUser(c.Request.Context(), auth.UserID(c))
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}
	renderPage(c, http.StatusOK, page.DashboardPage(layout.AppLayoutData{
		Title:     "ダッシュボード - 農業IoTシステム",
		UserName:  user.Name,
		CSRFToken: csrf.Token(c.Request),
		CSSURL:    view.CSSURL(),
	}))
}

// Root はルート (/) を認証状態に応じて振り分ける。
func (h *AuthHandler) Root(c *gin.Context) {
	if auth.UserID(c) > 0 {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.Redirect(http.StatusFound, "/login")
}

// renderPage は templ コンポーネントを HTML として描画する。
func renderPage(c *gin.Context, status int, comp templ.Component) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(status)
	_ = comp.Render(c.Request.Context(), c.Writer)
}

// renderError は機密情報を含まない簡潔なエラー応答を返す。
func renderError(c *gin.Context, status int) {
	c.String(status, "エラーが発生しました。時間をおいて再度お試しください。")
}
