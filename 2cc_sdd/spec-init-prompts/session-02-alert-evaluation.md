# セッション2 spec-init プロンプト: アラート判定ロジック（バックエンド・非UI）

> 使い方: 下の「--- spec-init 本文 ここから ---」以降を /kiro-spec-init の引数として渡す。
> 推奨 feature-name: alert-evaluation
> 実行例: /kiro-spec-init "（本文を貼り付け）"
> 前提セッション: なし（S1 と並行実装可。UI 非依存）
> 設計フェーズで参照: 実装現状サマリ.md §5.1（ComparisonOperator.Evaluate）・§5.4（sensor_api.go の Create フロー）/ DB設計書.md §Enum定義・§テーブル定義 alert_rules/alert_histories・§アラート判定の実行タイミング・§主要クエリパターン（アラート判定用）/ HTMX実装ガイド.md §5（OOB想定記述、1587-1616行）

--- spec-init 本文 ここから ---

## 機能概要

農場運営者が設定したアラートルール（気温が 35℃ を超えたら、湿度が 30% 未満になったら等）に基づき、ESP8266 デバイスから送信されたセンサーデータを受信した際に、異常値判定を同期的に実行して `alert_histories` テーブルへ保存する。現状は domain.ComparisonOperator.Evaluate() と CreateAlertHistory は実装済みだが、センサーデータ受信ハンドラ（POST /api/sensor-data）内でこれらの接続が未着手のため、本セッションではこの接続とテストを完成させる。

## 背景・現状

**実装済み資産:**
- `internal/domain/comparison_operator.go`: Evaluate(actual, threshold float64) bool メソッド実装済み（> / < / >= / <= の4演算子対応、境界値検証テスト完備）
- `db/queries/alert_rules.sql`: ListEnabledAlertRulesByDevice と CreateAlertHistory の sqlc クエリ定義完了
- `internal/handler/sensor_api.go`: Create メソッド実装済み。L57 に「Step18/フェーズ7で同期追加予定」コメントのみ
- `cmd/seed/main.go`: シードデータで alert_rules 4件（温度>35.00℃、湿度<30.00% など）と alert_histories 3件を自動投入可能

**未実装**:
- sensor_api.go の Create 内、CreateSensorReading（L89-98）の後にアラート判定ロジックが接続されていない
- internal/service が空ディレクトリ（アラート判定サービスの設置先）
- 判定フロー全体のテスト（単体・統合）

**UI 依存性:**
- なし。このセッションはバックエンド判定ロジック単体で完結（Web UI 層とは独立）
- 将来 S3（ダッシュボード未対応アラートバナー）や S8（アラート履歴画面）で判定結果の表示が必要になるが、本セッションスコープ外

## このセッションのスコープ（実装対象）

### 実装内容

**1. アラート判定サービス（internal/service/alert_evaluator.go 新規作成）**
   - 型: `AlertEvaluator` struct（`*repository.Queries` を持つ）
   - メソッド: `EvaluateAndNotify(ctx context.Context, reading *repository.SensorReading) ([]repository.AlertHistory, error)`
   - フロー:
     - `ListEnabledAlertRulesByDevice(ctx, reading.DeviceID)` で対象デバイスの有効ルール取得
     - ルール0件の場合は空スライス返却（alert_histories 保存なし）
     - 各ルール (alert_rules) について以下の判定:
       - metric が temperature なら actual_value = reading.Temperature
       - metric が humidity なら actual_value = reading.Humidity
       - domain.ComparisonOperator(rule.Operator).Evaluate(actual_value, pgconv.NumericToFloat(rule.Threshold))
       - 判定結果が true なら CreateAlertHistory で INSERT → 戻り値スライスに追加
     - 判定フロー中に CreateAlertHistory が失敗した場合は error を返す（トランザクション不使用、best-effort）
     - 成功した alert_histories のスライスを返す

