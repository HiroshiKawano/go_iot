package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// device_show_gdd_test.go は GDD パネル組立 (buildGDDPanel・period 非連動) を Querier 手書きモックで
// DB 非依存に検証する (タスク 6.1/6.2)。正常系・前提欠落・未来日・データ未到着・予測不能を網羅する。

// dateOnlyUTC はカレンダー日付 (y-m-d) を UTC 0:00 の time.Time にする (テスト用)。
func dateOnlyUTC(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// dailyAggRow は ListDailySensorAggregates の1行 (reading_date + 日次 max/min 気温) を作る。
// max/min は本番 SQL で明示キャストが無く interface{} ゆえ float64 を渡す (aggregateToFloat が処理)。
func dailyAggRow(date time.Time, tMax, tMin float64) repository.ListDailySensorAggregatesRow {
	return repository.ListDailySensorAggregatesRow{
		ReadingDate:    pgtype.Date{Time: date, Valid: true},
		MaxTemperature: tMax,
		MinTemperature: tMin,
	}
}

// gddRepo は所有者(7)・デバイス1(crop=rice・定植日2026-04-01)・日次気温3日分を備えた fake を返す。
// 各日 tMax=35/tMin=25 → 日次GDD=(35+25)/2−10(Tbase)=20、cum=[20,40,60]、elapsed=[0,1,2]。
func gddRepo() *fakeDeviceRepo {
	repo := showDeviceRepo()
	cropStr := "rice"
	d := repo.devices[1]
	d.Crop = &cropStr
	d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 4, 1), Valid: true}
	repo.devices[1] = d
	repo.dailyAggs = []repository.ListDailySensorAggregatesRow{
		dailyAggRow(dateOnlyUTC(2026, 4, 1), 35, 25),
		dailyAggRow(dateOnlyUTC(2026, 4, 2), 35, 25),
		dailyAggRow(dateOnlyUTC(2026, 4, 3), 35, 25),
	}
	return repo
}

func TestBuildGDDPanel_正常系は具体値で埋まる(t *testing.T) {
	h := &DeviceHandler{Repo: gddRepo()}
	device := gddRepo().devices[1]
	now := dateOnlyUTC(2026, 4, 4) // データ日以降・定植日は未来でない

	v, err := h.buildGDDPanel(context.Background(), device, now)
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}

	// 前提充足ゆえ Guidance は空・累積曲線 OptionJSON は非空。
	if v.Guidance != "" {
		t.Errorf("正常系で Guidance が非空: %q", v.Guidance)
	}
	if v.OptionJSON == "" {
		t.Error("正常系で OptionJSON が空（累積曲線が描かれていない）")
	}
	if v.CropLabel != "米" {
		t.Errorf("CropLabel = %q, want 米", v.CropLabel)
	}
	// 数値カード: 累積60・残り 1400−60=1340・経過日数2日・現在ステージ発芽(cum60<300)。
	if !strings.Contains(v.Card.Cumulative, "60") {
		t.Errorf("Cumulative = %q, want 60 を含む", v.Card.Cumulative)
	}
	if !strings.Contains(v.Card.Remaining, "1340") {
		t.Errorf("Remaining = %q, want 1340 を含む", v.Card.Remaining)
	}
	if !strings.Contains(v.Card.ElapsedDays, "2") {
		t.Errorf("ElapsedDays = %q, want 2 を含む", v.Card.ElapsedDays)
	}
	if v.Card.Stage != "発芽" {
		t.Errorf("Stage = %q, want 発芽", v.Card.Stage)
	}
	// 予測収穫日: 回帰直線 cum=20x+20（切片20）, target=1400 → (1400−20)/20=69日後 = 2026-06-09。
	if v.Card.ForecastDate != "2026-06-09" {
		t.Errorf("ForecastDate = %q, want 2026-06-09", v.Card.ForecastDate)
	}
	// 生育ステージ表は5段で、現在段(発芽)に Current マークが付く。
	if len(v.Stages) != 5 {
		t.Fatalf("Stages 数 = %d, want 5", len(v.Stages))
	}
	if !v.Stages[0].Current {
		t.Errorf("発芽段が Current でない: %+v", v.Stages[0])
	}
	for i := 1; i < len(v.Stages); i++ {
		if v.Stages[i].Current {
			t.Errorf("非現在段 %s に Current が付いている", v.Stages[i].Name)
		}
	}
}

