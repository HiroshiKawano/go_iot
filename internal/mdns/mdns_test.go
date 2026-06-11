package mdns

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// fakePublisher は publisher のスパイ。Shutdown 呼び出し回数を -race 安全に記録する。
type fakePublisher struct {
	mu            sync.Mutex
	shutdownCalls int
}

func (f *fakePublisher) Shutdown() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCalls++
	return nil
}

func (f *fakePublisher) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.shutdownCalls
}

func ipOf(s string) net.IP { return net.ParseIP(s).To4() }

var testIface = &net.Interface{Name: "eth0", Flags: net.FlagUp}

func ipProvider(ip net.IP) func() (net.IP, *net.Interface, error) {
	return func() (net.IP, *net.Interface, error) { return ip, testIface, nil }
}

// --- reregisterIfChanged: 状態遷移の中核 (observable完了a) ---

func TestReregisterIfChanged_IPUnchanged_NoOp(t *testing.T) {
	ip := ipOf("10.0.0.1")
	cur := &fakePublisher{}
	a := &advertiser{
		currentIP: ipProvider(ip),
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			t.Fatal("IP 不変時に publish を呼んではならない")
			return nil, nil
		},
	}
	gotPub, gotIP := a.reregisterIfChanged("go-iot", 8080, cur, ip)
	if gotPub != cur || !gotIP.Equal(ip) {
		t.Errorf("IP 不変時は現状維持すべき: pub=%v ip=%v", gotPub, gotIP)
	}
	if cur.calls() != 0 {
		t.Errorf("IP 不変時に Shutdown を呼んではならない: %d", cur.calls())
	}
}

func TestReregisterIfChanged_IPChanged_ReregisterAndShutdownOld(t *testing.T) {
	oldIP, newIP := ipOf("10.0.0.1"), ipOf("10.0.0.2")
	cur := &fakePublisher{}
	newPub := &fakePublisher{}
	a := &advertiser{
		currentIP: ipProvider(newIP),
		publish: func(_ string, _ int, ip net.IP, _ *net.Interface) (publisher, error) {
			if !ip.Equal(newIP) {
				t.Errorf("新 IP %v で publish すべきだが %v", newIP, ip)
			}
			return newPub, nil
		},
	}
	gotPub, gotIP := a.reregisterIfChanged("go-iot", 8080, cur, oldIP)
	if cur.calls() != 1 {
		t.Errorf("IP 変化時は旧公開を 1 回 Shutdown すべき: %d", cur.calls())
	}
	if gotPub != publisher(newPub) || !gotIP.Equal(newIP) {
		t.Errorf("新公開へ更新すべき: pub=%v ip=%v", gotPub, gotIP)
	}
}

func TestReregisterIfChanged_GetIPFails_KeepCurrent(t *testing.T) {
	ip := ipOf("10.0.0.1")
	cur := &fakePublisher{}
	a := &advertiser{
		currentIP: func() (net.IP, *net.Interface, error) { return nil, nil, errors.New("取得失敗") },
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			t.Fatal("IP 取得失敗時に publish を呼んではならない")
			return nil, nil
		},
	}
	gotPub, gotIP := a.reregisterIfChanged("go-iot", 8080, cur, ip)
	if gotPub != cur || !gotIP.Equal(ip) {
		t.Errorf("IP 取得失敗時は現公開を維持すべき")
	}
	if cur.calls() != 0 {
		t.Error("IP 取得失敗時に Shutdown を呼んではならない")
	}
}

func TestReregisterIfChanged_PublishFails_ReturnsNil(t *testing.T) {
	oldIP, newIP := ipOf("10.0.0.1"), ipOf("10.0.0.2")
	cur := &fakePublisher{}
	a := &advertiser{
		currentIP: ipProvider(newIP),
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			return nil, errors.New("公開失敗")
		},
	}
	gotPub, gotIP := a.reregisterIfChanged("go-iot", 8080, cur, oldIP)
	if cur.calls() != 1 {
		t.Errorf("IP 変化時は旧公開を Shutdown すべき: %d", cur.calls())
	}
	if gotPub != nil {
		t.Error("再公開失敗時は nil を返すべき")
	}
	if !gotIP.Equal(newIP) {
		t.Errorf("IP は新 IP に更新すべき: %v", gotIP)
	}
}

