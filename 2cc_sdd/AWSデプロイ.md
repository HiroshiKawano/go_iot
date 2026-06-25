# AWS デプロイ手順（go_iot サーバ）— S: AWS MCP セットアップ / A: プロビジョニング〜受け入れ

> 📂 **このファイルは [残作業.md](残作業.md) から AWS デプロイ部分（S 章・A 章・付録A〜E）を分離したもの**（2026-06-25）。ESP8266 実機書き込み（B 章）・現地実証/継続運用（C 章）・付録F〜H は [残作業.md](残作業.md) を参照。本文中の「§B」「C 章」「付録F〜H」への参照は [残作業.md](残作業.md) 内にある。

## 目次・作業順序（S 章から上から順に実施する）

| 順 | 章 | 内容 | 主担当 |
|---|---|---|---|
| 1 | **S** | AWS MCP のセットアップ（接続・認証・最小権限・承認方針）＝**全 MCP 設定の正本** | 人手→AI |
| 2 | **A-1** | プロビジョニング（インスタンス／静的IP／ファイアウォール／DNS） | AI＋承認 |
| 3 | **A-2** | インスタンス初回自動構成（cloud-init launch script・秘密は焼かない） | AI |
| 4 | **A-3** | バイナリのビルドと配布・DBマイグレーション・systemd 起動・日次バックアップ仕込み（**2回目以降のコード更新反映は §A-3-R＝`bash deploy/redeploy.sh` 一発**） | AI（SSH） |
| 5 | **A-4** | 本番ドメインの Caddy 自動TLS 化 → デプロイ後の受け入れ確認 | AI＋承認 |
| — | 付録A | 調査の確度と要確認事項（AWS） | 参照 |
| — | 付録B | 査読で残ったリスク（AWS） | 参照 |
| — | 付録C | 要決定事項（AWS） | 参照 |
| — | 付録D | 背景・方針・役割分担・構成図（環境構築後に読めばよい） | 参照 |
| — | **付録E** | **実デプロイ実行記録（2026-06-23）— 実手順・成功手順・つまづき** | **参照（コマンドはこちら優先）** |

> 🔗 **後続**: A 章完了（`https://<本番FQDN>/health` 200・device_id・Bearer トークン確定）後に [残作業.md](残作業.md) の **B 章（ESP8266 実機書込）→ C 章（現地実証・継続運用）** へ進む。

---

## 🟢 最短再現ルート（2026-06-23 実証済み・既定）— 初めての人はまずこれを読む

> **AWS も Lightsail も初めての人が「上から順に1回で再現する」ための単一の正本フロー**。本文 §S/§A の各節は**設計意図・背景**で、別ルート（マネージド版 MCP・SSO・独自ドメイン購入 等）も併記しているが、**実際に成功したのは下記の一本道だけ**。**値やコマンドが本文と食い違ったら、必ず付録E（実デプロイ実行記録）と本ボックスを優先**する。

**確定した既定（迷ったらこの値）**:

| 項目 | 既定（実証値） | 備考 |
|---|---|---|
| MCP 接続 | **self-host `awslabs.aws-api-mcp-server`（§S-4）** | マネージド版（§S-1）は本案件**未検証＝使わない** |
| 認証 | **IAM ユーザー長期キー（§S-3-1 ルートB）＋ `aws configure --profile go-iot-mcp`** | SSO（ルートA）は使わない＝`aws sso login` **不要** |
| `.mcp.json` 設定 | **§S-6-2 実証済みプレイブックのとおり**（env 全文・退避/復元・書込フェーズ切替） | Claude Code は uvx 直叩きで可・繋がらなければ uv run 形 |
| リージョン/AZ | `ap-northeast-1` / `ap-northeast-1a` | 全 Lightsail 操作に **`--region ap-northeast-1` を明示** |
| bundle / blueprint | **`micro_3_0`（$7/月・1GB・amd64）/ `ubuntu_24_04`** | ⚠️ IPv6専用 `micro_ipv6_3_0` は**選ばない**（IPv4 が無く sslip.io/SSH 不成立） |
| 本番ドメイン | **無し → `<静的IP>.sslip.io`**（例 `57.182.65.19.sslip.io`） | 購入・NS 伝播待ち・Lightsail DNS（create-domain）すべて**不要** |
| TLS | Caddy が **sslip.io ホスト名で Let's Encrypt 自動取得**（TLS-ALPN-01/443） | A-4 で Caddyfile を `<静的IP>.sslip.io { reverse_proxy localhost:8080 }` に |
| SSH | **`get-instance-access-details`** で一時鍵取得（`download-default-key-pair` は AccessDenied で不可） | **privateKey + certKey 両方必須**（`~/.ssh/lightsail-goiot.pem` と `…-cert.pub`・両 600）・ユーザー **`ubuntu`**・**証明書は約13分で失効**＝取得直後にまとめて実行 |
| SSM | **使わない**（SSH 一本で全工程完遂） | |
| 配置 | env=`/opt/go_iot/go_iot.env`(600) / バイナリ=`/opt/go_iot/go_iot_server`・`/opt/go_iot/go_iot_gen-token` | `/etc/...` や `bin/` 配下では**ない** |
| DB パスワード | cloud-init がサーバ内生成 → `/root/go_iot_db_pass`(600)。**人は値を知らない**（`cat` で読む） | URL エンコード不要（記号除去済） |
| 承認ゲート | elicitation はクライアント非対応（`User rejected`）→ §S-5-2 代替 | CONSENT=false＋elicitList 5件除外＋READ_ONLY トグル＋IAM＋denyList |

**一本道の順序（この順に実施）**:
1. **着手前ゲート ①②③④**（直下）→ ただし ③ は **self-host＋ルートB** で進める（SSO/マネージドは飛ばす）。
2. **§S-6-2 実証済みプレイブック**を上から実行（接続→疎通→ポリシー配置→書込フェーズ切替）。S-1/S-3 本文の別ルートは読まなくてよい。
3. **A-1**: `create-instances --blueprint-id ubuntu_24_04 --bundle-id micro_3_0 --ip-address-type dualstack --user-data file://deploy/cloud-init.sh`（**`--tags` は付けない**＝TagResource 未付与で AccessDenied）→ `get-instance` で **ARN（INSTANCE_GUID）取得 → IAM 第3ステートメントを実 ARN に更新（人手・これをしないと次のポート開放が AccessDenied）** → `allocate/attach-static-ip` → `put-instance-public-ports`（22 は `<管理元IP>/32`・80/443 は `0.0.0.0/0`・**IPv4 の cidrs のみ**）。**DNS は作らない**（sslip.io）。
4. **A-2**: cloud-init は**正本 `deploy/cloud-init.sh`**（付録E-4 で4バグ修正済）。**初回ブートで失敗しやすく再実行不可**なので、A-3 の SSH 確立後に scp して `sudo bash deploy/cloud-init.sh` で**手動冪等実行**して完成させる。
5. **A-3**: ローカルで `make sync-css && go tool templ generate` → `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`（amd64 確定）→ scp → systemd `EnvironmentFile`（DB パスは `/root/go_iot_db_pass` から）→ goose（SSH トンネル＋`GOOSE_DBSTRING` で秘密を ps 露出させない）→ service 起動。
6. **A-4**: Caddyfile を `<静的IP>.sslip.io`＋443 自動TLS へ → ブラウザでユーザー登録（**user_id=1**）→ デバイス登録（**mac_address 必須**）→ `gen-token -user=1`（**サーバ上**で実行・DB は localhost）→ 受入確認（curl ログインは **Referer 必須**）。
7. **B（実機）/ C（運用）**は付録F／C 章。

> 💰 **課金開始点**: **A-1 の `create-instances`** で `micro_3_0`=$7/月 の課金が始まる（それ以前は課金ゼロ）。前段⑨の請求アラート（例 10 USD）を有効化してから A-1 へ。

---

## S. AWS MCP のセットアップ（接続・認証・最小権限・承認方針）

> 本セクション S は AI が AWS を触れるようにする**初回セットアップ**（接続・認証・最小権限 IAM・承認ゲート）。**S が完了するまで AI は AWS を一切操作できず、A/B/C はすべて S 完了が前提**。
>
> ⚠️ **要確認**: MCP サーバ名・env 名・AWS API 名・料金・リージョン提供は変動（2026-06 調査時点）。接続前に公式（awslabs.github.io / docs.aws.amazon.com）で再確認すること。特に `claude mcp add` の引数構文は Claude Code 一般仕様に基づく記載（awslabs に Claude Code 専用例なし）ため最新を要確認。

### 🚩 着手前ゲート（S-1 の前に・上から順に）

> **進め方**: **⓪（推奨）→ ① → ② → ③** の順。**Knowledge MCP（ドキュメント参照）は認証も AWS アカウントも不要なので ⓪ で最初に繋げる**（以降「（要確認）」の裏取りに使える）。一方 **`call_aws` を動かす AWS API MCP は ① のアカウント＋③ の IAM が前提**で、最初には繋げない（このセッションは未接続）。① は人が通常ブラウザで実施（chrome-devtools 代行は AWS の bot 検知で不可・後述）、② は AI が入力を聞き取り、③ は AI と人で。④ は後続章の直前まででよい。値が未確定のまま実行・捏造しないこと。

#### ⓪ 先に（推奨）：AI が Knowledge MCP を接続（AWS アカウント不要）

Knowledge MCP は**認証もアカウントも不要**な AWS 公式ドキュメント参照サーバ。最初に繋ぐと、① のサインアップや以降の「（要確認）」を AI が公式で裏取りできる。**AWS API MCP（`call_aws`）はここでは繋がらない**（① のアカウント＋③ の IAM が要るため）。

> ▶ **人 → AI**:「§S-4-2 のとおり Knowledge MCP（`https://knowledge-mcp.global.api.aws`・認証不要）を `claude mcp add` で接続して。」
> → 🤖 AI が `claude mcp add --transport http aws-knowledge -s project https://knowledge-mcp.global.api.aws` を実行。🙋 **人が Claude Code を再起動**（新セッションで有効化）。
> ✅ 再起動後 `/mcp` に `aws-knowledge` が connected で、`search_documentation` ツールが見える。
>
> **認証不要の chrome-devtools MCP**（ブラウザ操作）は、**A-4 の自前アプリ Web UI 操作**（デバイス登録・アラートルール作成）には使える（自前アプリは bot 検知しない・付録F に実績）。ただし **①サインアップ・S-3-1 の IAM コンソールなど AWS の画面は bot 検知で代行不可**＝人が自分の通常ブラウザで手動操作する（2026-06-23 実証）。
>
> 飛ばす場合は、① 以降の「Knowledge MCP で裏取り」は**人が AWS 公式サイトを直接確認**に読み替える。

#### ① 最初に：AWS アカウント開設（🙋 人が通常ブラウザで実施／AI は手順案内のみ）

アカウント開設・課金・本人確認・MFA はブラウザで行う（②の入力も③の IAM 発行もアカウントが要るので最初に済ませる）。**全工程を人が自分の通常ブラウザ（Chrome/Safari 等）で実施する**。当初は chrome-devtools MCP で AI 代行する想定だったが、**AWS のサインアップ/コンソールは CDP 制御ブラウザをセッションごと bot 検知して弾く**ため使えない（下記「進め方」と 2026-06-23 実証）。AI の役割は各画面で入力すべき値の案内のみ。所要 15〜30 分。事前に用意するもの: **クレジットカード**／**SMS か音声を受けられる電話**／**長く使えるメールアドレス**／**認証アプリ（スマホの Google Authenticator・Authy・1Password 等）**。

> ⚠️ **氏名・住所・カード名義はローマ字（半角英数）で入力する**。日本語（漢字/かな）を入れると弾かれる・後でトラブルになることがある（経験則・要確認）。
> - 住所例: `1-2-3 Otemachi, Chiyoda-ku`（番地） / City `Tokyo` / 郵便番号 `100-0001` / 国 `Japan`
> - 氏名例: `Taro Yamada` / 電話例: `+81 90-1234-5678`（**国番号 +81、先頭の 0 を取る**: 090→+81 90）
> ⚠️ 画面の文言・項目順は AWS の改定で変わりうる（要確認）。詰まったら AI に Knowledge MCP（⓪ で接続済みの場合）で最新のサインアップ手順を裏取りさせる。未接続なら AWS 公式サインアップページを人が直接確認。

> 🙋 **進め方（人が通常ブラウザで全工程・標準）**: 普段使いのブラウザ（Chrome/Safari 等）で `https://portal.aws.amazon.com/billing/signup` を開き、下の手順1〜9を順に実施する。**AI 代行（chrome-devtools MCP）は使わない**（AWS のサインアップ/コンソールは CDP 制御ブラウザをセッションごと bot 検知して弾く＝下の ✅実証）。迷ったら「次の画面で何を入れるか」を AI に聞きながら進めればよい。
>
> 🙋 **人だけが扱う機微情報（チャットや他者に渡さない・画面に直接入力）**: ルートパスワード（手順2）／メール確認コード（手順2）／カード番号（手順4）／電話・SMS の CAPTCHA と確認コード（手順5）／サインイン時パスワード（手順7）／MFA 登録（手順8）。
>
> ✅ **実証（2026-06-23・重要）**: 上の chrome-devtools MCP 代行は **AWS サインアップでは実際には使えなかった**。最初の「E メールアドレスを確認」クリックで、CAPTCHA に到達する前に汎用エラー「申し訳ありませんが、リクエストの処理中にエラーが発生しました」が**連続発生**（＝CDP 制御ブラウザはセッション全体が bot 検知される）。リトライや人による同ブラウザ内クリックでも回避できないことがあるため、**人が自分の通常ブラウザ（Chrome/Safari 等）でサインアップ全体を実施**するのが確実（メール確認コード・ルートパスワード・カード・電話確認・MFA はどのみち人の作業）。**chrome-devtools 代行は AWS サインアップでは当てにしない**。

**1. アカウント作成ページを開く**
- ブラウザで https://aws.amazon.com/ を開き、**画面右上の「アカウントを作成」（"Create an AWS Account"）** ボタンを押す。または直接 https://portal.aws.amazon.com/billing/signup を開く。
- 買い物用の Amazon.co.jp アカウントとは**別物**。AWS 専用に新規作成する。

**2. ルートユーザーの E メールとアカウント名を入力**
- **ルートユーザーの E メールアドレス**: これが今後のログイン ID 兼最重要の通知先になる。
  - 個人の使い捨てではなく**長く使える/共有できるメール**にする（後から変更は可能だが手間）。例: `go-iot-aws@example.com`
- **AWS アカウント名**: 画面表示用の**わかりやすい名前**（ログイン ID ではない・後から変更可）。
  - 制限はゆるく英数・スペース等が使える（ローマ字推奨）。メールをそのまま入れることも可能だが、用途が分かる名前が良い。例: `go-iot-prod`
- 「メールアドレスを検証」を押す → メールに届いた**確認コード**を入力。
- 続けて**ルートパスワード**を設定（要確認: 8 文字以上＋大文字・小文字・数字・記号など。パスワードマネージャで強いものを生成し安全に保管）。

**3. 連絡先情報を入力**
- **アカウントの種類**: 会社案件なら**ビジネス**、個人利用なら**個人**（税・請求項目が少し変わるだけ）。
- 氏名・電話番号・国（日本）・住所を**すべてローマ字**で入力（冒頭 ⚠️ の例を参照）。
- **AWS カスタマーアグリーメント**にチェックして同意。

**4. 支払い情報を登録（無料利用枠でも必須）**
- **クレジットカード**番号・有効期限・名義（ローマ字）・請求先住所を入力。
  - デビット/一部プリペイドカードは弾かれることがある（要確認）。
  - 登録時に**少額の与信確認**（例: 1 USD／100 円程度）が一時計上され、後で自動的に取り消される。

**5. 本人確認（電話）**
- 電話番号を入力 → **SMS か音声通話**を選択 → 画面の CAPTCHA（文字）を入力 → 届いた**確認コード**を入力。

> ⚠️ **新画面（2025〜・要選択）「アカウントプランを選択」**: サポートプラン（下記手順6）とは**別に**、サインアップ中に **無料（6 か月）／有料** を選ぶ画面が追加された。**本番運用は必ず「有料」を選ぶ**。「無料(6か月)」は 6 か月後またはクレジット枯渇で**アカウントが自動閉鎖**され、全サービス/機能アクセスやクレジットしきい値超えの拡張も制限される（学習用サンドボックス）。「有料」でもクレジット（最大 100〜200 USD・時期により変動）は同額付与され、**使い切るまで課金は発生しない**ので本番でも初期は実質無料。

**6. サポートプランを選ぶ**
- **ベーシック（無料）** を選ぶ（Developer/Business は有料サポート）。「サインアップ完了」まで進む。

**7. ルートでサインイン**
- 「AWS マネジメントコンソールにお進みください」→ **ルートユーザー**を選び、手順2の E メール＋パスワードでサインイン。

**8. ルートユーザーを保護（MFA 有効化・必須）**
- サインイン後、**画面右上のアカウント名 → 「セキュリティ認証情報」（Security credentials）** を開く。
- **MFA** セクションの **「MFA デバイスを割り当て」（Assign MFA device）** を押す。
- 種類を選ぶ:
  - **認証アプリ（推奨・TOTP）**: スマホのアプリで画面の **QR コードを読み取り**、表示される**連続した 2 つのコード**を入力して登録。
  - または FIDO2 セキュリティキー／ハードウェア TOTP トークン。
- 以後 **ルートユーザーは常用しない**。日常の AWS 操作は ③ で作る最小権限 IAM / IAM Identity Center（SSO）で行い、**ルートのアクセスキーは絶対に作らない**。

**9. （推奨）請求アラートを設定**
- 想定外の課金に早く気づくため、**Billing コンソール → 「Budgets（予算）」→ 「予算を作成」** で月額しきい値（例: 10 USD）とメール通知先を設定する（または CloudWatch 請求アラーム）。
- 「IAM ユーザー/ロールによる請求情報へのアクセス」を有効化しておくと、③ で作る IAM でも請求を確認できる（任意）。

> ✅ **① 完了判定**: ルートでコンソールにサインインでき、**MFA が有効**（再サインイン時に MFA コードを要求される）。請求アラート設定済み（推奨）。→ これで ②（入力の聞き取り）・③（IAM 発行）へ進める。

#### ② 次に：AI が人から聞き取る入力（① 完了後）

**人がこの残作業.md を渡した AI に最初に打つ一言**（これでセットアップが動き出す。コピペ可）:

> ▶ **人 → AI**:
> 「`2cc_sdd/残作業.md` に沿って AWS へデプロイを始めます。① の AWS アカウント作成・課金・MFA は完了済みです。まず §S 着手前ゲート② の入力表に従って、私が決めるべき入力を**一括で質問**してください。後段で確定する値（`<BUNDLE_ID>` 等）は聞かなくて結構です。」

これを受けて AI は下表の不足を一括質問し、回答を控える（以降の章で参照する）。`<BUNDLE_ID>`/`<UBUNTU_BLUEPRINT_ID>`/プロファイル `go-iot-mcp` は ③（接続後）に AI が `get-bundles`/`get-blueprints`・IAM 発行で実値を確定するので、**人は「プラン階級（Micro-1GB 等）」だけ**決めればよい（**CPU アーキは本案件 amd64 で確定**）。

| 入力 | 本文の表記 | 例 | 使う章 | メモ |
|---|---|---|---|---|
| 本番ドメイン（FQDN） | `<host>`/`<本番FQDN>` | **既定＝`<静的IP>.sslip.io`**（無料・購入不要） | A-1/A-4/B | sslip.io で Caddy 自動TLS。独自ドメインは任意（購入・NS伝播は人手・④） |
| 管理元グローバルIP | `<管理元IP>`（+IPv6があれば） | `203.0.113.10/32` | A-1(FW・SSH22限定) | SSH(22) を許す送信元。運用端末の固定IP（動的なら都度更新） |
| 管理者メール | `<ADMIN_EMAIL>` | `admin@example.com` | A-2/A-4(Caddyfile) | Let's Encrypt の通知先 |
| プラン階級 | （→ `<BUNDLE_ID>` を ③ で確定） | Micro-1GB | A-1 | 低メモリ＋同居のため **Micro-1GB 以上推奨** |
| CPU アーキ | **`amd64`（確定）** | amd64 | A-1/A-3(`GOARCH`) | 本案件 amd64 確定。取り違えると `Exec format error` |
| AWS リージョン | `<REGION>` | `ap-northeast-1`（東京） | 全AWS操作 | **DNS/ドメイン系のみ `--region us-east-1`** |
| AZ | — | `ap-northeast-1a` | A-1 | |
| Wi-Fi SSID | `<設置場所SSID>`/`<SSID>` | `farm-wifi` | B(config.h) | 設置場所のもの |
| Wi-Fi パスワード | （config.h はプレースホルダのまま） | （現地で河野さんが入力） | B(config.h) | `config.h` は `.gitignore`・**コミット禁止** |
| SSH 鍵 | — | `~/.ssh/...`（`Host go-iot-prod`） | A-3/C | **人の端末のみ**に置く・権限600・④ |

> **実行中に前段ステップが確定する値（人に聞かない）**: `<BUNDLE_ID>`/`<UBUNTU_BLUEPRINT_ID>`（③ で `get-bundles`/`get-blueprints`）、`go-iot-mcp`（③ の IAM 発行）、`<STATIC_IP>`（A-1 操作2）、`<ACCOUNT_ID>`/`<INSTANCE_GUID>`（A-1 作成後の `GetInstance`）、`<user_id>`（A-4 のユーザー登録）、`<device_id>`（A-4 のデバイス登録）、**平文 Bearer トークン**（A-4 の `gen-token`・再表示不可）。これらは未確定でも正常で、前段の完了判定で埋まる。

#### ③ 次に：AI と共同で初回セットアップ（① 完了後・AI が主導／人は鍵を伴う実行・ブラウザ認証・再起動のみ）

上から順に。各項目の ▶ が**人が打つプロンプト**（コピペ可）、🙋 が**人の手作業**。AI が ▶ を受けて動き、🙋 のところで人に処理を渡す。

- [ ] **前提ツールの導入**（§S-2）
  ▶「§S-2 のとおり `uv`/`uvx` と AWS CLI v2 を Bash で導入して、`uvx --version`・`aws --version` まで確認して。」
  → 🤖 AI が導入・確認。🙋 人は Bash 実行を承認するだけ。
- [ ] **最小権限 IAM の発行**（§S-3）
  ▶「§S-3-2 の最小権限ポリシー JSON を生成して。私が §S-3-1 の手順でコンソールに IAM を作ります。」
  → 🤖 AI が JSON 生成。🙋 **人がコンソールで IAM ユーザー `go-iot-mcp`（§S-3-1 ルートB＝実証済み）を作り、`aws configure --profile go-iot-mcp` で長期キーを保存**（SSO=ルートA は本案件未検証なので使わない）。**AI に admin 鍵は渡さない**（自己権限昇格を防ぐ）。
- [ ] **AWS API MCP の接続**（`call_aws` 用。**既定＝self-host §S-4 ＋ §S-6-2 実証済みプレイブック**。マネージド版§S-1は未検証＝使わない。Knowledge MCP は ⓪ で接続済み）
  ▶「**§S-6-2 実証済みプレイブックのとおり** self-host の `.mcp.json`（`aws-api`）と `mcp-security-policy.json` を作って `claude mcp add` まで実行して。」
  → 🤖 AI が生成・登録。🙋 **人が Claude Code を再起動**（新セッションで MCP 有効化。Cursor は Cmd+Shift+P→Reload Window）。
- [ ] **SSO ログインは不要**（ルートB＝長期キーのため `aws configure` で完了済み）。SSO（ルートA）を選んだ場合のみ作業前に毎回 `aws sso login --profile go-iot-mcp`。

#### ④ 後続章の着手までに（時期は各章の直前でよい。**長納期のものは早めに手配すると中断が減る**）

- [ ] **（任意）独自ドメイン購入**（→ 独自ドメインを使う場合のみ。**既定の sslip.io ならドメイン購入・NS伝播待ちは不要**＝`<静的IP>.sslip.io`・付録E-7）
- [ ] **承認ゲート（elicitation）は非対応と確定済み**（→ A-1 の書込前。Cursor で `User rejected` を実証＝付録E-3。**`REQUIRE_MUTATION_CONSENT=false`＋elicitList から A-1書込5件を一時除外＋READ_ONLYトグル＋IAM＋denyList＋本体プロンプト**に一本化・§S-5-2/§S-6-2）
- [ ] **SSH 経路の確立は確定済み**（→ A-3。`get-instance-access-details` で一時鍵（`download-default-key-pair` は AccessDenied）・**`certKey` 必須**・ユーザー `ubuntu`・約13分失効・鍵は端末のみ＝付録E-5）
- [ ] CloudTrail 証跡（trail）作成と S3 継続配信（任意・推奨。AI 操作の監査基盤）
- [ ] **ESP8266＋SHT31＋データ通信対応 USB ケーブル**（→ B。現地・河野さん。手配リードタイムがあるので早めに。**AWS 操作には不要**）

> ✅ **ゲート通過の判定（AI が S-6 以降の AWS 操作を実行できる条件）**: **①②③ が完了**し、`aws sts get-caller-identity --profile go-iot-mcp` が当該 IAM を返し、Claude Code の `/mcp` で AWS MCP が **connected**。④ は各章の直前までに揃えばよい。

### S-0. このセクションのゴールと不変条件

本セクション完了時の到達点は次のとおり。

| 区分 | ゴール |
|---|---|
| MCP 接続 | Claude Code に **AWS API MCP Server**（任意の AWS CLI を自然言語で実行）と **AWS Knowledge MCP Server**（ドキュメント参照・認証不要）が接続済み |
| 認証 | **Lightsail 操作に限定した最小権限 IAM 認証情報**（専用プロファイル `go-iot-mcp`）が発行され、MCP に供給されている |
| 安全策 | read-only 起動／書込前の人承認ゲート／破壊・billable 操作の denyList・elicitList／CloudTrail 監査／認証情報の非コミット が整備済み |
| 疎通 | AI に「Lightsail のインスタンス一覧を見せて」と依頼し、**読み取り操作だけで疎通確認**できる |

**MCP 化しても変えない不変条件（A 章以降で AI が守るべき設計の核）**:

- 低メモリ Lightsail 前提。PostgreSQL は同一インスタンスに **native apt**（Docker 不使用）、**swap 必須**。
- **サーバでビルドしない**。ローカル(Mac)で `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`（純 Go）したバイナリのみ配布。CSS/Scalar は go:embed 同梱。**sync-css → templ generate → build** の順。
- アプリは平文 HTTP で **`:8080` を `0.0.0.0` bind**（TLS 自前なし）。HTTPS は前段 **Caddy 自動 TLS（Let's Encrypt）**。
- 公開は **80/443（+ 接続元 IP 限定の SSH 22）のみ**。**8080 と PostgreSQL 5432 は外部公開禁止**。
- 環境変数（`DATABASE_URL` / `SESSION_SECRET`[本番 32 文字以上] / `APP_ENV=production` / `APP_PORT`）は **`.env` 自動読込なし** ＝ **systemd の `EnvironmentFile`／`Environment` で供給**。秘密ファイルは権限 `600`。

> 🔒 **userData（cloud-init）に秘密を平文で焼かない（最重要・不変条件）**: `create-instances --user-data` に渡すスクリプトは **作成 API のリクエストとして送られ、CloudTrail やコンソールに残りうる**。したがって `DATABASE_URL`（DB パスワード入り）・`SESSION_SECRET`・Bearer トークンを **userData に直書きしてはならない**。userData では「swap／PostgreSQL（native apt）／Caddy／systemd unit の雛形配置」までに留め、**秘密値は A 章で初回 SSH 後に systemd の `EnvironmentFile`（権限 600）として配置**する（DB パスワードもサーバ上で生成・設定）。DB は `localhost:5432` 外部非公開のため、`goose migrate-up` / `cmd/gen-token` はいずれも**サーバ上で実行**する（手元 Mac からは DB に到達できない）。

> **MCP では賄えない作業の境界（重要）**: AWS API MCP の `call_aws` は **AWS API のみ**を実行し、SSH/scp や対話的セッションのストリームは扱わない。したがって **AWS リソース層（Lightsail インスタンス／静的 IP／ポート／DNS）= AWS MCP**、**インスタンス内の初期構成（秘密以外）= `create-instances --user-data`（cloud-init）に渡しきる**、**デプロイ反復（バイナリ配布・systemd 再起動・ログ確認・トークン発行・秘密配置）= SSH を既定**、という三分割で運用する（A 章で詳述）。SSM Run Command 化は将来オプション（S-7 参照）。

---

### S-1. 使う MCP サーバ（2 本）と役割分担

本プロジェクトでは公式（awslabs）の 2 サーバを使う。**Lightsail 専用の公式 awslabs MCP は存在しない**（要確認・2026-06 時点／awslabs/mcp のサーバ一覧と PyPI・GitHub 検索からの推定。PyPI `awslabs-lightsail-mcp-server` は publisher を直接確認できず非公式の可能性が高い）ため、リソース操作は汎用の AWS API MCP の `call_aws` で `aws lightsail ...` を実行する。**本番運用に第三者製 Lightsail MCP を入れるのは最小権限・監査の観点で非推奨。**

| MCP サーバ | パッケージ／エンドポイント | 役割 | 認証 |
|---|---|---|---|
| **AWS API MCP Server** | PyPI `awslabs.aws-api-mcp-server`（`uvx awslabs.aws-api-mcp-server@latest`） | 任意の AWS CLI を自然言語で実行（Lightsail のインスタンス／静的 IP／ポート／DNS／SSM などすべて） | **最小権限 IAM が必須**（S-3） |
| **AWS Knowledge MCP Server** | リモート HTTP `https://knowledge-mcp.global.api.aws` | AWS ドキュメント・手順・リージョン可用性の裏取り（read-only） | **不要**（AWS アカウント不要・レート制限あり） |

**AWS API MCP Server が公開するツールは 3 つ**（粒度＝任意 CLI 実行・個別 API 単位ではない）:

- `call_aws` — AWS CLI コマンドを検証・エラー処理付きで実行。`aws lightsail create-instances ...` 等を叩く本命ツール。
- `suggest_aws_commands` — 自然言語クエリから最も可能性の高い CLI コマンド候補 5 件（説明＋全パラメータ）を提示。モデルが知らない最新 CLI を補える。
- `get_execution_plan` — 複雑タスクの手順ガイド（**実験的**・`EXPERIMENTAL_AGENT_SCRIPTS=true` で有効。将来変更・廃止の可能性あり・要確認）。

**AWS Knowledge MCP Server のツール**: `search_documentation` / `read_documentation` / `recommend` / `list_regions` / `get_regional_availability` / `retrieve_skill`（GA = 2025-10・要確認）。本書中の「（要確認）」の手順・リージョン可用性を、AI が対話中にこのサーバで裏取りできる。なお `get_regional_availability` は **AWS サービス／API のリージョン提供状況**を返すもので、**Lightsail バンドルの月額・スペックの権威ある値は `call_aws` の `aws lightsail get-bundles` で実 ID とともに突き合わせる**こと（Knowledge MCP は補助）。

> **接続手順は §S-4（self-host・既定ルート）と §S-6-2（実証済みプレイブック・コピペ可）にある。** 本書は self-host（`awslabs.aws-api-mcp-server`）一本で実証済み（2026-06-23）。AWS がホストするマネージド版 AWS MCP Server（`mcp-proxy-for-aws`・SSO）は GA 済みだが本案件では未検証のため不採用。

---

### S-2. 前提ツールの用意（🤖 AI が実施・ローカル Bash）

ローカル(Mac)に次を揃える。**AWS MCP は不要**（ローカルシェル操作なので AI が Bash で実行できる。MCP 未接続でも問題ない）。人は Bash 実行を承認し、結果を確認するだけ。

| ツール | 用途 | 導入 |
|---|---|---|
| **`uv` / `uvx`（Astral）+ Python** | AWS API MCP / `mcp-proxy-for-aws` を `uvx` で起動 | Astral 公式手順。必要 Python は要確認（調査時点は 3.10 系・`@latest` で要件が変わりうるので接続前に最新 README を確認） |
| **AWS CLI v2** | `call_aws` がサーバ内部で呼ぶ／手元の疎通検証にも使う | 公式インストーラ |
| **Claude Code（本 CLI）** | MCP クライアント（`claude mcp add` で接続） | 導入済み（このセッション） |

> ▶ **人 → AI（プロンプト例）**: 「§S-2 のツール（`uv`/`uvx`・AWS CLI v2）を Bash で導入して、`uvx --version`・`aws --version` の確認まで実行して。」
>
> 🤖 **AI が実施（Bash）**: 上記を `brew install` / 公式インストーラで導入し、`uvx --version` と `aws --version` で確認する。Knowledge MCP が接続済みなら `search_documentation` で最新の導入要件を裏取りしてよい（読み取りのみ・課金なし）。
>
> ✅ **実証（2026-06-23・macOS）**: `brew install awscli` は**応答が返らず固まった**（環境依存）。確実な回避＝**AWS 公式 .pkg のユーザーローカル展開（sudo 不要）**:
> ```bash
> # uv/uvx（未導入なら）: curl -LsSf https://astral.sh/uv/install.sh | sh   →（~/.local/bin に入る）確認: uvx --version
> # --- AWS CLI v2（公式pkg・no-sudo）。choices.xml も含めコピペで完結 ---
> curl -fsSL https://awscli.amazonaws.com/AWSCLIV2.pkg -o AWSCLIV2.pkg
> cat > choices.xml <<XML
> <?xml version="1.0" encoding="UTF-8"?>
> <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
> <plist version="1.0"><array><dict>
>   <key>choiceAttribute</key><string>customLocation</string>
>   <key>attributeSetting</key><string>$HOME</string>
>   <key>choiceIdentifier</key><string>default</string>
> </dict></array></plist>
> XML
> installer -pkg AWSCLIV2.pkg -target CurrentUserHomeDirectory -applyChoiceChangesXML choices.xml
> ln -sf "$HOME/aws-cli/aws" ~/.local/bin/aws && ln -sf "$HOME/aws-cli/aws_completer" ~/.local/bin/aws_completer
> aws --version   # aws-cli/2.x（実績: 2.35.10。~/.local/bin は uv 同様 PATH 済み）
> ```
> AWS CLI **v2 は PyPI に無い**ため `uvx awscli` は使えない（v1 になる）。`uv`/`uvx` は既導入なら追加作業不要。MCP の事前DL（任意・初回起動を速く）: `uv tool install awslabs.aws-api-mcp-server`。
>
> 🙋 **人が確認**: Bash 実行を承認し、返ってきた版が想定どおりか目視する。
>
> ✅ **完了判定**: `uvx --version` がバージョンを返し、`aws --version` が `aws-cli/2.x` を返す。

