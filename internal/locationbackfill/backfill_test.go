package locationbackfill

import (
	"context"
	"errors"
	"testing"

	"github.com/HiroshiKawano/go_iot/internal/repository"
)

func strPtr(s string) *string { return &s }

// fakeBackfillRepo は repository.Querier の部分モック (DB 非依存)。
// backfill が使う ListAllDevices / UpdateDeviceLocality のみ実装し、その他のメソッドは
// 埋め込み nil interface に委ねる (誤って他メソッドを呼べば nil panic で検出できる)。
// SoftDeleteDevice だけは「削除を一切呼ばない」非破壊性を検証するため記録のみ行う。
type fakeBackfillRepo struct {
	repository.Querier // 埋め込み (nil)。未実装メソッド呼び出しは panic で気づく

	devices         []repository.Device
	listErr         error
	updateErr       error
	updates         []repository.UpdateDeviceLocalityParams // UpdateDeviceLocality 呼び出し記録
	softDeleteCalls int                                     // 削除呼び出し回数 (非破壊性の検証用)
}

func (f *fakeBackfillRepo) ListAllDevices(_ context.Context) ([]repository.Device, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.devices, nil
}

func (f *fakeBackfillRepo) UpdateDeviceLocality(_ context.Context, arg repository.UpdateDeviceLocalityParams) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates = append(f.updates, arg)
	// 実 DB の UPDATE と同様、対象行の locality のみ書き換える (location 等は不変)。
	// 冪等再実行 (2 回目で更新 0 件) の検証のため fake 内 devices にも反映する。
	for i := range f.devices {
		if f.devices[i].ID == arg.ID {
			f.devices[i].Locality = arg.Locality
		}
	}
	return nil
}

// SoftDeleteDevice は backfill が削除を一切行わないこと (非破壊性) を検証するため記録だけする。
func (f *fakeBackfillRepo) SoftDeleteDevice(_ context.Context, _ int64) error {
	f.softDeleteCalls++
	return nil
}

// コンパイル時に repository.Querier を満たすことを保証する (埋め込みで未実装分を充足)。
var _ repository.Querier = (*fakeBackfillRepo)(nil)

// updatesByID は UpdateDeviceLocality 呼び出し記録を id→locality 値 のマップへ変換する。
func updatesByID(updates []repository.UpdateDeviceLocalityParams) map[int64]string {
	m := make(map[int64]string, len(updates))
	for _, u := range updates {
		if u.Locality != nil {
			m[u.ID] = *u.Locality
		}
	}
	return m
}

// TestBackfillLocations_一致locationのみlocalityを設定し他は不変 は backfill の中核挙動を検証する。
// location が地域 (正式名/一意な短縮名/未合併=現市町村名) に解決できる行のみ locality を設定し、
// 解決不能・曖昧・未設定・既設定の行は変更しない。location・他フィールド・件数は不変、削除は呼ばない
// (R7.1/R7.2/R7.3)。
func TestBackfillLocations_一致locationのみlocalityを設定し他は不変(t *testing.T) {
	repo := &fakeBackfillRepo{
		devices: []repository.Device{
			{ID: 1, Location: strPtr("佐敷町")},                       // 正式名 → 佐敷町
			{ID: 2, Location: strPtr("佐敷")},                        // 一意な短縮名 → 佐敷町
			{ID: 3, Location: strPtr("名護市")},                       // 未合併=現市町村名 → 名護市
			{ID: 4, Location: strPtr("具志川市")},                      // 同名の一方 (正式名) → 具志川市
			{ID: 5, Location: strPtr("ハウスA")},                      // 自由文字列 → 解決不能 (無変更)
			{ID: 6, Location: strPtr("南城市")},                       // 合併後市町村名=曖昧 → 解決不能 (無変更)
			{ID: 7, Location: nil},                                 // location 未設定 → 無変更
			{ID: 8, Location: strPtr("佐敷町"), Locality: strPtr("名護市")}, // locality 既設定 → 冪等スキップ
			{ID: 9, Location: strPtr("")},                          // 空 location → 無変更
		},
	}

	n, err := BackfillLocations(context.Background(), repo)
	if err != nil {
		t.Fatalf("BackfillLocations error: %v", err)
	}

	// 設定されたのは id 1,2,3,4 の 4 件のみ。
	if n != 4 {
		t.Errorf("更新件数 = %d, want 4", n)
	}
	got := updatesByID(repo.updates)
	want := map[int64]string{1: "佐敷町", 2: "佐敷町", 3: "名護市", 4: "具志川市"}
	if len(got) != len(want) {
		t.Errorf("UpdateDeviceLocality 呼び出し = %v, want %v", got, want)
	}
	for id, wv := range want {
		if got[id] != wv {
			t.Errorf("id=%d の locality = %q, want %q", id, got[id], wv)
		}
	}
	// 解決不能・未設定・既設定の行には UpdateDeviceLocality を呼ばない。
	for _, id := range []int64{5, 6, 7, 8, 9} {
		if _, ok := got[id]; ok {
			t.Errorf("id=%d は更新対象外のはずが UpdateDeviceLocality を呼んでいる", id)
		}
	}

	// location は不変 (UPDATE は locality 列のみ)。
	if loc := repo.devices[0].Location; loc == nil || *loc != "佐敷町" {
		t.Errorf("id=1 の location が変化している: %v", loc)
	}
	if loc := repo.devices[4].Location; loc == nil || *loc != "ハウスA" {
		t.Errorf("id=5 の location が変化している: %v", loc)
	}
	// 既設定 (id=8) の locality は上書きしない (冪等)。
	if loc := repo.devices[7].Locality; loc == nil || *loc != "名護市" {
		t.Errorf("id=8 の既設定 locality が上書きされた: %v", loc)
	}
	// 削除は一切呼ばない (件数非減少・非破壊)。
	if repo.softDeleteCalls != 0 {
		t.Errorf("SoftDeleteDevice 呼び出し = %d, want 0 (非破壊)", repo.softDeleteCalls)
	}
}

