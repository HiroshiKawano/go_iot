// 統計分析ページ（長期トレンド・季節サマリ／GET /analysis/trend）の表示検証用に、
// 「長期トレンドテスト（沖縄・10年）」デバイスと約10年分の温湿度データを投入する開発ツール。
//
// 目的:
//
//	/analysis/trend は平年比（暦月平均・3年以上で表示）やトレンド検出力（span 3年・N_eff 10 以上）の
//	本格判定に「数年スパンの蓄積」を要する。実機・既存 seed は数日〜数週間しか無いため、那覇の
//	季節サイクル＋温暖化トレンド（温度のみ上昇・湿度はほぼ横ばい）を約10年分生成し、月次/年次
//	ロールアップ・平年比・Sen 線・CI 帯・日較差ΔT・判定バッジを実データで確認できるようにする。
//
// 重要な設計方針（既存データ保護・seed-testsensor と同方針）:
//   - 全テーブルを TRUNCATE する cmd/seed とは異なり、本ツールは一切 TRUNCATE しない。
//   - 専用デバイス（MAC: AA:BB:CC:DD:EE:04）を別途追加し、そのデバイスにのみ投入する。
//   - 再実行は冪等: 既にあればそのデバイスの計測のみ削除して作り直す（他デバイス非対象）。
//
// 使い方:
//
//	go run ./cmd/seed-trendsensor            # 既定（3時間間隔・10年分）
//	go run ./cmd/seed-trendsensor -years=5 -interval=6h
//
// 前提: make up + make migrate-up 済み。最低 1 名のユーザーが存在すること（所有者になる）。
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

// テストセンサーの識別情報（seed の :01/:02・seed-testsensor の :03 と衝突しない）。
const (
	trendSensorMAC      = "AA:BB:CC:DD:EE:04"
	trendSensorName     = "長期トレンドテスト（沖縄・10年）"
	trendSensorLocation = "沖縄県那覇市（長期トレンド表示テスト用）"
)

// jst は計測時刻の基準タイムゾーン。DB セッション TZ も Asia/Tokyo のため日境界が一致する。
var jst = time.FixedZone("JST", 9*60*60)

func main() {
	years := flag.Int("years", 10, "生成する蓄積年数（例: 10, 5）")
	interval := flag.Duration("interval", 3*time.Hour, "計測サンプルの間隔（例: 3h, 6h・日較差ΔTのため複数/日）")
	flag.Parse()

	if err := run(*years, *interval); err != nil {
		log.Fatalf("seed-trendsensor failed: %v", err)
	}
}

func run(years int, interval time.Duration) error {
	if years < 1 {
		return fmt.Errorf("years は 1 以上を指定してください: %d", years)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	pool, err := infradb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	userID, err := firstUserID(ctx, pool)
	if err != nil {
		return fmt.Errorf("ユーザー取得: %w（先に make seed 等でユーザーを作成してください）", err)
	}

	deviceID, reused, err := ensureTrendDevice(ctx, pool, userID)
	if err != nil {
		return fmt.Errorf("テストデバイス準備: %w", err)
	}
	if reused {
		log.Printf("  ✓ 既存テストデバイスを再利用: id=%d（既存の計測のみ削除して作り直し）", deviceID)
	} else {
		log.Printf("  ✓ テストデバイスを新規作成: id=%d name=%s mac=%s", deviceID, trendSensorName, trendSensorMAC)
	}

	// 生成範囲: (今年-years+1) 年の 1/1 00:00 JST 〜 現在。10年指定なら distinct 10 暦年で平年比も成立。
	now := time.Now()
	start := time.Date(now.In(jst).Year()-years+1, time.January, 1, 0, 0, 0, 0, jst)

	rows, lastAt := generateReadings(deviceID, start, now, interval)
	if err := copyReadings(ctx, pool, rows); err != nil {
		return fmt.Errorf("計測データ投入: %w", err)
	}
	if err := updateLastCommunicated(ctx, pool, deviceID, lastAt); err != nil {
		return fmt.Errorf("最終通信日時更新: %w", err)
	}

	log.Printf("  ✓ sensor_readings: %d 件（%s 〜 %s, 間隔 %s）",
		len(rows),
		start.Format("2006-01-02 15:04"),
		lastAt.In(jst).Format("2006-01-02 15:04"),
		interval)
	log.Printf("seed-trendsensor 完了 — 統計分析: /analysis/trend?device_id=%d", deviceID)
	return nil
}

// firstUserID はテストデバイスの所有者となる最小 id のユーザーを返す。
func firstUserID(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `SELECT id FROM users ORDER BY id LIMIT 1`).Scan(&id)
	return id, err
}

// ensureTrendDevice は専用テストデバイスを用意し device_id と「再利用か否か」を返す（冪等）。
func ensureTrendDevice(ctx context.Context, pool *pgxpool.Pool, userID int64) (deviceID int64, reused bool, err error) {
	err = pool.QueryRow(ctx,
		`SELECT id FROM devices WHERE mac_address = $1 AND deleted_at IS NULL`,
		trendSensorMAC,
	).Scan(&deviceID)

	switch {
	case err == nil:
		if _, derr := pool.Exec(ctx, `DELETE FROM sensor_readings WHERE device_id = $1`, deviceID); derr != nil {
			return 0, false, derr
		}
		return deviceID, true, nil
	case err == pgx.ErrNoRows:
		loc := trendSensorLocation
		ierr := pool.QueryRow(ctx, `
			INSERT INTO devices (user_id, name, mac_address, location, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id`,
			userID, trendSensorName, trendSensorMAC, loc,
		).Scan(&deviceID)
		if ierr != nil {
			return 0, false, ierr
		}
		return deviceID, false, nil
	default:
		return 0, false, err
	}
}

