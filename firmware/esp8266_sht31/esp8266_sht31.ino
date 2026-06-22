// =============================================================================
// 農業IoT デバイスファーム — ESP8266 (ESP-WROOM-02) + SHT31 温湿度センサー
//
// 役割:
//   1. SHT31 から I2C で温湿度を取得する
//   2. 自前 Go API (POST /api/sensor-data) へ JSON + Bearer トークンで送信する
//
// 送信先の契約 (サーバ側 internal/handler/sensor_api.go が正):
//   POST {API_ENDPOINT}
//   Authorization: Bearer <平文トークン>
//   Content-Type: application/json
//   body: {"device_id":1,"temperature":27.30,"humidity":62.10,
//          "recorded_at":"2026-06-22T15:04:05+09:00"}
//   正常: 201 Created。エラー: 400/401/403/422/500。
//
// 設定 (Wi-Fi/エンドポイント/トークン/device_id 等) は config.h に分離する。
//   config.example.h をコピーして config.h を作成すること (config.h は .gitignore 済み)。
//
// 必要ライブラリ (Arduino IDE のライブラリマネージャ):
//   - Adafruit SHT31 Library (依存: Adafruit BusIO / Adafruit Unified Sensor)
//   ※ JSON は標準の snprintf で組み立てるため ArduinoJson は不要。
//
// 詳細な手順は firmware/README.md を参照。
// =============================================================================

#include <ESP8266WiFi.h>
#include <ESP8266HTTPClient.h>
#include <WiFiClientSecure.h>
#include <Wire.h>
#include <Adafruit_SHT31.h>
#include <time.h>

#include "config.h"

// SHT31 ドライバ (既定 I2C アドレスは config.h の SHT31_ADDR)
static Adafruit_SHT31 sht31 = Adafruit_SHT31();

// 次に送信する時刻 (millis ベース)。0 起動直後は即送信。
static unsigned long nextSendAt = 0;

// -----------------------------------------------------------------------------
// Wi-Fi 接続。未接続なら接続を試みる (タイムアウト付き)。
// 戻り値: 接続できたら true。
// -----------------------------------------------------------------------------
static bool connectWiFi() {
  if (WiFi.status() == WL_CONNECTED) {
    return true;
  }

  Serial.printf("[WiFi] connecting to %s ...\n", WIFI_SSID);
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

  const unsigned long deadline = millis() + WIFI_TIMEOUT_MS;
  while (WiFi.status() != WL_CONNECTED && millis() < deadline) {
    delay(500);
    Serial.print(".");
  }
  Serial.println();

  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[WiFi] connect failed (timeout)");
    return false;
  }
  Serial.print("[WiFi] connected. IP: ");
  Serial.println(WiFi.localIP());
  return true;
}

// -----------------------------------------------------------------------------
// NTP で時刻同期する。recorded_at に正しい計測時刻を載せるために必須。
// ESP8266 には RTC が無く、同期前は 1970 年付近の値になるため同期完了を待つ。
// 戻り値: 同期できたら true。
// -----------------------------------------------------------------------------
static bool syncTime() {
  // JST (UTC+9)。サマータイムなし。
  configTime(9 * 3600, 0, NTP_SERVER_1, NTP_SERVER_2);

  Serial.print("[NTP] syncing time");
  const unsigned long deadline = millis() + NTP_TIMEOUT_MS;
  // epoch がしきい値 (2021-01-01 頃) を超えたら同期完了とみなす。
  while (time(nullptr) < 1609459200L && millis() < deadline) {
    delay(300);
    Serial.print(".");
  }
  Serial.println();

  if (time(nullptr) < 1609459200L) {
    Serial.println("[NTP] sync failed (timeout)");
    return false;
  }
  Serial.printf("[NTP] synced. epoch=%ld\n", (long)time(nullptr));
  return true;
}

// -----------------------------------------------------------------------------
// SHT31 から温湿度を読む。
// 戻り値: 読み取り成功かつ値が有効範囲なら true。値は引数経由で返す。
// SHT31 は読み取り失敗時 NaN を返すため isnan で弾く。
// 範囲はサーバ側バリデーション (temperature -40..125 / humidity 0..100) に合わせる。
// -----------------------------------------------------------------------------
static bool readSensor(float &temperature, float &humidity) {
  const float t = sht31.readTemperature();
  const float h = sht31.readHumidity();

  if (isnan(t) || isnan(h)) {
    Serial.println("[SHT31] read failed (NaN)");
    return false;
  }
  if (t < -40.0f || t > 125.0f || h < 0.0f || h > 100.0f) {
    Serial.printf("[SHT31] value out of range: t=%.2f h=%.2f\n", t, h);
    return false;
  }

  temperature = t;
  humidity = h;
  return true;
}

