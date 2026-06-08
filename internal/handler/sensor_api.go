package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
)

// SensorRepo は SensorAPI が必要とする最小の DB ポート (consumer interface)。
// repository.Querier / *repository.Queries はこれを満たす。最小メソッドに限定することで
// テスト時のモックを小さく保つ (DIP / アーキ決定の「consumer 最小 interface」)。
// 所有者認可に使う GetDevice は authz.DeviceGetter から取り込む。
type SensorRepo interface {
	authz.DeviceGetter // GetDevice(ctx, id) (repository.Device, error)
	CreateSensorReading(ctx context.Context, arg repository.CreateSensorReadingParams) (repository.SensorReading, error)
	UpdateDeviceLastCommunicated(ctx context.Context, id int64) error
}

// AlertEvaluator は SensorAPI が受信時のアラート判定を委譲する consumer interface。
// *service.AlertEvaluator がこれを満たす。SensorRepo にアラート用メソッドを足さず
// 別フィールドで受けることで、受信の関心事とアラート判定の関心事を分離する。
type AlertEvaluator interface {
	EvaluateAndNotify(ctx context.Context, reading *repository.SensorReading) ([]repository.AlertHistory, error)
}

// SensorAPI は ESP8266 等のデバイスから呼ばれる REST API ハンドラ。
// 認証は auth.DeviceAuth ミドルウェアで済んでいる前提。
type SensorAPI struct {
	Repo      SensorRepo
	Evaluator AlertEvaluator
}

// CreateSensorReadingRequest は POST /api/sensor-data のリクエストボディ。
//
// バリデーションルールは DB設計書.md のバリデーションルール定義
// (temperature: -40〜125, humidity: 0〜100) に準拠。
type CreateSensorReadingRequest struct {
	DeviceID    int64     `json:"device_id"    binding:"required,min=1"`
	Temperature float64   `json:"temperature"  binding:"gte=-40,lte=125"`
	Humidity    float64   `json:"humidity"     binding:"gte=0,lte=100"`
	RecordedAt  time.Time `json:"recorded_at"  binding:"required"`
}

type CreateSensorReadingResponse struct {
	ID          int64     `json:"id"`
	DeviceID    int64     `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	RecordedAt  time.Time `json:"recorded_at"`
	CreatedAt   time.Time `json:"created_at"`
	// AlertsFired は当該受信で発火・記録されたアラート履歴の件数。
	AlertsFired int `json:"alerts_fired"`
}

// Create はセンサーデータを保存する。
//
// 手順:
//  1. JSON Bind + バリデーション
//  2. device_id の存在確認 + 所有者確認
//  3. sensor_readings INSERT
//  4. devices.last_communicated_at 更新 (失敗しても続行)
//  5. アラート判定を同期実行 (失敗しても 201 を妨げない / ベストエフォート)
//
// HTTP ステータス:
//   - 201: 作成成功 (アラート判定の成否に関わらず)
//   - 400: JSON 形式エラー
//   - 401: 未認証 (多重防御。通常は DeviceAuth が先に 401 を返す)
//   - 403: 他ユーザーのデバイスに書き込もうとした
//   - 422: バリデーションエラー / 存在しない device_id
//   - 500: DB エラー (保存まで。アラート判定の失敗は 500 にしない)
func (h *SensorAPI) Create(c *gin.Context) {
	var req CreateSensorReadingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// JSON 形式が不正な場合は 400、それ以外 (バリデーション) は 422 を返す。
		var syntaxErr *json.SyntaxError
		var unmarshalErr *json.UnmarshalTypeError
		if errors.As(err, &syntaxErr) || errors.As(err, &unmarshalErr) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid JSON body: " + err.Error()})
			return
		}
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	userID := auth.UserID(c)

	// 所有者認可は authz に集約 (BOLA 防止)。sentinel error を HTTP ステータスへ写す。
	// 注: 「存在しない(422)」と「他ユーザー所有(403)」を区別する。device_id は連番のため
	// 認証済みクライアントには存在オラクルとなりうるが、書込は防げており IoT 用途では許容する。
	device, err := authz.RequireDeviceOwner(ctx, h.Repo, req.DeviceID, userID)
	if err != nil {
		switch {
		case errors.Is(err, authz.ErrUnauthenticated):
			c.JSON(http.StatusUnauthorized, gin.H{"message": "authentication required"})
		case errors.Is(err, sql.ErrNoRows):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "device not found or deleted"})
		case errors.Is(err, authz.ErrNotOwner):
			c.JSON(http.StatusForbidden, gin.H{"message": "device belongs to a different user"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"message": "device lookup failed"})
		}
		return
	}

	reading, err := h.Repo.CreateSensorReading(ctx, repository.CreateSensorReadingParams{
		DeviceID: req.DeviceID,
		// SQLite の REAL は float64 を丸めず保持するため、移行前の NUMERIC(5,2) 保存と
		// 等価にするよう INSERT 直前に小数第2位へ量子化する (R4.1)。
		Temperature: pgconv.Quantize2(req.Temperature),
		Humidity:    pgconv.Quantize2(req.Humidity),
		RecordedAt:  req.RecordedAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to save reading: " + err.Error()})
		return
	}

	_ = h.Repo.UpdateDeviceLastCommunicated(ctx, device.ID)

	// アラート判定を同期実行する (DB設計書 §アラート判定の実行タイミング)。
	// ベストエフォート: 判定・履歴作成が失敗しても受信成功 (201) を妨げない。
	// エラーは service 側でログ済みのため、ここでは件数のみ利用する。
	alerts, _ := h.Evaluator.EvaluateAndNotify(ctx, &reading)

	c.JSON(http.StatusCreated, CreateSensorReadingResponse{
		ID:          reading.ID,
		DeviceID:    reading.DeviceID,
		Temperature: reading.Temperature,
		Humidity:    reading.Humidity,
		RecordedAt:  reading.RecordedAt,
		CreatedAt:   reading.CreatedAt,
		AlertsFired: len(alerts),
	})
}