// 6.2 縮退系: 前提未設定（作物未設定／定植日 NULL）→ 汎用の設定導線注記・OptionJSON 空。
// 作物・定植日のどちらかが欠けている状態ゆえ「設定してください」の汎用文言（gddGuidanceNote）を出す。
func TestBuildGDDPanel_前提未設定は設定導線へ縮退(t *testing.T) {
	now := dateOnlyUTC(2026, 4, 4)
	tests := []struct {
		name   string
		mutate func(d *repository.Device)
	}{
		{
			// 定植日はあるが作物が未設定（NULL）。
			name: "作物未設定（定植日あり）→設定導線",
			mutate: func(d *repository.Device) {
				d.Crop = nil
				d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 4, 1), Valid: true}
			},
		},
		{
			// 作物は米だが定植日 NULL。
			name: "定植日NULL→設定導線",
			mutate: func(d *repository.Device) {
				cropStr := "rice"
				d.Crop = &cropStr
				d.PlantingDate = pgtype.Date{Valid: false}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := gddRepo()
			d := repo.devices[1]
			tt.mutate(&d)
			repo.devices[1] = d
			h := &DeviceHandler{Repo: repo}

			v, err := h.buildGDDPanel(context.Background(), d, now)
			if err != nil {
				t.Fatalf("buildGDDPanel() error: %v", err)
			}
			// 汎用の設定導線注記であること（未対応作物の専用注記と区別する）。
			if v.Guidance != gddGuidanceNote {
				t.Errorf("Guidance = %q, want 汎用の設定導線注記 %q", v.Guidance, gddGuidanceNote)
			}
			if v.OptionJSON != "" {
				t.Errorf("前提未設定で OptionJSON が非空（チャートを描いてしまっている）")
			}
		})
	}
}

// 6.2 縮退系: 作物・定植日は設定済みだが、その作物の GDD 具体モデルが未対応（サトウキビ等）→
// 「設定してください」ではなく「この作物は未対応・対応作物は〇〇」を明示する専用注記へ縮退する。
// これが設定済みユーザーの誤解（『設定したのに出ない』）を防ぐ本修正の核心。
func TestBuildGDDPanel_未対応作物は専用注記へ縮退(t *testing.T) {
	repo := gddRepo()
	d := repo.devices[1]
	cropStr := "sugarcane" // 有効な作物だが GDD 具体モデルなし（既定フォールバック）
	d.Crop = &cropStr
	d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 4, 1), Valid: true}
	repo.devices[1] = d
	h := &DeviceHandler{Repo: repo}

	v, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4))
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}
	if v.OptionJSON != "" {
		t.Errorf("未対応作物で OptionJSON が非空（チャートを描いてしまっている）")
	}
	// 汎用の設定導線注記ではない（誤解を招く文言を出さない）。
	if v.Guidance == gddGuidanceNote {
		t.Errorf("未対応作物に汎用の設定導線注記が出ている（設定済みユーザーの誤解を招く）: %q", v.Guidance)
	}
	// 当該作物名（サトウキビ）と、対応作物の一例（米・ゴーヤ）を含む案内であること。
	for _, want := range []string{"サトウキビ", "米", "ゴーヤ"} {
		if !strings.Contains(v.Guidance, want) {
			t.Errorf("Guidance = %q, want %q を含む", v.Guidance, want)
		}
	}
}

// 6.2 正常系（段階拡張）: ゴーヤ（GDD 具体モデルあり）は前提充足で累積曲線を描く。
// 米以外の年1作物にも GDD が拡張されたことを固定する（本修正のもう一つの核心）。
func TestBuildGDDPanel_ゴーヤは具体GDDで描画(t *testing.T) {
	repo := gddRepo()
	d := repo.devices[1]
	cropStr := "goya"
	d.Crop = &cropStr
	d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 4, 1), Valid: true}
	repo.devices[1] = d
	h := &DeviceHandler{Repo: repo}

	v, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4))
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}
	if v.Guidance != "" {
		t.Errorf("ゴーヤ（対応作物）で Guidance が非空: %q", v.Guidance)
	}
	if v.OptionJSON == "" {
		t.Error("ゴーヤで OptionJSON が空（累積曲線が描かれていない）")
	}
	if v.CropLabel != "ゴーヤ" {
		t.Errorf("CropLabel = %q, want ゴーヤ", v.CropLabel)
	}
	// ステージ表が具体モデル（複数段）で描かれる。
	if len(v.Stages) < 2 {
		t.Errorf("Stages 数 = %d, want ≥2（ゴーヤの生育ステージ）", len(v.Stages))
	}
}

// 6.2 縮退系: 未来定植日 → 未開始注記（経過日数を負にしない・要件 2.6）。
func TestBuildGDDPanel_未来定植日は未開始注記(t *testing.T) {
	repo := gddRepo()
	d := repo.devices[1]
	d.PlantingDate = pgtype.Date{Time: dateOnlyUTC(2026, 5, 1), Valid: true} // now(4/4) より未来
	repo.devices[1] = d
	h := &DeviceHandler{Repo: repo}

	v, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4))
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}
	if v.Guidance == "" {
		t.Error("未来定植日で Guidance（未開始注記）が空")
	}
	if !strings.Contains(v.Guidance, "未来") {
		t.Errorf("Guidance = %q, want 未来日の旨を含む", v.Guidance)
	}
	if v.OptionJSON != "" {
		t.Error("未来定植日で OptionJSON が非空")
	}
}

// 6.2 縮退系: 定植日以降データ0件 → データ未到着注記。
func TestBuildGDDPanel_データ未到着は注記へ縮退(t *testing.T) {
	repo := gddRepo()
	repo.dailyAggs = nil // 定植日以降の日次集計が空
	d := repo.devices[1]
	h := &DeviceHandler{Repo: repo}

	v, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4))
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}
	if v.Guidance == "" {
		t.Error("データ0件で Guidance（データ未到着）が空")
	}
	if v.OptionJSON != "" {
		t.Error("データ0件で OptionJSON が非空")
	}
}

