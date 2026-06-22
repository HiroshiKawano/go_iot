# デバイスファームウェア（ESP8266 + SHT31）

農業IoT のセンサー側ファームウェア。**ESP8266（ESP-WROOM-02 系）+ SHT31 温湿度センサー**で計測した温湿度を、本リポジトリの Go API（`POST /api/sensor-data`）へ HTTPS + Bearer トークンで送信する。

> **位置づけ**: 本リポジトリの主体はバックエンド + Web UI（Go）だが、眞境名さん案件の実機検証用に**参考ファームウェアを同梱**する。サーバ側の契約（エンドポイント・JSON・認証）は [internal/handler/sensor_api.go](../internal/handler/sensor_api.go) / [internal/docs/openapi.yaml](../internal/docs/openapi.yaml) が正。背景・残課題は [2cc_sdd/実装計画.md](../2cc_sdd/実装計画.md) §8、引継ぎ経緯は [2cc_sdd/農業IoT（眞境名さん案件）引継ぎメモ.md](../2cc_sdd/農業IoT（眞境名さん案件）引継ぎメモ.md) を参照。

## 構成

```
firmware/esp8266_sht31/
├── esp8266_sht31.ino    メインスケッチ（SHT31 読取 → JSON → HTTPS POST）
├── config.example.h     設定テンプレート（コミット対象・プレースホルダのみ）
└── config.h             実設定（コピーして作成・.gitignore 済み・コミットしない）
```

## 1. 必要ハードウェア・配線

| 部品 | 備考 |
|---|---|
| ESP8266 / ESP-WROOM-02 系ボード | 当初想定の ESP32 から変更（実機の刻印より確定） |
| SHT31 温湿度センサーモジュール | I2C（既定アドレス 0x44） |
| データ通信対応 Micro USB ケーブル | 充電専用ケーブルだと PC 認識しない（引継ぎメモ §7-1）|

**I2C 配線（NodeMCU 既定ピン。基板に合わせて `config.h` で調整）:**

| SHT31 | ESP8266 |
|---|---|
| VIN / VCC | 3V3 |
| GND | GND |
| SDA | GPIO4（D2）|
| SCL | GPIO5（D1）|

## 2. Arduino IDE のセットアップ

1. **ESP8266 ボード定義を追加**
   - `ファイル → 環境設定 → 追加のボードマネージャURL` に以下を追加:
     ```
     http://arduino.esp8266.com/stable/package_esp8266com_index.json
     ```
   - `ツール → ボード → ボードマネージャ` で **esp8266 by ESP8266 Community** をインストール。
   - ボード: **Generic ESP8266 Module**（または実機に合うもの）。
2. **ポート選択**
   - Windows: `ツール → ポート → COM7` 等。
   - macOS: `ツール → ポート → /dev/cu.usbserial-xxxx`（または `/dev/cu.SLAB_USBtoUART` / `/dev/cu.wchusbserial-xxxx`）。
     認識しない場合は USB シリアル変換チップのドライバ（CH340 / CP210x / FTDI 系）が必要。
3. **ライブラリをインストール**（`スケッチ → ライブラリをインクルード → ライブラリを管理`）
   - **Adafruit SHT31 Library**（依存の **Adafruit BusIO** / **Adafruit Unified Sensor** も一緒に入れる）。
   - ※ JSON は標準 `snprintf` で組み立てるため ArduinoJson は不要。

## 3. 設定（config.h）

`config.example.h` をコピーして `config.h` を作り、値を埋める:

```bash
cp config.example.h config.h
```

| 項目 | 説明 |
|---|---|
| `WIFI_SSID` / `WIFI_PASSWORD` | 接続する Wi-Fi |
| `API_ENDPOINT` | 自前サーバの `https://<host>/api/sensor-data` |
| `API_BEARER` | `make gen-token user=<id> name="..."` で発行した**平文トークン**（発行時のみ表示）|
| `DEVICE_ID` | Web UI のデバイス登録で発番された ID |
| `SHT31_ADDR` / `I2C_SDA` / `I2C_SCL` | センサーのアドレス・I2C ピン |
| `SEND_INTERVAL_MS` | 送信間隔（既定 5 分）|
| `TLS_INSECURE` | 開発は `1`（証明書検証なし）、本番は `0` にしてピン留め等を実装 |

