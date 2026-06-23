// I2C アドレス＆ピン自動探索スキャナ (ESP8266)
// 実機センサーが I2C のどのピン・どのアドレスに居るかを総当たりで特定する。
// 結果を 115200 baud で出力。SHT31=0x44/0x45, BME280=0x76/0x77, etc.
#include <Wire.h>

struct PinPair { uint8_t sda; uint8_t scl; const char* name; };
// ESP8266 で I2C に使える候補 GPIO の組合せ (TX/RX=1/3, 15,16 は除外)
PinPair pairs[] = {
  {4, 5,  "SDA=GPIO4(D2)  SCL=GPIO5(D1)  ★既定"},
  {5, 4,  "SDA=GPIO5(D1)  SCL=GPIO4(D2)  (逆)"},
  {0, 2,  "SDA=GPIO0(D3)  SCL=GPIO2(D4)"},
  {2, 0,  "SDA=GPIO2(D4)  SCL=GPIO0(D3)"},
  {12,14, "SDA=GPIO12(D6) SCL=GPIO14(D5)"},
  {14,12, "SDA=GPIO14(D5) SCL=GPIO12(D6)"},
  {13,12, "SDA=GPIO13(D7) SCL=GPIO12(D6)"},
  {12,13, "SDA=GPIO12(D6) SCL=GPIO13(D7)"},
  {2, 14, "SDA=GPIO2(D4)  SCL=GPIO14(D5)"},
  {4, 14, "SDA=GPIO4(D2)  SCL=GPIO14(D5)"},
};

static void scanPair(uint8_t sda, uint8_t scl, const char* name) {
  Wire.begin(sda, scl);
  Wire.setClock(100000);
  delay(60);
  int found = 0;
  for (uint8_t addr = 1; addr < 127; addr++) {
    Wire.beginTransmission(addr);
    if (Wire.endTransmission() == 0) {
      if (found == 0) Serial.printf(">>> %s\n", name);
      Serial.printf("    ✓ 応答 0x%02X", addr);
      if (addr==0x44||addr==0x45) Serial.print("  (SHT3x の可能性)");
      if (addr==0x76||addr==0x77) Serial.print("  (BME280/BMP280 の可能性)");
      if (addr==0x40)             Serial.print("  (HTU21/Si7021 の可能性)");
      if (addr==0x38||addr==0x39) Serial.print("  (AHT10/AHT20 の可能性)");
      Serial.println();
      found++;
    }
  }
}

void setup() {
  Serial.begin(115200);
  delay(400);
  Serial.println("\n\n================ I2C スキャナ開始 ================");
  Serial.println("(全ピン組合せ x 全アドレスを総当たり)");
  for (auto &p : pairs) scanPair(p.sda, p.scl, p.name);
  Serial.println("================ スキャン完了 ================");
  Serial.println("どこにも ✓ が出なければ I2C 以外(DHT22等)か配線/電源の問題");
}
void loop() {
  delay(8000);
  Serial.println("-- (再スキャンは RESET) --");
}
