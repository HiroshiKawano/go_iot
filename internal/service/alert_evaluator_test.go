package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// fixedRecordedAt はテスト全体で使う固定の計測時刻。
// triggered_at = reading.RecordedAt を採用しているため、time.Now() を持ち込まず決定的に検証できる。
var fixedRecordedAt = time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

// fakeAlertRepo は AlertEvaluatorRepo の最小モック（DB 非依存）。
// rules を ListEnabledAlertRulesByDevice の戻り値として返し、CreateAlertHistory の呼び出しを記録する。
type fakeAlertRepo struct {
	rules     []repository.AlertRule
	listErr   error
	createErr error
	failOnNth int // 0 = createErr を全呼び出しで適用。N>=1 で N 回目の呼び出しだけ失敗させる。

	createN int                                   // CreateAlertHistory が呼ばれた回数
	created []repository.CreateAlertHistoryParams // 成功した CreateAlertHistory の引数記録
}

func (f *fakeAlertRepo) ListEnabledAlertRulesByDevice(_ context.Context, _ int64) ([]repository.AlertRule, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.rules, nil
}

func (f *fakeAlertRepo) CreateAlertHistory(_ context.Context, arg repository.CreateAlertHistoryParams) (repository.AlertHistory, error) {
	f.createN++
	if f.createErr != nil && (f.failOnNth == 0 || f.failOnNth == f.createN) {
		return repository.AlertHistory{}, f.createErr
	}
	f.created = append(f.created, arg)
	return repository.AlertHistory{
		ID:          int64(f.createN),
		AlertRuleID: arg.AlertRuleID,
		Metric:      arg.Metric,
		Operator:    arg.Operator,
		Threshold:   arg.Threshold,
		ActualValue: arg.ActualValue,
		TriggeredAt: arg.TriggeredAt,
	}, nil
}

// rule は有効なアラートルールを組み立てるテストヘルパー。
func rule(id int64, metric domain.Metric, op domain.ComparisonOperator, threshold float64) repository.AlertRule {
	return repository.AlertRule{
		ID:        id,
		DeviceID:  1,
		Metric:    string(metric),
		Operator:  string(op),
		Threshold: threshold,
		IsEnabled: true,
	}
}

// makeReading は計測値（温度・湿度）を持つ SensorReading を組み立てるテストヘルパー。
func makeReading(deviceID int64, temp, hum float64) *repository.SensorReading {
	return &repository.SensorReading{
		DeviceID:    deviceID,
		Temperature: temp,
		Humidity:    hum,
		RecordedAt:  fixedRecordedAt,
	}
}

// --- 1.1 単一ルールの発火判定と発火履歴スナップショットの作成 ---

func TestEvaluateAndNotify_温度ルール発火で履歴1件とスナップショット(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		rule(10, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
	}}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 50.0))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if got := len(fired); got != 1 {
		t.Fatalf("発火件数: got %d, want 1", got)
	}
	if repo.createN != 1 {
		t.Errorf("CreateAlertHistory 呼び出し回数: got %d, want 1", repo.createN)
	}

	// 発火時点のルール内容を非正規化スナップショットとして保存していること
	p := repo.created[0]
	if p.AlertRuleID != 10 {
		t.Errorf("AlertRuleID: got %d, want 10", p.AlertRuleID)
	}
	if p.Metric != string(domain.MetricTemperature) {
		t.Errorf("Metric: got %q, want temperature", p.Metric)
	}
	if p.Operator != string(domain.OpGreaterThan) {
		t.Errorf("Operator: got %q, want >", p.Operator)
	}
	if got := p.Threshold; got != 35.00 {
		t.Errorf("Threshold: got %v, want 35.00", got)
	}
	// 実測値は温度の値（湿度と取り違えない）
	if got := p.ActualValue; got != 36.0 {
		t.Errorf("ActualValue: got %v, want 36.0", got)
	}
	// triggered_at = 計測時刻
	if !p.TriggeredAt.Equal(fixedRecordedAt) {
		t.Errorf("TriggeredAt: got %v, want %v", p.TriggeredAt, fixedRecordedAt)
	}
}

