// Package desktop はデスクトップ起動 UX (ポート自動採番・既定ブラウザ自動オープン) を提供する。
//
// 本パッケージは下位ユーティリティ層であり、handler/service/repository を import しない。
// 配線は合成ルート cmd/server が行う。すべて pure-Go (CGO 不使用) で単一 .exe を維持する。
package desktop

import (
	"fmt"
	"net"
)

// Listen は preferredPort での listen を試み、使用中なら空きポート (:0) を自動採番して listen する。
//
// 全インターフェース (":port") で待ち受け、同一 LAN のデバイス (ESP32) から到達可能にする (R6)。
// 既定ポートが他プロセスで使用中なら空きポートへフォールバックして起動を継続する (R5.3)。
// 返した net.Listener は呼び出し側が http.Server.Serve へ渡す。actualPort は実際に listen 中の
// ポートで、呼び出し側がログ出力・ブラウザ URL に用いる (R5.4)。
func Listen(preferredPort int) (net.Listener, int, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", preferredPort))
	if err != nil {
		// 既定ポートが使用中等で失敗 → 空きポートを自動取得 (R5.3)
		firstErr := err
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			// 両方失敗した場合のみ、診断のため既定ポート側の失敗理由も併記する
			return nil, 0, fmt.Errorf("空きポートの listen に失敗 (既定 %d も失敗: %v): %w", preferredPort, firstErr, err)
		}
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port, nil
}
