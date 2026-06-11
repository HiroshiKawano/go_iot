package desktop

import (
	"net"
	"testing"
)

// 既定ポートが他プロセスで使用中なら、空きポートを自動採番し、
// 返す実ポートが listener の実ポートと一致する (R5.3, R5.4)。
func TestListen_FallsBackToFreePortWhenOccupied(t *testing.T) {
	// 既定ポートを別 listener で占有する。
	// 実装は ":port" (全 NIC・0.0.0.0) で listen するため、占有も同じワイルドカードで行う
	// (127.0.0.1 等の具体 IP で占有すると SO_REUSEADDR 下で 0.0.0.0:port と共存でき衝突を検知できない)。
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("占有用 listener 取得: %v", err)
	}
	defer occupied.Close()
	preferredPort := occupied.Addr().(*net.TCPAddr).Port

	ln, actualPort, err := Listen(preferredPort)
	if err != nil {
		t.Fatalf("Listen() エラー: %v", err)
	}
	defer ln.Close()

	if actualPort == preferredPort {
		t.Errorf("占有ポート %d がそのまま使われた (空きポート採番されるべき)", preferredPort)
	}
	if actualPort == 0 {
		t.Error("actualPort が 0 (実ポートを返すべき)")
	}
	// 返した実ポートが実際に listen 中のポートと一致する
	lnPort := ln.Addr().(*net.TCPAddr).Port
	if actualPort != lnPort {
		t.Errorf("actualPort %d が listener の実ポート %d と一致しない", actualPort, lnPort)
	}
}

// 既定ポートが空いていればそのポートで listen する。
func TestListen_UsesPreferredPortWhenFree(t *testing.T) {
	// 空きポート番号を取得してすぐ解放する (実装と同じワイルドカードで確保)
	probe, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("プローブ listener 取得: %v", err)
	}
	freePort := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	ln, actualPort, err := Listen(freePort)
	if err != nil {
		t.Fatalf("Listen() エラー: %v", err)
	}
	defer ln.Close()

	if actualPort != freePort {
		t.Errorf("空きポート %d を使うべきだが actualPort = %d", freePort, actualPort)
	}
}