// 回復: 前回失敗で公開消失 (current==nil) なら IP 不変でも再公開を試みる (high 指摘の固着回避)。
func TestReregisterIfChanged_RecoversFromNilPublisher(t *testing.T) {
	ip := ipOf("10.0.0.1")
	newPub := &fakePublisher{}
	published := false
	a := &advertiser{
		currentIP: ipProvider(ip),
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			published = true
			return newPub, nil
		},
	}
	gotPub, gotIP := a.reregisterIfChanged("go-iot", 8080, nil, ip)
	if !published {
		t.Error("current==nil なら IP 不変でも再公開を試みるべき (固着回避)")
	}
	if gotPub != publisher(newPub) || !gotIP.Equal(ip) {
		t.Errorf("回復後は新公開へ更新すべき: pub=%v ip=%v", gotPub, gotIP)
	}
}

// --- loop: ticker シームで決定論的に検証 ---

func TestLoop_ReregisterOnTick_ShutdownLatestOnCancel(t *testing.T) {
	tickCh := make(chan time.Time)
	stopCalled := false
	ip1, ip2 := ipOf("10.0.0.1"), ipOf("10.0.0.2")
	initialPub, newPub := &fakePublisher{}, &fakePublisher{}

	published := make(chan net.IP, 1)
	a := &advertiser{
		currentIP: ipProvider(ip2), // tick 時に新 IP を返す
		publish: func(_ string, _ int, ip net.IP, _ *net.Interface) (publisher, error) {
			published <- ip
			return newPub, nil
		},
		newTicker: func(time.Duration) (<-chan time.Time, func()) {
			return tickCh, func() { stopCalled = true }
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go a.loop(ctx, done, "go-iot", 8080, initialPub, ip1)

	// tick 発火 → IP 変化検知 → 旧 Shutdown + 新 publish
	tickCh <- time.Time{}
	gotIP := <-published
	if !gotIP.Equal(ip2) {
		t.Errorf("tick で新 IP %v を publish すべきだが %v", ip2, gotIP)
	}
	if initialPub.calls() != 1 {
		t.Errorf("再登録時に旧公開を Shutdown すべき: %d", initialPub.calls())
	}

	// cancel → loop は最新 pub (newPub) を Shutdown して終了
	cancel()
	<-done
	if newPub.calls() != 1 {
		t.Errorf("cancel 時に最新公開を Shutdown すべき: %d", newPub.calls())
	}
	if !stopCalled {
		t.Error("loop 終了時に ticker を停止すべき")
	}
}

// --- Start / Stop: ライフサイクル ---

func TestStart_RejectsDoubleStart(t *testing.T) {
	a := newStubAdvertiser(ipOf("10.0.0.1"), &fakePublisher{})
	if err := a.Start(context.Background(), "go-iot", 8080); err != nil {
		t.Fatalf("1 回目 Start: %v", err)
	}
	defer a.Stop()
	if err := a.Start(context.Background(), "go-iot", 8080); err == nil {
		t.Error("二重 Start は拒否されるべき (goroutine/公開リーク防止)")
	}
}

func TestStartStop_ShutsDownPublisherAndIsIdempotent(t *testing.T) {
	pub := &fakePublisher{}
	a := newStubAdvertiser(ipOf("10.0.0.1"), pub)
	if err := a.Start(context.Background(), "go-iot", 8080); err != nil {
		t.Fatalf("Start: %v", err)
	}
	a.Stop()
	if pub.calls() != 1 {
		t.Errorf("Stop で公開を Shutdown すべき: %d", pub.calls())
	}
	a.Stop() // 二重 Stop が panic しないこと
	if pub.calls() != 1 {
		t.Errorf("二重 Stop で Shutdown が増えてはならない: %d", pub.calls())
	}
}

// publish 失敗時は cancel/done を設定せずエラーを返し、再 Start を可能なまま維持する
// (cancel を publish より前に設定すると失敗時に二重起動ガードへ固着し Stop ハングを招く)。
func TestStart_ErrorOnPublishFailure(t *testing.T) {
	calls := 0
	pub := &fakePublisher{}
	a := &advertiser{
		interval:  time.Hour,
		currentIP: ipProvider(ipOf("10.0.0.1")),
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("公開失敗")
			}
			return pub, nil
		},
		newTicker: realTicker,
	}

	if err := a.Start(context.Background(), "go-iot", 8080); err == nil {
		t.Fatal("publish 失敗時は Start がエラーを返すべき")
	}
	// publish 失敗時は cancel 未設定のため、二重起動ガードに固着せず再 Start できる
	if err := a.Start(context.Background(), "go-iot", 8080); err != nil {
		t.Errorf("publish 失敗後は再 Start 可能であるべき (cancel 未設定): %v", err)
	}
	a.Stop()
}

