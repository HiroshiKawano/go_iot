# ギャップ分析: alert-evaluation

> 既存コードベースと要件の差分を分析し、設計フェーズの方針決定に資する。本書は判断ではなく「情報と選択肢」を提供する。
> 調査日: 2026-06-07 / 対象要件: requirements.md（Req 1〜6）

## 分析サマリ

- **アルゴリズム本体は実装・テスト済み**。`domain.ComparisonOperator.Evaluate` が4演算子の境界値挙動を完備（Req 3 はほぼ既存資産でカバー）。
- **DB ポートも実装済み**。`ListEnabledAlertRulesByDevice`（is_enabled+deleted_at で絞込）・`CreateAlertHistory`（非正規化スナップショット保存）が sqlc 生成済みで `repository.Querier` に含まれる。
- **欠落は実質3点**: ①`internal/service/alert_evaluator.go`（判定オーケストレーション）②`sensor_api.go` への同期接続配線 ③service とハンドラ拡張のテスト。
- **主要な設計判断は「evaluator の注入シーム」**。session プロンプトの inline 構築（`&service.AlertEvaluator{Repo: h.Repo}`）は既存ハンドラ用モック `fakeSensorRepo`（8テスト）にアラート2メソッドの実装を強制する。steering の DIP ルール（consumer 最小 interface）と整合する代替がある。
- **総合: Effort S / Risk Low**。確立パターンの延長で、新規外部依存なし。

---

## 1. 現状資産の調査（Current State）

### 1.1 実装済み・再利用可能な資産

| 資産 | 場所 | 状態 | 備考 |
|---|---|---|---|
| `ComparisonOperator.Evaluate(actual, threshold float64) bool` | `internal/domain/comparison_operator.go:34` | **完成・テスト済** | `>` `<` `>=` `<=` の境界値挙動。`comparison_operator_test.go` 併設 |
| `ComparisonOperator.Valid()` / `ParseComparisonOperator` | 同上 | 完成 | Enum 値域外の防御に使用可 |
| `Metric`（temperature/humidity）+ `Valid()` | `internal/domain/metric.go` | 完成 | metric→実測値マッピングの分岐に使用 |
| `ListEnabledAlertRulesByDevice(ctx, deviceID int64) ([]AlertRule, error)` | `db/queries/alert_rules.sql` / `repository` 生成 | 完成 | **`is_enabled=TRUE AND deleted_at IS NULL` を既にクエリ側で担保**（Req 2-1/2-2 をクエリで充足） |
| `CreateAlertHistory(ctx, CreateAlertHistoryParams) (AlertHistory, error)` | `db/queries/alert_histories.sql` / `repository` 生成 | 完成 | 非正規化 metric/operator/threshold + actual_value + triggered_at を INSERT。is_notified は DB デフォルト false |
| `repository.Querier`（interface） | `internal/repository/querier.go` | 完成 | 上記2メソッドを含む。service の consumer 最小 interface はここから切出し可 |
| `pgconv.Numeric2 / NumericToFloat / Timestamptz / TimestamptzToTime` | `internal/infra/pgconv/pgconv.go` | 完成 | Numeric↔float64 変換。**actual_value/threshold は pgtype.Numeric のまま渡せる**（後述・精度温存） |
| `SensorAPI.Create`（受信ハンドラ） | `internal/handler/sensor_api.go:71` | 受信〜保存まで完成 | L117 `UpdateDeviceLastCommunicated` 後が判定接続点。所有者検証（403）は L91 で完了済み |
| ハンドラテスト雛形（`fakeSensorRepo` + httptest） | `internal/handler/sensor_api_test.go` | 完成 | 日本語テスト名・`newRouterWithUser`・`postSensorData` ヘルパーを踏襲可 |

### 1.2 型シグネチャ（設計の前提・すべて生成済み）

```
AlertRule{ ID int64; DeviceID int64; Metric string; Operator string;
           Threshold pgtype.Numeric; IsEnabled bool; ... DeletedAt }
SensorReading{ ID int64; DeviceID int64; Temperature pgtype.Numeric;
               Humidity pgtype.Numeric; RecordedAt; CreatedAt; ... }
AlertHistory{ ID; AlertRuleID; Metric string; Operator string;
              Threshold pgtype.Numeric; ActualValue pgtype.Numeric;
              IsNotified bool; TriggeredAt pgtype.Timestamptz; ... }
CreateAlertHistoryParams{ AlertRuleID int64; Metric string; Operator string;
                          Threshold pgtype.Numeric; ActualValue pgtype.Numeric;
                          TriggeredAt pgtype.Timestamptz }
```

