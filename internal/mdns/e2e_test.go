package mdns

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// TestE2E_RealMulticastAResponse は実マルチキャストで go-iot.local. の A 応答を検証する。
//
// flaky/CI 非対応 (実 224.0.0.251:5353 を bind し FW・複数 NIC・他レスポンダの影響を受ける) のため
// 環境変数ガードでデフォルト Skip とする。Skip 理由と有効化方法は t.Skip に記し、`go test -v` 実行時に
// 表示される (非 -v の `go test`/`make test` では 'ok' のみで理由は出ない点に注意)。実機 E2E は別フェーズ。
func TestE2E_RealMulticastAResponse(t *testing.T) {
	if os.Getenv("GO_IOT_MDNS_E2E") != "1" {
		t.Skip("実マルチキャスト疎通テストはデフォルト無効。有効化: GO_IOT_MDNS_E2E=1 go test ./internal/mdns/ -run TestE2E_RealMulticastAResponse")
	}

	wantIP, _, err := currentLANIPv4()
	if err != nil {
		t.Skipf("非ループバック IPv4 NIC なし: %v", err)
	}

	adv := New()
	if err := adv.Start(context.Background(), "go-iot", 18080); err != nil {
		t.Fatalf("mDNS Start: %v", err)
	}
	t.Cleanup(adv.Stop)
	time.Sleep(300 * time.Millisecond) // 告知の立ち上がりを待つ

	ips := queryMulticastA(t, "go-iot.local.", 2*time.Second)
	for _, ip := range ips {
		if ip.Equal(wantIP) {
			return // 期待 IP の A 応答を確認
		}
	}
	t.Errorf("go-iot.local. の A 応答に現在 LAN IP %v が含まれない: %v", wantIP, ips)
}

// queryMulticastA は go-iot.local. の A レコードを mDNS マルチキャストで問い合わせ、
// 応答に含まれる A アドレスを返す。Qtype は必ず A を指定する
// (hashicorp/mdns は HostName 宛 ANY クエリには応答しないため)。
func queryMulticastA(t *testing.T, name string, timeout time.Duration) []net.IP {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		t.Fatalf("UDP listen: %v", err)
	}
	defer conn.Close()

	msg := new(dns.Msg)
	msg.SetQuestion(name, dns.TypeA)
	buf, err := msg.Pack()
	if err != nil {
		t.Fatalf("DNS pack: %v", err)
	}
	if _, err := conn.WriteToUDP(buf, &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: 5353}); err != nil {
		t.Fatalf("mDNS クエリ送信: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	var ips []net.IP
	resp := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFromUDP(resp)
		if err != nil {
			break // タイムアウトで終了
		}
		var r dns.Msg
		if r.Unpack(resp[:n]) != nil {
			continue
		}
		for _, ans := range r.Answer {
			if a, ok := ans.(*dns.A); ok && a.Hdr.Name == name {
				ips = append(ips, a.A)
			}
		}
		if len(ips) > 0 {
			break
		}
	}
	return ips
}
