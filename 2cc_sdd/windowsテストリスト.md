# Windows 受け入れ検証チェックリスト（go_iot デスクトップ単一 .exe）

検証日: ____________　検証者: ____________　対象端末: ____________（Windows バージョン: ________）
検証対象バイナリ: `dist/go_iot-console.exe` / `dist/go_iot-gui.exe`（ビルド元コミット: ____________）

---

## このファイルの使い方（Claude Code on Windows への依頼）

このファイルは、本リポジトリ（go_iot）を **Windows 端末に clone もしくはコピーした状態**で、Claude Code に「このファイルを開いて上から順に実行して」と依頼することを前提に書かれている。Claude Code は各項目の手順を実行し、`[ ] PASS / [ ] FAIL` 欄を埋め、メモに観測値（実ポート・パス・ステータスコード等）を記録すること。

各手順には次のタグが付く。タグに従って実行主体を切り替える。

- **【自動】** … Claude Code が Bash（Git Bash）または PowerShell で実行・確認できる。実コマンドを書いてある。コマンドの出力を判定根拠とする。
- **【目視】** … 人間が画面（GUI 窓・ブラウザ・タスクバー等）を見て確認する必要がある。Claude はコマンドでは判定できないため、人間に依頼し結果を記入してもらう。なお「サーバが待受開始しブラウザ起動を試行したか」までは Claude がログで自動確認でき、最終的な窓・画面の目視のみ人間に委ねる項目がある（各項目に明記）。
- **【ESP32/別端末】** … 実 ESP32 デバイス、または同一 LAN 上の別ホスト（別 PC・スマホ等）が必要。これらが無い場合は各項目の「代替（近似確認）」に従い、同一ホストからの `curl` / `Resolve-DnsName` で可能な範囲を確認する。**何が ESP32/別端末でしか確認できないか**は各項目に明示する。

### Claude が止まらず進めるための実行規約（重要・全章共通）

Claude Code の Bash/PowerShell ツールは**呼び出しごとにシェル状態（環境変数・PowerShell 変数 `$p` 等・cwd）がリセットされる**。本チェックリストは複数のコードブロックを別々の呼び出しで実行しても破綻しないよう、次の規約に従うこと。

1. **プロセス起動はバックグラウンドで行う。** フォアグラウンドで `.exe` を起動すると待受でブロックし Claude のターンが固まる。`Start-Process`（PowerShell）または `run_in_background`（Bash）を使う。
2. **プロセス停止は PID 変数（`$p.Id` 等）ではなく名前ベースで行う。** PID 変数は呼び出しを跨ぐと失われるため、各起動の前に必ず次を実行して残存プロセスを止める。

   ```powershell
   Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force
   Start-Sleep 1
   ```
3. **実ポートは固定 8080 と決め打ちせず、起動ログから取得して使う。** 8080 が占有・採番された場合に `http://localhost:8080` 決め打ちだと軒並み外れて誤 FAIL を量産する。実ポートの取得方法は 5-3 / 2-1 参照。
4. **`curl` は Git Bash で実行する。** PowerShell では `curl` が `Invoke-WebRequest` のエイリアスのため `-i -X POST -H ...` 形式は失敗する。PowerShell で叩くなら `curl.exe` と明示するか `Invoke-WebRequest` 版を使う（各 curl 節に PowerShell 版を併記）。
5. **APP_ENV は設定しない（既定 development で検証する）。** `APP_ENV=production` を設定すると SESSION_SECRET の厳格検証（base64 32 文字以上）が走り、短い鍵を渡すと起動失敗する（config.go）。production 検証は本受入のスコープ外。

### コマンドの前提と表記

- **シェルは PowerShell と Git Bash の双方を要所で示す。** Claude Code on Windows の既定 Bash ツールは Git Bash 相当だが、`Resolve-DnsName` 等 Windows 固有のコマンドレットは PowerShell（`powershell -Command "..."`）で実行する。
- **パスの環境変数展開はシェルで異なる。** 必ず使い分けること。
  - PowerShell: `$env:LOCALAPPDATA`（例: `$env:LOCALAPPDATA\go_iot\app.db`）
  - コマンドプロンプト（cmd）: `%LOCALAPPDATA%`（例: `%LOCALAPPDATA%\go_iot\app.db`）
  - Git Bash: `$LOCALAPPDATA`（バックスラッシュは `/` でも可。例: `"$LOCALAPPDATA/go_iot/app.db"`）
- 典型展開先は `C:\Users\<ユーザー名>\AppData\Local\go_iot\`。配下に `app.db`、GUI ビルド時は `app.log`、CSRF 認証鍵 `session_secret` が置かれる。
- `sqlite3` CLI は **Windows 標準では同梱されない**。`winget install SQLite.SQLite` 等で別途導入するか、各項目の「sqlite3 が無い場合の代替」（アプリの `/health` ＋起動ログ）を用いる。
- **重要な前提（mDNS 単一インスタンス）**: 同一 LAN に本アプリを 2 台以上同時起動しない。`go-iot.local` の名前が衝突し誤端末へ静かに到達しうる（プローブ/衝突解決は未実装）。検証中は LAN 内 1 台のみ起動する。

---

## 0. 事前準備とビルド

> macOS/Linux でクロスビルドして `.exe` を作る運用が基本。Windows 上でネイティブビルドする場合の差分も併記する。Claude が直接ビルドできる箇所は【自動】。

### 0-1. Go と make の確認 【自動】

```bash
# Git Bash / PowerShell 共通
go version          # go1.2x が出ること（CGO_ENABLED=0 でビルドするため C コンパイラは不要）
make --version 2>/dev/null || echo "make なし: 0-3 の go build フォールバックを使う"
```

- Windows に make は標準で無い。無い場合は Chocolatey（`choco install make`）/ winget（`winget install GnuWin32.Make`）/ scoop（`scoop install make`）で導入するか、0-3 の素の `go build` を使う。
- CRLF 改行だと make が `\r: command not found` を出す。その場合は `git config --global core.autocrlf input` 後に再 clone する。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 0-2. console 版・GUI 版を別名で保持してビルド 【自動】

> **注意（上書き問題）**: `make build-windows` と `make build-windows-gui` はどちらも `-o dist/go_iot.exe` を出力先にするため、後にビルドした方が同一ファイルを上書きする。両方を保持して検証するには、片方をビルド後に別名へ退避してから他方をビルドする。

```bash
# Git Bash 推奨（cp/rm が使え Makefile 相性が最良）
# 1) console 版をビルド → 別名退避
make build-windows
cp dist/go_iot.exe dist/go_iot-console.exe

# 2) GUI 版をビルド → 別名退避（dist/go_iot.exe を上書きするので先に退避済み）
make build-windows-gui
cp dist/go_iot.exe dist/go_iot-gui.exe

# 3) 生成物を確認（2 つの別名 exe が存在すること）
ls -la dist/go_iot-console.exe dist/go_iot-gui.exe