// 6.2 縮退系: 予測不能（生育せず=傾き0）→ 予測カード "—"＋理由注記。累積曲線・目標線は描く。
func TestBuildGDDPanel_予測不能は予測カードダッシュと理由注記(t *testing.T) {
	repo := gddRepo()
	// 全日 Tbase 未満（日平均 < 10）→ 日次 GDD=0・累積=0・傾き0 で予測不能。
	repo.dailyAggs = []repository.ListDailySensorAggregatesRow{
		dailyAggRow(dateOnlyUTC(2026, 4, 1), 8, 4),
		dailyAggRow(dateOnlyUTC(2026, 4, 2), 9, 5),
		dailyAggRow(dateOnlyUTC(2026, 4, 3), 7, 3),
	}
	d := repo.devices[1]
	h := &DeviceHandler{Repo: repo}

	v, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4))
	if err != nil {
		t.Fatalf("buildGDDPanel() error: %v", err)
	}
	// 累積曲線・目標線は描く（縮退でなく予測のみ不能）。
	if v.OptionJSON == "" {
		t.Error("予測不能でも累積曲線 OptionJSON は描くべき（空になっている）")
	}
	// 予測カードは "—"。
	if v.Card.ForecastDate != "—" {
		t.Errorf("ForecastDate = %q, want —（予測不能）", v.Card.ForecastDate)
	}
	// 理由注記（予測不能の説明）が Note に出る。
	if !strings.Contains(v.Note, "算出できません") {
		t.Errorf("Note = %q, want 予測不能の理由を含む", v.Note)
	}
}

// 6.2 DB 想定外: 日次集計取得失敗は error を返す（呼出側 Show が 500 化・縮退でなく error）。
func TestBuildGDDPanel_日次集計エラーはerror(t *testing.T) {
	repo := gddRepo()
	repo.dailyErr = errors.New("db down") // ListDailySensorAggregates が失敗
	d := repo.devices[1]
	h := &DeviceHandler{Repo: repo}

	if _, err := h.buildGDDPanel(context.Background(), d, dateOnlyUTC(2026, 4, 4)); err == nil {
		t.Error("日次集計取得失敗で error が返らない（500 化されない）")
	}
}

// gddSectionOf は GDD パネル部分（"GDD（積算温度" から "最新計測データ" の手前まで）を抽出する。
// period 非連動の検証で、期間が変わっても GDD 部分の HTML が不変であることを比較するのに使う。
func gddSectionOf(t *testing.T, html string) string {
	t.Helper()
	start := strings.Index(html, "GDD（積算温度")
	if start < 0 {
		t.Fatalf("GDD パネルが描画されていない:\n%s", html)
	}
	end := strings.Index(html[start:], "最新計測データ")
	if end < 0 {
		return html[start:]
	}
	return html[start : start+end]
}

// 6.3 ページ経路: GET /devices/{id} に GDD パネルが描画され、period に依らず GDD 部分が不変（非連動）。
func TestShow_GDDパネルがページに描画されperiod非連動(t *testing.T) {
	r := newShowRouterWithUser(&DeviceHandler{Repo: gddRepo()}, 7)

	w24 := getPath(r, "/devices/1?period=24h")
	if w24.Code != http.StatusOK {
		t.Fatalf("period=24h status=%d, want 200", w24.Code)
	}
	body24 := w24.Body.String()

	// GDD 累積曲線の器（#gdd-chart）と数値カード（累積 GDD）が描画される。
	if !strings.Contains(body24, `id="gdd-chart"`) {
		t.Errorf("GDD パネル(#gdd-chart)が描画されていない:\n%s", body24)
	}
	if !strings.Contains(body24, "60 ℃·日") {
		t.Errorf("GDD 累積カードの具体値が描画されていない")
	}

	// period=30d でも GDD 部分は同一（定植日→現在の全期間ゆえ期間に依らない・R6.2）。
	r2 := newShowRouterWithUser(&DeviceHandler{Repo: gddRepo()}, 7)
	w30 := getPath(r2, "/devices/1?period=30d")
	if w30.Code != http.StatusOK {
		t.Fatalf("period=30d status=%d, want 200", w30.Code)
	}
	body30 := w30.Body.String()

	if gddSectionOf(t, body24) != gddSectionOf(t, body30) {
		t.Errorf("GDD パネル部分が period で変化している（period 非連動に反する）\n24h:\n%s\n30d:\n%s",
			gddSectionOf(t, body24), gddSectionOf(t, body30))
	}
}

// 6.3 所有者認可: 非所有デバイスの GET は 404（列挙防止・R8.1/8.2）。GDD パネルも漏らさない。
func TestShow_GDD非所有デバイスは404(t *testing.T) {
	// device 1 の所有者は uid=7。別ユーザー(8)で要求 → 404。
	r := newShowRouterWithUser(&DeviceHandler{Repo: gddRepo()}, 8)
	w := getPath(r, "/devices/1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("非所有デバイスの status=%d, want 404", w.Code)
	}
}
