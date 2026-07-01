# センサー登録地域の天気API取得・DB保存 — 実現可能性調査レポート（完全版）

> **調査日:** 2026-06-30 / 2026-07-01 補足
> **区分:** 実現性調査（spec化・実装は未着手）
> **調査方法:** 並行サブエージェントによる①コードベース調査(3観点)②天気API一次情報調査(2系統)③役場座標検証(合併5市町)
> **位置づけ:** 河野(スタンプ)発の案で調査依頼のみ。顧客(眞境名さん)の確定依頼かは未確認。

---

## 目次

- 第1部 実現可能性レポート（統合判定）
- 第2部 コードベース調査（詳細）
- 第3部 天気API調査（詳細）
- 第4部 役場座標の検証（53地域の代表点）
- 第5部 座標表ドラフト（現時点判明分）
- 付録 調査メタ情報

---

# 第1部 実現可能性レポート（統合判定）

The facts check out: 53 localities are Go string constants (market/town names, not coordinates), no lat/lon anywhere, and `sensor_readings` uses no foreign key with logical-delete + partial indexes. I have everything needed to write the report.

# センサー登録地域(locality)の天気API取得・DB保存 — 実現可能性レポート

作成日: 2026-06-30 / 区分: 調査・実現性判定（実装ではない）

---

## 1. 結論（実現可能か）

**実現可能。難易度 M（中）、工数感は spec 化込みで実働 2〜3 人日、PoC だけなら半日。**

- DB スキーマ追加・sqlc 生成・repository/handler 配線・config 拡張はいずれも `sensor_readings` / `SensorAPI` の既存パターンの複写で済み、技術的リスクは低い（コードベース調査が「学習曲線なし」と評価）。
- 難所は技術ではなく**前提整備が2つ**ある: (a) locality(市町村名の文字列)→緯度経度の対応表が**コード内に一切無い**ため新規作成が必要、(b) **定期実行(ticker/cron)基盤がアプリ内に存在しない**ため新規実装が必要。この2つが「M」の主因。
- ただし**商用ライセンスとユーザー確定要否は未確認**（後述§7・§8）。ここが詰まると工数より先に判断待ちになる。

難易度の内訳:
- weather_readings テーブル+クエリ+repository: **S**（複写で完了）
- 外部HTTPクライアント: **S**（`cmd/sensor-sim/main.go` の `send()` を流用可）
- 53地域→座標の代表点表: **M**（一次データ作成と検証が要る、後述）
- 定期実行基盤: **M**（goroutine+Ticker か systemd-timer の新規導入）

---

## 2. 最大の論点 — locality → 緯度経度の変換が必要か

**必要。これが本案の最大の前提整備コスト。**

### 事実（コードベース調査で確定）
- `internal/domain/locality.go` の 53 地域は **Go の string 定数（市町村名・旧町村名そのもの）**であり、座標は持たない（例: `LocalityNaha Locality = "那覇市"`）。
- `devices` テーブルにも `lat`/`lon` 列は無く（`location` フリーテキスト + `locality` 構造化キーのみ）、後発マイグレーション 00008〜00010 でも座標は追加されていない。
- `device-location-select/design.md` の Non-Goals に「緯度経度・字・番地」が**明示的に対象外**と書かれている（=意図的に座標を持っていない設計）。

### 含意
推奨API（Open-Meteo / 気象庁いずれも）はピンポイント取得に緯度経度を使う。よって、locality を天気取得キーにするには次のいずれかが要る:

| 方式 | 内容 | コスト | 精度 |
|---|---|---|---|
| **A. 53地域の代表座標表(コード定数 or マスタ)** | locality → (lat, lon) を1表で持つ | 一次データ作成 **約半日**（市役所/役場所在地 or 圃場中心の代表点） | 市町村中心の代表点なので圃場ピンポイントではない |
| B. デバイスに lat/lon 列追加 | デバイス登録時に座標入力 | UI追加が必要・既存方針(Non-Goal)に反する | 圃場ピンポイント可 |
| C. 気象庁予報区/AMeDAS番号への対応表 | locality → 予報区コード(471000等) or 観測所番号 | 4予報区+約26観測所のマッピング作成 | 予報区/観測所単位（粗い） |

**推奨は A（53地域の代表座標表）**。理由:
- 既存の「locality 単一select」設計を壊さず、座標入力UIも不要。
- 53件は有限・固定であり、`localityTable`（`locality.go` L88〜）と同じく**Go定数の単一ソース表**として持てば既存コーディング規約（イミュータブル・ドメイン集約）に合致。
- 旧町村17件は親市町村（うるま市等）の代表点で代用しても天気粒度では実害が小さい（要ユーザー確認だが、5km格子のMSMでは同一格子に落ちる地域も多い）。

**未確認**: 代表点を「市役所/役場所在地」にするか「主要農業地域の中心」にするかは方針未定。Open-Meteoは標高ダウンスケーリングするため標高差の大きい地域では代表点選定が露点/気温に効く可能性があるが、影響度の定量は未確認。

> 補足: 53件の正確な緯度経度の一次データは本調査では未取得（=「未確認」）。作成自体は公開情報で可能だが、値は実装フェーズで一次ソース確認のうえ確定すること。

---

## 3. 推奨API

### 推奨: **Open-Meteo（JMAモデル + Historical/ERA5）**

根拠（4観点）:
- **沖縄離島カバレッジ**: 緯度経度で任意地点を取得（観測所リスト不要）。気象庁 JMA の MSM(0.05°≒5km) / GSM を直接配信し、本島・石垣・宮古・与那国・大東・多良間・久米島まで格子補間で引ける。圃場ピンポイント志向の本案に最も合う。
- **履歴/予報**: 予報は MSM 4日 + GSM 11日。過去は Historical Weather API(ERA5) で1940年〜、JMA MSM履歴は2016年〜。GDD/季節トレンド等の既存分析ロードマップ(P7/P8)とも親和。
- **変数網羅**: 気温/相対湿度/露点/降水/**日射(shortwave/direct/diffuse)**/風/気圧/雲量/ET0。VPD・露点病害・GDD・日射分析（go_iotの本命派生指標）に直結。
- **無料枠**: 非商用はAPIキー不要・登録不要で 1万call/日。PoC・社内検証は即着手できる。

却下しきれない**最大の注意**: 無料ホスト枠は**非商用限定**。go_iot を顧客向け商用サービスとして配信するなら有料プラン（Standard $29/月=1Mcall、Professional $99/月=5Mcall、`customer-api.open-meteo.com` + APIキー + 99.9% SLA目標）が必要。データ自体は CC-BY 4.0 で帰属表示すれば商用可だが、**ホスト枠の利用条件とデータライセンスは別レイヤ**である点に注意（§7）。

### 次点: **気象庁 bosai JSON（AMeDAS観測値 + 予報）二段構え**
- 長所: 公式一次情報・無料・キー不要・**商用可（公共データ利用規約・出典明示必須）**。AMeDASは10分粒度の実測値で沖縄離島の四要素観測所（南大東92011・北大東92006・久米島91146・下地島93012・西表島94062・与那国94011 等）をカバー。
- 短所: (1) 公式「API」ではなく防災サイト内部ファイルの非公式流用で**SLA・仕様安定の保証なし**（予告なき変更リスク）。(2) **湿度が沖縄離島の四要素観測所で基本非観測** → VPD/露点/THI計算に致命的な穴。(3) 予報JSONは代表都市で離島ピンポイントでない。(4) locality→予報区/観測所番号の対応表が別途必要。
- 位置づけ: 「実測の出典明示で信頼性を出したい」要件が立てば**Open-Meteo（離島ピンポイント・湿度・日射・長期過去）を主、AMeDAS実測を補完**の二段が理想。ただし二段は実装が増えるので初手は単一推奨。

### 却下/フォールバック
- **OpenWeatherMap One Call 3.0**: 無料でもクレカ登録+従量課金前提でコスト予測が難しい。JMAモデル非配信で沖縄優位性が弱い。日射変数も劣る。→ fallback。
- **WeatherAPI.com**: 無料枠10万/月と寛容で商用可だが、**無料は履歴1日のみ・予報3日・日射が乏しい**。go_iotの過去/長期/日射分析に不足。→ fallback。
- **Ambient**: 気象官署データでなく自設置センサーの蓄積で、go_iot本体と機能重複。広域の「地域の天気」用途には不適（引継ぎ案件として既知だが別物）。

---

## 4. 取得頻度と定期実行

### 既存基盤（事実）
- アプリ内に **time.Ticker / scheduler / cron は存在しない**（`cmd/sensor-sim` の `time.After` を除く）。アラート判定は受信時の同期実行のみ。
- `cmd/server/main.go` は `context.WithCancel` による graceful shutdown 基盤（signal.Notify + Shutdown）は整備済み。バックグラウンド goroutine は ListenAndServe の待機用1本のみ。
- 本番は AWS Lightsail 上の **systemd サービス `go_iot`** で管理。crontab・CloudWatch等の外部スケジューラは未使用。

### 周期の妥当性
- センサーは5分間隔（既存）。**天気をセンサーと同じ5分にする必要はない**。気象モデルの更新粒度は MSM が1時間値、AMeDASが10分。天気は外部の予報/再解析であり、5分でポーリングしても新しい値は来ず無駄call。
- **推奨: 現在天気/直近実況は 30分〜60分間隔、予報は1日2〜3回（JMA予報更新の05/11/17時頃に合わせる）**。Open-Meteo無料枠（1万call/日）に対し、53地域×毎時=約1,272call/日 で十分余裕。30分なら約2,544call/日でも収まる。
- 重複取得回避と合わせると（§5の locality 単位）、53地域分だけ引けばよく call 数はさらに抑えられる。

### 実装方式の選択
| 方式 | 長所 | 短所 | 推奨度 |
|---|---|---|---|
| **アプリ内 goroutine + time.Ticker + rootCtx子context** | 既存 graceful shutdown 基盤にそのまま乗る・追加インフラ不要・デプロイ単純 | サーバ多重起動時に重複取得（現状は単一インスタンスなので問題小） | **◎ 初手はこれ** |
| systemd-timer + 別バイナリ(cmd/fetch-weather) | プロセス分離・cron的に明快 | サービスユニット追加・本番に新ファイル配置（現状ユニット定義はリポジトリ外） | ○ |
| CloudWatch Events / SSM | マネージド | IAM/外部統合が新規・運用複雑 | △（現状オーバースペック） |

初手は**アプリ内 Ticker**を推奨。`run()` 内で `rootCtx` の子contextを渡した goroutine を1本起動し、`Ticker.C` で取得→DB保存、shutdown 信号で `Ticker.Stop()`+context cancel する標準パターン（コードベース調査も同パターンを推奨）。

---

## 5. DBスキーマ案（weather_readings 仮）

### 設計判断: **locality 単位で持つ（device 単位にしない）**
- 理由: 同一地域に複数デバイスがあっても天気は同じ。device単位だと重複取得・重複保存になる。「locality + 観測/予報時刻」でユニークにすれば call も行数も最小化でき、§4の重複回避と整合。
- デバイスからの参照は `devices.locality` 経由で結合（外部キーは張らない=既存方針）。

### カラム案
```sql
-- +goose Up
CREATE TABLE weather_readings (
    id            BIGSERIAL     PRIMARY KEY,
    locality      VARCHAR(64)   NOT NULL,          -- domain.Locality の string 値（例「那覇市」）。FKは張らない
    observed_at   TIMESTAMPTZ   NOT NULL,          -- 天気の対象時刻（実況/予報の対象時刻）
    source        VARCHAR(32)   NOT NULL,          -- 'open-meteo' 等のデータ源（出典明示・将来の二段構え用）
    kind          VARCHAR(16)   NOT NULL,          -- 'observation' | 'forecast'（実況か予報か）
    temperature   NUMERIC(5,2),                    -- ℃（CHECK -40〜125）。nullable=欠測許容
    humidity      NUMERIC(5,2),                    -- %（CHECK 0〜100）
    dew_point     NUMERIC(5,2),                    -- 露点℃（VPD/病害用）
    precipitation NUMERIC(7,2),                    -- 降水 mm
    wind_speed    NUMERIC(5,2),                    -- m/s
    shortwave_radiation NUMERIC(7,2),              -- 日射 W/m^2（GDD/日射分析用）
    weather_code  INTEGER,                         -- 天気コード
    fetched_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(), -- サーバ取得時刻
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    CONSTRAINT weather_readings_temp_range CHECK (temperature IS NULL OR temperature BETWEEN -40 AND 125),
    CONSTRAINT weather_readings_humidity_range CHECK (humidity IS NULL OR humidity BETWEEN 0 AND 100)
);

-- 重複取得回避: 同一地域・同一対象時刻・同一種別・同一源で一意
CREATE UNIQUE INDEX weather_readings_uniq
    ON weather_readings(locality, observed_at, kind, source) WHERE deleted_at IS NULL;

-- 地域別・時系列の取得高速化（sensor_readings と同パターン）
CREATE INDEX weather_readings_locality_observed_at_idx
    ON weather_readings(locality, observed_at DESC) WHERE deleted_at IS NULL;

COMMENT ON TABLE weather_readings IS '気象API(Open-Meteo等)から取得した地域(locality)単位の天気データ';
-- +goose Down
DROP TABLE IF EXISTS weather_readings;
```

### 既存方針との整合（確認済み）
- **外部キー無し**: `sensor_readings.device_id` がFKを張らない設計（00004）と同じく、`locality` にもFKを張らない（domain側Enumで整合性を担保）。
- **論理削除 + 部分インデックス**: `deleted_at` + `WHERE deleted_at IS NULL` の partial index は既存テーブル全てと同形。
- **expand-contract**: 初回は CREATE TABLE のみで破壊的変更ゼロ → redeploy.sh の「入替前 goose up」ウィンドウで無害（旧バイナリは新テーブルを触らない）。マイグレーション1個で完結。
- **NUMERIC(5,2) nullable**: 天気は欠測がありうるため数値列を nullable にし、CHECKは `IS NULL OR …` で欠測を許容（sensor_readings は NOT NULL だが、天気は欠測前提なので意図的に緩める）。

UPSERT は `CreateWeatherReading` を `INSERT … ON CONFLICT (locality, observed_at, kind, source) DO UPDATE` にすれば、予報の上書き取得（同時刻の予報が更新される）に対応できる。

**注意（メモリ参照）**: 既存の `pgtype.Numeric` ↔ float 往復の精度劣化回避ルール（`pgconv` 利用・計算系列は表示前2桁丸め）が天気の集計表示でも適用対象になる。

---

## 6. 既存アーキへの統合点（作業リスト）

