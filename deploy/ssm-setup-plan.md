# SSM Session Manager 導入 作業準備（B）— SSH(22) を FW から外す

> 状態: **準備のみ（未実行）**。本書は手順・前提・IAM・コスト・トレードオフを文書化したもの。
> 実行は別セッションで、各 mutation の承認（または S-5-2 の書込フェーズ切替/ローカルCLI）を経て行う。
> 出典: AWS 公式ドキュメント（2026-06-25 時点で aws-knowledge MCP により確認）。料金は実行前に最新を再確認すること。

## 0. ゴールと動機

- **目的**: SSH(22) を Lightsail ファイアウォールから**恒久的に閉じる**。IP 許可リスト管理を不要化する。
  - 現状の課題（[[project_aws_deploy_inputs]] 参照）: VPN で egress が `104.234.140.0/24` 内変動、管理元 IP も動的 → 単一 /32 固定運用が破綻。当面は `deploy/redeploy.sh`（A）でオンデマンド開放→復元している。
- **到達点**: アウトバウンドのみで成立する SSM 経由に切り替え、デプロイ時の FW 変更を撤廃する。

## 1. 最重要の制約（採否を分ける）

**Lightsail は EC2 ではない**ため、SSM では「ハイブリッドアクティベーション」で**managed node**（ID 接頭辞 `mi-`）として登録する。

⚠️ **コスト注意（出典: SSM 公式 "enable SSH connections" / 機器種別ドキュメント）**
> 「オンプレミス/VM 等を activated managed node として **Session Manager** で使うには **advanced-instances tier** を使う必要がある」
- advanced-instances tier は**有料**（ノード時間課金）。概算 **約 $0.00695/ノード時間 ≈ 月 ~$5**（**要・最新料金確認**: https://aws.amazon.com/systems-manager/pricing/ ）。
- 単一インスタンスでも Lightsail $7/月 に上乗せ ≈ 実質 1.7 倍。**Session Manager 系（対話シェル/SSH/ポートフォワード）は標準(無料)tierの hybrid node では不可。**
- → 「Session Manager で SSH/SCP（B-1）」を選ぶ場合はこのコストが前提。**コストを避けたい場合は B-2（Run Command + S3）**を検討する。

## 2. 2 つのルート

### ルート B-1: Session Manager 経由の SSH/SCP（現行 scp フローを温存・**advanced tier 必須**）

inbound 22 を閉じたまま、`ssh`/`scp` を SSM の TLS WebSocket トンネルに通す。`redeploy.sh` の scp/ssh ロジックをほぼ流用でき、移行が最小。

- ノード側: SSH デーモンは起動のまま（inbound はFWで閉じる）。SSM Agent **2.3.672.0+**、**advanced-instances tier**。
- ローカル側: **Session Manager プラグイン 1.1.23.0+**、`~/.ssh/config` に ProxyCommand 追加:
  ```
  # SSH over Session Manager
  Host mi-*
      ProxyCommand sh -c "aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters 'portNumber=%p' --profile go-iot-mcp --region ap-northeast-1"
      User ubuntu
  ```
- 接続: `ssh -i ~/.ssh/<key> mi-xxxxxxxx` / `scp ... mi-xxxxxxxx:` （**IP も 22 開放も不要**）。
  - ※ 鍵は managed node に紐づく公開鍵が必要（Lightsail 一時鍵ではなく、`~/.ssh/go-iot-prod.pub` をサーバ `~ubuntu/.ssh/authorized_keys` に登録して使う想定。要設計）。

### ルート B-2: Run Command + S3（**advanced tier 不要**・SSH も不要）

ビルド成果物を S3 に置き、SSM Run Command でインスタンスに「S3 から取得→swap→restart」を実行させる。Session Manager を使わないため**標準 tier で可**（= 追加のノード課金を回避。ただし S3 と Run Command の通常料金）。

- 流れ: ローカル build → `aws s3 cp go_iot_server s3://<bucket>/...` → `aws ssm send-command --document-name AWS-RunShellScript --instance-ids mi-xxxx --parameters 'commands=[...aws s3 cp で取得→/opt/go_iot へ swap→systemctl restart...]'` → `aws ssm get-command-invocation` で結果確認。
- 必要追加: 専用 S3 バケット（非公開）+ ノードIAMロールに `s3:GetObject` 限定 + ローカルユーザーに `s3:PutObject`/`ssm:SendCommand`/`ssm:GetCommandInvocation`。
- 利点: inbound 22 完全閉鎖・SSH 鍵運用も不要・advanced tier 不要。
- 欠点: バイナリが S3 を一度経由（25MB）・cron 的な対話性は無い・スクリプト改修量は B-1 より多い。

## 3. 共通の前提セットアップ（どちらのルートでも必要）

1. **ノード用 IAM サービスロール**（SSM Agent が assume）
   - 信頼ポリシー: `ssm.amazonaws.com` を信頼。
   - アタッチ: `AmazonSSMManagedInstanceCore`（B-2 はこれに S3 取得の最小インラインを追加）。
   - 作成は一度きり（コンソール推奨。CLI なら `iam:CreateRole`/`AttachRolePolicy` が要る）。