---

### S-3. 最小権限 IAM の発行（🙋 人が実施＝手順書／🤖 AI が補助）

MCP 用の AWS 認証情報の初回発行は**人がコンソールで行う**。AI には設計上 admin 権限を渡さない（AI が自分の権限を広げる＝自己権限昇格を防ぐ）ため、**IAM プリンシパルの作成は AI には不可能**。AI の役割は「貼り付ける**ポリシー JSON の生成（S-3-2）**」と「**プロファイル設定・SSO ログイン起動（S-3-3）**」に限る。公式も AWS API MCP に最小権限 IAM を必須と明記（要確認・出典: awslabs セキュリティ注記）。本サーバはマルチテナント非対応のため **1 ユーザー専用＋専用認証情報**で動かす（専用化すると CloudTrail で「AI が打った操作」をフィルタしやすい）。

#### S-3-1. 🙋 人が実施 ／ 🤝 AI 共同作業：IAM プリンシパルの発行（手順書）

AWS マネジメントコンソールで MCP 用の認証情報を作る。**ルートではなく**可能なら管理用の別 IAM 管理者で操作する（初回はルートでも可）。**先に §S-3-2 のポリシー JSON を AI に生成させてから**始める。所要 10〜20 分。

**どちらのルートを選ぶか**: **本案件の既定＝ルートB（IAM ユーザー＋長期キー）＝2026-06-23 実証済み**（付録E は全てルートB・`aws configure --profile go-iot-mcp` で完了）。長期キーの保管・ローテを避け SSO 短期クレデンシャルにしたい場合のみ **ルートA（IAM Identity Center / SSO・本案件未検証）**。

> ⚠️ **`AdministratorAccess` を絶対に付けない**（AI が任意操作できてしまう）。必ず §S-3-2 の Lightsail 限定ポリシーを使う。
> ⚠️ **IAM Identity Center はリージョンを持つ**。一度有効化したリージョン（ホーム）に ID ストアが置かれ、後から変えるには削除・再作成が要る。**最初に固定するリージョンを決めてから**有効化する（本書はリソースと同じ `ap-northeast-1` を例にする。組織方針があればそれに従う）。
> ⚠️ 画面の文言・項目位置は AWS の改定で変わりうる（要確認）。詰まったら AI に Knowledge MCP で最新手順を裏取りさせる。

> ⚠️ **chrome-devtools MCP による AI 代行は不可（2026-06-23 実証）**: 当初は chrome-devtools MCP で下記コンソール操作（ユーザー作成・ポリシー貼り付け・割り当て）を AI に代行させる想定だったが、**AWS のコンソール/サインアップは CDP 制御ブラウザをセッションごと bot 検知して弾く**（① と同じ症状）。よって **IAM の発行は人が自分の通常ブラウザで手動実施**する。AI の役割は **§S-3-2 のポリシー JSON 生成**と、③ 接続後の **疎通確認**（`aws sts get-caller-identity` が当該 IAM・Lightsail 読取成功・`ec2 describe-instances` が AccessDenied）に限る。
> - 🙋 **人が手動で実施**: ルート（または管理用 IAM 管理者）で**自分の通常ブラウザ**からコンソールにログインし、下記ルートのいずれかを行う。**`AdministratorAccess` は付けない**（§S-3-2 の Lightsail 限定ポリシーのみ）。
> - 🔒 **機微情報はどこにも貼らない**: アクセスキー ID／シークレット（ルートB）・パスワード・MFA は画面に直接入力し、チャットや他者に渡さない。プロファイル設定は**人のターミナル**で `aws configure --profile go-iot-mcp` を使い、キーは対話入力する。

**ルートA（未検証・任意）: IAM Identity Center の権限セット（SSO・短期クレデンシャル）**

1. **Identity Center を有効化**: コンソール右上のリージョンを**ホームリージョン（例 `ap-northeast-1`）に設定** → 上部の検索に `IAM Identity Center` と入力して開く → **［有効にする］（Enable）**。単一アカウントでも AWS Organizations が自動作成され、このアカウントが管理アカウントになる。
2. **ユーザーを作成**: 左メニュー **［ユーザー］→［ユーザーを追加］** → ユーザー名（例 `go-iot-operator`）・メール・氏名を入力して作成。
   - 本人宛に**招待メール**が届く → リンクから**パスワード設定と MFA 登録**を済ませる（このユーザーはルートとも IAM ユーザーとも別物）。
3. **権限セットを作成**: 左メニュー **［権限セット］→［権限セットを作成］→［カスタム権限セット］**。
   - ポリシーの種類で **［インラインポリシー］** を選び **§S-3-2 の JSON を貼り付け**（事前に作った顧客管理ポリシーのアタッチでも可）。
   - 名前: `GoIotMcpLightsail`（この名前が後で `sso_role_name` になる）。
   - **セッション期間**: 1〜4 時間（`aws sso login` の有効時間。短いほど安全）。
4. **割り当て**: 左メニュー **［AWS アカウント］** → 対象アカウントにチェック → **［ユーザーまたはグループを割り当て］** → 手順2のユーザー＋手順3の権限セットを選び **［送信］**。プロビジョニング完了まで数十秒待つ。
5. **接続情報を控える**: **［設定（Settings）／ダッシュボード］** の **AWS アクセスポータル URL（例 `https://d-xxxxxxxxxx.awsapps.com/start`）** と **Identity Center のリージョン**をメモ（§S-3-3 の `aws configure sso` で「SSO start URL」「SSO region」として入力する）。

**ルートB（既定・2026-06-23 実証済み）: IAM ユーザー＋長期アクセスキー**

1. **ユーザー作成**: 検索から **IAM** を開く → **［ユーザー］→［ユーザーを作成］** → ユーザー名 `go-iot-mcp` → **「AWS マネジメントコンソールへのアクセスを提供する」はチェックしない**（プログラム利用のみ＝コンソールパスワードを作らない）。
2. **権限なしで作成 → 後でインラインポリシー付与**（新コンソールの実動線・2026-06-23 実証）: 「許可を設定」で **［ポリシーを直接アタッチ］を選び、何も選択せず**「次へ」→「ユーザーの作成」（権限ゼロで作成。「権限なし」の警告が出るが問題ない）。作成後、ユーザー詳細 →「**許可**」タブ →「**許可を追加**」→「**インラインポリシーを作成**」→「**JSON**」タブに **§S-3-2 の JSON を貼り付け** → 名前 `GoIotMcpLightsail` で作成。（※ウィザード内の「ポリシーの作成」は*顧客管理ポリシー*用でインラインは作れないため、この 2 段階で行う）
3. **アクセスキー発行**: 作成したユーザー → **［セキュリティ認証情報］→［アクセスキーを作成］** → ユースケースは **「コマンドラインインターフェイス (CLI)」** → 確認にチェック → 作成。
   - **アクセスキー ID とシークレットアクセスキーはこの画面でしか表示されない**。安全な場所（パスワードマネージャ）に控える。`.csv` をダウンロードしてもよいが**どこにもコミット/チャット投稿しない**。
   - 長期キーは失効しないので**定期ローテ**する。可能なら ルートA（SSO）へ移行する。
4. **プロファイル設定（人のターミナル・キーはチャットに貼らない）**: `aws configure --profile go-iot-mcp` を実行し、対話で **アクセスキー ID／シークレット／region=`ap-northeast-1`／output=`json`** を入力する（キーはターミナルにだけ入れ、AI に渡さない）。→ §S-3-3 で疎通確認（`get-caller-identity` が `user/go-iot-mcp`・Lightsail 読取成功・EC2 が AccessDenied）。

> 破壊系（`DeleteInstance`/`ReleaseStaticIp`/`DeleteDomain` 等）は §S-3-2 で**そもそも付与しない**（一次防御）。必要時だけ一時的に付け、平時は外す。

> ✅ **S-3-1 完了判定**: ルートA は「アクセスポータル URL＋リージョン＋権限セット名」が手元にある。ルートB は「アクセスキー ID＋シークレット」を安全に保管済み。いずれも `AdministratorAccess` ではなく §S-3-2 の最小権限。→ §S-3-3 でプロファイル `go-iot-mcp` を設定する。

#### S-3-2. 🤖 AI が生成：最小権限 IAM ポリシー（Lightsail 限定）

AI がこの JSON を生成し、**人が §S-3-1 の手順でコンソールに貼り付ける**。CLI/API のみの利用なら `lightsail:*` フルアクセスは不要（フルアクセスはコンソール用）。**Get 系・Create 系は `Resource:"*"` 必須**、変更系（ポート変更・再起動等）は **特定インスタンス ARN に限定**できる（要確認・出典: AWS Service Authorization Reference for Lightsail。下記ポリシーは公式の 2 例から組み立てた**草案**であり、各アクションの resource-level 対応・`CreateDomainEntry`/`CreateKeyPair` が本当に `Resource:"*"` 必須かは**接続前に要検証**）。`<ACCOUNT_ID>` と `<INSTANCE_GUID>` は、インスタンス作成後に `GetInstance` の結果から取得して埋める（初回はインスタンス未作成のため第 3 ステートメントは後から付ける運用でよい）。

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "LightsailReadOnly",
      "Effect": "Allow",
      "Action": [
        "lightsail:GetInstances", "lightsail:GetInstance",
        "lightsail:GetInstanceState", "lightsail:GetInstancePortStates",
        "lightsail:GetInstanceAccessDetails", "lightsail:GetInstanceMetricData",
        "lightsail:GetStaticIps", "lightsail:GetStaticIp",
        "lightsail:GetKeyPairs", "lightsail:GetKeyPair",
        "lightsail:GetBundles", "lightsail:GetBlueprints",
        "lightsail:GetRegions", "lightsail:GetDomains", "lightsail:GetDomain",
        "lightsail:GetOperations", "lightsail:GetOperation"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightsailProvisionRequiresStar",
      "Effect": "Allow",
      "Action": [
        "lightsail:CreateInstances",
        "lightsail:AllocateStaticIp",
        "lightsail:AttachStaticIp",
        "lightsail:CreateDomain",
        "lightsail:CreateDomainEntry",
        "lightsail:CreateKeyPair", "lightsail:ImportKeyPair"
      ],
      "Resource": "*"
    },
    {
      "Sid": "LightsailManageThisInstance",
      "Effect": "Allow",
      "Action": [
        "lightsail:PutInstancePublicPorts",
        "lightsail:OpenInstancePublicPorts",
        "lightsail:CloseInstancePublicPorts",
        "lightsail:RebootInstance", "lightsail:StartInstance", "lightsail:StopInstance",
        "lightsail:CreateInstanceSnapshot"
      ],
      "Resource": "arn:aws:lightsail:ap-northeast-1:<ACCOUNT_ID>:Instance/<INSTANCE_GUID>"
    }
  ]
}
```

> **付与しないもの（多層防御の一次層）**: `DeleteInstance` / `ReleaseStaticIp` / `DeleteDomain` などの破壊系は **IAM でそもそも付与しない**。どうしても必要なときだけ一時的に付け、平時は外す。付与する場合も S-5 の MCP `denyList`/`elicitList` で二重ゲートにする。
> **DNS の resource-level（注意）**: `CreateDomain` / `CreateDomainEntry` は上記では `Resource:"*"` に置いているが、Lightsail の DNS/ドメイン系は **`us-east-1` のみ動作**（S-3-3 参照）。ポリシーに region 条件を付けると DNS 操作だけ別扱いになりうるため、当面 region 条件は付けず resource `*` のままにし、実運用で必要なら絞る。
> **補強（公式推奨・要確認）**: タグ条件（`aws:RequestTag/allow=true` で作成許可、`aws:ResourceTag/allow=true` で削除許可）、SSL 強制条件、MFA 条件、IAM Access Analyzer での検証。

#### S-3-3. プロファイル設定と疎通確認（ルートB＝`aws configure`・2026-06-23 実証）

**既定＝ルートB**: §S-3-1 ルートBで発行したアクセスキーを、**人が自分のターミナルで `aws configure --profile go-iot-mcp`** に対話入力する（キーはチャットに貼らない・どこにもコミットしない）。これだけでプロファイルが完成し、**SSO ログインは不要**。

```bash
# ルートB（既定・実証済み）: 端末で対話入力（キーはここでだけ入れる）
aws configure --profile go-iot-mcp
#   AWS Access Key ID     : （§S-3-1 ルートBで控えたキーID）
#   AWS Secret Access Key : （同シークレット）
#   Default region name   : ap-northeast-1
#   Default output format : json
# → ~/.aws/credentials [go-iot-mcp] と ~/.aws/config [profile go-iot-mcp] が作られる
```

（任意・ルートA=SSO を選んだ場合のみ）`aws configure sso` で SSO start URL／region／権限セット／プロファイル名 `go-iot-mcp` を対話設定し、作業前に毎回 `aws sso login --profile go-iot-mcp`（トークン失効で `call_aws` が認証エラー）。**本案件では未使用**。

> ⚠️ **Lightsail の DNS/ドメイン系 API は `us-east-1` でのみ動作**する（要確認・出典: AWS CLI create-domain-entry リファレンス）。インスタンスは `ap-northeast-1`、DNS は `us-east-1` とリージョンが分かれるため、DNS 操作の `call_aws` には毎回 `--region us-east-1` を明示する設計にする（A 章の DNS ステップで AI に指示）。プロファイルの既定 region は `ap-northeast-1` にしておく。

> 🙋 **人が確認する点**: 発行した認証情報が **Lightsail 以外を触れない**こと（最小権限）。CloudTrail で追跡できるよう MCP 用 IAM を専用化したこと。長期キーをどこにもコミットしていないこと。
>
> ✅ **完了判定**: `aws sts get-caller-identity --profile go-iot-mcp` が当該 IAM プリンシパルを返し、`aws lightsail get-regions --profile go-iot-mcp --region ap-northeast-1` が成功する一方、`aws ec2 describe-instances --profile go-iot-mcp --region ap-northeast-1` が **AccessDenied** で弾かれる（= Lightsail 限定が効いている）。

---

### S-4. 既定ルート（実証済み）: self-host AWS API MCP Server を接続（claude mcp add / .mcp.json）

> **本書の既定はこの self-host 版（2026-06-23 実証済み・付録E/§S-6-2）。** SSO を使わず IAM アクセスキー（ルートB）を自前管理する。S-1 のマネージド版（SSO）は本案件未検証なので使わない。IAM 最小権限（S-3）・安全策（S-5）・read-only 疎通（S-6）は共通。**コピペで再現するなら §S-6-2 実証済みプレイブックを上から実行する**。

接続は CLI（`claude mcp add`）でも設定ファイル（`.mcp.json` / `~/.claude.json`）でもよい。**プロジェクト共有なら `.mcp.json`、自分専用なら `~/.claude.json`** を使う。

> 認証情報は MCP 設定の `env` に長期キーをベタ書きせず、**`AWS_API_MCP_PROFILE_NAME` でプロファイル参照**するのが安全（S-3 で作った `go-iot-mcp`）。`AWS_REGION` の既定は `us-east-1` なので、**東京は `ap-northeast-1` を明示**する。

#### S-4-1. AWS API MCP Server を接続する

**CLI で追加**（`--` 以降がサーバ起動コマンド。`-s project` でプロジェクトスコープ、`--env KEY=VALUE` で環境変数。`claude mcp add` の引数構文は Claude Code 一般仕様に基づく・**最新は要確認**）:

```bash
claude mcp add aws-api -s project \
  --env AWS_REGION=ap-northeast-1 \
  --env AWS_API_MCP_PROFILE_NAME=go-iot-mcp \
  --env READ_OPERATIONS_ONLY=true \
  --env REQUIRE_MUTATION_CONSENT=true \
  -- uvx awslabs.aws-api-mcp-server@latest
```

**または `.mcp.json` に記述**（最初は安全側＝`READ_OPERATIONS_ONLY=true` で起動し、構築フェーズに入る時だけ false にする。`autoApprove: []` で都度承認を強制する。`autoApprove` / `disabled` はクライアント側拡張キーで Claude Code が解釈するか要確認・未対応なら無害に無視される）:

```json
{
  "mcpServers": {
    "awslabs.aws-api-mcp-server": {
      "command": "uvx",
      "args": ["awslabs.aws-api-mcp-server@latest"],
      "env": {
        "AWS_REGION": "ap-northeast-1",
        "AWS_API_MCP_PROFILE_NAME": "go-iot-mcp",
        "READ_OPERATIONS_ONLY": "true",
        "REQUIRE_MUTATION_CONSENT": "true",
        "AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS": "no-access",
        "FASTMCP_LOG_LEVEL": "INFO"
      },
      "disabled": false,
      "autoApprove": []
    }
  }
}
```

> ⚠️ **【実機で踏んだ罠・2026-06-23 / macOS】上の `"command": "uvx"` 形は macOS では起動失敗する**。`uvx` が生成する relocatable ランチャースクリプトが `realpath` で同梱 python を解決する作りだが、**macOS には `realpath` が標準で無い**ため解決に失敗し、相対パスの `python`（＝`$PWD/python`）を実行しようとして落ちる（`realpath: command not found` → `.../python: No such file or directory`）。加えて **Cursor/GUI 起動時は最小 PATH で子プロセスを起こすため `command: "uvx"` 自体が PATH 未解決になりうる**（`~/.local/bin` が無い）。`aws-knowledge`（HTTP・別プロセス不要）は繋がるのに `aws-api`（stdio）だけ繋がらない症状が出る。
>
> **回避策（検証済み・採用）**: ラッパーを通さず `uv run` 経由で `python -c` から `main()` を直接呼ぶ。`uv` も**絶対パス**で指定して最小 PATH 問題を同時に回避する。`tools/list` で `call_aws` / `suggest_aws_commands` の公開を確認済み。
>
> ```json
>     "aws-api": {
>       "type": "stdio",
>       "command": "/Users/c/.local/bin/uv",
>       "args": [
>         "run", "--no-project", "--with", "awslabs.aws-api-mcp-server",
>         "python", "-c",
>         "import sys; sys.argv=['awslabs.aws-api-mcp-server']; from awslabs.aws_api_mcp_server.server import main; main()"
>       ],
>       "env": { "AWS_REGION":"ap-northeast-1", "AWS_API_MCP_PROFILE_NAME":"go-iot-mcp", "READ_OPERATIONS_ONLY":"true", "REQUIRE_MUTATION_CONSENT":"true", "AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS":"no-access", "FASTMCP_LOG_LEVEL":"INFO" }
>     }
> ```
>
> 切り分け手順: ① `which uvx` の絶対パス確認 → ② 壊れたランチャーは `head -2 <entrypoint>` で `realpath` 依存を確認 → ③ `printf '<initialize JSON>' | perl -e 'alarm shift; exec @ARGV' 25 <command> <args...>` でサーバが MCP `initialize` に応答するか実起動テスト（perl の `alarm` で上限を付ける。macOS に `timeout` は無い）。
>
> ℹ️ **クライアント差（2026-06-23 追記・重要）**: 上の uvx 失敗は **Cursor（最小 PATH＋`realpath` 不在）固有**だった。**Claude Code（VSCode 拡張/CLI）では `claude mcp add aws-api -s project --env … -- uvx awslabs.aws-api-mcp-server@latest`（uvx 直叩き）のまま再起動で接続成功**（付録E-1）。判断＝**まず uvx 形で繋ぎ、繋がらなければ上の `uv run … python -c …` 形にフォールバック**。事前に `uv tool install awslabs.aws-api-mcp-server` でパッケージを DL しておくと初回起動が速い。

主な環境変数（要確認・出典: awslabs AWS API MCP Server README、2026-06 時点。`@latest` 固定だと挙動が変わりうるため接続前に最新 README で変数名・既定値・取りうる値を再確認）:

| 変数 | 役割 | 本プロジェクトでの値 |
|---|---|---|
| `READ_OPERATIONS_ONLY` | 書込（Write）操作を禁止し read-only のみ許可 | 調査フェーズ `true` → 構築フェーズのみ `false` |
| `REQUIRE_MUTATION_CONSENT` | 非 read-only 操作の前に明示同意（elicitation）を要求 | `true`（書込の人承認ゲートの中核） |
| `AWS_API_MCP_PROFILE_NAME` | 使う認証プロファイル名（既定 `default`） | `go-iot-mcp` |
| `AWS_REGION` | 既定リージョン（既定 `us-east-1`） | `ap-northeast-1`（東京。DNS のみ `--region us-east-1` を都度明示） |
| `AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS` | ローカルファイルアクセス範囲（取りうる値 `no-access`／`workdir`／`true`=unrestricted・2026-06-23 実値で確認） | 調査フェーズ `no-access` → **構築フェーズで `--user-data file://` を渡す間のみ `workdir`**（`no-access` のままだと `Cannot accept file path: local file access is disabled` で拒否＝§S-6-2/付録E-3） |
| `AWS_API_MCP_WORKING_DIR` | `file://` 参照の基点ディレクトリ（相対パスの基準） | **構築フェーズのみ設定** `/Users/c/Desktop/dev/go_iot`（`workdir` と対で必須・調査フェーズでは行ごと削除） |
| `FASTMCP_LOG_LEVEL` | ログ詳細度（`ERROR`→`INFO`/`DEBUG` で実行 CLI を可視化） | `INFO`（AI が叩いた CLI を人が監査できるように） |

> ⚠️ **`REQUIRE_MUTATION_CONSENT`（elicitation）はクライアントが elicitation 対応している必要がある**。**Claude Code の elicitation 対応状況は要確認**。未対応なら同意ダイアログが出ないため、**`READ_OPERATIONS_ONLY` の付け外し運用＋IAM 最小権限＋ `denyList`/`elicitList`＋人による最終承認**で多層に代替する（S-5）。同意 UI が実際に出るかは接続後に実機検証すること。

#### S-4-2. AWS Knowledge MCP Server を接続する（認証不要・HTTP）

```bash
claude mcp add --transport http aws-knowledge -s project https://knowledge-mcp.global.api.aws
```

または `.mcp.json`:

```json
{
  "mcpServers": {
    "aws-knowledge-mcp-server": {
      "type": "http",
      "url": "https://knowledge-mcp.global.api.aws"
    }
  }
}
```

> HTTP transport 非対応クライアント向けには proxy 例 `uvx fastmcp run https://knowledge-mcp.global.api.aws`（要確認）。Knowledge MCP は認証不要だがレート制限あり。

> 🙋 **人が確認する点**: `.mcp.json` を **git にコミットしてよいのは「プロファイル名参照」型だけ**。`env` に長期キーをベタ書きした場合は **コミット禁止**（`.gitignore` に入れるか `~/.claude.json` 側に置く）。
>
> ✅ **完了判定**: Claude Code を再起動後、MCP サーバ一覧に `aws-api`（または `awslabs.aws-api-mcp-server`）と `aws-knowledge` が表示され、ツール `call_aws` / `suggest_aws_commands` / `search_documentation` 等が見える。

---

### S-5. 安全策（多層防御）

AI に AWS 書込権限を与えるため、**単一の仕組みに依存せず多層で守る**。下表の上から順に効く。

| 層 | 仕組み | 内容 |
|---|---|---|
| ① IAM（一次防御） | 最小権限ポリシー（S-3-2） | Lightsail のみ・破壊系は非付与。AccessDenied が最後の砦 |
| ② read-only 固定 | `READ_OPERATIONS_ONLY=true` | 調査フェーズは書込を物理的に不可能化。`get-bundles`/`get-instance-state` 等のみ |
| ③ 書込同意ゲート | `REQUIRE_MUTATION_CONSENT=true` | 構築フェーズで非 read-only 操作のたびに人が承認（elicitation・クライアント対応前提） |
| ④ コマンド allowlist/denylist | `~/.aws/aws-api-mcp/mcp-security-policy.json` | `denyList`=完全ブロック、`elicitList`=同意要求。完全一致のみ（ワイルドカード不可） |
| ⑤ クライアント側承認 | `.mcp.json` の `autoApprove: []` | ツール呼び出しを都度承認（Claude Code の解釈は要確認） |
| ⑥ 監査 | CloudTrail | Lightsail 全アクション（`GetInstance`/`AttachStaticIp`/`RebootInstance`/`CreateInstances` 等）を実行者・送信元 IP・時刻つきで記録 |

#### S-5-1. セキュリティポリシーファイル（denyList / elicitList）

ファイル `~/.aws/aws-api-mcp/mcp-security-policy.json`（要確認・出典: awslabs AWS API MCP Server、2026-06 時点。ファイルパス・スキーマは README で再確認）。表記は **`"aws <service> <operation>"`**（service/operation は kebab-case）。**完全一致のみでワイルドカード非対応**のため、破壊系は 1 行ずつ列挙する。

```json
{
  "version": "1.0",
  "policy": {
    "denyList": [
      "aws lightsail delete-instance",
      "aws lightsail delete-instances",
      "aws lightsail release-static-ip",
      "aws lightsail delete-domain",
      "aws lightsail delete-domain-entry",
      "aws lightsail delete-key-pair",
      "aws iam delete-user",
      "aws iam create-access-key"
    ],
    "elicitList": [
      "aws lightsail create-instances",
      "aws lightsail allocate-static-ip",
      "aws lightsail attach-static-ip",
      "aws lightsail put-instance-public-ports",
      "aws lightsail open-instance-public-ports",
      "aws lightsail reboot-instance",
      "aws lightsail stop-instance",
      "aws lightsail create-instance-snapshot",
      "aws lightsail create-domain",
      "aws lightsail create-domain-entry"
    ]
  }
}
```

> ⚠️ **denyList は CLI 名の完全一致**である。AI が `aws lightsail delete-instance` 相当を `delete-instances`（複数形）や別表現で呼ぶ余地があるため、**想定される表記ゆれ（単数/複数）を両方列挙**する（上記は delete-instance / delete-instances を併記済み）。最終防壁は ① の IAM 非付与であり、denyList 単独に依存しない。
> **billable 操作の代表（人承認必須）**: `create-instances`（インスタンス起動＝課金開始）、`allocate-static-ip`（未アタッチ放置で課金）、スナップショット作成、プランのアップグレード。これらは `elicitList` に入れて毎回確認させる。

> ⚠️ **MCP サーバは deploy / emr サービスの ssh/sock/get/put/install/uninstall をデフォルト denylist**する（要確認・出典: awslabs README）。`lightsail get-instance-access-details`（= 一時 SSH 鍵を返す操作）が **read-only 扱いか／denylist 対象か**は公式に明記がなく不明（要確認）。`READ_OPERATIONS_ONLY=true` 下でこの API が通るかも未確証なので、A 章で SSH 接続情報をこの API 経由で取りたい場合、**接続後に `call_aws` で実際に通るかを疎通確認**する必要がある。通らない場合は、人が手元で `aws lightsail get-instance-access-details` を打つか、登録鍵で SSH するフォールバックにする。

#### S-5-2. read-only ↔ 構築フェーズの切替運用

elicitation がクライアント未対応の場合の現実解として、**フェーズで設定を切替える**。**コピペ可能な完全手順（env 全文・ポリシー全文・退避/復元コマンド）は §S-6-2 実証済みプレイブックに集約**。要点:

1. **調査フェーズ**（プラン選定・現状確認）: `READ_OPERATIONS_ONLY=true` のまま。書込は物理的に不可。
2. **構築フェーズへ切替（A-1 作成等の直前・4点を“同時に”変える）**: ① `READ_OPERATIONS_ONLY=false` ② `REQUIRE_MUTATION_CONSENT=false` ③ `AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS=workdir` ④ `AWS_API_MCP_WORKING_DIR=/Users/c/Desktop/dev/go_iot` を新規追加（③④が無いと `--user-data file://` が `Cannot accept file path` で拒否）。あわせて `mcp-security-policy.json` の **`elicitList` から A-1 書込5件**（create-instances/allocate-static-ip/attach-static-ip/put-instance-public-ports/open-instance-public-ports）を一時除外（**denyList 8件は維持**）。**事前に必ず退避** `cp -n ~/.aws/aws-api-mcp/mcp-security-policy.json ~/.aws/aws-api-mcp/mcp-security-policy.json.bak-fulllist`。②と elicitList を両方外す理由＝`policy.py` の優先 **deny > elicit > default** で、`REQUIRE_MUTATION_CONSENT=true` か elicitList 該当の**どちらか一方でも** ELICIT が強制されるため。発効後の承認は **Claude Code 本体プロンプト＋READ_ONLY トグル＋IAM 最小権限＋denyList** に一本化。
3. **作業後は必ず安全構成へ復帰**: `.mcp.json` の env を①〜④の逆（`true`/`true`/`no-access`・WORKING_DIR 行削除）に戻し、ポリシーを `cp ~/.aws/aws-api-mcp/mcp-security-policy.json.bak-fulllist ~/.aws/aws-api-mcp/mcp-security-policy.json` で復元 → 発効（Claude Code 再起動 / Cursor は Cmd+Shift+P →「Developer: Reload Window」）。これで「AI が勝手に課金操作する」事故を設定の物理状態で防ぐ。

---

### S-6. 接続確認（read-only での疎通）

書込を一切させずに、AI が AWS を読み取れることだけを確認する。`READ_OPERATIONS_ONLY=true` のまま行うのが安全。

#### 操作: Lightsail インスタンス一覧の読み取り

> ▶ **AI への依頼（プロンプト例）**
> 「東京リージョン（ap-northeast-1）の Lightsail インスタンス一覧を見せて。まだ何も作っていないはずなので、空でも構わない」
>
> 🤖 **AI が行う MCP 操作（裏で呼ばれる AWS API/コマンド）**: `call_aws` ツールで以下の **read-only** コマンドを実行する（透明性のため、AI が裏で何を叩くかを明示）。
> ```bash
> aws sts get-caller-identity --profile go-iot-mcp           # 誰として繋がっているか → user/go-iot-mcp
> aws lightsail get-instances --region ap-northeast-1         # → 未作成なら空 []
> aws lightsail get-regions   --region ap-northeast-1         # → ap-northeast-1 を含む
> aws ec2 describe-instances  --region ap-northeast-1         # → AccessDenied（=最小権限が効いている。200 なら権限過剰）
> ```
> 自然言語からコマンドが定まらない場合、AI は先に `suggest_aws_commands` で候補 5 件を出してから `call_aws` を実行することがある。
>
> 🙋 **人が確認する点**: (1) AI が叩いたのが **read-only コマンドのみ**であること（`FASTMCP_LOG_LEVEL=INFO` のログ、または `.mcp.json` の `autoApprove: []` による承認プロンプトで確認）。(2) `create-*`/`delete-*` 等の書込が**呼ばれていない**こと。
>
> ✅ **完了判定**: `sts get-caller-identity` が `user/go-iot-mcp` を返し、`get-instances` が（未作成なら）空配列・または既存インスタンスの JSON を**エラーなく**返し、`get-regions` が `ap-northeast-1` を含み、`ec2 describe-instances` が **AccessDenied**（=最小権限が効いている）で弾かれる。これで「IAM 認証 → MCP → Lightsail API」の経路と最小権限が疎通確認できる。

#### 操作: ドキュメント参照（Knowledge MCP）＋実バンドルの突き合わせ

> ▶ **AI への依頼（プロンプト例）**
> 「東京リージョン（ap-northeast-1）で利用できる Lightsail のバンドル（プラン）の bundle-id・月額・メモリを、`get-bundles` の実値で出して。あわせて公式ドキュメントの最新仕様とも突き合わせて」
>
> 🤖 **AI が行う MCP 操作**: AWS API MCP の `call_aws` で `aws lightsail get-bundles --region ap-northeast-1`（read-only）を実行し**実 ID/価格/スペックを取得**。補助的に AWS Knowledge MCP の `search_documentation` / `read_documentation` で手順・仕様を裏取り（認証不要・課金なし・read-only）。
>
> 🙋 **人が確認する点**: 返ってきた料金・メモリが本書の「（要確認）」値（Nano-0.5GB / Micro-1GB 等・$5/$7 は時点要確認）と整合するか。**bundle-id は世代でサフィックスが変動**するため、A 章の作成時に必ず `get-bundles` の実 ID を使うこと。
>
> ✅ **完了判定**: `get-bundles` が実バンドル ID とスペックを返し、Knowledge MCP がドキュメント本文を返す。
>
> ✅ **【確定済み・2026-06-23 / `call_aws` 実値】** 東京の Linux バンドル（デュアルスタック＝IPv4 込み `*_3_0`）: `nano_3_0`=**$5**/0.5GB、**`micro_3_0`=$7/1GB・2vCPU・40GB SSD・2TB 転送**、`small_3_0`=$12/2GB。本書の目安値（Nano$5/Micro$7/Small$12）と**完全一致**。→ **本案件の `<BUNDLE_ID>` = `micro_3_0`（$7/月・`platforms:["LINUX_UNIX"]`・`isActive:true`）に確定**。**IPv6 専用バンドル（`micro_ipv6_3_0`=$5）は選ばない**＝パブリック IPv4 を持たず、sslip.io(IPv4 HTTPS)・IPv4 静的 IP・管理元 IPv4 `<管理元IP>/32` からの SSH が成立しないため。Lightsail 標準バンドルは x86_64＝amd64（ARM バンドルは Lightsail に無い・取り違えによる `Exec format error` の懸念なし）。A-1 作成時も `get-bundles` で実 ID を再確認する運用は維持。