| # | 領域 | ファイル | 作業 | 規模 |
|---|---|---|---|---|
| 1 | domain | `internal/domain/locality.go`（or 新 `locality_coords.go`） | 53地域→(lat,lon) 代表座標表を定数で追加。`Coords(l Locality) (float64,float64,bool)` を追加 | M（データ作成） |
| 2 | migration | `db/migrations/00011_create_weather_readings.sql` | §5 の CREATE TABLE | S |
| 3 | queries | `db/queries/weather_readings.sql` | UpsertWeatherReading / GetLatestByLocality / ListByLocalityInRange の3〜4本 | S |
| 4 | sqlc生成 | `make sqlc` → `internal/repository/weather_readings.sql.go` ほか | 自動生成（models.go / querier.go に自動追記） | S（自動） |
| 5 | config | `internal/config/config.go` / `.env.example` | `WeatherAPIBaseURL`（必要なら `WeatherAPIKey`）を `getEnv` で追加。非商用Open-Meteoはキー不要なのでURLのみでも可 | S |
| 6 | service | `internal/service/weather_fetcher.go`（新規） | 外部HTTP取得→正規化→Upsert。`cmd/sensor-sim` の `send()`（http.Client+context+timeout+LimitReader）を参考。consumer interface `WeatherFetcherRepo` を定義しDI | M |
| 7 | scheduler | `cmd/server/main.go` `run()` | rootCtx子contextで goroutine+Ticker を起動、shutdownでStop（§4） | S |
| 8 | (任意)handler | `internal/handler/weather_*.go` | 画面/JSON表示が要るなら。表示は別フェーズでも可 | 任意 |

- repository は **consumer最小interface + DI** の既存流儀（`SensorRepo` / `AlertEvaluatorRepo`）に倣う。`*repository.Queries` が全メソッドを実装するので interface 定義だけで疎結合。
- 認可: weather_readings は **locality 単位の共有データ**でデバイス所有者に紐づかない。取得ジョブはサーバ内部処理なので `authz.RequireDeviceOwner` 不要。ただし**画面で天気を出す際は、ユーザーが見られるのは自分のデバイスの locality の天気のみ**となるよう、表示クエリ側でデバイス所有チェックを通すこと（BOLA注意点）。
- マイグレーションを足したら **`make db-snapshot` で `docs/database_snapshot/` 再生成**（CLAUDE.md 規約）。

---

## 7. リスク・注意点

- **商用ライセンス（最重要・要判断）**: Open-Meteo無料枠は非商用限定。go_iot が有償の顧客サービスなら有料プラン必須（$29〜/月）。データはCC-BY 4.0で**帰属表示が必須**（例: 「Weather data by Open-Meteo.com (CC BY 4.0)」+ 元データ気象庁併記推奨）。気象庁bosai/AMeDAS採用時は「出典:気象庁ホームページ(URL)」+加工明記が必須。**画面フッタ等に出典表示UIを用意する作業が別途要る**（未設計）。
- **API障害時のリトライ**: 外部依存なので5xx/タイムアウトは起こる。指数バックオフ+上限リトライ、失敗時はログ（slog）して**次のTickで再試行**（天気は数十分遅延しても致命的でない）。`sensor-sim` の `errors.Is` ステータス分岐パターンを流用。
- **レート制限**: Open-Meteo無料は600call/分・1万call/日。53地域を毎時でも余裕だが、起動直後に53件を同時バーストさせない（地域ループに小スリープ or バッチ複数座標カンマ区切り1リクエスト）。商用移行時は日次制限が外れる。
- **過去欠測・データ品質**: 天気は欠測あり（§5でnullable化済み）。ERA5は気候トレンド向きで日々精度はHistorical Forecastに劣る。current値はモデルのシミュレーション値で**現地観測の生値ではない**。実測重視要件があればAMeDAS二段構えを再検討。
- **座標精度**: 代表点は市町村中心であり圃場ピンポイントではない。標高差の大きい地域（やんばる等）で気温/露点に差が出うる（定量は未確認）。圃場精度が要るなら方式B（デバイスlat/lon）へ。
- **コスト増**: 商用プラン月額 + （二段構えなら）保守対象API増。AMeDAS非公式流用は仕様変更で**ある日壊れる**運用リスクをコストとして織り込む。
- **重複・多重起動**: アプリ内Tickerはサーバ複数台化すると重複取得。現状は単一Lightsailインスタンスなので問題ないが、スケールアウト時はUPSERT(§5)とロック or 外部cron化が必要。

---

## 8. この後の進め方の選択肢

- **(a) このまま `future/{feature}` ブランチで cc-sdd spec化** — feature名は `weather-fetch`（仮）。1フェーズ=1spec規約に沿い `/kiro-spec-init`→requirements→design→tasks。spec化前に**§2の代表座標方式と§3のAPI（=商用かどうか）を確定**しておくのが望ましい（design分岐の根）。
- **(b) PoCスクリプトで1地点取得を試す（推奨初手）** — `cmd/poc-weather`（or 使い捨てスクリプト）で那覇1地点を Open-Meteo（キー不要）から取得し、(1)沖縄座標で期待変数が返るか、(2)離島（与那国/南大東）でMSM格子に乗るか、(3)レスポンス形式とJSONマッピングを実地確認。**半日・無料・無リスク**で§2/§3の前提を検証でき、spec化の精度が上がる。
- **(c) ユーザー確認待ち事項** — 以下は本調査で**未確認**、判断が要る:
  1. **商用かどうか**: go_iot は最終的に有償の顧客サービスか（→Open-Meteo有料/気象庁bosai/別API の分岐）。
  2. **この案自体がユーザー（眞境名さん）の確定依頼か、スタンプ側仮説か**。メモリ上、確定依頼は引継ぎメモ（Ambient代替+24h温湿度グラフ）のみで、ロードマップ系はベンダー仮説の可能性が高い。本案も仮説なら「動くデモにして提案」方針でPoC(b)から進めるのが筋。
  3. **天気の用途**: 実況表示だけか／センサー実測との比較・補正に使うか／予報も要るか（→スキーマ kind と取得頻度に影響）。
  4. 代表点の取り方（役場所在地 vs 主要農業地域中心）、旧町村17件を親市町村で代用してよいか。

---

### 参照した主なファイル（いずれも絶対パス）
- `/Users/c/Desktop/dev/go_iot/internal/domain/locality.go`（53地域がstring定数・座標なしを確認）
- `/Users/c/Desktop/dev/go_iot/db/migrations/00004_create_sensor_readings.sql`（FK無し・論理削除・partial index・CHECKの既存パターン）
- `/Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go`（consumer interface + DI + authz の流儀）
- `/Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go`（service層パターン）
- `/Users/c/Desktop/dev/go_iot/cmd/sensor-sim/main.go`（外部HTTP `send()` の流用元）
- `/Users/c/Desktop/dev/go_iot/internal/config/config.go`（env追加パターン）
- `/Users/c/Desktop/dev/go_iot/cmd/server/main.go`（graceful shutdown基盤・Ticker追加点）
- `/Users/c/Desktop/dev/go_iot/deploy/redeploy.sh`（expand-contract / 入替前goose up）
- `/Users/c/Desktop/dev/go_iot/.kiro/specs/device-location-select/design.md`（座標がNon-Goalである根拠）

**誇張なしの要点**: 技術的には既存パターンの複写で「実現可能・難易度M」。ただし(1)53地域の座標表作成、(2)定期実行基盤の新規追加、(3)商用ライセンス判断、(4)この案がユーザー確定依頼かの確認、の4点が前提であり、特に(3)(4)は本調査では**未確認**。初手はリスクゼロのPoC(b)を推奨する。

---

# 第2部 コードベース調査（詳細）

> 3つのサブエージェントが go_iot のコードを実読し、(1)地域モデルと座標の有無 (2)定期実行基盤 (3)既存アーキへの統合点 を調べた結果の全文。

### 2-1 地域(Locality)モデルと座標の有無

Go IoTプロジェクト（農業IoT・Gin+templ+HTMX+PostgreSQL+sqlc）のコードベース調査から、「センサーに登録した地域の天気をAPI取得しDB保存する」案の実現性を【地域(Locality)モデルと座標の有無】の観点で調査完了。主な発見：(1) Locality モデルは完全に実装済み：沖縄53地域(未合併36市町村+平成合併5市町村の旧町村17)がGo定数として定義済み、Municipality()で親市町村導出可、(2) 座標(lat/lon)はコード内に存在しない：devices テーブルに location(フリーテキスト)と locality(構造化キー)のみ、(3) 設計意図として座標は Non-Goal に明記、(4) 天気API取得機能は未実装。

#### Locality モデルの定義と Municipality() メソッド

```text
ファイル: /Users/c/Desktop/dev/go_iot/internal/domain/locality.go
型定義: type Locality string、53個の const(未合併36+旧町村17)
主要メソッド:
- Label(): 画面表示用(未合併は市町村名そのもの、旧町村は"短縮名(現市町村)")
- Municipality(): 親市町村を返す(line 214-216)。実装は localityParent[l] でO(1)導出
- Valid(): Enum検証(line 219-222)
- ParseLocality(s string): 正式名/現市町村名/短縮名のエイリアス解決(line 228-233)
- AllLocalities(): 53地域を列挙(line 236-242)
対応テーブル: localityTable(line 88-153)に地域→親市町村の対応を一元化。localityParent(line 156-162)マップで高速導出
```

**実現性への影響:** 【Locality側の実現性】OK。地域マスタは完全に構造化済みで、localityを基準に集計・フィルタ可能。Municipality()で親市町村も導出できるため、地域別集計の前提条件は満たしている。

#### 座標(緯度経度)のコード内での有無

```text
結論: 座標は現在コード内に存在しない。
devicesスキーマ(/Users/c/Desktop/dev/go_iot/db/migrations/00002_create_devices.sql)フィールド: id, user_id, name, mac_address, location(VARCHAR255), is_active, last_communicated_at, created_at, updated_at, deleted_at。lat/lon列なし。
RepositoryDevice型(/Users/c/Desktop/dev/go_iot/internal/repository/models.go, line 48-67): ID, UserID, Name, MacAddress, Location(*string), IsActive, LastCommunicatedAt, CreatedAt, UpdatedAt, DeletedAt, Locality(*string), Crop(*string), PlantingDate(pgtype.Date)。緯度経度フィールドなし。
後発の拡張マイグレーション00008-00010にも座標追加なし。
```

**実現性への影響:** 【座標の有無】NG。天気APIは座標で引くのが基本だが、devices テーブルに lat/lon列がないため、座標を別途追加するか Locality→座標の対応テーブルを作成する必要がある。

#### 設計の Non-Goal に座標が明示的に除外

```text
ドキュメント: /Users/c/Desktop/dev/go_iot/.kiro/specs/device-location-select/design.md, Non-Goals セクション(line 20)
記述: "緯度経度・字・番地、都道府県マスタ、自由記述補足欄、字レベルの細粒度。"
理由: P1(device-location-select)は平坦な単一地域select の実装に特化。座標は当スペックの対象外と明記されている。
```

**実現性への影響:** 【設計意図】座標取得は現在の実装計画(P1-P15)に含まれていない。追加実装するには design 見直しとスペック新設が必要。

#### locality と AMeDAS観測所の対応

```text
対応関係なし。コード grep で "AMeDAS", "観測所", "アメダス" を検索した結果、実装的な対応テーブルは存在しない。
分析アイデアメモ(line 207)でアメダス平年値の外部基準利用について記載があるが、これは地域→観測所のマッピングではなく、平年値の外部参照のみ。
```

**実現性への影響:** 【AMeDAS対応】なし。locality(53地域) をAMeDAS観測所へ対応付けるためには、別途「地域→観測所 ID」のマッピング表またはJSONを作成する必要がある(沖縄のAMeDAS観測所は限定的のため実現可能性は高い)。

#### 天気API取得機能の実装状況

```text
実装なし。実装計画(実装計画.md)に P1-P15 の spec がある が、いずれも外部気象データ自動取得は記載されていない。
P7(gdd-forecast)は GDD の積算計算だが、外部気象データ取得ではなく、devices に登録されたセンサーデータ(temperature)を利用。
P8(seasonal-trend)は長期トレンド検定で気象庁アメダス平年値を外部基準として参照するデザイン(分析アイデアメモ G-9)だが、自動取得 API 実装ではなく、データ品質メタの記述統計範囲。
```

**実現性への影響:** 【天気API取得】未実装。実現には以下の新規実装が必須: (1) Locality→座標(lat,lon)の対応、(2) 外部気象API(気象庁/OpenWeather等)の選定とHTTPクライアント実装、(3) 天気データの永続スキーマ(weather_readingsテーブル等)、(4) バックグラウンドジョブ(scheduler)による定期取得。

#### devices.location (フリーテキスト) の使われ方

```text
現在の使用: デバイス登録時にユーザが自由に入力する所在地メモ(VARCHAR255)。
Backfill処理: locationbackfill パッケージ(/Users/c/Desktop/dev/go_iot/internal/locationbackfill/backfill.go)が既存 location テキストを domain.ParseLocality() で解決し、新設 devices.locality 列へ非破壊移行する仕組み(cmd/migrate-locations/main.go)。ParseLocality の aliasMap(locality.go line 164-186)で正式名/短縮名/現市町村名のエイリアス解決により既存テキストを構造化キーへマッピング可能。
```

**実現性への影響:** 【location活用】既存の自由入力テキストが部分的に locality へ移行可能。ただし同名曖昧(具志川市/具志川村)や合併後市町村名(うるま市等)は解決不可のため、全migration成功率は100%ではない。新規取得には位置情報入力UI(郵便番号/住所検索)の追加設計が別途必要。

**参照ファイル:**

- `/Users/c/Desktop/dev/go_iot/internal/domain/locality.go`
- `/Users/c/Desktop/dev/go_iot/internal/domain/municipality.go`
- `/Users/c/Desktop/dev/go_iot/internal/repository/models.go`
- `/Users/c/Desktop/dev/go_iot/db/migrations/00002_create_devices.sql`
- `/Users/c/Desktop/dev/go_iot/db/migrations/00008_add_locality_to_devices.sql`
- `/Users/c/Desktop/dev/go_iot/db/queries/devices.sql`
- `/Users/c/Desktop/dev/go_iot/internal/locationbackfill/backfill.go`
- `/Users/c/Desktop/dev/go_iot/cmd/migrate-locations/main.go`
- `/Users/c/Desktop/dev/go_iot/.kiro/specs/device-location-select/design.md`
- `/Users/c/Desktop/dev/go_iot/.kiro/specs/gdd-forecast/research.md`
- `/Users/c/Desktop/dev/go_iot/2cc_sdd/実装計画.md`

---

### 2-2 定期実行(スケジューラ/バックグラウンドジョブ)基盤

