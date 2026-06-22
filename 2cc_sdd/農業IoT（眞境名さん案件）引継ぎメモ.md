# 農業IoT（眞境名さん案件）引継ぎメモ

担当引継ぎ先：河野さん向け

## 1. 案件概要

眞境名元次さんより、農業用の温湿度センサー等について、現在利用している Ambient 以外の方法でデータを可視化・保存できないか、という相談がありました。

現在の相談の中心は、以下です。

* ESP8266 / ESP32 系のセンサー機からデータを送信する
* Ambient 以外に、自前サーバーやGoogleスプレッドシートへ保存できないか検討する
* 将来的には、温度・湿度などの農業データをWeb画面でグラフ表示する
* 既存のAmbient運用を壊さず、小さく検証する

現時点では、いきなり本格的な農業IoTサービスを作るというより、
**ESP8266系センサー機から自前PHPサーバへデータを送れるかを確認し、その後DB保存・グラフ表示へ進める小規模実証案件**
として考えています。

---

## 2. 関係者

### 相談元

眞境名 元次さん
沖縄県農業研究センター 農業システム開発班 主任研究員

### スタンプ側

* 當山：これまでの相談・初期検証担当
* 河野さん：今後の技術検証・整理・実装引継ぎ候補

---

## 3. これまでの背景

眞境名さん側では、以下のような農業IoT関連の仕組み・候補が出ています。

* Ambient
* UECS Monitor
* UECS-GEAR
* M5UECSrecorder
* uecsrxdump
* ESP32 → Googleスプレッドシート連携
* 自作ESP8266系センサー機

当初はESP32の可能性も考えていましたが、預かった実機の刻印・構成から、
**ESP-WROOM-02 / ESP8266系の可能性が高い**
と判断しています。

---

## 4. 預かった・確認した機材

確認済みの機材・情報は以下です。

* Ambientテスト機
* 眞境名さんチームの自作センサー機
* ESP-WROOM-02 / ESP8266系モジュール搭載基板
* 温湿度データのAmbient表示例

  * ハウス内外温湿度
  * 事務所テスト用温湿度
  * 「温湿度_東」「温湿度_西」のような2地点計測の画面例あり

---

## 5. 現在のAmbient構成イメージ

現状は、おそらく以下のような構成です。

```text
温湿度センサー
↓
ESP8266系マイコン
↓ Wi-Fi
Ambient
↓
グラフ表示
```

ESP側では、Ambient用ライブラリ等を使い、
`ambient.send()` のような形で送信している可能性があります。

ただし、実際のスケッチ内容は今後確認が必要です。

---

## 6. これまでに検討した構成案

### 案1：Ambient継続

```text
ESP8266
↓
Ambient
↓
グラフ表示
```

メリット：

* すでに動いている
* 導入が簡単
* グラフ表示がすぐできる

デメリット：

* 外部サービス依存
* UIや保存形式の自由度が低い
* 自社サービス化や独自画面化には不向き

---

### 案2：Googleスプレッドシート連携

```text
ESP8266 / ESP32
↓
Google Apps Script
↓
Googleスプレッドシート
↓
グラフ
```

メリット：

* 無料または低コスト
* 共有しやすい
* 小規模実証に向いている

デメリット：

* 長期・大量データには弱い
* グラフや画面の自由度は限定的
* 本格的なサービス化には向きにくい

---

### 案3：自前PHPサーバ

現在、スタンプ側で最も現実的と考えている案です。

```text
ESP8266
↓ HTTPS
PHP API
↓
MySQL / MariaDB
↓
Web画面
↓
Chart.js等でグラフ表示
```

メリット：

* スタンプの得意分野で対応しやすい
* PHP / MySQL で保守しやすい
* さくらレンタルサーバーやVPSで開始可能
* 将来的に画面・CSV・通知・圃場管理へ拡張しやすい

デメリット：

* DB設計とグラフ画面を作る必要がある
* セキュリティ、APIキー、認証、データ整合性を考える必要がある

---

### 案4：UECS系

眞境名さん側から以下の候補が共有されています。

* UECS Monitor
* UECS-GEAR
* M5UECSrecorder
* uecsrxdump

UECSは農業IoTの文脈では本格的ですが、現時点では、
対象の自作センサー機がUECS形式で通信しているのか、
単純にESP8266からAmbientへ送っているだけなのか、
まだ確認が必要です。

まずは、ESP8266から通常のHTTP/HTTPSで自前サーバーへ送れるかを優先して確認しています。

---

## 7. すでに完了している技術検証

