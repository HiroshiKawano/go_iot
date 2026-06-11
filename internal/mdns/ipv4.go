package mdns

import (
	"errors"
	"fmt"
	"net"
)

// interfaces は net.Interfaces を差し替え可能にするシーム (テストで列挙失敗を注入する)。
var interfaces = net.Interfaces

// ifaceAddrs は NIC とそのアドレス群の組。selectLANIPv4 を実 NIC 非依存にテストするための単位。
type ifaceAddrs struct {
	iface *net.Interface
	addrs []net.Addr
}

// currentLANIPv4 は現在の非ループバック IPv4 アドレスと、それを持つ NIC を返す。
// 告知 A レコードの IP と mDNS 応答送出インターフェースを揃えるため、IP と NIC を対で返す。
// 該当 NIC が無ければエラー (呼び出し側は非致命としてログのみ)。
func currentLANIPv4() (net.IP, *net.Interface, error) {
	ifaces, err := interfaces()
	if err != nil {
		return nil, nil, fmt.Errorf("NIC 列挙: %w", err)
	}
	candidates := make([]ifaceAddrs, 0, len(ifaces))
	for i := range ifaces {
		addrs, err := ifaces[i].Addrs()
		if err != nil {
			continue
		}
		candidates = append(candidates, ifaceAddrs{iface: &ifaces[i], addrs: addrs})
	}
	return selectLANIPv4(candidates)
}

// selectLANIPv4 は候補から最初の「UP かつ非ループバック」NIC の非ループバック IPv4 を選ぶ。
// 純粋ロジック (副作用なし) で、down NIC・ループバック NIC・IPv6 のみ・ループバック IPv4 を除外する。
func selectLANIPv4(candidates []ifaceAddrs) (net.IP, *net.Interface, error) {
	for _, c := range candidates {
		if c.iface.Flags&net.FlagUp == 0 || c.iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		for _, addr := range c.addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 != nil && !ip4.IsLoopback() {
				return ip4, c.iface, nil
			}
		}
	}
	return nil, nil, errors.New("非ループバック IPv4 アドレスを持つ NIC が見つかりません")
}
