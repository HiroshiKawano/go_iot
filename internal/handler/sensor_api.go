package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
)

// SensorAPI は ESP8266 等のデバイスから呼ばれる REST API ハンドラ。
// 認証は auth.DeviceAuth ミドルウェアで済んでいる前提。
type SensorAPI struct {
	Repo *repository.Queries
}

// CreateSensorReadingRequest は POST /api/sensor-data のリクエストボディ。
//
// バリデーションルールは DB設計書.md のバリデーションルール定義
// (temperature: -40〜125, humidity: 0〜100) に準拠。
type CreateSensorReadingRequest struct {
	DeviceID    int64     `json:"device_id"    validate:"required,min=1"`
	Temperature float64   `json:"temperature"  validate:"gte=-40,lte=125"`
	Humidity    float64   `json:"humidity"     validate:"gte=0,lte=100"`
	RecordedAt  time.Time `json:"recorded_at"  validate:"required"`
}

type CreateSensorReadingResponse struct {
	ID          int64     `json:"id"`
	DeviceID    int64     `json:"device_id"`
	Temperature float64   `json:"temperature"`
	Humidity    float64   `json:"humidity"`
	RecordedAt  time.Time `json:"recorded_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// Create はセンサーデータを保存する。
//
// 手順:
//  1. JSON Bind + バリデーション
//  2. device_id の存在確認 + 所有者確認
//  3. sensor_readings INSERT
//  4. devices.last_communicated_at 更新 (失敗しても続行)
//
// HTTP ステータス:
//   - 201: 作成成功
//   - 400: JSON 形式エラー
//   - 403: 他ユーザーのデバイスに書き込もうとした
//   - 422: バリデーションエラー / 存在しない device_id
//   - 500: DB エラー
//
// アラート判定は DB設計書.md の方針に従い Step 18 (フェーズ7) で同期追加予定。
func (h *SensorAPI) Create(c echo.Context) error {
	var req CreateSensorReadingRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid JSON body: "+err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}

	ctx := c.Request().Context()
	userID := auth.UserID(c)

	device, err := h.Repo.GetDevice(ctx, req.DeviceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "device not found or deleted")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "device lookup failed")
	}
	if device.UserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "device belongs to a different user")
	}

	reading, err := h.Repo.CreateSensorReading(ctx, repository.CreateSensorReadingParams{
		DeviceID:    req.DeviceID,
		Temperature: pgconv.Numeric2(req.Temperature),
		Humidity:    pgconv.Numeric2(req.Humidity),
		RecordedAt:  pgconv.Timestamptz(req.RecordedAt),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save reading: "+err.Error())
	}

	_ = h.Repo.UpdateDeviceLastCommunicated(ctx, device.ID)

	return c.JSON(http.StatusCreated, CreateSensorReadingResponse{
		ID:          reading.ID,
		DeviceID:    reading.DeviceID,
		Temperature: pgconv.NumericToFloat(reading.Temperature),
		Humidity:    pgconv.NumericToFloat(reading.Humidity),
		RecordedAt:  pgconv.TimestamptzToTime(reading.RecordedAt),
		CreatedAt:   pgconv.TimestamptzToTime(reading.CreatedAt),
	})
}