Go IoTプロジェクト(農業IoT・Gin+templ+HTMX+PostgreSQL+sqlc)のコードベースを調査。「センサー地域の天気を定期的にAPI取得しDB保存する」ために必要な【定期実行(スケジューラ/バックグラウンドジョブ)基盤】の有無を確認した。

結果: **アプリケーション内に定期実行基盤は存在しない**。アラート判定は「受信時に同期実行」、graceful shutdown は「HTTP Serverの標準パターン」のみで、ticker/scheduler/cron機構がない。AWS Lightsail本番運用ではsystemdで管理され、外部cronやIAMベースのスケジューラ統合は未実装。新規にバックグラウンドジョブ基盤の追加が必要。

#### 1. アラート判定の定期実行方式

```text
/Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go（全123行）のEvaluateAndNotify()メソッドは、受信時の同期実行のみ。/Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go第128〜131行で「アラート判定を同期実行する」と明記。定期実行の仕組みなし。
```

**実現性への影響:** 天気APIを定期取得するには、新規にgoroutine+time.Tickerか外部スケジューラが必須。現在のアラート判定も「受信トリガ」なので、天気データは別のバックグラウンドジョブで独立駆動する必要がある。

#### 2. アプリ内周期実行の仕組み

```text
/Users/c/Desktop/dev/go_iot全体をgrepで確認した結果、time.Ticker/time.Timer/scheduler/cronを使う実装は存在しない（cmd/sensor-sim/main.goでのtime.After例外を除く）。/Users/c/Desktop/dev/go_iot/cmd/server/main.go第27〜89行の起動フロー（main()→run()）ではhttpサーバのListen&Serveのみ。バックグラウンドgoroutineループなし。
```

**実現性への影響:** 定期実行基盤が完全に欠けている。天気API取得の定期ジョブを実装するには「goroutine + time.Ticker + context.Context」パターン、または「AWS CloudWatch Events + Lambda」等の外部スケジューラの統合が必須。

#### 3. AWS本番(Lightsail)での運用形態

```text
/Users/c/Desktop/dev/go_iot/deploy/redeploy.sh行39で「SERVICE=go_iot」と指定され、systemd "go_iot" サービス経由で管理。/Users/c/Desktop/dev/go_iot/deploy/cloud-init.sh第194〜195行で「systemctl enable caddy; systemctl restart caddy」。本番はsystemd管理のサービスユニット前提。サービスファイルの定義は/Users/c/Desktop/dev/go_iot内に無く、Lightsail本番インスタンス上で別途配置と推定（セッション期間の初回実行時に自動生成される可能性）。
```

**実現性への影響:** systemd のみで管理されており、crontab等の外部cron設定は現在未使用。バックグラウンドジョブを追加する場合、「アプリ内goroutineで実装する」か「systemd-timer + 別ユーティリティバイナリ」か「外部IAMスケジューラ(CloudWatch/SSM)」を選択する必要。

#### 4. graceful shutdown とcontext伝播パターン

```text
/Users/c/Desktop/dev/go_iot/cmd/server/main.go第39行で「rootCtx, rootCancel := context.WithCancel(context.Background())」。第82〜86行で「shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)」→ srv.Shutdown(shutdownCtx)。httpサーバの標準的なGraceful Shutdownパターン（signal待機→Shutdown()で進行中リクエストの完了を待つ）を実装。DBコネクション(pool)はdefer pool.Close()で確実に閉じる。ただし、バックグラウンドgoroutineはない。
```

**実現性への影響:** graceful shutdownの基盤は整備済み(signal.Notify + context timeout)。天気APIの定期ジョブを追加する場合、「rootCtxの子contextを別goroutineに渡す」→「終了信号でcontextをcancel」→「Tickerをstopしてgoroutineを終了」する標準パターンを採用すればよい。

#### 5. 既存バックグラウンド処理

```text
cmd/server/main.go第65〜70行で「go func() { ...srv.ListenAndServe() }()」が唯一の非同期実行。これはHTTPサーバ起動の待機goroutineのみ。それ以外のバックグラウンドタスク(Ticker/worker/queue等)はない。
```

**実現性への影響:** バックグラウンドジョブは全く新規実装。天気データ定期取得の場合、main()起動時にゴルーチンと外部context通知を新規追加する必要。

**参照ファイル:**

- `/Users/c/Desktop/dev/go_iot/cmd/server/main.go`
- `/Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go`
- `/Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go`
- `/Users/c/Desktop/dev/go_iot/deploy/redeploy.sh`
- `/Users/c/Desktop/dev/go_iot/deploy/cloud-init.sh`
- `/Users/c/Desktop/dev/go_iot/deploy/ssm-setup-plan.md`
- `/Users/c/Desktop/dev/go_iot/2cc_sdd/実装計画.md`

---

### 2-3 既存アーキへの統合点・配線

Goプロジェクト go_iot (農業IoT: Gin+templ+HTMX+PostgreSQL+sqlc, DIP配線, レイヤードlite) のコードベース調査完了。「天気API取得→DB保存」統合の実装パスを洗い出した。

【調査範囲】
- 外部HTTP呼び出し: 検出1箇所 (sensor-sim)、実装パターンあり
- 設定管理: internal/config で環境変数管理、拡張パターン明確
- DB マイグレーション: goose + sqlc 生成、expand-contract 方針確認、redeploy.sh での migration 前置・冪等保証
- リポジトリ/サービス: DIP 配線完成 (consumer interface + sentinel error 処理)
- sensor_readings 実例: 10 クエリ、複合インデックス、論理削除対応、集計複数経路
- 所有者認可: authz パッケージで BOLA 防止、ハンドラへの混在なし

【新テーブル (weather_readings) 追加の難易度評価】
- 既存パターンの完全コピーで、15〜20分での追加可能（学習曲線なし、テンプレート通り）。

#### 外部HTTP呼び出し（net/http client）

```text
プロジェクト内に既存の外部API呼び出し実装1箇所。
ファイル: /Users/c/Desktop/dev/go_iot/cmd/sensor-sim/main.go
関数: send() (L118)
パターン: http.Client{Timeout:10*time.Second} → http.NewRequestWithContext(ctx, http.MethodPost, url, ...) → req.Header.Set('Authorization', 'Bearer '+token) → client.Do(req)。
認証: Bearer Token (デバイスAPI用)、Content-Type: application/json
エラーハンドリング: io.LimitReader(4096) でレスポンス本文制限、errors.Is で分岐（400/401/403/422/500を区別）
→ 新テーブル: weather_readings への API 呼び出しは同パターンで設計可能。
```

**実現性への影響:** 天気API統合の外部呼び出し部分は既存パターン (sensor-sim の send 関数) を参考にコピー可能。HTTP client, context, timeout, error handling が既に実装済みのため、weather API 側の URL/ヘッダ/レスポンス形式だけ変更する low-risk な追加。

#### 設定管理・環境変数

```text
ファイル: /Users/c/Desktop/dev/go_iot/internal/config/config.go
Struct: Config{AppEnv, AppPort, DatabaseURL, SessionSecret}
読み込み関数: Load() (L22〜46) で os.Getenv + getEnv/getEnvInt ヘルパで環境変数取得。
バリデーション: 必須チェック (DatabaseURL, SessionSecret)、production で SessionSecret 最小32文字強制
パターン:
  - 必須: fmt.Errorf("required env vars missing: %v") で複数missing を列挙
  - オプション: getEnv(key, fallback) で default 値設定 (AppEnv=development, AppPort=8080)
Makefile: include .env + export により事前に .env 読み込み (Makefile L4〜7)
.env.example: L12 に DEVICE_TOKEN_SECRET の例あり（将来拡張用コメント）
→ weather API key を追加する場合: config.go に WEATHER_API_KEY string フィールド + getEnv("WEATHER_API_KEY", "") + missing チェック、.env.example に記入。
```

**実現性への影響:** 新しい設定項目 (e.g., WEATHER_API_KEY, WEATHER_API_BASE_URL) の追加は internal/config/config.go の Config struct へ 2行追加 + Load() の getEnv 呼び出し + missing 配列検査で完結。既存パターンで拡張性高し。

#### DB マイグレーション (新テーブル追加パターン)

```text
ツール: goose (migrate tool)
配置: db/migrations/ に ++goose Up/Down SQL
命名: 00001_*, 00002_*, ..., 00010_add_planting_date_to_devices.sql
パターン例:
  ファイル: /Users/c/Desktop/dev/go_iot/db/migrations/00004_create_sensor_readings.sql
  - CREATE TABLE + CHECK 制約 + 複合INDEX (device_id, recorded_at DESC) + COMMENT
  - 論理削除: deleted_at TIMESTAMPTZ + WHERE deleted_at IS NULL インデックス (partial index)
  - PRIMARY KEY BIGSERIAL + created_at/updated_at DEFAULT NOW()
  - goose Down: DROP TABLE IF EXISTS
expand-contract 方針 (redeploy.sh L152〜175 に記載):
  - migration は入替の「前」に goose up する (旧バイナリ×新スキーマ で数秒動作)
  - ADD COLUMN(nullable), ADD INDEX, ADD TABLE は旧バイナリに無害 (冪等・no-op if already applied)
  - DROP/RENAME/型変更/NOT NULL追加 は破壊的→複数回migration で分割
  - goose 失敗時: バイナリ入替えず中止 (prod は旧コード×直前スキーマで継続)
→ weather_readings テーブル追加: 00011_create_weather_readings.sql で CREATE TABLE (nullable columns, indexes, comments, logical delete) を定義。
```

**実現性への影響:** 新テーブル追加は CREATE TABLE 一行で十分（破壊的変更がないため expand-contract 複数段階不要）。既存 sensor_readings パターンを複写・フィールド名変更するだけで migration 完成。goose は冪等なので deploy 重複実行・ロールバック・リトライも安全。

#### sqlc クエリ + リポジトリ生成

```text
ツール: sqlc v1.30.0
ファイル: sqlc.yaml (L1〜15)
設定: 
  engine: postgresql
  queries: db/queries/
  schema: db/migrations/
  gen.go: package=repository, out=internal/repository, sql_package=pgx/v5, emit_json_tags=true, emit_pointers_for_null_types=true, emit_interface=true
クエリ配置: db/queries/sensor_readings.sql (L1〜109)
例: sensor_readings.sql 内 10 クエリ
  - CreateSensorReading :one (INSERT RETURNING)
  - GetLatestSensorReading :one (ORDER BY DESC LIMIT 1)
  - ListLatestSensorReadings :many (LIMIT 10)
  - ListRecentSensorReadings :many (期間+昇順・24h グラフ用)
  - ListDailySensorAggregates :many (日別集計・AVG/MAX/MIN・UTC 暦)
  - ListDailySensorAggregatesJST :many (JST 暦版・同)
  - GetSensorReadingsSummary :one (集計BOX用)
  - ListSensorReadingsPaginated :many (履歴画面・LIMIT/OFFSET)
  - CountSensorReadingsInRange :one (ページング用件数)
  - ListSensorReadingsInRange :many (CSV エクスポート・昇順・LIMIT なし)
生成: make sqlc → internal/repository/sensor_readings.sql.go (L1〜) に Queries.CreateSensorReading() 等メソッド自動生成
Querier interface: internal/repository/querier.go (L11〜78) に全メソッドシグネチャ
→ weather_readings.sql に 3〜5 基本クエリ (Create, GetLatest, ListByDevice, Delete) を定義 → make sqlc で自動生成。
```

**実現性への影響:** 新テーブルのクエリは db/queries/weather_readings.sql に SQL を記述するだけで、sqlc generate が internal/repository/weather_readings.sql.go を自動生成。手作業コード生成なし・型安全・JSON タグ自動付与。Makefile の make sqlc one-shot で完結。

#### リポジトリ層 (DIP 配線)

```text
ファイル: internal/repository/
構成:
  - db.go (L1〜32): DBTX interface (Exec/Query/QueryRow)、func New(db DBTX) *Queries、WithTx(pgx.Tx) 再配線
  - models.go (L1〜119): 型定義 (Device, SensorReading, AlertRule, AlertHistory, User, DeviceToken, Session)
  - querier.go (L11〜78): Querier interface (全メソッドシグネチャ・生成)
  - sensor_readings.sql.go (生成・L1〜): CreateSensorReading/GetLatestSensorReading/... メソッド実装
  - (同) alert_*.sql.go, devices.sql.go, users.sql.go 等
DIP のポイント:
  - ハンドラは Querier interface を DI（全メソッド不要・consumer 最小 interface を定義）
  例: /Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go L22〜26
    type SensorRepo interface {
      authz.DeviceGetter // GetDevice(ctx, id) (Device, error)
      CreateSensorReading(...) (SensorReading, error)
      UpdateDeviceLastCommunicated(ctx context.Context, id int64) error
    }
  - サービス層も同様: /Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go L17〜20
    type AlertEvaluatorRepo interface {
      ListEnabledAlertRulesByDevice(ctx context.Context, deviceID int64) ([]AlertRule, error)
      CreateAlertHistory(ctx context.Context, arg CreateAlertHistoryParams) (AlertHistory, error)
    }
  - cmd/server/main.go L48, L126 で *repository.Queries を DI
→ weather_readings 用: type WeatherRepo interface { CreateWeatherReading(...), GetLatestByDevice(...) } と定義し、ハンドラ/サービスで DI。
```

**実現性への影響:** 新テーブルのリポジトリ層は既存パターン (SensorRepo, AlertEvaluatorRepo) を参考に consumer interface を定義・DI するだけ。*repository.Queries は全メソッドを実装しているため、interface 定義だけで疎結合を実現できる。

#### サービス層 (アラート判定の実装例)

```text
ファイル: /Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go
パターン:
  struct: AlertEvaluator { Repo AlertEvaluatorRepo, Logger *slog.Logger }
  メソッド: EvaluateAndNotify(ctx context.Context, reading *SensorReading) ([]AlertHistory, error)
  フロー:
    1. Repo.ListEnabledAlertRulesByDevice で有効ルール取得
    2. ルール各々について、reading の実測値と比較 (Evaluate)
    3. マッチしたら Repo.CreateAlertHistory で履歴化
    4. 返却: 発火済み履歴スライス + err
  エラー方針 (ベストエフォート):
    - ルール取得失敗 → (nil, err) で中止
    - 履歴作成失敗 → 既作成分と err を返して中断 (トランザクション不使用・rolled back なし)
    - 値域外ルール・未知の演算子 → 安全に読み飛ばし継続 (fail-safe)
  ロギング: slog で device_id, rules_evaluated, alerts_fired, error を記録
→ weather_readings は temperature 単一値のみ「アラート判定」は不要の可能性あり。DLP は追加されない想定。
```

