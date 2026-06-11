# 最終統合検証レポート — desktop-exe-packaging（タスク7）

検証日: 2026-06-11
対象: `desktop-exe-packaging`（SQLite 化済みの農業 IoT アプリを単一 Windows `.exe` のデスクトップアプリへ梱包）
位置づけ: タスク7「最終統合検証」の観測可能完了の根拠記録。タスク1〜6 完了後の統合確認。

---

## 1. 受け入れ判定サマリ

| 観点 | 結果 | 根拠 |
|---|---|---|
| 全テスト green（`go test -race ./...`） | ✅ 合格 | 全 27 パッケージ ok（FAIL 0・race 検出 0） |
| 業務ロジック カバレッジ 80% 以上 | ✅ 合格 | `make cover` = **93.7%**（生成コード sqlc/templ・cmd・docs を分母から除外） |
| Windows 単一 `.exe` ビルド成功（console / GUI 両方） | ✅ 合格 | `make build-windows` = `PE32+ executable (console) x86-64`、`make build-windows-gui` = `PE32+ executable (GUI) x86-64`、いずれも単一ファイル（約 27MB） |
| pure-Go（cgo 非依存・単一 .exe 不変条件） | ✅ 合格 | `CGO_ENABLED=0` クロスビルド成功。バイナリ依存に PostgreSQL ドライバ 0 件、一次コードに `import "C"` 無し |
| docker / PostgreSQL 非依存（ビルド/起動/テスト） | ✅ 合格 | コンテナ構成ファイル無し、Makefile に docker 依存無し、`go list -deps ./cmd/server` に pgx/lib/pq 非含有・modernc 含有 |

これらは `cmd/server/packaging_test.go`（PE32+/サブシステム/Version 注入/docker・PG 撤去）と
`cmd/server/final_integration_test.go`（ゼロ設定 e2e 起動）で自動回帰として固定済み。

---

## 2. 統合検証の自動テスト（タスク7 で追加）

`cmd/server/final_integration_test.go` の
`TestIntegration_ゼロ設定起動でDB自動作成しマイグレーション適用後にHTTPが応答する` が、env を完全に
未設定相当にしアプリデータ解決先を一時ディレクトリへリダイレクトしたうえで、本番と同じ起動シーケンス
（`config.Load` → `openAndMigrate` → `newHTTPHandler` → Serve）を end-to-end で検証する:

1. env/.env 無しでも `config.Load` がハードフェイルせず成立（R2.1）。
2. DB ファイルが CWD ではなくアプリデータ配下に自動作成される（R2.2 / R2.5）。
3. 起動時にスキーマ（業務6テーブル + sessions の全 7 テーブル）が自動適用される（R4.1）。
4. 合成ルートが実 HTTP（`/health` = DB 疎通込み）に 200 で応答する（R2.1 待受到達）。

各サブシステム（appdata / config / migrate / desktop / mdns / handler）は個別テスト済みのため、
本テストはそれらが「ゼロ設定起動」として正しく合成されることの統合確認に絞っている。

---

## 3. 残存リスク（本機能では検証しない＝別フェーズ）

下記は本機能の受け入れ基準（クロスビルド成功＋ロジック/結合テスト green）の対象外であり、
実機・実デバイスを要するため別フェーズで検証する（requirements.md「Residual Risk」/ design.md「残存リスク」と整合）。

| # | 残存リスク | 関連要件 | 別フェーズでの確認内容 |
|---|---|---|---|
| R-1 | **別 Windows 端末コピー起動の配布性** | R1.4 | 生成した `dist/go_iot.exe` を別 Windows 端末へコピーし、追加ランタイム（DB サーバ / コンテナ / 組込ブラウザ）無しでダブルクリック起動できること |
| R-2 | **実 ESP32 からの `go-iot.local` 到達** | R7.1〜7.3 | 同一 LAN の実 ESP32 が mDNS で公開した `go-iot.local` を名前解決し、Bearer 付き `POST /api/sensor-data` が 201 で保存されること。DHCP による IP 変動後の追従も含む |
| R-3 | **GUI ビルドのコンソール窓非表示と `app.log` 書出（実機）** | R9.1 / R9.2 | GUI ビルド（`-H windowsgui`）の `.exe` を実 Windows で起動した際にコンソール窓が出ないこと、動作ログが `%LOCALAPPDATA%\go_iot\app.log` に書かれること |

補足（設計上の既知トレードオフ・別途検討）:

- **ポート自動採番 ↔ mDNS の到達性**: 既定ポート競合で自動採番が発火すると、A レコードのみ引き
  port を out-of-band（既定 8080）で持つ ESP32 は新ポートを知れず到達不能になりうる（design.md 参照）。
- **並行安定性の保証範囲**: SQLITE_BUSY 由来 500 不発の保証は、受信/アラート評価/scs cleanup が
  すべて単一文 auto-commit である前提に限定（タスク 5.3 で検証）。将来 read-then-write Tx を導入する
  場合は DSN へ `_txlock=immediate` を付与する（Decision 6・別フェーズ）。
- **steering（tech.md / structure.md）の PostgreSQL→SQLite 反映**: 本機能スコープ外（別途 doc 更新を推奨）。

---

## 4. 要件トレーサビリティ（タスク7: R1.4 / R8.1 / R10.2）

| 要件 | 充足状況 |
|---|---|
| R1.4 別 Windows 端末でランタイム追加なしに起動 | クロスビルド成功・pure-Go・単一 .exe を自動確認。**実機コピー起動は R-1 として別フェーズ** |
| R8.1 並行アクセス下で DB ビジー由来の 500 を出さない | タスク 5.3 の並行負荷テストで 500 = 0 件・全書込完了を確認（保証範囲は §3 補足のとおり単一文 auto-commit 限定） |
| R10.2 ビルド/起動/テストが docker/PostgreSQL 非依存 | コンテナ構成撤去・Makefile docker 依存撤去・バイナリ pg 非依存を自動確認 |