**2. sensor_api.go の Create ハンドラ修正**
   - L100（UpdateDeviceLastCommunicated 後）に以下を追加:
     ```go
     evaluator := &service.AlertEvaluator{Repo: h.Repo}
     alerts, err := evaluator.EvaluateAndNotify(ctx, &reading)
     // エラーハンドリング: 判定失敗が 201 レスポンスを妨げない (best-effort)
     if err != nil {
         // ログ出力するが、レスポンスは影響を受けない
     }
     // レスポンスボディに alert_histories の件数を追加（任意）
     ```
   - CreateSensorReadingResponse を必要に応じて拡張（AlertsFired int など）

### 使用する sqlc クエリ

- `ListEnabledAlertRulesByDevice(ctx context.Context, deviceID int64) ([]AlertRule, error)` — DB設計書.md §主要クエリパターン / db/queries/alert_rules.sql L10-15
- `CreateAlertHistory(ctx context.Context, params CreateAlertHistoryParams)` — alert_histories テーブルへ INSERT、非正規化カラム (metric / operator / threshold) を保存。db/queries/alert_histories.sql L1-6

> ※ デバイス所有者確認（GetDevice）は本サービスでは不要。POST /api/sensor-data 受信時に sensor_api.go の Create が既に GetDevice で所有者検証（403）を済ませた後に判定が走るため、ここで重複確認しない。判定は reading.DeviceID をそのまま ListEnabledAlertRulesByDevice に渡す。

### テスト（TDD カバレッジ80%以上）

**internal/service/alert_evaluator_test.go（新規作成）**
- 基本系: ルール1件で判定結果 true → alert_history 作成
- 複数ルール: 4件のルール（温度> / 湿度< 等）に対し実測値が各々マッチするかで判定実行
- 境界値: OpGreaterThan でちょうどの値（35.00 = threshold）は false、35.01 は true。OpLessThan で 29.99 は true、30.00 は false。OpGreaterThanOrEqual / OpLessThanOrEqual でちょうどの値 true
- ルール0件時: ListEnabledAlertRulesByDevice が空スライス返却 → alert_history 保存なし（スライス長0）
- CreateAlertHistory エラー: Repo モック で return error → EvaluateAndNotify が error を返す
- 複数ルール同時発火: device_id=1 に温度> / 湿度< 両ルール定義、実測値が両条件マッチ → alert_histories 2件作成

**internal/handler/sensor_api_test.go（既存、拡張）**
- POST /api/sensor-data で 201 返却 → alert_history がバックグラウンドで作成される（HTMX OOB 未対応なので確認は SQL レベル）
- 温度 36℃ 送信、ルール「温度>35℃」有効 → CreateAlertHistory 呼び出し確認（モック verify）
- alert_history 作成失敗が 201 ステータスを妨げない（best-effort）

### 新規作成ファイル

- `internal/service/alert_evaluator.go`（約80-120行）
- `internal/service/alert_evaluator_test.go`（約150-200行、テーブル駆動テスト）
- （既存ファイル修正）`internal/handler/sensor_api.go`（L100 付近に判定接続コード追加）

### 使用する sqlc 関数と型

| 関数 | 入力型 | 戻り値 | 出所 |
|-----|--------|--------|------|
| ListEnabledAlertRulesByDevice | context, int64 | []AlertRule | alert_rules.sql |
| CreateAlertHistory | context, CreateAlertHistoryParams | AlertHistory | alert_histories.sql |

### 型定義（すべて repository 生成済み）

- `AlertRule`: ID, DeviceID, Metric (string), Operator (string), Threshold (pgtype.Numeric), IsEnabled, CreatedAt, UpdatedAt, DeletedAt
- `AlertHistory`: ID, AlertRuleID, Metric (string), Operator (string), Threshold (pgtype.Numeric), ActualValue (pgtype.Numeric), IsNotified, TriggeredAt, CreatedAt, UpdatedAt, DeletedAt
- `SensorReading`: ID, DeviceID, Temperature (pgtype.Numeric), Humidity (pgtype.Numeric), RecordedAt, CreatedAt, UpdatedAt, DeletedAt
- `CreateAlertHistoryParams`: AlertRuleID, Metric, Operator, Threshold, ActualValue, TriggeredAt

## スコープ外（このセッションでやらないこと）

