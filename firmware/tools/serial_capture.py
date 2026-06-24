#!/usr/bin/env python3
# =============================================================================
# ESP8266/ESP32 シリアル採取ツール（Arduino Serial Monitor の代替）
#
# 用途: 起動ログ / I2C スキャン結果 / センサー送信(201)ログ を CLI から採取する。
#       Arduino IDE を開かずに、AI/スクリプトから直接ログを読むためのもの。
#
# 背景（この案件の実機固有の罠。詳細は 2cc_sdd/残作業.md 付録F-7/F-8/F-9）:
#   - USB-シリアルチップ非搭載の基板に外付け FTDI を繋いでログを採る構成。
#   - ポート open 時に DTR/RTS が RST 線を握ると、チップが「無出力」または
#     「リセット保持」になり何も出ない。→ 起動させるため明示的にリセットパルスを打つ。
#   - ただし自動リセットの極性が基板により不定。**物理 RST ボタンが最も確実**
#     （その場合 --reset 無しで本ツールを起動 → ボタンを押す）。
#   - 日本語の終了マーカーは .encode() で渡す（Python の bytes リテラル b"..." は
#     非 ASCII を含められず SyntaxError になるため）。
#
# 依存: pyserial（esptool を入れた venv に同梱される）
#   python3 -m venv esptool-venv && esptool-venv/bin/pip install esptool
#
# 例:
#   esptool-venv/bin/python firmware/tools/serial_capture.py \
#       --port /dev/cu.usbserial-XXXX --seconds 80 --reset \
#       --until "201 Created OK" --until "not found" --until "スキャン完了"
#
# ポートの探し方（macOS）: ls /dev/cu.*
#   /dev/cu.usbserial-XXXX = FTDI（標準ドライバで即認識）
#   /dev/cu.SLAB_USBtoUART = CP210x（Silicon Labs ドライバ要）
#   /dev/cu.wchusbserial*  = CH340（WCH ドライバ要）
# =============================================================================
import argparse
import sys
import time

try:
    import serial  # pyserial
except ImportError:
    sys.exit("pyserial が必要です: python3 -m venv venv && venv/bin/pip install esptool pyserial")


def reset_pulse(s):
    """RST を一発叩いて通常起動させる。DTR/RTS どちらが RST か不定なので両方試す。
    GPIO0 を Low に引かないよう（=書込モードに誤って入らないよう）両線を個別に扱う。"""
    s.setDTR(False); s.setRTS(False); time.sleep(0.1)
    s.setRTS(True);  time.sleep(0.25); s.setRTS(False); time.sleep(0.3)  # RTS=RST 仮定
    s.setDTR(True);  time.sleep(0.25); s.setDTR(False); time.sleep(0.3)  # DTR=RST 仮定


def main():
    ap = argparse.ArgumentParser(description="ESP シリアル採取（Serial Monitor 代替）")
    ap.add_argument("--port", required=True, help="例: /dev/cu.usbserial-XXXX")
    ap.add_argument("--baud", type=int, default=115200)
    ap.add_argument("--seconds", type=float, default=80, help="最大採取秒数")
    ap.add_argument("--reset", action="store_true",
                    help="開始時にリセットパルスを打つ（物理 RST ボタンを押すなら不要）")
    ap.add_argument("--until", action="append", default=[],
                    help="この文字列が出たら早期終了（複数指定可・日本語可）")
    a = ap.parse_args()

    markers = [m.encode("utf-8") for m in a.until]  # 日本語は encode 必須
    s = serial.Serial(a.port, a.baud, timeout=0.2)
    s.setDTR(False); s.setRTS(False)  # 線を解放（RST を握らない）
    if a.reset:
        reset_pulse(s)

    buf = b""
    t0 = time.time()
    repulsed = False
    while time.time() - t0 < a.seconds:
        d = s.read(512)
        if d:
            buf += d
        # --reset 指定で無音が続くなら 1 回だけ再パルス（取りこぼし保険）
        if a.reset and not repulsed and not buf.strip() and time.time() - t0 > 20:
            reset_pulse(s)
            repulsed = True
        # マーカーが見えたら、続きを少し拾って終了
        if markers and any(m in buf for m in markers) and time.time() - t0 > 4:
            time.sleep(1.5)
            buf += s.read(8192)
            break
    s.close()

    txt = buf.decode("utf-8", "replace")
    print(txt if txt.strip()
          else "(無出力 — 物理 RESET ボタンを押す / 書込モード用ボタンとの割当を確認)")


if __name__ == "__main__":
    main()