// -----------------------------------------------------------------------------
// 現在時刻を RFC3339 (JST 固定オフセット +09:00) で buf に書き込む。
// 例: 2026-06-22T15:04:05+09:00
// -----------------------------------------------------------------------------
static void buildTimestamp(char *buf, size_t len) {
  const time_t now = time(nullptr);
  struct tm tmInfo;
  localtime_r(&now, &tmInfo);
  strftime(buf, len, "%Y-%m-%dT%H:%M:%S+09:00", &tmInfo);
}

// -----------------------------------------------------------------------------
// 計測値を Go API へ送信する。
// 戻り値: HTTP ステータスコード。ローカル失敗 (begin 失敗等) は負値。
// -----------------------------------------------------------------------------
static int sendReading(float temperature, float humidity) {
  char recordedAt[32];
  buildTimestamp(recordedAt, sizeof(recordedAt));

  // JSON ボディを組み立てる。temperature/humidity は小数2桁 (NUMERIC(5,2) 相当)。
  char body[192];
  snprintf(body, sizeof(body),
           "{\"device_id\":%ld,\"temperature\":%.2f,\"humidity\":%.2f,\"recorded_at\":\"%s\"}",
           (long)DEVICE_ID, temperature, humidity, recordedAt);

  WiFiClientSecure client;
#if TLS_INSECURE
  // ★開発用: 証明書検証を無効化。本番は証明書フィンガープリント固定等に置き換えること。
  client.setInsecure();
#endif
  // BearSSL のメモリ使用を抑える (ESP8266 はヒープが限られるため)。
  client.setBufferSizes(512, 512);

  HTTPClient https;
  if (!https.begin(client, API_ENDPOINT)) {
    Serial.println("[HTTP] begin() failed");
    return -1;
  }
  https.addHeader("Content-Type", "application/json");
  https.addHeader("Authorization", String("Bearer ") + API_BEARER);

  Serial.printf("[HTTP] POST %s\n  body=%s\n", API_ENDPOINT, body);
  const int code = https.POST(reinterpret_cast<const uint8_t *>(body), strlen(body));
  const String resp = https.getString();
  https.end();

  // サーバ契約に沿ってログを出し分ける。
  if (code == 201) {
    Serial.printf("[HTTP] 201 Created OK. resp=%s\n", resp.c_str());
  } else if (code > 0) {
    // 400=JSON不正 / 401=トークン不正 / 403=他ユーザーのdevice / 422=バリデーション・存在しないdevice_id / 500=DB
    Serial.printf("[HTTP] %d (error). resp=%s\n", code, resp.c_str());
  } else {
    Serial.printf("[HTTP] transport error: %s\n", https.errorToString(code).c_str());
  }
  return code;
}

// -----------------------------------------------------------------------------
void setup() {
  Serial.begin(115200);
  delay(200);
  Serial.println();
  Serial.println("=== 農業IoT ESP8266 + SHT31 firmware ===");

  // I2C 初期化 (ESP8266 はピンを明示指定できる)。
  Wire.begin(I2C_SDA, I2C_SCL);
  if (!sht31.begin(SHT31_ADDR)) {
    Serial.printf("[SHT31] not found at 0x%02X. 配線/アドレスを確認してください\n", SHT31_ADDR);
    // センサー未検出でも再起動ループは避け、loop 側で再試行できるよう続行する。
  } else {
    Serial.printf("[SHT31] found at 0x%02X\n", SHT31_ADDR);
  }

  connectWiFi();
  syncTime();
}

// -----------------------------------------------------------------------------
void loop() {
  const unsigned long nowMs = millis();
  if (nowMs < nextSendAt) {
    delay(100);
    return;
  }
  // 次回送信時刻を先に設定 (この周期の処理が長引いても間隔を保つ)。
  nextSendAt = nowMs + SEND_INTERVAL_MS;

  // 前提を順に確認: Wi-Fi → NTP → センサー。いずれか欠ければこの周期はスキップ。
  if (!connectWiFi()) {
    Serial.println("[loop] skip: WiFi unavailable");
    return;
  }
  // 時刻が未同期なら recorded_at が狂うため、ここで再同期を試みる。
  if (time(nullptr) < 1609459200L && !syncTime()) {
    Serial.println("[loop] skip: time not synced");
    return;
  }

  float temperature = 0.0f;
  float humidity = 0.0f;
  if (!readSensor(temperature, humidity)) {
    Serial.println("[loop] skip: sensor read failed");
    return;
  }

  Serial.printf("[loop] t=%.2f C  h=%.2f %%\n", temperature, humidity);
  sendReading(temperature, humidity);
}