### 1.3 規約・アーキ制約（structure.md / tech.md より）

- **DIP は2点限定**: DB ポート=`repository.Querier`、service の consumer interface は必要時のみ最小メソッドで定義。先回り抽象化はしない。
- **依存方向**: `handler → service → repository → infra` / `domain` は純粋層。handler が service を import するのは順方向で許容。
- **domain 純粋性**: `domain` は pgtype/gin/repository を import しない（既存遵守。本実装でも崩さない）。
- **既存のベストエフォート前例**: `sensor_api.go:117` は `_ = UpdateDeviceLastCommunicated(...)` でエラーを握り潰す。アラート判定も同様の「201 を妨げない」前例に倣える。

---

## 2. 要件→資産マップ（ギャップタグ: Missing / Decision / Constraint）

| 要件 | 充足する既存資産 | ギャップ |
|---|---|---|
| **R1** 受信時の同期評価・デバイス限定 | `ListEnabledAlertRulesByDevice`（デバイス限定済）/ `Create` フロー | **Missing**: `EvaluateAndNotify` 本体 / L117 後への同期呼び出し配線 |
| **R2** 有効ルール選定・metric→実測値 | クエリが is_enabled+deleted_at を担保 / `domain.Metric` / reading.Temperature・Humidity | **Missing**: metric 分岐で実測値を選ぶ service ロジック |
| **R3** 演算子境界値 | **`ComparisonOperator.Evaluate` 完成・テスト済** | ギャップほぼ無し（呼び出すだけ）。Risk Low |
| **R4** 発火時スナップショット記録 | `CreateAlertHistory` / `CreateAlertHistoryParams`（非正規化列）/ is_notified DB デフォルト false | **Missing**: params 組立 / **Decision**: triggered_at の出所（RecordedAt vs 受信時刻 now vs CreatedAt） |
| **R5** 複数発火・No-op | `[]AlertRule` 反復（空も自然に No-op）/ スライス返却 | **Missing**: 反復ロジック / **Decision(任意)**: `CreateSensorReadingResponse.AlertsFired` 拡張の要否 |
| **R6** ベストエフォート耐性 | 既存 `_ = Update...` 前例 | **Missing**: evaluator 呼出後のエラー握り潰し配線 / **Decision**: ロガー（slog）の導入形態 |

> 補足: クエリ側で is_enabled/deleted_at を絞るため、Req 2-1/2-2 は service 側で再フィルタ不要。service は「取得済み有効ルール」を反復するだけでよい。

---

## 3. 実装アプローチの選択肢

3つの「シーム」を分けて考えると整理しやすい:
- **シームA**（service→repository）: evaluator が必要とする2メソッドへの依存形態。
- **シームB**（handler→evaluator）: ハンドラが評価機能をどう保持/注入するか。← **最大の判断点**
- **シームC**（ロギング）: best-effort 失敗の記録手段。

### Option A — session プロンプト準拠（inline 構築・SensorRepo 拡張）
`SensorRepo` interface に `ListEnabledAlertRulesByDevice` と `CreateAlertHistory` を追加し、ハンドラ内で `evaluator := &service.AlertEvaluator{Repo: h.Repo}` を inline 構築。
- ✅ 配線が最小（main.go は無改修。h.Repo を流用）
- ✅ session プロンプトの記述そのまま
- ❌ **既存 `fakeSensorRepo`（8テスト）にアラート2メソッドの実装追加を強制**（既存テストへ波及）
- ❌ `SensorRepo`（受信の関心事）にアラート関心事が混入し、consumer 最小 interface の趣旨が薄れる

### Option B — 新規 service + evaluator を interface 注入（推奨候補）
`internal/service/alert_evaluator.go` に `AlertEvaluator` と**最小 repo interface**（`ListEnabledAlertRulesByDevice`+`CreateAlertHistory` の2メソッド）を定義。`SensorAPI` に小さな `Evaluator interface { EvaluateAndNotify(ctx, *repository.SensorReading) ([]repository.AlertHistory, error) }` フィールドを追加し、合成ルート `cmd/server/main.go` で注入。
- ✅ **DIP ルール（consumer 最小 interface）に最も整合**
- ✅ 既存 `fakeSensorRepo` は無改修。ハンドラテストは no-op/スパイ evaluator を注入するだけ（波及最小）
- ✅ service を単体テストで隔離しやすい（最小 repo モック）
- ❌ main.go の配線が1行増える / ハンドラに interface フィールド追加

