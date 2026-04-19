# Backlog SSH設定 & リポジトリクローン手順

作業日: 2026-02-22

---

## 概要

BacklogのGitリポジトリにSSH接続し、ローカルにクローンするまでの手順を記録する。

---

## 1. SSH公開鍵の確認

既存のSSH鍵ペアがあるか確認した。

```bash
ls ~/.ssh/*.pub
```

**結果:** `~/.ssh/id_rsa.pub` が存在することを確認。

---

## 2. SSH公開鍵をクリップボードにコピー

```bash
pbcopy < ~/.ssh/id_rsa.pub
→ Windowsでは clip < ~/.ssh/id_rsa.pub が同等のコマンド。
```

---

## 3. BacklogへのSSH公開鍵登録

1. Backlogにログイン
2. 右上のアカウントメニュー → **個人設定** を開く
3. 左メニューの **SSH公開鍵** を選択
4. 「SSH公開鍵」欄に `Cmd+V` で貼り付け
5. 「メモ」欄に任意の名前を入力（例: `Mac`）
6. **「登録」** ボタンをクリック

> **登録URL:** `https://kdcsystem.backlog.com/EditUserSshKey.action`

---

## 4. リポジトリのクローン

クローン先ディレクトリ: `/Users/c/Desktop/dev/farm_iot/backup/kdcs_erp`

使用したSSH URL:
```
kdcsystem@kdcsystem.git.backlog.com:/KDCS_PRJ/kdcs_erp.git
```

---

## 発生した問題と解決方法

### 問題1: `Host key verification failed`

**エラー内容:**
```
Host key verification failed.
fatal: Could not read from remote repository.
```

**原因:**
`~/.ssh/known_hosts` にBacklogのホスト鍵が登録されていなかった。

**試みた解決策（失敗）:**
`ssh-keyscan` コマンドでホスト鍵を取得しようとしたが、以下のエラーが発生し失敗した。

```bash
ssh-keyscan kdcsystem.git.backlog.com >> ~/.ssh/known_hosts
# → ssh-keyscan: fdlim_get: bad value
```

**最終的な解決方法:**
`GIT_SSH_COMMAND` に `StrictHostKeyChecking=accept-new` オプションを指定し、初回接続時にホスト鍵を自動承認・登録した。

```bash
GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=accept-new" \
  git clone kdcsystem@kdcsystem.git.backlog.com:/KDCS_PRJ/kdcs_erp.git \
  /Users/c/Desktop/dev/farm_iot/backup/kdcs_erp
```

このコマンドにより `~/.ssh/known_hosts` にBacklogのホスト鍵（ED25519）が自動追加され、クローンが成功した。

---

## 5. クローン結果

以下のファイル・ディレクトリが取得できた。

```
backup/kdcs_erp/
├── Makefile
├── README.md
├── backend/
├── buildspec.yml
├── buildspec_build.sh
├── docker-compose.yml
└── infra/
```

---

## 次回以降のクローン方法

一度ホスト鍵が登録されたため、次回からは通常のクローンコマンドで接続可能。

```bash
git clone kdcsystem@kdcsystem.git.backlog.com:/KDCS_PRJ/kdcs_erp.git
```

---

## 初心者向け用語説明集

---

### SSH（Secure Shell）とは

SSHとは、ネットワーク越しに**別のコンピュータへ安全に接続するための仕組み**のこと。
通信内容が暗号化されているため、第三者に通信内容を盗み見られる心配がない。

GitHubやBacklogなどのサービスでは、コードをやり取りする際にSSH接続を使うことが多い。

---

### 公開鍵・秘密鍵（キーペア）

SSHでは「**鍵のペア**」を使って本人確認を行う。南京錠と鍵のような関係。

| 種類 | ファイル名の例 | 役割 |
|---|---|---|
| **秘密鍵** | `~/.ssh/id_rsa` | 自分だけが持つ鍵。絶対に他人に渡してはいけない |
| **公開鍵** | `~/.ssh/id_rsa.pub` | 南京錠にあたるもの。サービス側に登録する |

**仕組みのイメージ:**
```
自分のPC                 Backlogサーバー
  秘密鍵    ──→公開鍵 （登録済み）
 （手元の鍵） 認証←──  （サーバーの南京錠）
一致したら接続OK！
```

- `.pub` がついているファイルが**公開鍵**（Public Key）
- `.pub` がついていないファイルが**秘密鍵**（Private Key）

> **重要:** 秘密鍵（`id_rsa`）は絶対に他人に見せない・送らない。
> Backlogに登録するのは必ず公開鍵（`id_rsa.pub`）だけ。

---

### ホスト鍵（Host Key）とは

**接続先のサーバーが本物かどうかを確認するための鍵**。

公開鍵・秘密鍵が「自分の身分証明」なら、ホスト鍵は「相手サーバーの身分証明」にあたる。

初めてSSH接続するサーバーには「このサーバーを信頼しますか？」と聞かれる。
信頼すると、そのサーバーのホスト鍵が `~/.ssh/known_hosts` に保存される。

```
~/.ssh/
├── id_rsa           ← 自分の秘密鍵
├── id_rsa.pub       ← 自分の公開鍵
└── known_hosts      ← 信頼済みサーバーのホスト鍵一覧
```

次回以降の接続では `known_hosts` の内容と照合し、一致すれば接続を許可する。
一致しない場合はなりすまし（中間者攻撃）の可能性があるとして接続を拒否する。

---

### 暗号化アルゴリズム（ED25519・RSA）