> この 2 つが通れば、**S 章のゴール（接続・認証・最小権限・安全策・疎通）は達成**。以降の A 章では同じ対話フォーマット（▶依頼 / 🤖 MCP 操作 / 🙋 人の承認 / ✅ 完了判定）で、`create-instances --user-data`（cloud-init で swap/PostgreSQL/Caddy/systemd を初期構成・**秘密は焼かない**）・`allocate-static-ip`/`attach-static-ip`・`put-instance-public-ports`（22/80/443 のみ全置換・8080/5432 を渡さない）・`create-domain`/`create-domain-entry`（`--region us-east-1`）を AI に依頼していく。

---

#### S-6-2. 実証済み再現プレイブック（2026-06-23 / self-host・IAM ユーザー長期キー / コピペ可・省略なし）

> **2026-06-23 に実際に成功した手順を、次回これだけ見て再現できるよう一本化**したもの（出典＝付録E）。**接続は self-host `awslabs.aws-api-mcp-server`（§S-4）＋ IAM ユーザー長期キー（§S-3-1 ルートB）が実証済みルート**。マネージド版（§S-1）は本プロジェクトで未検証なので、再現最優先ならこのプレイブックに従う。**env は毎回フルで書く**（差分省略「…上と同じ…」は写し漏れ事故のもと）。値（profile=`go-iot-mcp` / region=`ap-northeast-1` / WORKING_DIR=リポジトリ絶対パス / ACCOUNT_ID=`474025757751`）は本案件の実値・別環境では置換する。

**0. 前提ツール**（§S-2）: `uvx --version`・`aws --version`(=2.x) が通ること。初回起動を速くするため先に DL: `uv tool install awslabs.aws-api-mcp-server`

**1. 接続（調査フェーズ＝安全構成）**。クライアントで初手が変わる:

- **Claude Code（本案件で成功）= uvx 直叩きを env 6変数フルで**:
  ```bash
  claude mcp add aws-api -s project \
    --env AWS_REGION=ap-northeast-1 \
    --env AWS_API_MCP_PROFILE_NAME=go-iot-mcp \
    --env READ_OPERATIONS_ONLY=true \
    --env REQUIRE_MUTATION_CONSENT=true \
    --env AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS=no-access \
    --env FASTMCP_LOG_LEVEL=INFO \
    -- uvx awslabs.aws-api-mcp-server@latest
  ```
  → 再起動 → `/mcp` で `aws-api` が connected。

- **Cursor / 繋がらない時 = uv 絶対パス＋uv run 形（env 6変数フル）にフォールバック**（uvx は macOS で `realpath` 不在＋GUI最小PATHで失敗＝`aws-knowledge` だけ繋がり `aws-api` が出ない症状）。`.mcp.json` の `aws-api` を差し替え:
  ```json
  "aws-api": {
    "type": "stdio",
    "command": "<UV_ABS_PATH>",
    "args": ["run","--no-project","--with","awslabs.aws-api-mcp-server",
             "python","-c","import sys; sys.argv=['awslabs.aws-api-mcp-server']; from awslabs.aws_api_mcp_server.server import main; main()"],
    "env": {
      "AWS_REGION":"ap-northeast-1", "AWS_API_MCP_PROFILE_NAME":"go-iot-mcp",
      "READ_OPERATIONS_ONLY":"true", "REQUIRE_MUTATION_CONSENT":"true",
      "AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS":"no-access", "FASTMCP_LOG_LEVEL":"INFO"
    }
  }
  ```
  `<UV_ABS_PATH>` は環境依存 → `command -v uv` で取得（例 `/Users/c/.local/bin/uv`）。

- **起動の単体テスト**（macOS に `timeout` 無し→perl alarm 代用）:
  ```bash
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"probe","version":"0"}}}\n' \
    | perl -e 'alarm shift; exec @ARGV' 25 "$(command -v uv)" run --no-project --with awslabs.aws-api-mcp-server \
        python -c "import sys; sys.argv=['awslabs.aws-api-mcp-server']; from awslabs.aws_api_mcp_server.server import main; main()"
  ```
  `initialize` 応答が返り、`tools/list`（`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`）に `call_aws`/`suggest_aws_commands` が出れば OK。

- **発効**: Claude Code は CLI/拡張を再起動、Cursor は Cmd+Shift+P →「Developer: Reload Window」。プロジェクト MCP は初回に承認。

**2. 接続確認＋read-only 疎通**（`READ_OPERATIONS_ONLY=true` のまま）:
```bash
aws sts get-caller-identity --profile go-iot-mcp                          # → arn:aws:iam::474025757751:user/go-iot-mcp（別人なら IAM 取り違え）
aws lightsail get-regions   --profile go-iot-mcp --region ap-northeast-1  # → ap-northeast-1 を含む
aws lightsail get-instances --profile go-iot-mcp --region ap-northeast-1  # → 未作成なら空 []
aws ec2 describe-instances  --profile go-iot-mcp --region ap-northeast-1  # → AccessDenied（=最小権限が効いている。200 なら権限過剰＝IAM 見直し）
```
`/mcp` で `aws-api`/`aws-knowledge` が connected、`call_aws` が見えることも確認。

**3. セキュリティポリシー配置＋退避**:
```bash
mkdir -p ~/.aws/aws-api-mcp
# ~/.aws/aws-api-mcp/mcp-security-policy.json に §S-5-1 の全文（denyList 8件 / elicitList 10件）を保存
cp -n ~/.aws/aws-api-mcp/mcp-security-policy.json ~/.aws/aws-api-mcp/mcp-security-policy.json.bak-fulllist   # 原本退避（-n=上書き禁止）
```

**4. 書込フェーズへ切替（A章 create-instances 等の直前・4点を“同時に”）**。これを一度にやらないと、`--user-data file://` が `Cannot accept file path` で拒否＋`create-instances` が elicitation 却下 `User rejected the execution of the command` で弾かれ、何度も書き直す羽目になる:

- `.mcp.json` の `aws-api.env` を**構築フェーズ完全形（7変数・WORKING_DIR を新規追加）**に差し替え:
  ```json
  "env": {
    "AWS_REGION": "ap-northeast-1",
    "AWS_API_MCP_PROFILE_NAME": "go-iot-mcp",
    "READ_OPERATIONS_ONLY": "false",
    "REQUIRE_MUTATION_CONSENT": "false",
    "AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS": "workdir",
    "AWS_API_MCP_WORKING_DIR": "/Users/c/Desktop/dev/go_iot",
    "FASTMCP_LOG_LEVEL": "INFO"
  }
  ```
  （差分 4点: READ_OPERATIONS_ONLY `true→false` / REQUIRE_MUTATION_CONSENT `true→false` / FILE_ACCESS `no-access→workdir` / **WORKING_DIR 行を新規追加**。`file://deploy/cloud-init.sh` は WORKING_DIR 基点の相対で通る）

- `mcp-security-policy.json` の `elicitList` から **A-1 書込5件**（`create-instances`/`allocate-static-ip`/`attach-static-ip`/`put-instance-public-ports`/`open-instance-public-ports`）を一時除外（**denyList 8件は維持**）。除外後の全文:
  ```json
  { "version": "1.0", "policy": {
    "denyList": [
      "aws lightsail delete-instance", "aws lightsail delete-instances", "aws lightsail release-static-ip",
      "aws lightsail delete-domain", "aws lightsail delete-domain-entry", "aws lightsail delete-key-pair",
      "aws iam delete-user", "aws iam create-access-key"
    ],
    "elicitList": [
      "aws lightsail reboot-instance", "aws lightsail stop-instance", "aws lightsail create-instance-snapshot",
      "aws lightsail create-domain", "aws lightsail create-domain-entry"
    ]
  } }
  ```

- **発効**（再起動 / Reload Window）。承認は **Claude Code 本体の許可プロンプト＋READ_ONLY トグル＋IAM 最小権限＋denyList** に一本化。
- **なぜ CONSENT と elicitList を両方外すか**: `policy.py` の優先は **deny > elicit > default**。`REQUIRE_MUTATION_CONSENT=true` か elicitList 該当の**どちらか一方でも** ELICIT が強制されるため、elicitation 非対応クライアントでは両方外す。
- **罠**: `create-instances --tags ...` は `lightsail:TagResource` が最小権限（§S-3-2）に無く `AccessDeniedException`（atomic 拒否＝オーファン無し）。**`--tags` を付けない**。

**5. 作業後の安全構成への復帰（必須）**:
- `.mcp.json` の `aws-api.env` を **1. の安全構成（6変数）**へ戻す（READ_OPERATIONS_ONLY=`true` / REQUIRE_MUTATION_CONSENT=`true` / FILE_ACCESS=`no-access`、**AWS_API_MCP_WORKING_DIR 行は削除**）。
- ポリシーを原本へ復元:
  ```bash
  cp ~/.aws/aws-api-mcp/mcp-security-policy.json.bak-fulllist ~/.aws/aws-api-mcp/mcp-security-policy.json
  ```
- 発効（再起動 / Reload Window）→ `aws lightsail get-instances ...` は通り、書込が拒否されることを確認。

---

### S-7. デプロイ反復（インスタンス内操作）の経路: 既定 SSH / オプション SSM

AWS API MCP の `call_aws` は **AWS API のみ**で SSH/scp を扱わない。よってバイナリ配布・systemd 再起動・ログ確認・バックアップ・**秘密（EnvironmentFile）配置**・トークン発行（`cmd/gen-token`）などの**インスタンス内反復**は、次のいずれかで行う（A 章で詳述）。

- **既定 = SSH**（推奨）: 低メモリ Lightsail に余計な常駐を増やさない方針と整合。AI は `call_aws` で `aws lightsail get-instance-access-details`（一時 SSH 鍵・username・publicIpAddress・有効期限を返す。接続直前に取得する設計で鍵を端末に常置しない）を取得できる場合があるが、**この API が read-only 扱いか／MCP の denylist 対象かは要確認**（S-5-1）。**実際の SSH/scp 実行自体は AWS MCP の範囲外**のため、人が手元端末から行うか、AI に shell 実行させたい場合は別途 shell 系 MCP/CLI を用いる（その場合は別途の最小権限・監査設計が必要）。
- **オプション = SSM Run Command**: Lightsail は SSM ネイティブ非対応だが、**ハイブリッドアクティベーション**で managed node 化すれば `aws ssm send-command`（`AWS-RunShellScript`）が `call_aws` から単発実行でき、CloudTrail 監査にも乗る。手順は (a) IAM サービスロール（既定 `service-role/AmazonEC2RunCommandRoleForManagedInstances` 等。無いと `CreateActivation` が `ValidationException`）→ (b) `aws ssm create-activation`（ActivationCode/ActivationId を取得・既定 24h/最大 30 日で失効）→ (c) インスタンス上で SSM Agent を導入のうえ `sudo amazon-ssm-agent -register -code <code> -id <id> -region ap-northeast-1`（**SSM Agent の具体インストール手順は要確認**）。**ただし SSM Agent の常駐メモリ（数十 MB 級）が低メモリ機には負担**で、対話的 SSM セッションのストリームは MCP の単発ツール実行と相性が悪い。→ **デプロイ反復は SSH を既定とし、SSM は運用簡素化の将来オプション**とする（要確認・出典: AWS re:Post「Add a Lightsail instance to Systems Manager」。本文 403 で一次取得できず二次情報ベース・接続後に実機再確認）。

> ⚠️ **2回目以降のリデプロイ（コード更新の反映）と SSH の IP 問題は §A-3-R に集約**: VPN で接続元IP(egress)が接続ごとに変動する／管理元IPも動的なため、SSH(22) は**毎回その場で egress を検出して一時開放 → 作業 → 復元**する（`bash deploy/redeploy.sh` で全自動）。`443` が通り `22` だけ `timed out` なら**サーバ障害ではなく FW のIP不一致**。**FW変更は `call_aws`(MCP) だと elicit で必ず却下されるためローカルCLI（`--profile go-iot-mcp`）で行う**（読み取りは call_aws 可）。手動手順・3大トラップは §A-3-R を参照。

> **初期構成は `create-instances --user-data` に渡しきる（秘密は除く）**: cloud-init は **初回ブートのみ root 実行**（追加・再実行不可・ログ `/var/log/cloud-init-output.log`）。swap/PostgreSQL(native apt)/Caddy/systemd unit の**雛形配置**を、A 章で AI が `create-instances` 時の `--user-data` 一括スクリプトとして渡す。**`DATABASE_URL`/`SESSION_SECRET`/Bearer トークン等の秘密は userData に書かず、初回 SSH 後に EnvironmentFile（権限 600）として配置する。** 例（透明性のため AI が渡す中身を明示。`sudo` 不要＝root 実行）:
>
> ```bash
> #!/bin/bash
> # cloud-init で初回ブート時に root 実行される（追加・再実行不可）。秘密は焼かない。
> set -euxo pipefail
> apt-get update -y
> # --- swap（低メモリ機の OOM 対策・必須）---
> fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
> chmod 600 /swapfile; mkswap /swapfile; swapon /swapfile
> grep -q '/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
> echo 'vm.swappiness=10' > /etc/sysctl.d/99-swappiness.conf; sysctl -p /etc/sysctl.d/99-swappiness.conf
> # --- PostgreSQL 16（native apt・Docker 不使用）/ Caddy / systemd unit の雛形配置 ... ---
> #   listen_addresses='localhost'、5432 非公開を維持。DB パスワード・SESSION_SECRET は
> #   ここに書かず、初回 SSH 後に EnvironmentFile(600) で設定する（詳細は A 章）。
> ```
> ✅ 検証: 作成後に AI が `get-instance-state` で `running` を確認し、人が（または SSH で）`/var/log/cloud-init-output.log` を見て初期構成の成否を確認する（re:Post に「UserData not working」既知質問あり＝記法/権限が典型原因。ログ確認ゲートを必ず置く）。

---

### S-9. このセクションの完了確認（A 章の前提）

- [ ] Claude Code に `aws-api`（AWS API MCP）と `aws-knowledge`（Knowledge MCP）が接続済み（`call_aws` / `search_documentation` が見える）。
- [ ] 専用プロファイル `go-iot-mcp` が **Lightsail 限定の最小権限**で、`aws ec2 ...` が AccessDenied、`aws lightsail get-regions` が成功。
- [ ] `READ_OPERATIONS_ONLY=true` で起動し、書込は不可。構築時のみ false に切り替え、終わったら true に戻す運用を理解している。
- [ ] `mcp-security-policy.json` の `denyList`（破壊系・単数/複数の表記ゆれ列挙）/`elicitList`（billable・要確認系）を設定済み。`.mcp.json` は `autoApprove: []`。
- [ ] CloudTrail で MCP 用 IAM の操作が追跡できる（証跡作成＋S3 配信を推奨）。認証情報を git にコミットしていない。userData に秘密を焼いていない。
- [ ] read-only で「Lightsail インスタンス一覧」「Lightsail バンドル（get-bundles 実値）」を AI 経由で取得でき、疎通確認できた。
- [ ] インスタンス内反復は **SSH を既定**、SSM は将来オプション、初期構成は **`create-instances --user-data`（cloud-init・秘密は除く）に渡しきる**方針を理解している。`get-instance-access-details` が read-only/denylist でどう扱われるかは接続後に実機確認する。

> **誠実な明記（人手が不可避に残る作業）**: AWS アカウント作成・課金/クレカ登録・ルート/MFA 設定・ドメイン購入・**MCP 用 最小権限 IAM 認証情報の初回発行**・ESP8266 の物理書込（B 章）・**billable/破壊的操作の最終承認**（elicitList／人ゲート）・**秘密値（DB パスワード/SESSION_SECRET/Bearer トークン）のサーバ上での生成と EnvironmentFile 配置**は、AI に委ねられない人手作業である。本書は「人がコマンドを直接打たない」を主眼としつつ、これらの境界を隠さない。

---

## A-1. プロビジョニング（インスタンス／静的IP／ファイアウォール／DNS）をAIに依頼

このセクションは、東京リージョンに低メモリ Lightsail を 1 台作り、静的 IP を割り当て、ファイアウォールで `22/80/443` のみ開け、ドメインの A レコードを静的 IP に向けるまでを、**「人がコマンドを直接打たず、AI と対話して AI が MCP で実行する」**形で扱う。狙いは今後のサーバ運用を楽にすること（持続可能な運用）。

維持する不変条件 — 低メモリプラン前提・**サーバでビルドしない（ローカル Mac で `CGO_ENABLED=0` クロスコンパイルしたバイナリのみ配布）**・**8080（アプリ）と 5432（PostgreSQL）は外部公開禁止**・HTTPS は前段 **Caddy 自動 TLS**・環境変数は **systemd の EnvironmentFile/Environment**（`.env` 自動読込なし・秘密ファイルは権限 600）で供給。AI が AWS リソース層を操作するだけで、この境界線は一切緩めない。

> ⚠️ **§S 完了（AWS MCP 接続・最小権限 IAM 発行）が前提**（未接続なら先に §S）。人手が残る作業の全体像は付録D-2。

> ⚠️ **要確認の外部事実**: MCP のパッケージ名・ツール名・環境変数・AWS API 名・料金・バンドル/ブループリント ID は変動する。本文の値は**調査時点（2026-06）の目安**であり、該当箇所に「（要確認・<出典/時点>）」を付す。AI は契約・実行直前に AWS Knowledge MCP（後述）や `aws lightsail get-bundles` で実値を取り直すこと。ハルシネーション厳禁。
>
> 特に次は**契約/接続直前に最新の STABLE 版ドキュメントで再確認**すること（@latest 固定だと挙動が変わりうる）:
> - awslabs は「**Agent Toolkit for AWS**（マネージド版 AWS MCP Server・GA）」への統合を推奨しており、**本書の既定は self-host 版**（マネージド版は本案件未検証で不採用・§S-4）。本文の具体コマンド例は self-host の語彙（`call_aws` / `awslabs.aws-api-mcp-server` の env 名）で書いてある——AI が実行する `aws lightsail ...` 自体は両ルート共通なので、マネージド版では**接続経路（`uvx mcp-proxy-for-aws@latest https://aws-mcp.us-east-1.api.aws/mcp`）と env 名だけ読み替える**（要確認・出典: 調査[mcp]/[inst]・docs.aws.amazon.com/agent-toolkit）。
> - `claude mcp add` の引数構文（`-s` / `--env` / `--transport` / `--`）は Claude Code の一般仕様に基づく（awslabs 公式 installation には Claude Code 専用記載が無い＝Claude Code 側ドキュメントの最新確認を推奨。出典: 調査[mcp] caveats）。
> - **Claude Code の elicitation 対応状況は要確認**。`REQUIRE_MUTATION_CONSENT`（後述）はクライアントの elicitation 対応が前提のため、未対応なら代替運用（後述）に倒す。

---

### A-1-0. このセクションで AI に任せること／人手が残ること

| 区分 | 担当 | 内容 |
|---|---|---|
| AWS リソース層 | **AI（AWS API MCP の `call_aws`）** | インスタンス作成（cloud-init 同梱）／静的 IP 確保・付与／公開ポート設定（22/80/443 のみ）／DNS A レコード作成／状態・FW・dig による完了判定 |
| インスタンス内初期構成 | **AI（cloud-init/userData）** | swap・PostgreSQL・Caddy・systemd 雛形を CreateInstances の `--user-data` に同梱して初回ブートで自動構成（詳細は A-2 で起草。本セクションは「userData を渡す」設計と検証ゲートまで） |
| 不可避な人手 | **人** | AWS アカウント作成・課金/クレカ登録・ルート/MFA・**ドメイン購入**・**MCP 用最小権限 IAM 認証情報の初回発行**・**billable/破壊的操作の最終承認**（後述の承認ゲート）・cloud-init 成否や `dig` の最終目視 |

---

### A-1-A. 前提（S 章で接続済み）と A-1 固有の人手作業

**§S 完了（MCP 接続・最小権限 IAM 発行・安全策の整備）が前提**（未了なら先に §S）。一般の人手作業（アカウント/課金/MFA・IAM 初回発行）は A-1-0 の表／付録D-2 のとおり。**既定（sslip.io）ではドメイン購入は不要**＝A-1 固有の事前人手は無い（独自ドメインを使う場合のみ購入・課金・NS 設定が要る → 操作4 の任意ルート）。

最小権限 IAM ポリシー全文＝§S-3-2、MCP 登録（**既定=self-host §S-4・実証済み**／マネージド版は未検証）、承認ゲートと `mcp-security-policy.json`＝§S-5（再掲しない）。A-1 で効く要点だけ確認する:

- **A-1 が使う書込操作**: `create-instances`（課金開始）/`allocate-static-ip`（未アタッチ放置で課金）/`attach-static-ip`/`put-instance-public-ports`（公開ポート全置換）/`create-domain`/`create-domain-entry`（`--region us-east-1`）。いずれも S-3-2 の作成系ポリシーで実行でき、S-5-1 の `elicitList` に登録済みである前提。
- **承認の回し方（書込フェーズ切替）**: A-1 の構築操作の直前に **§S-6-2 手順4 のとおり 4点を同時に切替**（`READ_OPERATIONS_ONLY=false` / `REQUIRE_MUTATION_CONSENT=false` / `AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS=workdir` / `AWS_API_MCP_WORKING_DIR=/Users/c/Desktop/dev/go_iot` 追加）＋ `mcp-security-policy.json` の `elicitList` から A-1 書込5件を一時除外（denyList8件維持・事前に `.bak-fulllist` へ退避）→ 再起動/Reload Window で発効。承認は Claude Code 本体プロンプト＋READ_ONLY トグル＋IAM 最小権限＋denyList に一本化（elicitation はクライアント非対応＝付録E-3）。**A-1 完了後は §S-6-2 手順5 で安全構成へ戻す**。破壊系は IAM 非付与（一次防御）。CloudTrail に `go-iot-mcp` プリンシパルで記録。
- **`get-instance-access-details`（SSH 鍵返却）は✅実証済みで使える**（付録E-5）。`download-default-key-pair` は最小権限に無く AccessDenied なので使わない。返る一時アクセスは **SSH 証明書ベースで `privateKey` に加え `certKey`（SSH 証明書）が必須**（無いと `Permission denied (publickey)`）。`~/.ssh/lightsail-goiot.pem`(600)＋`~/.ssh/lightsail-goiot.pem-cert.pub`(600・`<秘密鍵名>-cert.pub` 命名で ssh 自動探索)に置き、`ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@<STATIC_IP>`（ユーザー=`ubuntu`）。**証明書は実測 約13分で失効**＝取得直後に scp/ssh をまとめて実行。鍵は `call_aws` ではなく端末で直接 `aws` 実行しファイルへ落として抽出（チャットに鍵を残さない）。

---

### A-1-B. プロビジョニング運用（接続後・対話で実施）

ここからは AWS MCP 接続済み・IAM 発行済みを前提に、各操作を **対話フォーマット**で示す。AI が裏で呼ぶ具体コマンドはすべて明示する（透明性＝人が監査できるように）。AI は `call_aws` で実行する前に `suggest_aws_commands` で最新 CLI 候補を確認してよい。

#### 不変条件の事前確認（CPU 種別と GOARCH 整合・最重要）

**プラン／料金は「要確認」**（Lightsail のバンドル名・月額・スペックは変動する）。**CPU は本案件 amd64（x86）で確定**だが、**選ぶバンドルが amd64 であること**は必ず確認する（ARM バンドルと混同すると `GOARCH` 不一致で `Exec format error`）。本アプリは純 Go＝`CGO_ENABLED=0` で追加ツールなしでビルド可。インスタンスを作る前に、選ぶバンドルが amd64 であることを AI に確認させる（ローカルのビルドも `GOARCH=amd64`）。

> ▶ **AI への依頼（プロンプト例）**
> 「東京リージョン（ap-northeast-1）の Lightsail バンドルを調べて。低メモリ前提で Nano(0.5GB)/Micro(1GB)/Small(2GB) の月額・vCPU・メモリ・SSD・転送量と、各バンドルが x86(amd64) か ARM(arm64) かを表にして。Micro-1GB が Go+PostgreSQL+Caddy 同居（swap 併用）に妥当か、Knowledge MCP で最新の料金/仕様も裏取りして。まだ何も作らないで、調査だけ。」

> 🤖 **AI が行う MCP 操作**（read-only・課金なし。`READ_OPERATIONS_ONLY=true` のまま実行可）
> ```bash
> aws lightsail get-bundles --region ap-northeast-1 \
>   --query "bundles[].[bundleId,name,ramSizeInGb,cpuCount,diskSizeInGb,transferPerMonthInGb,price]" \
>   --output table
> aws lightsail get-blueprints --region ap-northeast-1 \
>   --query "blueprints[?type=='os'].[blueprintId,name,platform]" --output table
> aws lightsail get-regions --query "regions[?name=='ap-northeast-1']" --output table
> ```
> 併せて AWS Knowledge MCP（`search_documentation` / `get_regional_availability`）で東京リージョンの最新バンドル/料金/可用性を裏取り。

> 🙋 **人が確認/承認する点**
> - 表示された月額・スペックが**東京リージョンの実額**か（バンドル価格はリージョン共通だが転送量は地域差・超過は従量。要確認・本レビューでは実額の裏取り不可）。目安は Nano $5 / Micro $7 / Small $12（いずれも時点要確認）。
> - **選ぶバンドルが amd64（x86）であること**を確認（本案件確定）。ローカルのビルドは `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`（A-3 で実施）。
> - **公開 IPv4 付きバンドル（dual-stack）を選ぶ**こと。IPv6-only バンドルは 2〜3 割安いが、ESP8266（多拠点・動的 IP）・Let's Encrypt・運用端末の到達性で詰まりやすい。これは後段 `--ip-address-type` とは別に**バンドル選択時の判断**。
> - 新規/対象アカウントの 3 か月無料枠の適用可否（要確認・コンソール表示。1 アカウント 1 バンドル・月 750 時間まで等の条件あり）。

> ✅ **完了判定**
> - **✅ 本案件の確定値（付録E-0/§S-6-1）**: bundleId=**`micro_3_0`**（$7/月・1GB・2vCPU・40GB・amd64・dualstack）、blueprintId=**`ubuntu_24_04`**。⚠️ **IPv6専用 `micro_ipv6_3_0`（$5）は選ばない**（IPv4 が無く sslip.io HTTPS・IPv4 静的IP・管理元IP からの SSH が不成立）。Lightsail に ARM バンドルは無いので `GOARCH=amd64` 確定（`Exec format error` の懸念なし）。`get-bundles`/`get-blueprints` は念のため実 ID 再確認に使う。**この時点では課金は発生していない**。

---

#### 操作 1: インスタンス作成（cloud-init を userData に同梱）

インスタンス作成＝**課金開始**のため、最初の承認ゲート対象（出典: 調査[sec] §7）。swap/PostgreSQL/Caddy/systemd の初期構成は **`--user-data` の launch script に集約**して初回ブートで自動構成する（Lightsail は cloud-init で root 実行・**初回ブートのみ・後から再実行不可**。ログは `/var/log/cloud-init-output.log`。出典: 調査[inst]/[api]）。userData の中身（swap 作成・PGDG 導入・Caddy・systemd 雛形）は A-2 で起草するため、ここでは**渡す設計と検証ゲート**を確定する。

> ▶ **AI への依頼（プロンプト例）**
> 「東京リージョンに go-iot-prod を Ubuntu 24.04 LTS / 確定したバンドルで 1 台作って。起動スクリプト（A-2 で用意した userData の launch script ファイル）を `--user-data file://...` で同梱して。AZ は ap-northeast-1a。`--ip-address-type` は dualstack（公開 IPv4 付き）。作成は課金が始まるので、実行前に必ず私の承認を取って。」

> 🤖 **AI が行う MCP 操作**（`REQUIRE_MUTATION_CONSENT` で実行前に同意要求。未対応時は構築直前だけ `READ_OPERATIONS_ONLY=false` に切替）
> ```bash
> aws lightsail create-instances \
>   --instance-names go-iot-prod \
>   --availability-zone ap-northeast-1a \
>   --blueprint-id ubuntu_24_04 \
>   --bundle-id micro_3_0 \
>   --ip-address-type dualstack \
>   --user-data file://deploy/cloud-init.sh \
>   --region ap-northeast-1
>   # ⚠️ --tags は付けない: lightsail:TagResource が最小権限(§S-3-2)に無く AccessDeniedException(atomic拒否=オーファン無し・2026-06-23実証/付録E-3)。タグ運用は IAM に TagResource 追加が要る(人手)
>   # 必須: --instance-names / --availability-zone / --blueprint-id / --bundle-id
>   # 任意: --user-data(cloud-init/初回ブートのみroot実行) / --ip-address-type(dualstack|ipv4|ipv6, 既定dualstack) / --key-pair-name
>   #       出典: AWS CLI create-instances リファレンス（調査[api]）
> ```
> launch script は冒頭に `#!/bin/bash` を付ける（公式例は shebang 無しでも動くが、確実性のため付与＝出典: 調査[inst]）。userData 内では root 実行のため `sudo` 不要。**8080/5432 を開ける記述を userData に入れない**（FW は操作 3 で 22/80/443 のみ）。**DB パスワード・SESSION_SECRET 等の秘密を userData に平文で焼かない**（後述）。

> 🙋 **人が確認/承認する点**
> - **課金開始の最終承認**（CreateInstances＝billable。要承認・出典: 調査[sec] §7）。
> - blueprintId/bundleId が調査で確定した実値か、AZ が `ap-northeast-1a` か。
> - `--ip-address-type` を IPv6-only にしない（ESP8266・Let's Encrypt・運用端末の到達性で詰まる＝公開 IPv4 付き dualstack）。
> - **userData に 8080/5432 公開や秘密のベタ書きが無いか**を必ず目視。DB パスワード・SESSION_SECRET・Bearer トークン等の長期秘密は userData に書かず、systemd の EnvironmentFile（権限 600）で渡す（既存方針。userData は CreateInstances API パラメータとして CloudTrail にも残りうるため平文秘密は厳禁）。

> ✅ **完了判定**
> ```bash
> aws lightsail get-instance-state --instance-name go-iot-prod \
>   --query "state.name" --output text   # → running
> aws lightsail get-instance --instance-name go-iot-prod --region ap-northeast-1 \
>   --query "instance.arn" --output text # → 例 arn:aws:lightsail:ap-northeast-1:<ACCOUNT_ID>:Instance/<INSTANCE_GUID>
> ```
> - `running` を返す。CloudTrail に `CreateInstances` が記録される。
> - ⚠️ **次の操作2.5（IAM 第3ステートメント更新）を必ず実施**してからポート開放（操作3）へ。この ARN の `Instance/` 以降の UUID が `<INSTANCE_GUID>`、`:` の前の12桁が `<ACCOUNT_ID>`（=`aws sts get-caller-identity` の Account）。
> - **cloud-init の成否**は `/var/log/cloud-init-output.log` を A-3（SSH 確立後）で必ず確認するゲートを置く（UserData が効かない既知事例あり＝要確認・出典: 調査[inst]）。**この段階では成否未確定**。

---

#### 操作 2: 静的 IP の確保・付与

既定パブリック IP は再起動・停止で変わりうるため、DNS A レコードの宛先を固定する目的で静的 IP を割り当てる。**アタッチ中は無料・未アタッチ放置は課金**。`AllocateStaticIp` も billable 扱いで承認ゲート対象。

> ▶ **AI への依頼（プロンプト例）**
> 「go-iot-prod 用に静的 IP `go-iot-prod-ip` を確保して、go-iot-prod にアタッチして。付与後に IP を教えて。未アタッチ放置は課金なので、確保したら必ずアタッチまで通して。」

> 🤖 **AI が行う MCP 操作**（実行前に同意要求）
> ```bash
> aws lightsail allocate-static-ip --static-ip-name go-iot-prod-ip --region ap-northeast-1
> aws lightsail attach-static-ip --static-ip-name go-iot-prod-ip --instance-name go-iot-prod --region ap-northeast-1
> aws lightsail get-static-ip --static-ip-name go-iot-prod-ip --region ap-northeast-1 \
>   --query "staticIp.ipAddress" --output text   # → <STATIC_IP> を控える（例 57.182.65.19）。本番FQDN は <STATIC_IP>.sslip.io
> ```

> 🙋 **人が確認/承認する点**
> - allocate/attach の承認（未アタッチ放置課金の回避＝**確保したら必ずアタッチまで**通すことを AI に厳守させる。途中で止めない）。
> - 返ってきた `<STATIC_IP>` を控える（操作 4 の DNS A レコードの宛先。ESP8266 はこの IP ではなく**ドメイン**＝`https://<ドメイン>/api/sensor-data` を `API_ENDPOINT` に設定。出典: firmware/README.md）。

> ✅ **完了判定**
> - `get-static-ip` が `<STATIC_IP>` を返し、`attachedTo` が `go-iot-prod`。CloudTrail に `AllocateStaticIp`/`AttachStaticIp`。

---

#### 操作 2.5: IAM 第3ステートメントを実インスタンス ARN で確定（人手・コンソール・操作3の前提）

**`put-instance-public-ports`（操作3）は §S-3-2 の第3ステートメント（`LightsailManageThisInstance`・特定インスタンス ARN 限定）の権限が要る**。初回 IAM 発行時はインスタンス未作成なので第3ステートメントは未設定（または `Instance/*`）。操作1で ARN が確定したので、**ここで実 ARN を埋めないと操作3が AccessDenied になる**。

1. 操作1で得た ARN（`arn:aws:lightsail:ap-northeast-1:<ACCOUNT_ID>:Instance/<INSTANCE_GUID>`）をコピー。
2. コンソールで **IAM → ユーザー `go-iot-mcp` → 許可 → インラインポリシー `GoIotMcpLightsail` を編集**し、第3ステートメント（`LightsailManageThisInstance`）の `Resource` を実 ARN に置換して保存（CLI なら `aws iam put-user-policy`）。
   - ※ §S-3-2 で第3ステートメントを `Instance/*`（この専用アカウントの全インスタンス）にしてある場合は、この操作2.5 は不要（実 ARN 限定にして締めたいときのみ実施）。
