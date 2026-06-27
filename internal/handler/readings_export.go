// readings_export.go はセンサーデータ履歴の CSV エクスポート (GET /devices/:device/readings.csv)
// を担う (sensor-data-export フェーズ)。期間内の全計測行を、メタ列 (デバイスID/名称/地点/作物) を
// 各行へ反復付与した CSV ファイルとして添付ダウンロードさせる。データ主権=Ambient 脱却の核として
// 外部ツール (Excel/R/Python) で地点別/作物別に横断・pivot できる「集計軸の CSV 化」を提供する。
//
// 整形ヘルパ (writeReadingsCSV/csvFilename/selectMetricCols/newCSVMeta) は HTTP/DB に依存しない
// 純度の高い関数として切り出し単体テスト可能にする。エスケープ (カンマ/改行/引用符) は標準
// encoding/csv に委任し、文字化け回避の UTF-8 BOM・RFC 5987 ファイル名のみ手書きする。
// 計測値は §33.5 に従い pgconv.NumericToFloat→2桁固定、計測日時は JST RFC3339 で出力する。
package handler

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/HiroshiKawano/go_iot/internal/auth"
	"github.com/HiroshiKawano/go_iot/internal/authz"
	"github.com/HiroshiKawano/go_iot/internal/infra/pgconv"
	"github.com/HiroshiKawano/go_iot/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// utf8BOM は Excel が UTF-8 を正しく認識するための先頭バイト列 (R3.1 文字化け回避)。
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// csvMetaHeader は CSV 先頭の固定メタ列見出し (デバイスID/名称/地点/作物/計測日時)。
// 各データ行へメタ列を反復付与する (先頭メタブロックではなく行ごと・外部 pivot 用・R2.1)。
var csvMetaHeader = []string{"デバイスID", "デバイス名", "地点", "作物", "計測日時"}

// csvMeta は CSV の各行へ反復付与するデバイスメタ (整形済み表示文字列)。
// Locality/Crop は表示名解決済み (未設定は "未設定"・newCSVMeta が解決)。
type csvMeta struct {
	DeviceID int64
	Name     string
	Locality string // 認識名 or "未設定"
	Crop     string // 作物ラベル or "未設定"
}

// metricCol は出力する計測項目1列の見出しと値取り出し (温度/湿度)。
// 項目フィルタ (items) に応じて列を増減するために列を値として扱う (selectMetricCols)。
type metricCol struct {
	Header string                                // "温度(℃)" / "湿度(%)"
	value  func(repository.SensorReading) string // 小数2桁の数値文字列
}

// tempCol/humCol は温度・湿度の出力列定義 (値は formatActual で小数2桁・§33.5)。
var (
	tempCol = metricCol{Header: "温度(℃)", value: func(r repository.SensorReading) string { return formatActual(r.Temperature) }}
	humCol  = metricCol{Header: "湿度(%)", value: func(r repository.SensorReading) string { return formatActual(r.Humidity) }}
)

// 項目フィルタの enum (sensor_readings の CHECK と整合・派生指標は本フェーズ対象外)。
const (
	metricTemperature = "temperature"
	metricHumidity    = "humidity"
)

// effectiveMetricItems は items から有効な選択 (temperature/humidity) を温度→湿度の順で返す。
// 1つも有効な選択が無い (未指定/不正のみ) ときは両方を既定とする (R1.5)。CSV 列選択・CSV リンクの
// items・フィルタフォームの checked echo (4.2) でこの既定ロジックを共有し、出口間の項目を一致させる。
func effectiveMetricItems(items []string) []string {
	wantTemp, wantHum := false, false
	for _, it := range items {
		switch it {
		case metricTemperature:
			wantTemp = true
		case metricHumidity:
			wantHum = true
		}
	}
	if !wantTemp && !wantHum {
		wantTemp, wantHum = true, true // 既定は両方 (R1.5)
	}
	out := make([]string, 0, 2)
	if wantTemp {
		out = append(out, metricTemperature)
	}
	if wantHum {
		out = append(out, metricHumidity)
	}
	return out
}

