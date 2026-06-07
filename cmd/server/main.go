package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/docs"
	"github.com/HiroshiKawano/go_iot/internal/handler"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/middleware"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/service"
	"github.com/HiroshiKawano/go_iot/internal/view"
	"github.com/alexedwards/scs/v2"
	"github.com/gin-gonic/gin"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	pool, err := infradb.NewPool(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	q := repository.New(pool)
	sm := auth.NewSessionManager(pool, cfg)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	httpHandler := newHTTPHandler(cfg, sm, q, pool.Ping)

	// --- サーバ起動 / Graceful shutdown ---
	addr := fmt.Sprintf(":%d", cfg.AppPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: httpHandler,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s (env=%s)", addr, cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal: %v, shutting down", sig)
	case err := <-serverErrCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Println("shutdown complete")
	return nil
}

// newHTTPHandler は全ルート (Web UI / デバイス API / 静的アセット / ドキュメント / ヘルス) を
// 配線し、http.Handler 層のミドルウェアで包んで返す合成ルート。
//
// 合成順 (外側→内側): MethodOverride → scs LoadAndSave → gin.Engine。
// Gin はミドルウェア前にメソッドでルーティングするため、メソッド上書きと
// セッション load/save は engine の外側 (http.Handler 層) で適用する。
// CSRF はデバイス API を除外するため Web ルートグループ限定の gin ミドルウェアとする。
func newHTTPHandler(cfg *config.Config, sm *scs.SessionManager, q repository.Querier, ping func(ctx context.Context) error) http.Handler {
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	// ヘルスチェック (DB 疎通込み・セッション/CSRF 不要)
	engine.GET("/health", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unreachable", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// デバイス API ドキュメント (Scalar UI / OpenAPI)
	engine.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", docs.IndexHTML)
	})
	engine.GET("/docs/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPIYAML)
	})

	// 静的アセット (go:embed → /static)
	view.MountStatic(engine)

	// デバイス API (Bearer トークン認証・CSRF 対象外)
	// アラート判定サービスを注入 (受信時に同期評価)。q (Querier) が AlertEvaluatorRepo を満たす。
	sensorAPI := &handler.SensorAPI{Repo: q, Evaluator: &service.AlertEvaluator{Repo: q}}
	deviceAuth := auth.DeviceAuth(auth.DeviceAuthConfig{Repo: q})
	apiGroup := engine.Group("/api", deviceAuth)
	apiGroup.POST("/sensor-data", sensorAPI.Create)

	// Web UI (Session 認証 + CSRF)
	authH := &handler.AuthHandler{Repo: q, SM: sm}
	web := engine.Group("/", middleware.SessionLoad(sm), middleware.CSRF(cfg))
	web.GET("/", authH.Root)
	web.GET("/login", authH.LoginGet)
	web.POST("/login", authH.LoginPost)
	web.GET("/register", authH.RegisterGet)
	web.POST("/register", authH.RegisterPost)
	web.POST("/logout", authH.Logout)
	web.GET("/dashboard", middleware.RequireAuth(), authH.Dashboard)

	// デバイス登録・編集 (Session 認証 + CSRF + 所有者認可)。
	// 静的経路 /devices/create とパラメータ経路 /devices/:device を共存させる。
	// 編集の更新は hidden _method=put → 外側の MethodOverride が PUT へ上書きしてルーティング。
	deviceH := &handler.DeviceHandler{Repo: q}
	web.GET("/devices/create", middleware.RequireAuth(), deviceH.ShowCreateForm)
	web.POST("/devices", middleware.RequireAuth(), deviceH.Create)
	web.GET("/devices/:device/edit", middleware.RequireAuth(), deviceH.ShowEditForm)
	web.PUT("/devices/:device", middleware.RequireAuth(), deviceH.Update)

	// デバイス詳細 (device-detail): 詳細表示・期間切替フラグメント・論理削除。
	// 静的 /devices/create と同じ階層のパラメータ node ":device" に GET/DELETE ハンドラを追加する
	// (:device は /edit・PUT で既存のため node は既存。Gin は静的 create を優先解決)。
	// 削除は HTMX の真の DELETE と、非 HTMX フォーム (_method=delete → 外側 MethodOverride) の
	// 両経路が同一 deviceH.Delete に到達する。
	web.GET("/devices/:device", middleware.RequireAuth(), deviceH.Show)
	web.GET("/devices/:device/chart", middleware.RequireAuth(), deviceH.Chart)
	web.DELETE("/devices/:device", middleware.RequireAuth(), deviceH.Delete)

	// 合成: メソッド上書き (ルーティング前) → セッション load/save → engine
	return middleware.MethodOverride(sm.LoadAndSave(engine))
}