**実現性への影響:** weather_readings がセンサー受信と異なりバッチ取得 (API→DB 一括) の場合、AlertEvaluator 相当のサービスは不要。ただし同一パターン (RepositoryRepo interface + EvaluateXXX メソッド) で拡張可能。

#### 認可層 (BOLA 防止・authz パッケージ)

```text
ファイル: /Users/c/Desktop/dev/go_iot/internal/authz/ownership.go
関数:
  - RequireDeviceOwner(ctx, q DeviceGetter, deviceID, userID) (Device, error)
    - userID<=0 で ErrUnauthenticated (fail-closed)
    - device 不在で pgx.ErrNoRows (透過)
    - device.UserID != userID で ErrNotOwner (403 に写す)
  - RequireAlertRuleOwner(ctx, q AlertRuleDeviceGetter, ruleID, userID) (AlertRule, Device, error)
    - rule → device → owner の 2 段判定
ハンドラでの使用例: /Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go L100
  device, err := authz.RequireDeviceOwner(ctx, h.Repo, req.DeviceID, userID)
  if err != nil {
    switch {
    case errors.Is(err, authz.ErrUnauthenticated): c.JSON(401, ...)
    case errors.Is(err, pgx.ErrNoRows): c.JSON(422, ...)
    case errors.Is(err, authz.ErrNotOwner): c.JSON(403, ...)
    ...
→ weather_readings も device 単位のリソースなら RequireDeviceOwner で保護。所有者チェックは authz に集約。
```

**実現性への影響:** 新テーブルが device 属性なら RequireDeviceOwner で認可。ハンドラに所有者チェックを散らさない設計が既存・定着済み。DIP と組み合わせた BOLA 完全防止設計。

#### ハンドラ配線 (DI + ミドルウェア)

```text
ファイル: /Users/c/Desktop/dev/go_iot/cmd/server/main.go L98〜199 (newHTTPHandler 関数)
配線流れ:
  1. config.Load() で設定 (L34)
  2. infradb.NewPool(ctx, cfg.DatabaseURL) で DB コネクションプール (L42)
  3. repository.New(pool) で Queries 初期化 (L48)
  4. auth.NewSessionManager(pool, cfg) で Session 初期化 (L49)
  5. newHTTPHandler で全ルート配線 (L55)
Gin engine 構築:
  - 静的ルート: /health, /docs, /login, /register, /dashboard
  - デバイス API: /api/sensor-data (Bearer auth + AlertEvaluator DI)
    L126: sensorAPI := &handler.SensorAPI{Repo: q, Evaluator: &service.AlertEvaluator{Repo: q}}
  - Web UI: /devices/*, /alerts/*, /analysis/* (Session auth + CSRF + RequireAuth ミドルウェア)
ミドルウェア合成 (L198):
  MethodOverride (hidden _method=put → PUT 昇格) → SessionManager.LoadAndSave → engine
Routing order (Gin 最適化):
  - 静的 /devices/create は 先に解決 (パラメータ :device より優先)
  - パラメータ :device は /edit, PUT, DELETE で共有
  - HTMX と no-JS の両立: _method=hidden + MethodOverride で POST→PUT 変換
→ weather_readings API: デバイスAPI群 (/api/*) に 新ルート /api/weather-data 追加 (sensor-sim と同パターンの Bearer auth)。
```

**実現性への影響:** 新 API エンドポイント追加は cmd/server/main.go の L126 パターンをコピー (handler.WeatherAPI{Repo: q} として DI)。ルーティングと認証は existing structure に組み込むだけ。

#### sensor_readings の実装例（新テーブル設計の参考）

```text
テーブル定義: /Users/c/Desktop/dev/go_iot/db/migrations/00004_create_sensor_readings.sql
列:
  - id BIGSERIAL PRIMARY KEY
  - device_id BIGINT NOT NULL (FK を定義しない・外連「してもいい」が設計決定)
  - temperature NUMERIC(5,2) NOT NULL (CHECK temperature BETWEEN -40 AND 125)
  - humidity NUMERIC(5,2) NOT NULL (CHECK humidity BETWEEN 0 AND 100)
  - recorded_at TIMESTAMPTZ NOT NULL (デバイス側計測時刻)
  - created_at TIMESTAMPTZ DEFAULT NOW() (サーバ受信時刻・遅延計算用)
  - updated_at TIMESTAMPTZ DEFAULT NOW() (監査用)
  - deleted_at TIMESTAMPTZ (論理削除)
インデックス:
  - sensor_readings_device_id_recorded_at_idx (複合・device_id ASC, recorded_at DESC, deleted_at IS NULL)
  - sensor_readings_recorded_at_idx (recorded_at DESC, deleted_at IS NULL)
クエリ (db/queries/sensor_readings.sql L1〜109):
  - CreateSensorReading :one (INSERT)
  - GetLatestSensorReading :one (最新1件・ダッシュボード)
  - ListLatestSensorReadings :many (最新10件・詳細ページ)
  - ListRecentSensorReadings :many (期間+昇順・24h グラフ)
  - ListDailySensorAggregates :many (日別集計・AVG/MAX/MIN・UTC)
  - ListDailySensorAggregatesJST :many (JST 暦版)
  - GetSensorReadingsSummary :one (集計BOX)
  - ListSensorReadingsPaginated :many (履歴テーブル・LIMIT/OFFSET)
  - CountSensorReadingsInRange :one (ページング件数)
  - ListSensorReadingsInRange :many (CSV・昇順)
クエリ特徴:
  - DATE() バケット (集計カラムなし・計算で生成)
  - NUMERIC(5,2) の float 往復による精度劣化回避 (pgconv.NumericToFloat のみ比較時)
  - 複数経路の集計 (24h は日次非集計、3d/7d/30d は ListDailySensorAggregates、長期トレンドは ListDailySensorAggregatesJST)
→ weather_readings は定期バッチ取得のため、クエリは少なく (Create, GetLatestByDevice, ListByDeviceInRange) で十分。集計は不要の想定。
```

**実現性への影響:** sensor_readings の 10 クエリは複雑 (複数集計経路・グラフ形式多様) だが、weather_readings は「API取得→DB保存」の最小形なら Create + GetLatest + ListInRange 3 クエリのみで開始可能。後に必要に応じ追加。既存パターンの simple version として新テーブル設計。

#### マイグレーション方針 (expand-contract・後方互換性)

```text
ファイル: /Users/c/Desktop/dev/go_iot/deploy/redeploy.sh L152〜175
不変条件: migration は「追加専用＝後方互換(expand-contract)」
フロー:
  1. 本番デプロイ時、ビルド済みバイナリ (goose up 「前」) を scp で転送
  2. SSH トンネル経由で goose up を実行 (未適用 migration を全適用)
  3. その直後に systemd restart でバイナリを swap → 新バイナリ起動
  ※ goose up から restart までの数秒は「旧バイナリ × 新スキーマ」で動作
タイムウィンドウ内で安全な操作:
  - ADD COLUMN(nullable): 旧バイナリは NULL を見て OK
  - ADD INDEX: バックグラウンド・旧バイナリに影響なし
  - ADD TABLE: テーブルはあるが旧バイナリが触らないなら OK
タイムウィンドウで危険な操作:
  - DROP COLUMN: 旧バイナリが読もうとして エラー
  - RENAME COLUMN: 旧バイナリが古い列名で読もうとして エラー
  - 型変更: スキャン/キャスト エラー
  - NOT NULL 追加: 旧バイナリの INSERT が NULL をセットして チェック違反
  → 破壊的変更は expand-contract で複数 migration に分割 (例: 1 回目 ADD COLUMN nullable, 2 回目 UPDATE DEFAULT, 3 回目 NOT NULL 追加)
goose 失敗時:
  - バイナリ入替えず中止
  - 本番は「旧バイナリ × 直前スキーマ」で継続
  - DB は新 migration 適用済み (rollback しない・migrate down は手動)
→ weather_readings: 初回は CREATE TABLE のみで OK（ADD COLUMN 等の破壊的変更なし）。
```

**実現性への影響:** 新テーブル追加時は migration 1 個で完結（CREATE TABLE はexpand-contract に抵触しない）。本番デプロイはgoose 冪等・自動で、手動 rollback/revert 不要。後述の DROP 等は将来 migration の手数を増やす (2〜3 段階) が必要だが、追加段階では影響ゼロ。

#### 新テーブル weather_readings 追加の具体的作業リスト

```text
順序: migration → sqlc query → repository models (自動) → querier (自動) → DIP 配線 (handler/service)

【1】db/migrations/00011_create_weather_readings.sql
  -- +goose Up
  CREATE TABLE weather_readings (
    id BIGSERIAL PRIMARY KEY,
    device_id BIGINT NOT NULL,
    temperature NUMERIC(5,2) NOT NULL,
    humidity NUMERIC(5,2),
    wind_speed NUMERIC(5,2),
    rain NUMERIC(7,2),
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
  );
  CREATE INDEX weather_readings_device_id_recorded_at_idx ON weather_readings(device_id, recorded_at DESC) WHERE deleted_at IS NULL;
  COMMENT ON TABLE weather_readings IS '気象 API からの定期取得データ';
  -- +goose Down
  DROP TABLE IF EXISTS weather_readings;

【2】db/queries/weather_readings.sql
  -- name: CreateWeatherReading :one
  INSERT INTO weather_readings (device_id, temperature, humidity, wind_speed, rain, recorded_at)
  VALUES ($1, $2, $3, $4, $5, $6)
  RETURNING *;

  -- name: GetLatestWeatherReading :one
  SELECT * FROM weather_readings
  WHERE device_id = $1 AND deleted_at IS NULL
  ORDER BY recorded_at DESC
  LIMIT 1;

  -- name: ListWeatherReadingsInRange :many
  SELECT * FROM weather_readings
  WHERE device_id = $1 AND recorded_at BETWEEN $2 AND $3 AND deleted_at IS NULL
  ORDER BY recorded_at ASC;

【3】make sqlc → internal/repository/weather_readings.sql.go (自動生成)
  - func (q *Queries) CreateWeatherReading(...) (WeatherReading, error)
  - func (q *Queries) GetLatestWeatherReading(...) (WeatherReading, error)
  - func (q *Queries) ListWeatherReadingsInRange(...) ([]WeatherReading, error)

【4】internal/repository/models.go (sqlc が自動 append)
  type WeatherReading struct {
    ID       int64
    DeviceID int64
    Temperature pgtype.Numeric
    Humidity pgtype.Numeric
    WindSpeed pgtype.Numeric
    Rain pgtype.Numeric
    RecordedAt pgtype.Timestamptz
    CreatedAt pgtype.Timestamptz
    UpdatedAt pgtype.Timestamptz
    DeletedAt pgtype.Timestamptz
  }

【5】internal/repository/querier.go (sqlc が自動 append)
  WeatherRepo interface に 3 メソッド追加

【6】internal/handler/weather_api.go (新規作成・DIP 配線)
  type WeatherRepo interface {
    authz.DeviceGetter
    CreateWeatherReading(ctx context.Context, arg repository.CreateWeatherReadingParams) (repository.WeatherReading, error)
  }

  type WeatherAPI struct {
    Repo WeatherRepo
  }

  func (h *WeatherAPI) Create(c *gin.Context) {
    var req CreateWeatherReadingRequest
    c.ShouldBindJSON(&req)
    device, err := authz.RequireDeviceOwner(...) // BOLA 認可
    reading, err := h.Repo.CreateWeatherReading(...) // DB 保存
    c.JSON(http.StatusCreated, ...)
  }

【7】cmd/server/main.go L126 に追加
  weatherAPI := &handler.WeatherAPI{Repo: q}
  apiGroup.POST("/weather-data", weatherAPI.Create)

【8】.env + internal/config/config.go
  - WEATHER_API_KEY env var 追加
  - Config.WeatherAPIKey string 追加
  - getEnv で読み込み

合計ファイル: 6 新規 + 5 修正 = 11 ファイル操作。
```

**実現性への影響:** 既存パターン (sensor_readings, AlertEvaluator) の完全コピーテンプレートで、15〜20分での実装可能。sqlc 自動生成のため型安全・手作業エラー最小化。学習曲線なし。

**参照ファイル:**

- `/Users/c/Desktop/dev/go_iot/cmd/sensor-sim/main.go`
- `/Users/c/Desktop/dev/go_iot/internal/config/config.go`
- `/Users/c/Desktop/dev/go_iot/.env.example`
- `/Users/c/Desktop/dev/go_iot/db/migrations/00004_create_sensor_readings.sql`
- `/Users/c/Desktop/dev/go_iot/db/migrations/00008_add_locality_to_devices.sql`
- `/Users/c/Desktop/dev/go_iot/db/queries/sensor_readings.sql`
- `/Users/c/Desktop/dev/go_iot/internal/repository/sensor_readings.sql.go`
- `/Users/c/Desktop/dev/go_iot/internal/repository/models.go`
- `/Users/c/Desktop/dev/go_iot/internal/repository/querier.go`
- `/Users/c/Desktop/dev/go_iot/internal/repository/db.go`
- `/Users/c/Desktop/dev/go_iot/internal/service/alert_evaluator.go`
- `/Users/c/Desktop/dev/go_iot/internal/handler/sensor_api.go`
- `/Users/c/Desktop/dev/go_iot/internal/authz/ownership.go`
- `/Users/c/Desktop/dev/go_iot/internal/domain/metric.go`
- `/Users/c/Desktop/dev/go_iot/internal/domain/comparison_operator.go`
- `/Users/c/Desktop/dev/go_iot/cmd/server/main.go`
- `/Users/c/Desktop/dev/go_iot/deploy/redeploy.sh`
- `/Users/c/Desktop/dev/go_iot/Makefile`
- `/Users/c/Desktop/dev/go_iot/sqlc.yaml`

---

# 第3部 天気API調査（詳細）

> 2つのサブエージェントが公式一次情報(WebSearch/WebFetch)で調べた天気APIの全文。

### 3-1 海外系API（Open-Meteo中心 / OpenWeatherMap / WeatherAPI.com）

