package mdns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

// newService は go-iot.local. の A レコードを明示 IP で告知する MDNSService を構築する。
// 実 socket (NewServer) を経ずに Records() で A 応答契約を検証する (observable完了b の代替根拠)。
func TestNewService_AdvertisesHostAAtIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.50").To4()

	svc, err := newService("go-iot", 8080, ip)
	if err != nil {
		t.Fatalf("newService: %v", err)
	}

	if svc.HostName != "go-iot.local." {
		t.Errorf("HostName = %q, want go-iot.local. (末尾ドット正規化)", svc.HostName)
	}
	if svc.Port != 8080 {
		t.Errorf("Port = %d, want 8080", svc.Port)
	}
	// ips を明示しているため net.LookupIP に落ちず指定 IP そのものが入る
	if len(svc.IPs) != 1 || !svc.IPs[0].Equal(ip) {
		t.Errorf("IPs = %v, want [%v] (LookupIP に落ちていないこと)", svc.IPs, ip)
	}

	// A 応答契約: go-iot.local. への A クエリで A レコード (IP 一致) が返る。
	// hashicorp/mdns は HostName 宛 A/AAAA のみ応答するため Qtype は必ず A を指定する。
	recs := svc.Records(dns.Question{Name: "go-iot.local.", Qtype: dns.TypeA})
	if !hasARecord(recs, "go-iot.local.", ip) {
		t.Errorf("go-iot.local. への A クエリで期待 A レコードが返らない: %v", recs)
	}
}

// design の「A＋SRV 告知」契約のうち SRV 側を固定する。SRV はインスタンスアドレス
// (go-iot._http._tcp.local.) への SRV クエリで返り、Port=実ポート・Target=go-iot.local. を含む。
// これによりサービス名・Port・Target の回帰 (例 serviceName 誤り) を検出できる。
func TestNewService_AdvertisesSRVWithPortAndTarget(t *testing.T) {
	svc, err := newService("go-iot", 8080, net.ParseIP("192.168.1.50").To4())
	if err != nil {
		t.Fatalf("newService: %v", err)
	}

	recs := svc.Records(dns.Question{Name: "go-iot._http._tcp.local.", Qtype: dns.TypeSRV})
	var srv *dns.SRV
	for _, rr := range recs {
		if s, ok := rr.(*dns.SRV); ok {
			srv = s
			break
		}
	}
	if srv == nil {
		t.Fatalf("インスタンスアドレスへの SRV クエリで SRV レコードが返らない: %v", recs)
	}
	if srv.Port != 8080 {
		t.Errorf("SRV Port = %d, want 8080", srv.Port)
	}
	if srv.Target != "go-iot.local." {
		t.Errorf("SRV Target = %q, want go-iot.local.", srv.Target)
	}
}

func hasARecord(recs []dns.RR, name string, ip net.IP) bool {
	for _, rr := range recs {
		if a, ok := rr.(*dns.A); ok && a.Hdr.Name == name && a.A.Equal(ip) {
			return true
		}
	}
	return false
}

// iface=nil では publishMDNS はエラーを返す (暗黙の既定 multicast IF フォールバックを許さない)。
// 告知 IP の NIC と応答送出 IF を揃えるため、NIC は必須とする。
func TestPublishMDNS_ErrorWhenNilInterface(t *testing.T) {
	_, err := publishMDNS("go-iot", 8080, net.ParseIP("192.168.1.50").To4(), nil)
	if err == nil {
		t.Error("iface=nil では publishMDNS はエラーを返すべき (暗黙フォールバック禁止)")
	}
}
