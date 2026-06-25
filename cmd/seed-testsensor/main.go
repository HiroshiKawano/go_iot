// 表示テスト用の「沖縄テストセンサー」とその長期計測データを投入する開発ツール。
//
// 目的:
//   実機センサーは数時間分のデータしか無く、3日/7日/30日の期間表示を検証できない。
//   そこで沖縄(那覇)の気候平年値に基づく 5/1〜現在 の温湿度データを生成し、
//   期間グラフの見栄えを確認できるようにする。
//
// 重要な設計方針 (既存データ保護):
//   - 全テーブルを TRUNCATE する cmd/seed とは異なり、本ツールは一切 TRUNCATE しない。
//   - 既存デバイス・実センサーの計測には触れず、専用の「テストセンサー（沖縄）」
//     (MAC: AA:BB:CC:DD:EE:03) を別デバイスとして追加し、そのデバイスにのみ投入する。
//   - 再実行は冪等: テストデバイスが既にあれば、そのデバイスの計測のみ削除して作り直す
//     (他デバイスの計測は対象外)。
//
// 使い方:
//
//	go run ./cmd/seed-testsensor          # 既定 (10分間隔・5/1〜現在)
//	go run ./cmd/seed-testsensor -interval=5m
//
// 前提: make up + make migrate-up 済み。最低 1 名のユーザーが存在すること。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand/v2"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/config"
	infradb "github.com/HiroshiKawano/go_iot/internal/infra/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// テストセンサーの識別情報 (この MAC は seed の AA:BB:CC:DD:EE:01/02 と衝突しない)。
const (
	testSensorMAC      = "AA:BB:CC:DD:EE:03"
	testSensorName     = "テストセンサー（沖縄）"
	testSensorLocation = "沖縄県那覇市（表示テスト用）"
)

// jst は計測時刻を組み立てる際の基準タイムゾーン (日本標準時)。
// DB セッション TZ も Asia/Tokyo のため、DATE(recorded_at) の日境界と一致する。
var jst = time.FixedZone("JST", 9*60*60)

func main() {
	interval := flag.Duration("interval", 10*time.Minute, "計測サンプルの間隔 (例: 10m, 5m)")
	flag.Parse()

	if err := run(*interval); err != nil {
		log.Fatalf("seed-testsensor failed: %v", err)
	}
}

func run(interval time.Duration) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	userID, err := firstUserID(ctx, pool)
	if err != nil {
		return fmt.Errorf("ユーザー取得: %w (先に make seed 等でユーザーを作成してください)", err)
	}

	deviceID, reused, err := ensureTestDevice(ctx, pool, userID)
	if err != nil {
		return fmt.Errorf("テストデバイス準備: %w", err)
	}
	if reused {
		log.Printf("  ✓ 既存テストデバイスを再利用: id=%d (既存の計測のみ削除して作り直し)", deviceID)
	} else {
		log.Printf("  ✓ テストデバイスを新規作成: id=%d name=%s mac=%s", deviceID, testSensorName, testSensorMAC)
	}

	// 生成範囲: 5/1 00:00 JST 〜 現在。期間グラフ(3d/7d/30d)はすべてこの範囲に収まる。
	now := time.Now()
	start := time.Date(now.In(jst).Year(), time.May, 1, 0, 0, 0, 0, jst)

	rows, lastAt := generateReadings(deviceID, start, now, interval)

	if err := copyReadings(ctx, pool, rows); err != nil {
		return fmt.Errorf("計測データ投入: %w", err)
	}
	if err := updateLastCommunicated(ctx, pool, deviceID, lastAt); err != nil {
		return fmt.Errorf("最終通信日時更新: %w", err)
	}

	log.Printf("  ✓ sensor_readings: %d 件 (%s 〜 %s, 間隔 %s)",
		len(rows),
		start.Format("2006-01-02 15:04"),
		lastAt.In(jst).Format("2006-01-02 15:04"),
		interval)
	log.Printf("seed-testsensor 完了 — 詳細画面: /devices/%d", deviceID)
	return nil
}

// firstUserID はテストデバイスの所有者となる最小 id のユーザーを返す。
func firstUserID(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&id)
	return id, err
}

// ensureTestDevice はテストセンサーを用意し、その device_id と「再利用か否か」を返す。
// 既存(論理削除されていない)テストデバイスがあれば、その計測のみ物理削除して作り直す
// (他デバイスの計測には触れない)。無ければ新規作成する。
func ensureTestDevice(ctx context.Context, pool *pgxpool.Pool, userID int64) (deviceID int64, reused bool, err error) {
	err = pool.QueryRow(ctx,
		`SELECT id FROM devices WHERE mac_address = $1 AND deleted_at IS NULL`,
		testSensorMAC,
	).Scan(&deviceID)

	switch {
	case err == nil:
		// 既存テストデバイスの計測のみクリア (TRUNCATE しない・他デバイス非対象)。
		if _, derr := pool.Exec(ctx, `DELETE FROM sensor_readings WHERE device_id = $1`, deviceID); derr != nil {
			return 0, false, derr
		}
		return deviceID, true, nil
	case err == pgx.ErrNoRows:
		loc := testSensorLocation
		ierr := pool.QueryRow(ctx, `
			INSERT INTO devices (user_id, name, mac_address, location, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id`,
			userID, testSensorName, testSensorMAC, loc,
		).Scan(&deviceID)
		if ierr != nil {
			return 0, false, ierr
		}
		return deviceID, false, nil
	default:
		return 0, false, err
	}
}

