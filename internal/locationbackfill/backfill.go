// Package locationbackfill は既存デバイスの自由入力「設置場所」(devices.location) を
// 構造化所在地 (devices.locality) へ非破壊・冪等に移行する一度きりのロジックを提供する。
// 永続化は repository.Querier (sqlc) 経由のみ、地域解決は internal/domain を単一ソースに用い、
// DB 非依存に fakeRepo で検証できる (実 DB テスト基盤を新設しない)。
package locationbackfill

import (
	"context"

	"github.com/HiroshiKawano/go_iot/internal/domain"
	"github.com/HiroshiKawano/go_iot/internal/repository"
)

// BackfillLocations は全デバイスを走査し、location が沖縄の地域に対応付けられる行のみ
// locality を構造化所在地として設定する。設定した件数を返す。
//
// 移行規則:
//   - locality が既に設定済みの行はスキップする (冪等。再実行で更新 0 件・R7.1)。
//   - location が未設定 (nil/空) の行は移行元が無いのでスキップする。
//   - location を domain.ParseLocality (旧町村名/正式名/未合併=現市町村名 のエイリアス) で
//     地域へ解決できる行のみ UpdateDeviceLocality で locality を設定する (R7.1)。
//   - いずれの地域にも対応付けられない (曖昧な短縮名・合併後市町村名・自由文字列を含む) 行は
//     locality 未設定のまま残す (R7.2)。
//
// 非破壊性: UPDATE は locality 列のみで location・他フィールドは不変、削除・件数減少を伴わない
// (R7.3)。移行先 sqlc クエリ (ListAllDevices/UpdateDeviceLocality) は task 1.4 で生成済み。
func BackfillLocations(ctx context.Context, q repository.Querier) (int, error) {
	devices, err := q.ListAllDevices(ctx)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, d := range devices {
		// 既に locality 設定済みはスキップ (冪等・R7.1)。
		if d.Locality != nil && *d.Locality != "" {
			continue
		}
		// location 未設定は移行元が無いのでスキップ。
		if d.Location == nil || *d.Location == "" {
			continue
		}
		// 地域へ解決できないものは未設定のまま残す (R7.2)。
		loc, err := domain.ParseLocality(*d.Location)
		if err != nil {
			continue
		}

		value := string(loc)
		if err := q.UpdateDeviceLocality(ctx, repository.UpdateDeviceLocalityParams{
			ID:       d.ID,
			Locality: &value,
		}); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, nil
}