日本・沖縄県の農業IoT向けに、緯度経度で現在天気・過去履歴・予報をプログラム取得できる天気APIを公式一次情報で調査した。結論として Open-Meteo を第一推奨とする。理由: (1) 気象庁JMAの数値予報モデル(GSM 0.5°≒55km / MSM 0.05°≒5km)を直接配信し、沖縄本島・先島(石垣/宮古/与那国)・大東島を含む全離島を緯度経度で引ける(MSM域は2014年以降4,320km×3,300kmに拡大、域外でもGSMグローバルで必ずカバー)。(2) 非商用はAPIキー不要・登録不要・10,000 calls/日(5,000/時・600/分・約300,000/月)、過去データはERA5で1940年まで遡及、Historical Forecast(2021〜、JMA MSM履歴は2016〜)で実測整合の高解像度履歴も取得可。(3) 取得変数が農業に必要な気温/相対湿度/露点/降水/日射(shortwave_radiation・direct・diffuse)/風/気圧/雲量を網羅し、露点VPDやGDD・日射分析に直結。(4) ライセンスはCC-BY 4.0で帰属表示すれば良く明快。注意点: 無料枠は「非商用」限定で、go_iotが顧客向け商用サービスとして配信する場合は商用プラン($29/月=1M calls、$99/月=5M calls、専用エンドポイント customer-api.open-meteo.com + APIキー + 99.9% SLA)契約が必要(これは推測でなく公式定義: 購読/広告/商用製品組込は商用扱い)。比較対象: OpenWeatherMap One Call 3.0は履歴1979年〜・1,000 calls/日無料だが要クレカ登録&従量課金(0.0014 EUR/call相当)。WeatherAPI.comは無料100,000/月と大きいが無料は履歴1日のみ・気象庁モデル非配信(自社予報)。沖縄ピンポイントの気象庁モデル直接性・過去遡及・無料枠・変数網羅のバランスでOpen-Meteoが最適。数値は全て公式ドキュメント確認済み(後述cons内の遡及開始年など一部はモデル依存で「約」表記)。

#### Open-Meteo (open-meteo.com)

- **認証**: 非商用: APIキー不要・登録不要・クレカ不要(HTTP GETのみ)。商用: 専用エンドポイント customer-api.open-meteo.com 用のAPIキーが購読で発行される。
- **無料枠/料金**: 非商用無料。レート制限: 10,000 calls/日、5,000 calls/時、600 calls/分、約300,000 calls/月(公式pricing記載)。無料枠はuptime保証なし。商用: Standard $29/月=1M calls/月、Professional $99/月=5M calls/月、Enterprise(>50M calls/月)はカスタム。商用は日次レート制限なし+99.9% uptime目標。(価格はopenmeteo.substack公式告知でUSD表記、pricingページは月/日上限のみ明記)
- **過去データ**: 可。Historical Weather API=ERA5(0.25°≒25km, 1940年〜)/ERA5-Land(0.1°≒11km, 1950年〜)/ECMWF IFS(9km, 2017年〜)。1940年まで遡及・基本hourly。別途Historical Forecast API=高解像度モデルの予報初期時刻を縫合した実測整合アーカイブ(2021年頃〜、JMA MSM履歴は2016-01-01〜・0.05°)。Forecastでも past_days で直近の過去取得可。商用予約リソース利用時のみAPIキー要。
- **予報**: 可。Forecast API最長16日。JMA専用: jma_msm=4日(0.05°≒5km高解像度・短期1-3日精度重視)、jma_gsm=11日(0.5°)。current(現在天気)はJMA APIでは15分値ベース。hourly予報対応。jma_seamlessでMSM+GSM自動接続。
- **沖縄カバレッジ**: 緯度経度(WGS84)で任意地点を引く方式・観測点リスト不要。グリッドへ補間+標高ダウンスケーリング。JMA MSM(0.05°≒5km)は日本+韓国域(2014年以降ドメイン4,320km×3,300kmへ拡大)で沖縄本島・先島(石垣N24.3/E124, 宮古, 与那国N24.45/E123)・大東島(N25.8/E131)を内包。仮にMSM域外でもGSMはグローバルで全離島カバー。複数座標をカンマ区切りで1リクエスト可。(MSM個別島の内包は域寸法からの判断・公式は『Japan, Korea』と記載)
- **商用利用**: データはCC-BY 4.0。帰属表示(クレジット)必須。無料枠は非商用限定(個人/非営利サイト・家庭用自動化・公的機関の公開研究・教育コンテンツ等)。購読/広告のあるサイト・アプリ、商用製品/販促への組込、商用主体の非公開研究は『商用』扱いで有料プラン契約必須。
- **取得変数**: 気温(2m)/相対湿度(2m)/露点/体感温度/降水/雨/にわか雨/降雪/日射(shortwave_radiation・direct_radiation・diffuse_radiation)/風(速度・向き・突風, 10/80/120/180m)/気圧(海面・地表面)/雲量(全/低/中/高層)/天気コード/土壌温度・水分。農業のVPD・露点病害・GDD・日射分析を全て満たす。
- **長所**: 気象庁JMAモデル(GSM/MSM)を直接配信し沖縄ピンポイントに強い。非商用は無料・キー不要で即利用。過去1940年まで(ERA5)。変数が農業向けに網羅的(露点/日射/相対湿度)。CC-BY 4.0で利用条件が明快。JSON/CSV/XLSX対応・複数座標一括取得。オープンソースで自前ホストも可能。
- **短所**: 無料枠は非商用限定で、顧客提供型サービスは商用プラン必須(月$29〜)。JMA MSMは『データライセンス制限により限定版』とされ全JMA公式データは網羅しない。current天気はMSMモデルベースのシミュレーション値(現地観測の生値ではない)。ERA5は気候トレンド向きで日々の精度はHistorical Forecastに劣る。無料枠はSLAなし。遡及開始年はモデルにより異なる(MSM履歴は2016年〜)。
- **判定**: recommended

#### OpenWeatherMap (One Call API 3.0 / openweathermap.org)

- **認証**: APIキー必須。One Call 3.0は『One Call by Call』購読登録が必要で、無料枠利用でも従量課金のためクレジットカード登録が事実上必要(公式は明示せずだが課金前提)。
- **無料枠/料金**: One Call 3.0: 1,000 calls/日まで無料、超過は従量(Base plan 0.14 EUR/100 calls=約0.0014 EUR/call, VAT別)。別系統のClassic無料API(現況+3時間予報等)は60 calls/分・1,000,000 calls/月。
- **過去データ**: 可。One Call 3.0のTime Machineで1979年1月1日〜任意タイムスタンプを取得(約47年遡及)。ただし履歴呼び出しもcall消費・従量課金対象。
- **予報**: 可。current・1分単位1時間先・hourly 48時間・daily 8日・政府気象警報を1エンドポイントで提供。
- **沖縄カバレッジ**: 緯度経度で任意地点取得可(グローバル)。沖縄離島も座標で引けるが、配信は自社統合モデル中心で気象庁JMAモデルの直接配信ではない。離島ピンポイントの解像度は公式に明示なし(推測: グローバルモデル由来で数km〜十数km級)。
- **商用利用**: 商用利用可(従量課金モデル)。プランに応じた商用ライセンス。帰属/クレジットの扱いはプラン規約に従う。
- **取得変数**: 気温/体感/相対湿度/露点/気圧/雲量/風(速度・向き・突風)/降水(雨・雪)/UV指数/視程/天気概況。日射は派生指標が中心でshortwave_radiation等の直接配信はOpen-Meteoほど明確でない。
- **長所**: 履歴1979年〜と遡及が長い。current/分・時・日予報+警報を1エンドポイント統合。世界的に実績・ドキュメント豊富。1,000 calls/日まで無料。
- **短所**: 無料でもクレカ登録+従量課金前提でコスト予測しづらい。気象庁JMAモデルの直接配信ではない(沖縄ピンポイント優位性が弱い)。日射変数の充実度はOpen-Meteoに劣る。料金体系がOne Call 3.0/4.0等で分かりにくい。
- **判定**: viable

#### WeatherAPI.com (weatherapi.com)

- **認証**: APIキー必須(サインアップで取得・プラン変更時もキー据置)。
- **無料枠/料金**: 無料枠100,000 calls/月(大きめ)・uptime 95.5%。有料: Starter $7/月($75/年)、Pro+ $25/月、Business $65/月、Enterpriseカスタム(年払い10%割引)。
- **過去データ**: 無料は過去1日のみ。有料で拡大: Starter=過去7日、Pro+=過去365日、Business/Enterprise=2010年1月1日〜。
- **予報**: current+3日予報(無料)。上位プランで予報日数拡張。予報は各機関の生データに地形・人口・標高等を加味した自社予報エンジン出力。
- **沖縄カバレッジ**: 緯度経度・地名で任意地点取得可(グローバル)。沖縄離島も座標で引けるが自社統合予報で気象庁JMAモデルの直接配信ではない。離島の解像度は公式に明示なし。
- **商用利用**: 商用利用は有料プランで可。帰属の扱いはプラン規約に従う。
- **取得変数**: 気温/体感/相対湿度/露点/気圧/雲量/風(速度・向き・突風)/降水/視程/UV/天文(日出没)/大気質(AQI)/警報。日射(shortwave_radiation)の直接配信はなく農業の日射分析にはやや不足。
- **長所**: 無料枠100,000/月と大きく現況+3日予報を低コストで。単一の整理されたエンドポイント群。大気質・天文・警報も統合。料金が安価で分かりやすい($7〜)。
- **短所**: 無料は履歴1日のみで過去分析に弱い(長期履歴は有料2010年〜)。気象庁JMAモデル非配信(自社予報)で沖縄ピンポイント優位性なし。日射変数が乏しくGDD/日射分析に不向き。予報日数も無料3日と短い。
- **判定**: fallback

---

### 3-2 気象庁・日本特化データ源（bosai/AMeDAS/obsdl / Open-Meteo / WeatherAPI / Ambient）

日本の気象データをプログラム取得する手段を、一次情報(WebSearch/WebFetch)で確認した結果をまとめる。【気象庁の準公式JSON】天気予報は `https://www.jma.go.jp/bosai/forecast/data/forecast/{予報区コード}.json` で取得でき、沖縄県は1つではなく4予報区に分かれる(沖縄本島地方=471000、大東島地方=472000、宮古島地方=473000、八重山地方=474000)。重要: 471000には大東島・宮古・八重山は含まれず、離島カバーには472000/473000/474000の併用が必須。取得内容は天気・気温・降水確率・風・波で最大7日先。実機検証で471000.json/amedastable.json/latest_time.txt とも稼働確認済み(latest_time.txtは2026-06-30T22:30:00+09:00を返答)。ただしこれは公式に「API」として提供されたものではなく、気象庁防災情報サイトが内部利用するファイル配信を非公式に流用するもので、SLA・仕様安定の保証はなく予告なき変更リスクがある(気象庁自身も問い合わせ対象外と整理)。【AMeDAS】観測所一覧は `bosai/amedas/const/amedastable.json`、最新時刻は `bosai/amedas/data/latest_time.txt`、全国マップ値は `bosai/amedas/data/map/{時刻}.json`、地点別時系列は `bosai/amedas/data/point/{観測所番号}_{3時間区切}.json`。沖縄の離島は四要素観測所(気温・降水・風・日照)として南大東(92011)・北大東(92006)・久米島(91146)・下地島(93012)・仲筋=多良間(93062)・所野=与那国(94011/島は94017)・西表島(94062)などが該当し、農業IoT用途で重要な気温・降水・風・日照は取得可能。ただしAMeDASは「湿度」を観測する地点が極めて限定的(沖縄離島の四要素観測所は気温/降水/風/日照が基本で湿度は基本的に非観測)で、湿度が必須なら気象台級地点か別データ源が必要。【利用規約】気象庁HPコンテンツは「公共データ利用規約(第1.0版)」準拠で商用利用可・改変可だが出典明示必須(「出典:気象庁ホームページ(URL)」)、加工時は加工した旨の明記も必須。過去データのobsdl(過去の気象データ・ダウンロード)は「自動化ツール等による過度のアクセスはお控えください」と明記、公式APIは無くCSV手動DL or スクレイピング(各実装例は1リクエストあたり数秒スリープ)。【代替】最有力はOpen-Meteo: JMA MSM(5km/1時間/4日)+GSMを使い、緯度経度で沖縄離島ピンポイント取得可、変数に日射(shortwave_radiation)・ET0・露点・湿度まで揃い、Historical Weather API(ERA5)で1940年〜の過去データ取得可。データはCC BY 4.0で商用含め帰属表示すれば利用可だが、無料ホスト枠は非商用限定(1万call/日)で商用は有料APIキー必須。WeatherAPI.comはFree=10万call/月・商用可・予報3日・履歴は過去1日のみ(長期履歴は要有料)。総合推奨: 観測実測値は気象庁AMeDAS(bosai JSON)を出典明示で利用しつつ、離島ピンポイント予報・日射/湿度/長期過去データはOpen-Meteoで補完する二段構え。

#### 気象庁 防災情報JSON(天気予報) bosai/forecast

- **認証**: 不要(APIキー不要・登録不要)。公開URLへGETするのみ。
- **無料枠/料金**: 完全無料。公式なレート制限の記載は無いが、非公式流用のため節度あるアクセス(キャッシュ・ポーリング間隔を空ける)が前提。予報は1日数回更新(05/11/17時頃+随時)。
- **過去データ**: 不可。最新の予報スナップショットのみ配信され、過去予報のアーカイブAPIは無い。過去の実測は別系統(obsdl/AMeDAS)を使う。
- **予報**: 可。天気・気温・降水確率・風・波を当日〜翌々日(時系列)+週間で最大7日先まで。reportDatetimeで更新時刻取得。
- **沖縄カバレッジ**: 重要: 沖縄県は4予報区に分割。沖縄本島地方=471000、大東島地方=472000、宮古島地方=473000、八重山地方=474000。471000には大東・宮古・八重山は含まれないため離島網羅には4コード併用必須。本島内さらに細分(本島中南部471010/本島北部471020/久米島471030)。区域定義は bosai/common/const/area.json。
- **商用利用**: 可。公共データ利用規約(第1.0版)準拠。出典明示必須(例:『出典:気象庁ホームページ(URL)』)、加工時は加工した旨も明記。
- **取得変数**: 天気(weatherCode/天気文)、最高/最低気温(temps)、降水確率(pops)、風向、波(沿岸)。湿度・日射・気圧は含まない(予報JSONのため)。
- **長所**: 公式一次情報・無料・キー不要・JSONで扱いやすい・沖縄全離島を予報区単位でカバー・出典明示すれば商用可。
- **短所**: 公式『API』ではなく防災サイトの内部ファイル配信の非公式流用でSLA/仕様安定の保証なし(予告なき変更リスク)。過去予報の蓄積なし。気温は地点が限定的(代表都市)で離島ピンポイントではない。湿度/日射が無く農業IoTの派生指標(VPD等)計算には不足。
- **判定**: viable

#### 気象庁 AMeDAS観測値JSON bosai/amedas

