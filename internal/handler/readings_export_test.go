package handler

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// readings_export_test.go は CSV 整形ヘルパ (純度高・DB/HTTP 非依存) を検証する
// (テストガイダンス集 §33・§33.5)。BOM・日本語ヘッダ・メタ列反復・項目フィルタ・
// エスケープ (csv パーサ往復)・空行・ファイル名エンコードを単体で確認する。

var utf8BOMBytes = []byte{0xEF, 0xBB, 0xBF}

// stripBOM は先頭の UTF-8 BOM 3 バイトを落とす (csv パーサへ渡す前処理)。
func stripBOM(b []byte) []byte { return bytes.TrimPrefix(b, utf8BOMBytes) }

// exportRow は CSV 出力検証用の計測行を作る (固定時刻・値)。
func exportRow(rec time.Time, temp, hum float64) repository.SensorReading {
	return sensorRow(1, rec, temp, hum)
}

// --- 3.1 BOM・日本語ヘッダ・メタ列反復 (R2.1/R3.1) ---

func TestWriteReadingsCSV_BOMと日本語ヘッダとメタ列反復(t *testing.T) {
	meta := csvMeta{DeviceID: 1, Name: "ハウスA温湿度計", Locality: "佐敷（南城市）", Crop: "ゴーヤ"}
	rows := []repository.SensorReading{
		exportRow(time.Date(2026, 4, 20, 5, 30, 0, 0, time.UTC), 28.50, 65.30),
		exportRow(time.Date(2026, 4, 20, 5, 35, 0, 0, time.UTC), 28.60, 65.10),
	}

	var buf bytes.Buffer
	if err := writeReadingsCSV(&buf, meta, []metricCol{tempCol, humCol}, rows); err != nil {
		t.Fatalf("writeReadingsCSV() error: %v", err)
	}
	body := buf.Bytes()

	// 先頭 UTF-8 BOM (Excel 文字化け回避)。
	if !bytes.HasPrefix(body, utf8BOMBytes) {
		t.Fatal("先頭に UTF-8 BOM (0xEF 0xBB 0xBF) が無い")
	}

	recs, err := csv.NewReader(bytes.NewReader(stripBOM(body))).ReadAll()
	if err != nil {
		t.Fatalf("CSV パース失敗: %v", err)
	}
	// ヘッダ1行 + データ2行。
	if len(recs) != 3 {
		t.Fatalf("行数 = %d, want 3 (header+2): %v", len(recs), recs)
	}
	wantHeader := []string{"デバイスID", "デバイス名", "地点", "作物", "計測日時", "温度(℃)", "湿度(%)"}
	if strings.Join(recs[0], ",") != strings.Join(wantHeader, ",") {
		t.Errorf("ヘッダ = %v, want %v", recs[0], wantHeader)
	}
	// メタ列 (ID/名称/地点/作物) は各データ行に反復付与される (外部 pivot 用)。
	for i, r := range recs[1:] {
		if r[0] != "1" || r[1] != "ハウスA温湿度計" || r[2] != "佐敷（南城市）" || r[3] != "ゴーヤ" {
			t.Errorf("データ行%d のメタ列 = %v, want [1 ハウスA温湿度計 佐敷（南城市） ゴーヤ ...]", i, r[:4])
		}
	}
	// 計測日時は JST RFC3339、温湿度は小数2桁。
	if recs[1][4] != "2026-04-20T14:30:00+09:00" {
		t.Errorf("計測日時 = %q, want 2026-04-20T14:30:00+09:00", recs[1][4])
	}
	if recs[1][5] != "28.50" || recs[1][6] != "65.30" {
		t.Errorf("温湿度 = %q/%q, want 28.50/65.30", recs[1][5], recs[1][6])
	}
}

// --- 3.1 項目フィルタで列増減 (R1.4) ---