// TestBackfillLocations_再実行は冪等で更新0件 は 2 回目の実行で更新が発生しないこと (R7.1 冪等) を検証する。
func TestBackfillLocations_再実行は冪等で更新0件(t *testing.T) {
	repo := &fakeBackfillRepo{
		devices: []repository.Device{
			{ID: 1, Location: strPtr("佐敷町")},
			{ID: 2, Location: strPtr("名護市")},
		},
	}

	n1, err := BackfillLocations(context.Background(), repo)
	if err != nil {
		t.Fatalf("1 回目 error: %v", err)
	}
	if n1 != 2 {
		t.Fatalf("1 回目の更新件数 = %d, want 2", n1)
	}

	firstCallCount := len(repo.updates)
	n2, err := BackfillLocations(context.Background(), repo)
	if err != nil {
		t.Fatalf("2 回目 error: %v", err)
	}
	if n2 != 0 {
		t.Errorf("2 回目の更新件数 = %d, want 0 (冪等)", n2)
	}
	if len(repo.updates) != firstCallCount {
		t.Errorf("2 回目で UpdateDeviceLocality が追加呼び出しされた (%d → %d)", firstCallCount, len(repo.updates))
	}
}

// TestBackfillLocations_デバイス0件は更新0件 は走査対象が無いとき正常に 0 件を返すことを検証する。
func TestBackfillLocations_デバイス0件は更新0件(t *testing.T) {
	repo := &fakeBackfillRepo{}
	n, err := BackfillLocations(context.Background(), repo)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if n != 0 {
		t.Errorf("更新件数 = %d, want 0", n)
	}
	if len(repo.updates) != 0 {
		t.Errorf("UpdateDeviceLocality を呼んでいる: %v", repo.updates)
	}
}

// TestBackfillLocations_ListAllDevicesエラーは伝播 は列挙失敗時にエラーを返し更新しないことを検証する。
func TestBackfillLocations_ListAllDevicesエラーは伝播(t *testing.T) {
	wantErr := errors.New("list failed")
	repo := &fakeBackfillRepo{listErr: wantErr}

	n, err := BackfillLocations(context.Background(), repo)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if n != 0 {
		t.Errorf("更新件数 = %d, want 0", n)
	}
	if len(repo.updates) != 0 {
		t.Errorf("列挙失敗で UpdateDeviceLocality を呼んでいる")
	}
}

// TestBackfillLocations_UpdateDeviceLocalityエラーは伝播 は更新失敗時にエラーを返すことを検証する。
func TestBackfillLocations_UpdateDeviceLocalityエラーは伝播(t *testing.T) {
	wantErr := errors.New("update failed")
	repo := &fakeBackfillRepo{
		updateErr: wantErr,
		devices:   []repository.Device{{ID: 1, Location: strPtr("佐敷町")}},
	}

	_, err := BackfillLocations(context.Background(), repo)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
