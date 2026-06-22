・GPTより
Macでも基本は同じで、**Arduino IDE** を使うよ。
画面もだいたい同じだけど、違うのは主に **ポート名** と **ドライバ**。

Windowsだと今は、

```text
COM7
```

みたいに出ているけど、Macだとだいたいこう出る。

```text
/dev/cu.usbserial-xxxx
/dev/cu.wchusbserial-xxxx
/dev/cu.SLAB_USBtoUART
/dev/cu.usbmodemxxxx
```

ESP8266系のUSBシリアル変換チップによって名前が変わる。

流れはMacでも同じ。

```text
1. Arduino IDEをMacにインストール
2. ESP8266ボードマネージャURLを追加
3. esp8266 by ESP8266 Community をインストール
4. Board：Generic ESP8266 Module
5. Port：/dev/cu.xxx を選択
6. Upload
7. Serial Monitorで確認
```

Windowsとの違いで注意するのはここ。

```text
Windows：
COM7 など

Mac：
/dev/cu.usbserial...
/dev/cu.wchusbserial...
/dev/cu.SLAB_USBtoUART...
```

あと、Macで認識しない場合は、USBシリアル変換チップのドライバが必要なことがある。

よくあるのはこのへん。

```text
CH340 / CH341 系
CP210x 系
FTDI 系
```

今回の基板がどれかはまだ不明だけど、WindowsでCOM7として見えているなら、Macでもドライバが合えば `/dev/cu...` として出るはず。

つまり河野さんがMacでやる場合は、引継ぎメモの

```text
Tools → Port → COM7
```

のところを、

```text
Tools → Port → /dev/cu.usbserial-xxxx 等
```

に読み替えればOK。

あと、書き込み時の **FLASH/BOOT + RESET 操作** はMacでも同じ。
Arduino IDEがWindowsかMacかではなく、ESP8266基板側の問題だからね。


下記はROMメモリー内のサンプルプログラムです
=========================================================


#include <ESP8266WiFi.h>
#include <ESP8266HTTPClient.h>
#include <WiFiClientSecure.h>

const char* ssid     = "elecom-a01c07";   // 自宅WiFi名
const char* password = "35dkjtk5ceur";    // パスワード

void setup() {
  Serial.begin(115200);
  delay(1000);

  Serial.println();
  Serial.println("Connecting WiFi...");
  WiFi.begin(ssid, password);

  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }

  Serial.println();
  Serial.println("WiFi Connected!");
  Serial.print("IP: ");
  Serial.println(WiFi.localIP());

  // ===== ここから HTTPS アクセス =====
  if (WiFi.status() == WL_CONNECTED) {

    WiFiClientSecure client;
    client.setInsecure();   // ★開発用：証明書チェックを無効化（あとで直そうにゃ）

    HTTPClient https;

    // ★↓ここを當山サーバのURLに書き換え
    String url = "https://tinamini.org/test/test.php?value=123";

    Serial.print("HTTPS GET: ");
    Serial.println(url);

    if (https.begin(client, url)) {
      int httpCode = https.GET();

      if (httpCode > 0) {
        Serial.printf("HTTP code: %d\n", httpCode);
        String payload = https.getString();
        Serial.println("Response:");
        Serial.println(payload);
      } else {
        Serial.printf("HTTPS error: %s\n", https.errorToString(httpCode).c_str());
      }

      https.end();
    } else {
      Serial.println("HTTPS begin() failed");
    }
  }
}

void loop() {
  // 今は何もしない
}