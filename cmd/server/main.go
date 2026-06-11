package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/applog"
	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/desktop"
	"github.com/HiroshiKawano/go_iot/internal/docs"
	"github.com/HiroshiKawano/go_iot/internal/handler"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/mdns"
	"github.com/HiroshiKawano/go_iot/internal/middleware"
	"github.com/HiroshiKawano/go_iot/internal/migrate"
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
	// ログ出力先を決定し、標準ロガーと gin の出力を差し替える。
	// Windows GUI ビルド (ldflags で applog.Mode=file) や LOG_FILE 指定時はファイルへ、
	// 既定 (console) では標準出力へ出力する。gin.Logger() は生成時に gin.DefaultWriter を
	// 取り込むため、newHTTPHandler より前に設定する必要がある。
	logWriter, closeLog, err := applog.Setup(applog.Destination(applog.Mode, os.Getenv("LOG_FILE"), applog.DefaultPath))
	if err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	defer func() { _ = closeLog() }()
	log.SetOutput(logWriter)
	gin.DefaultWriter = logWriter
	gin.DefaultErrorWriter = logWriter

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// DB をオープンし、listen 開始前にスキーマを自動適用する (R4)。
	// マイグレーション失敗は致命とし、起動を中断する (fail fast・R4.4)。
	pool, err := openAndMigrate(rootCtx, cfg.DatabaseURL, migrate.Up)
	if err != nil {
		return err
	}
	defer pool.Close()
	log.Printf("database ready: %s", cfg.DatabaseURL)

	q := repository.New(pool)
	sm := auth.NewSessionManager(pool, cfg)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// health は DB 疎通を ctx 付きで確認するため PingContext を渡す
	// (*sql.DB.Ping は func() error で newHTTPHandler の func(ctx) error と非互換)。
	httpHandler := newHTTPHandler(cfg, sm, q, pool.PingContext)

	// --- サーバ起動 / Graceful shutdown ---
	// ポートを自動採番し (既定ポート競合時は空きポート)、取得した listener で Serve する (R5.3/5.4)。
	ln, actualPort, err := desktop.Listen(cfg.AppPort)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: httpHandler}
	log.Printf("起動環境: env=%s", cfg.AppEnv)
	serverErrCh := startServer(srv, ln, actualPort, desktop.OpenBrowser)

	// 安定ホスト名 go-iot.local を mDNS で公開する (R7)。開始失敗は非致命 (IP 直打ちで到達可)。
	// actualPort (自動採番後の実ポート) 渡しと defer Stop は run() 直読みで担保する
	// (run() 全体は signal 待ちのため統合テスト対象外。startAdvertiser の引数/非致命性と
	// adv.Stop の安全性は各々ユニットで検証済み)。
	adv := mdns.New()
	startAdvertiser(rootCtx, adv, mdns.HostName, actualPort)
	defer adv.Stop()

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

// openAndMigrate は DB をオープンし、listen 開始前にマイグレーションを冪等適用する。
// マイグレーション失敗時は pool を閉じてエラーを返し、呼び出し元 (run) が起動を中断する
// (fail fast・R4.4)。migrateUp を引数化することで、失敗時の起動中断をテスト可能にする。
func openAndMigrate(ctx context.Context, dsn string, migrateUp func(*sql.DB) error) (*sql.DB, error) {
	pool, err := infradb.NewPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}
	if err := migrateUp(pool); err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return pool, nil
}

// startServer は確定済み listener でサーバを goroutine で起動し、実ポートをログ出力後に
// 既定ブラウザで localhost URL を開く (R5.1/5.4)。ブラウザ起動失敗は非致命とし、URL をログに
// 提示して継続する (mDNS の go-iot.local は ESP32 向けで別系統。本 URL は同端末向け localhost)。
// openURL を引数化することでブラウザ起動配線をテスト可能にする。
//
// Serve のエラーは errCh 経由で run() の select が回収する。listener は desktop.Listen で
// 確立済みのため、bind 起因の即時失敗はここでは起きない (起きるのは shutdown 時の ErrServerClosed のみ)。
func startServer(srv *http.Server, ln net.Listener, port int, openURL func(string) error) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	url := fmt.Sprintf("http://localhost:%d", port)
	log.Printf("待受開始: %s", url)
	if err := openURL(url); err != nil {
		log.Printf("ブラウザ自動起動に失敗しました（手動で %s を開いてください）: %v", url, err)
	}
	return errCh
}

// startAdvertiser は listen・実ポート確定後に mDNS 公開を開始する (R7)。
// 開始失敗は致命でなく、URL/IP 直打ちで到達可能なためログのみで継続する (R7.1)。
// adv.Stop() は未 Start でも安全なため、呼び出し側は常に defer adv.Stop() してよい。
func startAdvertiser(ctx context.Context, adv mdns.Advertiser, hostname string, port int) {
	if err := adv.Start(ctx, hostname, port); err != nil {
		log.Printf("mDNS 公開の開始に失敗しました（IP 直打ちで到達可能・継続します）: %v", err)
	}
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

	// センサーデータ履歴 (sensor-readings-history): 期間フィルタ + 集計 + 一覧 (20件/ページ)。
	// :device 配下の子経路 (/edit・/chart と同階層) のため既存 node を共有し競合しない。
	// デバイス詳細の「もっと見る」(/devices/{id}/readings) からの遷移先。検索/ページ送りは
	// HTMX 部分更新 (フラグメント #device-readings-list)。GET のみのため CSRF 検証対象外。
	readingsH := &handler.ReadingsHandler{Repo: q}
	web.GET("/devices/:device/readings", middleware.RequireAuth(), readingsH.Index)

	// アラートルール管理 (alert-rules): デバイスごとのルールをインライン CRUD する 6 ルート。
	// 初期表示/デバイス切替(GET)・追加(POST)・編集読込(GET)・更新(PUT)・有効切替(PATCH)・削除(DELETE)。
	// 全ルート RequireAuth。ミューテーション(POST/PUT/PATCH/DELETE)は web グループの CSRF で保護され、
	// HTMX は実メソッドを、no-JS は POST + _method を MethodOverride が昇格する。HTMX 部分更新。
	alertRuleH := &handler.AlertRuleHandler{Repo: q}
	web.GET("/alerts/rules", middleware.RequireAuth(), alertRuleH.Index)
	web.POST("/alerts/rules", middleware.RequireAuth(), alertRuleH.Add)
	web.GET("/alerts/rules/:rule/edit", middleware.RequireAuth(), alertRuleH.Edit)
	web.PUT("/alerts/rules/:rule", middleware.RequireAuth(), alertRuleH.Update)
	web.PATCH("/alerts/rules/:rule/toggle", middleware.RequireAuth(), alertRuleH.Toggle)
	web.DELETE("/alerts/rules/:rule", middleware.RequireAuth(), alertRuleH.Delete)

	// アラート履歴 (alert-history): 発火済みアラートの時系列一覧 + デバイス/期間フィルタ + ページ送り。
	// /alerts/rules 群に隣接する静的経路。初期表示=フルページ、検索/ページ送りは HTMX 部分更新
	// (フラグメント #alert-history-list)。GET のみのため CSRF 検証対象外。テナント分離はクエリの
	// d.user_id スコープに集約 (authz 非経由・design Decision 4)。
	alertHistoryH := &handler.AlertHistoryHandler{Repo: q}
	web.GET("/alerts/history", middleware.RequireAuth(), alertHistoryH.Index)

	// 合成: メソッド上書き (ルーティング前) → セッション load/save → engine
	return middleware.MethodOverride(sm.LoadAndSave(engine))
}
