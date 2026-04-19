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

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
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

	// sqlc 生成リポジトリの初期化。ハンドラ追加時に受け渡す。
	q := repository.New(pool)
	_ = q

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "go_iot: 農業IoTシステムへようこそ")
	})

	// DB 疎通込みのヘルスチェック (Lightsail や ALB 等のヘルスチェック対象)
	e.GET("/health", func(c echo.Context) error {
		pingCtx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"status": "db_unreachable",
				"error":  err.Error(),
			})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	serverErrCh := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.AppPort)
		log.Printf("listening on %s (env=%s)", addr, cfg.AppEnv)
		if err := e.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	if err := e.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Println("shutdown complete")
	return nil
}