### Option C — ハイブリッド（concrete を合成ルートで組立）
service は最小 repo interface に依存（Bと同じ）。ただしハンドラは `Evaluator interface` ではなく `*service.AlertEvaluator` を**フィールドとして保持**し、main.go で `repository.Queries`（Querier 実装）から構築。
- ✅ service の隔離性は確保
- ✅ ハンドラの interface 追加を避けつつ既存 SensorRepo を汚さない
- ❌ ハンドラテストで evaluator を差し替えるには concrete を組む必要があり、handler 単体テストの隔離が B より弱い（service の repo モックまで持ち込む）

---

## 4. Effort / Risk

| 区分 | 評価 | 根拠 |
|---|---|---|
| **Effort** | **S（1–3日）** | 判定アルゴリズム・クエリ・型変換が既存。新規は service 1ファイル + 配線 + テスト2系統。新規外部依存なし |
| **Risk** | **Low** | 確立パターンの延長・スコープ明確・統合点は1箇所（L117 後）。残る不確実性は triggered_at の出所と注入シームの選択のみ |

---

## 5. 設計フェーズへの申し送り

### 推奨方針（design で確定）
- **シームB は Option B を第一候補**（DIP 整合・既存テスト無改変）。session プロンプトの inline 構築（Option A）は「動くが関心事が混入」する点を design で明示的に比較し採否を決める。
- **actual_value / threshold は pgtype.Numeric のまま** `CreateAlertHistoryParams` へ渡す（reading.Temperature/Humidity・rule.Threshold はすでに Numeric）。比較のときだけ `pgconv.NumericToFloat` で float 化する。float↔Numeric の往復による精度劣化を避ける。
- metric/operator はルールの文字列値を**そのまま**履歴へ非正規化（Req 4-2）。`domain.Metric/ComparisonOperator.Valid()` は防御ガードとして使い、値域外ルールは skip + ログ（CHECK 制約で通常到達しないが fail-safe）。

### Research / Decision items（design で要確定）
1. **triggered_at の出所**: `reading.RecordedAt`（計測時刻）/ 受信サーバ時刻 now / `reading.CreatedAt` のいずれか。→ `DB設計書.md §アラート判定の実行タイミング` を確認して確定。
2. **注入シーム**: Option A / B / C の採否（上記比較に基づく）。既存 `fakeSensorRepo` への波及可否が判断材料。
3. **ロガー形態**: パッケージレベル `slog` vs 注入 `*slog.Logger`。tech 制約「判定フロー入出力を DEBUG 出力し、テストで検証」を満たす形を選ぶ。出力をテストで検証するなら注入式が有利。
4. **レスポンス拡張**: `CreateSensorReadingResponse.AlertsFired int` を今回含めるか（Req 5-4 は Where=任意）。デバイス/運用者が件数を観測できる利点 vs スコープ最小化。
5. **値域外ルールの扱い**: `Valid()` false 時に skip(+log) か error か。best-effort 方針との整合で skip 推奨だが design で明文化。
6. **空スライスの返却契約**: ルール0件/非発火時に `nil` を返すか `[]AlertHistory{}` を返すか（テスト期待値 len==0 に影響。Req 5-2/5-3）。

### テスト設計の起点（tasks/impl で詳細化・テストガイダンス集を参照）
- **service 単体**: 最小 repo モック（手書き Querier 部分実装）で DB 非依存。テーブル駆動で基本系/複数ルール/境界値(35.00/35.01・29.99/30.00 等)/0件/CreateAlertHistory エラー/複数同時発火。
- **handler 拡張**: 既存 `fakeSensorRepo`+`newRouterWithUser` を踏襲。Option B なら no-op/スパイ evaluator を注入し「201 が評価失敗で妨げられない」「発火時に評価が呼ばれる」を検証。

---

## 6. 設計フェーズ確定事項（design synthesis / 2026-06-07）

> design.md 生成にあたり、§5 の Research/Decision items 6件を以下のとおり確定。根拠は DB設計書 §アラート判定の実行タイミング（L649-）/ §alert_histories（L482-）/ tech.md DIP ルール / テストガイダンス集 §39（Querier モック統一）。

