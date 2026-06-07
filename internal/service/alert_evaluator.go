// Package service はアラート判定等のドメインサービスを提供する。
package service

import (
	"context"
	"log/slog"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
)

// AlertEvaluatorRepo は AlertEvaluator が必要とする最小 DB ポート (consumer interface)。
// repository.Querier / *repository.Queries がこれを満たす。先回りの抽象化を避け、
// アラート判定に必要な2メソッドだけに限定する (DIP / アーキ決定の「consumer 最小 interface」)。
type AlertEvaluatorRepo interface {
	ListEnabledAlertRulesByDevice(ctx context.Context, deviceID int64) ([]repository.AlertRule, error)
	CreateAlertHistory(ctx context.Context, arg repository.CreateAlertHistoryParams) (repository.AlertHistory, error)
}

// AlertEvaluator は受信センサーデータに対するアラート判定の入口。
// センサーデータ受信ハンドラから同期的に呼ばれる。
type AlertEvaluator struct {
	Repo   AlertEvaluatorRepo
	Logger *slog.Logger // nil の場合は slog.Default() を使う
}

// logger は Logger 未設定時に slog.Default() へフォールバックする。
func (e *AlertEvaluator) logger() *slog.Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return slog.Default()
}

// EvaluateAndNotify は reading のデバイスに紐づく有効なアラートルールを評価し、
// 条件にマッチしたルールについて alert_histories を作成して返す。
//
// フロー:
//   - 有効ルール取得 (ListEnabledAlertRulesByDevice。is_enabled かつ未削除のみ)
//   - 各ルールの指標 (temperature / humidity) に対応する実測値を Evaluate で判定
//   - 発火したルールは CreateAlertHistory で履歴化し、戻り値スライスへ追加
//
// エラー方針 (ベストエフォート・トランザクション不使用):
//   - ルール取得に失敗したら (nil, err)
//   - 履歴作成に失敗したら、それまでに作成済みのスライスと err を返して中断
//     (既作成分はロールバックしない)
//   - 値域外の指標・演算子を持つルールは安全に読み飛ばして継続する (fail-safe)
//
// 戻り値: 作成した履歴スライス (発火0件なら nil) と error。
func (e *AlertEvaluator) EvaluateAndNotify(
	ctx context.Context,
	reading *repository.SensorReading,
) ([]repository.AlertHistory, error) {
	rules, err := e.Repo.ListEnabledAlertRulesByDevice(ctx, reading.DeviceID)
	if err != nil {
		e.logger().ErrorContext(ctx, "アラートルールの取得に失敗",
			"device_id", reading.DeviceID, "error", err)
		return nil, err
	}

	var fired []repository.AlertHistory
	for _, rule := range rules {
		actual, ok := actualValueFor(rule.Metric, reading)
		if !ok {
			// CHECK 制約で通常到達しないが、値域外の指標は無視して継続する。
			e.logger().WarnContext(ctx, "未知の指標のルールをスキップ",
				"rule_id", rule.ID, "metric", rule.Metric)
			continue
		}

		op := domain.ComparisonOperator(rule.Operator)
		if !op.Valid() {
			e.logger().WarnContext(ctx, "未知の演算子のルールをスキップ",
				"rule_id", rule.ID, "operator", rule.Operator)
			continue
		}

		// 比較のときだけ float へ変換する。保存値 (actual / threshold) は
		// pgtype.Numeric のまま渡して float 往復による精度劣化を避ける。
		if !op.Evaluate(pgconv.NumericToFloat(actual), pgconv.NumericToFloat(rule.Threshold)) {
			continue
		}

		history, err := e.Repo.CreateAlertHistory(ctx, repository.CreateAlertHistoryParams{
			AlertRuleID: rule.ID,
			Metric:      rule.Metric,   // 発火時点の値を非正規化保持
			Operator:    rule.Operator, // 同上
			Threshold:   rule.Threshold,
			ActualValue: actual,
			TriggeredAt: reading.RecordedAt, // 発火日時 = 計測時刻
			// is_notified は渡さない → DB デフォルト false (未通知)
		})
		if err != nil {
			e.logger().ErrorContext(ctx, "アラート履歴の作成に失敗",
				"rule_id", rule.ID, "error", err)
			return fired, err
		}
		fired = append(fired, history)
	}

	e.logger().DebugContext(ctx, "アラート判定完了",
		"device_id", reading.DeviceID,
		"rules_evaluated", len(rules),
		"alerts_fired", len(fired),
	)
	return fired, nil
}

// actualValueFor はルールの指標 (metric) に対応する実測値を reading から選ぶ。
// 既知の指標 (temperature / humidity) でない場合は ok=false を返す。
func actualValueFor(metric string, reading *repository.SensorReading) (pgtype.Numeric, bool) {
	switch domain.Metric(metric) {
	case domain.MetricTemperature:
		return reading.Temperature, true
	case domain.MetricHumidity:
		return reading.Humidity, true
	default:
		return pgtype.Numeric{}, false
	}
}