// selectMetricCols は items クエリを出力列へ写す (列順は温度→湿度で安定・ヘッダ順と一致)。
// 既定 (未選択は両方) は effectiveMetricItems に集約する。
func selectMetricCols(items []string) []metricCol {
	cols := make([]metricCol, 0, 2)
	for _, it := range effectiveMetricItems(items) {
		switch it {
		case metricTemperature:
			cols = append(cols, tempCol)
		case metricHumidity:
			cols = append(cols, humCol)
		}
	}
	return cols
}

// csvDownloadURL は結果領域の CSV ダウンロードリンク (適用済み from/to/items を反映) を組む。
// from/to は未指定 ("") のとき省略し、items は effectiveMetricItems で正規化して反復付与する
// (未選択でも温度/湿度を明示・モック正本 readings.html の href と一致)。CSV ハンドラ Export と同じ
// 既定ゆえ、表示中の項目とダウンロード内容が一致する (R1.5)。
func csvDownloadURL(deviceID int64, from, to string, items []string) string {
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	for _, it := range effectiveMetricItems(items) {
		q.Add("items", it)
	}
	return fmt.Sprintf("/devices/%d/readings.csv?%s", deviceID, q.Encode())
}

// newCSVMeta はデバイス行を CSV メタへ写す。地点/作物は表示名を解決し、未設定 (NULL)・不正は
// "未設定" にフォールバックして欠落や不正値でファイルを壊さない (R2.2/R2.3)。
// 解決は情報パネルと同じ既存ヘルパ (deviceLocalityOrUnset/deviceCropLabelOrUnset) を流用する。
func newCSVMeta(d repository.Device) csvMeta {
	return csvMeta{
		DeviceID: d.ID,
		Name:     d.Name,
		Locality: deviceLocalityOrUnset(d),
		Crop:     deviceCropLabelOrUnset(d),
	}
}

