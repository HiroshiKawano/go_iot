-- +goose Up
-- +goose StatementBegin
-- 定植/播種日 planting_date を追加 (DDL のみ・backfill DML なし)。GDD 積算の起点。
-- 温湿度ログから導出できない利用者入力ゆえ、本フェーズ唯一の永続化列。
-- NULL=未設定の既存デバイスは GDD パネルが導線注記へ縮退する (要件 6.3)。
-- CHECK は張らない (自由日付ゆえ許容集合がない)。索引も張らない (絞込キーでない=YAGNI・crop と同方針)。
ALTER TABLE devices
    ADD COLUMN planting_date DATE;

COMMENT ON COLUMN devices.planting_date IS '定植/播種日 (GDD 積算の起点・domain.GDDModel と併用。NULL=未設定で GDD パネル非表示)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 列 DROP のみ (CHECK・索引は張っていない)。planting_date は不変ゆえ復元不要。
ALTER TABLE devices DROP COLUMN IF EXISTS planting_date;
-- +goose StatementEnd