| # | 判断 | 確定内容 | 根拠 |
|---|---|---|---|
| 1 | **triggered_at の出所** | `reading.RecordedAt`（デバイス計測時刻）を採用 | 「発火日時」は異常計測が発生した時刻が最も意味的に正確。かつ `time.Now()` を service に持ち込まずに済み**クロック注入なしで決定的にテスト可能**。代替（受信サーバ now / CreatedAt）は棄却（テスト容易性と意味で劣る）。RecordedAt と CreatedAt の差は同期実行のため小さい |
| 2 | **注入シーム** | **Option B**: `service.AlertEvaluator`（concrete）+ 最小 `service.AlertEvaluatorRepo`（2メソッド）。`handler` 側に consumer interface `AlertEvaluator{ EvaluateAndNotify }` を定義し `SensorAPI.Evaluator` フィールドで注入 | tech.md/structure.md の DIP「consumer 最小 interface は消費側に定義」。既存 `fakeSensorRepo`（8テスト）を**無改変**に保ち、handler テストは spy evaluator 注入で隔離。session プロンプトの inline 構築（Option A）は SensorRepo にアラート関心事が混入するため棄却 |
| 3 | **ロガー** | `AlertEvaluator.Logger *slog.Logger`（nil 時 `slog.Default()`）。**エラーログは service が出す**（Req 6.2 を service が充足）。DEBUG で判定フロー入出力（ルール件数・発火件数）を出力 | tech.md「判定フロー入出力を DEBUG 出力しテストで検証」。注入式で `slog.New(handler)` をテストから差し替え `bytes.Buffer` で検証可能。handler はエラーを握り潰すのみ（service が記録済み） |
| 4 | **レスポンス拡張** | `CreateSensorReadingResponse.AlertsFired int`（= `len(alerts)`）を**今回含める** | Req 5.4（Where=任意）を充足。受入基準7（seed 後の動作確認）の観測点になり低コスト。デバイス/運用者が発火件数を確認できる |
| 5 | **値域外ルールの扱い** | `domain.Metric/ComparisonOperator.Valid()` が false のルールは **skip + WARN ログ**して継続（error にしない） | CHECK 制約で通常到達しない fail-safe。1件の不正ルールで評価全体を止めない（best-effort 方針と整合） |
| 6 | **空スライス返却契約** | 発火0件時は `nil` を返す（Go 慣用）。呼び出し側・テストは `len(result)` で判定（`nil` でも `len==0`） | Req 5.2/5.3。`AlertsFired` も `len(nil)==0` で正しく0 |

### 値の取り回し（精度温存・確定）
- 比較は `pgconv.NumericToFloat(reading.Temperature|Humidity)` と `pgconv.NumericToFloat(rule.Threshold)` を `domain.ComparisonOperator.Evaluate(actual, threshold)` へ。
- 履歴保存は **pgtype.Numeric のまま**: `CreateAlertHistoryParams.ActualValue = reading.Temperature|Humidity`、`.Threshold = rule.Threshold`、`.Metric/.Operator = rule.Metric/rule.Operator`（文字列を非正規化そのまま）。float↔Numeric 往復を判定時のみに限定し保存値の精度劣化を回避。
- `CreateAlertHistory` は `is_notified` を渡さない → DB デフォルト `false`（Req 4.4）。

### CreateAlertHistory 失敗時の挙動（確定）
- 反復中に最初の `CreateAlertHistory` が error を返したら `(発火済みスライス, err)` を返して**中断**。トランザクション不使用のため**作成済み履歴はロールバックしない**（Req 6.3）。呼び出し側 handler は err を握り潰し 201 を返す（Req 6.1）。service が error を ERROR ログ出力（Req 6.2）。

### Build vs Adopt / Simplification（synthesis）
- **Adopt**: 判定アルゴリズム=既存 `domain.ComparisonOperator.Evaluate`、ルール取得/履歴保存=既存 sqlc クエリ、型変換=既存 pgconv。新規ロジックは「取得→分岐→判定→保存」のオーケストレーションのみ。
- **Simplification**: 先回りの抽象化をしない。`service.AlertEvaluatorRepo` は2メソッド限定、handler `AlertEvaluator` は1メソッド限定。Notifier/通知抽象・トランザクション層・非同期キューは現スコープ外のため**作らない**。
