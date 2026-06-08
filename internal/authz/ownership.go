// Package authz は所有者認可 (リソースが要求ユーザーに属するかの検証) を集約する。
//
// `device.UserID == userID` の類の所有者チェックは、ほぼ全画面の CRUD に現れる横断関心事である。
// これを各ハンドラへ散らすと検証漏れ = BOLA (Broken Object Level Authorization) を招くため、
// 本パッケージに集約する。ハンドラは返却された sentinel error を HTTP ステータスへ写すだけにする。
package authz

import (
	"context"
	"errors"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// ErrNotOwner はリソースが要求ユーザー以外の所有である場合に返す sentinel エラー。
// 呼び出し側は errors.Is で判定し 403 Forbidden へ写す。
var ErrNotOwner = errors.New("authz: resource belongs to a different user")

// ErrUnauthenticated は userID が未設定 (<=0) のまま認可を要求された場合に返す sentinel エラー。
// 認証ミドルウェア欠落や将来の Session 認証での SetUserID 忘れに対する fail-closed であり、
// 呼び出し側は errors.Is で判定し 401 Unauthorized へ写す。
var ErrUnauthenticated = errors.New("authz: unauthenticated (zero user id)")

// DeviceGetter は所有者認可に必要な最小の DB ポート (consumer interface)。
// repository.Querier も *repository.Queries もこれを満たす。
// 最小メソッドに限定することで、テスト時のモックを小さく保つ (DIP / アーキ決定の「consumer 最小 interface」)。
type DeviceGetter interface {
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
}

// RequireDeviceOwner は deviceID のデバイスを取得し、所有者が userID であることを検証する。
//
// 返却:
//   - (device, nil)             : 所有者一致。device をそのまま利用してよい
//   - (zero, ErrUnauthenticated): userID が未設定 (<=0)。認証前 / ミドルウェア欠落の fail-closed
//   - (zero, sql.ErrNoRows)     : デバイスが存在しない / 論理削除済み (GetDevice のエラーを透過)
//   - (zero, ErrNotOwner)       : 他ユーザーのデバイス
//   - (zero, その他 err)         : DB エラー (そのまま透過)
//
// 呼び出し側は errors.Is で分岐し HTTP ステータスへ写す:
// ErrUnauthenticated→401 / ErrNoRows→404 か 422 (本 sensor API は 422) / ErrNotOwner→403 / その他→500。
func RequireDeviceOwner(ctx context.Context, q DeviceGetter, deviceID, userID int64) (repository.Device, error) {
	// 未認証 (ゼロ値以下の userID) は所有者判定の前に fail-closed する。
	// これにより device.UserID も 0 のシード/移行行が紛れても 0==0 で誤一致しない (BOLA 多重防御)。
	if userID <= 0 {
		return repository.Device{}, ErrUnauthenticated
	}
	device, err := q.GetDevice(ctx, deviceID)
	if err != nil {
		return repository.Device{}, err
	}
	if device.UserID != userID {
		return repository.Device{}, ErrNotOwner
	}
	return device, nil
}

// AlertRuleDeviceGetter はルール所有者認可に必要な最小の DB ポート (consumer interface)。
// repository.Querier も *repository.Queries もこれを満たす。
// ルール → 所属デバイス → 所有者 の 2 段判定のため GetAlertRule と GetDevice を要する。
type AlertRuleDeviceGetter interface {
	GetAlertRule(ctx context.Context, id int64) (repository.AlertRule, error)
	GetDevice(ctx context.Context, id int64) (repository.Device, error)
}

// RequireAlertRuleOwner は ruleID のルールを取得し、その所属デバイスの所有者が userID であることを検証する。
// rule → device → owner の所有者認可を 1 箇所に合成し、判定をハンドラへ散らさない (BOLA 集約)。
//
// 返却:
//   - (rule, device, nil)              : 所有者一致。rule / device をそのまま利用してよい
//   - (zero, zero, ErrUnauthenticated) : userID が未設定 (<=0)。GetAlertRule 前の fail-closed
//   - (zero, zero, sql.ErrNoRows)      : ルールが存在しない / 論理削除済み (GetAlertRule のエラーを透過)
//   - (rule, zero, ErrNotOwner)        : 他ユーザーのデバイスに属するルール (取得済み rule は返す)
//   - (rule, zero, その他 err)          : デバイス取得時の DB エラー等 (そのまま透過)
//
// 呼び出し側は errors.Is で分岐し HTTP ステータスへ写す:
// ErrUnauthenticated→401 / ErrNoRows→404 / ErrNotOwner→403 / その他→500。
func RequireAlertRuleOwner(ctx context.Context, q AlertRuleDeviceGetter, ruleID, userID int64) (repository.AlertRule, repository.Device, error) {
	// 未認証 (ゼロ値以下の userID) はルール取得の前に fail-closed する (BOLA 多重防御)。
	if userID <= 0 {
		return repository.AlertRule{}, repository.Device{}, ErrUnauthenticated
	}
	// GetAlertRule は deleted_at IS NULL 条件付き。論理削除済み / 不在は sql.ErrNoRows として透過する。
	rule, err := q.GetAlertRule(ctx, ruleID)
	if err != nil {
		return repository.AlertRule{}, repository.Device{}, err
	}
	// 所有者判定は既存の RequireDeviceOwner を内部再利用して一元化する。
	device, err := RequireDeviceOwner(ctx, q, rule.DeviceID, userID)
	if err != nil {
		// 取得済みの rule は返す (型契約)。device 判定の sentinel はそのまま透過する。
		return rule, repository.Device{}, err
	}
	return rule, device, nil
}
