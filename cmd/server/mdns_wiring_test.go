package main

import (
	"context"
	"errors"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/mdns"
)

// fakeAdvertiser は mdns.Advertiser のスパイ。Start/Stop の呼び出しと引数を記録する。
type fakeAdvertiser struct {
	started  bool
	gotHost  string
	gotPort  int
	startErr error
}

func (f *fakeAdvertiser) Start(_ context.Context, hostname string, port int) error {
	f.started = true
	f.gotHost = hostname
	f.gotPort = port
	return f.startErr
}

func (f *fakeAdvertiser) Stop() {}

// コンパイル時に mdns.Advertiser を満たすことを保証する。
var _ mdns.Advertiser = (*fakeAdvertiser)(nil)

// startAdvertiser は listen 後に mDNS 公開を開始する (実ポート・ホスト名を渡す)。
func TestStartAdvertiser_StartsWithHostAndPort(t *testing.T) {
	adv := &fakeAdvertiser{}
	startAdvertiser(context.Background(), adv, "go-iot", 8123)

	if !adv.started {
		t.Error("mDNS の Start が呼ばれるべき")
	}
	if adv.gotHost != "go-iot" || adv.gotPort != 8123 {
		t.Errorf("Start 引数 = (%q, %d), want (go-iot, 8123)", adv.gotHost, adv.gotPort)
	}
}

// mDNS 開始失敗は非致命: panic せず継続する (IP 直打ちで到達可能なため) (R7.1)。
func TestStartAdvertiser_NonFatalOnError(t *testing.T) {
	adv := &fakeAdvertiser{startErr: errors.New("mDNS 公開失敗")}
	// panic せず正常に戻ること = 起動継続
	startAdvertiser(context.Background(), adv, "go-iot", 8123)

	if !adv.started {
		t.Error("失敗時も Start を試行すべき")
	}
}
