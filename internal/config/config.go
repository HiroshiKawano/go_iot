package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config はアプリケーション全体で使用する設定値を保持する。
// 環境変数から読み込まれ、起動時にバリデーションされる。
type Config struct {
	AppEnv        string
	AppPort       int
	DatabaseURL   string
	SessionSecret string
}

// Load は環境変数を読み込んで Config を構築する。
// 必須項目が不足している場合はエラーを返す。
// .env の読み込みは Makefile (`include .env` + `export`) 側で行う前提。
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		AppPort:       getEnvInt("APP_PORT", 8080),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.SessionSecret == "" {
		missing = append(missing, "SESSION_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required env vars missing: %v (ヒント: .env ファイルを作成し make dev で起動してください)", missing)
	}

	if cfg.AppEnv == "production" && len(cfg.SessionSecret) < 32 {
		return nil, errors.New("SESSION_SECRET must be at least 32 chars in production")
	}

	return cfg, nil
}

// IsDevelopment は開発環境かどうかを判定する。
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