// generateReadings は start〜end を interval 刻みで走査し、那覇の多年気候モデルに基づく計測行を生成する。
func generateReadings(deviceID int64, start, end time.Time, interval time.Duration) (rows [][]any, lastAt time.Time) {
	model := newOkinawaLongTermClimate(start, end)

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

// copyReadings は COPY（pgx CopyFrom）でまとめて高速投入する。created_at/updated_at は DB デフォルト。
func copyReadings(ctx context.Context, pool *pgxpool.Pool, rows [][]any) error {
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"sensor_readings"},
		[]string{"device_id", "temperature", "humidity", "recorded_at"},
		pgx.CopyFromRows(rows),
	)
	return err
}

// updateLastCommunicated は「最終通信」を最新サンプル時刻に合わせる。
func updateLastCommunicated(ctx context.Context, pool *pgxpool.Pool, deviceID int64, at time.Time) error {
	_, err := pool.Exec(ctx,
		`UPDATE devices SET last_communicated_at = $1, updated_at = now() WHERE id = $2`,
		pgtype.Timestamptz{Time: at, Valid: true}, deviceID,
	)
	return err
}

// --- 沖縄(那覇)長期気候モデル ------------------------------------------------

// okinawaLongTermClimate は那覇の平年値に「年周（季節サイクル）＋温暖化トレンド＋日較差＋天候ゆらぎ」
// を重ねた多年生成器。温度は緩やかに上昇（Sen 線が右肩上がり）、湿度はほぼ横ばい（非有意の対比）。
type okinawaLongTermClimate struct {
	start time.Time
	// 日単位の天候オフセット（曇雨で気温↓・湿度↑）。インデックスは start からの経過日数。
	dayWeatherTemp []float64
	dayWeatherHum  []float64
	rng            *rand.Rand
}

// 那覇の年周パラメータ（気象庁平年値に整合する近似）。
const (
	annualMeanTemp = 23.3 // 年平均気温(℃)
	seasonAmpTemp  = 5.7  // 季節振幅(℃)（夏≈29 / 冬≈17.6）
	coldestDOY     = 20   // 最寒の年内日（≈1/20）
	warmRatePerYr  = 0.09 // 温暖化トレンド(℃/年)。10年で約+0.9℃ → 右肩上がりの Sen 線

	annualMeanHum = 74.0 // 年平均湿度(%)
	seasonAmpHum  = 6.0  // 季節振幅(%)（梅雨〜盛夏に高い）
	wetPeakDOY    = 191  // 多湿ピークの年内日（≈7月上旬）
	humRatePerYr  = -0.0 // 湿度トレンドは横ばい（温度との対比で非有意を示す）

	tempDiurnalAmp = 3.0 // 日較差の振幅（最高-最低 ≒ 6℃）
	humDiurnalAmp  = 9.0 // 湿度の日内変動（日中低・夜間高）
)

func newOkinawaLongTermClimate(start, end time.Time) *okinawaLongTermClimate {
	rng := rand.New(rand.NewPCG(20260629, 0x5EED7E))
	totalDays := int(end.Sub(start).Hours()/24.0) + 2
	if totalDays < 1 {
		totalDays = 1
	}
	dayT := make([]float64, totalDays)
	dayH := make([]float64, totalDays)
	for i := range dayT {
		wt := rng.NormFloat64() * 1.3 // 天候による気温の上下動(σ≈1.3℃)
		dayT[i] = wt
		dayH[i] = -2.0*wt + rng.NormFloat64()*3.0 // 寒い日=雨=多湿 + 独自ノイズ
	}
	return &okinawaLongTermClimate{start: start, dayWeatherTemp: dayT, dayWeatherHum: dayH, rng: rng}
}

// sample は時刻 t における (温度℃, 湿度%) を返す。
func (m *okinawaLongTermClimate) sample(t time.Time) (temp, hum float64) {
	lt := t.In(jst)
	dayIdx := int(lt.Sub(m.start).Hours() / 24.0)
	if dayIdx < 0 {
		dayIdx = 0
	}
	if dayIdx >= len(m.dayWeatherTemp) {
		dayIdx = len(m.dayWeatherTemp) - 1
	}
	yearsSince := lt.Sub(m.start).Hours() / 24.0 / 365.25
	doy := float64(lt.YearDay())
	hour := float64(lt.Hour()) + float64(lt.Minute())/60.0

	// 年周成分: 最寒 coldestDOY を底とする余弦（+半年で頂点）。
	seasonalT := -seasonAmpTemp * math.Cos(2*math.Pi*(doy-coldestDOY)/365.25)
	seasonalH := seasonAmpHum * math.Sin(2*math.Pi*(doy-wetPeakDOY+91.3125)/365.25)

	// 日周成分: 最低 ≈3時 / 最高 ≈15時。
	diurnal := math.Sin(2 * math.Pi * (hour - 9) / 24.0)

	meanTemp := annualMeanTemp + warmRatePerYr*yearsSince + seasonalT
	meanHum := annualMeanHum + humRatePerYr*yearsSince + seasonalH

	temp = meanTemp + tempDiurnalAmp*diurnal + m.dayWeatherTemp[dayIdx] + (m.rng.Float64()-0.5)*0.8
	hum = meanHum - humDiurnalAmp*diurnal + m.dayWeatherHum[dayIdx] + (m.rng.Float64()-0.5)*2.0
	return temp, hum
}

// --- pgtype ヘルパ ----------------------------------------------------------

// numeric2 は float を NUMERIC(5,2) 相当（小数2桁）の pgtype.Numeric へ変換する。
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