func TestWriteReadingsCSV_温度のみは湿度列を出さない(t *testing.T) {
	meta := csvMeta{DeviceID: 1, Name: "デバイス", Locality: "未設定", Crop: "未設定"}
	rows := []repository.SensorReading{exportRow(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), 28.50, 65.30)}

	var buf bytes.Buffer
	if err := writeReadingsCSV(&buf, meta, []metricCol{tempCol}, rows); err != nil {
		t.Fatalf("writeReadingsCSV() error: %v", err)
	}
	recs, _ := csv.NewReader(bytes.NewReader(stripBOM(buf.Bytes()))).ReadAll()

	header := strings.Join(recs[0], ",")
	if !strings.Contains(header, "温度(℃)") {
		t.Errorf("温度列が無い: %v", recs[0])
	}
	if strings.Contains(header, "湿度(%)") {
		t.Errorf("温度のみ指定なのに湿度列がある: %v", recs[0])
	}
	// データ行も湿度値を持たない (メタ5列+温度1列=6列)。
	if len(recs[1]) != 6 {
		t.Errorf("データ列数 = %d, want 6 (メタ5+温度1)", len(recs[1]))
	}
}

// --- 3.1 エスケープ (カンマ/改行/引用符を含むデバイス名・csv パーサ往復) (R3.2) ---

func TestWriteReadingsCSV_特殊文字を含むデバイス名をエスケープする(t *testing.T) {
	tricky := `ハウス"A",北 改行→
棟`
	meta := csvMeta{DeviceID: 1, Name: tricky, Locality: "未設定", Crop: "未設定"}
	rows := []repository.SensorReading{exportRow(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), 1.00, 2.00)}

	var buf bytes.Buffer
	if err := writeReadingsCSV(&buf, meta, []metricCol{tempCol, humCol}, rows); err != nil {
		t.Fatalf("writeReadingsCSV() error: %v", err)
	}
	// 標準 csv パーサで往復させ、列・行構造が崩れず名称が完全復元されることを確認する。
	recs, err := csv.NewReader(bytes.NewReader(stripBOM(buf.Bytes()))).ReadAll()
	if err != nil {
		t.Fatalf("CSV パース失敗 (構造破損): %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("行数 = %d, want 2 (改行を含んでも1データ行): %v", len(recs), recs)
	}
	if recs[1][1] != tricky {
		t.Errorf("デバイス名 = %q, want %q (往復で完全復元)", recs[1][1], tricky)
	}
}

// --- 3.1 空行でヘッダのみ (R1.6) ---

func TestWriteReadingsCSV_空行でヘッダのみ(t *testing.T) {
	meta := csvMeta{DeviceID: 1, Name: "デバイス", Locality: "未設定", Crop: "未設定"}

	var buf bytes.Buffer
	if err := writeReadingsCSV(&buf, meta, []metricCol{tempCol, humCol}, nil); err != nil {
		t.Fatalf("writeReadingsCSV() error: %v", err)
	}
	body := buf.Bytes()
	if !bytes.HasPrefix(body, utf8BOMBytes) {
		t.Error("空でも BOM は付与する")
	}
	recs, _ := csv.NewReader(bytes.NewReader(stripBOM(body))).ReadAll()
	if len(recs) != 1 {
		t.Errorf("行数 = %d, want 1 (ヘッダのみ)", len(recs))
	}
}

// --- 3.1 項目フィルタの選択ロジック (R1.4/R1.5) ---

func TestSelectMetricCols_未選択は温湿度両方を既定とする(t *testing.T) {
	cases := []struct {
		name  string
		items []string
		want  []string // 期待ヘッダ列
	}{
		{"未指定は両方", nil, []string{"温度(℃)", "湿度(%)"}},
		{"空も両方", []string{}, []string{"温度(℃)", "湿度(%)"}},
		{"温度のみ", []string{"temperature"}, []string{"温度(℃)"}},
		{"湿度のみ", []string{"humidity"}, []string{"湿度(%)"}},
		{"両方指定", []string{"temperature", "humidity"}, []string{"温度(℃)", "湿度(%)"}},
		{"不正値は無視し既定", []string{"vpd"}, []string{"温度(℃)", "湿度(%)"}},
		{"温度+不正は温度のみ", []string{"temperature", "xxx"}, []string{"温度(℃)"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cols := selectMetricCols(tc.items)
			var got []string
			for _, c := range cols {
				got = append(got, c.Header)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("selectMetricCols(%v) = %v, want %v", tc.items, got, tc.want)
			}
		})
	}
}

