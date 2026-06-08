package handler

import (
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/HiroshiKawano/go_iot/internal/view/component"
)

// TestComposeAlertMessage は未対応アラート1件の表示文言合成を検証する。
// 「{デバイス名}: {指標}が{閾値}{単位}{動詞}（{実測値}{単位}）」の書式で、
// 閾値は末尾ゼロ trim、実測値は小数2桁固定、演算子は超過/下回りの動詞へ写像する。
func TestComposeAlertMessage(t *testing.T) {
	tests := []struct {
		name string
		row  repository.ListUnnotifiedAlertHistoriesWithDeviceRow
		want string
	}{
		{
			name: "温度×超過(>)はモック例の文言と一致",
			row: repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
				DeviceName:  "ハウスA温湿度計",
				Metric:      "temperature",
				Operator:    ">",
				Threshold:   35.00,
				ActualValue: 38.50,
			},
			want: "ハウスA温湿度計: 温度が35℃を超えました（38.50℃）",
		},
		{
			name: "湿度×下回り(<)はモック例の文言と一致",
			row: repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
				DeviceName:  "ハウスB温湿度計",
				Metric:      "humidity",
				Operator:    "<",
				Threshold:   30.00,
				ActualValue: 25.00,
			},
			want: "ハウスB温湿度計: 湿度が30%を下回りました（25.00%）",
		},
		{
			name: ">= は「を超えました」",
			row: repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
				DeviceName:  "D",
				Metric:      "temperature",
				Operator:    ">=",
				Threshold:   40.00,
				ActualValue: 40.00,
			},
			want: "D: 温度が40℃を超えました（40.00℃）",
		},
		{
			name: "<= は「を下回りました」",
			row: repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
				DeviceName:  "D",
				Metric:      "humidity",
				Operator:    "<=",
				Threshold:   20.00,
				ActualValue: 20.00,
			},
			want: "D: 湿度が20%を下回りました（20.00%）",
		},
		{
			name: "閾値の末尾ゼロを trim (35.50→35.5)",
			row: repository.ListUnnotifiedAlertHistoriesWithDeviceRow{
				DeviceName:  "D",
				Metric:      "temperature",
				Operator:    ">",
				Threshold:   35.50,
				ActualValue: 36.00,
			},
			want: "D: 温度が35.5℃を超えました（36.00℃）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := composeAlertMessage(tt.row); got != tt.want {
				t.Errorf("composeAlertMessage() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// timePtr は NULL 許容 datetime 列 (*time.Time) のフィクスチャ生成ヘルパ。
func timePtr(t time.Time) *time.Time { return &t }

// TestBuildDashboardDevice はデバイス行＋最新計測(無い場合あり)＋基準時刻から
// 表示用デバイスデータへの写像を検証する。温湿度は小数2桁＋単位、未受信は「ー」、
// 通信実績なしは「通信実績なし」、ありは相対時間。決定的テストのため now を固定注入する。
func TestBuildDashboardDevice(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	reading := &repository.SensorReading{
		Temperature: 28.50,
		Humidity:    65.30,
	}

	tests := []struct {
		name    string
		device  repository.Device
		reading *repository.SensorReading
		want    component.DashboardDevice
	}{
		{
			name: "計測あり・通信実績あり・稼働中",
			device: repository.Device{
				ID:                 1,
				Name:               "ハウスA温湿度計",
				Location:           strPtr("ビニールハウスA"),
				IsActive:           true,
				LastCommunicatedAt: timePtr(now.Add(-2 * time.Minute)),
			},
			reading: reading,
			want: component.DashboardDevice{
				ID:           1,
				Name:         "ハウスA温湿度計",
				Location:     "ビニールハウスA",
				IsActive:     true,
				TempText:     "28.50℃",
				HumidityText: "65.30%",
				LastCommText: "2分前",
			},
		},
		{
			name: "計測未受信は温湿度とも「ー」",
			device: repository.Device{
				ID:                 2,
				Name:               "計測待ちデバイス",
				Location:           strPtr("ハウスC"),
				IsActive:           true,
				LastCommunicatedAt: timePtr(now.Add(-3 * time.Hour)),
			},
			reading: nil,
			want: component.DashboardDevice{
				ID:           2,
				Name:         "計測待ちデバイス",
				Location:     "ハウスC",
				IsActive:     true,
				TempText:     "ー",
				HumidityText: "ー",
				LastCommText: "3時間前",
			},
		},
		{
			name: "通信実績なし(Valid=false)は「通信実績なし」",
			device: repository.Device{
				ID:                 3,
				Name:               "未通信デバイス",
				Location:           strPtr("ハウスD"),
				IsActive:           false,
				LastCommunicatedAt: nil, // 未通信(NULL)
			},
			reading: reading,
			want: component.DashboardDevice{
				ID:           3,
				Name:         "未通信デバイス",
				Location:     "ハウスD",
				IsActive:     false, // 停止中
				TempText:     "28.50℃",
				HumidityText: "65.30%",
				LastCommText: "通信実績なし",
			},
		},
		{
			name: "設置場所未設定(nil)は空文字",
			device: repository.Device{
				ID:                 4,
				Name:               "場所なしデバイス",
				Location:           nil,
				IsActive:           true,
				LastCommunicatedAt: nil,
			},
			reading: nil,
			want: component.DashboardDevice{
				ID:           4,
				Name:         "場所なしデバイス",
				Location:     "",
				IsActive:     true,
				TempText:     "ー",
				HumidityText: "ー",
				LastCommText: "通信実績なし",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDashboardDevice(tt.device, tt.reading, now); got != tt.want {
				t.Errorf("buildDashboardDevice() =\n  %+v\nwant\n  %+v", got, tt.want)
			}
		})
	}
}
