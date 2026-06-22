package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildPayload(t *testing.T) {
	recordedAt := time.Date(2026, 6, 22, 15, 4, 5, 0, jst)

	got, err := buildPayload(1, 27.305, 62.149, recordedAt)
	if err != nil {
		t.Fatalf("buildPayload returned error: %v", err)
	}

	// サーバの CreateSensorReadingRequest と同じキー・型でデコードできること。
	var decoded struct {
		DeviceID    int64   `json:"device_id"`
		Temperature float64 `json:"temperature"`
		Humidity    float64 `json:"humidity"`
		RecordedAt  string  `json:"recorded_at"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("生成 JSON をデコードできない: %v (json=%s)", err, got)
	}

	if decoded.DeviceID != 1 {
		t.Errorf("device_id = %d, want 1", decoded.DeviceID)
	}
	// NUMERIC(5,2) 相当の小数2桁丸め。
	if decoded.Temperature != 27.31 {
		t.Errorf("temperature = %v, want 27.31 (小数2桁丸め)", decoded.Temperature)
	}
	if decoded.Humidity != 62.15 {
		t.Errorf("humidity = %v, want 62.15 (小数2桁丸め)", decoded.Humidity)
	}
	// RFC3339 + JST 固定オフセット。ファームの recorded_at と同形式。
	if want := "2026-06-22T15:04:05+09:00"; decoded.RecordedAt != want {
		t.Errorf("recorded_at = %q, want %q", decoded.RecordedAt, want)
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{27.305, 27.31},
		{27.304, 27.30},
		{0, 0},
		{-12.346, -12.35},
		{100, 100},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestClamp(t *testing.T) {
	cases := []struct {
		f, lo, hi, want float64
	}{
		{50, 0, 100, 50},   // 範囲内
		{-5, 0, 100, 0},    // 下限
		{130, -40, 125, 125}, // 上限
	}
	for _, c := range cases {
		if got := clamp(c.f, c.lo, c.hi); got != c.want {
			t.Errorf("clamp(%v,%v,%v) = %v, want %v", c.f, c.lo, c.hi, got, c.want)
		}
	}
}