3. 保存後でないと `put/open-instance-public-ports`・`reboot/stop-instance` は AccessDenied。

> ✅ **完了判定**: IAM ユーザー `go-iot-mcp` のインラインポリシー第3ステートメントの `Resource` が実 ARN（または `Instance/*`）になっている。

---

#### 操作 3: ファイアウォール（公開ポートを 22/80/443 のみに・最重要のセキュリティ境界）

**8080（アプリ）と 5432（PostgreSQL）は決して外部公開しない**。`put-instance-public-ports` は**指定ルールで全置換**（含めなかった既存開放ポートは全て閉じる）＝22/80/443 のみ渡せば 8080/5432 は閉じたまま。**誤って 8080/5432 を含めると即公開**されるので、AI が渡す配列を人が必ず点検する（出典: 調査[api]）。SSH(22) は接続元 IP を限定する。

> ✅ **既定は IPv4 の `cidrs` のみ（付録E-3 実績）**: 操作1 が `dualstack` でも、`cidrs`（IPv4）だけ指定すれば 22/80/443 のみに全置換でき IPv6 側は開かない。**`ipv6Cidrs` は付けない**＝管理元に固定 IPv6 が無いのが普通で、付けると埋めようのない `<管理元IPv6>` で詰まる。管理元に固定 IPv6 があり IPv6 SSH を使う場合のみ各ルールに `ipv6Cidrs` を足す。
>
> 📍 **`<管理元IP>` の取得**: SSH 接続元（このローカル端末）のグローバル egress IP。`curl -s https://checkip.amazonaws.com` で取得し `/32` を付ける。動的IPなら変わるたびに put-instance-public-ports で更新（付録E-9）。
>
> 🔑 **前提**: 操作2.5 で IAM 第3ステートメントを実 ARN（または `Instance/*`）に更新済みであること（未更新だと AccessDenied）。

> ▶ **AI への依頼（プロンプト例）**
> 「go-iot-prod の公開ポートを、HTTP(80) と HTTPS(443) は anywhere（IPv4 `0.0.0.0/0` と IPv6 `::/0`）、SSH(22) は私の管理元 IP `<管理元IP>/32`（と該当する IPv6 があれば `/128`）からのみ、の 3 つだけに**全置換**して。8080 と 5432 は IPv4/IPv6 とも絶対に含めないで。設定後、開いているポート一覧を見せて 8080/5432 が無いことを確認させて。」

> 🤖 **AI が行う MCP 操作**（put-instance-public-ports＝全置換・実行前に同意要求）
> ```bash
> aws lightsail put-instance-public-ports \
>   --instance-name go-iot-prod --region ap-northeast-1 \
>   --port-infos \
>     'fromPort=22,toPort=22,protocol=TCP,cidrs=["<管理元IP>/32"]' \
>     'fromPort=80,toPort=80,protocol=TCP,cidrs=["0.0.0.0/0"]' \
>     'fromPort=443,toPort=443,protocol=TCP,cidrs=["0.0.0.0/0"]'
> # ※ IPv4 cidrs のみで 22/80/443 に全置換＝IPv6 側は開かない（付録E-3 実績）。管理元に固定IPv6があり IPv6 SSH を使うときだけ各ルールに ipv6Cidrs を足す。
> # 検証（全置換後の状態。8080/5432 が無いこと）
> aws lightsail get-instance-port-states --instance-name go-iot-prod --region ap-northeast-1 \
>   --query "portStates[].[fromPort,toPort,protocol,state,cidrs,ipv6Cidrs]" --output table
> ```
> 1 ポートだけ追加したい将来用途では `open-instance-public-ports`（単数形 `--port-info`・既存を閉じない追加のみ）を使うが、本操作は全置換の `put-` を使う。

> 🙋 **人が確認/承認する点**
> - **渡す `--port-infos` 配列に 8080/5432 が（IPv4/IPv6 とも）含まれていないこと**（AI が提示したコマンドを人が目視。全置換のため最重要）。
> - 22 の `cidrs`/`ipv6Cidrs` が `0.0.0.0/0`・`::/0`（anywhere）になっていないか＝**管理元アドレス限定**であること。80/443 が anywhere なのは ESP8266（多拠点・動的 IP）と Let's Encrypt（HTTP-01=80／TLS-ALPN=443）の到達のため。

> ✅ **完了判定**
> - `get-instance-port-states` の出力が **22/80/443 のみ**で、**8080 と 5432 が存在しない**。22 の cidrs/ipv6Cidrs が管理元限定。CloudTrail に `PutInstancePublicPorts`。
> - 多層防御として OS 側 `ufw` を 22/80/443 のみで有効化する設計は A-2 に委ねる（8080/5432 は localhost バインドで守る＝既存方針維持）。

---

#### 操作 4: 本番FQDN を決める（既定＝sslip.io・DNS 作成は不要）

**既定（本案件）＝ドメイン無し → `<STATIC_IP>.sslip.io`**。sslip.io は `<IPアドレス>.sslip.io` がそのIPに解決する無料ワイルドカードDNSで、**ドメイン購入・Lightsail DNS（`create-domain`）・NS 設定・伝播待ちが一切不要**。本番FQDN は操作2の静的IPをドット連結するだけ（例: `57.182.65.19` → `57.182.65.19.sslip.io`）。Caddy はこの FQDN で Let's Encrypt を TLS-ALPN-01（443）で自動取得（A-4）。確認は **ローカル端末で `dig +short <STATIC_IP>.sslip.io A` が `<STATIC_IP>` を返せばOK**（即時・伝播待ち不要・付録E-7 実績）。**`create-domain`/`create-domain-entry` は実行しない**。→ そのまま A-4 へ進む。

**以下は（任意）独自ドメインを使う場合のみ。** ドメインの A レコードを `<STATIC_IP>` に向ける。Caddy の Let's Encrypt 自動取得は FQDN への到達が前提で、**DNS 伝播完了後**に Caddy を起動する（伝播前に取得を試みると失敗）。⚠️ **Lightsail のドメイン/DNS 系 API は `us-east-1` でのみ動作**＝インスタンスは `ap-northeast-1`、DNS は `us-east-1` とリージョンが分かれる。`--region us-east-1` を付けないと失敗する（最重要・出典: 調査[api]）。Lightsail DNS ゾーンは最大 6 個・A/AAAA/CNAME/MX/NS/SOA/SRV/TXT のみ。本件（A レコード 1 本）は Lightsail DNS で十分だが、既存運用が Route 53 なら `route53 change-resource-record-sets` でも可。**ドメイン購入自体は人手（課金）**。

> ⚠️ **AAAA（IPv6）レコードは作らない**: dualstack で IPv6 が付いていても、Caddy が AAAA を見て IPv6 でも証明書取得を試み、IPv6 の到達性が整っていないと取得失敗しうる。本件は A レコード 1 本のみ作成する。

> ▶ **AI への依頼（プロンプト例）**
> 「購入済みドメイン `<ドメイン>` の DNS ゾーンを Lightsail に作って（初回のみ）、A レコード `<ドメイン>` → `<STATIC_IP>` を作成して。AAAA は作らないで。Lightsail の DNS API は us-east-1 限定なので `--region us-east-1` を必ず付けて。作成後 dig で名前解決を確認させて。」

> 🤖 **AI が行う MCP 操作**（DNS は us-east-1・実行前に同意要求）
> ```bash
> # ゾーン作成（初回のみ。Lightsail で DNS 管理する場合）
> aws lightsail create-domain --domain-name <ドメイン> --region us-east-1
> # A レコード（type=A, target=静的IP, isAlias=false）
> aws lightsail create-domain-entry \
>   --domain-name <ドメイン> \
>   --domain-entry 'name=<ドメイン>,type=A,target=<STATIC_IP>,isAlias=false' \
>   --region us-east-1
> # 確認
> aws lightsail get-domain --domain-name <ドメイン> --region us-east-1 \
>   --query "domain.domainEntries[].[name,type,target]" --output table
> ```
> ※ ドメインのネームサーバが Lightsail を指していない（レジストラ側 NS が別）場合は、レジストラで NS を Lightsail DNS に向ける作業が別途必要（**人手・要確認**）。既存 NS が Route 53 等なら Lightsail でなく Route 53 側に A レコードを作る判断もある。

> 🙋 **人が確認/承認する点**
> - ドメインが購入済み（人手・課金）で、ネームサーバが Lightsail（または Route 53）の管理下にあること（レジストラの NS 設定は人手）。
> - `--region us-east-1` が付いているか（無いと DNS API が失敗）。
> - 既存運用が Route 53 か Lightsail DNS か（ゾーン最大 6 個・レコード型制約に抵触しないか・出典: 調査[api]）。

> ✅ **完了判定**
> ```bash
> dig +short <ドメイン> A      # → <STATIC_IP> が返る（DNS 伝播完了の確認）
> ```
> - `dig`（人手で打つか、AI が shell 系 MCP/CLI を持つ場合は実行。`call_aws` は AWS API 専用で `dig` は実行できない）で `<ドメイン>` が `<STATIC_IP>` を返す。**伝播完了後**に A-4（Caddy 本番ドメインの自動TLS化）へ進む（Let's Encrypt は 80 番到達を要求＝操作 3 で 80 を開けてある）。CloudTrail に `CreateDomain`/`CreateDomainEntry`。

---

### A-1-C. このセクションの完了確認（次工程の前提）

AI に下記を `call_aws`（read-only）でまとめて確認させ、人が結果を点検する。

```bash
aws lightsail get-instance-state --instance-name go-iot-prod --query "state.name" --output text   # running
aws lightsail get-static-ip --static-ip-name go-iot-prod-ip --query "staticIp.[ipAddress,attachedTo]" --output table
aws lightsail get-instance-port-states --instance-name go-iot-prod \
  --query "portStates[].[fromPort,toPort,protocol,state,cidrs,ipv6Cidrs]" --output table   # 22/80/443 のみ・8080/5432 不在
aws lightsail get-domain --domain-name <ドメイン> --region us-east-1 \
  --query "domain.domainEntries[?type=='A'].[name,target]" --output table
# 名前解決（人手 or shell 系 MCP。call_aws では不可）
dig +short <ドメイン> A
```

- インスタンス `go-iot-prod` が東京リージョンで `running`、OS は Ubuntu LTS、**amd64 バンドルで `GOARCH=amd64` 確定**。
- 静的 IP `<STATIC_IP>` がアタッチ済み（未アタッチ放置課金なし）。
- 公開ポートは **22/80/443 のみ**（22 は管理元アドレス限定・IPv4/IPv6 とも）、**8080/5432 が存在しない**。
- `<ドメイン>` の A レコードが `<STATIC_IP>` を指し、`dig` で解決できる（DNS 伝播完了）。AAAA は作っていない。
- cloud-init（userData）の初期構成（swap/PostgreSQL/Caddy/systemd）の成否は A-3（SSH 確立後）に `/var/log/cloud-init-output.log` で検証するゲートが残っている（**この時点では未検証**）。
- 全操作が CloudTrail に MCP 用 IAM プリンシパルで記録され、監査可能。

次（A-2 以降）でやること:
- **A-2**: userData の launch script の中身を起草（swap・PGDG/PostgreSQL 低メモリチューニング・Caddy・systemd 雛形）。秘密は EnvironmentFile（権限 600）で渡し userData に焼かない。本セクションはそれを `--user-data` で渡す設計まで。
- **A-3**: ローカルで `make build`（`sync-css → templ generate → build`・`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`）したバイナリのみ配布。**デプロイ反復（バイナリ配布・systemd 再起動・ログ確認・トークン発行 `cmd/gen-token`）は SSH を既定**（AI が `get-instance-access-details` で一時鍵を取得して接続。一時鍵は有効期限ありなので接続直前に取得する）。SSM は Lightsail ネイティブ非対応＝ハイブリッドアクティベーションで managed node 化すれば Session Manager/Run Command 可だが、SSM Agent 常駐メモリ増のトレードオフがあり低メモリ機では任意オプション（出典: 調査[inst]/[api]/[sec]）。**AWS API MCP の `call_aws` は AWS API のみで SSH/scp は扱わない**ため、反復は SSM Run Command か shell 系 MCP/CLI 経由になる点に留意。
- **A-4**: DNS 伝播完了を確認してから Caddy 自動 TLS で 443→localhost:8080 を終端（本番ドメインへ差し替え）。AAAA は作らない。続けて受け入れ確認を行う。

---

## A-2. インスタンス初回自動構成（cloud-init launch script）

A-1 で AI に `aws lightsail create-instances` を実行させる際、`--user-data` に渡す**初回ブート自動構成スクリプト（cloud-init / launch script）**を本セクションで確定する。これが「人が SSH で打たない」運用の中核である。**swap・タイムゾーン・SSH ハードニング・native apt PostgreSQL（＋低メモリチューニング＋localhost bind＋本番 DB/ロール作成）・Caddy 導入と Caddyfile 雛形・アプリ配置先と専用ユーザ作成**を、**1 本の冪等な cloud-init スクリプトに集約**し、CreateInstances 時に渡しきる。

> **なぜ作成時に渡しきるのか（cloud-init の重大制約）**: Lightsail の launch script は **インスタンス作成時（初回ブート）に root で一度だけ**実行され、**作成後に追加・再実行できない**（"launch scripts can't retroactively run after deployment"）（要確認・出典: AWS Compute Blog "Create, Use, and Troubleshoot Launch Scripts on Amazon Lightsail" / 2026-06 時点。re:Post の "UserData not working" 既知質問は本文未取得=要確認）。root 実行のため**スクリプト内に `sudo` は不要**だが、root から特定ユーザへ降りる用途（`sudo -u postgres` 等）は残す。よって OS 土台（swap/TZ/SSH/PostgreSQL/Caddy/配置先/専用ユーザ）はここに渡しきり、**反復系（バイナリ配布・systemd 起動・トークン発行・マイグレーション）は初回ブートでは賄えない**ため A-3 以降（SSH 既定）に委ねる。

### 不変条件の維持（このスクリプトで必ず守る）

- 低メモリ前提 → **swap を最初に作成**（OOM 回避）。
- **PostgreSQL は同一インスタンスに native apt（Docker 不使用）**、低メモリチューニング、`listen_addresses='localhost'`（5432 を外部 bind しない。Debian/Ubuntu の PGDG パッケージは既定で localhost のため、本スクリプトは belt-and-suspenders で明示固定する）。
- アプリは別途 systemd で起動（A-3）。アプリは `:8080`（= `0.0.0.0:8080`、全インターフェース bind。`cmd/server/main.go` L58 `addr := fmt.Sprintf(":%d", cfg.AppPort)` で確認）のため、**8080/5432 の外部遮断はファイアウォール（Lightsail FW を A-1 で設定済み）が一次防御**。本スクリプトは listen を localhost に縛れる範囲（DB）だけ縛り、アプリ 8080 は FW 依存である点を変えない（OS ufw を二重化で足すのも可だが、本スクリプトでは Lightsail FW を一次防御とし ufw は A-1/別章に委ねる）。
- **HTTPS は前段 Caddy 自動 TLS**（Let's Encrypt）。本スクリプトは Caddy を導入し Caddyfile 雛形を置くが、**DNS 伝播前に証明書取得が走らないよう、雛形は到達確認用に `:80`（HTTP のみ・自動 TLS を要求しない形）で待受**にしておき、ドメイン確定後（A-4）に本番ドメインへ差し替える。ESP8266 センサーは **ドメインへ HTTPS で送る**（`firmware/esp8266_sht31/config.example.h` の `API_ENDPOINT` は `https://...`）ため、`:80` 雛形は到達確認専用であり A-4 で 443 へ切り替えるまでセンサー本番投入はしない。
- 環境変数は `.env` 自動読込なし（`internal/config/config.go` L21 のコメントどおり、`.env` 読込は Makefile 側＝本番では行わない）→ **systemd の EnvironmentFile/Environment で渡す**（A-3）。本スクリプトは環境ファイルの**置き場と権限 600 の枠だけ**用意し、秘密値は焼かない。

> **秘密値を userData に焼かない方針（厳守）**: DB パスワード・`SESSION_SECRET`・Bearer トークンは **launch script に平文で書かない**。理由＝Lightsail の launch script / user-data は **コンソールやインスタンスメタデータから後から閲覧できる**ことがあり（要確認・2026-06 時点。メタデータ経由の user-data 取得は cloud-init の一般的挙動）、平文秘密が残ると漏えい経路になる。本スクリプトでは DB パスワードを**インスタンス内で `openssl rand` 生成し root:600 のファイルに格納**（外に出さない）、`SESSION_SECRET` は **A-3 で人が安全に注入**（systemd EnvironmentFile を後段で配置）する。より厳格にするなら DB パスワードも生成せず後段注入とし、cloud-init はロール作成を**スキップ**して A-3 で人が投入する選択も可（本書は「生成して localhost-only に閉じる」を既定とし、外部到達不可なので実害が小さい前提）。

---

### このセクションの対話フロー全体像

```
▶ 人 → AI: 「この launch script を userData にして go-iot-prod を作成して」
🤖 AI(MCP): call_aws → aws lightsail create-instances --user-data file://deploy/cloud-init.sh ...
🙋 人: ① userData スクリプトに秘密が焼かれていないかレビュー ② 課金開始(CreateInstances)を承認
🤖 AI(MCP): REQUIRE_MUTATION_CONSENT の同意ダイアログ(対応時)→ 実行
✅ 人 → AI: cloud-init 完了ログ(/var/log/cloud-init-output.log)の確認を依頼 → 成功判定
```

---

### ▶ AI への依頼（プロンプト例）

> 「東京リージョン（`ap-northeast-1`）に `go-iot-prod` を Ubuntu LTS（22.04 または 24.04・`blueprint-id` は実値要確認）/ Micro-1GB（`bundle-id` は実値要確認）で作成して。`--user-data` には、これから渡す cloud-init スクリプト（`./deploy/cloud-init.sh`）をそのまま使って。**秘密値は一切焼いていない**ことを実行前に一緒に確認したい。静的 IP の割り当てとファイアウォール（22 は管理元 IP 限定・80/443 のみ、8080・5432 は開けない）は A-1 の手順で。」

補助的に、最新の引数・ID を AI に裏取りさせる依頼例:

> 「`suggest_aws_commands` で `lightsail create-instances に user-data を file 指定で渡す最新の CLI 形式` を確認して。あわせて `get-blueprints`・`get-bundles`（read-only）で Ubuntu LTS の `blueprint-id` と Micro-1GB の `bundle-id` の実値・東京リージョン実額を取得して。」（read-only なので `READ_OPERATIONS_ONLY=true` でも実行可）

---

### 🤖 AI が行う MCP 操作（裏で呼ばれる AWS API / コマンド）

AI は **AWS API MCP Server**（`awslabs.aws-api-mcp-server`・`uvx awslabs.aws-api-mcp-server@latest`）の `call_aws` で、A-1 で確定した引数に `--user-data` を足して `create-instances` を実行する。`--user-data` は **inline 文字列**でも **`file://`** でも渡せる（出典: AWS CLI `create-instances` リファレンス）。長い cloud-init は `file://` が確実。

> 補足（公式の後継動向・要確認）: awslabs は self-host の `aws-api-mcp-server` を **マネージドの「AWS MCP Server / Agent Toolkit for AWS」へ統合する方向**（`mcp-proxy-for-aws` 経由・OAuth2.1→SigV4 変換、提供リージョン us-east-1 / eu-central-1・東京提供有無は要確認）と案内している。ただし self-host 版は当面現役で、本書は self-host 版（自前 IAM 鍵）を既定とする。どちらを採るかは「自前 IAM 鍵の初回発行（人手不可避）」とのトレードオフで選定する（要確認・出典: docs.aws.amazon.com/agent-toolkit / 2026-06 時点）。

```text
# AI が内部で実行する CLI（call_aws の入力。<...> は A-1 の実値・get-bundles/get-blueprints で確認）
aws lightsail create-instances \
  --instance-names go-iot-prod \
  --availability-zone ap-northeast-1a \
  --blueprint-id ubuntu_24_04 \
  --bundle-id micro_3_0 \
  --ip-address-type dualstack \
  --user-data file://deploy/cloud-init.sh \
  --region ap-northeast-1
```

- `create-instances` は **billable（課金開始）かつ非 read-only** 操作なので、`REQUIRE_MUTATION_CONSENT=true`（elicitation 対応クライアント時）または `mcp-security-policy.json` の `elicitList` により**実行前に人の承認を要求**する（出典: awslabs AWS API MCP Server ドキュメント・2026-06 時点。Claude Code の elicitation 対応可否は要確認 → 未対応なら同意ダイアログが出ないため、`READ_OPERATIONS_ONLY` の付け外し運用＋IAM 最小権限＋人の最終承認で代替する）。
- `--user-data` を `file://` で渡すには MCP 側の**ローカルファイルアクセス許可**が要る。`AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS` の既定は `workdir`（作業ディレクトリ限定）なので、`deploy/cloud-init.sh` を `AWS_API_MCP_WORKING_DIR` 配下に置く（要確認・2026-06 時点）。`file://` が読めない場合は inline 文字列でも渡せる。

#### userData に渡す cloud-init スクリプト全文（`deploy/cloud-init.sh`）

> ⚠️ **【コピペ前に必読・付録E-4 実証】下のスクリプト全文は設計記録（旧版）。正本はリポジトリの `deploy/cloud-init.sh`（4バグ修正済）を使う**。下記をそのまま userData に貼ると、**Lightsail が `/bin/sh`(dash) で実行するため `set -o pipefail` で即死**するなど 4 バグ（①`pipefail`→`set -eu` ②`psql -c` の `:'pass'` 非展開→heredoc `\set` ③`umask 077` 漏れで Caddy 鍵リング 600→NO_PUBKEY→サブシェルに閉込め＋`chmod 644` ④600 鍵リング連鎖故障）で **OS 土台が一切構成されない**。さらに **cloud-init は初回ブートで失敗しやすく再実行不可**なので、A-3 の SSH 確立後に **`deploy/cloud-init.sh` を scp して `sudo bash deploy/cloud-init.sh` で手動冪等実行**して完成させる（付録E-4）。

**冪等**（再実行・部分失敗後の再ブートでも壊れない）に作る。各処理の前に「既に done か」を判定し、二重作成を避ける。秘密値はプレースホルダ／インスタンス内生成のみで、**平文を焼かない**。

```bash
#!/bin/bash
# go_iot 本番インスタンス 初回自動構成 (cloud-init launch script / Lightsail user-data)
# 実行: 初回ブート時に root で1回のみ (再実行不可)。ログ: /var/log/cloud-init-output.log
# 方針: 秘密値(DBパスワード/SESSION_SECRET)は平文で焼かない。
#   - DBパスワードはインスタンス内で生成し /root/go_iot_db_pass(600) に保管 (外に出さない)
#   - SESSION_SECRET と systemd EnvironmentFile は A-3 で人が安全注入する
set -eu   # ⚠️ pipefail は Lightsail の dash 実行で即死(付録E-4①)。POSIX sh 互換の set -eu にする
export DEBIAN_FRONTEND=noninteractive
log() { echo "[go_iot cloud-init] $(date -Is) $*"; }
log "=== 開始 ==="

# 設定値 (秘密ではない。必要なら作成時に置換)
PG_VER=16
APP_DIR=/opt/go_iot
APP_USER=go_iot
SWAP_SIZE=2G              # Nano-0.5GB は 1G に下げる
SWAPPINESS=10

# ---------------------------------------------------------------------------
# 0. タイムゾーン (JST。ログと recorded_at(JST) の突き合わせのため)
# ---------------------------------------------------------------------------
if [ "$(cat /etc/timezone 2>/dev/null || true)" != "Asia/Tokyo" ]; then
  log "タイムゾーンを Asia/Tokyo に設定"
  timedatectl set-timezone Asia/Tokyo || ln -sf /usr/share/zoneinfo/Asia/Tokyo /etc/localtime
fi

# ---------------------------------------------------------------------------
# 1. swap (低メモリ機の OOM 回避・最優先・冪等)
# ---------------------------------------------------------------------------
if ! swapon --show | grep -q '/swapfile'; then
  log "swap ($SWAP_SIZE) を作成"
  fallocate -l "$SWAP_SIZE" /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
fi
grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
# 普段は物理RAM優先 (ピーク時のみ swap)
printf 'vm.swappiness=%s\nvm.vfs_cache_pressure=50\n' "$SWAPPINESS" > /etc/sysctl.d/99-swap.conf
sysctl --system >/dev/null

# ---------------------------------------------------------------------------
# 2. パッケージ更新 + 共通ツール
# ---------------------------------------------------------------------------
log "apt update / 共通パッケージ"
apt-get update -y
apt-get install -y curl ca-certificates gnupg lsb-release debian-keyring debian-archive-keyring apt-transport-https

# ---------------------------------------------------------------------------
# 3. SSH ハードニング (パスワード認証無効 / root SSH 無効 / 公開鍵のみ)
#    Lightsail 既定鍵(authorized_keys)は cloud-init が配置済み。鍵ログインを切らない。
#    drop-in を 99- で後勝ちにし、cloud-init の 50-cloud-init.conf を上書き。
#    注意: Ubuntu 22.04.2+/24.04 は ssh が socket activation (ssh.socket) の場合があり、
#          ssh.service の restart だけでは設定が再読込されないことがある。両系統を扱う。
# ---------------------------------------------------------------------------
log "SSH ハードニング"
cat > /etc/ssh/sshd_config.d/99-hardening.conf <<'SSHD'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin no
PubkeyAuthentication yes
SSHD
if sshd -t; then
  systemctl daemon-reload
  # socket activation 環境では ssh.socket を、従来型では ssh(.service)/sshd を再起動
  if systemctl is-active --quiet ssh.socket; then
    systemctl restart ssh.socket
  fi
  systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
else
  log "WARN: sshd -t 失敗。設定を反映せず継続 (締め出し回避)"
fi

# ---------------------------------------------------------------------------
# 4. PostgreSQL 16 (native apt / PGDG) + 低メモリチューニング + localhost bind
# ---------------------------------------------------------------------------
if ! command -v psql >/dev/null 2>&1; then
  log "PGDG リポジトリ + postgresql-${PG_VER} 導入"
  install -d /usr/share/postgresql-common/pgdg
  curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
    -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc
  echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] \
https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list
  apt-get update -y
  apt-get install -y "postgresql-${PG_VER}"
fi

PG_CONF="/etc/postgresql/${PG_VER}/main/postgresql.conf"   # Debian/Ubuntu 配置 (要確認)
PG_HBA="/etc/postgresql/${PG_VER}/main/pg_hba.conf"

# 4-1. localhost のみ待受 (5432 を外部 bind しない。Debian 既定も localhost だが明示固定)
log "PostgreSQL を localhost 待受に固定 + 低メモリチューニング"
sed -i "s/^#\?listen_addresses\s*=.*/listen_addresses = 'localhost'/" "$PG_CONF"
sed -i "s/^#\?password_encryption\s*=.*/password_encryption = scram-sha-256/" "$PG_CONF"

# 4-2. 低メモリチューニング (RAM 1GB級・DBとapp同居前提。値は要調整)
#      既存同名行と衝突しないよう、専用ブロックを冪等に管理
if ! grep -q '# --- go_iot tuning ---' "$PG_CONF"; then
  cat >> "$PG_CONF" <<'PGCONF'

# --- go_iot tuning --- (RAM 1GB級・DBとapp同居前提 / 値は要調整)
shared_buffers = 128MB
effective_cache_size = 512MB
work_mem = 8MB
maintenance_work_mem = 64MB
max_connections = 20
PGCONF
fi

# 4-3. pg_hba を local / ループバックのみ (外部 host 行を作らない)
#      Debian 既定はほぼこの形。scram-sha-256 に統一。
sed -i 's/^\(host\s\+all\s\+all\s\+127\.0\.0\.1\/32\s\+\).*/\1scram-sha-256/' "$PG_HBA" || true
sed -i 's/^\(host\s\+all\s\+all\s\+::1\/128\s\+\).*/\1scram-sha-256/' "$PG_HBA" || true

systemctl enable postgresql
systemctl restart postgresql

# 4-4. 本番ロール go_iot / DB go_iot を作成 (冪等)
#      DBパスワードはインスタンス内で生成し /root/go_iot_db_pass(600) に保管 (平文を焼かない)。
#      A-3 で systemd EnvironmentFile の DATABASE_URL を組み立てる際にこのファイルを参照する。
PASS_FILE=/root/go_iot_db_pass
if ! sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='go_iot'" | grep -q 1; then
  log "本番ロール go_iot を生成パスワードで作成 (パスワードは $PASS_FILE 600 に保管)"
  # URLエンコード事故を避けるため記号を除いた32文字
  DB_PASS="$(openssl rand -base64 24 | tr -d '/+=' | cut -c1-32)"
  # umask はサブシェルに閉じ込める (077 が後続の Caddy 鍵リング作成に漏れると 600 で作られ _apt が読めず NO_PUBKEY・付録E-4③)
  ( umask 077; printf '%s' "$DB_PASS" > "$PASS_FILE" )
  chmod 600 "$PASS_FILE"
  # psql -c では :'pass' が展開されず syntax error → stdin スクリプトモードの \set で渡す (付録E-4②)
  sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
\set pass '$DB_PASS'
CREATE ROLE go_iot WITH LOGIN PASSWORD :'pass';
SQL
  unset DB_PASS
fi
sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='go_iot'" | grep -q 1 \
  || sudo -u postgres createdb -O go_iot go_iot

# ---------------------------------------------------------------------------
# 5. アプリ配置先 + 専用システムユーザ + 環境ファイルの枠 (秘密は焼かない)
# ---------------------------------------------------------------------------
log "配置先 $APP_DIR と専用ユーザ $APP_USER を用意"
id "$APP_USER" >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin "$APP_USER"
install -d -m 750 -o "$APP_USER" -g "$APP_USER" "$APP_DIR"
# 環境ファイルは「枠」だけ作る。SESSION_SECRET と DATABASE_URL は A-3 で人が安全注入。
if [ ! -f "$APP_DIR/.env" ]; then
  cat > "$APP_DIR/.env" <<'ENVTPL'
APP_ENV=production
APP_PORT=8080
# DATABASE_URL=postgres://go_iot:<URLエンコード済みパスワード>@localhost:5432/go_iot?sslmode=disable
# SESSION_SECRET=<32文字以上のランダム値>   # A-3 で安全注入 (平文を焼かない)
ENVTPL
  chown "$APP_USER":"$APP_USER" "$APP_DIR/.env"
  chmod 600 "$APP_DIR/.env"
fi

# ---------------------------------------------------------------------------
# 6. Caddy (公式 apt) + Caddyfile 雛形
#    DNS 伝播前に Let's Encrypt 取得が暴発しないよう、雛形は :80 (HTTP・自動TLS要求なし) にする。
#    ドメイン確定後 (A-4) に本番ドメインへ差し替えて自動TLSを有効化する。
# ---------------------------------------------------------------------------
if ! command -v caddy >/dev/null 2>&1; then
  log "Caddy 導入 (公式 apt)"
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
    | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
    > /etc/apt/sources.list.d/caddy-stable.list
  # 鍵リング/.list を _apt ユーザが読めるよう明示 644 (umask 漏れ対策・付録E-4③④)
  chmod 644 /usr/share/keyrings/caddy-stable-archive-keyring.gpg /etc/apt/sources.list.d/caddy-stable.list
  apt-get update -y
  apt-get install -y caddy
fi
# 雛形: 到達確認用に :80 (HTTP) → 127.0.0.1:8080。本番ドメイン+自動TLSは A-4 で差し替え。
if ! grep -q 'go_iot placeholder' /etc/caddy/Caddyfile 2>/dev/null; then
  cp -n /etc/caddy/Caddyfile /etc/caddy/Caddyfile.bak 2>/dev/null || true
  cat > /etc/caddy/Caddyfile <<'CADDY'
# go_iot placeholder (A-4 で本番ドメイン + email に差し替えて自動TLSを有効化する)
# 例:
#   {
#       email <ADMIN_EMAIL>
#   }
#   <DOMAIN> {
#       reverse_proxy localhost:8080
#   }
:80 {
    reverse_proxy localhost:8080
}
CADDY
  caddy fmt --overwrite /etc/caddy/Caddyfile || true
fi
systemctl enable caddy
systemctl restart caddy || true   # アプリ未起動でも Caddy 自体は上がる

log "=== 完了 (アプリ本体/SESSION_SECRET/DATABASE_URL/本番ドメインTLSは A-3/A-4 で) ==="
```

> このスクリプトが触らないもの（＝反復系・A-3 以降）: アプリバイナリ（`go_iot_server`。ローカル Mac で `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`・サーバでビルドしない）の配布、`SESSION_SECRET` の実値注入、`DATABASE_URL` の最終組み立て、goose マイグレーション（現在 v7）、`go_iot.service`（systemd）の起動、トークン発行（`cmd/gen-token`）、本番ドメインの Caddyfile 差し替え。これらは初回ブートでは賄えないため SSH 既定で実施する。

---

### 🙋 人が確認 / 承認する点

cloud-init は root 全権・初回のみ・再実行不可なので、**実行前のスクリプトレビューが最大の安全弁**。AI に渡す `deploy/cloud-init.sh` を人が次の観点で精査する。