- **認証**: 不要(APIキー不要・登録不要)。公開URLへGET。
- **無料枠/料金**: 完全無料。明示的レート制限は無いが防災サイト流用のため過度アクセス回避が前提。観測は10分更新(latest_time.txtで最新時刻取得→該当ファイル取得のフロー)。
- **過去データ**: 限定的に可。bosai/amedasは概ね直近(数日〜十数日程度の3時間区切りファイル point/{番号}_{時刻}.json)の時系列に留まる。長期の過去実測は obsdl(過去の気象データ・ダウンロード)側でCSV取得(公式API無し)。
- **予報**: 不可(観測専用)。予報は bosai/forecast を使う。
- **沖縄カバレッジ**: 良好。沖縄離島の四要素観測所(気温/降水/風/日照)として 南大東=92011・北大東=92006・久米島=91146・下地島=93012・仲筋(多良間)=93062・所野(与那国)=94011/与那国島=94017・西表島=94062 等が利用可。三要素(気温/風/雨量)に伊是名91011・大原94101・波照間94116等、雨量のみに粟国91096・渡名喜91151等。沖縄県のアメダスは計約26地点(四要素8/三要素9/雨量9)。観測所一覧は amedastable.json で全番号・緯度経度・観測要素(elems)取得可。
- **商用利用**: 可。公共データ利用規約(第1.0版)準拠で商用可・出典明示必須・加工時は加工明記。
- **取得変数**: 気温・降水量・風向風速・日照時間(四要素地点)。湿度は観測地点が極めて限定的で沖縄離島の四要素観測所では基本的に非観測。積雪は沖縄では非対象。各地点の取得可否は amedastable.json の elems で要確認。
- **長所**: 公式の実測値・無料・キー不要・10分粒度・沖縄離島を実観測点でカバー・出典明示で商用可。農業IoTの実測補完(気温/降水/風/日照)に最適。
- **短所**: 公式API扱いでなく非公式流用(仕様変更リスク)。湿度が離島でほぼ取れず派生指標(VPD/THI)計算には別源が必要。長期過去データはこのJSONでは賄えずobsdl併用。点別時系列は観測所番号管理が必要でやや煩雑。
- **判定**: recommended

#### 気象庁 過去の気象データ・ダウンロード(obsdl / etrn)

- **認証**: 不要だがWebフォーム経由。プログラム取得は公式API無くスクレイピング(POSTフォーム模倣 or HTMLパース)になる。
- **無料枠/料金**: 無料。ただし公式に『自動化ツール等による過度のアクセスはお控えください』と明記。実装例では1取得ごとに数秒(例:5秒)スリープが慣行。データ量(地点×項目×期間)100%上限で表示/DL不可になる制限あり。
- **過去データ**: 可(本命)。時別値・日別値・半旬/旬/月/3か月別値などを地点指定でCSV取得。観測開始(地点により数十年〜)まで遡及可。沖縄離島地点も対象。
- **予報**: 不可(過去データ専用)。
- **沖縄カバレッジ**: 良好。AMeDAS/地上気象観測所として沖縄離島の各地点(南大東・北大東・久米島・石垣島・宮古島・与那国島等)の長期統計を取得可。地点コードは etrn の prec_no(都道府県=沖縄)+ block_no(地点)で指定。
- **商用利用**: 可。公共データ利用規約(第1.0版)準拠・出典明示必須・加工明記。ただし過度アクセス自粛要請の遵守が前提。
- **取得変数**: 気温(平均/最高/最低)・降水量・風・日照時間・全天日射量(観測地点による)・湿度(気象台級地点)・気圧等。obsdlは項目が豊富で日射/湿度も気象台級で取得可。
- **長所**: 公式・無料・長期(数十年)の高品質統計・沖縄離島カバー・農業の年次/季節トレンド分析(GDD/月次MK-Sen等)に最適。
- **短所**: 公式APIが無くスクレイピング前提=実装/保守コストとHTML仕様変更リスク。過度アクセス自粛要請でバッチは低速(数秒間隔)必須。リアルタイム/予報用途は不可。
- **判定**: viable

#### Open-Meteo (JMAモデル + Historical/ERA5)

- **認証**: 無料(非商用)はAPIキー不要。商用利用は有料プランのAPIキー(customer-付きエンドポイント)が必須。
- **無料枠/料金**: 無料枠は非商用限定で 1万call/日・5千call/時・600call/分(月約30万)。商用はStandard(100万/月)〜Professional(500万/月)〜Enterprise(5000万+/月)の有料(料金は要問い合わせ)。
- **過去データ**: 可(強力)。Historical Weather API(ERA5)で1940年〜現在、ERA5-Land 1950年〜(約9km)。Historical Forecast APIは2017年〜(9km)。長期の過去解析に最適。
- **予報**: 可。JMA MSM(0.05°≒5km/1時間/4日)+GSM(0.5°/6時間/11日)。デフォルト7日・最大11日。
- **沖縄カバレッジ**: 優秀。緯度経度指定で任意地点をピンポイント取得でき、沖縄本島・大東・宮古・八重山・与那国・多良間・久米島など全離島を観測所有無に関係なく取得可(複数座標カンマ区切り対応)。MSMの5km格子は日本全域カバー。
- **商用利用**: データはCC BY 4.0で商用含め帰属表示すれば利用可。ただしOpen-Meteoの無料ホスティングは非商用限定で、商用利用は有料APIキー契約が必要(データのライセンスとホスト枠の利用条件は別レイヤ)。出典明示必須(例:『Weather data by Open-Meteo.com (CC BY 4.0)』、元データ気象庁/ECMWFも併記推奨)。
- **取得変数**: 気温・相対湿度・露点・体感温度・降水・降雪・気圧・雲量・風・短波(全天)日射/直達/散乱日射・日照時間・蒸発散量ET0・気圧面データ。農業IoTのVPD/GDD/日射収支に必要な変数が網羅。
- **長所**: 緯度経度で離島ピンポイント・湿度/日射/ET0まで揃う・1940年〜の長期過去・予報も同一APIで一貫取得・無料枠潤沢(非商用)・JMAモデル使用で日本精度良好。
- **短所**: 商用利用は有料(本番サービス組込時はコスト発生)・実測値ではなくモデル/再解析値(局地的な実観測との差あり)・第三者サービス依存(可用性/仕様は自社管理外)・無料枠の非商用条件に注意。
- **判定**: recommended

#### WeatherAPI.com

- **認証**: 要(無料登録でAPIキー発行)。
- **無料枠/料金**: Free=10万call/月。Starter $7/月($75/年)〜。
- **過去データ**: Freeは過去1日のみ(実質ほぼ不可)。長期履歴(2010年〜)は有料プラン。
- **予報**: Freeは3日先まで。最大14日は上位プラン。15分間隔・時別・日別あり。
- **沖縄カバレッジ**: 緯度経度/地名指定で世界中対応のため沖縄離島も座標指定で取得可能(AI/MLによる任意地点推定)。ただし実観測網ベースでなく日本ローカル精度はJMA系より劣る可能性。
- **商用利用**: Freeでも商用利用可(料金表でCommercial Use✓)。
- **取得変数**: 現況・予報(気温/湿度/降水/風/気圧/UV)、海洋、空気質、花粉、ET、天文等。日射は限定的。
- **長所**: Free商用可・10万call/月と寛容・登録のみで即利用・現況/予報/空気質まで単一キー。
- **短所**: Freeは履歴1日・予報3日と短く農業の過去/長期分析には不足→有料前提。日本ローカル精度は気象庁系に劣る可能性。実測ではなく推定値。長期過去はOpen-Meteoの方が無料で優位。
- **判定**: fallback

#### Ambient(アンビエントデータ ※ユーザー既知の引継ぎ案件)

- **認証**: 要(自前デバイス登録+APIキー)。気象官署データではなく自設置IoTセンサーのクラウド蓄積/可視化サービス。
- **無料枠/料金**: 個人/小規模は無料枠あり(チャネル数・送信間隔・保持期間に制限)。商用/大量は要確認。
- **過去データ**: 可。自デバイスが送信した過去データをAPI/CSVで取得可(保持期間はプラン依存)。気象官署の過去データではない点に注意。
- **予報**: 不可(自センサー実測の蓄積/可視化が主目的、予報機能は持たない)。
- **沖縄カバレッジ**: 設置場所依存(圃場に置いた自センサーの地点のみ)。離島カバレッジは『そこにデバイスを置けるか』に等しく、気象官署網のような広域カバーではない。go_iot本体と用途が重複。
- **商用利用**: サービス規約依存。自データなので基本は自由度高いがAmbientの利用規約に従う。
- **取得変数**: 接続したセンサー次第(温度/湿度/気圧/照度/CO2等)。標準で温湿度。
- **長所**: 圃場ピンポイントの実測・引継ぎ案件で既知・温湿度を直接取得・導入容易。気象官署が無い圃場の実値を埋められる。
- **短所**: 気象官署の公式データではない・地点は自設置箇所のみで広域比較に弱い・go_iot(自前IoT基盤)と機能重複しベンダーロックインの懸念・予報/長期統計の代替にはならない。
- **判定**: fallback

---

# 第4部 役場座標の検証（53地域の代表点）

## 4-0 統合回答

この質問は調査済みの検証結果(JSON)に基づく整理と判断であり、コード変更やファイル探索を伴わない。提供データを精査して直接回答する。

---

## 1. 結論 ― 「必ず役場があるか」

**未合併36地域(8市9町19村)**: 全て今も独立した市町村なので、現役の役所/役場が必ず存在する。代表点として無条件で使える(自明)。

**合併17地域**: 独立役場は平成大合併で消滅したが、「代表点として使える施設」の有無は地域ごとに分かれる。検証結果は以下の通り。

| 区分 | 件数 | 旧町村(地域) |
|---|---|---|
| 本庁あり(合併先本庁が当該地域内) | 5 | 具志川市→うるま市本庁、平良市→宮古島市総合庁舎、佐敷町→南城市本庁、東風平町→八重瀬町本庁、仲里村→久米島町本庁 |
| 出張所/支所/庁舎あり(座標確認済) | 9 | 石川市・与那城町・勝連町(うるま市)、城辺町・下地町・上野村・伊良部町(宮古島市)、具志頭村(八重瀬町)、具志川村(久米島町・現出張所) |
| 廃止(施設なし) | 2 | **玉城村**(玉城庁舎2018閉庁・跡地=オキナワインターナショナルスクール)、**大里村**(大里庁舎2018閉庁・跡地=ソニービジネスオペレーションズ。ただし旧庁舎建物の座標は取得済 26.17616/127.74462) |
| 現存/廃止が公式未確認 | 1 | **知念村**(旧知念庁舎。2018統合は公式に「玉城・大里の2庁舎から」と明記され知念は記載なし。窓口は本庁集約) |

つまり合併17のうち**14地域は代表点に使える施設が現存・座標確認済、残り3地域(玉城・大里・知念)が要手当て**。ただし大里は廃止だが旧庁舎建物の座標は取得済なので地理的代表点としては流用可能。

---

## 2. 役場座標を代表点に使う案の妥当性

役所/役場座標は **公開・権威・安定** の三拍子が揃い、代表点として優れている。
- 公開: 公式サイトに住所・度分秒が載る。
- 権威: 行政が公示する地点で恣意性がない。
- 安定: 移転は数十年に一度で、移転時もニュース化されるため追従が容易。

**ただし合併17で「支所/出張所の座標」を使う場合の注意**:
- 支所は旧役場の系譜を引くものの、旧町村の**地理的中心とずれる**ことがある(例: 与那城出張所は本島側にあり、与那城地域に属する平安座島・宮城島・伊計島等の離島群とは数km離れる。八重瀬町具志頭出張所は旧役場と同住所で問題なし)。
- **ただし天気APIは概ね5km格子(メッシュ)で配信されるため、数km程度のずれは同一格子内に収まることが多く実害は小さい。** むしろ役場は人口集積地=農地に近いことが多く、農業用途の代表点として妥当。
- 離島を含む地域(与那城・勝連)で、関心対象が離島側の圃場なら役場座標は不適。後述4の離島扱いで補正する。

---

## 3. 同名異所の罠

検証JSONが繰り返し警告している通り、**「具志川」が2つある**:
- **具志川市** → 現うるま市(沖縄本島中部・東海岸)。本庁 26.379256/127.857403。
- **具志川村** → 現久米島町(久米島・離島の西半部)。現具志川出張所 ≒ 26.34912/126.74728(旧庁舎座標)。

経度が **127.8°台(本島) vs 126.7°台(久米島)** で約100km離れる別物。locality値(現市町村名・親)で区別済みだが、**座標表をCSVや配列で作る際にlat/lonの行を取り違えると、うるま市の点が久米島に飛ぶ等の致命的ミスになる**。表作成時は「具志川」を含む2行を必ず経度で突き合わせ検証すること。

---

## 4. 代替・補強案

役場が不適/未確認な地域への手当て(優先度順):

**(a) 合併先の本庁座標で代用** ― 最も手堅い。玉城・大里・知念はいずれも南城市域なので、南城市本庁(26.16321/127.77059)で代用可能。ただし3地域とも本庁から数km離れるため、地域差を見たい用途では精度不足。

**(b) 旧町村の地理的中心(旧自治体境界の重心)** ― 旧町村界ポリゴンの重心を取る。役場(人口集積地寄り)より農地分布を均す意味では中立的。国土数値情報の旧行政界データが必要で実装コスト中。玉城・大里・知念や離島地域の補正に有効。

**(c) 主要農業地域の中心** ― 本案件は農業IoTなので、各地域の主要圃場エリア中心が理想だが、データ源が無く恣意性が高い。ユーザー(沖縄農業実地知見)のヒアリングが前提。第一版では採らず、特定地域で精度問題が出たら個別適用が現実的。

**離島の扱い**:
- **伊良部町(伊良部島)**: 伊良部出張所(24.831979/125.193493・伊良部島内)が島内にあり代表点として適切。ただし旧役場(長浜1296)と現出張所(前里添1056)の住所混同に注意、公式の前里添を採用済。
- **久米島の2地域(仲里村・具志川村)**: 本庁・出張所とも久米島内にあり橋連絡不要。代表点として適切(具志川村は現出張所966-33の正確座標は未確認、旧庁舎699座標で近接代用)。
- **与那城・勝連の離島群**(平安座島・宮城島・伊計島・浜比嘉島・津堅島等): 役場は本島側。離島圃場が対象なら(b)重心または個別指定が必要。

---

## 5. 次アクション

**53地域の代表座標表を役場/支所座標で作る場合の充足度**:

