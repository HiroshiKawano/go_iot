// 開発用の初期データ投入ツール。
// 実行前に `make up` + `make migrate-up` でスキーマが適用されている必要がある。
//
// 使い方:
//
//	make seed
//
// 実行すると以下を実施する (冪等):
//  1. アプリケーションテーブルを TRUNCATE (goose_db_version は温存)
//  2. テストユーザー 1 名
//  3. デバイス 2 台 (ハウスA / ハウスB)
//  4. 各デバイス直近24時間分のセンサーデータ (5分間隔, 計576件)
//  5. 各デバイスのアラートルール 2 件
//  6. アラート履歴 3 件
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand/v2"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	"github.com/HiroshiKawano/go_iot/internal/domain"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("seed failed: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := truncateAll(ctx, pool); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}

	q := repository.New(pool)

	user, err := seedUser(ctx, q)
	if err != nil {
		return fmt.Errorf("seed user: %w", err)
	}
	log.Printf("  ✓ user: id=%d email=%s", user.ID, user.Email)

	devices, err := seedDevices(ctx, q, user.ID)
	if err != nil {
		return fmt.Errorf("seed devices: %w", err)
	}
	for _, d := range devices {
		log.Printf("  ✓ device: id=%d name=%s mac=%s", d.ID, d.Name, d.MacAddress)
	}

	readingCount := 0
	for _, d := range devices {
		n, err := seedSensorReadings(ctx, q, d.ID)
		if err != nil {
			return fmt.Errorf("seed readings for device %d: %w", d.ID, err)
		}
		readingCount += n
	}
	log.Printf("  ✓ sensor_readings: %d 件", readingCount)

	rules, err := seedAlertRules(ctx, q, devices)
	if err != nil {
		return fmt.Errorf("seed alert rules: %w", err)
	}
	log.Printf("  ✓ alert_rules: %d 件", len(rules))

	histCount, err := seedAlertHistories(ctx, q, rules)
	if err != nil {
		return fmt.Errorf("seed alert histories: %w", err)
	}
	log.Printf("  ✓ alert_histories: %d 件", histCount)

	log.Println("seed complete")
	return nil
}

func truncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	// RESTART IDENTITY で BIGSERIAL も 1 から振り直す。
	// goose_db_version は対象外 (マイグレーション履歴を保持)。
	_, err := pool.Exec(ctx, `
		TRUNCATE TABLE
		  alert_histories,
		  alert_rules,
		  sensor_readings,
		  device_tokens,
		  devices,
		  users
		RESTART IDENTITY CASCADE
	`)
	return err
}

func seedUser(ctx context.Context, q *repository.Queries) (repository.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		return repository.User{}, err
	}
	return q.CreateUser(ctx, repository.CreateUserParams{
		Name:         "テストユーザー",
		Email:        "test@example.com",
		PasswordHash: string(hash),
	})
}

func seedDevices(ctx context.Context, q *repository.Queries, userID int64) ([]repository.Device, error) {
	specs := []struct {
		name     string
		mac      string
		location string
	}{
		{"ハウスA温湿度計", "AA:BB:CC:DD:EE:01", "ビニールハウスA"},
		{"ハウスB温湿度計", "AA:BB:CC:DD:EE:02", "ビニールハウスB"},
	}

	out := make([]repository.Device, 0, len(specs))
	for _, s := range specs {
		loc := s.location
		d, err := q.CreateDevice(ctx, repository.CreateDeviceParams{
			UserID:     userID,
			Name:       s.name,
			MacAddress: s.mac,
			Location:   &loc,
			IsActive:   true,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

func seedSensorReadings(ctx context.Context, q *repository.Queries, deviceID int64) (int, error) {
	// 直近24時間を 5 分間隔で生成 (24h * 60min / 5min = 288 件)
	const interval = 5 * time.Minute
	const samples = 24 * 60 / 5

	// 日周変化をシミュレート: 温度は深夜 20℃ → 昼 30℃、湿度は逆位相
	end := time.Now().UTC().Truncate(interval)
	rng := rand.New(rand.NewPCG(uint64(deviceID), 0xCAFE))

	for i := 0; i < samples; i++ {
		recordedAt := end.Add(-interval * time.Duration(samples-1-i))
		hour := float64(recordedAt.Hour()) + float64(recordedAt.Minute())/60.0

		// 温度: 25℃ ± 5℃ の日周変化 + 揺らぎ
		baseTemp := 25.0 + 5.0*math.Sin((hour-6)/24.0*2*math.Pi)
		temp := baseTemp + (rng.Float64()-0.5)*0.6
		// 湿度: 60% ± 15% の逆位相
		baseHum := 60.0 - 15.0*math.Sin((hour-6)/24.0*2*math.Pi)
		hum := baseHum + (rng.Float64()-0.5)*3.0

		_, err := q.CreateSensorReading(ctx, repository.CreateSensorReadingParams{
			DeviceID:    deviceID,
			Temperature: numeric2(temp),
			Humidity:    numeric2(clamp(hum, 0, 100)),
			RecordedAt:  timestamptz(recordedAt),
		})
		if err != nil {
			return 0, err
		}
	}
	return samples, nil
}

func seedAlertRules(ctx context.Context, q *repository.Queries, devices []repository.Device) ([]repository.AlertRule, error) {
	out := make([]repository.AlertRule, 0, len(devices)*2)
	for _, d := range devices {
		// 温度 > 35℃
		r1, err := q.CreateAlertRule(ctx, repository.CreateAlertRuleParams{
			DeviceID:  d.ID,
			Metric:    string(domain.MetricTemperature),
			Operator:  string(domain.OpGreaterThan),
			Threshold: numeric2(35.00),
			IsEnabled: true,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, r1)

		// 湿度 < 30%
		r2, err := q.CreateAlertRule(ctx, repository.CreateAlertRuleParams{
			DeviceID:  d.ID,
			Metric:    string(domain.MetricHumidity),
			Operator:  string(domain.OpLessThan),
			Threshold: numeric2(30.00),
			IsEnabled: true,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, r2)
	}
	return out, nil
}

func seedAlertHistories(ctx context.Context, q *repository.Queries, rules []repository.AlertRule) (int, error) {
	if len(rules) < 2 {
		return 0, nil
	}

	now := time.Now().UTC()
	// 3 件のサンプル履歴 (ハウスA温度上限発火 2 件 + ハウスB湿度下限発火 1 件)
	specs := []struct {
		rule     repository.AlertRule
		actual   float64
		at       time.Time
		notified bool
	}{
		{rules[0], 38.50, now.Add(-1 * time.Hour), false},
		{rules[0], 36.20, now.Add(-6 * time.Hour), true},
		{rules[3], 25.00, now.Add(-2 * time.Hour), false},
	}

	for _, s := range specs {
		hist, err := q.CreateAlertHistory(ctx, repository.CreateAlertHistoryParams{
			AlertRuleID: s.rule.ID,
			Metric:      s.rule.Metric,
			Operator:    s.rule.Operator,
			Threshold:   s.rule.Threshold,
			ActualValue: numeric2(s.actual),
			TriggeredAt: timestamptz(s.at),
		})
		if err != nil {
			return 0, err
		}
		if s.notified {
			if err := q.MarkAlertHistoryNotified(ctx, hist.ID); err != nil {
				return 0, err
			}
		}
	}
	return len(specs), nil
}

// numeric2 は float を NUMERIC(5,2) 相当に変換する。
// 小数 2 桁に丸め (Exp=-2) て pgtype.Numeric を構築する。
func numeric2(f float64) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(math.Round(f * 100))),
		Exp:   -2,
		Valid: true,
	}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