func TestEvaluateAndNotify_湿度ルールは湿度を実測値に使う(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		rule(20, domain.MetricHumidity, domain.OpLessThan, 30.00),
	}}
	e := &AlertEvaluator{Repo: repo}

	// 温度 99（高い）・湿度 25（閾値割れ）。湿度ルールなので湿度で判定・記録されるべき。
	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 99.0, 25.0))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(fired) != 1 {
		t.Fatalf("発火件数: got %d, want 1", len(fired))
	}
	if got := repo.created[0].ActualValue; got != 25.0 {
		t.Errorf("ActualValue: got %v, want 25.0（湿度。温度99と取り違えていない）", got)
	}
}

// --- 1.2 比較演算子の境界値網羅と非該当時の無発火 ---

func TestEvaluateAndNotify_演算子の境界値(t *testing.T) {
	tests := []struct {
		name      string
		op        domain.ComparisonOperator
		threshold float64
		actual    float64
		wantFired bool
	}{
		{"gt ちょうどは非発火", domain.OpGreaterThan, 35.00, 35.00, false},
		{"gt 超過は発火", domain.OpGreaterThan, 35.00, 35.01, true},
		{"gt 下回りは非発火", domain.OpGreaterThan, 35.00, 34.99, false},
		{"lt ちょうどは非発火", domain.OpLessThan, 30.00, 30.00, false},
		{"lt 下回りは発火", domain.OpLessThan, 30.00, 29.99, true},
		{"gte ちょうどは発火", domain.OpGreaterThanOrEqual, 35.00, 35.00, true},
		{"gte 下回りは非発火", domain.OpGreaterThanOrEqual, 35.00, 34.99, false},
		{"lte ちょうどは発火", domain.OpLessThanOrEqual, 30.00, 30.00, true},
		{"lte 超過は非発火", domain.OpLessThanOrEqual, 30.00, 30.01, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeAlertRepo{rules: []repository.AlertRule{
				rule(1, domain.MetricTemperature, tt.op, tt.threshold),
			}}
			e := &AlertEvaluator{Repo: repo}

			fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, tt.actual, 50.0))
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			wantN := 0
			if tt.wantFired {
				wantN = 1
			}
			if len(fired) != wantN || repo.createN != wantN {
				t.Errorf("発火: len=%d createN=%d, want %d (op=%s threshold=%v actual=%v)",
					len(fired), repo.createN, wantN, tt.op, tt.threshold, tt.actual)
			}
		})
	}
}

// --- 1.3 複数同時発火と No-op の返却契約 ---

func TestEvaluateAndNotify_複数ルール同時発火で各1件(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		rule(1, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
		rule(2, domain.MetricHumidity, domain.OpLessThan, 30.00),
	}}
	e := &AlertEvaluator{Repo: repo}

	// 温度 36（>35 成立）・湿度 25（<30 成立）→ 両方発火
	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(fired) != 2 || repo.createN != 2 {
		t.Fatalf("発火件数: len=%d createN=%d, want 2", len(fired), repo.createN)
	}
	if repo.created[0].Metric != string(domain.MetricTemperature) {
		t.Errorf("1件目 Metric: got %q, want temperature", repo.created[0].Metric)
	}
	if repo.created[1].Metric != string(domain.MetricHumidity) {
		t.Errorf("2件目 Metric: got %q, want humidity", repo.created[1].Metric)
	}
}

func TestEvaluateAndNotify_ルール0件はNoop(t *testing.T) {
	repo := &fakeAlertRepo{rules: nil}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(fired) != 0 {
		t.Errorf("発火件数: got %d, want 0", len(fired))
	}
	if repo.createN != 0 {
		t.Errorf("CreateAlertHistory は呼ばれないはず: got %d", repo.createN)
	}
}

func TestEvaluateAndNotify_全ルール非該当はNoop(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		rule(1, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
		rule(2, domain.MetricHumidity, domain.OpLessThan, 30.00),
	}}
	e := &AlertEvaluator{Repo: repo}

	// 温度 20（>35 不成立）・湿度 50（<30 不成立）
	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 20.0, 50.0))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(fired) != 0 || repo.createN != 0 {
		t.Errorf("発火件数: len=%d createN=%d, want 0", len(fired), repo.createN)
	}
}

