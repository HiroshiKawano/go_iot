# エラーハンドリング標準

[目的: エラーの分類・形・伝播・ログ・監視を統一する]

## 思想
- できる限り早く失敗する (fail fast)。システム境界ではグレースフルに縮退
- スタック全体で一貫したエラー形（人間も機械も読める）
- 既知エラーは発生源近くで処理。未知エラーはグローバルハンドラに委譲

## 分類（発生源で処理を決める）
- **クライアント**: 入力/バリデーション/ユーザー操作 → 4xx
- **サーバー**: システム障害・想定外例外 → 5xx
- **ビジネス**: ルール/状態違反 → 4xx（例: 409 Conflict）
- **外部**: サードパーティ/ネットワーク障害 → 5xx または文脈付き 4xx

## エラー形（単一正準フォーマット）
```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "人間向けメッセージ",
    "requestId": "trace-id",
    "timestamp": "ISO-8601"
  }
}
```
原則: コードは列挙で安定化、シークレットを含めない、トレース情報を含める。

## 伝播（変換の責務境界）
- **API 層 (handler)**: ドメインエラー → HTTP ステータス + 正準ボディへ変換
- **サービス層**: 型付きビジネスエラーを返す。`errors.New("...")` の文字列エラーは避け、sentinel か独自型を使う
- **データ/外部層**: プロバイダのエラーを安全な code でラップ（`fmt.Errorf("...: %w", err)`）
- **未知エラー**: グローバルハンドラへ委譲 → 500 + 汎用メッセージ

Go + Gin のパターン:
```go
// ドメイン層: sentinel エラーと独自型
var ErrDeviceNotFound = errors.New("device not found")

type ValidationError struct {
    Field, Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// handler: エラーを HTTP レスポンスにマッピング
func (h *Handler) GetDevice(c *gin.Context) {
    d, err := h.svc.FindByID(c.Request.Context(), id)
    if err != nil {
        switch {
        case errors.Is(err, ErrDeviceNotFound):
            c.JSON(http.StatusNotFound, apiError("DEVICE_NOT_FOUND", err.Error(), c))
        case errors.As(err, &validationErr):
            c.JSON(http.StatusBadRequest, apiError("VALIDATION", err.Error(), c))
        default:
            h.log.Error("unexpected", "err", err, "requestId", requestID(c))
            c.JSON(http.StatusInternalServerError, apiError("INTERNAL", "internal error", c))
        }
        return
    }
    c.JSON(http.StatusOK, d)
}
```

- エラーラップは `%w` を使い、呼出側で `errors.Is` / `errors.As` で判定できる形を保つ
- パニックはグローバルリカバリミドルウェア (`gin.Recovery()`) で 500 に変換

## ログ（ノイズより文脈）
記録する: 操作名、user_id（あれば）、code、message、スタック、requestId、最小限のコンテキスト。
記録しない: パスワード、トークン、シークレット、PII 全量、機微ボディ。
レベル: ERROR（失敗）/ WARN（回復可・端境）/ INFO（主要イベント）/ DEBUG（診断）。
Go 標準 `log/slog` または `zap` を推奨。構造化ログを統一。

## リトライ（安全な場合のみ）
リトライしてよい: ネットワーク/タイムアウト/一時的 5xx **かつ** 操作が冪等であるとき。
リトライしない: 4xx、ビジネスエラー、非冪等処理。
戦略: 指数バックオフ + ジッタ、試行回数に上限、冪等キーを要求。

## 監視 & ヘルス
追跡: エラー率（コード/分類別）、レイテンシ、飽和度。スパイク/SLI 違反で通知。
ヘルスエンドポイント: `/health`（生存）、`/health/ready`（準備完了）。エラーをトレースと紐付ける。

---
_パターンと意思決定に集中。実装詳細や列挙全件は避ける_