- **Web UI 層全体**（S1 で実装）: templ 画面・HTMX 動的化・ブラウザセッション認証
- **ダッシュボード未対応アラートバナー表示**（S3）: UnreadAlertBanner の templ コンポーネント
- **/alerts/check エンドポイント（将来）**: HTMX OOB で未通知アラート件数をリアルタイム更新する仕組み。DB設計書.md に想定記述（§5 OOB 同時更新エンドポイント一覧）があるが本セッションでは不要
- **アラート履歴画面**（S8）: ListAlertHistoriesPaginated による UI 表示
- **メール等の外部通知送信**: alert_histories.is_notified の更新のみ。実際の送信ロジックは今後実装
- **トランザクション管理**: CreateAlertHistory 失敗時のロールバック。best-effort（失敗は log のみ）で対応
- **非同期キュー化**: センサー受信時の判定は同期実行。非同期化（Redis 等）は不採用（DB設計書.md 設計根拠参照）

## 技術制約・準拠事項

| 項目 | 内容 | 準拠ドキュメント |
|-----|-----|----------------|
| **言語・フレームワーク** | Go 1.26 + Gin v1.12 + sqlc v1.30 | 実装現状サマリ.md §3 |
| **DB 層** | pgx/v5 + sqlc。pgtype.Numeric / pgtype.Timestamptz は pgconv ヘルパーで float64 / time.Time へ変換 | DB設計書.md §Go 構造体でのフィールド型 |
| **Enum 判定** | domain.Metric / domain.ComparisonOperator の値をキャスト・Evaluate() で判定。Enum 値域外は Valid() で事前検証 | 実装現状サマリ.md §5.1 / DB設計書.md §Enum定義 |
| **テスト** | TDD カバレッジ 80%以上。testify/assert / testify/require。リポジトリはモック（emit_interface=true） | 一般的な Go テスト規約 |
| **エラーハンドリング** | CreateAlertHistory 失敗は log.Printf で記録。sensor_api.go の 201 レスポンスを妨げない（best-effort） | 実装現状サマリ.md §5.4 |
| **ログ・デバッグ** | log/slog 等の統一ログレベル使用。判定フロー入出力を DEBUG で出力し、テストで検証 | Gin Logger 標準 |
| **日本語** | コメント・ドキュメント・ログは日本語。コード識別子は英語 | プロジェクト共通 |
| **同期実行タイミング** | sensor_api.go Create() 内で同期実行。非同期キュー不使用 | DB設計書.md §アラート判定の実行タイミング §データフロー対応表 |
| **イミュータブル方針** | SensorReading / AlertRule をポインタで受け取り、内容の変更はなし。スライス返却は新規割当て | Go ベストプラクティス |

## 受け入れ基準（概略）

1. **EvaluateAndNotify メソッド実装**: internal/service/alert_evaluator.go で定義済み、単体テスト 5+ ケース（基本系・複数ルール・境界値・ルール0件・エラー系）で合格
2. **sensor_api.go ハンドラ接続**: L100 後に evaluator 呼び出しコード追加、201 レスポンスが妨げられない（エラー時も HTTP 201 返却）
3. **テストカバレッジ 80%以上**: alert_evaluator.go / sensor_api.go 修正箇所が coverage 80% 達成
4. **境界値テスト合格**: Threshold 35.00℃ で Op> / >= / < / <= の各判定が正しく動作（Evaluate() テスト既存で合格済み想定）
5. **複数ルール同時発火**: 4件ルール + 両条件マッチ実測値で alert_histories 複数件作成確認（SQL 直接確認 または テーブル駆動テスト）
6. **ルール0件時 No-op**: device に alert_rules が0件の場合、alert_histories は作成されない（スライス長 == 0）
7. **シードデータでの動作確認**: `make seed` 後、API テストで温度 36℃・湿度 25% 送信 → `SELECT * FROM alert_histories` で新規行確認

## 未確定事項・要確認（あれば）

- **トランザクション**: CreateAlertHistory 複数件失敗時の扱い。本セッションでは one-by-one INSERT で best-effort（トランザクション不使用）とするが、今後 atomic な一括 INSERT への変更を検討

--- spec-init 本文 ここまで ---