ESP8266系と思われる実機について、以下まで確認済みです。

### 7-1. PC認識

* USBケーブル問題があり、最初はCOMポートが出なかった
* 充電専用ケーブルの疑いあり
* データ通信対応のMicro USBケーブルに変更
* COM7 Serial Port として認識成功

### 7-2. Arduino IDE設定

当初、ESP32用設定を入れたが、実機がESP8266系と判明したため変更。

追加したBoards Manager URL：

```text
http://arduino.esp8266.com/stable/package_esp8266com_index.json
```

設定：

* esp8266 by ESP8266 Community をインストール
* ボード：Generic ESP8266 Module
* ポート：COM7

### 7-3. Wi-Fi接続

ESP8266用スケッチでWi-Fi接続成功。

ESP32用：

```cpp
#include <WiFi.h>
```

ではなく、ESP8266用：

```cpp
#include <ESP8266WiFi.h>
```

を使用。

シリアルモニタで以下を確認済み。

```text
WiFi Connected!
IP: 10.77.45.74
```

### 7-4. HTTPS通信

HTTPサーバーではなくHTTPSのみだったため、ESP8266からHTTPS通信を確認。

使用方針：

* `WiFiClientSecure`
* `client.setInsecure()`
* 開発検証として証明書検証を無効化

接続先：

```text
https://tinamini.org/test/test.php?value=123
```

結果：

```text
HTTPS GET: https://tinamini.org/test/test.php?value=123
HTTP code: 200
Response:
OK from server
```

つまり、以下の通信は成功済みです。

```text
ESP8266
↓
自宅Wi-Fi
↓
インターネット
↓
自社Webサーバ HTTPS
↓
PHP
↓
レスポンス取得
```

---

## 8. 現在の到達点

現時点では、以下が確認済みです。

* ESP8266のPC認識
* Arduino IDEからの書き込み
* Wi-Fi接続
* HTTPSで自社WebサーバへGETアクセス
* PHPから `OK from server` のレスポンス取得

これにより、
**Ambientを使わず、ESP8266系センサー機から自前PHPサーバへデータを送信する構成は、技術的には可能な見通し**
と判断できます。

ただし、まだ以下は未実装です。

* 実センサー値の取得
* 実センサー値の送信
* PHP側での値受信処理
* DB保存
* グラフ表示
* CSV出力
* 複数センサー対応
* 本番向けセキュリティ対策

---

## 9. 次に河野さんにお願いしたいこと

### 優先1：現在の実験内容を再現・整理

まず、以下を確認してください。

1. ESP8266がPCで認識できるか
2. Arduino IDEで書き込みできるか
3. Wi-Fi接続スケッチが動くか
4. HTTPSでPHPサーバーへアクセスできるか
5. `value=123` のような固定値を送れるか

目的は、當山側で確認した
**ESP8266 → HTTPS → PHP**
の通信を、河野さん側でも再現できる状態にすることです。

---

### 優先2：固定値をPHP側で受け取る

現在は、PHP側で `OK from server` を返すだけの状態です。

次は、例えば以下のようなURLで値を受け取り、ログに残してください。

```text
https://example.com/iot/ingest.php?device=test01&temp=27.3&hum=62.1
```

まずはDB保存ではなく、JSONLやテキストログ保存でも構いません。

例：

```json
{"device":"test01","temp":27.3,"hum":62.1,"received_at":"2026-xx-xx xx:xx:xx"}
```

---

### 優先3：DB保存

ログ保存が確認できたら、MySQL / MariaDB に保存する形へ進めます。

仮テーブル案：

```sql
CREATE TABLE sensor_logs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  device_id VARCHAR(64) NOT NULL,
  temperature DECIMAL(5,2) NULL,
  humidity DECIMAL(5,2) NULL,
  battery_voltage DECIMAL(5,2) NULL,
  raw_payload JSON NULL,
  received_at DATETIME NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

最初は最低限、以下だけでも大丈夫です。

* device_id
* temperature
* humidity
* received_at

---

### 優先4：簡易グラフ表示

DBに保存できたら、Chart.js等で簡易グラフを表示します。

最初の画面は、以下だけで十分です。

* センサー選択
* 直近24時間の温度グラフ
* 直近24時間の湿度グラフ
* 最新値
* 最終受信日時

画面イメージ：

```text
[センサー：test01]

現在値
温度：27.3℃
湿度：62.1%
最終受信：2026-xx-xx xx:xx

