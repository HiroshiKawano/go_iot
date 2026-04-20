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
	"github.com/HiroshiKawano/go_iot/internal/repository"
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

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// --- 公開エンドポイント ---
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "go_iot: 農業IoTシステムへようこそ")
	})

	// DB 疎通込みのヘルスチェック (Lightsail や ALB 等のヘルスチェック対象)
	r.GET("/health", func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "db_unreachable",
				"error":  err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// --- デバイスAPI ドキュメント (Scalar UI / OpenAPI 3.0) ---
	r.GET("/docs", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", docs.IndexHTML)
	})
	r.GET("/docs/openapi.yaml", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", docs.OpenAPIYAML)
	})

	// --- デバイスAPI (Bearer トークン認証) ---
	sensorAPI := &handler.SensorAPI{Repo: q}
	deviceAuth := auth.DeviceAuth(auth.DeviceAuthConfig{Repo: q})

	apiGroup := r.Group("/api", deviceAuth)
	apiGroup.POST("/sensor-data", sensorAPI.Create)

	// --- サーバ起動 / Graceful shutdown ---
	addr := fmt.Sprintf(":%d", cfg.AppPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
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
