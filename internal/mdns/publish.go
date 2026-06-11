package mdns

import (
	"fmt"
	"net"

	mdnslib "github.com/hashicorp/mdns"
)

const (
	// serviceName は DNS-SD サービス名。NewMDNSService は trimDot するため末尾ドットは不要。
	serviceName = "_http._tcp"
	// domainName は mDNS ドメイン。末尾ドット付き FQDN を構成するため "local." とする。
	domainName = "local."
)

// publisher は単一 IP に対する mDNS 公開ライフサイクルの最小抽象。
// hashicorp/mdns の *Server (Shutdown() error) が満たす。テストでは fake に差し替える。
type publisher interface {
	Shutdown() error
}

// newService は go-iot.local. の A＋SRV を告知する MDNSService を構築する。
//
// hostName は末尾ドット付き FQDN へ正規化する (hashicorp/mdns の validateFQDN が末尾ドット無しを
// 拒否するため)。ips は明示渡しする (空だと NewMDNSService が net.LookupIP に落ち、意図しない IP を
// 引くため)。service/domain は内部で trimDot されるため末尾ドット不要、hostName のみ末尾ドット必須。
func newService(hostname string, port int, ip net.IP) (*mdnslib.MDNSService, error) {
	fqdn := hostname + "." + domainName // 例: "go-iot" + "." + "local." → "go-iot.local."
	return mdnslib.NewMDNSService(hostname, serviceName, domainName, fqdn, port, []net.IP{ip}, nil)
}

// publishMDNS は指定 NIC で go-iot.local. を mDNS 公開する Server を起動する。
//
// 告知 A レコードの IP と応答送出インターフェースを揃えるため、IP を選んだ NIC を Config.Iface に
// 明示指定する (nil 指定だと OS 既定 multicast IF に落ち、マルチ NIC 環境で告知 IP と応答 IF が
// 食い違う)。iface が nil の場合はエラーとし、暗黙フォールバックを許さない。
// Logger は未指定 (hashicorp/mdns 既定の log.Default()) とし、起動側で差し替えた applog 経由で
// 診断ログをファイルへ集約する (io.Discard だと無反応時の切り分け手段を失うため)。
func publishMDNS(hostname string, port int, ip net.IP, iface *net.Interface) (publisher, error) {
	if iface == nil {
		return nil, fmt.Errorf("mDNS 公開には IPv4 を持つ非ループバック NIC が必要です")
	}
	svc, err := newService(hostname, port, ip)
	if err != nil {
		return nil, fmt.Errorf("mDNS サービス構築: %w", err)
	}
	server, err := mdnslib.NewServer(&mdnslib.Config{Zone: svc, Iface: iface})
	if err != nil {
		return nil, fmt.Errorf("mDNS サーバ起動: %w", err)
	}
	return server, nil
}