今回のログに `ED25519` という文字が出てきた。これは鍵の**暗号化方式**の種類。

| 方式 | 特徴 |
|---|---|
| **RSA** | 古くからある方式。鍵ファイルが `id_rsa` 。互換性が高い |
| **ED25519** | 新しい方式。より安全で鍵が短い。現在の推奨 |

今回は自分の鍵がRSA（`id_rsa`）で、Backlogサーバーのホスト鍵がED25519だった。

---

### `~/.ssh/` ディレクトリ

`~` はホームディレクトリ（`/Users/ユーザー名/`）を指す省略記号。
`.ssh` はSSH関連ファイルをまとめる隠しフォルダ（`.` から始まるフォルダは隠しフォルダ）。

```bash
ls ~/.ssh/
# id_rsa         秘密鍵
# id_rsa.pub     公開鍵
# known_hosts    信頼済みサーバー一覧
```

---

### 使用したコマンド詳細

#### `ls ~/.ssh/*.pub`

```bash
ls ~/.ssh/*.pub
```

| 部分 | 意味 |
|---|---|
| `ls` | ファイル一覧を表示するコマンド（list） |
| `~/.ssh/` | SSHフォルダのパス |
| `*.pub` | `.pub` で終わるファイルすべて（`*` はワイルドカード） |

→ 公開鍵ファイルが存在するか確認するために使った。

---

#### `pbcopy < ~/.ssh/id_rsa.pub`

```bash
pbcopy < ~/.ssh/id_rsa.pub
```

| 部分 | 意味 |
|---|---|
| `pbcopy` | Mac専用コマンド。クリップボードにコピーする |
| `<` | ファイルの内容をコマンドへ渡す（リダイレクト） |
| `~/.ssh/id_rsa.pub` | コピー元の公開鍵ファイル |

→ 公開鍵の内容をコピーして、Backlogの画面に貼り付けるために使った。
→ Windowsでは `clip < ~/.ssh/id_rsa.pub` が同等のコマンド。

---

#### `ssh-keyscan`（今回は失敗したコマンド）

```bash
ssh-keyscan kdcsystem.git.backlog.com >> ~/.ssh/known_hosts
```

| 部分 | 意味 |
|---|---|
| `ssh-keyscan` | 指定したサーバーのホスト鍵を取得するコマンド |
| `>>` | 出力をファイルに**追記**する（`>` は上書き、`>>` は追記） |
| `~/.ssh/known_hosts` | 追記先のファイル |

→ サーバーのホスト鍵を手動で `known_hosts` に追加しようとしたが、環境の問題で失敗した。

---

#### `GIT_SSH_COMMAND`

```bash
GIT_SSH_COMMAND="ssh -o StrictHostKeyChecking=accept-new" git clone ...
```

**環境変数**とは、コマンドに設定を渡す仕組み。
`GIT_SSH_COMMAND` はGitがSSH接続する際に使うコマンドをカスタマイズできる特別な環境変数。

| 部分 | 意味 |
|---|---|
| `GIT_SSH_COMMAND="..."` | GitのSSH接続に使うコマンドを上書き指定する |
| `ssh` | SSH接続コマンド本体 |
| `-o` | オプション（option）を指定するフラグ |
| `StrictHostKeyChecking=accept-new` | 未知のホスト鍵は自動承認して `known_hosts` に追加する |

`StrictHostKeyChecking` の設定値の違い:

| 値 | 動作 |
|---|---|
| `yes`（デフォルト） | 未知のホストには接続しない（安全） |
| `accept-new` | 初めてのホストは自動承認、既知のホストが変わった場合は拒否 |
| `no` | すべて自動承認（危険なため本番環境では非推奨） |

→ 今回は `accept-new` を使い、安全性を保ちながらホスト鍵を自動登録した。

---

#### `git clone`

```bash
git clone kdcsystem@kdcsystem.git.backlog.com:/KDCS_PRJ/kdcs_erp.git /path/to/dest
```

| 部分 | 意味 |
|---|---|
| `git clone` | リモートリポジトリをローカルにコピー（複製）するコマンド |
| `kdcsystem@` | Backlogへのログインユーザー名 |
| `kdcsystem.git.backlog.com` | 接続先のサーバーアドレス（ホスト名） |
| `:/KDCS_PRJ/kdcs_erp.git` | サーバー上のリポジトリのパス |
| `/path/to/dest` | ローカルに保存するフォルダのパス（省略すると現在地に作成） |

---

### SSH URLの読み方

```
kdcsystem@kdcsystem.git.backlog.com:/KDCS_PRJ/kdcs_erp.git
│          │                          │
│          │                          └─ サーバー上のリポジトリパス
│          └─ 接続先サーバーのアドレス
└─ ログインユーザー名
```

GitHubの場合は `git@github.com:ユーザー名/リポジトリ名.git` という形式。
Backlogの場合はユーザー名とサーバーアドレスが独自の形式になっている。

---

### HTTPS URL vs SSH URL

リポジトリへの接続方法は2種類ある。

| 方式 | URL例 | 認証方法 | 特徴 |
|---|---|---|---|
| **HTTPS** | `https://kdcsystem.backlog.com/git/...` | ID・パスワード | 設定が簡単 |
| **SSH** | `kdcsystem@kdcsystem.git.backlog.com:/...` | 公開鍵認証 | 毎回パスワード不要で便利 |

今回はSSH方式を使ったため、一度公開鍵を登録すれば以降はパスワード入力不要でgit操作できる。
