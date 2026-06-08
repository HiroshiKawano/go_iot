package authz

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// fakeDeviceGetter は DeviceGetter の最小モック。
type fakeDeviceGetter struct {
	device repository.Device
	err    error
}

func (f fakeDeviceGetter) GetDevice(_ context.Context, _ int64) (repository.Device, error) {
	return f.device, f.err
}

func TestRequireDeviceOwner_所有者一致ならdeviceを返す(t *testing.T) {
	q := fakeDeviceGetter{device: repository.Device{ID: 10, UserID: 7}}

	device, err := RequireDeviceOwner(context.Background(), q, 10, 7)
	if err != nil {
		t.Fatalf("所有者一致で err: %v", err)
	}
	if device.ID != 10 {
		t.Errorf("device.ID: got %d, want 10", device.ID)
	}
}

func TestRequireDeviceOwner_他ユーザー所有ならErrNotOwner(t *testing.T) {
	q := fakeDeviceGetter{device: repository.Device{ID: 10, UserID: 999}}

	_, err := RequireDeviceOwner(context.Background(), q, 10, 7)
	if !errors.Is(err, ErrNotOwner) {
		t.Errorf("他ユーザー所有: got %v, want ErrNotOwner", err)
	}
}

func TestRequireDeviceOwner_存在しなければErrNoRowsを透過(t *testing.T) {
	q := fakeDeviceGetter{err: sql.ErrNoRows}

	_, err := RequireDeviceOwner(context.Background(), q, 10, 7)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("存在しない: got %v, want sql.ErrNoRows", err)
	}
	if errors.Is(err, ErrNotOwner) {
		t.Errorf("存在しないエラーを ErrNotOwner に誤分類してはならない")
	}
}

func TestRequireDeviceOwner_userIDがゼロなら認証前としてErrUnauthenticated(t *testing.T) {
	// device.UserID も 0 にして 0==0 で誤って所有者一致しないこと (BOLA 境界) を固定する。
	q := fakeDeviceGetter{device: repository.Device{ID: 10, UserID: 0}}

	_, err := RequireDeviceOwner(context.Background(), q, 10, 0)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("userID=0: got %v, want ErrUnauthenticated", err)
	}
	if errors.Is(err, ErrNotOwner) {
		t.Error("ゼロ値の未認証を所有者判定(ErrNotOwner)へ流してはならない")
	}
}

func TestRequireDeviceOwner_DBエラーはそのまま透過(t *testing.T) {
	dbErr := errors.New("db down")
	q := fakeDeviceGetter{err: dbErr}

	_, err := RequireDeviceOwner(context.Background(), q, 10, 7)
	if !errors.Is(err, dbErr) {
		t.Errorf("DB エラー透過: got %v, want %v", err, dbErr)
	}
	if errors.Is(err, ErrNotOwner) {
		t.Errorf("DB エラーを ErrNotOwner に誤分類してはならない")
	}
}

// fakeAlertRuleDeviceGetter は AlertRuleDeviceGetter の最小モック。
// ruleCalls で GetAlertRule の呼び出し有無を記録し、未認証時の fail-closed (GetAlertRule 前に弾く) を検証する。
type fakeAlertRuleDeviceGetter struct {
	rule      repository.AlertRule
	ruleErr   error
	device    repository.Device
	deviceErr error
	ruleCalls int
}

func (f *fakeAlertRuleDeviceGetter) GetAlertRule(_ context.Context, _ int64) (repository.AlertRule, error) {
	f.ruleCalls++
	return f.rule, f.ruleErr
}

func (f *fakeAlertRuleDeviceGetter) GetDevice(_ context.Context, _ int64) (repository.Device, error) {
	return f.device, f.deviceErr
}

func TestRequireAlertRuleOwner_本人所有ならruleとdeviceを返す(t *testing.T) {
	q := &fakeAlertRuleDeviceGetter{
		rule:   repository.AlertRule{ID: 5, DeviceID: 10},
		device: repository.Device{ID: 10, UserID: 7},
	}

	rule, device, err := RequireAlertRuleOwner(context.Background(), q, 5, 7)
	if err != nil {
		t.Fatalf("本人所有で err: %v", err)
	}
	if rule.ID != 5 {
		t.Errorf("rule.ID: got %d, want 5", rule.ID)
	}
	if device.ID != 10 {
		t.Errorf("device.ID: got %d, want 10", device.ID)
	}
}

func TestRequireAlertRuleOwner_ルール不在ならErrNoRowsを透過(t *testing.T) {
	q := &fakeAlertRuleDeviceGetter{ruleErr: sql.ErrNoRows}

	rule, _, err := RequireAlertRuleOwner(context.Background(), q, 5, 7)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("ルール不在: got %v, want sql.ErrNoRows", err)
	}
	if errors.Is(err, ErrNotOwner) {
		t.Errorf("不在(論理削除済み含む)を ErrNotOwner に誤分類してはならない")
	}
	if rule.ID != 0 {
		t.Errorf("ルール取得失敗時に rule を返してはならない: got ID %d", rule.ID)
	}
}

func TestRequireAlertRuleOwner_他ユーザーデバイスならErrNotOwner(t *testing.T) {
	q := &fakeAlertRuleDeviceGetter{
		rule:   repository.AlertRule{ID: 5, DeviceID: 10},
		device: repository.Device{ID: 10, UserID: 999},
	}

	rule, _, err := RequireAlertRuleOwner(context.Background(), q, 5, 7)
	if !errors.Is(err, ErrNotOwner) {
		t.Errorf("他ユーザーデバイス: got %v, want ErrNotOwner", err)
	}
	// 型契約: 他ユーザーでも取得済みの rule は返す ((rule, zero, ErrNotOwner))。
	if rule.ID != 5 {
		t.Errorf("ErrNotOwner 時も取得済み rule を返すべき: got ID %d, want 5", rule.ID)
	}
}

func TestRequireAlertRuleOwner_userIDがゼロなら認証前としてErrUnauthenticated(t *testing.T) {
	q := &fakeAlertRuleDeviceGetter{
		rule:   repository.AlertRule{ID: 5, DeviceID: 10},
		device: repository.Device{ID: 10, UserID: 0},
	}

	_, _, err := RequireAlertRuleOwner(context.Background(), q, 5, 0)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("userID=0: got %v, want ErrUnauthenticated", err)
	}
	if errors.Is(err, ErrNotOwner) {
		t.Error("ゼロ値の未認証を所有者判定(ErrNotOwner)へ流してはならない")
	}
	// fail-closed: GetAlertRule を呼ぶ前に弾く (BOLA 多重防御)。
	if q.ruleCalls != 0 {
		t.Errorf("未認証は GetAlertRule 前に fail-closed すべき: GetAlertRule が %d 回呼ばれた", q.ruleCalls)
	}
}
