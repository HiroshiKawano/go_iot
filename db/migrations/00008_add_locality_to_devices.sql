-- +goose Up
-- +goose StatementBegin
-- 圃場所在地の構造化キー locality を追加 (DDL のみ・DML は backfill CLI で別途実施)。
-- 沖縄53地域 (未合併36市町村 + 平成の大合併5件の旧町村17)。値は domain.Locality と同期する二重ミラー。
ALTER TABLE devices
    ADD COLUMN locality VARCHAR(20);

-- 許容値を domain.Locality の53値に限定 (NULL は任意項目ゆえ許容)。
-- ※ domain.Locality 集合を変更したらこの CHECK も同期すること (revalidation trigger)。
ALTER TABLE devices
    ADD CONSTRAINT devices_locality_valid CHECK (
        locality IS NULL OR locality IN (
        '那覇市', '宜野湾市', '石垣市', '浦添市', '名護市', '糸満市',
        '沖縄市', '豊見城市', '石川市', '具志川市', '与那城町', '勝連町',
        '平良市', '城辺町', '下地町', '上野村', '伊良部町', '佐敷町',
        '知念村', '玉城村', '大里村', '本部町', '金武町', '嘉手納町',
        '北谷町', '西原町', '与那原町', '南風原町', '仲里村', '具志川村',
        '東風平町', '具志頭村', '竹富町', '与那国町', '国頭村', '大宜味村',
        '東村', '今帰仁村', '恩納村', '宜野座村', '伊江村', '読谷村',
        '北中城村', '中城村', '渡嘉敷村', '座間味村', '粟国村', '渡名喜村',
        '南大東村', '北大東村', '伊平屋村', '伊是名村', '多良間村'
        )
    );

-- 地点 (地域) を集計の前提キーとするための部分索引 (論理削除を除外)。
CREATE INDEX devices_locality_idx
    ON devices(locality) WHERE deleted_at IS NULL;

COMMENT ON COLUMN devices.locality IS '圃場所在地の地域キー (沖縄53地域・domain.Locality と対応。親市町村は Locality.Municipality() で導出)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 列 DROP で CHECK 制約・索引も連鎖削除される。location は不変ゆえ復元不要。
ALTER TABLE devices DROP COLUMN IF EXISTS locality;
-- +goose StatementEnd
