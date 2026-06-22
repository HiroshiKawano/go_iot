// =============================================================================
// 設定テンプレート — このファイルを config.h にコピーして値を埋めること。
//
//   cp config.example.h config.h
//
// config.h は機密 (Wi-Fi パスワード・Bearer トークン) を含むため
// .gitignore 済み (コミットされない)。本ファイル (example) はプレースホルダのみ。
// =============================================================================
#ifndef CONFIG_H
#define CONFIG_H

// --- Wi-Fi ---
#define WIFI_SSID      "YOUR_WIFI_SSID"
#define WIFI_PASSWORD  "YOUR_WIFI_PASSWORD"

// --- 送信先 Go API ---
// 自前サーバの POST /api/sensor-data を HTTPS で指定する。
// 例: "https://your-server.example.com/api/sensor-data"
#define API_ENDPOINT   "https://YOUR_SERVER/api/sensor-data"

// Bearer トークン (平文)。サーバ側で発行する:
//   make gen-token user=<ユーザーID> name="ハウスA温湿度計"
// 表示された平文を貼り付ける (平文は発行時の1回しか表示されない)。
#define API_BEARER     "YOUR_PLAINTEXT_TOKEN"

// このデバイスの device_id。Web UI のデバイス登録画面で発番された ID。
#define DEVICE_ID      1

// --- 送信間隔 ---
// 既定 5 分。seed データと同じ粒度。
#define SEND_INTERVAL_MS  (5UL * 60UL * 1000UL)

// --- SHT31 (I2C) ---
// 既定アドレスは 0x44。基板の ADDR ピンが High なら 0x45。
#define SHT31_ADDR     0x44
// ESP8266 の I2C ピン (NodeMCU 既定: SDA=GPIO4/D2, SCL=GPIO5/D1)。
// 基板に合わせて変更すること。
#define I2C_SDA        4
#define I2C_SCL        5

// --- ネットワークのタイムアウト / NTP ---
#define WIFI_TIMEOUT_MS  20000UL
#define NTP_TIMEOUT_MS   15000UL
#define NTP_SERVER_1     "ntp.nict.jp"
#define NTP_SERVER_2     "pool.ntp.org"

// --- TLS ---
// 1 = 証明書検証を無効化 (開発用)。本番は 0 にして証明書ピン留め等を実装すること。
#define TLS_INSECURE   1

#endif // CONFIG_H
