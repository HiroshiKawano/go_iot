// sensor-sim は ESP8266 ファーム (firmware/esp8266_sht31) と同一の JSON を
// 自前 Go API (POST /api/sensor-data) へ送る、ハードウェア不要のテスト送信ツール。
//
// 実機を用意する前に「サーバ到達性・device_id・Bearer トークン・受信〜画面反映〜
// アラート判定」までを検証する用途。引継ぎメモの優先2 / 実装計画 §8-2 優先2 に相当。
//
// 例:
//
//	# 1 回だけ送信 (固定値)
//	go run ./cmd/sensor-sim -url http://localhost:8080/api/sensor-data \
//	    -token "$(make gen-token user=1 name=sim)" -device 1
//
//	# 5 分間隔で連続送信 (ランダム変動。Ctrl-C で停止)
//	go run ./cmd/sensor-sim -token <平文> -device 1 -count 0 -interval 5m -random
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// JST 固定オフセット。recorded_at をファームと同じ +09:00 で送る。
var jst = time.FixedZone("JST", 9*3600)

// reading は POST /api/sensor-data のリクエストボディ。
// サーバの handler.CreateSensorReadingRequest と同一フィールド。
type reading struct {
	DeviceID    int64   `json:"device_id"`
	Temperature float64 `json:"temperature"`
	Humidity    float64 `json:"humidity"`
	RecordedAt  string  `json:"recorded_at"`
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("sensor-sim: %v", err)
	}
}

func run() error {
	url := flag.String("url", "http://localhost:8080/api/sensor-data", "送信先 API エンドポイント")
	token := flag.String("token", "", "Bearer トークン (平文。make gen-token で発行)")
	device := flag.Int64("device", 0, "device_id (Web UI で登録した ID)")
	temp := flag.Float64("temp", 27.3, "温度 (-random 指定時は基準値)")
	hum := flag.Float64("hum", 62.1, "湿度 (-random 指定時は基準値)")
	random := flag.Bool("random", false, "基準値の周りでランダムに変動させる")
	count := flag.Int("count", 1, "送信回数 (0 = 無限。Ctrl-C で停止)")
	interval := flag.Duration("interval", 0, "送信間隔 (例: 5m, 30s)")
	flag.Parse()

	if *token == "" || *device <= 0 {
		flag.Usage()
		return fmt.Errorf("-token と -device は必須です")
	}

	// Ctrl-C / SIGTERM で安全に停止する。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := &http.Client{Timeout: 10 * time.Second}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	sent, ok := 0, 0
	for i := 0; *count == 0 || i < *count; i++ {
		if i > 0 && *interval > 0 {
			select {
			case <-ctx.Done():
				log.Printf("停止しました (送信 %d 件 / 成功 %d 件)", sent, ok)
				return nil
			case <-time.After(*interval):
			}
		}

		t, h := *temp, *hum
		if *random {
			t = clamp(*temp+rng.NormFloat64()*1.5, -40, 125)
			h = clamp(*hum+rng.NormFloat64()*5, 0, 100)
		}

		status, body, err := send(ctx, client, *url, *token, *device, t, h, time.Now().In(jst))
		sent++
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("停止しました (送信 %d 件 / 成功 %d 件)", sent-1, ok)
				return nil
			}
			log.Printf("[%d] 送信エラー: %v", i+1, err)
			continue
		}
		if status == http.StatusCreated {
			ok++
			log.Printf("[%d] 201 Created OK  t=%.2f h=%.2f  resp=%s", i+1, t, h, body)
		} else {
			// 400=JSON不正 / 401=トークン / 403=他ユーザー / 422=バリデーション・存在しないdevice_id / 500=DB
			log.Printf("[%d] %d (失敗)  resp=%s", i+1, status, body)
		}
	}

	log.Printf("完了: 送信 %d 件 / 成功 %d 件", sent, ok)
	if ok == 0 {
		return fmt.Errorf("成功した送信がありません")
	}
	return nil
}

// send は 1 件の計測値を POST する。戻り値は HTTP ステータスとレスポンス本文。
func send(ctx context.Context, client *http.Client, url, token string, deviceID int64, temp, hum float64, recordedAt time.Time) (int, string, error) {
	payload, err := buildPayload(deviceID, temp, hum, recordedAt)
	if err != nil {
		return 0, "", fmt.Errorf("build payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(bytes.TrimSpace(body)), nil
}

// buildPayload はファームと同形の JSON を生成する。
// 温湿度は小数2桁に丸める (サーバ側 NUMERIC(5,2) 相当)、時刻は RFC3339。
func buildPayload(deviceID int64, temp, hum float64, recordedAt time.Time) ([]byte, error) {
	return json.Marshal(reading{
		DeviceID:    deviceID,
		Temperature: round2(temp),
		Humidity:    round2(hum),
		RecordedAt:  recordedAt.Format(time.RFC3339),
	})
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func clamp(f, lo, hi float64) float64 {
	if f < lo {
		return lo
	}
	if f > hi {
		return hi
	}
	return f
}
