package mdns

import (
	"errors"
	"net"
	"testing"
)

// selectLANIPv4 は (NIC, アドレス群) の候補から最初の「UP かつ非ループバック」NIC の
// 非ループバック IPv4 を選ぶ純粋ロジック。net.Interface.Addrs() を介さずテストする。
func TestSelectLANIPv4(t *testing.T) {
	const up = net.FlagUp
	const loopback = net.FlagUp | net.FlagLoopback
	const down = net.Flags(0)
	ipnet := func(s string) net.Addr { return &net.IPNet{IP: net.ParseIP(s)} }

	tests := []struct {
		name       string
		candidates []ifaceAddrs
		wantIP     string
		wantIface  string
		wantErr    bool
	}{
		{
			name:       "UP 非ループバック NIC の IPv4 を選ぶ",
			candidates: []ifaceAddrs{{&net.Interface{Name: "eth0", Flags: up}, []net.Addr{ipnet("192.168.1.5")}}},
			wantIP:     "192.168.1.5", wantIface: "eth0",
		},
		{
			name: "ループバック NIC はスキップし次の NIC を選ぶ",
			candidates: []ifaceAddrs{
				{&net.Interface{Name: "lo0", Flags: loopback}, []net.Addr{ipnet("127.0.0.1")}},
				{&net.Interface{Name: "wlan0", Flags: up}, []net.Addr{ipnet("10.0.0.2")}},
			},
			wantIP: "10.0.0.2", wantIface: "wlan0",
		},
		{
			name: "down NIC はスキップする",
			candidates: []ifaceAddrs{
				{&net.Interface{Name: "eth0", Flags: down}, []net.Addr{ipnet("192.168.1.5")}},
				{&net.Interface{Name: "wlan0", Flags: up}, []net.Addr{ipnet("10.0.0.2")}},
			},
			wantIP: "10.0.0.2", wantIface: "wlan0",
		},
		{
			name: "IPv6 のみの NIC はスキップする",
			candidates: []ifaceAddrs{
				{&net.Interface{Name: "eth0", Flags: up}, []net.Addr{ipnet("fe80::1")}},
				{&net.Interface{Name: "wlan0", Flags: up}, []net.Addr{ipnet("10.0.0.2")}},
			},
			wantIP: "10.0.0.2", wantIface: "wlan0",
		},
		{
			name:       "UP NIC 内のループバック IPv4 はスキップし非ループバック IPv4 を選ぶ",
			candidates: []ifaceAddrs{{&net.Interface{Name: "eth0", Flags: up}, []net.Addr{ipnet("127.0.0.1"), ipnet("192.168.1.5")}}},
			wantIP:     "192.168.1.5", wantIface: "eth0",
		},
		{
			name:       "該当する NIC が無ければエラー",
			candidates: []ifaceAddrs{{&net.Interface{Name: "lo0", Flags: loopback}, []net.Addr{ipnet("127.0.0.1")}}},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, iface, err := selectLANIPv4(tt.candidates)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("エラーを期待したが nil (ip=%v)", ip)
				}
				return
			}
			if err != nil {
				t.Fatalf("予期せぬエラー: %v", err)
			}
			if ip.String() != tt.wantIP {
				t.Errorf("IP = %v, want %s", ip, tt.wantIP)
			}
			if iface == nil || iface.Name != tt.wantIface {
				t.Errorf("iface = %v, want %s", iface, tt.wantIface)
			}
		})
	}
}

// currentLANIPv4 は interfaces 列挙が失敗したらエラーを返す。
func TestCurrentLANIPv4_ErrorOnInterfacesFailure(t *testing.T) {
	orig := interfaces
	t.Cleanup(func() { interfaces = orig })
	interfaces = func() ([]net.Interface, error) { return nil, errors.New("NIC 取得不可") }

	if _, _, err := currentLANIPv4(); err == nil {
		t.Error("interfaces 列挙失敗時はエラーを返すべき")
	}
}

// currentLANIPv4 の実環境スモーク: パニックせず、返る IP は非ループバック IPv4 であること。
func TestCurrentLANIPv4_Smoke(t *testing.T) {
	ip, iface, err := currentLANIPv4()
	if err != nil {
		t.Skipf("非ループバック IPv4 NIC なし (CI 等): %v", err)
	}
	if ip.To4() == nil || ip.IsLoopback() {
		t.Errorf("非ループバック IPv4 を返すべき: %v", ip)
	}
	if iface == nil {
		t.Error("IP と対の NIC を返すべき")
	}
}