1. **秘密値が焼かれていないこと（最重要）**: スクリプト内に DB パスワード平文・`SESSION_SECRET` 平文・Bearer トークンが**書かれていない**ことを目視確認。DB パスワードは `openssl rand` 生成＋`/root/go_iot_db_pass`(600) 保管のみで、外（git・チャット・userData 文字列）に出ない設計か。
2. **不変条件の維持**: `listen_addresses='localhost'`（5432 を外部 bind しない）、Caddyfile 雛形が**本番ドメインの自動 TLS を暴発させない**（`:80` HTTP 待受で自動 TLS を要求していない）こと、環境ファイルが 600・専用ユーザ所有であること。
3. **課金（billable）の承認**: `create-instances` はインスタンス起動＝課金開始。`REQUIRE_MUTATION_CONSENT` の同意（elicitation 非対応クライアントなら人の明示承認）。プラン（Micro-1GB 等）と月額が想定どおりか（時点要確認・コンソール実額）。
4. **MCP 安全策の確認**: AI を動かす AWS API MCP Server が **最小権限 IAM**（Lightsail 限定・`lightsail:CreateInstances`／`lightsail:GetBundles` 等のみ。`lightsail:*` フルや破壊系は付与しない）で、`mcp-security-policy.json` に破壊系（`aws lightsail delete-instance`／`aws lightsail release-static-ip` 等・完全一致で1行ずつ。ワイルドカード不可）が `denyList` 登録されていること。CloudTrail に `CreateInstances` が残る前提（Lightsail は CloudTrail 統合済み）。

> 補足（誠実な明記・AI に任せられない人手作業）: AWS アカウント作成・課金/クレカ登録・ルート/MFA・ドメイン購入・**MCP 用 最小権限 IAM 認証情報の初回発行**・ESP8266 物理書込（B）・billable/破壊操作の**最終承認**は人手が不可避で AI に任せられない（A-1 / セットアップ章で実施済みの前提）。本セクションの AI 操作はすべて「MCP 接続済み＋IAM 鍵発行済み」を前提とする。

---

### ✅ 完了判定（AI に確認を依頼）

cloud-init は `create-instances` の応答だけでは成否が分からない（バックグラウンドで進行）。**ログ確認を AI に依頼**して判定する。

> **SSH 鍵の入手（✅実証済み・付録E-5）**: 完了判定は SSH 経由でログ/サービス状態を取る。鍵は端末で直接 `aws lightsail get-instance-access-details --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp` で取得（**`download-default-key-pair` は AccessDenied で不可**）。返る一時アクセスは **SSH 証明書ベースで `privateKey` に加え `certKey` が必須**（無いと `Permission denied (publickey)`）。`~/.ssh/lightsail-goiot.pem`(600)＋`~/.ssh/lightsail-goiot.pem-cert.pub`(600・`<秘密鍵名>-cert.pub` 命名で ssh 自動探索)に置き、`ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@<STATIC_IP>`（ユーザー=`ubuntu`）。**証明書は約13分で失効**＝取得直後に scp/ssh をまとめて実行。鍵は `call_aws`（チャットに出る）ではなく端末で直接 `aws` 実行しファイルへ落として `privateKey`/`certKey` を抽出する（チャット/コミットに鍵を残さない）。`call_aws` は AWS API のみで SSH/scp 自体は実行せず、SSH は Claude Code の Bash 等ローカルシェルで打つ（付録E-6・別 shell MCP 不要）。

▶ 依頼例:
> 「`go-iot-prod` の cloud-init が成功したか確認したい。インスタンスが `running` になったら、SSH で `/var/log/cloud-init-output.log` の末尾と各サービスの状態を取得して、`[go_iot cloud-init] === 完了` が出ているか、PostgreSQL と Caddy が active か、swap が有効かを報告して。」

🤖 AI が行う確認:

- 起動状態（read-only・MCP）: `call_aws` → `aws lightsail get-instance-state --instance-name go-iot-prod`（`running` を確認）。`READ_OPERATIONS_ONLY` でも実行可。
- cloud-init ログ・サービス状態（**SSH 経由**。AWS API MCP の `call_aws` は AWS API のみで SSH/scp は扱わないため、デプロイ反復と同じく SSH を既定とする。将来 SSM ハイブリッドアクティベーションで Run Command 化も可だが、SSM Agent 常駐メモリが低メモリ機には負担で、かつ別途エージェント導入＋IAM ロールが要るため任意オプション・要確認）:

```bash
# AI が SSH 経由で実行する確認コマンド (人が監査できるよう明示。root 取得は sudo)
sudo tail -n 40 /var/log/cloud-init-output.log    # "[go_iot cloud-init] === 完了" が出ること
cloud-init status --long                           # status: done (要確認: Lightsail での cloud-init status 対応)
free -h                                             # Swap 行に 2.0Gi 等
timedatectl | grep 'Time zone'                      # Asia/Tokyo (JST, +0900)
systemctl is-active postgresql caddy                # ともに active
sudo ss -tlnp | grep 5432                           # 127.0.0.1:5432 / ::1 のみ (0.0.0.0 が無い)
sudo sshd -T | grep -Ei 'passwordauthentication|permitrootlogin'  # ともに no
sudo -u postgres psql -tAc "SELECT rolname FROM pg_roles WHERE rolname='go_iot'"  # go_iot
sudo -u postgres psql -tAc "SELECT datname FROM pg_database WHERE datname='go_iot'"  # go_iot
sudo test -f /root/go_iot_db_pass && sudo stat -c '%a' /root/go_iot_db_pass  # 600
```

完了条件（すべて満たすこと）:
- `/var/log/cloud-init-output.log` に `[go_iot cloud-init] === 完了` が出ている（途中で `set -euo pipefail` により失敗していない）。
- `free -h` で swap 有効、`timedatectl` が Asia/Tokyo。
- PostgreSQL・Caddy が `active`、5432 は `127.0.0.1`/`::1` のみ待受（外部 bind なし）。
- SSH はパスワード認証・root ログインともに `no`、鍵ログインは引き続き可能（締め出されていない。socket activation 環境では `ssh.socket` 再起動済みであること）。
- ロール `go_iot`・DB `go_iot` が存在し、`/root/go_iot_db_pass` が 600 で存在（DB パスワードはインスタンス内のみ）。
- ファイアウォール（A-1 で設定）が 22（管理元 IP 限定）/80/443 のみで、`get-instance-port-states` に 8080/5432 が無い。

> ここまで通れば OS 土台は完成。次は A-3（バイナリ配布・`SESSION_SECRET`/`DATABASE_URL` 注入・goose マイグレーション・systemd 起動）、A-4（本番ドメインの DNS A レコード → Caddyfile 差し替え → Let's Encrypt 自動 TLS。**Lightsail で DNS を管理する場合 `create-domain-entry` は us-east-1 でのみ動作する**点に注意・要 `--region us-east-1`）へ進む。**反復系は SSH を既定**とし、AWS リソース層のみ AWS API MCP に任せる二段構えを維持する。

---

## A-3. バイナリのビルドと配布・DBマイグレーション・systemd起動・日次バックアップ仕込みをAIに依頼

> 🟢 **A-3 確定値（付録E-6/E-8 実証・本文の `<...>` はこの値に読み替える）**
>
> | プレースホルダ | 確定値 |
> |---|---|
> | `<user>`（SSHユーザー） | **`ubuntu`**（`get-instance-access-details` の username・NOPASSWD sudo） |
> | `<静的IP>` | A-1 操作2で確定（例 `57.182.65.19`） |
> | `<DBユーザー>` | `go_iot` |
> | `<本番強パスワード>` | **サーバ内生成値＝`/root/go_iot_db_pass`(600) を `cat` で読む**（人は値を知らない・URLエンコード不要＝記号除去済） |
> | バイナリ | `/opt/go_iot/go_iot_server`・`/opt/go_iot/go_iot_gen-token`（`bin/` 無し） |
> | EnvironmentFile | `/opt/go_iot/go_iot.env`(600・`go_iot` 所有。`/etc/...` ではない) |
> | GOARCH | **`amd64` 固定**（Lightsail に ARM バンドル無し＝`aarch64` 分岐は不要） |
>
> - **SSH（毎回）**: 端末で `aws lightsail get-instance-access-details --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp` を実行（`call_aws` ではなくファイルへ）→ `privateKey`→`~/.ssh/lightsail-goiot.pem`(600)、**`certKey`→`~/.ssh/lightsail-goiot.pem-cert.pub`(600)**（両方必須・無いと `Permission denied (publickey)`）→ `ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@<静的IP>`。**証明書は約13分失効**＝取得直後にまとめて。サーバ操作は `ssh ... 'sudo bash -c "..."'`（個別 sudo より heredoc 事故が少ない・付録E-6）。
> - **goose（STEP5）**: トンネル `ssh -fN -i ~/.ssh/lightsail-goiot.pem -o ExitOnForwardFailure=yes -L 15432:localhost:5432 ubuntu@<静的IP>` → `export GOOSE_DRIVER=postgres; export GOOSE_DBSTRING="postgres://go_iot:$(ssh ... 'sudo cat /root/go_iot_db_pass')@localhost:15432/go_iot?sslmode=disable"` → `go tool goose -dir db/migrations up`（**DSN を引数に書かず env で**＝`ps` に秘密を出さない・期待 `migrated to version: 7`）。
> - **.pgpass（STEP8）**: db フィールドは **`*`**（`localhost:5432:*:go_iot:<pass>`）＝本番 `go_iot` と復元検証 `go_iot_restore_test` の両方に効く。cron 登録は `( crontab -l 2>/dev/null | grep -Fv pg_backup.sh || true; echo "10 3 * * * ..." ) | crontab -`（**`|| true` で初回 set -e 中断回避**・cron は `/etc/localtime`=JST で 03:10 発火）。
> - **gen-token（STEP7）**: 先にブラウザで管理者ユーザー登録（**user_id=1**）→ サーバ上で `gen-token -user=1`（DB は localhost・ability 既定 `["sensor:write"]`・無期限・平文は1回のみ表示）。

このセクションは、A-1/A-2（Lightsail インスタンス作成＋cloud-init による swap/PostgreSQL/Caddy/systemd 雛形の自動構成）が完了し、**素の箱が立ち上がっている**状態を起点に、「ローカルでクロスコンパイルした Go バイナリを `/opt/go_iot` へ配布 → DB マイグレーションを v7 まで適用 → systemd で常駐起動」までを、**人が直接コマンドを打たず、AI と対話しながら AI が実行する**形でまとめる。MCP 化しても**事前コンパイル方針（サーバでビルドしない）・8080/5432 非公開・環境変数は systemd・Caddy 自動TLS**といった不変条件はすべて維持する。

> ⚠️ 前提（このセクション全体に効く）
>
> **(1) 接続・人手作業は §S/付録D-2 を参照**: AWS MCP の接続と最小権限 IAM 発行は §S で完了済み前提（未接続なら先に §S）。AI に任せられない人手作業（アカウント/課金/MFA・ドメイン購入・IAM 初回発行・ESP8266 物理書込・billable/破壊操作の最終承認）は付録D-2。本セクションは「インスタンス起動済み・IAM 発行済み」を前提に書く。MCP の env 名・`mcp-security-policy.json` 構造は接続前に STABLE 版 README で再確認（要確認）。
>
> **(2) `call_aws` は AWS API のみで SSH/scp は扱わない**（§S-7）。本セクション中核の「バイナリ配布・マイグレーション・systemd 操作」は AI がインスタンス内でコマンドを実行する経路が要る。**SSH 実行を既定**とする:
>   - **既定: SSH 経路** — AI がローカル汎用シェル（`Bash` 等）から `scp`/`ssh`。鍵は A-1 で配置済み、または `call_aws` で `aws lightsail get-instance-access-details` の一時鍵を都度取得（鍵を端末に常置しない・有効期限あり。denylist 可否は未確認・ブロック時は常設鍵にフォールバック）。
>   - **任意: SSM Run Command 経路** — ハイブリッドアクティベーション済みなら `aws ssm send-command --document-name AWS-RunShellScript` を単発実行（CloudTrail 監査が乗る）。低メモリ機では SSM Agent 常駐コスト（数十MB級）とのトレードオフ。
>
> **(3) ビルドはローカル（Mac）でのみ**: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`、純 Go（pgx/v5・gin・templ・scs・gorilla/csrf）。**サーバに Go を入れない・サーバでビルドしない**。`sync-css → templ generate → build` の順を厳守（CSS・Scalar/OpenAPI は go:embed 同梱なので build 時に生成済みである必要がある。`internal/config/config.go` の `config.Load()` は `.env` を読まず `os.Getenv` 直読みのため、環境変数は systemd の `EnvironmentFile` で渡す）。

### A-3 のゴールと配布物

| 配布物 | 生成元 | 役割 | サーバ上の配置 |
|---|---|---|---|
| `go_iot_server` | `./cmd/server` をクロスコンパイル | 本番アプリ常駐（全 IF で `:8080` listen＝事実上 `0.0.0.0:8080`、平文HTTP） | `/opt/go_iot/go_iot_server`（`go_iot` 所有・755） |
| `go_iot_gen-token` | `./cmd/gen-token` をクロスコンパイル | デバイス用 Bearer トークンを DB 直書きで発行 | `/opt/go_iot/go_iot_gen-token`（同上） |
| `go_iot_goose`（任意） | goose を Linux バイナリ化 or ローカル SSH トンネル | マイグレーション v7 適用 | 使い捨て（常駐させない） |
| `go_iot.env` | サーバ上で生成（権限 600） | `DATABASE_URL`/`SESSION_SECRET`/`APP_ENV`/`APP_PORT` | `/opt/go_iot/go_iot.env`（`go_iot` 所有・600） |

> `cmd/seed` は TRUNCATE する開発専用ツールのため**本番には配布も実行もしない**。`cmd/sensor-sim`（疎通確認）は任意（A-4 の受け入れ確認用）。

---

### STEP 1. ローカルでクロスコンパイルする（AI 主導・サーバでビルドしない）

> このステップは AWS には触れない。AI がローカル（リポジトリルート）でビルドコマンドを実行する。AWS MCP は不要。`make build` は `go build -o ./tmp/main ./cmd/server`（クロスコンパイル指定なし・ldflags なし）なので**使わず**、下記の個別 build を使う。

**▶ AIへの依頼（プロンプト例）**
> 「go_iot をローカルで Linux 向けにクロスコンパイルして。`go_iot_server` と `go_iot_gen-token` の 2 つ。プランは amd64（Intel/AMD）。`make build` ではなく sync-css → templ generate → 個別 build の順でお願い。ビルド後に `file` で ELF x86-64 になっているか確認して。」

**🤖 AI が行う操作（ローカルシェルで実行・透明性のため明示）**
```bash
# リポジトリルートで実行（例: /Users/c/Desktop/dev/go_iot）
# ① go:embed 対象を生成（Makefile の sync-css と templ generate を利用）
make sync-css                  # mocks/html/style.css -> internal/view/public/css/style.css
go tool templ generate         # *.templ -> *_templ.go（.gitignore 済みのため必須）

# ② サーバ本体をクロスコンパイル（本案件は amd64 確定）
#    -X ... internal/view.Version はキャッシュバスティング用（static.go の var Version="dev" を上書き）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w \
    -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(git rev-parse --short HEAD)" \
  -o go_iot_server ./cmd/server

# ③ トークン発行 CLI も同条件でビルド（GOARCH をサーバ本体と必ず揃える）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o go_iot_gen-token ./cmd/gen-token