> `config.h` は機密（Wi-Fi パスワード・Bearer トークン）を含むため **`.gitignore` 済み**。コミットしないこと。

### サーバ側の準備（送信先）

1. デバイスを Web UI で登録 → `DEVICE_ID` を確認。
2. トークン発行: `make gen-token user=<ユーザーID> name="ハウスA温湿度計"` → 表示された平文を `API_BEARER` に設定。
3. サーバが HTTPS で到達可能なこと（本番ドメイン + 証明書、または開発用に到達可能な URL）。

## 4. 書き込みと動作確認

### 4-0. 実機の前に: テスト送信でサーバ側を先に確認（推奨）

ESP8266 に書き込む前に、**このファームと同形の JSON を送るテスト CLI** でサーバ側（到達性・トークン・`device_id`・受信〜画面反映〜アラート判定）を検証できる。実機の問題とサーバ/設定の問題を切り分けられる。

```bash
# 1 回だけ送信（固定値）
make sensor-sim token=<make gen-token で発行した平文> device=<DEVICE_ID>
# または直接:
go run ./cmd/sensor-sim -url http://localhost:8080/api/sensor-data -token <平文> -device 1

# 5 分間隔で連続送信（ランダム変動。グラフ/アラートの確認に。Ctrl-C で停止）
go run ./cmd/sensor-sim -token <平文> -device 1 -count 0 -interval 5m -random
```

`201 Created OK` が出て、Web UI のダッシュボード/デバイス詳細に値が反映されれば、サーバ側の準備は完了。ここまで通ってから実機（4-1）へ進む。

### 4-1. 実機への書き込みと確認

1. `config.h` を作成・設定。
2. `esp8266_sht31.ino` を開いてマイコンへ書き込み（必要に応じて FLASH/BOOT + RESET 操作）。
3. **シリアルモニタを 115200 baud** で開く。以下のようなログが出れば成功:
   ```
   [WiFi] connected. IP: 10.x.x.x
   [NTP] synced. epoch=...
   [SHT31] found at 0x44
   [loop] t=27.30 C  h=62.10 %
   [HTTP] POST https://.../api/sensor-data
   [HTTP] 201 Created OK. resp={...}
   ```
4. **Web UI のダッシュボード / デバイス詳細**に値が反映されることを確認（最新値・グラフ）。

### 送信する JSON（サーバ契約）

```json
{
  "device_id": 1,
  "temperature": 27.30,
  "humidity": 62.10,
  "recorded_at": "2026-06-22T15:04:05+09:00"
}
```

| HTTP | 意味（サーバ `sensor_api.go`）|
|---|---|
| 201 | 作成成功（`alerts_fired` を含む）|
| 400 | JSON 形式エラー |
| 401 | Bearer トークン不正・未付与 |
| 403 | 他ユーザーのデバイスへの書き込み |
| 422 | バリデーション違反（範囲外）・存在しない `device_id` |
| 500 | サーバ DB エラー |

## 5. 動作の要点（実装メモ）

- **時刻**: ESP8266 に RTC が無いため **NTP 同期後に** `recorded_at` を付与（JST 固定 `+09:00`）。未同期時はその周期の送信をスキップ（時刻が狂うとサーバ側の通信遅延分析が崩れるため）。
- **センサー失敗**: SHT31 が NaN を返す/範囲外のときは送信せずスキップ。
- **Wi-Fi 断**: 各周期の冒頭で再接続を試行。
- **間隔維持**: 次回送信時刻を周期の先頭で確定し、処理が長引いても間隔を保つ。

## 6. 本番化で検討すること（→ 実装計画 §8-4）

- `TLS_INSECURE=0` にして証明書フィンガープリント固定 or ルート CA 同梱（`setInsecure()` は開発のみ）。
- トークン/`device_id` の実機投入手順と失効運用（`expires_at` / トークン再発行）。
- 計測項目の拡張（CO2・照度・土壌水分等）が出た場合は、サーバ DB スキーマ（現状 `sensor_readings` は temperature/humidity）も併せて拡張。