# 4) サブシステム種別を機械判定し、取り違え（両方 console 等）を検知する
#    file があれば console 版= "(console)"、GUI 版= "(GUI)" が出ること
file dist/go_iot-console.exe   # → PE32+ executable (console) ... 期待
file dist/go_iot-gui.exe       # → PE32+ executable (GUI) ...     期待
```

PowerShell 版:

```powershell
make build-windows;     Copy-Item dist\go_iot.exe dist\go_iot-console.exe -Force
make build-windows-gui; Copy-Item dist\go_iot.exe dist\go_iot-gui.exe -Force
Get-Item dist\go_iot-console.exe, dist\go_iot-gui.exe | Format-Table Name, Length
# file が無い場合は PE サブシステムを直接読む（console=3 / GUI=2）
function Get-PESubsystem($path) {
  $b = [IO.File]::ReadAllBytes($path); $pe = [BitConverter]::ToInt32($b, 0x3C)
  [BitConverter]::ToUInt16($b, $pe + 0x5C)   # 3=console, 2=GUI(windows)
}
"console版 Subsystem = $(Get-PESubsystem 'dist\go_iot-console.exe')"  # 3 期待
"gui版     Subsystem = $(Get-PESubsystem 'dist\go_iot-gui.exe')"      # 2 期待
```

期待結果: `dist/go_iot-console.exe`（PE32+ console / Subsystem=3）と `dist/go_iot-gui.exe`（PE32+ GUI / Subsystem=2）が生成される。各々約 27MB 程度（実測 27,190,272 バイト）の単一ファイル。追加同梱ファイルは不要（R1.1/R1.2/R1.3）。

合否: [ ] PASS / [ ] FAIL　メモ（各サイズ・サブシステム）: ____________

### 0-3. make 非依存の go build フォールバック 【自動】

> make が無い／使えない端末向け。**go:embed 同梱物（CSS・マイグレーション・templ）が古い/欠落しないよう、素の go build の前に前段 3 ステップ（sync-css・sync-migrations・templ generate）を手動で済ませる**こと。これらは通常 make が前段で実行している。
> なお Version は make 版が git SHA を注入するのに対し、ここでは固定 `dev` とするが、`view.Version` はキャッシュバスティング用途のため実害はない。

```bash
# 前段（make を使えるなら make build-windows が自動でやる部分）
mkdir -p internal/view/public/css internal/migrate/migrations
cp mocks/html/style.css internal/view/public/css/style.css
rm -f internal/migrate/migrations/*.sql
cp db/migrations/*.sql internal/migrate/migrations/
go tool templ generate

# console 版（Windows 上ネイティブなら GOOS/GOARCH は不要）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=dev" \
  -o dist/go_iot-console.exe ./cmd/server

# GUI 版（ldflags に -H windowsgui と applog.Mode=file を追加）
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags "-s -w -H windowsgui -X github.com/HiroshiKawano/go_iot/internal/applog.Mode=file -X github.com/HiroshiKawano/go_iot/internal/view.Version=dev" \
  -o dist/go_iot-gui.exe ./cmd/server
```

期待結果: make 無しでも 0-2 と同等の 2 つの exe が生成される。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 0-4. cgo 不使用・PostgreSQL ドライバ不在の確認 【自動】

> **重要（偽 FAIL 回避）**: 検索語に `postgres` を含めてはならない。goose ライブラリ（`github.com/pressly/goose/v3/internal/dialects`）が全 dialect（postgres/mysql/sqlite3 等）を**無条件で同梱**するため、正しい pure-Go SQLite ビルドでも `postgres` という文字列は必ず複数件ヒットする（実測 13 件・すべて `*dialects.postgres` 由来で無害）。これを不在判定に使うと**正しいビルドを不合格扱いしてしまう**。実ドライバの import パスである `jackc/pgx`（pgx）と `lib/pq` のみを 0 件期待で判定すること。

```bash
# 実 PostgreSQL ドライバ (pgx / lib/pq) が未リンクであること（0 件期待）
strings dist/go_iot-console.exe 2>/dev/null | grep -ci "jackc/pgx\|lib/pq" || echo "実pgドライバ 0 件 (OK)"
# pure-Go SQLite (modernc) が含有されること（多数ヒット=OK。実測 ≈4812 件）
strings dist/go_iot-console.exe 2>/dev/null | grep -ci "modernc"

# リポジトリにコンテナ構成ファイルが無いこと（R10.1）
ls docker-compose.yml compose.yml docker-compose.yaml Dockerfile 2>/dev/null || echo "コンテナ構成なし (OK)"
```

> Windows に `strings` が無い場合の PowerShell 代替。`Select-String -Encoding Byte` は**実行不能**（Byte は -Encoding の有効値でない／NUL を含む exe で行分割が破綻）なので使わないこと。下記のように一旦 ASCII 文字列化して NUL で分割してから検索する。あるいは Git Bash の `strings`/`grep` に一本化する。
> ```powershell
> $txt = [System.Text.Encoding]::ASCII.GetString([IO.File]::ReadAllBytes('dist\go_iot-console.exe')) -split "`0"
> ($txt | Select-String -Pattern 'jackc/pgx|lib/pq' -AllMatches).Count   # 0 期待
> ($txt | Select-String -Pattern 'modernc'         -AllMatches).Count   # 多数=OK
> ```

期待結果: 実 pg ドライバ（jackc/pgx・lib/pq）0 件、modernc 含有、コンテナ構成ファイル不在（R1.2/R7.4/R10.1）。実測参考値: jackc/pgx=0, lib/pq=0, modernc≈4812, postgres=13（goose 由来・無害なので判定に使わない）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 0-5. 別 Windows 端末へのコピー準備（R-1 / R1.4） 【目視・別端末】

> 配布性（別端末でランタイム無しで起動）の検証はセクション 1 で行う。ここではコピーを準備する。

- `dist/go_iot-console.exe`（または `-gui.exe`）を USB / 共有フォルダ経由で**検証用の別 Windows 端末**へコピーする。
- コピー先端末には Go / make / DB サーバ / Docker / 組込ブラウザを**インストールしない**こと（追加ランタイム不要を検証するため）。
- 別端末が無い場合は本端末で代替できるが、「別端末でランタイム不要」だけは ESP32/別端末でしか厳密確認できない点を記録する。

合否: [ ] PASS / [ ] FAIL　メモ（コピー先端末名）: ____________

---

## 1. 配布・起動スモーク（R1.4 / R-1）

### 1-1. 別 Windows 端末でダブルクリック起動 【目視・別端末】

目的: 生成 .exe を別 Windows 端末へコピーして実行すると、追加ランタイム（別 DB サーバ / コンテナ基盤 / 組込ブラウザ）のインストール無しで起動する（R1.4）。

手順（人間が別端末で実施）:
1. コピーした `go_iot-console.exe` をダブルクリックする。
2. コンソール窓にログが流れ、既定ブラウザでログイン画面が開くことを確認する。
3. （任意）console 版なら起動ログに `待受開始: http://localhost:<ポート>` と `database ready: ...` が出ることを確認する。

代替（別端末が無い場合・自動近似）: 本端末上で `dist/go_iot-console.exe` をバックグラウンド起動し、起動ログに `待受開始: http://localhost:<port>` が在り、かつ `ブラウザ自動起動に失敗しました` が**無い**ことを確認すれば、待受開始とブラウザ起動試行までは自動判定できる（起動の試行成立）。窓・画面の最終目視と「別端末でランタイム不要」は近似に留まる旨を明記する。

ESP32/別端末でしか確認できないこと: **Go/DB/Docker を一切入れていないクリーンな別 Windows 端末**での起動成立。

期待結果: 別端末で追加インストール無しに起動し、ブラウザにログイン画面が表示される。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 1-2. 単独 .exe であることの確認 【自動】

目的: 生成物が追加同梱ファイルを要しない単独の .exe である（R1.3）。

```bash
# exe 単体を任意の空ディレクトリへコピーして起動できる＝同梱物不要
mkdir -p /tmp/go_iot_smoke && cp dist/go_iot-console.exe /tmp/go_iot_smoke/
ls -la /tmp/go_iot_smoke/   # exe 1 ファイルのみ
```

期待結果: exe 単体のコピーで起動可能（CSS/マイグレーション/テンプレートは go:embed 済み）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 2. ゼロ設定起動とデータ保存場所（R2）

> このセクションは console 版（`go_iot-console.exe`）で行うとログが標準出力に出て自動判定しやすい。
> **クリーン状態を作る**: 既存データを退避してから起動すると「初回起動で自動作成」を確認できる。

### 2-0. 既存アプリデータの退避（クリーン初回起動の準備） 【自動】

```powershell
# PowerShell: 既存 go_iot データを退避（消さずに rename）
$d = "$env:LOCALAPPDATA\go_iot"
if (Test-Path $d) { Rename-Item $d "$d.bak_$(Get-Date -Format yyyyMMdd_HHmmss)" }
Test-Path $d   # False になること（クリーン状態）
```

```bash
# Git Bash 版
if [ -d "$LOCALAPPDATA/go_iot" ]; then mv "$LOCALAPPDATA/go_iot" "$LOCALAPPDATA/go_iot.bak_$(date +%Y%m%d_%H%M%S)"; fi
[ -d "$LOCALAPPDATA/go_iot" ] && echo "残存(NG)" || echo "クリーン(OK)"
```

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 2-1. env/.env なしでサーバ待受まで到達（R2.1） 【自動】

目的: env/.env/コンテナ基盤がいずれも無くても、必須設定欠如のハードフェイル無くサーバ待受まで到達する（R2.1）。

> 実ポートは 8080 と決め打ちせず、起動ログ（console は標準出力、GUI は app.log）の `待受開始: http://localhost:<ポート>` から読み取って後続で使うこと（実行規約 3）。下記は標準出力をファイルに落として実ポートを抽出する。

```powershell
# PowerShell: 必須 env を未設定にし、標準出力をファイルへ落としてバックグラウンド起動（APP_ENV は設定しない）
Remove-Item Env:DATABASE_URL, Env:SESSION_SECRET, Env:APP_PORT, Env:LOG_FILE, Env:APP_ENV -ErrorAction SilentlyContinue
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
$out = "$env:TEMP\go_iot_stdout.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep -Seconds 3
# 起動ログから実ポートを抽出
$port = (Select-String -Path $out -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
"実ポート = $port"
# /health が 200 を返せば待受到達（DB 疎通込み）
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode   # 200 期待
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).Content       # {"status":"ok"}
```

期待結果: 必須欠如エラーで落ちず、`/health` が 200 `{"status":"ok"}`（R2.1）。

合否: [ ] PASS / [ ] FAIL　メモ（実ポート）: ____________

### 2-2. app.db が %LOCALAPPDATA%\go_iot に自動作成（R2.2/R2.3/R2.5） 【自動】

目的: 初回起動で DB が無ければ既定パス `%LOCALAPPDATA%\go_iot\app.db` に自動作成し、ディレクトリも自動作成する。Program Files 等の書込制限領域を既定にしない（R2.2/R2.3/R2.5）。

```powershell
# PowerShell: 既定パスに app.db が実在すること
Test-Path "$env:LOCALAPPDATA\go_iot\app.db"           # True 期待（R2.2）
Test-Path "$env:LOCALAPPDATA\go_iot"                  # True 期待（ディレクトリ自動作成 R2.3）
"$env:LOCALAPPDATA"                                    # C:\Users\<user>\AppData\Local（Program Files でない R2.5）
Get-Item "$env:LOCALAPPDATA\go_iot\app.db" | Format-List FullName, Length, CreationTime
```

```bash
# Git Bash 版
ls -la "$LOCALAPPDATA/go_iot/app.db" && echo "app.db 実在(OK)"
echo "$LOCALAPPDATA" | grep -iv "program files" >/dev/null && echo "ProgramFiles配下でない(OK)"
```

DB ファイルが CWD（exe を置いたフォルダ）に作られて**いない**ことも確認:

```powershell
Test-Path .\app.db        # False 期待（CWD に作られていない）
Test-Path .\go_iot.sqlite # False 期待（開発時の相対パスでない）
```

期待結果: `%LOCALAPPDATA%\go_iot\app.db` が実在し、`%LOCALAPPDATA%` は `Program Files` 配下でなくユーザープロファイル配下。CWD には DB が作られない（R2.2/R2.3/R2.5）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 2-3. DATABASE_URL 指定が既定より優先（R2.4） 【自動】

目的: `DATABASE_URL` 指定時は既定値より優先しその指定パスを使う（R2.4）。

> **重要（Windows パスの file: DSN 表記）**: バックスラッシュ生パスの `file:C:\...\app.db` は SQLite URI パーサが `\` を区切りと解釈せず literal バックスラッシュ名のファイルを黙って作るため**不可**。forward-slash 化しドライブレター前に `/` を付した `file:/C:/.../custom.db` 形式を使う（既定の自動生成 DSN もこの形式）。

```powershell
# 先行プロセスを名前で停止（PID 変数に依存しない）
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

# 任意の別パスを指定して起動
$custom = "$env:TEMP\go_iot_custom\custom.db"
New-Item -ItemType Directory -Force -Path (Split-Path $custom) | Out-Null
$env:DATABASE_URL = "file:/$($custom -replace '\\','/')"   # 例 file:/C:/Users/.../custom.db
$out = "$env:TEMP\go_iot_stdout2.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep -Seconds 3
$port = (Select-String -Path $out -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
Test-Path $custom          # True 期待（指定パスに DB ができる R2.4）
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode   # 200 期待
# 後始末
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force
Remove-Item Env:DATABASE_URL
```

期待結果: `DATABASE_URL` で指定したパスに DB が作られ、既定 `%LOCALAPPDATA%\go_iot\app.db` は（このセッションでは）使われない（R2.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 3. マイグレーション自動適用（R4）

> このセクションの一次根拠は**アプリ起動による自動適用**である。テーブル作成は起動時に `internal/migrate` が go:embed 済みマイグレーションを適用して完了するため、`/health` 200 ＋ 起動ログ `database ready: <dsn>` で十分自動確認できる。`sqlite3` CLI 直クエリは CLI がある場合の精密確認（任意）に位置づける（配布 exe 単体にも素の Windows にも sqlite3 CLI は含まれない）。

### 3-1. 全 7 テーブルが待受開始前に自動作成（R4.1） 【自動】

目的: 初回起動で DB が空なら必要な全テーブルを作成するスキーマ適用を待受開始前に自動実行する（R4.1）。

一次根拠（CLI 不要・自動）:

```powershell
# /health が 200 = DB オープン＋疎通成功＝スキーマ適用が待受前に完了している
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode  # 200 期待
# 起動ログ（console 窓 / 標準出力リダイレクト先 / GUI 版 app.log）に「database ready: ...」が出ていること
Select-String -Path $out -Pattern "database ready" | Select-Object -Last 1
```

精密確認（`sqlite3` CLI がある場合のみ・任意）:

```bash
# Git Bash（sqlite3 が PATH にある前提）
sqlite3 "$LOCALAPPDATA/go_iot/app.db" ".tables"
# 期待: users devices device_tokens sensor_readings alert_rules alert_histories sessions（＋ goose_db_version）
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;"
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT version_id FROM goose_db_version ORDER BY version_id;"
# 期待: 0,1,2,3,4,5,6,7（初期 + 7 マイグレーション 00001〜00007）
```

期待結果: `/health` 200 ＋ `database ready` ログ。CLI がある場合は 7 テーブル（users / devices / device_tokens / sensor_readings / alert_rules / alert_histories / sessions）＋ `goose_db_version`（最大 version_id=7）が存在（R4.1）。

合否: [ ] PASS / [ ] FAIL　メモ（テーブル一覧／health）: ____________

### 3-2. 再起動で差分なし no-op・既存データ不変（R4.3） 【自動】

目的: 2 回目以降で最新スキーマなら既存データを破壊せず、差分なしなら no-op（R4.3）。

```powershell
# 1) 現行プロセスを名前で停止
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

# 2) 再起動前の app.db のサイズを記録
$db = "$env:LOCALAPPDATA\go_iot\app.db"
$before = (Get-Item $db).Length

# 3) 再起動 → 数秒待ち → 停止
$out = "$env:TEMP\go_iot_stdout3.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep -Seconds 3
$port = (Select-String -Path $out -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode  # 200 期待
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force

# 4) サイズが極端に変わっていない（テーブル再作成・データ破壊が無い）こと
$after = (Get-Item $db).Length
"before=$before after=$after"
```

`sqlite3` があれば、再起動前後で `goose_db_version` の最大バージョンが不変（7 のまま増えも減りもしない）かつ既存行が残ることを確認:

```bash
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT MAX(version_id) FROM goose_db_version;"  # 再起動後も 7
```

期待結果: 再起動でテーブルは再作成されず（既存データ不変）、最新適用済みなので no-op（R4.3）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 3-3. 未適用差分のみを適用（R4.2） 【自動・任意】

目的: 未適用のスキーマ差分が存在する場合、その差分のみを適用する（R4.2）。

> R4.2 は `internal/migrate` の冪等／差分適用ユニットテスト（goose ライブラリ + modernc）で担保済み。実機での観測は新規マイグレーション追加時のみ可能なため任意とするが、再現したい場合は以下のダミー 8 本目追加で確認できる。

```bash
# 1) ダミー 8 本目マイグレーションを追加（テーブルを 1 つ作るだけの no-harm SQL）
cat > db/migrations/00008_acceptance_probe.sql <<'SQL'
-- +goose Up
CREATE TABLE acceptance_probe (id INTEGER PRIMARY KEY, note TEXT);
-- +goose Down
DROP TABLE acceptance_probe;
SQL

# 2) 再ビルド（go:embed に 8 本目を取り込む）
make build-windows && cp dist/go_iot.exe dist/go_iot-console.exe

# 3) 再起動（クリーンにせず既存 app.db のまま）→ /health 200 を確認後に停止（実行規約に従いバックグラウンド起動）
#    PowerShell 側で起動・停止（名前ベース）

# 4) sqlite3 がある場合: goose の MAX(version_id) が 7→8 へ増分し、既存 7 テーブルは再作成されない（行数不変）
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT MAX(version_id) FROM goose_db_version;"  # 8 期待
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT name FROM sqlite_master WHERE type='table' AND name='acceptance_probe';"  # 1 行

# 5) 後始末: ダミーを除去して再ビルドし元へ戻す
rm -f db/migrations/00008_acceptance_probe.sql
make build-windows && cp dist/go_iot.exe dist/go_iot-console.exe
```

期待結果: 8 本目のみが追加適用され（MAX が 7→8）、既存テーブルは再作成されない（R4.2）。実施しない場合は「migrate_test で担保済み・実機観測は任意」をメモに記す。

合否: [ ] PASS / [ ] FAIL / [ ] 未実施（ユニットテスト担保）　メモ: ____________

### 3-4. （参考）fail fast の理解（R4.4） 【自動・確認のみ】

- スキーマ適用が失敗したら原因把握できるエラーを記録し、待受開始せず起動中断する（R4.4・fail fast）。正常端末では失敗を再現しにくいため、ここでは「自動テストで担保済み」を確認に留める。
- 既知の残存リスク（部分適用）: goose は各ファイルを単一トランザクションで実行するが、途中ファイル失敗時に先行 commit 済みファイルは巻き戻らず**部分適用（中途バージョン）が残りうる**。fail fast でログ記録・起動中断はするが自動修復はしない。

合否: [ ] PASS（理解・確認） / [ ] FAIL　メモ: ____________

---

## 4. セッション鍵（CSRF 認証鍵）と再起動後ログイン維持（R3）

> 用語注意: 要件の「SESSION_SECRET（セッション署名鍵）」の実体は **CSRF 認証鍵**。scs セッションは署名鍵を使わず不透明トークン＋DB セッションストアで動く。再起動後のログイン維持（R3.2）の主体は **DB セッションストア＋ app.db の永続**であり、鍵の同一性とは独立。鍵を永続化する効用は「再起動のたびに既発行 CSRF トークンが全無効化されフォーム再読込が必要になるのを防ぐ」点。

### 4-1. 初回起動で鍵を自動生成・永続化（R3.1） 【自動】

目的: 初回起動で SESSION_SECRET 未設定ならランダム鍵を生成しローカルに永続化する（R3.1）。

```powershell
# SESSION_SECRET 未設定での起動後（2 章で起動済み想定）、鍵ファイルが実在すること
$key = "$env:LOCALAPPDATA\go_iot\session_secret"
Test-Path $key                          # True 期待（R3.1）
$content = (Get-Content $key -Raw).Trim()
$content.Length                         # 44 文字前後（base64 で 32 バイト）期待
# base64 text 形式（生バイナリでない）であること（厳密判定: 印字可能 base64 文字＋末尾パディングのみ）
$content -match '^[A-Za-z0-9+/]+={0,2}$'   # True 期待
```

```bash
# Git Bash 版
ls -la "$LOCALAPPDATA/go_iot/session_secret" && echo "鍵ファイル実在(OK)"
wc -c < "$LOCALAPPDATA/go_iot/session_secret"   # 44 前後
```

期待結果: `%LOCALAPPDATA%\go_iot\session_secret` が実在し、base64 text（44 文字前後）で保存される（R3.1）。

合否: [ ] PASS / [ ] FAIL　メモ（鍵の長さ）: ____________

### 4-2. 再起動で既存鍵を再利用（R3.2 鍵の同一性） 【自動】

目的: 永続鍵が存在すれば再起動時に既存鍵を再利用する（R3.2）。

```powershell
# 1) 現在の鍵のハッシュを記録
$key = "$env:LOCALAPPDATA\go_iot\session_secret"
$h1 = (Get-FileHash $key -Algorithm SHA256).Hash

# 2) プロセス停止（名前ベース）→ 再起動 → 数秒 → 停止
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
$out = "$env:TEMP\go_iot_stdout4.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep 3
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force

# 3) 鍵が変わっていないこと（再利用＝同一ハッシュ）
$h2 = (Get-FileHash $key -Algorithm SHA256).Hash
if ($h1 -eq $h2) { "鍵再利用(OK)" } else { "鍵が変わった(NG)" }
```

期待結果: 再起動前後で `session_secret` のハッシュが一致＝既存鍵を再利用（R3.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 4-3. 再起動後のログインセッション維持（R3.2 セッション維持） 【目視】

目的: 再起動前のログインセッションを有効維持する（R3.2、実現主体は DB セッションストア＋ app.db 永続）。

手順（人間がブラウザで実施）:
1. ブラウザでログイン画面（`http://localhost:<実ポート>/login`）にアクセスし、登録済みユーザーでログインする（未登録なら `/register` でユーザー作成。6 章で `make seed` を使う場合は `test@example.com` / `password` でログイン可）。
2. ダッシュボードが表示されることを確認。
3. アプリを停止し、再起動する（4-2 の再起動相当）。
4. **同じブラウザ（Cookie 保持）**で `http://localhost:<実ポート>/dashboard` に再アクセスし、再ログインを求められず表示されることを確認する。

代替（自動・近似）: ブラウザ操作なしの厳密確認は難しいため、4-2（鍵再利用）＋ app.db に `sessions` テーブルが永続することで間接確認とする。完全な「ログイン状態維持」は目視必須。

期待結果: 再起動後も同じブラウザで再ログイン不要にダッシュボードが開く（R3.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 4-4. SESSION_SECRET を env で明示指定（R3.3） 【自動】

目的: SESSION_SECRET を env で明示指定した場合は自動生成せずその鍵を使う（R3.3）。

> 注: `APP_ENV=production` は設定しないこと。production では SESSION_SECRET に base64 32 文字以上の厳格検証が走る。本検証は既定 development で行う。

```powershell
# 1) 鍵ファイルを退避して env で明示指定 → 起動後に鍵ファイルが自動生成されないことを確認
$key = "$env:LOCALAPPDATA\go_iot\session_secret"
if (Test-Path $key) { Rename-Item $key "$key.bak" -Force }
$env:SESSION_SECRET = "dGVzdC1zZWNyZXQtMzJieXRlcy1iYXNlNjQtMDEyMzQ1Ng=="   # base64 44 文字
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
$out = "$env:TEMP\go_iot_stdout5.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep 3
$port = (Select-String -Path $out -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode  # 200 期待
# env 指定時は鍵ファイルを生成しない＝退避後も新規作成されないこと
Test-Path $key            # False 期待（R3.3: env 優先・ファイル生成しない）
# 後始末
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force
Remove-Item Env:SESSION_SECRET
if (Test-Path "$key.bak") { Rename-Item "$key.bak" $key -Force }
```

期待結果: env 指定時はその鍵を使い、鍵ファイルを新規生成しない（R3.3）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 5. ブラウザ自動表示とポート自動採番（R5）

### 5-1. 既定ブラウザで UI を自動表示（R5.1/R5.2） 【目視】

目的: サーバ待受開始時に既定ブラウザで自端末 UI（ログイン画面）を自動表示する。組込ブラウザ不要のアドレスバーが見える通常ブラウザで表示する（R5.1/R5.2）。

手順（人間が画面確認）:
1. `go_iot-console.exe` をダブルクリック起動する。
2. 既定ブラウザ（Edge/Chrome 等）が自動で開き、`http://localhost:<実ポート>` のログイン画面が表示されることを確認する。
3. **アドレスバーが見える通常ブラウザ**であること（組込/キオスク窓ではない）を確認する（R5.2）。

代替（自動・近似）: ブラウザ起動の「試行」までは Claude が自動確認できる。起動ログに `待受開始: http://localhost:<port>` が在り、かつ `ブラウザ自動起動に失敗しました` が**無い**ことを grep で確認すれば、サーバ稼働＋ブラウザ起動試行は成立。窓・画面の最終目視のみ人間に委ねる。

```powershell
Select-String -Path $out -Pattern "待受開始: http://localhost:" | Select-Object -Last 1   # 在ること
Select-String -Path $out -Pattern "ブラウザ自動起動に失敗"      | Select-Object -Last 1   # 無いこと
```

期待結果: 通常ブラウザでログイン画面が自動表示される（R5.1/R5.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 5-2. 既定ポート 8080 占有時に自動採番し採番ポートで到達（R5.3/R5.4） 【自動】

目的: 既定ポートが使用中なら空きポートを自動採番して待受し、その採番ポート（≠8080）で UI/HTTP に到達できる（R5.3）。採番された実ポートを利用者が確認できるよう記録/表示する（R5.4）。R5.3 と R5.4 を 1 フローで連結して観測する。

```powershell
# 1) 8080 をダミーで占有（0.0.0.0:8080。アプリは ":port" ワイルドカード listen のため Any で占有して衝突を誘発）
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Any, 8080)
$listener.Start()

# 2) アプリ起動（8080 が埋まっているので別ポートへ採番されるはず）。標準出力をファイルへ落とす
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
$out = "$env:TEMP\go_iot_port.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep 3

# 3) 起動ログから採番ポートを読み取る（R5.4）
$port = (Select-String -Path $out -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
"採番ポート = $port"
if ($port -ne "8080") { "8080 以外に採番された(OK・R5.3)" } else { "8080 のまま(NG)" }

# 4) 採番ポートへ実際に到達できること（R5.3 と R5.4 を連結観測）
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode   # 200 期待

# 5) 後始末（ダミー listener 解放・プロセス停止）
$listener.Stop()
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force
```

期待結果: 8080 占有時、8080 以外の空きポートで待受し、その採番ポートが起動ログに記録され、そのポートへ `/health` 200 で到達できる（R5.3/R5.4）。

合否: [ ] PASS / [ ] FAIL　メモ（採番ポート）: ____________

### 5-3. 実ポートが記録/表示される（R5.4） 【自動】

目的: ポート自動採番時、実際に待受しているポートを利用者が確認できるよう記録/表示する（R5.4）。

> **ログ参照先の分岐**: console 版は LOG_FILE 未指定だとログを**標準出力にのみ**出し app.log には書かない。app.log を見るのは GUI 版か LOG_FILE 指定時に限る。console 版で実ポートを確認したい場合は、起動窓（標準出力）か、`Start-Process -RedirectStandardOutput` で落としたファイル（本章で使用）を grep する。

```bash
# console 版（標準出力リダイレクト先）から実ポート行を抽出
grep -E "待受開始|http://localhost:" "$TEMP/go_iot_port.txt" | tail -5

# GUI 版: app.log に同じ行が出る（GUI 版のときのみ）
grep -E "待受開始|http://localhost:" "$LOCALAPPDATA/go_iot/app.log" 2>/dev/null | tail -5
```

```powershell
# console 版（リダイレクト先ファイル）
Select-String -Path "$env:TEMP\go_iot_port.txt" -Pattern "待受開始|http://localhost:" | Select-Object -Last 5
# GUI 版（または LOG_FILE 指定の console 版）は app.log
Select-String -Path "$env:LOCALAPPDATA\go_iot\app.log" -Pattern "待受開始|http://localhost:" -ErrorAction SilentlyContinue | Select-Object -Last 5
```

期待結果: 起動ログに実ポートを含む `待受開始: http://localhost:<ポート>` 行が出る（R5.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 6. LAN 受信とアラート（R6）

> このセクションは登録ユーザー・登録デバイス・有効 Bearer トークンが必要。**ソース同梱端末では `make seed` で前提データ（user・デバイス 2 台・アラートルール）を一括自動投入できる**ため、Web UI の手作業を排除できる（推奨）。配布 exe 単体にはトークン発行 CLI・seed が含まれないため、これらはソースのある端末で行う。
> **curl は Git Bash で実行する**（PowerShell では `curl` が `Invoke-WebRequest` のエイリアスのため curl 形式が失敗。PowerShell 版は各節に併記）。

### 6-0. 前提データの準備 【自動】

> **device_token と device の関係（重要）**: `device_tokens` と `devices` は別テーブルで FK は無い。`gen-token` が発行するトークンは **user_id に紐づく**（device_id ではない）。SensorAPI は「そのトークンの user がリクエストの device_id を所有しているか」を `authz.RequireDeviceOwner(device_id, userID)` で検査し、**未所有なら 403・存在しなければ 422** を返す。したがって**同一 user に対してデバイス作成とトークン発行を揃える**こと（不一致だと 6-1 が 201 でなく 403/422 になり原因不明で止まる）。

自動経路（推奨・ソース同梱端末）:

```bash
# 配布 .exe と同じ DB を指すよう DATABASE_URL を設定（forward-slash file: URI）
export DATABASE_URL="file:/$(echo "$LOCALAPPDATA/go_iot/app.db" | sed 's#\\#/#g')"

# 1) make seed で user(id=1)・デバイス2台(id=1,2)・アラートルール(温度>35 / 湿度<30)・履歴を冪等投入
#    seed ログの「✓ device: id=...」「✓ user: id=1」から device_id を取得する
make seed     # または go run ./cmd/seed

# 2) seed が作った user(id=1) に対してトークン発行（user を揃える）
make gen-token user=1 name="検証用センサ"     # または go run ./cmd/gen-token -user=1 -name="検証用センサ"
# → 平文トークンが標準出力に表示される（以降再表示不可）。控えておく。
#   トークンは user_id=1 に紐づく。6-1 では seed が作った device_id（=1 等）を使う。
```

フォールバック（seed が使えない端末）:
1. 【目視】ブラウザで `/register` からユーザー作成、`/devices/create` からデバイス作成（device_id を控える）。
2. 【自動】上記 `gen-token` を**そのユーザーの user_id**で発行する（user を揃える）。

控えた値: user_id = 1　device_id = ______　Bearer トークン = ____________

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 6-1. 有効 Bearer で 201・ローカル DB 保存（R6.1） 【自動】

目的: インターネット非接続でも、同一 LAN の有効 Bearer 付き POST /api/sensor-data に 201 を返しローカル DB に保存する（R6.1）。

```bash
# Git Bash（curl）。<PORT> は実ポート、<TOKEN> は 6-0 のトークン、<DEVICE_ID> は seed が作った id（user_id=1 所有）
curl -i -X POST "http://localhost:<PORT>/api/sensor-data" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"device_id":<DEVICE_ID>,"temperature":25.5,"humidity":60.0,"recorded_at":"2026-06-11T10:00:00Z"}'
# 期待: HTTP/1.1 201 Created、body に "id" と "alerts_fired"
```

```powershell
# PowerShell 版（curl エイリアス回避のため Invoke-WebRequest を使用）
$body = '{"device_id":<DEVICE_ID>,"temperature":25.5,"humidity":60.0,"recorded_at":"2026-06-11T10:00:00Z"}'
$r = Invoke-WebRequest "http://localhost:<PORT>/api/sensor-data" -Method POST `
  -Headers @{Authorization="Bearer <TOKEN>"} -ContentType "application/json" -Body $body -UseBasicParsing
$r.StatusCode   # 201 期待
$r.Content      # {"id":...,"alerts_fired":...}
```

トラブルシュート（201 にならない場合）:
- **403** = トークンの user とデバイス所有者が不一致（6-0 で user を揃え直す）。
- **422** = device_id が存在しない（seed が作った id を確認）。
- **401** = トークンが不正／ヘッダ欠如（認証段。6-3 参照）。

DB 保存の確認（sqlite3 がある場合）:

```bash
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT COUNT(*) FROM sensor_readings WHERE device_id=<DEVICE_ID>;"  # 1 以上
```

期待結果: 201 が返り、`sensor_readings` に行が保存される（R6.1）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 6-2. アラート判定の同期実行・閾値超過で履歴化（R6.2） 【自動】

> （アラート判定ロジックは `service` ユニットおよび `offline_ingest_regression_test` で担保済み。Windows 固有挙動は伴わないため、実機では「受信→同期判定→履歴化」の結合のみ簡易確認する。）

目的: 登録済みデバイスの計測値保存後にアラート判定を同期実行し、閾値超過があれば履歴化する（R6.2）。

```bash
# seed 投入済みなら device に「温度 > 35」ルールが既に存在する。閾値超過の値を送信
curl -s -X POST "http://localhost:<PORT>/api/sensor-data" \
  -H "Authorization: Bearer <TOKEN>" -H "Content-Type: application/json" \
  -d '{"device_id":<DEVICE_ID>,"temperature":40.0,"humidity":60.0,"recorded_at":"2026-06-11T10:05:00Z"}'
# 期待: レスポンス JSON の "alerts_fired" が 1 以上

# DB の alert_histories に履歴が増えること（sqlite3 がある場合）
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT COUNT(*) FROM alert_histories;"   # 増加
```

> seed を使わずルールを手作業で作る場合は、ブラウザの `/alerts/rules` で対象デバイスに「temperature > 35」等を作成しておく（目視）。

UI でも確認（目視・任意）: ブラウザの `/alerts/history` に発火履歴が表示されること。

期待結果: レスポンス `alerts_fired >= 1`、`alert_histories` に履歴が記録される（R6.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 6-3. 不正/未登録トークンは 401・保存しない（R6.3） 【自動】

目的: 不正/未登録トークンの受信 API アクセスには 401 を返し計測値を保存しない（R6.3）。`auth.DeviceAuth` はボディ評価より前に 401 を返すため、**401 は device_id の存在に依らず確定する**。この不変条件を存在/非存在の両 device_id で裏取りする。

```bash
# (a) ヘッダ無し + 存在する device_id → 401
curl -s -o /dev/null -w "(a) %{http_code}\n" -X POST "http://localhost:<PORT>/api/sensor-data" \
  -H "Content-Type: application/json" \
  -d '{"device_id":<DEVICE_ID>,"temperature":25,"humidity":60,"recorded_at":"2026-06-11T10:10:00Z"}'
# 期待: 401

# (b) 不正トークン + 存在する device_id → 401
curl -s -o /dev/null -w "(b) %{http_code}\n" -X POST "http://localhost:<PORT>/api/sensor-data" \
  -H "Authorization: Bearer INVALID_TOKEN_XXXX" -H "Content-Type: application/json" \
  -d '{"device_id":<DEVICE_ID>,"temperature":25,"humidity":60,"recorded_at":"2026-06-11T10:10:00Z"}'
# 期待: 401

# (c) 不正トークン + 存在しない device_id（例 999999） → 401（device_id 非依存で認証段が先）
curl -s -o /dev/null -w "(c) %{http_code}\n" -X POST "http://localhost:<PORT>/api/sensor-data" \
  -H "Authorization: Bearer INVALID_TOKEN_XXXX" -H "Content-Type: application/json" \
  -d '{"device_id":999999,"temperature":25,"humidity":60,"recorded_at":"2026-06-11T10:10:00Z"}'
# 期待: 401（422 や 403 でないこと＝認証が device 評価より先に確定する不変条件）

# 401 アクセス後、保存件数が増えていないこと（事前カウントと比較）
sqlite3 "$LOCALAPPDATA/go_iot/app.db" "SELECT COUNT(*) FROM sensor_readings;"
```

> 参考（valid トークンでの境界・任意）: **valid トークン + 未登録 device_id → 422**、**valid トークン + 他人所有 device_id → 403**。401（認証段）と 422/403（認可・存在段）の切り分けを区別観測すると、401 が認証段で確定する不変条件を裏取りできる。

期待結果: (a)(b)(c) いずれも 401 を返し、`sensor_readings` 件数が増えない（R6.3）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 6-4. インターネット非依存（R6.4） 【自動・簡易】

> （受信・保存・アラート判定ロジックはいずれも純ロジックで OS 非依存・ユニットテストで担保済み。実機ではオフライン経路の結合のみ簡易確認する。）

目的: 受信・保存・アラート判定のいずれも外部インターネットサービスへの依存を持たない（R6.4）。

```powershell
# 端末をインターネットから切断（または機内モード / NIC を内向きのみ）して 6-1〜6-3 を再実行し、
# 同じく 201/401・アラート履歴化が成立することを確認する。
# 簡易確認: アプリ稼働中の外部向け確立済み接続が無いこと（localhost/LAN 内のみ）
$proc = Get-Process go_iot-console -ErrorAction SilentlyContinue
Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue |
  Where-Object { $_.OwningProcess -eq $proc.Id } |
  Select-Object LocalAddress, RemoteAddress, State
```

期待結果: インターネット切断状態でも 6-1〜6-3 が成立。外部向け確立接続が無い（R6.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 7. mDNS 安定ホスト名（R7）

> hashicorp/mdns（pure-Go・cgo 不使用）で `go-iot`（→ `go-iot.local.`）の A＋SRV を 224.0.0.251:5353 へ告知する。**Windows 自身は mDNS リゾルバを持つ（Bonjour 不要）が、環境により `.local` 解決可否が異なる**。最も確実な到達確認は別端末/ESP32 から。
> **本章の一次根拠は起動ログ `mDNS 公開開始: go-iot.local. → <IP> (NIC ...)` である。** `Resolve-DnsName`/`ping` による自端末解決は OS 実装依存で失敗しうる補助確認に過ぎず、解決できなくても公開ログがあれば公開自体は成立している（FAIL にしない）。
> **ログ参照先の分岐**: console 版を素起動するとログは標準出力にのみ出る（app.log には書かない）。console 版でログ近似確認する場合は `Start-Process -RedirectStandardOutput` で落としたファイルか起動窓を見る。GUI 版は app.log に出る。

### 7-1. go-iot.local の mDNS 公開成立（R7.1） 【自動】

目的: サーバ待受中、同一 LAN で解決可能な安定ホスト名 go-iot.local を mDNS で公開する（R7.1）。

一次根拠（自動・確実）— 公開成立ログの確認:

```bash
# console 版（標準出力リダイレクト先）/ GUI 版（app.log）のいずれか実体のあるファイルを見る
grep -E "mDNS 公開開始" "$TEMP/go_iot_port.txt" 2>/dev/null | tail -1
grep -E "mDNS 公開開始" "$LOCALAPPDATA/go_iot/app.log" 2>/dev/null | tail -1
# 期待: 「mDNS 公開開始: go-iot.local. → <LAN IP> (NIC ...)」が出ていること
```

補助確認（自動・近似。成功は加点・失敗でも一次根拠があれば PASS）:

```powershell
# PowerShell: mDNS 名前解決を試みる（OS 内蔵 mDNS 経由）
Resolve-DnsName go-iot.local -ErrorAction SilentlyContinue   # 自端末 LAN IPv4 が返れば加点
ping -n 2 go-iot.local                                        # Windows の ping は内蔵 mDNS で解決を試みる
# 参考（任意）: LLMNR 経路を見たい場合のみ -LlmnrNetbiosOnly を使う（mDNS とは別経路なので主判定には使わない）
```

> 自端末からの解決は OS のループバック最適化や告知 IF と問い合わせ IF の不一致で成立しないことがある。その場合でも公開ログがあれば公開は成立。本命の到達確認は 7-2 の別端末。

ESP32/別端末でしか確認できないこと: **別セグメント/別ホストからの実 mDNS 名前解決と到達**。

期待結果: 起動ログに `mDNS 公開開始` が記録される（一次根拠）。可能なら自端末から `go-iot.local` が自 LAN IP に解決する（加点）。（R7.1）

合否: [ ] PASS / [ ] FAIL　メモ（公開ログ／解決 IP）: ____________

### 7-2. 別端末/ESP32 から go-iot.local へ到達（R7.1/R7.2） 【ESP32/別端末】

目的: 当該ホスト名の名前解決に自端末の現在 LAN IP を応答し、別端末/ESP32 から到達する（R7.1/R7.2）。

手順:
1. 【別端末】同一 LAN の別 PC（または mDNS 対応スマホ）から `ping go-iot.local` / `Resolve-DnsName go-iot.local` し、検証端末の LAN IP が返ることを確認。
2. 【別端末】別 PC から `curl -i -X POST http://go-iot.local:<PORT>/api/sensor-data -H "Authorization: Bearer <TOKEN>" -H "Content-Type: application/json" -d '{...}'` で 201 が返ることを確認。
3. 【ESP32】実 ESP32 ファームから `go-iot.local`（または LAN IP）の `POST /api/sensor-data` へ Bearer 付き送信し 201 で保存されることを確認（R-2）。

代替（別端末/ESP32 が無い場合）: 同一ホストから `curl http://localhost:<PORT>/api/sensor-data` で API 機能自体は確認可能（6 章）。ただし「`go-iot.local` を別セグメント/別ホストが解決して到達」は近似できない。

ESP32/別端末でしか確認できないこと: **実 ESP32 ファームでの go-iot.local 解決・送信成功**、**別ホストからの mDNS 名前解決**、**マルチ NIC（有線＋無線併用端末）での応答 IF 整合**。

期待結果: 別端末/ESP32 から `go-iot.local` が検証端末の LAN IP に解決し、Bearer 付き POST が 201（R7.1/R7.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 7-3. DHCP による IP 変動の追従（R7.2） 【ESP32/別端末】

目的: DHCP による IP 変動後も応答を追従させる（R7.2）。

手順:
1. アプリ稼働中に検証端末の LAN IP を変える（Wi-Fi 切替 / DHCP リース更新 `ipconfig /release` → `ipconfig /renew` / NIC 切替）。
2. 起動ログに `mDNS: 公開を更新しました → <新 IP>` が出る（定期 ticker・既定 10 秒間隔で追従）ことを確認【自動】:

```bash
# console 版はリダイレクト先、GUI 版は app.log
grep -E "mDNS: 公開を更新しました" "$TEMP/go_iot_port.txt" 2>/dev/null | tail -3
grep -E "mDNS: 公開を更新しました" "$LOCALAPPDATA/go_iot/app.log" 2>/dev/null | tail -3
```

3. 【別端末】別 PC から再度 `Resolve-DnsName go-iot.local` し、**新しい IP** が返ることを確認。

代替: IP 変動を起こせない／別端末が無い場合は、ログの再登録行（`mDNS: 公開を更新しました`）出力で近似確認。新 IP への解決自体は別端末必須。

ESP32/別端末でしか確認できないこと: **IP 変動後に別ホストが新 IP へ追従解決すること**。

期待結果: IP 変動後にログで再登録され、別端末が新 IP に解決する（R7.2）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 7-4. mDNS は LAN 限定・インターネット非送出（R7.3） 【自動・近似】

目的: mDNS 公開はローカルネットワーク内に限定され、インターネットへ情報を送出しない（R7.3）。

```powershell
# 5353/UDP・224.0.0.251（マルチキャスト・LAN 限定）のみで、外部宛 UDP 送出が無いことを確認
Get-NetUDPEndpoint | Where-Object { $_.LocalPort -eq 5353 }
# 期待: 5353 のローカルエンドポイントのみ。外部グローバル IP 宛の確立は mDNS では発生しない
```

> マルチキャスト 224.0.0.251 はリンクローカルで TTL によりルータを越えない設計。厳密にはパケットキャプチャ（Wireshark で 224.0.0.251:5353 のみ・外部宛なし）で確認するのが望ましい。

期待結果: mDNS トラフィックは 224.0.0.251:5353 のリンクローカルのみ。インターネットへ送出しない（R7.3）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 7-5. mDNS は cgo 不使用で動作（R7.4） 【自動】

目的: mDNS 公開機能は cgo を用いず単一 .exe・追加ランタイム不要で動作する（R7.4）。

```bash
# 0-4 で pg/cgo 不在は確認済み。mDNS が pure-Go で動く＝追加ランタイム無しで 7-1 の公開ログが出ること
grep -E "mDNS 公開開始" "$TEMP/go_iot_port.txt" 2>/dev/null | tail -1 && echo "mDNS 公開成立(pure-Go・OK)"
grep -E "mDNS 公開開始" "$LOCALAPPDATA/go_iot/app.log" 2>/dev/null | tail -1
```

期待結果: 追加ランタイム無しで mDNS 公開が成立（R7.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 7-6. ポート採番↔mDNS 到達性トレードオフの観察（R5.3 ↔ R7.1） 【自動・観察】

> **既知トレードオフ**: 既定 8080 競合で自動採番が発火すると、A レコードのみ引き port を out-of-band（既定 8080）で持つ ESP32 は新ポートを知れず到達不能になりうる。ESP32 到達性の前提は「**採番が発火していない（実ポート=8080）**」こと。

```bash
# 起動ログで実ポートが 8080 か確認（8080 以外なら ESP32 が到達できない恐れあり）
grep -E "待受開始: http://localhost:" "$TEMP/go_iot_port.txt" 2>/dev/null | tail -1
grep -E "待受開始: http://localhost:" "$LOCALAPPDATA/go_iot/app.log" 2>/dev/null | tail -1
```

期待結果: ESP32 検証時は実ポート=8080 であること。8080 以外に採番されている場合は ESP32 到達不能リスクをメモに記録する。

合否: [ ] PASS（=8080） / [ ] FAIL（採番発火）　メモ（実ポート）: ____________

---

## 8. GUI コンソール窓非表示とログ出力（R9）

> このセクションは GUI 版（`go_iot-gui.exe`）と console 版（`go_iot-console.exe`）を区別して使う。

### 8-1. GUI exe 起動でコンソール窓が出ない（R9.1） 【目視】

目的: GUI ビルドの .exe を起動するとコンソール窓を表示しない（R9.1・残存リスク R-3）。

手順（人間が画面確認）:
1. `go_iot-gui.exe` をダブルクリックする。
2. **黒いコンソール窓（cmd 風の窓）が一切表示されない**ことを確認する。
3. しばらくして既定ブラウザにログイン画面が開くことを確認する（窓は無いがサーバは動いている）。

代替（自動・近似）: 窓の有無は目視必須だが、サーバ稼働とブラウザ起動試行は自動確認できる。

```powershell
# プロセスが稼働している（app.log に待受開始が在りブラウザ失敗が無い）
Select-String -Path "$env:LOCALAPPDATA\go_iot\app.log" -Pattern "待受開始: http://localhost:" | Select-Object -Last 1
Select-String -Path "$env:LOCALAPPDATA\go_iot\app.log" -Pattern "ブラウザ自動起動に失敗"      | Select-Object -Last 1   # 無いこと
# Get-Process の MainWindowTitle 空は「コンソール窓なし」の積極的証拠にはならない（GUI 窓自体が無いプロセスでも空になる）。
# あくまで稼働の傍証に留め、窓の不在判定は目視を本命とする。
$port = (Select-String -Path "$env:LOCALAPPDATA\go_iot\app.log" -Pattern 'http://localhost:(\d+)' | Select-Object -Last 1).Matches.Groups[1].Value
(Invoke-WebRequest "http://localhost:$port/health" -UseBasicParsing).StatusCode               # 200（稼働）
```

ESP32/別端末でしか確認できないこと: なし（同端末の目視で確認）。ただし実機目視が本命（R-3）。

期待結果: GUI 版起動でコンソール窓が出ず、サーバは稼働する（R9.1）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 8-2. GUI 版はログを app.log に出力（R9.2） 【自動】

目的: GUI ビルドで動作中はログを `%LOCALAPPDATA%\go_iot\app.log` に出力する（R9.2・残存リスク R-3）。

> 注意: app.log に出るのは **GUI 版**（`-X applog.Mode=file` 注入）か **LOG_FILE 指定時の console 版**のみ。console 版を素起動した場合はログは標準出力にのみ出て app.log は空振りする。

```powershell
# GUI 版を名前停止してから起動 → app.log が生成・追記されていること
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
Start-Process .\dist\go_iot-gui.exe; Start-Sleep 3
$log = "$env:LOCALAPPDATA\go_iot\app.log"
Test-Path $log                              # True 期待（R9.2）
Get-Content $log -Tail 10                    # 起動ログ（database ready / 待受開始 / mDNS 公開開始 等）
(Get-Item $log).LastWriteTime                # 直近の起動時刻に更新されていること
Get-Process go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force
```

```bash
# Git Bash 版（GUI 版起動後）
ls -la "$LOCALAPPDATA/go_iot/app.log" && tail -10 "$LOCALAPPDATA/go_iot/app.log"
```

期待結果: `%LOCALAPPDATA%\go_iot\app.log` が生成され、起動ログ（実ポート・mDNS ホスト名・DB パス等）が書かれる（R9.2/R5.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 8-3. LOG_FILE 指定が既定より優先（R9.3） 【自動】

目的: LOG_FILE 環境変数指定時は既定値より優先してその指定先へログを出力する（R9.3）。

```powershell
# GUI 版で LOG_FILE を任意パスに指定して起動 → そのパスにログが出ること
$custom = "$env:TEMP\go_iot_custom.log"
Remove-Item $custom -ErrorAction SilentlyContinue
$env:LOG_FILE = $custom
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
Start-Process .\dist\go_iot-gui.exe
Start-Sleep 3
Test-Path $custom                # True 期待（R9.3: 指定先へ出力）
Get-Content $custom -Tail 5
# 後始末
Get-Process go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force
Remove-Item Env:LOG_FILE
```

期待結果: `LOG_FILE` 指定先にログが出力され、既定 app.log より優先される（R9.3）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

### 8-4. console 版はログを標準出力に出す（R9.4） 【自動】

目的: コンソール付きビルドで動作中はログを標準出力に出す（R9.4）。

> console 版はフォアグラウンド起動すると待受でブロックするため、必ずバックグラウンド起動＋出力ファイル方式で確認する（Git Bash の `timeout` は環境により不在のため使わない）。

```powershell
# console 版を標準出力リダイレクトで起動し、標準出力にログが出ること（app.log には出ない）
$out = "$env:TEMP\go_iot_stdout.txt"; Remove-Item $out -ErrorAction SilentlyContinue
Get-Process go_iot-console, go_iot-gui -ErrorAction SilentlyContinue | Stop-Process -Force; Start-Sleep 1
Start-Process .\dist\go_iot-console.exe -RedirectStandardOutput $out -WindowStyle Hidden
Start-Sleep 3
Get-Content $out -Tail 10        # 標準出力に起動ログ（待受開始 等）が出ている期待（R9.4）
# LOG_FILE 未指定の console 版は app.log を作らない（空振りが正常）
Test-Path "$env:LOCALAPPDATA\go_iot\app.log"   # 直前が GUI 版なら残存しうるが、console 版が更新しないことを LastWriteTime で確認可
Get-Process go_iot-console -ErrorAction SilentlyContinue | Stop-Process -Force
```

```bash
# Git Bash 版（run_in_background でバックグラウンド起動し、数秒後に出力ファイルを読む）
# ※ timeout / フォアグラウンド起動は使わない（ブロック・コマンド不在のため）
./dist/go_iot-console.exe > /tmp/go_iot_console_out.txt 2>&1 &
sleep 3
head -10 /tmp/go_iot_console_out.txt
# 後始末
kill %1 2>/dev/null || true
```

期待結果: console 版は標準出力にログを出す（ファイルへは出さない／LOG_FILE 未指定時）（R9.4）。

合否: [ ] PASS / [ ] FAIL　メモ: ____________

---

## 残存リスクと既知トレードオフ

### 残存リスク（別フェーズ・人手/実機検証）

- **R-1（配布性 / R1.4）**: Go/DB/Docker を入れていないクリーンな別 Windows 端末でのコピー起動。本チェックリストでは 1-1 で目視確認するが、CI 自動範囲外。
- **R-2（実 ESP32 到達 / R7.1〜7.3）**: 実 ESP32 が `go-iot.local` を解決して Bearer 付き POST が 201 保存されること。DHCP IP 変動後の追従を含む。別端末/ESP32 が無いと近似に留まる（7-2/7-3）。
- **R-3（GUI コンソール窓非表示＋app.log / R9.1/R9.2）**: 実 Windows で GUI exe にコンソール窓が出ないこと、app.log に書かれること。窓の有無は目視必須（8-1/8-2）。

### 既知トレードオフ（観察し、該当時はメモへ記録）

- **ポート自動採番 ↔ mDNS 到達性**: 8080 競合で採番が発火すると、A のみ引いて port を 8080 固定で持つ ESP32 は到達不能になりうる。ESP32 検証時は実ポート=8080 を前提（7-6）。
- **mDNS ホスト名衝突（単一インスタンス前提）**: 同一 LAN に本アプリを 2 台以上起動すると `go-iot.local` が衝突し誤端末へ静かに到達しうる（プローブ/衝突解決未実装）。検証中は 1 台のみ起動。
- **並行安定性 R8 の保証範囲**: WAL＋busy_timeout(5000)＋SetMaxOpenConns(4) は単一文 auto-commit 前提での SQLITE_BUSY 500 不発を保証。将来 read-then-write Tx を入れると `_txlock=immediate` 付与が必要になりうる。
- **マイグレーション部分適用**: 途中ファイル失敗時に先行 commit 済みは巻き戻らず中途バージョンが残りうる。fail fast でログ・起動中断はするが自動修復はしない。
- **0o600 の Windows 挙動**: `session_secret` の 0o600 は Windows では実質 no-op。保護は `%LOCALAPPDATA%`（ユーザープロファイル）の NTFS ACL に依存。鍵は base64 text 平文ローカル保存（単一ユーザー/ローカル運用前提）。
- **goose dialect 同梱**: バイナリには goose の全 dialect（postgres/mysql/sqlite3）の文字列が含まれるが、実 DB ドライバは pure-Go SQLite（modernc）のみリンクされ、PostgreSQL ドライバ（pgx/lib/pq）は未リンク（0-4 で `postgres` 文字列を不在判定に使わない理由）。
- **steering 陳腐化**: steering（tech.md/structure.md）の PostgreSQL→SQLite 反映は本機能スコープ外で陳腐化のまま（別途 doc 更新推奨）。

### R8 並行安定性（参考・自動テストで担保済み・実機任意）

- **R8.1/R8.2**: 複数デバイス連続送信（書込）×Web UI 閲覧（読取）×期限切れセッション削除（書込）の同時実行で DB ビジー起因の 500 を出さない／一時ビジーは規定待機内で自動リトライ（`concurrent_load_test` 等で担保済み）。実機での負荷再現は任意。簡易確認は 6-1 の POST をループ実行しつつブラウザ閲覧して 500 が出ないことを観察（curl は Git Bash で実行）:

```bash
# 簡易負荷（30 連続 POST）で 500 が出ないこと
for i in $(seq 1 30); do
  curl -s -o /dev/null -w "%{http_code} " -X POST "http://localhost:<PORT>/api/sensor-data" \
    -H "Authorization: Bearer <TOKEN>" -H "Content-Type: application/json" \
    -d "{\"device_id\":<DEVICE_ID>,\"temperature\":25,\"humidity\":60,\"recorded_at\":\"2026-06-11T10:00:0${i}Z\"}"
done; echo
# 期待: すべて 201（500 が混じらない）
```

合否: [ ] PASS / [ ] FAIL / [ ] 未実施　メモ: ____________

---

## 検証結果サマリ（記入用テーブル）

> 「任意/簡易」マーク: ロジックが自動テストで担保済みで実機は簡易確認のみの項目。必須項目と区別する。

| 章 | 要件 | 項目 | 主タグ | 任意/簡易 | PASS/FAIL | メモ |
|---|---|---|---|---|---|---|
| 0-1 | 環境 | Go/make 確認 | 自動 |  |  |  |
| 0-2 | R1.1/1.2/1.3 | console/GUI 別名ビルド・サブシステム判定 | 自動 |  |  |  |
| 0-3 | R1.1 | go build フォールバック | 自動 |  |  |  |
| 0-4 | R1.2/R7.4/R10.1 | 実pgドライバ不在(jackc/pgx・lib/pq)・compose 不在 | 自動 |  |  |  |
| 1-1 | R1.4 | 別端末ダブルクリック起動 | 目視/別端末 |  |  |  |
| 1-2 | R1.3 | 単独 exe 起動 | 自動 |  |  |  |
| 2-1 | R2.1 | ゼロ設定で待受到達(/health 200) | 自動 |  |  |  |
| 2-2 | R2.2/2.3/2.5 | app.db 自動作成・場所 | 自動 |  |  |  |
| 2-3 | R2.4 | DATABASE_URL 優先 | 自動 |  |  |  |
| 3-1 | R4.1 | 全 7 テーブル自動作成 | 自動 |  |  |  |
| 3-2 | R4.3 | 再起動 no-op・データ不変 | 自動 |  |  |  |
| 3-3 | R4.2 | 差分のみ適用（ダミー8本目） | 自動 | 任意 |  |  |
| 3-4 | R4.4 | fail fast 理解 | 自動 |  |  |  |
| 4-1 | R3.1 | 鍵自動生成・永続化 | 自動 |  |  |  |
| 4-2 | R3.2 | 鍵再利用 | 自動 |  |  |  |
| 4-3 | R3.2 | 再起動後ログイン維持 | 目視 |  |  |  |
| 4-4 | R3.3 | SESSION_SECRET env 優先 | 自動 |  |  |  |
| 5-1 | R5.1/5.2 | ブラウザ自動表示・通常ブラウザ | 目視 |  |  |  |
| 5-2 | R5.3/5.4 | 8080 占有→採番ポートで /health 200 | 自動 |  |  |  |
| 5-3 | R5.4 | 実ポート記録/表示 | 自動 |  |  |  |
| 6-0 | R6.1 | 前提データ投入(make seed) | 自動 |  |  |  |
| 6-1 | R6.1 | 有効 Bearer で 201・DB 保存 | 自動 |  |  |  |
| 6-2 | R6.2 | アラート同期判定・履歴化 | 自動 | 簡易 |  |  |
| 6-3 | R6.3 | 不正/未登録は 401(存在/非存在両方)・非保存 | 自動 |  |  |  |
| 6-4 | R6.4 | インターネット非依存 | 自動 | 簡易 |  |  |
| 7-1 | R7.1 | mDNS 公開ログ成立（一次根拠） | 自動 |  |  |  |
| 7-2 | R7.1/7.2 | 別端末/ESP32 から到達 | ESP32/別端末 |  |  |  |
| 7-3 | R7.2 | DHCP IP 変動追従 | ESP32/別端末 |  |  |  |
| 7-4 | R7.3 | LAN 限定・非送出 | 自動/近似 |  |  |  |
| 7-5 | R7.4 | mDNS pure-Go 動作 | 自動 |  |  |  |
| 7-6 | R5.3↔R7.1 | 採番↔到達性トレードオフ観察 | 自動/観察 | 観察 |  |  |
| 8-1 | R9.1 | GUI コンソール窓非表示 | 目視 |  |  |  |
| 8-2 | R9.2 | GUI ログ→app.log | 自動 |  |  |  |
| 8-3 | R9.3 | LOG_FILE 優先 | 自動 |  |  |  |
| 8-4 | R9.4 | console ログ→標準出力 | 自動 |  |  |  |
| R8 | R8.1/8.2 | 並行安定性（簡易負荷） | 自動 | 任意 |  |  |

### 総合判定

- 自動項目（claude-automatable・任意/簡易を除く必須）すべて PASS: [ ] はい / [ ] いいえ
- 目視項目（human-visual: 1-1/4-3/5-1/8-1）すべて PASS: [ ] はい / [ ] いいえ / [ ] 未実施
- ESP32/別端末項目（7-2/7-3）: [ ] PASS / [ ] 近似のみ / [ ] 未実施（ESP32/別端末なし）
- 任意/簡易項目（3-3/6-2/6-4/7-6/R8）: [ ] 確認済 / [ ] ユニットテスト担保で省略
- 残存リスク R-1/R-2/R-3 の扱い: ____________
- 総合: [ ] 受入可 / [ ] 条件付き受入（要フォロー項目あり） / [ ] 受入不可

検証完了日時: ____________　署名: ____________