- **即座に座標が埋まる(座標確認済)**: 未合併36 + 合併14(本庁5+出張所9) = **50地域**。ただし未合併36の役場座標自体はこのJSONに含まれず、別途公式サイト/国土数値情報からの取得が必要(取得は容易・自明)。
- **手当てが必要: 3地域**
  1. **玉城村** ― 座標完全未確認。施設廃止。→ 南城市本庁代用(a) または 旧庁舎住所(玉城富里143)を実装時にジオコーディングして一次確認。
  2. **大里村** ― 旧庁舎建物座標は取得済(26.17616/127.74462・**ただしMapionのPOI由来で一次公式値ではない**)。代表点として流用可だが、厳密には実装時に再確認推奨。
  3. **知念村** ― 知念庁舎の現存/廃止が**公式未確認**。旧役場住所(知念久手堅22)は確定なので、実装時にジオコーディング+現存確認。

**未確認と明記すべき座標**(検証で一次値が取れていないもの):
- 玉城村: lat/lon とも未確認(実装時に一次ソース確認)
- 知念村: lat/lon とも未確認(実装時に一次ソース確認)
- 大里村: 旧庁舎座標はMapion由来=一次公式値ではない(実装時に再確認推奨)
- 具志川村(久米島)・具志頭村・与那城・勝連: 現施設のピンポイント公式座標は未確認で地図サービス値または旧庁舎値を採用(実装時に再確認推奨)

**結論**: 役場/支所座標で **53地域中50地域は実用上ほぼ即埋まる**(うち未合併36は別途公式取得が必要だが自明)。**個別の一次確認が要るのは玉城・知念の2地域(座標完全未確認)、再確認推奨が大里+地図サービス値採用の数地域**。第一版は (a)合併先本庁代用 で3地域を暫定的に埋め、精度問題が出た地域だけ (b)重心 へ格上げする段階的アプローチを推奨する。

---

## 4-1 合併市町ごとの検証詳細

> 平成の大合併で統合された旧町村17地域について、合併先の本庁/支所/出張所の現存と代表座標を市町ごとに検証した全文。

### うるま市

2005年(平成17年)4月1日、具志川市・石川市・与那城町・勝連町の2市2町が新設合併してうるま市(沖縄本島中部・東海岸)が発足。役場系施設の現状は一次情報(うるま市公式サイト「市役所・出張所」「アクセスマップ」および各施設個別ページ)で確認できた。本庁はうるま市みどり町1-1-1(=旧具志川市域)に所在。背景ヒントの「本庁は旧具志川市役所」は地域(旧具志川市)としては合致するが、現本庁はみどり町であり旧具志川市役所の建物そのものを指すかは公式上明確でない(よって座標は現本庁=みどり町を代表点とした)。石川・与那城・勝連の3地域には現在「出張所(庁舎)」が現存し、それぞれ旧石川市役所・旧与那城町役場・旧勝連町役場の系譜。公式サイトは「石川出張所」「与那城出張所」「勝連出張所」(=庁舎)と表記。各代表点座標は公式個別ページおよびホームメイト(パブリネット)で取得。重要: 旧具志川市(うるま市)と旧具志川村(久米島町)は同名異所で別物、本調査は前者のみを対象とした。勝連・与那城地域には海中道路・架橋で結ばれた離島(平安座島・宮城島・伊計島・浜比嘉島・藪地島、津堅島)が属する。

| 旧町村(地域) | 役場/支所の状況 | 施設名 | 緯度 | 経度 | 座標の出典 |
|---|---|---|---|---|---|
| 具志川市 | 本庁あり | うるま市役所(本庁舎・西棟/東棟) | 26.37926 | 127.85740 | 地図(ホームメイト/パブリネット施設DB)。住所はうるま市公式サイトで確認(みどり町1-1-1) |
| 石川市 | 出張所あり | うるま市役所 石川出張所(石川庁舎) | 26.42715 | 127.82925 | うるま市公式サイト 石川出張所個別ページ記載の緯度経度(26.427147, 127.829249)。ホームメイトでも近似値(26.42717,127.82901)で整合 |
| 与那城町 | 出張所あり | うるま市役所 与那城出張所(与那城庁舎) | 26.32998 | 127.90456 | 地図(ホームメイト/パブリネット施設DB: 26.329978, 127.904561)。住所はうるま市公式アクセスマップで確認(与那城中央1) |
| 勝連町 | 出張所あり | うるま市役所 勝連出張所(勝連庁舎) | 26.31502 | 127.89669 | 地図(ホームメイト/パブリネット施設DB: 26.315017, 127.896692)。住所はうるま市公式アクセスマップで確認(勝連平安名3047) |

**地域ごとの補足:**

- **具志川市**（本庁あり）: 旧具志川市域。沖縄本島中部・東海岸。現本庁はうるま市みどり町1丁目1番1号(代表 098-974-3111)。背景ヒント『本庁は旧具志川市役所』は地域としては旧具志川市で合致するが、現本庁はみどり町に所在し、旧具志川市役所の建物そのものか(移転/新築か)は今回の一次情報では明確に確認できず=この点は不明。代表点は現本庁(みどり町)とした。具志川地域には旧具志川市の固有出張所表記は公式一覧に無く、本庁が当該地域の役場系施設を兼ねる。注意: 旧具志川村(現久米島町)とは別物。
- **石川市**（出張所あり）: 旧石川市役所をそのまま石川出張所(石川庁舎)として継承(『the former Ishikawa City Hall』)。所在地: うるま市石川石崎1丁目1(〒904-1104)、Tel 098-965-5609/5602。開庁8:30-17:15(土日祝・年末年始休)。窓口・事務室・会議室・議場あり。沖縄本島中部の本島側(離島ではない)。
- **与那城町**（出張所あり）: 旧与那城町役場の系譜。所在地: うるま市与那城中央1(〒904-2305)、Tel 098-978-2655。与那城地域には海中道路・架橋で結ばれた離島(平安座島・宮城島・伊計島・浜比嘉島・藪地島)が属する(庁舎自体は本島側)。市内全域の土木・道路関係部署が入るとの情報あり。
- **勝連町**（出張所あり）: 旧勝連町役場の系譜(『以前は勝連町役場で、合併してうるま市勝連出張所になった』)。所在地: うるま市勝連平安名3047 勝連地区公民館1階(〒904-2312)、Tel 098-978-7193。敷地内に肝高ホール・図書館。注: 公式アクセスマップは番地3047、Wikipedia等は3032と表記揺れあり(同一の平安名地区内)。勝連半島の本島側で、津堅島など離島が勝連地域に属する。

---

### 宮古島市

2005年(平成17年)10月1日、平良市・城辺町・下地町・上野村・伊良部町の1市3町1村が合併して宮古島市が発足した。合併後は分庁(分散)方式を採り、伊良部町のみ「総合支所」として設置され他は各庁舎に業務分散していた。2021年1月4日に旧平良地区(平良字西里1140)に新「総合庁舎」が開庁し本庁機能を集約。総合庁舎完成後も城辺・下地・上野・伊良部の各地区には支所機能(住民課窓口)が維持されている。宮古島市公式サイトの市民課ページでは各地区の市民窓口が「城辺出張所・上野出張所・下地出張所・伊良部出張所」として案内されており、いずれも現存し代表点として利用可能。施設(庁舎)ベースでは、旧平良庁舎系は総合庁舎へ統合、城辺・下地・上野の旧庁舎は公式に「旧城辺庁舎・旧下地庁舎・旧上野庁舎」と表記され(出張所はそこに併設/近接)、伊良部のみ「伊良部庁舎」「伊良部支所」として残る。対象5地区すべてに代表点として使える市の窓口施設が現存する。重要:旧具志川市(現うるま市)・旧具志川村(現久米島町)は本件と無関係であり混同していない。座標は宮古島市公式記載の各窓口住所をgeocoding.jpでジオコーディングした値(本庁は公式住所のジオコーディング値がWikipedia掲載のDMS北緯24度47分24秒/東経125度17分42秒と一致し検証済み)。庁舎名称・住所・電話は宮古島市公式サイト(city.miyakojima.lg.jp)の一次情報で確認した。

| 旧町村(地域) | 役場/支所の状況 | 施設名 | 緯度 | 経度 | 座標の出典 |
|---|---|---|---|---|---|
| 平良市 | 本庁あり | 宮古島市役所 総合庁舎(本庁) | 24.78982 | 125.29510 | 宮古島市公式記載住所(平良字西里1140番地)をgeocoding.jpでジオコーディング。Wikipedia掲載DMS(北緯24度47分24秒=24.7900/東経125度17分42秒=125.295)と一致し検証済み |
| 城辺町 | 出張所あり | 宮古島市役所 市民課 城辺出張所(施設名: 旧城辺庁舎) | 24.75799 | 125.38768 | 宮古島市公式記載住所(城辺字福里600番地1)をgeocoding.jpでジオコーディング |
| 下地町 | 出張所あり | 宮古島市役所 市民課 下地出張所(施設名: 旧下地庁舎) | 24.74866 | 125.27787 | 宮古島市公式記載住所(下地字上地505番地)をgeocoding.jpでジオコーディング |
| 上野村 | 出張所あり | 宮古島市役所 市民課 上野出張所(施設名: 旧上野庁舎) | 24.73799 | 125.31766 | 宮古島市公式記載住所(上野字上野395番地1)をgeocoding.jpでジオコーディング |
| 伊良部町 | 支所あり | 宮古島市役所 伊良部支所 / 市民課 伊良部出張所(施設名: 伊良部庁舎) | 24.83198 | 125.19349 | 宮古島市公式記載の伊良部出張所住所(伊良部字前里添1056番地1)をgeocoding.jpでジオコーディング |

**地域ごとの補足:**

- **平良市**（本庁あり）: 宮古島(本島)中心市街地。旧平良市域。2021年1月4日に新総合庁舎が開庁(背景ヒントの新庁舎2023は誤りで実際は2021年開庁)、本庁機能を集約。〒906-8501 沖縄県宮古島市平良字西里1140番地。代表電話0980-72-3751。出典: https://www.city.miyakojima.lg.jp/kurashi/shisetsu/tyousya/sougou.html
- **城辺町**（出張所あり）: 宮古島(本島)南東部、旧城辺町域。市民課ページに城辺出張所として現存(住民票・戸籍・印鑑証明等21種の証明発行に対応)。施設は公式に『旧城辺庁舎』と表記。〒906-0103 宮古島市城辺字福里600番地1、電話0980-77-4905。出典: 市民課 https://www.city.miyakojima.lg.jp/soshiki/shityo/seikatukankyou/shiminseikatu/ , 旧城辺庁舎 https://www.city.miyakojima.lg.jp/kurashi/shisetsu/tyousya/gusukube.html
- **下地町**（出張所あり）: 宮古島(本島)南西部、旧下地町域。市民課ページに下地出張所として現存。施設は公式に『旧下地庁舎』と表記(旧下地庁舎の所在地は下地字上地472番地39、出張所窓口は下地字上地505番地で記載される)。〒906-0304付近。電話0980-76-6001。出典: 市民課 https://www.city.miyakojima.lg.jp/soshiki/shityo/seikatukankyou/shiminseikatu/ , 旧下地庁舎 https://www.city.miyakojima.lg.jp/kurashi/shisetsu/tyousya/shimoji.html
- **上野村**（出張所あり）: 宮古島(本島)南部、旧上野村域(村役場は字上野に所在、合併後は宮古島市役所上野庁舎→現在は上野出張所)。市民課ページに上野出張所として現存。施設は公式に『旧上野庁舎』と表記。宮古島市上野字上野395番地1、電話0980-76-6821。出典: 市民課 https://www.city.miyakojima.lg.jp/soshiki/shityo/seikatukankyou/shiminseikatu/ , 旧上野庁舎 https://www.city.miyakojima.lg.jp/kurashi/shisetsu/tyousya/ueno.html
- **伊良部町**（支所あり）: 離島=伊良部島(下地島と隣接)。2015年1月31日開通の伊良部大橋で宮古島本島と陸路連絡(同時に定期航路は全廃)。旧伊良部町役場は伊良部字長浜1296に所在し、合併当初は『宮古島市役所伊良部総合支所』として設置(5地区で唯一の総合支所)。現在は公式組織として『伊良部支所』、市民窓口として『伊良部出張所』、施設として『伊良部庁舎』が現存。公式の伊良部出張所住所は伊良部字前里添1056番地1(電話0980-78-6250/FAX0980-78-3390)で、旧役場(長浜)から移転している可能性が高い。座標は前里添1056で算出(=伊良部島内)。注意: 一部のWeb要約は旧伊良部町役場の長浜1296と現出張所の前里添1056を混同しているため、公式記載の前里添1056を採用。出典: 伊良部支所 https://www.city.miyakojima.lg.jp/soshiki/shityo/seikatukankyou/irabu/ , 伊良部出張所 https://www.city.miyakojima.lg.jp/kurashi/shisetsu/koukyo/irabu_shucchoujo.html , 市民課 https://www.city.miyakojima.lg.jp/soshiki/shityo/seikatukankyou/shiminseikatu/

---

### 南城市

南城市は2006年(平成18年)1月1日、島尻郡の佐敷町・知念村・玉城村・大里村の1町3村が新設(対等)合併して誕生した(全域が沖縄本島南部の東海岸に位置し、知念村域には久高島・コマカ島などの離島を含む)。合併後しばらくは旧4役場をそれぞれ庁舎とする「分庁方式」で運営され、本庁機能は旧玉城村役場(玉城庁舎、玉城字富里)に置かれた。市民の庁舎間移動の負担と維持管理コストの解消のため新庁舎が建設され、2018年(平成30年)5月28日に新本庁舎(佐敷字新里1870番地)へ移転、同年5月25日をもって玉城庁舎・大里庁舎は閉庁した。【各旧町村の現状】(1)佐敷町=現在の南城市の本庁(新本庁舎)が佐敷地区内の佐敷字新里1870に立地。代表点として本庁が現存。ただし新本庁舎は旧佐敷町役場(佐敷字佐敷56)とは別地点。(2)玉城村=旧玉城村役場(玉城庁舎)は合併直後の本庁だったが2018年閉庁。跡地は株式会社オキナワインターナショナルスクール(玉城富里143)が利活用。役所系施設は廃止・移転。(3)大里村=旧大里村役場(大里庁舎、大里字仲間807)は2018年閉庁。跡地はソニービジネスオペレーションズ株式会社が利用。役所系施設は廃止・移転。(4)知念村=旧知念村役場(久手堅22)は合併後「知念庁舎」となったが、新本庁舎統合は公式に「玉城庁舎・大里庁舎」の2庁舎からと明記されており、知念庁舎の現存・廃止について公式サイト一次情報では明確な裏付けが取れず未確認(現在の証明書発行等の窓口は本庁市民課に集約)。重要な注意点として、旧具志川市(うるま市)・旧具志川村(久米島町)の同名異所は本件と無関係。座標は新本庁舎が複数ソースで一致(Mapion 26.16321/127.77059, ton2net国土地理院由来版 26.16306/127.77056)、旧大里庁舎はMapionで26.17616/127.74462を取得。旧玉城・旧知念は番地住所まで確定だが緯度経度の一次確認値は未取得。