// Stop は cancel/done を nil 化し、Stop 後の再 Start を可能にする (再起動の冪等性)。
func TestStart_AfterStop_CanRestart(t *testing.T) {
	pub := &fakePublisher{}
	a := newStubAdvertiser(ipOf("10.0.0.1"), pub)

	if err := a.Start(context.Background(), "go-iot", 8080); err != nil {
		t.Fatalf("1 回目 Start: %v", err)
	}
	a.Stop()
	if err := a.Start(context.Background(), "go-iot", 8080); err != nil {
		t.Errorf("Stop 後は再 Start 可能であるべき (cancel/done が nil 化される): %v", err)
	}
	a.Stop()
	if pub.calls() != 2 {
		t.Errorf("2 回の Start→Stop で 2 回 Shutdown すべき: %d", pub.calls())
	}
}

// New() が既定実装の全シームを非 nil で配線すること (nil シームは loop goroutine で
// panic=プロセスクラッシュを招くため、env ガード外の default 実行で配線ミスを捕捉する)。
func TestNew_WiresAllSeams(t *testing.T) {
	a, ok := New().(*advertiser)
	if !ok {
		t.Fatal("New() は *advertiser を返すべき")
	}
	if a.currentIP == nil {
		t.Error("currentIP シームが未配線")
	}
	if a.publish == nil {
		t.Error("publish シームが未配線")
	}
	if a.newTicker == nil {
		t.Error("newTicker シームが未配線 (nil だと loop goroutine が panic する)")
	}
	if a.interval != defaultInterval {
		t.Errorf("interval = %v, want %v", a.interval, defaultInterval)
	}
}

func TestStop_SafeWithoutStart(t *testing.T) {
	a := newStubAdvertiser(ipOf("10.0.0.1"), &fakePublisher{})
	a.Stop() // 未 Start でも panic しない (no-op)
}

func TestStart_ErrorOnIPFailure(t *testing.T) {
	a := &advertiser{
		interval:  time.Hour,
		currentIP: func() (net.IP, *net.Interface, error) { return nil, nil, errors.New("no ip") },
		publish: func(string, int, net.IP, *net.Interface) (publisher, error) {
			t.Fatal("IP 取得失敗時に publish を呼んではならない")
			return nil, nil
		},
		newTicker: realTicker,
	}
	if err := a.Start(context.Background(), "go-iot", 8080); err == nil {
		t.Error("IP 取得失敗時は Start がエラーを返すべき")
	}
}

// newStubAdvertiser は固定 IP・固定 publisher を返すスタブ Advertiser を構築する (tick しない)。
func newStubAdvertiser(ip net.IP, pub publisher) *advertiser {
	return &advertiser{
		interval:  time.Hour,
		currentIP: ipProvider(ip),
		publish:   func(string, int, net.IP, *net.Interface) (publisher, error) { return pub, nil },
		newTicker: realTicker,
	}
}