// --- 3.1 ファイル名 (ASCII フォールバック + RFC5987) (R3.4) ---

func TestCSVFilename_ASCIIフォールバックとRFC5987エンコード(t *testing.T) {
	ascii, rfc5987 := csvFilename("ハウスA温湿度計", "2026-04-13", "2026-04-20")

	// ASCII フォールバックは非 ASCII を含まず期間と .csv を判別できる。
	for _, r := range ascii {
		if r > 127 {
			t.Fatalf("ASCII フォールバックに非 ASCII 文字: %q", ascii)
		}
	}
	if !strings.Contains(ascii, "2026-04-13") || !strings.Contains(ascii, "2026-04-20") || !strings.HasSuffix(ascii, ".csv") {
		t.Errorf("ascii = %q, want 期間 + .csv を含む", ascii)
	}
	// RFC5987 は UTF-8'' 始まりで日本語をパーセントエンコードし、ASCII の期間部は素のまま残る。
	if !strings.HasPrefix(rfc5987, "UTF-8''") {
		t.Errorf("rfc5987 = %q, want UTF-8'' 始まり", rfc5987)
	}
	if !strings.Contains(rfc5987, "%E3%83%8F") { // 「ハ」= U+30CF = E3 83 8F
		t.Errorf("rfc5987 = %q, want 日本語のパーセントエンコードを含む", rfc5987)
	}
	if !strings.Contains(rfc5987, "2026-04-13") {
		t.Errorf("rfc5987 = %q, want ASCII 期間部はそのまま", rfc5987)
	}
}

func TestCSVPeriodLabel_期間の有無で判別片を切り替える(t *testing.T) {
	cases := []struct {
		name, from, to, want string
	}{
		{"全期間", "", "", "all"},
		{"終了日のみ", "", "2026-04-20", "until_2026-04-20"},
		{"開始日のみ", "2026-04-13", "", "since_2026-04-13"},
		{"両端", "2026-04-13", "2026-04-20", "2026-04-13_2026-04-20"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := csvPeriodLabel(tc.from, tc.to); got != tc.want {
				t.Errorf("csvPeriodLabel(%q,%q) = %q, want %q", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestCSVFilename_期間未指定でも生成できる(t *testing.T) {
	ascii, rfc5987 := csvFilename("デバイス", "", "")
	if !strings.HasSuffix(ascii, ".csv") {
		t.Errorf("ascii = %q, want .csv 終端", ascii)
	}
	if !strings.HasPrefix(rfc5987, "UTF-8''") {
		t.Errorf("rfc5987 = %q, want UTF-8'' 始まり", rfc5987)
	}
}

// --- 3.1 メタ生成 (地点/作物の未設定フォールバック) (R2.2/R2.3) ---

func TestNewCSVMeta_未設定は未設定ラベルで安全に出力する(t *testing.T) {
	t.Run("未設定 (NULL) は未設定", func(t *testing.T) {
		d := repository.Device{ID: 9, Name: "名無し", Locality: nil, Crop: nil}
		m := newCSVMeta(d)
		if m.DeviceID != 9 || m.Name != "名無し" {
			t.Errorf("ID/Name = %d/%q", m.DeviceID, m.Name)
		}
		if m.Locality != "未設定" || m.Crop != "未設定" {
			t.Errorf("未設定フォールバック = 地点%q/作物%q, want 未設定/未設定", m.Locality, m.Crop)
		}
	})
	t.Run("設定済みは表示名", func(t *testing.T) {
		loc, crop := "那覇市", "goya"
		d := repository.Device{ID: 1, Name: "ハウスA", Locality: &loc, Crop: &crop}
		m := newCSVMeta(d)
		if m.Locality == "未設定" || m.Crop != "ゴーヤ" {
			t.Errorf("設定済み = 地点%q/作物%q, want 認識名/ゴーヤ", m.Locality, m.Crop)
		}
	})
}
