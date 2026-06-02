package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/jackc/pgx/v5"
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
	q := fakeDeviceGetter{err: pgx.ErrNoRows}

	_, err := RequireDeviceOwner(context.Background(), q, 10, 7)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("存在しない: got %v, want pgx.ErrNoRows", err)
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