| 旧町村(地域) | 役場/支所の状況 | 施設名 | 緯度 | 経度 | 座標の出典 |
|---|---|---|---|---|---|
| 佐敷町 | 本庁あり | 南城市役所(本庁舎) | 26.16321 | 127.77059 | Mapion(南城市役所POI、住所=佐敷字新里1870と明記: 26.16321208,127.77058668)。ton2net国土地理院由来版でも南城市=26.16306,127.77056で一致。両者整合のため本庁=新本庁舎の代表点として採用 |
| 知念村 | 不明 | 旧称: 南城市役所知念庁舎(旧知念村役場、知念久手堅22)。現況の役所系施設の有無は公式一次情報で未確認 | — | — | 未確認(旧知念村役場の所在地は知念久手堅22番地で確定だが、緯度経度の一次確認値は取得できず) |
| 玉城村 | 廃止 | 旧南城市役所本庁舎(玉城庁舎、旧玉城村役場、玉城字富里) — 2018年閉庁。跡地は株式会社オキナワインターナショナルスクール(玉城富里143) | — | — | 未確認(旧玉城庁舎=玉城字富里、跡地利用のオキナワインターナショナルスクール住所=南城市玉城富里143で確定だが、緯度経度の一次確認値は取得できず) |
| 大里村 | 廃止 | 旧南城市役所大里庁舎(旧大里村役場、大里字仲間807) — 2018年閉庁。跡地はソニービジネスオペレーションズ株式会社 | 26.17616 | 127.74462 | Mapion(旧大里庁舎跡地に入居のソニービジネスオペレーションズ、住所=大里字仲間: 26.17616478,127.74461909)。建物=旧大里庁舎そのものの代表点として採用 |

**地域ごとの補足:**

- **佐敷町**（本庁あり）: 沖縄本島南部・本島側。現在の南城市の本庁(新本庁舎)が佐敷地区内の佐敷字新里1870番地に立地し、2018年5月28日供用開始。代表点として役所系施設(本庁)が現存。ただし新本庁舎は旧佐敷町役場(旧住所=佐敷字佐敷56、合併後は佐敷庁舎)とは別地点である点に注意。代表番号098-917-5309
- **知念村**（不明）: 沖縄本島南部・知念半島先端部の本島側に役場が所在。ただし知念村域は離島の久高島・コマカ島を含む(久高島へは海上交通=高速船/フェリー、安座真港連絡で橋ではない)。合併後は分庁方式で知念庁舎となったが、2018年の新庁舎統合は公式に『玉城庁舎・大里庁舎』の2庁舎からと明記され、知念庁舎の現存/廃止は公式サイトで確認できず未確認。現在の証明書発行等の窓口は本庁(市民課)に集約。混同注意: 旧具志川村(久米島町)とは無関係
- **玉城村**（廃止）: 沖縄本島南部・本島側(玉城地区には橋連絡の離島=奥武島があるが役場は本島側)。合併直後(2006-2018)は旧玉城村役場が南城市の本庁舎(玉城庁舎)だった。平成7年建築・地下1階地上3階・延床4,963㎡。2018年5月25日閉庁、新本庁舎(佐敷字新里)へ移転統合。跡地はオキナワインターナショナルスクールが利活用。役所系施設としては廃止・移転
- **大里村**（廃止）: 沖縄本島南部・本島側。沖縄県では珍しい海に面しない自治体だった。合併後は分庁方式で大里庁舎(平成12年建設・地下1階地上3階・延床5,164㎡)。2018年5月25日閉庁、新本庁舎(佐敷字新里)へ移転統合。跡地はソニービジネスオペレーションズ株式会社が利用(同社住所=南城市大里字仲間807番地)。役所系施設としては廃止・移転。混同注意: 旧具志川市(うるま市)とは無関係

---

### 八重瀬町

沖縄県島尻郡の東風平町と具志頭村が2006年1月1日に新設合併して八重瀬町が誕生した(沖縄本島南部、離島なし)。役場本庁は当初、旧具志頭村役場(字具志頭659)に置かれ、合併から2015年末まで10年間そこが本庁舎だった。2016年1月に東風平地区(字東風平1188番地)に新庁舎が完成し本庁機能が移転、具志頭側は出張所へ格下げ・規模縮小された。旧具志頭村役場跡地には2017年に「南の駅やえせ」が完成し、その施設内に八重瀬町役場具志頭出張所(字具志頭659、旧村役場と同一住所)が現存する。したがって両旧町村とも役場系施設が代表点として使える(東風平=本庁あり、具志頭=出張所あり)。本庁座標は公式サイト(町の位置と地勢)記載の北緯26度09分30秒・東経127度43分07秒。具志頭出張所(南の駅やえせ)座標は地図サービスMapFanの値。なお旧具志川市(現うるま市)・旧具志川村(現久米島町)は本件と無関係の同名異所であり、本調査の具志頭村とは別物。

| 旧町村(地域) | 役場/支所の状況 | 施設名 | 緯度 | 経度 | 座標の出典 |
|---|---|---|---|---|---|
| 東風平町 | 本庁あり | 八重瀬町役場(本庁舎) | 26.15830 | 127.71860 | 公式サイト(八重瀬町「位置と地勢」記載の北緯26度09分30秒・東経127度43分07秒をDMS→10進変換: 26.1583, 127.7186)。所在地は字東風平1188番地。 |
| 具志頭村 | 出張所あり | 八重瀬町役場 具志頭出張所(南の駅やえせ内) | 26.12180 | 127.74250 | 地図サービスMapFan(南の駅やえせ=八重瀬町字具志頭659): 10進26.1218394, 127.7425129。公式の度分秒座標は本庁分のみで具志頭出張所の公式座標は未確認のため地図値を採用。 |

**地域ごとの補足:**

- **東風平町**（本庁あり）: 沖縄本島南部、離島なし。2006-01-01に東風平町と具志頭村が新設合併して八重瀬町に。本庁は当初旧具志頭村役場に置かれたが、2016年1月に東風平地区(字東風平1188番地)へ新庁舎を建設して本庁機能を移転。現在の本庁所在地。〒901-0492、TEL 098-998-2200。
- **具志頭村**（出張所あり）: 沖縄本島南部、離島なし。旧具志頭村役場(字具志頭659)は合併後2015年末まで10年間、八重瀬町の本庁舎だった。2016年1月に本庁が東風平へ移転し具志頭側は出張所へ格下げ・規模縮小。2017年に旧役場敷地に『南の駅やえせ』が完成し、その施設内に具志頭出張所が現存(住所は旧村役場と同一の字具志頭659)。〒901-0592/0512、TEL 098-998-2101。※旧具志川市(うるま市)・旧具志川村(久米島町)とは同名異所の別物で本件と無関係。

---

### 久米島町(沖縄県島尻郡)

沖縄県の平成の大合併第1号として、2002年(平成14)4月1日に久米島内の旧仲里村(島東半部)と旧具志川村(島西半部)が合併し久米島町が成立。当初は旧両村の庁舎を併用する分庁方式を採用した。現状を一次情報(久米島町公式サイト・施設マップ・閉庁告知)で確認した結果: (1)旧仲里村側=現在の「本庁(本庁舎/仲里庁舎)」であり字比嘉2870番地に現存(旧仲里村役場と同一地)。(2)旧具志川村側=合併後「具志川庁舎(支所)」だったが、建物老朽化(築50年超)のため2020年9月25日に閉庁し、9月28日から字仲泊966番地の33の「具志川出張所」へ業務移転して現存。なお重要点として、背景ヒストの「本庁は旧具志川村(嘉手苅付近)」は誤りで、実際の本庁は旧仲里村側の字比嘉にある。旧具志川村(久米島)は旧具志川市(現うるま市)とは全くの別物。両旧村は同一の久米島内(東半部/西半部)にあり橋連絡は不要。

| 旧町村(地域) | 役場/支所の状況 | 施設名 | 緯度 | 経度 | 座標の出典 |
|---|---|---|---|---|---|
| 仲里村 | 本庁あり | 久米島町役場 本庁(本庁舎・仲里庁舎) | 26.34065 | 126.80503 | 公式サイト+Wikipedia(北緯26度20分26秒/東経126度48分18秒)+churatown地図(26.3406496,126.8050282)で一致。旧仲里村役場と同一地点 |
| 具志川村 | 移転 | 久米島町役場 具志川出張所(旧『具志川庁舎(支所)』を2020/9/25閉庁し移転) | 26.34912 | 126.74728 | 旧具志川村役場/具志川庁舎(字仲泊699)の座標=Wikipedia(北緯26度20分57秒/東経126度44分50秒)+Yahoo地図(26.349116,126.747284)。現出張所(字仲泊966-33)のピンポイント緯度経度は未確認だが同じ字仲泊内で近接。代表点としては字仲泊699座標を採用(出張所の正確値は未確認) |

**地域ごとの補足:**

- **仲里村**（本庁あり）: 久米島(離島)の東半部。所在地=〒901-3193 久米島町字比嘉2870番地(旧仲里村役場と同住所)。合併後はこちらが本庁(本庁舎)。背景ヒントの『本庁は旧具志川村(嘉手苅付近)』は誤り——本庁は旧仲里村側の字比嘉にある。代表点として最も確実。同一島内のため橋連絡は不要。
- **具志川村**（移転）: 久米島(離島)の西半部。久米島の具志川村であり、旧具志川市(現うるま市)とは別物(同名異所)——混同注意。合併後は『具志川庁舎(支所)』として分庁方式で使用されたが、建物老朽化(築50年超)で2020年9月25日に閉庁、9月28日から〒901-3192 久米島町字仲泊966番地の33の『具志川出張所(総合窓口/TEL 098-985-2001)』へ業務移転して現存。旧具志川庁舎(字仲泊699)は公式施設マップに『旧具志川庁舎』として残存表示。代表点を使う場合、厳密には現出張所(966-33)が役場系施設だが、座標が確認できたのは字仲泊699(旧庁舎)のみ。嘉手苅は旧具志川村の一字だが役場所在字は仲泊。同一島内のため橋連絡は不要。

---

# 第5部 座標表ドラフト（現時点で判明している代表点）

> 注意: 未合併36地域の役所座標は本調査では未取得（取得は容易・自明）。以下は合併17地域の検証で判明した座標のみ。確度「中」「未確認」は実装時に一次ソースで再確認すること。

| 地域(locality値) | 親市町村 | 状況 | 緯度 | 経度 | 確度 |
|---|---|---|---|---|---|
| 具志川市 | うるま市 | 本庁あり | 26.37926 | 127.85740 | 中(地図サービスDB・住所のみ公式) |
| 石川市 | うるま市 | 出張所あり | 26.42715 | 127.82925 | 高(公式サイト記載の緯度経度) |
| 与那城町 | うるま市 | 出張所あり | 26.32998 | 127.90456 | 中(地図サービスDB・住所のみ公式) |
| 勝連町 | うるま市 | 出張所あり | 26.31502 | 127.89669 | 中(地図サービスDB・住所のみ公式) |
| 平良市 | 宮古島市 | 本庁あり | 24.78982 | 125.29510 | 高(住所ジオコーディング+Wikipedia DMSと一致し検証済) |
| 城辺町 | 宮古島市 | 出張所あり | 24.75799 | 125.38768 | 中(公式住所のジオコーディング) |
| 下地町 | 宮古島市 | 出張所あり | 24.74866 | 125.27787 | 中(公式住所のジオコーディング) |
| 上野村 | 宮古島市 | 出張所あり | 24.73799 | 125.31766 | 中(公式住所のジオコーディング) |
| 伊良部町 | 宮古島市 | 支所あり | 24.83198 | 125.19349 | 中(公式住所のジオコーディング) |
| 佐敷町 | 南城市 | 本庁あり | 26.16321 | 127.77059 | 高(Mapion+国土地理院由来版が一致) |
| 知念村 | 南城市 | 不明 | —(未確認) | —(未確認) | 未確認 |
| 玉城村 | 南城市 | 廃止 | —(未確認) | —(未確認) | 未確認 |
| 大里村 | 南城市 | 廃止 | 26.17616 | 127.74462 | 中(Mapion・一次公式値ではない) |
| 東風平町 | 八重瀬町 | 本庁あり | 26.15830 | 127.71860 | 高(公式サイト記載) |
| 具志頭村 | 八重瀬町 | 出張所あり | 26.12180 | 127.74250 | 中(地図サービス・公式座標は未確認) |
| 仲里村 | 久米島町 | 本庁あり | 26.34065 | 126.80503 | 高(公式+Wikipedia+地図が一致) |
| 具志川村 | 久米島町 | 移転 | 26.34912 | 126.74728 | 中(Wikipedia+地図/旧庁舎値・現出張所のピンポイントは未確認) |

**役場が代表点に使えない/要手当ての地域（すべて南城市）:**

- **玉城村** — 玉城庁舎2018年閉庁(廃止)。座標完全未確認 → 南城市本庁で代用 or 旧庁舎住所(玉城富里)を実装時ジオコーディング。
- **大里村** — 大里庁舎2018年閉庁(廃止)。旧庁舎座標(26.17616/127.74462)はMapion由来=一次公式値でない → 実装時再確認。
- **知念村** — 知念庁舎の現存/廃止が公式未確認。旧役場住所(知念久手堅22)は確定 → 実装時にジオコーディング+現存確認。

**致命的注意 ― 「具志川」同名異所:**

- 具志川市 → うるま市(本島中部)・経度127.8°台
- 具志川村 → 久米島町(久米島・離島)・経度126.7°台（約100km差）
- 座標表作成時は2行を経度で突合検証（取り違えると点が久米島↔本島に飛ぶ）。

---

# 付録 調査メタ情報

- **調査1(天気API実現性)**: 6エージェント(コードベース3=Explore/Haiku、API調査2=Opus、統合1=Opus)。
- **調査2(役場座標検証)**: 6エージェント(合併市町5=Opus、統合1=Opus)。WebSearch/WebFetchで各市町公式サイト・地図サービスを参照。
- **未確認の主要事項**: ①商用ライセンス(go_iotが有償顧客サービスか) ②本案が顧客の確定依頼か(スタンプ側仮説の可能性) ③天気の用途(実況/予報/実測比較) ④玉城・大里・知念の座標一次確認。
- **一次ソースとしての権威**: 実DBスキーマは `docs/database_snapshot/`、地域マスタは `internal/domain/locality.go` を正とする。本レポートの天気API数値・役場座標は調査時点(2026-06-30)のもので、実装時に再確認すること。