// writeReadingsCSV は BOM→ヘッダ→全行を io.Writer へ書き出す (HTTP/DB 非依存・単体テスト可能)。
// メタ列を各行へ反復付与し、cols の項目列を末尾へ足す。エスケープ (カンマ/改行/引用符) は
// encoding/csv に委任する。空 rows はヘッダのみを書く (R1.6)。
func writeReadingsCSV(w io.Writer, meta csvMeta, cols []metricCol, rows []repository.SensorReading) error {
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)

	header := append([]string{}, csvMetaHeader...)
	for _, col := range cols {
		header = append(header, col.Header)
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	deviceID := strconv.FormatInt(meta.DeviceID, 10)
	for _, r := range rows {
		rec := []string{deviceID, meta.Name, meta.Locality, meta.Crop, csvRecordedAt(r.RecordedAt)}
		for _, col := range cols {
			rec = append(rec, col.value(r))
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// csvRecordedAt は計測時刻 (instant) を JST RFC3339 ("2006-01-02T15:04:05+09:00") へ整形する
// (絶対表記で外部ツールが TZ を誤らない・§33.5)。
func csvRecordedAt(ts pgtype.Timestamptz) string {
	return pgconv.TimestamptzToTime(ts).In(jst).Format(time.RFC3339)
}

// csvFilename は添付ファイル名を ASCII フォールバック (filename=) と RFC 5987 (filename*=) の
// 2 形式で組む。ASCII 側は期間ベース ("readings_<期間>.csv"・非 ASCII を含まない)、RFC 5987 側は
// 日本語デバイス名を含む完全名をパーセントエンコードする (R3.4・対象デバイスと期間が判別できる)。
func csvFilename(deviceName, from, to string) (ascii, rfc5987 string) {
	period := csvPeriodLabel(from, to)
	ascii = "readings_" + period + ".csv"
	rfc5987 = rfc5987Encode(deviceName + "_" + ascii)
	return ascii, rfc5987
}

// csvPeriodLabel は from/to (YYYY-MM-DD・ASCII) を ASCII のファイル名片へ写す。
// 未指定は "all"、片方のみは until_/since_ を付ける (全期間・片側開区間を判別できる)。
func csvPeriodLabel(from, to string) string {
	switch {
	case from == "" && to == "":
		return "all"
	case from == "":
		return "until_" + to
	case to == "":
		return "since_" + from
	default:
		return from + "_" + to
	}
}

// rfc5987Encode は文字列を RFC 5987 の "UTF-8”<pct-encoded>" 形式へ符号化する。
// attr-char (英数字 + !#$&+-.^_`|~) 以外の全バイトをパーセントエンコードする (日本語=各 UTF-8 バイト)。
func rfc5987Encode(s string) string {
	const upperhex = "0123456789ABCDEF"
	buf := make([]byte, 0, len(s)*3+7)
	buf = append(buf, "UTF-8''"...)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isAttrChar(c) {
			buf = append(buf, c)
		} else {
			buf = append(buf, '%', upperhex[c>>4], upperhex[c&0x0f])
		}
	}
	return string(buf)
}

// isAttrChar は RFC 5987 の attr-char (パーセントエンコード不要文字) かを判定する。
func isAttrChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '&', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// Export は期間内の全計測行を CSV ファイルとして添付ダウンロードさせる
// (GET /devices/:device/readings.csv・RequireAuth 前提・GET ゆえ CSRF 対象外)。
// 認可 (不在/非所有→404 列挙防止) → parseDateBounds (形式不正→400・データ無出力) → 全行取得 →
// BOM＋ヘッダ＋全行を text/csv で書き出す。期間境界は画面一覧と同一写像 (parseDateBounds 共有・R7)。
func (h *ReadingsHandler) Export(c *gin.Context) {
	ctx := c.Request.Context()
	uid := auth.UserID(c)

	id, err := strconv.ParseInt(c.Param("device"), 10, 64)
	if err != nil {
		renderError(c, http.StatusBadRequest) // 非数値 ID
		return
	}

	device, err := authz.RequireDeviceOwner(ctx, h.Repo, id, uid)
	if err != nil {
		renderDeviceReadError(c, err) // 不在/非所有とも 404 (列挙防止)・日付検証より先
		return
	}

	from := c.Query("from")
	to := c.Query("to")
	fromTS, toTS, errs := parseDateBounds(from, to)
	if len(errs) > 0 {
		renderError(c, http.StatusBadRequest) // 形式不正→400・計測データのファイルを返さない (R1.7)
		return
	}

	// 全行を取得 (BETWEEN・ASC・LIMIT なし)。ヘッダ送出前に DB エラーを検知し 500 を返せる。
	rows, err := h.Repo.ListSensorReadingsInRange(ctx, repository.ListSensorReadingsInRangeParams{
		DeviceID:     id,
		RecordedAt:   pgconv.Timestamptz(fromTS),
		RecordedAt_2: pgconv.Timestamptz(toTS),
	})
	if err != nil {
		renderError(c, http.StatusInternalServerError)
		return
	}

	cols := selectMetricCols(c.QueryArray("items"))
	ascii, rfc5987 := csvFilename(device.Name, from, to)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q; filename*=%s", ascii, rfc5987))
	c.Status(http.StatusOK)

	// 行取得成功後に書き出す。途中の I/O エラー (接続切断) はヘッダ送出済みゆえ握り潰す
	// (ステータスは変えられない・部分 CSV)。数ヶ月スケール前提の materialize＋一括 Flush
	// (R9・年スケールはキーセットバッチ版へ interface 互換で差し替える将来余地)。
	_ = writeReadingsCSV(c.Writer, newCSVMeta(device), cols, rows)
}