// generateReadings は start〜end を interval 刻みで走査し、沖縄の気候モデルに基づく
// 計測行 ([]any{device_id, temperature, humidity, recorded_at}) を生成する。
// 返り値の lastAt は最終サンプルの時刻 (last_communicated_at 用)。
func generateReadings(deviceID int64, start, end time.Time, interval time.Duration) (rows [][]any, lastAt time.Time) {
	model := newOkinawaClimate(start)

	rows = make([][]any, 0, int(end.Sub(start)/interval)+1)
	for t := start; !t.After(end); t = t.Add(interval) {
		temp, hum := model.sample(t)
		rows = append(rows, []any{
			deviceID,
			numeric2(temp),
			numeric2(clamp(hum, 0, 100)),
			pgtype.Timestamptz{Time: t, Valid: true},
		})
		lastAt = t
	}
	return rows, lastAt
}

// copyReadings は COPY (pgx CopyFrom) でまとめて高速投入する。
// created_at / updated_at は列指定しないため DB デフォルト (now()) が入る。
func copyReadings(ctx context.Context, pool *pgxpool.Pool, rows [][]any) error {
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"sensor_readings"},
		[]string{"device_id", "temperature", "humidity", "recorded_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

// updateLastCommunicated はダッシュボード/詳細表示の「最終通信」を最新サンプル時刻に合わせる。
func updateLastCommunicated(ctx context.Context, pool *pgxpool.Pool, deviceID int64, at time.Time) error {
	_, err := pool.Exec(ctx,
		`UPDATE devices SET last_communicated_at = $1, updated_at = now() WHERE id = $2`,
		pgtype.Timestamptz{Time: at, Valid: true}, deviceID,
	)
	return err
}

// --- 沖縄(那覇)気候モデル -------------------------------------------------

// okinawaClimate は那覇の平年値(気象庁 1991-2020)に基づく温湿度生成器。
//
//	平年値: 5月 平均24.2℃/最高27.0/最低22.1/湿度78%、6月 平均27.2℃/最高29.8/最低25.2/湿度83%
//
// これを「季節トレンド(日次線形補間)＋日較差(日周sin)＋天候ゆらぎ(日単位)＋微小ノイズ」で再現する。
type okinawaClimate struct {
	start time.Time
	// dayWeatherTemp[i] は i 日目の天候による気温オフセット(℃)。曇雨で下振れ。
	dayWeatherTemp []float64
	// dayWeatherHum[i] は i 日目の湿度オフセット(%)。気温と逆相関 + 独自ノイズ。
	dayWeatherHum []float64
	rng           *rand.Rand
}

const climateMaxDays = 370 // 1年分の天候オフセットを事前生成(範囲外参照防止)

func newOkinawaClimate(start time.Time) *okinawaClimate {
	// 固定シードで再現性を確保(再実行で同じ系列)。
	rng := rand.New(rand.NewPCG(20260501, 0x5EED))
	dayT := make([]float64, climateMaxDays)
	dayH := make([]float64, climateMaxDays)
	for i := range dayT {
		// 天候による気温の上下動 (標準偏差 ~1.3℃)。
		wt := rng.NormFloat64() * 1.3
		dayT[i] = wt
		// 湿度は気温と逆相関(寒い日=雨=多湿) + 独自ノイズ。
		dayH[i] = -2.2*wt + rng.NormFloat64()*3.0
	}
	return &okinawaClimate{start: start, dayWeatherTemp: dayT, dayWeatherHum: dayH, rng: rng}
}

// sample は時刻 t における (温度℃, 湿度%) を返す。
func (m *okinawaClimate) sample(t time.Time) (temp, hum float64) {
	lt := t.In(jst)
	daysSinceStart := lt.Sub(m.start).Hours() / 24.0
	dayIdx := int(daysSinceStart)
	if dayIdx < 0 {
		dayIdx = 0
	}
	if dayIdx >= climateMaxDays {
		dayIdx = climateMaxDays - 1
	}

	hour := float64(lt.Hour()) + float64(lt.Minute())/60.0
	// 日周成分: 最低 ~02時 / 最高 ~14時 (peak when hour=14)。
	diurnal := math.Sin((hour - 8) / 24.0 * 2 * math.Pi)

	// 季節トレンド (5/1 を起点に線形上昇):
	//   気温 22.8℃ + 0.097℃/日 → 中旬で平年値(5月24.2/6月27.2)に一致。
	//   湿度 76% + 0.16%/日 → 中旬で平年値(5月78/6月83)に一致。
	meanTemp := 22.8 + 0.097*daysSinceStart
	meanHum := 76.0 + 0.16*daysSinceStart

	const tempAmp = 2.45 // 日較差の振幅(最高-最低 ≒ 4.9℃ = 平年の日較差)
	const humAmp = 10.0  // 湿度の日内変動(日中低・夜間高)

	temp = meanTemp + tempAmp*diurnal + m.dayWeatherTemp[dayIdx] + (m.rng.Float64()-0.5)*0.5
	hum = meanHum - humAmp*diurnal + m.dayWeatherHum[dayIdx] + (m.rng.Float64()-0.5)*2.0
	return temp, hum
}

// --- pgtype ヘルパ --------------------------------------------------------

// numeric2 は float を NUMERIC(5,2) 相当 (小数2桁) の pgtype.Numeric へ変換する。
func numeric2(f float64) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(math.Round(f * 100))),
		Exp:   -2,
		Valid: true,
	}
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