# ④ アーキ確認（取り違えると起動時に Exec format error）
file go_iot_server go_iot_gen-token   # 期待: "ELF 64-bit ... x86-64"（本案件 amd64 確定）
```

**🙋 人が確認/承認する点**
- **プラン CPU 種別と GOARCH の一致**: **本案件は amd64 確定＝`GOARCH=amd64`**（選んだ Lightsail バンドルが x86 であることを確認）。**取り違えると起動時に `Exec format error`**（このエラーは本ステップ起因なので STEP6 では起点をここに戻して切り分ける）。
- `go tool templ generate` で `internal/view/*_templ.go` が 1 件以上生成され、`internal/view/public/css/style.css` が存在すること（0 件だと embed が空になり静的資産が欠ける）。

**✅ 完了判定**
- `file go_iot_server` の出力が選定プランのアーキ（x86-64 / aarch64）と一致。
- `go_iot_server`・`go_iot_gen-token` の 2 バイナリがローカルに生成済み。

---

### STEP 2. AI に配布先の状態を確認してもらう（read-only・課金/破壊なし）

> 配布前に、インスタンスが `running` で、ファイアウォールが 22/80/443 のみ（8080/5432 が無い）であることを AI に確認させる。**読み取りのみなので `READ_OPERATIONS_ONLY` 下でも実行可**（書込同意ゲートに引っかからない）。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod の状態を確認して。`running` か、そして開放ポートが 22/80/443 だけで 8080 と 5432 が外部に出ていないかを見せて。あと SSH 接続用の一時鍵情報（username / publicIp）も取って。」

**🤖 AI が行う MCP 操作（裏で呼ばれる AWS CLI を `call_aws` 経由で）**
```bash
# いずれも read-only（Get 系）なので承認ゲート対象外
aws lightsail get-instance-state      --instance-name go-iot-prod --region ap-northeast-1
aws lightsail get-instance-port-states --instance-name go-iot-prod --region ap-northeast-1 \
  --query "portStates[].[fromPort,toPort,protocol,state]" --output table
# SSH 用の一時鍵・username・publicIp・有効期限を取得（端末に鍵を常置しない運用）
aws lightsail get-instance-access-details --instance-name go-iot-prod --region ap-northeast-1
```

**🙋 人が確認/承認する点**
- `get-instance-port-states` の出力に **8080 / 5432 が無い**こと（A-1 のセキュリティ境界が崩れていないか）。出ていたら配布を中断し A-1 の FW を是正。
- `get-instance-access-details` は SSH 秘密鍵を返す。AWS API MCP は deploy/emr サービスの get/ssh 系をデフォルト denylist するが、本コマンドの扱いは公式明記なし（要確認）。**ブロックされた場合は A-1 で配置済みの常設鍵を使う**フォールバックに切り替える。一時鍵には**有効期限**があるため、接続直前に取得する。

**✅ 完了判定**
- インスタンスが `running`、開放ポートは 22/80/443 のみ、SSH 到達手段（一時鍵 or 常設鍵）が確定。

---

### STEP 3. AI にバイナリを scp で配布してもらう（SSH 経路・既定）

> `call_aws` は AWS API のみで SSH/scp を扱わないため、**ここは AI がローカルの汎用シェルから `scp`/`ssh` を実行する**。配置先 `/opt/go_iot`・実行ユーザー `go_iot` は A-2 の cloud-init で作成済みの前提（未作成なら下記 `useradd`/`mkdir` を含めて流す）。

**▶ AIへの依頼（プロンプト例）**
> 「ビルドした `go_iot_server` と `go_iot_gen-token` を go-iot-prod の `/opt/go_iot` に配布して。一旦 SSH ユーザーのホームへ scp して、サーバ側で `/opt/go_iot` へ move、所有者を `go_iot`、実行権 755 に整えて。」

**🤖 AI が行う操作（ローカルシェル → SSH。`ubuntu`=SSH オペレーター、`<静的IP>`=A-1 で確定）**
```bash
# --- ローカルで実行: バイナリ 2 つを SSH ユーザーのホームへ ---
scp go_iot_server go_iot_gen-token ubuntu@<静的IP>:/home/ubuntu/

# --- サーバ上で実行（ssh 経由・cloud-init 未作成時のみ useradd/mkdir も含める）---
ssh ubuntu@<静的IP> 'set -e
  id go_iot >/dev/null 2>&1 || sudo useradd --system --no-create-home --shell /usr/sbin/nologin go_iot
  sudo mkdir -p /opt/go_iot && sudo chown go_iot:go_iot /opt/go_iot
  sudo mv /home/'"ubuntu"'/go_iot_server /home/'"ubuntu"'/go_iot_gen-token /opt/go_iot/
  sudo chown go_iot:go_iot /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token
  sudo chmod 755 /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token
  ls -l /opt/go_iot/'
```

> **任意: SSM Run Command 経路**（A-1/A-2 で managed node 化済みの場合）。バイナリ転送自体は SSM 単発実行と相性が悪いため、`scp` 部分は SSH のまま、サーバ側の配置コマンドのみ SSM で実行する選択肢。**SSM RunShellScript は root として実行されるため `sudo` は不要**:
> ```bash
> # AI が call_aws 経由で（書込操作 = REQUIRE_MUTATION_CONSENT の同意対象）
> aws ssm send-command --region ap-northeast-1 \
>   --document-name AWS-RunShellScript \
>   --instance-ids <mi-xxxx> \
>   --parameters 'commands=[
>     "mv /home/ubuntu/go_iot_server /home/ubuntu/go_iot_gen-token /opt/go_iot/",
>     "chown go_iot:go_iot /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token",
>     "chmod 755 /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token"]'
> ```

**🙋 人が確認/承認する点**
- scp/ssh の**接続先 `<静的IP>` と SSH 鍵**が go-iot-prod のものか（誤配布防止）。**SSH/scp 経路は AWS の承認ゲート（CloudTrail/REQUIRE_MUTATION_CONSENT）が効かない**ため、ここは人の目視確認が唯一の防御線。
- SSM 経路を使う場合、`send-command` は**書込操作**なので `REQUIRE_MUTATION_CONSENT` の同意ダイアログで承認（または `mcp-security-policy.json` の `elicitList` 対象）。

**✅ 完了判定**
- `ls -l /opt/go_iot/` に `go_iot_server`・`go_iot_gen-token` が **`go_iot` 所有・755** で並ぶ。

---

### STEP 4. AI に本番 EnvironmentFile を生成・注入してもらう（秘密の安全生成）

> `config.Load()`（`internal/config/config.go`）は `os.Getenv` 直読みで `.env` を読まない。環境変数は **systemd の `EnvironmentFile`（権限 600）** で渡す。`APP_ENV=production` では `SESSION_SECRET` が **32 文字未満だと起動エラー**（`config.go` L41-43 `SESSION_SECRET must be at least 32 chars in production`）。秘密はサーバ上で生成し、ローカルや MCP 設定にベタ書きしない。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod の `/opt/go_iot/go_iot.env` を作って。`SESSION_SECRET` は 32 文字以上をサーバ上で安全生成。`DATABASE_URL` は localhost・sslmode=disable・A-2 で設定した本番強パスワード。`APP_ENV=production`、`APP_PORT=8080`。ファイルは `go_iot` 所有・600 に。値は画面に丸ごと出さず、生成できたことだけ報告して。」

**🤖 AI が行う操作（ssh 経由でサーバ上で生成。秘密はサーバ内で完結。`stat -c` は GNU stat 前提＝Ubuntu サーバ上なので可）**
```bash
ssh ubuntu@<静的IP> 'set -e
  # SESSION_SECRET をサーバ上で安全生成（base64 で約 64 文字となり 32 文字条件を確実に満たす）
  SECRET=$(openssl rand -base64 48)
  # EnvironmentFile を root 権限で生成（DBパスワードは A-2 の本番強パスワードに置換）
  sudo tee /opt/go_iot/go_iot.env >/dev/null <<EOF
APP_ENV=production
APP_PORT=8080
DATABASE_URL=postgres://go_iot:<本番強パスワード>@localhost:5432/go_iot?sslmode=disable
SESSION_SECRET=${SECRET}
EOF
  sudo chown go_iot:go_iot /opt/go_iot/go_iot.env
  sudo chmod 600 /opt/go_iot/go_iot.env
  echo "go_iot.env created: $(sudo stat -c "%U:%G %a" /opt/go_iot/go_iot.env)"'
# → 出力は "go_iot:go_iot 600" のみ（秘密値は表示しない）
```

> ⚠️ **DB パスワードの扱い**: `DATABASE_URL` のパスワードに `@ : / ?` 等が含まれる場合は URL エンコードが必要（A-2 で生成する本番パスワードはこれら記号を避けるか、エンコード済みの値を使う）。`SESSION_SECRET` を後で再生成すると**全セッションが無効化**される点に注意。

**🙋 人が確認/承認する点**
- AI が `go_iot.env` の中身（特に `SESSION_SECRET`・DB パスワード）を**ログ/チャットに丸出ししていない**こと。報告は「生成済み・権限 600・所有者 go_iot」までに留める。
- `DATABASE_URL` の host が `localhost`、`sslmode=disable`（5432 非公開が前提）、ユーザー/パスワードが**開発用ではなく本番強パスワード**であること。

**✅ 完了判定**
- `stat` の出力が `go_iot:go_iot 600`。`/opt/go_iot/go_iot.env` が存在し、必須 4 変数（`APP_ENV`/`APP_PORT`/`DATABASE_URL`/`SESSION_SECRET`）が入っている。

---

### STEP 5. AI に DB マイグレーションを v7 まで適用してもらう（seed は本番禁止）

> マイグレーションは goose（`db/migrations/`、現在 **version 7**＝`00001`〜`00007`）。**サーバに Go も goose も入れない**方針なので、次の二択。本書は**(a) ローカル SSH トンネル経由を既定**とする（Go・goose・マイグレーションファイルがローカルに揃い最も確実・サーバへ何も追加しない）。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod の PostgreSQL に goose マイグレーションを適用して。5432 は外部非公開だから SSH トンネル（ローカル 15432 → サーバ 5432）越しで。まず `status` で現状を見せて、未適用があれば `up` で v7 まで上げて。seed は本番では絶対に流さないで。」

**🤖 AI が行う操作（(a) ローカル SSH トンネル経由・既定）**
```bash
# --- ローカルで実行: 5432 を SSH トンネルで手元 15432 に転送（手元 5432 と衝突回避）---
ssh -fN -L 15432:localhost:5432 ubuntu@<静的IP>
# 適用前に現状確認 → 未適用があれば up
go tool goose -dir db/migrations \
  postgres "postgres://go_iot:<本番強パスワード>@localhost:15432/go_iot?sslmode=disable" status
go tool goose -dir db/migrations \
  postgres "postgres://go_iot:<本番強パスワード>@localhost:15432/go_iot?sslmode=disable" up
# --- 完了後: トンネルを必ず停止（15432 を占有し続けると次回 bind エラー）---
lsof -ti tcp:15432 | xargs kill   # ピンポイントで該当 PID のみ停止（pkill -f はパターン誤爆に注意）
```

> **任意: (b) goose Linux バイナリをサーバへ置いて実行**（SSH トンネルを張れない/張りたくない場合）。`go_iot_goose` を STEP 1 と同条件でクロスコンパイルして scp し、サーバ上で `go_iot.env` を source して実行。実行後は常駐させず削除してよい。
> ```bash
> # サーバ上: source して up（DATABASE_URL は localhost:5432）
> sudo -u go_iot bash -c 'set -a; . /opt/go_iot/go_iot.env; set +a; \
>   /opt/go_iot/go_iot_goose -dir /opt/go_iot/migrations postgres "$DATABASE_URL" up'
> ```

**🙋 人が確認/承認する点**
- `goose ... up` は**スキーマ変更（書込）**。トンネル経由（ローカル実行）なら AWS API を経由しないため AWS の承認ゲートは効かない → **「seed を流さない」「対象 DB が本番か」を人が目視確認**する。SSM 経由で流す場合のみ同意ゲート対象。
- `make seed` / `cmd/seed` は既存アプリデータを TRUNCATE するため**本番 DB に絶対実行しない**。本番のデバイス/トークンは Web UI 登録＋ `gen-token` で正規に発番。

**✅ 完了判定**
- `goose ... status` が **v1〜v7 すべて `Applied`**（Pending が無い）。トンネルを使った場合は**停止済み**（15432 が解放）。

---

### STEP 6. AI に systemd unit を配置・enable・start してもらう

> `/etc/systemd/system/go_iot.service` を AI に配置させる。**`Restart=always`・`EnvironmentFile`（STEP 4 の 600 ファイル）・`After=postgresql.service`** は不変条件として必ず含める。アプリは `:8080`（全 IF）を listen するため、8080/5432 の遮断は**ファイアウォール（A-1 の Lightsail FW＋OS ufw）が唯一の防御**で必須（A-2 で設定済みの前提）。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod に systemd unit `go_iot.service` を配置して。実行ユーザーは `go_iot`、`EnvironmentFile=/opt/go_iot/go_iot.env`、`Restart=always`、`After=postgresql.service`。配置したら daemon-reload → enable → start して、`status` と直近の `journalctl` を見せて。」

**🤖 AI が行う操作（ssh 経由でサーバ上に配置・起動）**
```bash
ssh ubuntu@<静的IP> 'set -e
  sudo tee /etc/systemd/system/go_iot.service >/dev/null <<EOF
[Unit]
Description=go_iot agricultural IoT server
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=go_iot
Group=go_iot
WorkingDirectory=/opt/go_iot
EnvironmentFile=/opt/go_iot/go_iot.env
ExecStart=/opt/go_iot/go_iot_server
Restart=always
RestartSec=5
# 低メモリプラン向けの保険（アプリ単体上限。同居 PostgreSQL は別枠＝swap で備える）
# MemoryMax=400M   # 512MB プランなら 360〜410M 目安・要実測
# セキュリティ強化（任意・推奨）
NoNewPrivileges=true
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
  sudo systemctl daemon-reload
  sudo systemctl enable --now go_iot          # enable + start を一括
  sleep 2
  sudo systemctl status go_iot --no-pager     # active (running) を確認
  sudo journalctl -u go_iot -n 30 --no-pager  # 起動失敗時はここに env 不足等が出る'
```

> 注記:
> - `ProtectSystem=full` は `/usr` `/boot` `/etc` を読取専用にする。アプリは `go_iot.env` の読込のみで書込不要なため問題ないが、将来ログやファイルを `/opt` 配下に書く実装にした場合は `ReadWritePaths=/opt/go_iot` の追加が必要。
> - PostgreSQL を（方針外だが）Docker で動かす場合のみ `After=postgresql.service` を `After=docker.service` に変える。低メモリ前提では native apt の `postgresql.service` を推奨。
> - `MemoryMax` を有効化する場合はアプリ単体の上限であり、Postgres 起因の OOM は swap で備える。数値は要実測（要確認）。

**🙋 人が確認/承認する点**
- unit に **`Restart=always` / `EnvironmentFile=/opt/go_iot/go_iot.env` / `After=postgresql.service`** が入っていること（不変条件）。
- 起動失敗ログの典型: `required env vars missing`（STEP 4 の env 不足）/ `SESSION_SECRET must be at least 32 chars in production`（32 文字未満）/ `Exec format error`（STEP 1 のアーキ取り違え）/ `connection refused`（PostgreSQL 未起動 = `After=postgresql.service` か A-2 の DB 構築を確認）。

**✅ 完了判定**
- `systemctl status go_iot` が **`active (running)`**、`enabled`（OS 起動時に自動起動）。
- `journalctl` にパニック/再起動ループが無い。

---

### STEP 7. 起動確認とトークン発行（/health 200 ＋ gen-token）

> 最後に「アプリが DB 込みで生きているか」と「デバイス用トークンを発行できるか」を AI に確認させる。8080 は外部非公開なので `/health` は**サーバ内から** `localhost:8080` で叩く（外部 HTTPS 経由の本番疎通は A-4 の Caddy 経路で実施）。`/health` は DB ping 込みで **200 `{"status":"ok"}` / 503 `{"status":"db_unreachable"}`** を返す（`cmd/server/main.go`）。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod でアプリのヘルスチェックして。サーバ内から `localhost:8080/health` で 200 が返るか。返ったら、user_id=<対象> 向けにデバイス用トークンを発行して（`gen-token`）。平文トークンは発行時の 1 回だけ表示されるから控えられるように見せて。」

**🤖 AI が行う操作（ssh 経由）**
```bash
ssh ubuntu@<静的IP> '
  # ① DB ping 込みヘルスチェック（200=正常 / 503=db_unreachable）
  curl -s -o /dev/null -w "health=%{http_code}\n" http://localhost:8080/health
  # ② トークン発行（gen-token は config.Load() を呼ぶので env を source してから実行）
  #    go_iot.env は 600・go_iot 所有なので go_iot ユーザーで読み込んで実行
  sudo -u go_iot bash -c "set -a; . /opt/go_iot/go_iot.env; set +a; \
    /opt/go_iot/go_iot_gen-token -user=<user_id> -name=\"ハウスA温湿度計\""'
# → 表示された平文トークンを控える（再表示不可）。B（ESP8266 書込）の config.h の API_BEARER に使う。
```

**🙋 人が確認/承認する点**
- `/health` が **200**（503 なら DB 未到達 = STEP 4 の `DATABASE_URL` か A-2 の PostgreSQL 起動を確認）。
- `gen-token` の `-user` は **device_id ではなくユーザーの user_id（整数）**。device_id は Web UI のデバイス登録で発番（発行元が分かれている点に注意）。`-name` はトークン名（通常デバイス名と揃える）。
- 平文トークンは**発行時 1 回のみ表示**（`gen-token` は平文を控えるよう警告も出す）。AI のチャットログに残る場合は、控えた後にログ取り扱いに注意（秘密の漏えい防止）。

**✅ 完了判定**
- `health=200`。`gen-token` が平文トークンを 1 回出力し、それを控えた。
- 以降は B（実機 ESP8266 への `API_ENDPOINT`/`API_BEARER`/`DEVICE_ID` 書込）へ進める。本番疎通の最終確認（HTTPS 経由・`sensor-sim` 単発 201）は Caddy/DNS 完了後（A-4）に行う。

---

### STEP 8. AI に日次バックアップ（pg_dump cron）を仕込んでもらう

> **なぜ A-3 でやるか**: コンパイル済みバイナリの差し替え（STEP 1〜7 の反復）では DB は無傷だが、**マイグレーション事故・インスタンス障害・誤操作**に備えて論理バックアップを最初から自動化しておく。**DB は PostgreSQL の別サービス（`/var/lib/postgresql`）に永続**しバイナリには同梱されない（＝アプリ差し替えでは消えない）が、それは「バックアップ不要」を意味しない。ここで日次 `pg_dump`（論理バックアップ）を仕込み、Lightsail スナップショット（物理・課金あり）と**二層**で守る運用は C-2-3 に接続する。**C-2-3 はこの STEP 8 で配置した `pg_backup.sh` が存在することを前提に書かれている**ため、A-3 で必ず実施する。
>
> **実行主体**: cron は **SSH オペレーター `ubuntu`** の crontab で回す（C-2-3 の確認コマンド `crontab -l` が sudo 無しで `ubuntu` の crontab を見る前提と一致）。`pg_dump` のパスワードは `ubuntu` の `~/.pgpass`（600）で解決し、コマンドラインや crontab に平文を置かない。

**▶ AIへの依頼（プロンプト例）**
> 「go-iot-prod に DB の日次バックアップを仕込んで。`pg_dump` を `/var/backups/go_iot` に gzip で吐く `/usr/local/bin/pg_backup.sh` を置いて、毎日 03:10 に走る cron を `ubuntu` に登録。パスワードは `~/.pgpass`（600）で解決して平文を crontab に書かないで。14 日より古いダンプは自動削除。仕込んだら手動で 1 本取って、復元検証用 DB に流して戻せることまで確認して。」

**🤖 AI が行う操作（ssh 経由。`go_iot`=`go_iot`、`<本番強パスワード>`=STEP 4 の `DATABASE_URL` と同一値）**
```bash
ssh ubuntu@<静的IP> 'set -e
  # ① 保存先とログを ubuntu 書込可で用意（pg_dump は ubuntu として走る）
  sudo mkdir -p /var/backups/go_iot && sudo chown '"ubuntu"':'"ubuntu"' /var/backups/go_iot
  sudo touch /var/log/pg_backup.log && sudo chown '"ubuntu"':'"ubuntu"' /var/log/pg_backup.log

  # ② パスワードは ~/.pgpass(600) で解決（host:port:db:user:password）。crontab に平文を置かない
  umask 077
  printf "localhost:5432:go_iot:%s:%s\n" "go_iot" "<本番強パスワード>" > ~/.pgpass
  chmod 600 ~/.pgpass

  # ③ バックアップスクリプトを配置（C-2-3 で再掲される本文と完全一致・root:root 755）
  sudo tee /usr/local/bin/pg_backup.sh >/dev/null <<"EOF"
#!/usr/bin/env bash
set -euo pipefail
STAMP="$(date +%Y%m%d_%H%M%S)"
OUT="/var/backups/go_iot/go_iot_${STAMP}.sql.gz"
pg_dump '"'"'postgres://go_iot@localhost:5432/go_iot'"'"' | gzip > "${OUT}"   # PW は ~/.pgpass で解決
find /var/backups/go_iot -name '"'"'go_iot_*.sql.gz'"'"' -mtime +14 -delete         # 低ディスク対策・14日保持
EOF
  sudo chmod 755 /usr/local/bin/pg_backup.sh

  # ④ cron を ubuntu に冪等登録（毎日 03:10。重複登録を避けて差し替え）
  ( crontab -l 2>/dev/null | grep -Fv "/usr/local/bin/pg_backup.sh"; \
    echo "10 3 * * * /usr/local/bin/pg_backup.sh >> /var/log/pg_backup.log 2>&1" ) | crontab -
  crontab -l | grep pg_backup

  # ⑤ 手動で 1 本取って生成を確認
  /usr/local/bin/pg_backup.sh
  ls -lh /var/backups/go_iot | tail -n 1'
```

> ⚠️ **`go_iot`/`<本番強パスワード>` は STEP 4 の `DATABASE_URL` と必ず一致させる**（不一致だと `pg_dump: error: ... password authentication failed`）。パスワードはチャットに丸出しせず、`~/.pgpass` は 600・`ubuntu` 所有を厳守。`pg_dump` は同梱の PostgreSQL クライアント（`postgresql-${PG_VER}`）に含まれるため追加導入は不要。

**🙋 人が確認/承認する点**
- `~/.pgpass` が **600・`ubuntu` 所有**で、パスワードが crontab・スクリプト本文・チャットログに**平文で出ていない**こと（出ていたらローテーション）。
- cron 行が `10 3 * * * /usr/local/bin/pg_backup.sh >> /var/log/pg_backup.log 2>&1` で**1 本だけ**（重複登録なし）。
- `pg_backup.sh` の本文が **C-2-3 の再掲と完全一致**（保持期間 `-mtime +14`・出力先 `/var/backups/go_iot`）であること。**SSH 越しの操作は AWS 承認ゲート外**なので接続先 `<静的IP>` の目視確認が防御線（STEP 3 と同様）。

**✅ 完了判定**
- `crontab -l | grep pg_backup` が 1 行返り、`/var/backups/go_iot/go_iot_*.sql.gz` が直近タイムスタンプで 1 本生成済み。
- **復元検証を最低 1 回**完了（下記。本番 DB を上書きしないよう検証用 DB へ流す）。以降の定常運用・手動取得・Lightsail スナップショットとの二層化は C-2-3 に引き継ぐ。

```bash
# 復元検証（A-3 で最低 1 回。本番 go_iot は触らず検証用 DB へ流して戻せることだけ確認）。AI が SSH で実行
ssh ubuntu@<静的IP> "createdb -h localhost go_iot_restore_test 2>/dev/null || true; \
  gunzip -c \$(ls -t /var/backups/go_iot/go_iot_*.sql.gz | head -1) \
  | psql 'postgres://go_iot@localhost:5432/go_iot_restore_test'; \
  psql 'postgres://go_iot@localhost:5432/go_iot_restore_test' \
    -tAc 'SELECT count(*) FROM sensor_readings'; \
  dropdb -h localhost go_iot_restore_test"   # 検証後は破棄（低ディスク対策）
```

---

### A-3 のセキュリティ・運用ガードレール（MCP化に伴う追加分）

- **AWS 書込操作は多層ゲート**: 調査フェーズ（STEP 2 の Get 系）は `READ_OPERATIONS_ONLY=true` で起動 → 配布フェーズで `REQUIRE_MUTATION_CONSENT=true`（`ssm send-command` 等の書込前に人が同意。クライアントの elicitation 対応が前提）。`~/.aws/aws-api-mcp/mcp-security-policy.json` の `denyList` に `aws lightsail delete-instance` 等の破壊系を**完全一致で列挙**（ワイルドカード非対応）。MCP クライアントは `autoApprove: []`（都度承認）。IAM は Lightsail 操作に限定の最小権限（`DeleteInstance`/`ReleaseStaticIp` 等の破壊系はそもそも付与しないか `elicitList` で二重化）。
- **SSH/scp は AWS API の外**: バイナリ配布・systemd 操作・マイグレーションは `call_aws` ではなくローカルシェルの `ssh`/`scp` 実行。**AWS の承認ゲートも CloudTrail 監査も効かない**ため、**接続先 IP・対象 DB・seed 不実行は人が目視確認**する（AWS MCP 任せにしない）。
- **秘密はサーバ上で生成・600・チャットに出さない**: `SESSION_SECRET` は `openssl rand -base64 48` をサーバ上で生成、`go_iot.env` は `go_iot` 所有・600。AI に値を復唱させない。CloudTrail で AWS 操作（AI が打った分も IAM プリンシパル単位）を監査。userData に秘密を焼かない不変条件は §S-0（正本）。
- **不変条件の維持**: 事前コンパイル（サーバでビルドしない）/ 8080・5432 非公開 / 環境変数は systemd EnvironmentFile / Caddy 自動TLS（前段・別セクション）は MCP 化後も一切緩めない。AI が「楽だから」とサーバ上で `go build` する・FW に 8080 を開ける・env を平文で晒す等を提案した場合は**拒否**する。

> 不確実な外部事実の再掲（接続/契約前に要確認）: AWS MCP のツール名・環境変数名・`mcp-security-policy.json` 構造（2026-06 時点）、`get-instance-access-details` の denylist 可否、SSM ハイブリッドアクティベーションの Agent インストール手順、Lightsail のプラン/料金/bundle-id、Claude Code の elicitation 対応状況、Lightsail DNS API の us-east-1 限定（A-4 申し送り）、マネージド AWS MCP Server（Agent Toolkit for AWS）への移行時期。いずれも AWS Knowledge MCP（`https://knowledge-mcp.global.api.aws`・認証不要・read-only）の `search_documentation`/`read_documentation` で対話中に裏取りできる。

---

### A-3-R. 2回目以降のリデプロイ（コード更新を本番へ反映）— 初心者向け・再現性重視

> A-1〜A-4 で初回デプロイ済みの本番（`go-iot-prod`）へ、**コードを直して本番へ反映し直す**ときの手順。
> **結論: `bash deploy/redeploy.sh` 一発でよい**（下記の全手順を自動化済み・2026-06-25 実機検証）。
> 手動でやる場合や、つまづいた時の切り分けのために、中身と既知の罠も明記する。

#### 🟢 いちばん簡単（推奨）: ワンコマンド

```bash
# リポジトリルートで実行。反映したい状態を git で commit 済みにしておく(Version 表示が commit に紐づくため)
bash deploy/redeploy.sh
```

これだけで **現在の接続元IP(egress)自動検出 → FW一時開放 → amd64ビルド → 配布 → 旧binary退避 → 入替 → `systemctl restart go_iot` → sha照合/health検証 → FW復元** まで実行される。失敗時は**旧binaryへ自動ロールバック**し、途中で止まっても **trap で FW は必ず管理元IPへ戻る**。完了後に外部 `https://<静的IP>.sslip.io/health` が 200 で、ログインページの `?v=<commit>` が新しくなっていれば反映成功。

#### ⚠️ 初心者がつまづく罠（重要・この3つで9割ハマる）

**罠①: SSH(22) が「timed out」で繋がらない＝接続元IPの不一致**
- 症状: `ssh ... ubuntu@<静的IP>` が `Operation timed out`（`Connection refused` では**ない**）。`443` は通るのに `22` だけ無反応。
- 原因: Lightsail FW の 22 は**管理元IP限定**だが、**今の接続元の実IP(egress)が許可リストに無い**。
  - **VPN を使っていると egress が接続ごとに変わる**（観測例: `104.234.140.0/24` の中で `.13/.25/.29`…）。**単一 `/32` 開放では当たらない**。
  - **管理元IP自体も動的**: 自宅/拠点のグローバルIPは停電・ISP再割当で変わる（記録済みの `123.226.213.236/32` はいずれ古くなる）。
- 対処: **その場で今のegressを検出してから開放する**（`redeploy.sh` は自動でやる）。**固定IPなら `/32`、VPN等で毎回変わるなら `/24` を一時開放**する。`443` が通って `22` だけ落ちるのは「ホストは生きていてFWで弾かれている」サインなので、サーバ障害と勘違いしないこと。

**罠②: `call_aws`(MCP) での FW 変更が「User rejected」で必ず失敗する**
- 症状: `aws lightsail put-instance-public-ports`／`open-instance-public-ports` を **`call_aws` 経由**で叩くと、こちらが承認したつもりでも `User rejected the execution of the command` で弾かれる。
- 原因: これらは `mcp-security-policy.json` の `elicitList`（同意必須）にあり、**Claude Code/Cursor の elicitation が「承認」として返らない**（§S-5-2／§③の `create-instances` 罠と同根）。
- 対処: **FW変更だけはローカルCLI（`--profile go-iot-mcp`）で実行する**（`call_aws` を使わない）。読み取り（`get-instance-port-states` 等）は `call_aws` で可。**「読み取り=call_aws / 書き込み(FW変更)=ローカルCLI」**と覚える。`redeploy.sh` も内部でローカルCLIを使っている。

**罠③: `open-` は追記・`put-` は全置換（混同すると事故る）**
- `open-instance-public-ports` は既存cidrに**追記**（古い許可が残る）。`put-instance-public-ports` は渡した配列で**全置換**（**含めなかったポートは閉じる**）。
- **復元は必ず `put-`** で「22→管理元`/32`・80→`0.0.0.0/0`・443→`0.0.0.0/0`」の**3つだけ**にする。**`put-` で 8080/5432 を絶対に含めない**（含めると即公開＝最悪の事故）。復元後に `get-instance-port-states` で 8080/5432 が無いことを必ず確認。

#### 手動でやる場合（`redeploy.sh` を使わない／切り分け用）

```bash
# ① いまの接続元(egress)を確認。VPNなら数回叩いて変動するか見る(変動するなら /24 を使う)
curl -s https://checkip.amazonaws.com           # 例: 104.234.140.29 → /24 採用なら 104.234.140.0/24

# ② FW一時開放(★ローカルCLIで。call_awsではない★)。put-で 22/80/443 だけに全置換(8080/5432は入れない)
aws lightsail put-instance-public-ports --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp \
  --port-infos '[{"fromPort":22,"toPort":22,"protocol":"tcp","cidrs":["<現egress>/24","123.226.213.236/32"]},{"fromPort":80,"toPort":80,"protocol":"tcp","cidrs":["0.0.0.0/0"]},{"fromPort":443,"toPort":443,"protocol":"tcp","cidrs":["0.0.0.0/0"]}]'
aws lightsail get-instance-port-states --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp \
  --query "portStates[].[fromPort,join(',',cidrs)]" --output json   # 8080/5432 が無いこと

# ③ ローカルでビルド(A-3 STEP1と同じ。サーバではビルドしない)
make sync-css && go tool templ generate
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(git rev-parse --short HEAD)" \
  -o go_iot_server ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o go_iot_gen-token ./cmd/gen-token

# ④ 一時SSH鍵を取得(certKey も必須・両600・約13分で失効＝接続直前に取る)
aws lightsail get-instance-access-details --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp --output json \
  | python3 -c "import sys,json,os;d=json.load(sys.stdin)['accessDetails'];h=os.path.expanduser('~/.ssh');open(h+'/lightsail-goiot.pem','w').write(d['privateKey']);open(h+'/lightsail-goiot.pem-cert.pub','w').write(d['certKey'])"
chmod 600 ~/.ssh/lightsail-goiot.pem ~/.ssh/lightsail-goiot.pem-cert.pub

# ⑤ 配布→旧binary退避→入替→再起動(サーバ上)
scp -i ~/.ssh/lightsail-goiot.pem go_iot_server go_iot_gen-token ubuntu@57.182.65.19:/home/ubuntu/
ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@57.182.65.19 'set -e; TS=$(date +%Y%m%d-%H%M%S)
  sudo cp -p /opt/go_iot/go_iot_server /opt/go_iot/go_iot_server.bak-$TS
  sudo cp -p /opt/go_iot/go_iot_gen-token /opt/go_iot/go_iot_gen-token.bak-$TS
  sudo mv /home/ubuntu/go_iot_server /home/ubuntu/go_iot_gen-token /opt/go_iot/
  sudo chown go_iot:go_iot /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token
  sudo chmod 755 /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token
  sudo systemctl restart go_iot; sleep 3
  systemctl is-active go_iot; curl -s -o /dev/null -w "health=%{http_code}\n" http://localhost:8080/health'

# ⑥ 外部から確認
curl -sS https://57.182.65.19.sslip.io/health         # {"status":"ok"} / 200
curl -sS https://57.182.65.19.sslip.io/login | grep -oE '\?v=[a-z0-9]+' | head -1   # ?v=<新commit>

# ⑦ ★必ず★ FW復元(put-で 22→管理元のみ。8080/5432が無いこと)
aws lightsail put-instance-public-ports --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp \
  --port-infos '[{"fromPort":22,"toPort":22,"protocol":"tcp","cidrs":["123.226.213.236/32"]},{"fromPort":80,"toPort":80,"protocol":"tcp","cidrs":["0.0.0.0/0"]},{"fromPort":443,"toPort":443,"protocol":"tcp","cidrs":["0.0.0.0/0"]}]'

# ローカルのビルド成果物は掃除(コミット対象外)
rm -f go_iot_server go_iot_gen-token
```

#### 知っておくと迷わない点

- **コードだけの変更なら DB マイグレーションは不要**: スキーマ非変更（例: グラフ期間「3日間」追加）は goose 版数据え置き（`v7` のまま）。A-3 STEP5 は skip してよい。マイグレーション（`db/migrations/` 変更）を伴うときだけ STEP5 を実施。
- **配信確認は `?v=<commit>`**: ログインページの `style.css?v=103bc48` のように `internal/view.Version`（ビルド時 ldflags で commit を埋め込む）でキャッシュバストされる。ここが新 commit になっていれば「新ビルドが配信中」。
- **Go ビルドはバイト再現しない**: 同じ commit でもビルドの度に sha が変わる（埋め込み build ID のため）。だから検証は「**ローカルsha == リモートsha** を**同一デプロイ実行内**で照合」する（`redeploy.sh` は自動）。前回ビルドとの sha 比較は無意味。
- **ロールバック**: サーバの `/opt/go_iot/go_iot_server.bak-<TS>` が旧binary。`sudo cp` で戻して `sudo systemctl restart go_iot`（`redeploy.sh` は health 異常時に自動でこれを行う）。
- **恒久策（FW開閉が煩わしい場合）**: SSM Session Manager で **22 を恒久クローズ**できる（`deploy/ssm-setup-plan.md`）。ただし Lightsail=非EC2 で hybrid node の Session Manager は **advanced-instances tier（有料・~$5/月）** が要る。少額運用では `redeploy.sh`（オンデマンド開放）継続が費用効率最良。

---

## A-4. デプロイ後の受け入れ確認をAIに依頼

> 🟢 **A-4 確定値（付録E-7/F 実証・本文の `<host>` 等はこの値に読み替える）**
>
> - **本番FQDN `<host>` = `<静的IP>.sslip.io`**（例 `https://57.182.65.19.sslip.io`）。ドメイン購入不要。ブラウザ・curl・config.h の `API_ENDPOINT` すべてこのURL。
> - **A-4 の最初の作業＝Caddyfile を本番化**（本文はここから始まる）: SSH で `/etc/caddy/Caddyfile` を「`{ email <ADMIN_EMAIL> }` ＋ `<静的IP>.sslip.io { reverse_proxy localhost:8080 }`」に差し替え → `caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy`。証明書は **TLS-ALPN-01(443) で自動取得**（sslip.io は即解決＝伝播待ち無し・付録E-7）。旧 `:80` 雛形は `.bak-placeholder` へ退避。
> - **SSH/env/バイナリ**: `ubuntu@<静的IP>`＋certKey（A-3 確定値ボックス参照）／env=`/opt/go_iot/go_iot.env`／`gen-token`=`/opt/go_iot/go_iot_gen-token`（`/etc/...`・`bin/` では**ない**）。`gen-token` は `SESSION_SECRET=dummy` でも可（`config.Load` が要求するだけ）。
> - **ユーザー登録**: user_id=1。**デバイス登録は `mac_address` 必須**（形式 `AA:BB:CC:DD:EE:FF`・大文字正規化・active 一意。実機なしなら未使用ダミー `02:00:00:00:00:01` 等）。
> - **curl ログイン検証は Referer 必須**（`-e https://<静的IP>.sslip.io/login`・gorilla/csrf が HTTPS で同一オリジン Referer を厳格チェック・付録E-8）。ブラウザ/chrome-devtools MCP なら自動付与。
> - **後始末**（テスト送信削除）: SSH→`sudo -u postgres psql go_iot` で `DELETE FROM sensor_readings WHERE device_id=<id> ...`（件数を目視確認・`cmd/seed` は TRUNCATE で本番厳禁・付録F-12）。

A-1〜A-3 が終わり、本番ドメインの Caddy 自動 TLS 化（DNS 伝播後に :80 雛形 → 本番ドメイン+443 へ差し替え）まで済んだ状態を起点に、**受け入れ確認**を人がコマンドを直接打たずに **AI との対話**で進めるセクション。確認シナリオ（到達性 → device_id → Bearer トークン → 受信 → 画面反映 → アラート発火 → グラフ反映）の**実行と結果解釈を AI（MCP 経由）に任せる**。ここが通れば「実機（ESP8266）なしで本番受信パイプラインが通る＝B へ進める」と判定する。

> **このセクションの大前提（§S・§A で完了済み）**: AWS MCP の接続・最小権限 IAM（§S）、不変条件（§S-0＝低メモリ/サーバでビルドしない/8080・5432 非公開/Caddy 自動TLS/環境変数は systemd・秘密は権限600・userData に秘密を焼かない）、安全策の多層（§S-5＝`READ_OPERATIONS_ONLY`/`REQUIRE_MUTATION_CONSENT`/denyList/elicitList/`autoApprove:[]`/CloudTrail）はすべて済んでいる前提。本 A-4 では再掲せず、A-4 固有の点だけ確認する:
> - **A-4 は基本 read-only 寄り**（ログ/メトリクス/状態確認が中心）で課金・破壊操作はほぼ無いが、**SSM Run Command 経由のサーバ内コマンド実行は mutation 扱い**になりうるため内容を人が確認してから承認する。
> - **AWS MCP の役割分担**: AWS リソース層（ポート状態・Lightsail メトリクス・CloudTrail）は `call_aws`。`cmd/sensor-sim` の HTTP POST と `/health` の curl は **AWS API ではないためローカル Mac から AI が直接実行**。トークン発行（`cmd/gen-token`）は DB 直書きのため **サーバ上**で SSH もしくは SSM Run Command 経由。
> - **秘密の前段点検**: 本 A-4 は新規に秘密を焼かないが、A-1〜A-3 の userData に `SESSION_SECRET`/DB パスワードを平文で焼いていないかを必ず点検する（§S-0 の不変条件）。

> **このセクションで使う識別子の例**（実値は A-1〜A-3 の実配置に合わせる）
> - `<静的IP>.sslip.io` = 本番ドメイン（例 `iot.example.com`・実値要確認）
> - `<user_id>` = Web UI 登録で得るユーザー ID（gen-token に必要）
> - `<device_id>` = Web UI のデバイス登録で発番される ID
> - `<平文トークン>` = サーバ上 `gen-token` の出力（発行時のみ表示・再表示不可）
> - `<INSTANCE>` = Lightsail インスタンス名（例 `go-iot-prod`）、`<REGION>` = `ap-northeast-1`
> - パス/ユニット名（`/opt/go_iot/go_iot.env`・`/opt/go_iot/go_iot_gen-token`・`go_iot.service`）は A-1〜A-3 で確定する値。実配置に合わせて読み替えること（`caddy.service` は Caddy 公式 apt パッケージの標準ユニット名）。

---

### 受け入れ確認の全体像（AI主導の流れ）

```
①/health 200 (AIが curl 解釈)         ── HTTP。AIがローカル(外部回線)から実行
   └→ 同時に 8080/5432 が外部閉(境界OK)を AIが確認
②Web UI 登録→ログイン→dashboard       ── ブラウザ操作(CSRFのため人 or chrome-devtools MCP)
③デバイス登録で device_id 取得          ── 同上
④本番側で gen-token しBearer発行        ── DB直書き=サーバ上。SSH/SSM Run Command を AIに依頼
⑤アラートルール登録(temp>35)           ── Web UI(人 or ブラウザMCP)。temp40検証の前提
⑥ローカル→本番HTTPS へ sensor-sim 単発  ── 201/alerts_fired=0 を AIが解釈
⑦temp40 で alerts_fired=1(発火)         ── AIが解釈し /alerts/history も確認
⑧連続送信でグラフ反映                   ── AIが sensor-sim 起動・グラフ反映を確認
⑨受け入れ判定＋ログ/メトリクスをAIで確認 ── AWS API MCP で監査・後始末
```

各ステップを「▶ AIへの依頼（プロンプト例）」「🤖 AIが行うMCP操作（裏で呼ばれる AWS API / コマンド）」「🙋 人が確認/承認する点」「✅ 完了判定」で示す。AI が内部で実行する具体コマンドはコードブロックで明示し、人が**何が実行されるかを監査**できるようにする。

> **安全策は §S-5 の多層（`READ_OPERATIONS_ONLY`/`REQUIRE_MUTATION_CONSENT`/denyList・elicitList/`autoApprove:[]`/IAM 最小権限/CloudTrail）をそのまま適用**する。A-4 で留意する点だけ:
> - `call_aws` は **AWS API のみ**で SSH/HTTP は扱えない。`/health` の curl と `sensor-sim` の POST はローカル Mac から、`gen-token` はサーバ上 SSH/SSM 経由。`get-instance-access-details`（SSH鍵返却）の denylist 可否は未確認（§S-5-1）。

---

### ステップ1. 本番ヘルスチェック（/health 200）＋境界確認（8080/5432 が外部閉）

リバースプロキシ経由の HTTPS で `GET /health` が 200（`{"status":"ok"}`・DB ping 込み。DB 不通なら 503 `{"status":"db_unreachable",...}`。`cmd/server/main.go` 確認済み）を返すこと、同時に **8080/5432 が外部から到達不可**（境界 OK）であることを AI に確認させる。`/health` の確認は AWS API ではなく素の HTTP なので、AI はローカル Mac の通常回線から `curl` を実行する。

**▶ AIへの依頼（プロンプト例）**
> 「本番 `https://<静的IP>.sslip.io/health` が 200 と `{"status":"ok"}` を返すか、ローカルから確認して。あわせて 8080 と 5432 が外部から閉じている（接続失敗が正常）ことも確かめて、結果を解釈して。」

**🤖 AIが行う操作（ローカル Mac で直接実行・AWS MCP 不要）**
HTTP 確認はローカル Mac から AI が直接実行する（AWS API ではない）。**外部ネットワークから**実行するのが要点（サーバ上や SSH トンネル内から打つと localhost の 8080/5432 に当たり境界破れを誤って成功と誤認する）。

```bash
# ローカル Mac の通常回線から（AI が実行・解釈）
curl -i https://<静的IP>.sslip.io/health
# 期待: HTTP/2 200 かつ {"status":"ok"}
# 503 {"status":"db_unreachable",...} なら DATABASE_URL / PostgreSQL 起動 / マイグレーション(A-3)を見直す

# 80/443 以外が公開されていないことの簡易確認（接続失敗 exit 7/28 が正常）
curl -m 5 -i http://<静的IP>.sslip.io:8080/health ; echo "exit=$?"
nc -z -w5 <静的IP>.sslip.io 5432 ; echo "exit=$?"           # PostgreSQL も到達不可(非0)が期待値
```

ファイアウォール設定の**真の状態**は AWS 側で確認するのが確実なので、AI は AWS API MCP でポート状態も突き合わせる（read-only）。

```bash
# AWS API MCP: call_aws（read-only。22/80/443 のみ・8080/5432 不在を確認）
aws lightsail get-instance-port-states --instance-name <INSTANCE> --region <REGION> \
  --query "portStates[].[fromPort,toPort,protocol,state]" --output table
```

**🙋 人が確認/承認する点**
- read-only 確認のみなので承認は基本不要。`get-instance-port-states` は読取（`lightsail:GetInstancePortStates`）で、`READ_OPERATIONS_ONLY` 下でも実行でき同意ダイアログは出ない想定。
- AI が「8080/5432 が外部から開いている」と報告したら、**B へ進む前に必ず塞ぐ**（境界違反。A-1 のファイアウォール手順＝`put-instance-public-ports` で 22/80/443 のみへ全置換し直す）。

**✅ 完了判定**
- `https://<静的IP>.sslip.io/health` が 200 + `{"status":"ok"}`。
- `curl http://<静的IP>.sslip.io:8080/health` と `nc <静的IP>.sslip.io 5432` が**いずれも接続失敗**。
- `get-instance-port-states` の出力が **22/80/443 のみ**（8080/5432 不在）。

---

### ステップ2. Web UI でユーザー登録 → ログイン

ブラウザで本番 URL を開き、登録・ログイン・dashboard 遷移ができることを確認する。**CSRF が効いている**ため（Session + gorilla/csrf。`cmd/server/main.go` 配線済み）、フォーム経由で操作する必要があり、素の `curl` では CSRF トークン不足で弾かれる。

**▶ AIへの依頼（プロンプト例）**
> 「`https://<静的IP>.sslip.io/register` でユーザー登録 → `/login` でログイン → `/dashboard` に遷移できるか確認して。HTTPS（鍵マーク）でアクセスでき、Cookie が Secure 付きで発行されることも見て。」

**🤖 AIが行う操作（ブラウザ自動化 MCP または人手・AWS MCP 不要）**
- ブラウザ操作はブラウザ自動化系 MCP（本セッションに接続中の **chrome-devtools MCP** 等）でフォーム入力 → 送信 → 遷移確認・スクリーンショットを取得できる（CSRF トークンはフォーム経由なので自動付与される）。
- chrome-devtools MCP が本番 URL を開けない/承認しない場合は、**人がブラウザで手動操作**するのが確実（CSRF・Secure Cookie の挙動は実ブラウザが最も正確）。AWS API MCP の `call_aws` はここでは使わない（AWS API ではないため）。

```text
1. https://<静的IP>.sslip.io/register でユーザー登録フォームを送信（メール・パスワード等）
   ※ APP_ENV=production のため SESSION_SECRET は 32 文字以上が必須。
     未設定/短いとアプリが起動しない（A-3 で設定済みのはず）。
2. https://<静的IP>.sslip.io/login でログイン → /dashboard へ遷移できること
3. アドレスバーが HTTPS（鍵マーク）。Cookie が Secure 付きで発行されること
```

**🙋 人が確認/承認する点**
- 登録メール・パスワードは人が決める（テスト用でも本番 DB に残る点に留意。後始末はステップ9）。
- chrome-devtools MCP に本番認証情報を入力させることに抵抗があれば、**この 1 ステップだけ人が手で操作**してよい（人手が残る誠実な明記）。

**✅ 完了判定**
- 登録 → ログイン → `/dashboard` 表示までブラウザで通る。HTTPS かつ Secure Cookie。

---

### ステップ3. デバイス登録で device_id を取得

ログイン状態のまま Web UI でデバイスを 1 件登録し、発番された `device_id` と、ログインユーザーの `user_id`（後続の gen-token に必要）を控える。

**▶ AIへの依頼（プロンプト例）**
> 「`https://<静的IP>.sslip.io/devices/create` のフォームからデバイス（名前: ハウスA温湿度計）を登録して、発番された device_id を教えて。ログインユーザーの user_id も分かれば控えて。」

**🤖 AIが行う操作（ブラウザ自動化 MCP または人手）**
- ステップ2 と同じくブラウザ系 MCP（または人手）で登録する。フォームは `GET /devices/create` で表示され、送信先は `POST /devices`（`cmd/server/main.go` 配線済み）。登録後、`/devices/<device_id>` の URL またはデバイス一覧から `device_id` を読み取る。
- `user_id` が画面から判別できない場合は、ステップ4 のサーバ上 DB 確認（`psql`）で取得する。

```text
1. https://<静的IP>.sslip.io/devices/create を開きデバイス名等を入力して登録（送信は POST /devices）
2. /devices/<device_id> またはデバイス一覧から発番された <device_id> を控える
```

**🙋 人が確認/承認する点**
- AI が報告した `device_id` が、後続の gen-token / sensor-sim で使う値と一致しているか確認（取り違えは 403/422 の主因）。

**✅ 完了判定**
- `<device_id>` が確定。可能なら `<user_id>` も控えた。

---

### ステップ4. 本番側で gen-token を実行し Bearer 発行（DB 直書き＝サーバ上）

`cmd/gen-token` は **DB へ直接書き込む**ため、PostgreSQL に到達できる場所＝**本番サーバ上**で実行する。サーバには Go を入れない方針なので、`go run`（= `make gen-token`）は使えず、**A-3 で一緒にクロスコンパイルして配布済みの `gen-token` バイナリ**を使う（旧 残作業.md A-1 補足の「(a) gen-token バイナリも scp」経路。A-3 が gen-token バイナリを成果物に含めていることが本ステップの前提）。サーバ内コマンドの実行を AI に任せる経路は 2 つ：**(a) SSM Run Command 経由（AWS API MCP の範囲・監査が CloudTrail に乗る）**、**(b) SSH 経由（既定。汎用 shell/CLI で AI が `ssh` 実行、または人が SSH）**。

> **経路の判断**：低メモリ Lightsail は SSM Agent 常駐（数十 MB 級）が負担になるため、本プロジェクトは **デプロイ反復・サーバ内コマンドは SSH を既定**とする（A-3 方針と整合）。SSM は Lightsail ネイティブ非対応で、**ハイブリッドアクティベーションで managed node 化（Fleet Manager に `mi-` プレフィックスのノード ID で登録）**済みの場合のみ (a) を選べる（出典: AWS re:Post「Add a Lightsail instance to Systems Manager」・本文 403 で二次情報ベース・要確認）。どちらも `gen-token` は env（`DATABASE_URL` / `SESSION_SECRET`[production 時 32 文字以上] / `APP_ENV`）を `config.Load()` で読むため、systemd の EnvironmentFile を source してから実行する（source しないと `required env vars missing` で失敗）。

**▶ AIへの依頼（プロンプト例）**
> 「本番サーバ上で `gen-token` バイナリを使い、user_id=`<user_id>`・name=`sim-acceptance` で Bearer トークンを発行して。env ファイルを読み込んでから実行し、出力された平文トークンを教えて（再表示不可なので確実に控える）。」

**🤖 AIが行うMCP操作（裏で実行されるコマンド）**

(a) **SSM Run Command 経由**（SSM 有効化済みの場合のみ。AWS API MCP の `call_aws` で `aws ssm send-command` を単発実行。managed node の ID は `mi-` プレフィックス）：

```bash
# AWS API MCP: call_aws（mutation 扱い → REQUIRE_MUTATION_CONSENT または人承認）
aws ssm send-command \
  --document-name "AWS-RunShellScript" \
  --targets "Key=InstanceIds,Values=<MANAGED_NODE_ID(mi-...)>" \
  --region <REGION> \
  --parameters 'commands=[
    "set -a; . /etc/go_iot.env; set +a",
    "/opt/go_iot/bin/gen-token -user=<user_id> -name=\"sim-acceptance\""
  ]'
# 実行後 aws ssm get-command-invocation で標準出力（平文トークン行）を取得
```

(b) **SSH 経由**（既定。AI は汎用 shell/CLI から `ssh` を実行、もしくは人が SSH。AWS API MCP の `call_aws` は SSH を扱わない）：

```bash
# ローカル Mac → 本番サーバ（A-1 で設定した鍵で SSH）
ssh ubuntu@<静的IP>.sslip.io '
  set -a; . /etc/go_iot.env; set +a
  # user_id 不明なら DB から確認（psql は localhost の内部接続）
  psql "$DATABASE_URL" -c "select id, email from users order by id;"
  # トークン発行（既定 abilities=["sensor:write"]。失効運用するなら -expire-days=<N>。0/未指定=無期限）
  /opt/go_iot/bin/gen-token -user=<user_id> -name="sim-acceptance"
'
# 出力の「🔑 平文トークン (ESP8266 に設定):」の値を控える（再表示不可）
```

**🙋 人が確認/承認する点**
- (a) SSM Run Command は **mutation 扱い**になりうる（任意シェル実行）。`REQUIRE_MUTATION_CONSENT`（elicitation 対応時）下では実行前に人が同意。非対応なら**人が最終承認**する。いずれにせよ実行されるシェルの中身（env source + gen-token のみ）を必ず目視確認する。
- 平文トークンは機密。AI のログ／会話履歴に残る点に留意し、漏洩経路を最小化する（受け入れ専用トークンは後始末対象＝ステップ9）。
- `gen-token` の `-user` は **device_id ではなくユーザー ID**（取り違え注意）。device_id は Web UI 発番（ステップ3）、トークンはここで発行と発行元が分かれる。

**✅ 完了判定**
- 平文トークン（`<平文トークン>`）が 1 つ得られた。Go 非インストールのままサーバ上で発行できている。

---

### ステップ5. アラート発火用のルールを Web UI で登録（temp 40 検証の前提）

`alerts_fired=1` は、**当該デバイスに発火する有効なアラートルールが登録されている場合のみ** 1 以上になる（受信時に同期判定。`ListEnabledAlertRulesByDevice`＝`is_enabled` かつ未削除のルールのみ評価。`internal/service/alert_evaluator.go` 確認済み）。temp=40 で発火させたいので、先に「温度がしきい値超で発火」するルールを 1 件作る。これを飛ばすと temp=40 を送っても `alerts_fired=0` になる。

**▶ AIへの依頼（プロンプト例）**
> 「`https://<静的IP>.sslip.io/alerts/rules` で、device=`<device_id>`・metric=温度・演算子=「より大きい(>)」・しきい値=35 の有効なルールを 1 件作って。」

**🤖 AIが行う操作（ブラウザ自動化 MCP または人手）**
- ブラウザ系 MCP（または人手）で Web UI 操作。AWS API は使わない（送信先は `POST /alerts/rules`）。

```text
1. https://<静的IP>.sslip.io/alerts/rules を開く
2. 対象デバイス = ステップ3 の <device_id> を選択
3. ルール追加: metric=温度(temperature) / 演算子=「より大きい(>)」/ しきい値=35
   （通常値 27.3 では発火せず、temp=40 で確実に発火する値）
4. ルールが「有効(ON)」で保存されていること
```

**🙋 人が確認/承認する点**
- しきい値 35 は**受け入れ検証専用**。実運用しきい値は後で直す/削除する（ステップ9）。

**✅ 完了判定**
- しきい値 35・有効状態のルールが対象デバイスに 1 件存在。

---

### ステップ6. ローカル Mac から本番 HTTPS URL へ sensor-sim（単発 201）

`cmd/sensor-sim`（ファームと同形 JSON を送る純 Go ツール・DB 不要・HTTP POST のみ）をローカル Mac から本番 HTTPS URL へ 1 件送り、**201** を得る。これは AWS API ではないので **AI がローカルで `go run` を直接実行**する（`call_aws` の範囲外）。`-url` に必ず本番 HTTPS を明示する（既定はローカル `http://localhost:8080/api/sensor-data`）。

**▶ AIへの依頼（プロンプト例）**
> 「ローカルから `cmd/sensor-sim` で本番 `https://<静的IP>.sslip.io/api/sensor-data` にトークン `<平文トークン>`・device `<device_id>` で 1 件送って、201 になるか・`alerts_fired` がいくつか結果を解釈して。」

**🤖 AIが行う操作（ローカル Mac で直接実行・AWS MCP 不要）**

```bash
# リポジトリルートで実行（ローカル Mac）。sensor-sim のフラグは cmd/sensor-sim 実装準拠
go run ./cmd/sensor-sim \
  -url https://<静的IP>.sslip.io/api/sensor-data \
  -token <平文トークン> \
  -device <device_id>
# 期待ログ: [1] 201 Created OK  t=27.30 h=62.10  resp={...,"alerts_fired":0}
```

返る `resp` は `CreateSensorReadingResponse`（`id` / `device_id` / `temperature` / `humidity` / `recorded_at` / `created_at` / `alerts_fired`。時刻 2 つは RFC3339 文字列）。通常値（既定 -temp 27.3）なのでこの単発は `alerts_fired:0` でよい（ステップ5 のしきい値 35 を超えないため）。

**🙋 人が確認/承認する点**
- 失敗時の切り分けを AI に解釈させる（sensor-sim のコメント準拠）：
  - `401`: トークン不正・未付与（ステップ4 の平文取り違え／env の DB が本番と別）
  - `403`: トークンの user と device_id の所有者が不一致（別ユーザーのデバイス）
  - `422`: バリデーション違反 or 存在しない device_id（ステップ3 の値を再確認）
  - `400`: JSON 不正、`500`: サーバ DB エラー、接続エラー: HTTPS/プロキシ（Caddy）設定

**✅ 完了判定**
- `[1] 201 Created OK` かつ `resp` に `alerts_fired:0`。

---

### ステップ7. temp=40 でアラート発火（alerts_fired=1）を確認

ステップ5 のルール（しきい値 35）を超える `-temp 40` を送り、レスポンスの `alerts_fired` が **1** になることを確認する。発火後は AI に `/alerts/history` の記録も確認させる。

**▶ AIへの依頼（プロンプト例）**
> 「同じく本番へ `-temp 40` で 1 件送って、`alerts_fired` が 1 になるか確認して。さらに `https://<静的IP>.sslip.io/alerts/history` に今の発火（温度 40・対象デバイス）が記録されているか見て。」

**🤖 AIが行う操作（ローカル Mac）**

```bash
go run ./cmd/sensor-sim \
  -url https://<静的IP>.sslip.io/api/sensor-data \
  -token <平文トークン> \
  -device <device_id> \
  -temp 40
# 期待: [1] 201 Created OK  t=40.00 ...  resp={...,"alerts_fired":1}
```

`alerts_fired:1` を確認したら、`https://<静的IP>.sslip.io/alerts/history` を（ブラウザ系 MCP または人手で）開き、たった今の発火が一覧に出ることを確認する。

**🙋 人が確認/承認する点**
- アラート判定（`internal/service/alert_evaluator.go`）には**クールダウン/重複抑制が無い**（同期判定でマッチ毎に履歴を 1 件作る実装を確認）ため、temp=40 を連続で送ると履歴がその件数分たまる。受け入れ確認では 1〜数件で十分。多重送信を AI に依頼しない。

**✅ 完了判定**
- `resp` に `alerts_fired:1`。`/alerts/history` に当該発火が記録。

---

### ステップ8. 連続送信でグラフ反映を確認（-count 0 -interval -random）

実機の 5 分周期を模した連続送信で、ダッシュボード/デバイス詳細のグラフに時系列が反映されることを確認する。受け入れ確認では待ち時間短縮のため `-interval` を短め（例 30s）にしてよい（実機本番周期は 5m）。`-temp` を付けず基準値（既定 27.3）付近にすれば、しきい値 35 を超えず余分な発火は起きない。

**▶ AIへの依頼（プロンプト例）**
> 「`cmd/sensor-sim` を 30 秒間隔・ランダム変動で連続起動して数件ためて、`https://<静的IP>.sslip.io/dashboard` と `/devices/<device_id>` のグラフに点が増えるか確認して。確認できたら停止して。」

**🤖 AIが行う操作（ローカル Mac）**

```bash
# 30 秒間隔・ランダム変動で連続送信（AI がバックグラウンド実行 → 数件後に停止）
go run ./cmd/sensor-sim \
  -url https://<静的IP>.sslip.io/api/sensor-data \
  -token <平文トークン> \
  -device <device_id> \
  -count 0 -interval 30s -random
# 各行が 201 Created OK で、t/h が基準値(27.3/62.1)の周りで変動していること
# 停止は Ctrl-C（SIGINT/SIGTERM を捕捉して安全に停止する実装）
```

数件たまったら別タブ（ブラウザ系 MCP または人手）で Web UI を開きグラフ更新を確認し、確認後に sensor-sim を停止する（連続送信ツールは本番常駐させない）。

```text
1. https://<静的IP>.sslip.io/dashboard で対象デバイスの最新値が更新されること
2. https://<静的IP>.sslip.io/devices/<device_id> のグラフに点が増え、期間切替でも表示されること
3. 確認できたら sensor-sim を停止（AI に明示的に停止を依頼。Ctrl-C で停止）
```

**🙋 人が確認/承認する点**
- AI がバックグラウンド送信を**確実に停止**したか（止め忘れると本番 DB に検証データがたまり続ける）。

**✅ 完了判定**
- 各送信が 201、グラフに点が増え期間切替でも表示、送信は停止済み。

---

### ステップ9. 受け入れ判定・ログ/メトリクス確認・クリーンアップ（AIで監査）

最後に、受信パイプラインの裏側（アプリ/Caddy ログ、Lightsail メトリクス）を AI に確認させ、受け入れチェックリストを満たすか判定する。さらに受け入れ専用に作った資産の後始末を行う。

**▶ AIへの依頼（プロンプト例）**
> 「直近の sensor-sim 送信が本番アプリに届いていたか、サーバの systemd / Caddy ログで確認して。Lightsail インスタンスの CPU メトリクスも取って異常がないか解釈して。最後に受け入れチェックリストを判定して。」

**🤖 AIが行うMCP操作（裏で実行されるコマンド）**

アプリ/Caddy ログ（サーバ内ジャーナル）の確認は SSM Run Command または SSH 経由（ステップ4 と同経路。ユニット名は A-3 で確定する想定値）：

```bash
# (a) SSM 経由（AWS API MCP: call_aws）または (b) SSH 経由で実行
journalctl -u go_iot.service --since "10 min ago" --no-pager | tail -n 50   # アプリの受信ログ
journalctl -u caddy.service  --since "10 min ago" --no-pager | tail -n 50   # TLS/プロキシのログ
```

Lightsail のリソースメトリクスは AWS API MCP の `call_aws`（read-only）で取得（Lightsail は専用メトリクス API `get-instance-metric-data`。CloudWatch 標準統合ではなく Lightsail メトリクス。メモリは標準メトリクスに無い場合があり要確認）：

```bash
# AWS API MCP: call_aws（read-only。直近のCPU使用率などを取得）
aws lightsail get-instance-metric-data \
  --instance-name <INSTANCE> --region <REGION> \
  --metric-name CPUUtilization --period 300 \
  --start-time <ISO8601> --end-time <ISO8601> \
  --unit Percent --statistics Average Maximum
# メモリ逼迫の確認は Lightsail メトリクスに無い場合があるため、journalctl/free -h(サーバ内)を併用（要確認）
```

CloudTrail 監査（AI=MCP が打った AWS 操作の追跡。証跡作成済み前提・read-only。Lightsail/SSM は CloudTrail 統合済み）：

```bash
# AWS API MCP: call_aws（read-only。本受け入れ中の AWS 操作が記録されているか）
aws cloudtrail lookup-events --region <REGION> \
  --lookup-attributes AttributeKey=EventName,AttributeValue=SendCommand \
  --max-results 10
```

**🙋 人が確認/承認する点（後始末）**

```text
[クリーンアップ：本番運用に持ち込むものは残し、受け入れ専用資産は片付ける]
- ステップ4 の sim-acceptance トークンは実機運用に使わないなら失効/再発行運用に従う
  （実機用は firmware/README §3 の config.h 設定で別途発行・投入。失効/expires_at 運用は
   実装計画.md §8-4 / firmware/README §6 を参照）。
- ステップ5 のしきい値 35 のテスト用ルールは、実運用しきい値に直すか削除する。
- ステップ6〜8 で本番 DB に残った検証用 readings と、ステップ7 の発火アラート履歴は、
  残すと運用ノイズになる。削除する場合 seed は使えない（TRUNCATE のため本番厳禁）ので、
  psql で対象 device_id の readings / alert_histories を個別 DELETE する運用を別途決める。
- sensor-sim の連続送信は停止済みであること（本番常駐させない）。
- cmd/seed は本番で絶対に実行しないこと（TRUNCATE する）。
```

DB の個別 DELETE は破壊操作なので、AI に任せる場合も**人が対象 device_id と件数を確認してから**承認する（SSM/SSH 経由で `psql ... DELETE`。SQL 文面を必ず目視）。

**✅ 受け入れ判定（このセクションのゴール）**

```text
[ ] /health が HTTPS で 200（DB 込み・{"status":"ok"}）
[ ] 8080 / 5432 が外部から到達不可（ローカル Mac=外部から確認・境界 OK）
[ ] get-instance-port-states が 22/80/443 のみ（AWS 側でも境界 OK）
[ ] Web UI で登録 → ログイン → dashboard 表示（HTTPS・Secure Cookie）
[ ] デバイス登録で device_id を取得
[ ] 本番側で gen-token しトークン発行（Go 非インストールのまま・SSM/SSH 経由）
[ ] ローカル Mac → 本番 HTTPS で単発 201（alerts_fired=0）
[ ] temp=40 で alerts_fired=1 かつ /alerts/history に記録
[ ] 連続送信でグラフ/最新値が反映（送信は停止済み）
[ ] アプリ/Caddy ログに受信が記録・Lightsail メトリクスに異常なし
[ ] (cloud-init を使った場合) userData に秘密が平文で焼かれていないことを確認
```

これらがすべて満たされれば **「実機（ESP8266）なしで本番受信パイプラインが通る＝B（ESP8266 実機書込）へ進める」** と判定する。残るは実機への書き込みと現地実証（`firmware/README.md` §4-1 / `2cc_sdd/実装計画.md` §8）。

---

### このセクションで人手が不可避に残る点（A-4 固有分。一般の不可避作業は付録D-2）

- **受け入れ専用の平文トークン**は AI の会話履歴に残るため、検証後に失効/掃除する。
- **CSRF・Secure Cookie が絡む Web UI 操作**（ステップ2/3/5）は chrome-devtools MCP で自動化できるが、本番認証情報をブラウザ MCP に渡したくなければ人が手で操作する。
- **SSM Run Command / DB の DELETE / 任意シェル実行**（ステップ4/9）は mutation＝承認ゲート（`REQUIRE_MUTATION_CONSENT`。elicitation 未対応なら `READ_OPERATIONS_ONLY` 切替＋人の最終承認）。

---

## 📦 付録【AWS 編】 ─ S 章・A 章とともに別ファイルへ移す対象（付録A〜E）

> ここから付録E までは AWS（MCP / IAM / Lightsail / cloud-init / SSH / デプロイ・受け入れ）に関する付録。後で S 章・A 章と一緒に別ファイルへ切り出す想定。

## 付録A. 調査の確度と主要出典

本書は Web 調査（2026-06 時点）に基づく。主要テーマ（[mcp] MCP サーバ構成 / [inst] 導入・cloud-init / [api] Lightsail API / [sec] 安全策・IAM）はいずれも confidence=high。ただし **本文中の「（要確認）」と、料金・bundle-id・提供リージョン・elicitation 対応・`get-instance-access-details` の denylist 可否は変動/未確認**なので、実作業前に一次情報で再確認すること（詳細リスクは付録B、要決定は付録C）。

> **マネージド版 AWS MCP Server は GA 済み（2026-05-06・下記の一次/二次情報で確認）**。本書の既定ルートは self-host（§S-4・2026-06-23 実証済み）。マネージド版（AWS SSO + `mcp-proxy-for-aws`）は本案件未検証で不採用。**ただし MCP エンドポイントの提供リージョン（us-east-1 / eu-central-1 のみ・東京エンドポイント無し）と料金は変動しうるため、接続前に要再確認。**
> - ユーザー提供記事（接続手順の出典）: Qiita「AWS MCPサーバー超進化してGAしたらしい」https://qiita.com/Syoitu/items/5022be3615ecd8b5337c
> - AWS 公式ブログ: "The AWS MCP Server is now generally available" https://aws.amazon.com/blogs/aws/the-aws-mcp-server-is-now-generally-available/
> - AWS 公式ドキュメント: "Setting up the AWS MCP Server" (Agent Toolkit for AWS) https://docs.aws.amazon.com/agent-toolkit/latest/userguide/getting-started-aws-mcp-server.html
> - GitHub: aws/agent-toolkit-for-aws ・ aws/mcp-proxy-for-aws / InfoQ "AWS MCP Server Reaches GA with Full API Coverage and IAM-Based Governance" / Future Architect ブログ「AWS MCP Server がGAに」

**主要出典（再検証用）:**
- self-host MCP: [aws-api-mcp-server](https://awslabs.github.io/mcp/servers/aws-api-mcp-server) / [knowledge-mcp-server](https://awslabs.github.io/mcp/servers/aws-knowledge-mcp-server) / [awslabs/mcp](https://github.com/awslabs/mcp)
- Lightsail: [launch script(cloud-init)](https://aws.amazon.com/blogs/compute/create-use-and-troubleshoot-launch-scripts-on-amazon-lightsail/) / [create-instances](https://docs.aws.amazon.com/cli/latest/reference/lightsail/create-instances.html) / [create-domain-entry](https://docs.aws.amazon.com/cli/latest/reference/lightsail/create-domain-entry.html) / [SSM登録(re:Post)](https://repost.aws/knowledge-center/add-lightsail-to-systems-manager)
- IAM/安全策: [Service Authorization (Lightsail)](https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonlightsail.html) / [IAMポリシー例](https://docs.aws.amazon.com/lightsail/latest/userguide/security_iam_id-based-policy-examples.html) / [CloudTrail](https://docs.aws.amazon.com/lightsail/latest/userguide/logging-lightsail-api-calls-using-aws-cloudtrail.html)

---

## 付録B. 査読で残ったリスク（AWS）

敵対的査読で挙がった、実行時に注意すべき残リスク（テーマ別に重複統合）。**いずれも「接続前の調査（2026-06 時点）ベース」であり、接続後に実機検証する**。

**MCP・承認ゲート（接続後に実機検証）**
- **AWS MCP 未接続**: 本書の MCP 挙動（env 名・`mcp-security-policy.json` 構造・既定 denylist・ツール名・elicitation 同意UI・`call_aws` 実挙動）はすべて接続前の調査ベース。接続後に実機で再検証する。`@latest` 固定のため接続前に STABLE 版 README で変数名・既定値を再確認。
- **elicitation（`REQUIRE_MUTATION_CONSENT`）が Claude Code で機能するか未検証**: 未対応だと書込同意ゲート（S-5 の③）が空振りし、`READ_OPERATIONS_ONLY` 付け外し＋IAM 最小権限＋denyList/elicitList＋人の最終承認の多層に依存する。自動化の度合いも下がる。
- **`claude mcp add` の引数構文・`.mcp.json` の `autoApprove`/`disabled` を Claude Code 現行版が解釈するか未確認**（awslabs 公式に Claude Code 専用例が無い）。
- **マネージド版 AWS MCP Server**: 東京（ap-northeast-1）エンドポイント提供有無・`mcp-proxy-for-aws` のエンドポイント/バージョンは未確定。us-east-1 経由で東京リソースを操作する際のレイテンシ/制約も未検証。self-host `aws-api-mcp-server` が即 deprecated になるかも未確定（マネージド版採用時は接続経路・env・IAM(SigV4/OAuth) を全面読み替え）。

**IAM・権限**
- **最小権限 IAM ポリシーは公式2例からの草案**: `CreateDomain`/`CreateDomainEntry`/`CreateKeyPair`/`ImportKeyPair` が Resource:* 必須か、各変更系が当該 Instance ARN に絞れるかは Service Authorization Reference で実装直前に要検証。過不足調整なしの本番投入は AccessDenied か過剰権限になりうる。
- **`READ_OPERATIONS_ONLY` の read-only 分類はサーバ内部実装依存**: 新規/特殊アクションが正しく write 分類されない可能性はゼロでない。IAM 最小権限を一次防御として重ねる。`READ_OPERATIONS_ONLY=false` の書込フェーズ中は denyList/elicitList/autoApprove と人の最終承認のみが防御＝プロンプトインジェクション耐性は IAM 最小権限（破壊系非付与）依存度が高い。
- **`lightsail get-instance-access-details`（一時SSH鍵返却）が AWS API MCP の既定 denylist 対象か未確認**（deploy/emr の get/ssh は既定 denylist）。ブロックされると AI 経由の一時鍵 SSH が成立せず、A-1 配置の登録鍵 SSH へフォールバック（鍵の常置リスクが残る）。接続後に `call_aws` で疎通要確認。
- **DNS/ドメイン系 API は us-east-1 限定**: 運用フェーズの IAM/コマンドが DNS を含む場合、東京プロファイルから `--region us-east-1` を付け忘れると失敗。
- **SSH 越しの SQL は MCP の制御外**: `READ_OPERATIONS_ONLY`/denyList が効かず、「参照系限定」は人の規律のみが防御線。AI が UPDATE/DELETE を打つ技術的ブロックは無い（read-only DB ロール分離などは未実装）。

**デプロイ経路（SSH/SSM）**
- **STEP3 のバイナリ配布(scp)は AWS の承認ゲート(CloudTrail/consent)が効かない**: 接続先IP・対象DB・seed 不実行は人の目視のみが防御線で、AI の誤ホスト配布を技術的に防げない。
- **SSM ハイブリッドアクティベーション手順は一次未取得**（re:Post/CloudBriefly が 403）: create-activation の IAM ロール詳細・SSM Agent の Lightsail への具体インストール(snap/deb)は二次情報ベース。SSM をデプロイ反復に採用する場合は公式 docs で実機再確認。低メモリ機では SSM Agent 常駐(数十MB)のメモリ圧迫リスク。
- **Lightsail メトリクスにメモリ使用率が無い可能性**: 低メモリ機で最も知りたいメモリ逼迫は AWS read-only では取れず、`journalctl`/`free -h`（サーバ内）併用に依存。
- **systemd ユニット名/env パス/配布先は A-1〜A-3 の確定待ち想定値**: 確定値と整合させないと `journalctl`/`gen-token` がそのままでは動かない。

**cloud-init・PostgreSQL（A-2）**
- **秘密配置順序**: cloud-init で秘密を排除した結果、ブート直後〜EnvironmentFile 配置までアプリは起動できない（required env 欠落）。systemd は秘密配置完了後に start する順序を厳守（A-3）。
- **cloud-init(userData) の成否は作成時点で未検証**: A-3(SSH確立後) の `/var/log/cloud-init-output.log` 確認に依存。UserData が効かない既知事例があり、初期構成失敗が後工程まで露見しないリスク。「作成後再実行不可」「userData が後から閲覧可」も要確認のまま（秘密非焼き込みのため、仮に閲覧可でも漏えいは localhost-only の生成DBパスワードに限定）。秘密を焼いていないかは A-4 開始時に実点検する。
- **PostgreSQL 設定パス(`/etc/postgresql/16/main/`) はディストリ/版差異で変わりうる**: sed が空振りすると localhost 固定/チューニングが効かない。
- **DBパスワードの URL 予約文字**: `@:/?` 等が入ると `DATABASE_URL` パースが壊れる。A-2 で記号回避を注記したが実装と要整合。記号除去で32文字に切り詰めるためエントロピーがやや下がる（localhost-only で実害小）。
- **Caddy cloudsmith/PGDG 鍵取得は外部ネットワーク依存**: 初回ブートで未確立だと `set -e` で中断（`=== 完了` が出ず完了判定で検知可）。

**Lightsail プラン/料金**
- **バンドル名・月額・bundle-id サフィックス・blueprint-id・転送量・無料枠・東京実額は変動**: 実行時に `get-bundles`/`get-blueprints` と契約画面で都度確認。本書の目安値（Nano$5/Micro$7/Small$12）は時点要確認。CPU 種別は **本案件 amd64 で確定**（配布バイナリも amd64 のみ。ARM バンドルを選ばないこと＝取り違えで `Exec format error`）。

> **センサー・基盤（ESP8266）／運用の後始末のリスクは付録G（センサー・基盤 編）へ分離**した。

---

## 付録C. 要決定事項（ユーザー判断・AWS）

着手前または途中で、ユーザーが具体値・方針を決める必要がある事項（テーマ別に重複統合）。

**MCP の形態・認証・承認・監査**
- **MCP 形態**: マネージド版 AWS MCP Server（Agent Toolkit for AWS・`mcp-proxy-for-aws`・SigV4/SSO・鍵管理は楽だが本案件未検証で不採用）か、**self-host `awslabs.aws-api-mcp-server`（既定・実証済み・自前 IAM 鍵の初回発行が必要だが東京リージョン非依存）**か。**本案件は self-host で確定**（マネージド版エンドポイントは us-east-1/eu-central-1 のみで東京無し）。
- **MCP 認証**: IAM Identity Center(SSO) の短期クレデンシャル（推奨・運用で都度 `aws sso login`）か、長期アクセスキー（プロファイル参照・env ベタ書き禁止）か。
- **承認ゲート方式**: Claude Code が elicitation 対応なら `REQUIRE_MUTATION_CONSENT` を主軸、未対応なら `READ_OPERATIONS_ONLY` 付け外し＋人の最終承認＋クライアント都度承認で代替（接続後の実機検証で確定）。
- **【決定（2026-06-23 改定）】ブラウザ MCP（chrome-devtools）で AWS 画面を AI に運転させるか**: **AWS は不採用**。① サインアップ・S-3-1 の IAM コンソール操作は **AWS の bot 検知で chrome-devtools 代行が弾かれた**ため（最初の操作でセッションごと弾かれ回避不可）、**人が自分の通常ブラウザで手動実施**する（AI はポリシー JSON 生成と疎通確認のみ）。一方 **A-4 の自前アプリ Web UI 操作（デバイス登録・アラートルール作成）は chrome-devtools MCP で実施できる**（自前アプリは bot 検知しない・付録F に実績）。詳細は §S 着手前ゲート①・§S-3-1 の冒頭注記。
- **CloudTrail 証跡の S3 継続配信**: 必須にするか任意か（AI 操作の監査基盤として推奨だが課金とセットアップ手間）。
- **破壊系 IAM**: `DeleteInstance`/`ReleaseStaticIp` 等を恒久非付与にするか、必要時のみ一時付与にするか。

**デプロイ反復・SSH・DB 到達経路**
- **デプロイ反復の経路**: SSH 既定（登録鍵 or `get-instance-access-details` の一時鍵）か、SSM ハイブリッドアクティベーション（無鍵＋CloudTrail 監査だが SSM Agent 常駐メモリ増）か。低メモリ機では SSH 既定が無難（本書の既定）。
- **SSH 鍵の扱い**: ローカル常置鍵（簡便）か、毎回 `get-instance-access-details` で一時鍵取得（常置しない・安全だが当該 API の denylist 可否が未確認）か。
- **トークン発行・マイグレーションの DB 到達経路**: (a) 専用バイナリ scp（`/opt/go_iot/go_iot_gen-token` 配置）/ (b) Web UI 発番（トークンのみ）/ (c) SSH トンネルでローカルから — どれに統一するか（既存方針は「どれかに統一」）。
- **DB マイグレーション実行場所**: 本番サーバ上で実行するか、ローカルから SSH トンネル越しか。

**Lightsail プラン・IP・DNS**
- **バンドル選定**: Nano(0.5GB)/Micro(1GB・推奨)/Small(2GB) のどれで開始するか。低メモリ＋同居構成のため Micro-1GB 以上を推奨。最小で始めてスケールアップ前提でよいか。
- **CPU アーキ**: **amd64（x86）で決定済み**（ビルドは `GOARCH=amd64`）。
- **公開IP種別**: 公開 IPv4 付き dualstack（推奨）か、2-3割安い IPv6-only（ESP8266/Let's Encrypt 到達性で詰まりやすい）か。
- **DNS 管理**: Lightsail DNS（ゾーン最大6個・us-east-1 限定）で完結か Route 53 か。本案件は A レコード 1 本なので Lightsail DNS で十分。レジストラの NS 切替は人手。

**cloud-init・秘密・systemd（A-2/A-3）**
- **DB パスワード生成**: cloud-init 内で生成し localhost-only に閉じる（既定）か、生成せず A-3 で人が後段注入（より厳格）か。
- **`SESSION_SECRET`**: 生成方式（サーバ上で `openssl rand -base64 48`）と注入手段（systemd EnvironmentFile を scp で配置／対話で生成）、再生成で全セッション無効化される運用を許容するか。
- **swap サイズ＋PostgreSQL チューニング**: swap（Micro=2G/Nano=1G）と `shared_buffers` 等を採用プランに合わせ確定する。
- **systemd `MemoryMax`**: 有効化するか（512MB プランなら 360-410M 目安・要実測）。アプリ単体上限であり Postgres 起因 OOM は swap で備える。
- **ufw による OS レベル二重防御**: 入れるか。入れるなら cloud-init に `ufw allow 22,80,443` を追加（本書は Lightsail FW を一次防御とし ufw は任意）。
- **systemd ユニット名/env パス/配布先**: 本書の値（`go_iot.service` / `/opt/go_iot/go_iot.env` / `/opt/go_iot/`）をそのまま採用するか。A-1〜A-3 の確定値と整合させる。

**受け入れ確認・本番データの後始末（A-4）**
- **Web UI 操作の自動化**: 受け入れの登録/デバイス/ルール操作を chrome-devtools MCP に本番認証情報を渡して自動化するか、その 1〜3 ステップは人手か（本番クレデンシャルをブラウザ MCP に渡す是非）。
- **受け入れ専用トークン(sim-acceptance)**: 検証後に失効させるか実機トークンへ転用するか（会話履歴に残る平文トークンの扱い）。
- **本番 DB の検証データ後始末**: 受け入れ用 readings/alert_histories/テストユーザー/B-4 テスト送信行を掃除するか残すか、掃除する場合の DELETE 手順と対象 device_id 確定方法。

> **センサー・基盤（ESP8266）の要決定事項は付録H（センサー・基盤 編）へ分離**した。

---

## 付録D. 背景・方針・役割分担・構成図（環境構築後に読めばよい）

> 本付録は「人が全体像を理解するための背景」。**環境構築（S→A→B→C）の実施には不要**で、必要時のみ参照する。AI は本文（S 章以降）を上から順に実施すればよい。

### 付録D-1. 役割分担（人／AI＋MCP／cloud-init）

作業を **「人（一度きりのブートストラップ）」「AI＋MCP（反復作業）」「cloud-init（初回ブート時の自動構成）」** の 3 者に振り分ける。原則は **「課金・認証・物理・秘密配布・最終承認は人」「AWS API 操作と調査は AI＋MCP」「インスタンス内の“秘密を含まない”初期構成は cloud-init に渡しきる」**。

| 区分 | 担い手 | 具体作業 | 頻度 |
|---|---|---|---|
| ブートストラップ | **人** | AWS アカウント作成、課金/クレカ登録、ルートユーザー/MFA 設定、ドメイン購入、**MCP 用最小権限 IAM 認証情報の初回発行**、MCP の `claude mcp add`、ESP8266 物理書込（B 章）| 一度きり |
| 最終承認 | **人** | billable（課金開始）/破壊操作の承認（`CreateInstances`・`AllocateStaticIp`・`DeleteInstance` 等）| 操作ごと |
| AWS リソース操作 | **AI＋MCP** | `call_aws` で `aws lightsail create-instances` / `allocate-static-ip` / `attach-static-ip` / `put-instance-public-ports` / `create-domain-entry` 等を実行、状態確認（`get-instance-state` 等）| 反復 |
| インスタンス初期構成（**秘密を含まない範囲のみ**）| **cloud-init** | swap 作成、PostgreSQL（native apt）導入＋低メモリチューニング、Caddy 導入、**systemd unit ファイルの配置（雛形のみ・秘密値は空欄/プレースホルダ）** — `create-instances --user-data` に渡し **初回ブート時に root 実行** | 作成時 1 回 |
| **秘密の配布**（DB パスワード/`SESSION_SECRET`/`DATABASE_URL`）| **人 or AI（SSH 経由）** | EnvironmentFile（権限 600）をブート後に SSH で配置、`systemctl start`。**userData には絶対に書かない（§S-5）**| 作成後・更新時 |
| デプロイ反復 | **AI（SSH 経由）** | クロスコンパイル済みバイナリの scp 配布、systemd 再起動、ログ確認、バックアップ、トークン発行（`cmd/gen-token`）、`migrate-up`（goose）| 反復 |
| 調査・裏取り | **AI＋Knowledge MCP** | Lightsail バンドル/料金/リージョン可用性・Caddy/PostgreSQL 手順の最新確認 | 随時 |
| ローカルビルド | **人 or AI（ローカルシェル）** | Mac で `make build`（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build`。sync-css → templ generate → build の順）。**サーバではビルドしない（Go を入れない）** | デプロイごと |

> **cloud-init の重大な制約（設計に直結・要確認: AWS Compute Blog "Create, Use, and Troubleshoot Launch Scripts on Amazon Lightsail"）**: `--user-data` のローンチスクリプトは **インスタンス作成時（初回ブート）にのみ cloud-init が root で実行し、作成後に追加・再実行できない**。よって swap/PostgreSQL/Caddy 導入・systemd unit 配置（雛形）の **初期構成は CreateInstances 時に user-data へ渡しきる**。以降の反復（バイナリ配布・再起動・トークン発行・マイグレーション・**秘密の配布**）は user-data では賄えないため SSH 経由で行う（A 章で詳述）。スクリプト内は root 実行のため `sudo` 不要、Ubuntu LTS 前提なので `apt-get` を使う。先頭に `#!/bin/bash` を付け、実行ログ `/var/log/cloud-init-output.log` を AI が作成後に確認して成否を判定する（UserData が効かない既知事例があるため、ログ検証ゲートを必ず入れる）。

### 付録D-2. 不可避に人手が残る作業（正直な列挙）

「人がコマンドを打たない」を主眼にしても、**次は AI に任せられない・任せるべきでない**。誠実に明記する。

1. **AWS アカウント作成・課金/クレジットカード登録**: アカウント開設、支払い手段の登録は AWS のサインアップ手続きであり AI の範囲外。
2. **ルートユーザー保護・MFA 設定**: ルート資格情報の保護と多要素認証の有効化は人の責任で行う。AI に渡さない。
3. **独自ドメインの購入**: ドメインの取得・支払いは人手（課金）。DNS の A レコード設定自体は AI が `aws lightsail create-domain-entry`（`--region us-east-1` 必須・付録D-5 参照）で行えるが、購入は人。
4. **MCP 用 IAM 認証情報の初回発行**: AI に AWS 書込権限を与えるための最小権限 IAM ユーザー/ロールの作成とアクセスキー（または SSO プロファイル）の発行。**ここを AI に任せると「AI が自分の権限を拡張できる」自己権限昇格になりかねない** ため、必ず人が行う（§S-3）。長期アクセスキーより IAM Identity Center（SSO）の短期クレデンシャルが望ましい。
5. **MCP の接続設定（`claude mcp add` / `.mcp.json`）**: どの MCP をどの権限・どのリージョンで動かすかの初期設定は人が行う（§S-1/§S-4）。
6. **秘密値の配布**: DB 本番パスワード・`SESSION_SECRET`（本番 32 文字以上）・`DATABASE_URL` は **userData ではなく** SSH 経由で EnvironmentFile（権限 600）として配置する。秘密の取り扱い境界は人が握る（§S-5）。
7. **ESP8266 の物理書込（B 章）**: USB 接続・Arduino IDE での焼き込みは物理作業。AI の範囲外。
8. **billable / 破壊操作の最終承認**: `CreateInstances`（課金開始）・`AllocateStaticIp`（未アタッチ放置で課金）・スナップショット作成・`DeleteInstance` / `ReleaseStaticIp`（破壊）等は、AI が提案・コマンド生成までを行い、**実行のトリガーは人が承認**する（§S-5 の承認ゲート）。

### 付録D-3. テキスト構成図（人 → AI 対話 → MCP → AWS / cloud-init / SSH）

役割を反映した経路は次のとおり。**人は自然言語で頼み、最終承認と秘密配布だけを握る**。AI は MCP（AWS API）と SSH を使い分け、インスタンスの“秘密を含まない”初期構成は cloud-init に委ねる。（図中の `awslabs.aws-api-mcp-server` は self-host 版＝**本書の既定（§S-4・実証済み）の表記どおり**。マネージド版は本案件未検証で不採用。）

```
                          ┌──────────────────────────────────────────────┐
   [人(運用者)]           │  ブートストラップ(一度きり・人手不可避):        │
      │                   │   AWSアカウント/課金/MFA・ドメイン購入・         │
      │  自然言語で依頼    │   最小権限IAM発行・claude mcp add・ESP8266書込   │
      │  ＋ 最終承認       │   ＋ 秘密(DBパス/SESSION_SECRET)のSSH配布         │
      │  ＋ 秘密配布       └──────────────────────────────────────────────┘
      v
 ┌─────────────┐   ツール呼び出し   ┌──────────────────────────┐
 │  AI         │ ───────────────► │ AWS API MCP Server        │  call_aws / suggest_aws_commands
 │ (Claude     │                   │ (awslabs.aws-api-mcp-     │  READ_OPERATIONS_ONLY /
 │  Code)      │ ◄─────────────── │  server, 最小権限IAM)      │  REQUIRE_MUTATION_CONSENT
 └─────────────┘   結果/同意要求    └──────────────────────────┘
      │  │                                      │
      │  │  ドキュメント裏取り(read-only)         │ aws lightsail ... (AWS API)
      │  v                                      v
      │ ┌────────────────────────┐        ┌──────────────────────────────────────┐
      │ │ AWS Knowledge MCP       │        │              AWS (Lightsail)            │
      │ │ (リモート・認証不要)      │        │  create-instances --user-data <スクリプト> │
      │ └────────────────────────┘        │        │(初回ブートのみ root 実行・秘密は含めない) │
      │                                    │        v                                │
      │  SSH/scp (反復デプロイ＋秘密配布)   │   cloud-init: swap/PostgreSQL/Caddy/systemd雛形 │
      └──────────────────────────────────►│   Go アプリ(:8080 平文/localhost) ← Caddy   │
         バイナリ配布・systemd再起動・        │   PostgreSQL(localhost:5432・native apt)    │
         ログ確認・gen-token・migrate-up・    │   EnvironmentFile(600/秘密) ← SSHで後置     │
         EnvironmentFile(600)をSSHで後置      │   公開ポートは 80/443(+22は接続元IP限定)のみ  │
                                            └──────────────────────────────────────┘

 監査: AWS API 呼び出し → CloudTrail（MCP用IAMで AI 操作を識別）
 注意: 秘密(DBパス/SESSION_SECRET/DATABASE_URL)は userData に焼かず SSH で EnvironmentFile(600) として配置
```

不変条件（MCP 化しても維持・正本は §S-0）: **低メモリ前提 / サーバでビルドしない（ローカルで事前クロスコンパイル）/ アプリは平文 :8080 を 0.0.0.0 bind しつつ外部 FW で非公開 / PostgreSQL :5432 と :8080 は外部非公開 / Caddy 自動 TLS で 80・443 を終端 / 環境変数は systemd の `EnvironmentFile=`・`Environment=` で渡す（`.env` 自動読込なし・秘密ファイルは権限 600）/ 秘密値は userData に平文で焼かない**。

### 付録D-4. A→B→C 依存と「AI に頼める／頼めない」の線引き

A → B → C は **直列依存**（前段の成果物が後段の入力）。各段で AI の守備範囲を明示する。

```
A. サーバデプロイ
   ├ 成果物: 本番URL(https://<ドメイン>/api/sensor-data) / device_id / Bearer平文トークン
   │  ┌ AIに頼める: Lightsailインスタンス作成(承認後)・静的IP・FWポート設定・DNS Aレコード(--region us-east-1)・
   │  │             状態確認/ログ確認・cloud-init同梱(秘密を含まない初期構成)・SSHでバイナリ配布/再起動/migrate-up/トークン発行
   │  └ AIに頼めない(人手): アカウント/課金/MFA・ドメイン購入・IAM初回発行・claude mcp add・秘密(DBパス/SESSION_SECRET)配布・課金/破壊の最終承認
   v
B. ESP8266 実機書込
   ├ A の URL/token/device_id を firmware/esp8266_sht31/config.h に転記して焼く
   │  ┌ AIに頼める: 焼く前の本番疎通先行確認(cmd/sensor-sim で実機同形 JSON を本番へ送る・SSH/ローカル)
   │  └ AIに頼めない(人手): USB接続・Arduino IDE での物理書込・実機の配線/起動
   v
C. 現地実証
   ├ 実機を圃場に設置し、実センサー値が Web UI の直近24hグラフに出ることを確認
   │  ┌ AIに頼める: サーバ側の受信確認・ログ/DB確認・グラフ表示確認・アラート判定確認
   │  └ AIに頼めない(人手): 現地への設置・電源/Wi-Fi 接続・物理トラブルシュート
```

**A を完了して本番 URL・device_id・トークンが確定しないと B の `config.h` を埋められず、B が動かないと C に進めない。** この直列性は MCP 化しても変わらない。

> **MCP 形態の選択（§S-1）**: 本書の既定は **self-host（§S-4・2026-06-23 実証済み）**。マネージド版 AWS MCP Server（Agent Toolkit for AWS）は本案件未検証で不採用だが、利点は CloudTrail 全記録・CloudWatch `AWS-MCP` 名前空間で人/AI 操作を分離監査・IAM context key ガードレール・長期鍵不要（SSO）。**本案件は self-host を採用**（自前 IAM 鍵の初回発行が要るが東京リージョン非依存・マネージド版は東京エンドポイント無し）。

### 付録D-5. 各操作の記述フォーマットと既知の罠

本文の AWS / インスタンス操作は、**「人がコマンドを直接打たない」を主眼**に、次の対話フォーマットで記述する。AI が内部で実行する具体コマンド・設定（cloud-init・systemd unit 等）は **透明性のためコードブロックで明示** する（人が「AI が何をするか」を監査できるように）。

> **▶ AI への依頼（プロンプト例）** — 人が AI に投げる自然言語の例。
> **🤖 AI が行う MCP 操作（裏で呼ばれる AWS API / コマンド）** — AI が `call_aws` 等で実行する CLI を透明に提示。
> **🙋 人が確認/承認する点** — 課金・破壊・秘密が発生する操作で人が握るゲート。
> **✅ 完了判定** — 客観的に確認できる成功条件（例: `get-instance-state` が `running`／CloudTrail に記録／`/var/log/cloud-init-output.log` にエラーなし／`get-instance-port-states` が 22/80/443 のみ）。

> **リージョンの罠（要確認・AWS CLI create-domain-entry リファレンス）**: Lightsail の **ドメイン/DNS 系 API は `us-east-1` でのみ動作** する。インスタンスは `ap-northeast-1`、DNS は `us-east-1` とリージョンが分かれるため、AI が DNS 操作を `call_aws` で行う際は **`--region us-east-1` を明示** しないと失敗する（A-1 の DNS ステップで明記）。Lightsail DNS ゾーンは最大 6 個・対応型 A/AAAA/CNAME/MX/NS/SOA/SRV/TXT のみ。本プロジェクト（A レコード 1 本）は Lightsail DNS で十分だが、超過時は Route 53 を使う。

> **デプロイ反復の経路（要確認）**: Lightsail は SSM にネイティブ非対応だが、ハイブリッドアクティベーション（`aws ssm create-activation` で ActivationCode/Id 取得 → インスタンス上で SSM Agent を `-register`）で Systems Manager に登録すれば Session Manager / Run Command が使える（SSM Agent 常駐メモリが低メモリ機の負担・手順詳細は re:Post 403 で一次未取得＝要確認）。`call_aws` は AWS API 専用で対話的 SSM セッションを扱いにくいため、**デプロイ反復（バイナリ scp・systemd 再起動・ログ確認・migrate-up・トークン発行・秘密配布）は SSH 経由を既定**とし、SSM Run Command（`aws ssm send-command` 単発）は将来オプション扱い（§S-7）。

> **SSH 鍵の取得経路（要確認）**: AI が SSH するための一時鍵は `aws lightsail get-instance-access-details`（一時秘密鍵・username・publicIp・有効期限を返す）で都度取得できるが、**AWS API MCP は deploy/emr 等の get/ssh 系操作を既定で denylist しており、`get-instance-access-details` が denylist 対象か公式未明記（要確認）**。ブロックされる場合は、A-1 でダウンロードした Lightsail 既定鍵（または登録した独自鍵）を端末に保持して SSH する従来経路にフォールバックする。いずれにせよ秘密鍵はコミットしない・権限 600。

### 付録D-6. AWS API MCP で賄えない作業の境界（DB 到達操作）

`migrate-up`（goose）・`cmd/gen-token`・`cmd/seed` は `DATABASE_URL` で **localhost:5432（外部非公開）** の PostgreSQL に直接接続する。これらは AWS API ではなくインスタンス内操作なので、AWS API MCP の `call_aws`（AWS API 専用・SSH/scp は扱わない）では実行できない。**SSH 経由でインスタンス上で実行**する。サーバには Go を入れないため、トークン発行・マイグレーションは (a) ローカルでクロスコンパイルした専用バイナリを scp して使う、(b) （トークンのみ）Web UI のデバイス登録経由で発番する、(c) SSH トンネル越しにローカルから DB へ接続して実行する、のいずれかに統一する。**`cmd/seed` は本番禁止（TRUNCATE するため）**。

---

## 付録E. 実デプロイ実行記録（2026-06-23 実施）— 実際の手順・成功手順・つまづき

> 2026-06-23 に AI（Claude）が **AWS API MCP（`call_aws`）＋ローカル Mac の SSH/scp/curl** で §S→A-1→A-2→A-3→A-4 を実際に通した記録。本書の各節は「設計意図」、本付録は「**実機で確定した値・修正後の正本手順・踏んだ罠**」。**コマンドが食い違う場合は本付録を優先**。cloud-init の正本は `deploy/cloud-init.sh`（本付録の修正反映済み・本書 §A-2 埋め込みスクリプトは旧版の設計記録）。

### E-0. 確定した実値（実機）

| 項目 | 値 |
|---|---|
| インスタンス | `go-iot-prod` / bundle **`micro_3_0`**（$7/月・1GB・2vCPU・40GB・amd64）/ blueprint **`ubuntu_24_04`** / AZ `ap-northeast-1a` / `--ip-address-type dualstack` |
| ARN（INSTANCE_GUID） | `arn:aws:lightsail:ap-northeast-1:<ACCOUNT_ID>:Instance/83a9ed3a-b9b1-4eba-a843-1899829d86b5` |
| 静的IP | **`57.182.65.19`**（`go-iot-prod-ip`・アタッチ済） |
| 本番FQDN | **`57.182.65.19.sslip.io`**（Let's Encrypt 自動TLS・有効期限 2026-09-21・Caddy 自動更新） |
| FW 開放ポート | 22（管理元 `<管理元IP>/32` のみ）/ 80 / 443 |
| AWSアカウント | `<ACCOUNT_ID>` / IAMユーザー `go-iot-mcp`（Lightsail 限定インラインポリシー） |
| 管理元IP | `<管理元IP>`（SSH 許可元＝ローカル Mac の egress IP） |
| ログインユーザー | user_id=1（<ユーザー名> / `<ADMIN_EMAIL>`） |

### E-1. §S AWS API MCP 接続のつまづき（最重要・macOS 固有）

**`uvx` 形は macOS で起動しない**:
- `.mcp.json` の `"command":"uvx","args":["awslabs.aws-api-mcp-server@latest"]` は **macOS で起動失敗**。uvx が生成する relocatable ランチャーが `realpath` で同梱 python を解決する作りだが、**macOS に `realpath` が標準で無い** → 解決失敗で相対 `python`（`$PWD/python`）を実行しようとして死ぬ（`realpath: command not found`）。
- さらに **Cursor / GUI は最小 PATH で子プロセスを起こす**ため `command:"uvx"`（`~/.local/bin` 配下）自体も PATH 未解決になりうる。症状＝`aws-knowledge`(HTTP)は繋がるが `aws-api`(stdio)だけ接続中リストにも出ない。
- **✅ 修正（正本・`.mcp.json` の `aws-api`）**: 壊れたラッパーを回避し `uv` 絶対パスで `python -c` から `main()` を直接呼ぶ。
  ```json
  "aws-api": {
    "type": "stdio",
    "command": "/Users/c/.local/bin/uv",
    "args": ["run","--no-project","--with","awslabs.aws-api-mcp-server",
             "python","-c","import sys; sys.argv=['awslabs.aws-api-mcp-server']; from awslabs.aws_api_mcp_server.server import main; main()"],
    "env": { "AWS_REGION":"ap-northeast-1","AWS_API_MCP_PROFILE_NAME":"go-iot-mcp","READ_OPERATIONS_ONLY":"true","REQUIRE_MUTATION_CONSENT":"true","AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS":"no-access","FASTMCP_LOG_LEVEL":"INFO" }
  }
  ```
- 起動切り分け: `printf '<initialize JSON>' | perl -e 'alarm shift; exec @ARGV' 25 <command...>` で MCP `initialize` 応答を確認（**macOS に `timeout` 無し** → perl alarm 代用）。`tools/list` に `call_aws`/`suggest_aws_commands` が出れば OK。
- `.mcp.json` 変更は **Cursor の Reload Window（Cmd+Shift+P → 「Developer: Reload Window」）で発効**。プロジェクト MCP は初回に承認も要る（`/mcp` で connected 確認）。
- ✅ **追記（2026-06-23・Claude Code では `uvx` 形でも接続成功）**: 上の uvx 不具合は **Cursor の最小 PATH＋`realpath` 不在**が要因。**Claude Code（VSCode 拡張/CLI）では `claude mcp add aws-api -s project --env … -- uvx awslabs.aws-api-mcp-server@latest`（uvx 直叩き）のまま再起動で `aws-api` が connected になり `call_aws` が動いた**。クライアントが変わると uvx 形の可否が変わるので、**繋がらなければ上の `uv run … python -c …` 形にフォールバック**。事前に `uv tool install awslabs.aws-api-mcp-server` でパッケージ DL を済ませると再起動後の初回起動が速い。

### E-2. §S-6 read-only 疎通（成功）

- ゲート: `aws sts get-caller-identity --profile go-iot-mcp` → `arn:.../user/go-iot-mcp`。
- `call_aws "aws lightsail get-instances --region ap-northeast-1"` → 空（未作成で正常）。
- `get-regions`→ Tokyo 含む / `ec2 describe-instances`→ AccessDenied（最小権限が効いている）。
- バンドル実値（`get-bundles`）: `nano_3_0`=$5 / **`micro_3_0`=$7**（1GB/2vCPU/40GB/2TB転送）/ `small_3_0`=$12。本書目安と一致。**IPv6 専用（`micro_ipv6_3_0`=$5）は不採用**＝パブリック IPv4 を持たず sslip.io/IPv4 SSH が成立しないため。

### E-3. A-1 プロビジョニングのつまづき

1. **承認ゲート（elicitation）が Cursor で却下される**: `create-instances` 実行時、MCP サーバの `ctx.elicit()`（`REQUIRE_MUTATION_CONSENT` / `elicitList`）が `User rejected the execution of the command` を返す（`METHOD_NOT_FOUND` ではない＝ダイアログは出るが承認として返らない＝**Cursor の elicitation 非互換**）。`policy.py` の優先＝**deny > elicit > default** で、`REQUIRE_MUTATION_CONSENT=true` か elicitList 該当のどちらでも ELICIT 強制。
   - **✅ 代替策（§S-5-2）**: `.mcp.json` `REQUIRE_MUTATION_CONSENT=false` ＋ `~/.aws/aws-api-mcp/mcp-security-policy.json` の elicitList から **A-1 書込オペ5件**（`create-instances`/`allocate-static-ip`/`attach-static-ip`/`put-instance-public-ports`/`open-instance-public-ports`）を一時除外。**denyList（破壊系8件）は維持**。承認ゲートは **Claude Code 本体の許可プロンプト＋READ_ONLY トグル＋IAM 最小権限**に一本化。作業後に元へ戻す（原本 backup＝`mcp-security-policy.json.bak-fulllist`）。
2. **`--tags` が AccessDenied**: `create-instances --tags key=project,value=go-iot` は `lightsail:TagResource` 権限が要る。最小権限ポリシーに無く `AccessDeniedException`（インスタンスは作られず **atomic に拒否**＝オーファン無し）。**✅ `--tags` を外して作成**（タグ無し・最小権限維持）。タグ運用したいなら IAM に TagResource 追加が要る（人手）。
3. **`--user-data file://` がブロックされる**: `AWS_API_MCP_ALLOW_UNRESTRICTED_LOCAL_FILE_ACCESS=no-access` だと call_aws が `Cannot accept file path: local file access is disabled` で拒否。値は **`no-access` / `workdir` / `true`(=unrestricted)**（ソース確認）。**✅ `workdir` ＋ `AWS_API_MCP_WORKING_DIR=/Users/c/Desktop/dev/go_iot`** を設定（プロジェクト内ファイル参照に限定したまま `file://deploy/cloud-init.sh` が通る・相対パスは WORKING_DIR 基点）。
4. **書込フェーズ切替＝Reload Window**: `READ_OPERATIONS_ONLY` / file access / `REQUIRE_MUTATION_CONSENT` / elicitList の変更は Reload Window で発効。

**実際に成功した A-1 コマンド列（call_aws 経由）**:
```bash
aws lightsail create-instances --instance-names go-iot-prod --availability-zone ap-northeast-1a \
  --blueprint-id ubuntu_24_04 --bundle-id micro_3_0 --ip-address-type dualstack \
  --user-data file://deploy/cloud-init.sh --region ap-northeast-1
# → get-instance-state で running 確認 / get-instance で ARN 取得
aws lightsail allocate-static-ip --static-ip-name go-iot-prod-ip --region ap-northeast-1
aws lightsail attach-static-ip --static-ip-name go-iot-prod-ip --instance-name go-iot-prod --region ap-northeast-1
# → get-static-ip で 57.182.65.19 取得（attach 必須・未attach放置は課金）
aws lightsail put-instance-public-ports --instance-name go-iot-prod --region ap-northeast-1 \
  --port-infos "fromPort=22,toPort=22,protocol=TCP,cidrs=<管理元IP>/32" \
               "fromPort=80,toPort=80,protocol=TCP,cidrs=0.0.0.0/0" \
               "fromPort=443,toPort=443,protocol=TCP,cidrs=0.0.0.0/0"
# → get-instance-port-states で 22/80/443 のみ・8080/5432 不在を確認
```

### E-4. A-2 cloud-init の4バグ（全て修正済・正本＝`deploy/cloud-init.sh`）

**cloud-init は初回ブートで失敗 → SSH から `sudo bash`（bash 明示）で手動冪等実行して完成**（cloud-init は再実行不可）。踏んだ順:

1. **Lightsail は user-data を `/bin/sh`(dash) で実行**（Lightsail 自社初期化スクリプトに連結され、こちらの shebang `#!/bin/bash` が無視される）。`set -o pipefail` は dash 非対応 → 連結後 line22 で即死（`set: Illegal option -o pipefail`）→ 何も構成されない。**✅ `set -euo pipefail` → `set -eu`（POSIX sh 互換）**。
2. **`psql -c "...PASSWORD :'pass'"` が `syntax error at or near ":"`**: `-c` では psql 変数 `:'pass'` が展開されずリテラル送出。**✅ stdin スクリプトモードへ**:
   ```bash
   sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
   \set pass '$DB_PASS'
   CREATE ROLE go_iot WITH LOGIN PASSWORD :'pass';
   SQL
   ```
3. **Caddy インストールが `NO_PUBKEY`**: DB パス保護用の `umask 077` が後続に漏れ、Caddy 鍵リング `/usr/share/keyrings/caddy-stable-archive-keyring.gpg` が 600 で作られ、apt 取得時の `_apt` ユーザーが読めない。**✅ `umask 077` をサブシェル `( umask 077; printf '%s' "$DB_PASS" > "$PASS_FILE" )` に閉じ込め＋鍵リング/.list を明示 `chmod 644`**。
4. **600 鍵リングの連鎖故障（鶏卵）**: 一度 600 鍵リングが出来ると、次回実行の **step2 の一般 `apt-get update`（全リポジトリ更新）も NO_PUBKEY で失敗**し、`set -e` で step6 の chmod に到達する前に中断。**フレッシュなインスタンスなら修正版（step6 で初めて 644 鍵リングを作る順序）で解消**。既存の壊れた状態は手動で `sudo chmod 644 <鍵リング> <.list>` してから `apt-get update && apt-get install -y caddy`。

**A-2 検証（全て✅）**: swap 2.0Gi / TZ Asia/Tokyo / postgresql+caddy active / 5432=127.0.0.1 のみ / sshd passwd・root=no / role+db `go_iot` / `/root/go_iot_db_pass` 600 / `/opt/go_iot` 750 go_iot / Caddyfile `:80{reverse_proxy localhost:8080}` / `curl localhost`=502（アプリ未配備で正常）。

### E-5. SSH アクセスの確立（つまづき）

- **`aws lightsail download-default-key-pair` は AccessDenied**（IAM 最小権限に無い）。
- **✅ `aws lightsail get-instance-access-details` を使う**（IAM 許可あり）。ただし **Lightsail の一時アクセスは SSH 証明書ベース**で、`privateKey` だけでなく **`certKey`（SSH 証明書）が必須**（秘密鍵だけだと `Permission denied (publickey)`）。
- **✅ 証明書は ssh 自動探索命名で置く**: 秘密鍵 `~/.ssh/lightsail-goiot.pem`（600）＋証明書 `~/.ssh/lightsail-goiot.pem-cert.pub`（600・**`<秘密鍵ファイル名>-cert.pub` 命名**）。`-o CertificateFile=` 指定は perms チェックで弾かれることがあるため自動探索が確実。接続: `ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@57.182.65.19`（user は `get-instance-access-details` の `username`＝`ubuntu`）。証明書は短命＝作業ごとに再取得。SSH 元 IP は管理元 `<管理元IP>` のみ（FW）。サーバ作業は `sudo bash`（Lightsail の ubuntu は NOPASSWD sudo）。
- 鍵/証明書を会話に残さないため、`get-instance-access-details` は **call_aws ではなく直接 `aws` CLI で JSON をファイルへ**取得し python で `privateKey`/`certKey` を抽出（チャットに鍵を出さない）。

### E-6. A-3 配備（成功手順・ローカル Mac 実行／AWS 不要）

```bash
# ① ビルド（amd64・sync-css → templ generate → 個別 build）
make sync-css && go tool templ generate
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
  -ldflags="-s -w -X github.com/HiroshiKawano/go_iot/internal/view.Version=$(git rev-parse --short HEAD)" \
  -o go_iot_server ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o go_iot_gen-token ./cmd/gen-token
file go_iot_server go_iot_gen-token   # ELF 64-bit x86-64 を確認

# ② scp → 配置（/opt/go_iot は A-2 で作成済・go_iot 所有・755）
scp -i ~/.ssh/lightsail-goiot.pem go_iot_server go_iot_gen-token ubuntu@57.182.65.19:/home/ubuntu/
ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@57.182.65.19 'sudo bash -c "
  mv /home/ubuntu/go_iot_server /home/ubuntu/go_iot_gen-token /opt/go_iot/
  chown go_iot:go_iot /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token
  chmod 755 /opt/go_iot/go_iot_server /opt/go_iot/go_iot_gen-token"'

# ③ EnvironmentFile（go_iot.env・600・秘密はサーバ内で生成しチャットに出さない）
#    config.go は os.Getenv 直読み＝実体は systemd EnvironmentFile（A-2 の .env は未使用の枠）
ssh ... 'sudo bash -c "
  DBPASS=\$(cat /root/go_iot_db_pass); SECRET=\$(openssl rand -base64 48)   # 64字（production は32字以上必須）
  umask 077
  tee /opt/go_iot/go_iot.env >/dev/null <<EOF
APP_ENV=production
APP_PORT=8080
DATABASE_URL=postgres://go_iot:\${DBPASS}@localhost:5432/go_iot?sslmode=disable
SESSION_SECRET=\${SECRET}
EOF
  chown go_iot:go_iot /opt/go_iot/go_iot.env; chmod 600 /opt/go_iot/go_iot.env"'

# ④ goose v7（SSH トンネル 15432→5432・GOOSE_DBSTRING で argv/ps 非露出）
DBPASS=$(ssh -i ~/.ssh/lightsail-goiot.pem ubuntu@57.182.65.19 'sudo cat /root/go_iot_db_pass')
ssh -fN -i ~/.ssh/lightsail-goiot.pem -o ExitOnForwardFailure=yes -L 15432:localhost:5432 ubuntu@57.182.65.19
export GOOSE_DRIVER=postgres
export GOOSE_DBSTRING="postgres://go_iot:${DBPASS}@localhost:15432/go_iot?sslmode=disable"
go tool goose -dir db/migrations up        # v1〜v7 適用（"migrated to version: 7"）
lsof -ti tcp:15432 | xargs kill            # トンネル停止（15432 解放）

# ⑤ systemd（/etc/systemd/system/go_iot.service・EnvironmentFile=/opt/go_iot/go_iot.env・Restart=always・After=postgresql.service）
#    → daemon-reload → systemctl enable --now go_iot
# ⑥ /health（サーバ内 localhost:8080・DB ping 込み）→ 200 {"status":"ok"}
```
**ポイント**:
- 環境ファイルは **`go_iot.env`**（A-2 が作る `.env` は枠で未使用）。`SESSION_SECRET` は `openssl rand -base64 48`＝64字。DB パスは英数のみ生成済で **URL エンコード不要**。
- goose の認証情報は **`GOOSE_DBSTRING` 環境変数**で渡し argv（`ps`）露出を避ける。
- 結果: `go_iot.service` active+enabled / `listening on :8080 (env=production)` / **`/health`=200 `{"status":"ok"}`** / Caddy:80→:8080 も 200（ルートは 302＝ログイン誘導）。8080 は `*:8080` listen だが FW で外部遮断。

### E-7. A-4 本番 HTTPS 公開（成功手順）

```bash
# sslip.io は「ドット形式」でも解決する
dig +short 57.182.65.19.sslip.io        # → 57.182.65.19
# Caddyfile を本番ドメイン+email へ差し替え → validate → reload（Let's Encrypt 自動TLS）
ssh ... 'sudo bash -c "
  tee /etc/caddy/Caddyfile >/dev/null <<EOF
{
    email <ADMIN_EMAIL>
}
57.182.65.19.sslip.io {
    reverse_proxy localhost:8080
}
EOF
  caddy validate --config /etc/caddy/Caddyfile && systemctl reload caddy"'
```
- **証明書は TLS-ALPN-01 チャレンジ（443 経由）で取得成功**（journal: `http.acme_client ... authorization finalized ... authz_status:"valid"`）。HTTP-01(80) ではなく ALPN(443) が使われた。
- 検証（ローカル Mac＝外部回線から）: `curl -i https://57.182.65.19.sslip.io/health` → **HTTP/2 200 `{"status":"ok"}`**（正規証明書・`-k` 不要）。issuer=Let's Encrypt(YE2)・subject=`57.182.65.19.sslip.io`・有効期限 2026-09-21・**Caddy 自動更新**。HTTP→HTTPS=308。
- 境界: `curl -m5 http://57.182.65.19:8080/health` → timeout(exit 28) / `nc -z -w5 57.182.65.19 5432` → 閉 / `get-instance-port-states` → 22/80/443 のみ。
- 旧 `:80` 雛形は `/etc/caddy/Caddyfile.bak-placeholder` に backup。

### E-8. A-4 受け入れ（ログイン検証まで実施・残りは未了）

- ユーザー登録（ブラウザ・人手）→ user_id=1（<ユーザー名> / `<ADMIN_EMAIL>`）。
- **ログインをローカル curl で実証**（gorilla/csrf 対策の要点）:
  - `GET /login` でフォームから `gorilla.csrf.Token`（masked・len88）と Cookie を取得（`-c jar`）。
  - `POST /login` は **Referer ヘッダ必須**（gorilla/csrf は HTTPS で Referer の同一オリジンを厳格チェック）→ `curl -e https://57.182.65.19.sslip.io/login`。`email` + `password` + CSRF を `--data-urlencode`。
  - 結果: `POST /login` → **303**（成功）、認証 Cookie で `GET /dashboard` → **200**（「ダッシュボード」「ログアウト」表示）。Secure Cookie。
- データモデル: `device_tokens` は **user_id 紐付**（`gen-token -user=1 -name=...`・ability 既定 `["sensor:write"]`・平文は発行時 1 回のみ表示）。`devices` は別エンティティ（name・**mac_address 必須** `^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`・location）。sensor POST は Bearer ＋ device_id を使う。
- **残だった項目 → すべて完了（2026-06-23）**: デバイス登録・`gen-token`・`cmd/sensor-sim`(201)・**実機(SHT31)からの送信(201)**・グラフ反映・**アラート発火**（§A-4 step3-9）は **付録F** で実施完了。**§A-3 STEP8 日次 `pg_dump` cron も実施完了**（`/usr/local/bin/pg_backup.sh`・`~/.pgpass`(600)・cron `10 3`＝03:10 JST・14日保持・**復元検証 sensor_readings 10=10 行 OK**）。残るは **C 章**（現地実証・継続運用）。
  - STEP8 実行知見: ①cron 未登録時に `crontab -l` が空で `set -euo pipefail` により中断 → `crontab -l 2>/dev/null | grep -Fv ... || true` で対処。②`/etc/timezone` ファイルは `Etc/UTC` だが `/etc/localtime`→Asia/Tokyo・`timedatectl`/`date` は **JST(+0900)** ＝cron は `/etc/localtime` 基準なので **03:10 JST に発火**（PostgreSQL timezone も Asia/Tokyo・created_at +09:00 と整合）。③`~/.pgpass` の db フィールドは `*`（cron の go_iot と復元検証 go_iot_restore_test の両方に適合）。

### E-9. 運用メモ（再実行時の要点）

- **書込フェーズの開閉**: A 章の AWS 書込時だけ `.mcp.json` を `READ_OPERATIONS_ONLY=false` / file access=`workdir`（+`AWS_API_MCP_WORKING_DIR`）/ `REQUIRE_MUTATION_CONSENT=false` ＋ elicitList から該当オペ除外。終わったら全て戻して Reload Window。A-3/A-4 以降の SSH 反復は AWS 書込不要なので read-only のままでよい。
- **SSH 再接続**: `aws lightsail get-instance-access-details --instance-name go-iot-prod --region ap-northeast-1 --profile go-iot-mcp` を再取得（**証明書は実測で有効期限 約13分**と非常に短い＝SSH/scp は取得直後にまとめて実行する）→ 鍵/証明書を `~/.ssh/lightsail-goiot.pem`(+`-cert.pub`) または `~/.ssh/go-iot-prod-temp`(+`-cert.pub`) に置き直す。長い作業は **SSH セッション確立後はトンネル/接続を維持**すれば証明書失効後も継続可（認証は接続時のみ）。
- **macOS に無いコマンド**: `realpath`（uvx ランチャー破損の原因）・`timeout`（`perl -e 'alarm shift; exec @ARGV' <sec> <cmd>` で代用）。
- **正本ファイル**: cloud-init=`deploy/cloud-init.sh`（4 バグ修正済）。再現時はこのファイルを使う（本書 §A-2 埋め込みは設計記録＝旧版）。
- **秘密の扱い（実績）**: DB パスはインスタンス内 `openssl rand` 生成→`/root/go_iot_db_pass`(600)、`SESSION_SECRET` はサーバ上 `openssl rand` 生成→`go_iot.env`(600)、いずれもチャット/コミットに出さない。Bearer トークンは発行時 1 回表示のためチャットログに残さない運用（本番分は人が直接発行推奨）。

