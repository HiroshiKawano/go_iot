// Package mdns は go-iot.local. の mDNS 公開ライフサイクルを提供する。
//
// 安定ホスト名 go-iot.local を LAN へ告知し (A＋SRV)、DHCP による IP 変動を定期検知して
// 停止→再登録することで ESP32 等のデバイスが固定ホスト名で到達できるようにする (R7)。
// hashicorp/mdns (pure-Go) を差替可能な Advertiser interface で隔離し、ライブラリ不確実性を吸収する。
//
// 本パッケージは下位ユーティリティ層であり、handler/service/repository を import しない。
// マルチキャスト (224.0.0.251:5353) は LAN 限定で、インターネットへ情報を送出しない (R7.3)。
package mdns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// HostName は mDNS で公開するベースホスト名 (→ go-iot.local.)。合成ルートから Start へ渡す。
const HostName = "go-iot"

// defaultInterval は LAN IP 変動を検知する定期チェック間隔。
const defaultInterval = 10 * time.Second

// Advertiser は mDNS 公開のライフサイクルを抽象化する (ライブラリ差替の隔離点)。
//
// Start/Stop は同一 goroutine から逐次呼び出すことを前提とする。逐次的な二重 Start (拒否)・
// 二重 Stop (no-op)・Stop 後の再 Start は安全だが、Start と Stop の並行呼び出しは未サポート
// (合成ルートは単一 Start＋defer Stop の逐次利用のみ)。
type Advertiser interface {
	Start(ctx context.Context, hostname string, port int) error
	Stop()
}

// New は hashicorp/mdns ベースの既定実装を返す。
func New() Advertiser {
	return &advertiser{
		interval:  defaultInterval,
		currentIP: currentLANIPv4,
		publish:   publishMDNS,
		newTicker: realTicker,
	}
}

// advertiser は Advertiser の既定実装。
//
// currentIP / publish / newTicker は注入シームで、生成後は不変 (テストでスタブ化する)。
// cancel / done は実行時に書き換わるため mu で保護する。
type advertiser struct {
	interval  time.Duration
	currentIP func() (net.IP, *net.Interface, error)
	publish   func(hostname string, port int, ip net.IP, iface *net.Interface) (publisher, error)
	newTicker func(time.Duration) (<-chan time.Time, func())

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// realTicker は本番用の time.Ticker シーム。
func realTicker(d time.Duration) (<-chan time.Time, func()) {
	t := time.NewTicker(d)
	return t.C, t.Stop
}

// Start は現在の LAN IP で mDNS 公開を開始し、IP 変動を追従する監視ループを起動する。
// 二重 Start は goroutine / 公開のリークを招くため拒否する (再入安全)。
func (a *advertiser) Start(ctx context.Context, hostname string, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		return errors.New("mdns: 既に開始済みです")
	}

	ip, iface, err := a.currentIP()
	if err != nil {
		return fmt.Errorf("LAN IP 取得: %w", err)
	}
	pub, err := a.publish(hostname, port, ip, iface)
	if err != nil {
		return fmt.Errorf("mDNS 公開: %w", err)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	a.cancel = cancel
	a.done = done
	go a.loop(loopCtx, done, hostname, port, pub, ip)
	log.Printf("mDNS 公開開始: %s.local. → %v (NIC %s)", hostname, ip, iface.Name)
	return nil
}

// loop は ctx がキャンセルされるまで定期的に IP を確認し、変化していれば再登録する。
// 停止時は最新の公開を Shutdown する (再公開失敗で nil になりうるため nil ガードする)。
func (a *advertiser) loop(ctx context.Context, done chan struct{}, hostname string, port int, pub publisher, lastIP net.IP) {
	defer close(done)
	tick, stop := a.newTicker(a.interval)
	defer stop()
	for {
		select {
		case <-ctx.Done():
			if pub != nil {
				_ = pub.Shutdown()
			}
			return
		case <-tick:
			pub, lastIP = a.reregisterIfChanged(hostname, port, pub, lastIP)
		}
	}
}

// reregisterIfChanged は現在 IP を取得し、必要なら旧公開を停止して新 IP で再公開する。
//
// 遷移: (1) IP 取得失敗→現公開を維持 (2) 公開中かつ IP 不変→何もしない
// (3) IP 変化→旧停止＋新公開 (4) 再公開失敗→nil 返却 (次 tick で再試行)。
// current==nil (前回失敗で公開消失) のときは IP 不変でも再公開を試み、公開消失の固着を避ける。
func (a *advertiser) reregisterIfChanged(hostname string, port int, current publisher, lastIP net.IP) (publisher, net.IP) {
	ip, iface, err := a.currentIP()
	if err != nil {
		log.Printf("mDNS: LAN IP 取得失敗（現公開を維持）: %v", err)
		return current, lastIP
	}
	if current != nil && ip.Equal(lastIP) {
		return current, lastIP
	}
	if current != nil {
		_ = current.Shutdown()
	}
	np, err := a.publish(hostname, port, ip, iface)
	if err != nil {
		log.Printf("mDNS: 公開に失敗（次回 tick で再試行）: %v", err)
		return nil, ip
	}
	log.Printf("mDNS: 公開を更新しました → %v", ip)
	return np, ip
}

// Stop は監視ループを停止し、公開を取り下げる。未 Start・二重 Stop でも安全 (no-op)。
// cancel/done をロック内でコピー＆nil 化し、ロック外で待つことでデッドロックを避ける。
func (a *advertiser) Stop() {
	a.mu.Lock()
	cancel := a.cancel
	done := a.done
	a.cancel = nil
	a.done = nil
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
