package repository

import "context"

// device-detail（デバイス詳細画面の最新計測テーブル）が依存する Querier メソッドの
// 存在をコンパイル時に保証するガード。
//
// 最新10件・降順の取得クエリ（ListLatestSensorReadings）が db/queries に追加され
// `make sqlc` で再生成されていないと、この構造的インターフェースへの代入が
// コンパイルエラーになる（TDD の RED）。生成後は型整合してコンパイルが通る（GREEN）。
//
// 引数形（deviceID int64 単体）は $1 のみのクエリに対する sqlc 既定の生成形であり、
// 既存の GetLatestSensorReading（LIMIT 1）と同じシグネチャ形になることを意図する。
var _ interface {
	ListLatestSensorReadings(ctx context.Context, deviceID int64) ([]SensorReading, error)
} = Querier(nil)
