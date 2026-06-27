-- +goose Up
-- +goose StatementBegin
-- 栽培作物 crop を追加 (DDL のみ・backfill DML なし)。
-- VPD 適正帯の切替に使う作物9種 (domain.Crop と同期する二重ミラー)。
-- 作物未設定 (NULL) の既存デバイスは既定帯 0.3-1.5kPa で破綻なく動作する (要件 2.6)。
ALTER TABLE devices
    ADD COLUMN crop VARCHAR(20);

-- 許容値を domain.Crop の9値に限定 (NULL は任意項目ゆえ許容)。
-- ※ domain.Crop 集合を変更したらこの CHECK も同期すること (revalidation trigger)。
ALTER TABLE devices
    ADD CONSTRAINT devices_crop_valid CHECK (
        crop IS NULL OR crop IN (
        'goya', 'ingen', 'sugarcane', 'mango', 'pineapple',
        'uri', 'rice', 'imo', 'leafy_vegetable'
        )
    );

COMMENT ON COLUMN devices.crop IS '栽培作物キー (domain.Crop と対応・VPD 適正帯の切替に使用。NULL=未設定で既定帯 0.3-1.5kPa)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 列 DROP で CHECK 制約も連鎖削除される。索引は張っていない (P3 は作物集計しない=YAGNI)。
ALTER TABLE devices DROP COLUMN IF EXISTS crop;
-- +goose StatementEnd