// --- 1.4 ベストエフォートのエラー耐性と観測ログ ---

func TestEvaluateAndNotify_ルール取得失敗はエラー返却(t *testing.T) {
	repo := &fakeAlertRepo{listErr: errors.New("db down")}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err == nil {
		t.Fatal("ルール取得失敗時はエラーを返すべき")
	}
	if fired != nil {
		t.Errorf("エラー時の戻りスライスは nil であるべき: got %v", fired)
	}
}

func TestEvaluateAndNotify_履歴作成失敗で中断しエラー返却(t *testing.T) {
	repo := &fakeAlertRepo{
		rules:     []repository.AlertRule{rule(1, domain.MetricTemperature, domain.OpGreaterThan, 35.00)},
		createErr: errors.New("insert failed"),
	}
	e := &AlertEvaluator{Repo: repo}

	_, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err == nil {
		t.Fatal("履歴作成失敗時はエラーを返すべき")
	}
}

func TestEvaluateAndNotify_複数発火の途中失敗は既作成分を取り消さない(t *testing.T) {
	repo := &fakeAlertRepo{
		rules: []repository.AlertRule{
			rule(1, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
			rule(2, domain.MetricHumidity, domain.OpLessThan, 30.00),
		},
		createErr: errors.New("insert failed"),
		failOnNth: 2, // 1件目は成功・2件目で失敗
	}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err == nil {
		t.Fatal("2件目の履歴作成失敗時はエラーを返すべき")
	}
	// ベストエフォート: 1件目は作成済みで取り消されない（戻りスライスに残る・2回試行している）
	if len(fired) != 1 {
		t.Errorf("既作成分の戻り: got %d, want 1（ロールバックしない）", len(fired))
	}
	if repo.createN != 2 {
		t.Errorf("両ルールを試行しているべき: createN got %d, want 2", repo.createN)
	}
}

func TestEvaluateAndNotify_値域外指標のルールはスキップして継続(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		{ID: 1, DeviceID: 1, Metric: "pressure", Operator: string(domain.OpGreaterThan), Threshold: 10, IsEnabled: true}, // 値域外
		rule(2, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
	}}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0))
	if err != nil {
		t.Fatalf("値域外ルールは無視して継続すべき（エラーにしない）: %v", err)
	}
	if len(fired) != 1 || repo.createN != 1 {
		t.Fatalf("有効ルールのみ発火すべき: len=%d createN=%d, want 1", len(fired), repo.createN)
	}
	if repo.created[0].AlertRuleID != 2 {
		t.Errorf("発火したのは有効ルール(id=2)のはず: got %d", repo.created[0].AlertRuleID)
	}
}

func TestEvaluateAndNotify_値域外演算子のルールはスキップ(t *testing.T) {
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		{ID: 1, DeviceID: 1, Metric: string(domain.MetricTemperature), Operator: "==", Threshold: 35, IsEnabled: true}, // 値域外演算子
	}}
	e := &AlertEvaluator{Repo: repo}

	fired, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 35.0, 25.0))
	if err != nil {
		t.Fatalf("値域外演算子は無視して継続すべき: %v", err)
	}
	if len(fired) != 0 || repo.createN != 0 {
		t.Errorf("値域外演算子は発火しないべき: len=%d createN=%d, want 0", len(fired), repo.createN)
	}
}

func TestEvaluateAndNotify_判定フローをデバッグログ出力(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	repo := &fakeAlertRepo{rules: []repository.AlertRule{
		rule(1, domain.MetricTemperature, domain.OpGreaterThan, 35.00),
	}}
	e := &AlertEvaluator{Repo: repo, Logger: logger}

	if _, err := e.EvaluateAndNotify(context.Background(), makeReading(1, 36.0, 25.0)); err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "alerts_fired") {
		t.Errorf("DEBUG ログに発火件数(alerts_fired)が含まれるべき: %q", out)
	}
}