[温度グラフ]
[湿度グラフ]
```

---

## 10. セキュリティ・本番時の注意点

現在のHTTPS検証では、ESP8266側で以下を使っています。

```cpp
client.setInsecure();
```

これは開発検証としては問題ありませんが、本番運用では注意が必要です。

本番時に検討すること：

* APIキーを付ける
* device_idごとに認証キーを持たせる
* 送信元チェックをする
* HTTPS証明書検証をどう扱うか検討する
* 不正な連続送信への対策
* 入力値バリデーション
* DB肥大化対策
* 古いデータのアーカイブ方針

最小構成では、まず以下のようなAPIキー方式が現実的です。

```text
https://example.com/iot/ingest.php?device=test01&key=xxxxx&temp=27.3&hum=62.1
```

---

## 11. 眞境名さんに確認したいこと

次回確認すべき点です。

### 機材

* 実機はESP8266で確定か
* ESP32機もあるのか
* 自作センサー機の台数
* テスト機を継続して借りられるか

### センサー

* 温度・湿度のセンサー型番

  * DHT11
  * DHT22
  * SHT31
  * BME280
  * その他
* CO2、照度、土壌水分、電圧なども取っているか

### 現在の送信方式

* Ambientへ直接送信しているのか
* UECS経由なのか
* MQTTは使っているのか
* Wi-Fi設定はどこに書かれているか
* AmbientのチャネルID、ライトキーはどのように管理しているか

### 今後の希望

* Ambientの代替が欲しいのか
* Ambientと併用したいのか
* データを自分たちで持ちたいのか
* CSV出力が必要か
* 農家さんとURL共有したいのか
* アラート通知が必要か
* 将来的に複数農家・複数圃場に広げたいのか

---

## 12. 開発方針案

最初から大きく作らず、以下の順番で進めるのが安全です。

### Phase 1：通信確認

```text
ESP8266
↓
PHP
↓
OKレスポンス
```

※ここは概ね確認済み。

### Phase 2：値の受信

```text
ESP8266
↓
PHP
↓
JSONL保存
```

### Phase 3：DB保存

```text
ESP8266
↓
PHP
↓
MySQL / MariaDB
```

### Phase 4：簡易表示

```text
DB
↓
PHP画面
↓
Chart.js
```

### Phase 5：運用検討

* 複数センサー
* 圃場一覧
* CSV出力
* アラート通知
* LINE通知
* 農家さん向け共有URL
* 管理画面

---

## 13. 今回の案件でやらない方がよいこと

最初から以下に踏み込むと重くなるため、まずは避けた方がよいです。

* いきなりUECS完全対応
* いきなりMQTT / InfluxDB / Grafana構成
* いきなり商用SaaS化
* いきなり多農家対応
* いきなりアラート・LINE通知まで実装
* いきなりAmbient完全置換

まずは、
**既存Ambient運用を壊さず、自前サーバー側にもデータを送れるか**
を確認するのが安全です。

---

## 14. 河野さんへの最初の依頼内容

まずお願いしたい作業は以下です。

1. これまでの引継ぎメモを読む
2. ESP8266 / ESP-WROOM-02 の開発環境を確認する
3. Arduino IDEでWi-Fi接続スケッチを動かす
4. `https://tinamini.org/test/test.php?value=123` 相当のHTTPS GET通信を再現する
5. PHP側でGET値を受け取り、ログ保存する小さな `ingest.php` を作る
6. 可能ならDB保存まで進める
7. 作業内容・詰まった点・次に必要な情報をメモに残す

---

## 15. 最小ゴール

最初のゴールは、以下です。

```text
ESP8266から
device_id、temperature、humidity を
PHP APIへ送信し、
サーバー側でDB保存し、
Web画面で直近24時間のグラフを表示する。
```

この状態まで行けば、眞境名さんへ
「Ambientを使わない小規模な自前可視化の試作ができました」
と報告できます。

---

## 16. 補足

この案件は、センサーそのものの開発よりも、
**農業現場の環境データを、保存・表示・共有・判断に使える形にすること**
が重要です。

そのため、最初は技術的に凝った構成よりも、
PHP / MySQL / Chart.js を使った、わかりやすく保守しやすい構成を優先します。

最終的には、以下のような展開も考えられます。

* 圃場ごとのデータ管理
* ハウス内外の温湿度比較
* CO2、照度、土壌水分などの追加
* CSV出力
* 警報メール
* LINE通知
* 農家さん向け共有画面
* 県内農家向けの小規模IoT可視化サービス

ただし、現時点ではまず、
**ESP8266 → PHP → DB → グラフ**
の最小実証を優先します。