2. **ハイブリッドアクティベーション作成**（Activation Code/ID は**作成直後の1回のみ表示・保存必須**）
   ```bash
   aws ssm create-activation \
     --default-instance-name go-iot-prod \
     --iam-role <上記サービスロール名> \
     --registration-limit 1 \
     --region ap-northeast-1 --profile go-iot-mcp
   # → ActivationId / ActivationCode を安全に控える（再取得不可・流出厳禁）
   ```
3. **インスタンスに SSM Agent 導入 + 登録**（Ubuntu 24.04・SSH 経由で一度だけ実施）
   - 公式: hybrid Linux 用 SSM Agent インストール手順に従う（snap ではなく hybrid 用 deb 推奨）。
   - 登録: `sudo amazon-ssm-agent -register -code <ActivationCode> -id <ActivationId> -region ap-northeast-1` → `sudo systemctl enable --now amazon-ssm-agent`。
   - 確認: `aws ssm describe-instance-information --profile go-iot-mcp --region ap-northeast-1`（`mi-xxxx` が `Online` で出る）。
4. **ローカル: Session Manager プラグイン導入**（B-1 のみ必須 / B-2 は不要）
   - `session-manager-plugin --version` が出ること。

## 4. ローカルユーザー（go-iot-mcp）IAM への追加（最小権限・準備）

> 現行インラインポリシー `GoIotMcpLightsail` に SSM 用ステートメントを**追記**する想定（実適用は実行時）。

- 共通: `ssm:DescribeInstanceInformation`
- セットアップ時のみ: `ssm:CreateActivation` + `iam:PassRole`（上記サービスロールを Resource 限定で）
- B-1: `ssm:StartSession`（Resource = `arn:aws:ssm:ap-northeast-1:474025757751:managed-instance/mi-xxxx` と document `AWS-StartSSHSession`）, `ssm:TerminateSession`（`*` か自セッション）
- B-2: `ssm:SendCommand`（document `AWS-RunShellScript` + 当該 `mi-xxxx`）, `ssm:GetCommandInvocation`, `s3:PutObject`（`arn:aws:s3:::<bucket>/*` 限定）
- denyList は維持（破壊系）。`ssm:CreateActivation` 等は elicitList へ入れて確認対象にするか、S-5-2 のローカルCLI運用で実行。

## 5. 切替後に変わること

- **Lightsail FW: 22 を削除**（`put-instance-public-ports` で 80/443 のみへ）。inbound SSH ゼロ。
- `deploy/redeploy.sh` の改修:
  - B-1: egress 検出・FW open/restore・`get-instance-access-details` を撤廃。`STATIC_IP` の代わりに `mi-xxxx` をターゲットにして scp/ssh（ProxyCommand 経由）。
  - B-2: scp/ssh を「S3 アップロード + send-command」に置換。
- 管理元 IP・VPN 変動は**一切無関係**になる（IP 管理の撤廃が達成）。

## 6. 推奨と判断材料

| 観点 | A: redeploy.sh（現行） | B-1: SSM-SSH/SCP | B-2: RunCommand+S3 |
|---|---|---|---|
| 追加月額 | 0 | **~$5（advanced tier）** | ほぼ0（S3/SSM通常） |
| inbound 22 | デプロイ時のみ一時開放 | **常時閉** | **常時閉** |
| 実装コスト | 済 | 小（scp流用） | 中（S3経路） |
| IP管理 | 自動だが都度FW変更 | 不要 | 不要 |

- **少額運用なら A 継続が最もコスト効率が良い**（FW 変更は自動化済み・実害小）。
- **「inbound 22 を恒久的に無くす」ことを優先するなら**: 月額許容なら **B-1**、コスト最優先なら **B-2**。
- 単一の小規模インスタンスである現状では、**B 即時導入の費用対効果は限定的**。実証運用が安定し「IP 管理を二度としたくない/22 を常時閉じたい」要件が固まった時点で B（推奨は B-1）へ移行するのが妥当。

## 7. ロールバック / 解除

- `aws ssm deregister-managed-instance --instance-id mi-xxxx`（managed node 解除）。
- advanced tier はノード解除で課金停止。S3 バケット・IAM 追加分は手動削除。
- FW で 22 を管理元 /32 に戻せば A 運用へ即復帰可能。

## 8. 実行チェックリスト（B 着手時にここから）

- [ ] ルート決定（B-1 / B-2）と月額コスト承認
- [ ] ノード用 IAM サービスロール作成（`AmazonSSMManagedInstanceCore`）
- [ ] `create-activation` 実行 → Code/ID 安全保管
- [ ] サーバへ SSM Agent 導入 + register + enable（SSH 経由・一度きり）
- [ ] `describe-instance-information` で `mi-xxxx` Online 確認
- [ ] （B-1）advanced-instances tier 有効化 + Session Manager プラグイン + `~/.ssh/config`
- [ ] （B-2）S3 バケット作成 + ノード/ローカル IAM 追加
- [ ] go-iot-mcp IAM に SSM ステートメント追加
- [ ] 疎通（B-1: `ssh mi-xxxx` / B-2: テスト send-command）
- [ ] `redeploy.sh` を SSM 版へ改修・検証
- [ ] **最後に** Lightsail FW から 22 を削除（80/443 のみ）
