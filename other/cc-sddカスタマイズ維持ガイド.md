# cc-sdd カスタマイズ維持ガイド（再生成耐性と再適用チェックリスト）

> **目的:** 本プロジェクトでは cc-sdd（spec-driven development フレームワーク）が生成したテンプレート・スキル・ルールに多数のプロジェクト固有カスタマイズを加えている。これらは **`npx cc-sdd` を再実行（再インストール / バージョン更新）すると上書きで失われうる**。本書はその理由・リスク階層・**再適用に必要な完全な目録**・再生成時の運用手順をまとめ、いつでも安全に復元・再適用できるようにする。
>
> 最終更新: 2026-06-06／対象 cc-sdd: v3 系 skills モード（`npx cc-sdd@latest --claude-skills`）

---

## 0. 一行サマリ

`npx cc-sdd` を**再実行したときに限り**、`.claude/skills/kiro-spec-*/`（SKILL本体＋rules）と `.kiro/settings/templates/specs/*` に加えた編集がインストーラの上書きで消えうる。**steering・フック・メモリは比較的安全**で、かつ**全部 git 管理下なので復元は可能**。本書の第6章が「再適用チェックリスト」。

---

## 1. 用語の具体化

| 用語 | 本プロジェクトでの実体 |
|---|---|
| **machinery（機構）** | cc-sdd が**生成した**ファイル群。具体的には `.kiro/settings/templates/specs/*`（出力テンプレート）と `.claude/skills/kiro-spec-*/`（`SKILL.md` ＋ `rules/*.md`） |
| **upstream（上流）** | npm パッケージ **`cc-sdd`**。`npx cc-sdd@latest --claude-skills` でこのプロジェクトに導入された（v3 系 skills モード） |
| **再生成（regeneration）** | その `npx cc-sdd@latest …` を**もう一度実行**すること。動機例: バージョン更新（v2→v3 等）、入れ直し、別エージェント追加（Cursor/Copilot 等）、チームメンバーのオンボード、壊れたファイルの修復 |

→ 「machinery のカスタマイズが upstream 再生成で失われる」＝ **`npx cc-sdd` を再実行するとインストーラが上記の生成ファイルを書き直し、私たちが加えた編集が上書きで消える**、の意。

---

## 2. なぜ「失われる」と言えるか（cc-sdd 公式挙動の根拠）

`bk/cc-sdd-doc/migration-guide.md`（cc-sdd 公式の移行ガイド）に明記されている。

1. **再インストールは上書きを伴う**
   - インストーラは「ファイル群ごとに『上書き(overwrite)／追記(append)／保持(keep)』を選ぶよう尋ねる」。
   - = overwrite を選ぶ（またはデフォルト/フラグで流す）と、その生成ファイルはパッケージ新版で**置き換わる**。

2. **公式の移行手順 Step 1 が「バックアップ」**
   - `cp -r .kiro .kiro.backup` ／ `cp -r .claude .claude.backup` を最初に実行せよ、とある。
   - わざわざバックアップを促すのは、再生成で上書きされうるから。

3. **Step 3–4 が「テンプレート/rules の再生成 & 差分マージ」「カスタムルールを移植」**
   - 再生成後に**自分のカスタマイズを手で入れ直す**前提になっている。

4. **「変わらないもの」として steering と specs は保持される**
   - `.kiro/steering/`（プロジェクトメモリ）は従来通り読み込まれ、インストーラも keep を選べる。
   - `.kiro/specs/<feature>/` も残る。

→ つまり cc-sdd 自身が「再生成するとカスタマイズは自動では残らない。バックアップして再移植せよ。ただし steering は保持される」と設計・明記している。これが本書の前提の根拠。

---

## 3. リスク階層（どこに置いた編集が、どれだけ消えやすいか）

| リスク | 置き場所 | 再生成時の挙動 |
|:---:|---|---|
| **対象外（消えない）** | `.claude/hooks/*`、`.claude/settings.json`、（プロジェクト外の）AI メモリ | これらは私（人間/アシスタント）が作ったハーネス側のファイルで **cc-sdd の生成対象ではない**。`npx cc-sdd` は触らないので残る。※ただし将来 cc-sdd が settings.json のフック管理を始めた場合は要注意 |
| **低（保持されやすい）** | `.kiro/steering/*.md` | cc-sdd 公式が「変わらないもの」と明言。インストーラも keep が既定的。**最も堅牢な置き場所** |
| **中（overwrite で消える）** | `.kiro/settings/templates/specs/*` | cc-sdd 公認のカスタマイズ面だが、overwrite を選べば新版で置換される |
| **高（丸ごと再生成）** | `.claude/skills/kiro-spec-*/SKILL.md` と `rules/*.md` | スキル束ごと再生成される。さらに cc-sdd 公式は「**SKILL（プロンプト）直接編集は非推奨**。ルール/ステアリングへ寄せよ」としている＝ここの編集が最も脆い |

**設計上の含意:** 同じルールでも「steering に書く」ほど再生成耐性が高く、「SKILL 本文に書く」ほど脆い。本プロジェクトは即効性を優先して SKILL/rules にも直書きしているため、高リスク帯に多くの資産がある（第6章参照）。

---

## 4. 現実的な影響と前提

- **再実行しなければ無風。** これらは普通の git 管理ファイル。`npx cc-sdd` を二度と実行しなければ何も起きない（個人開発はバージョン固定で再実行しないことが多い）。
- **git が安全網。** すべてコミット済みなので、万一上書きされても `git diff` / `git checkout <path>` / `git revert` で復元できる。真のリスクは「再インストール時に黙って消えたのに気づかない」こと。
- **steering・フック・メモリは比較的安全。** HTMX/DB 参照の強制はフック（対象外）＋ steering（低リスク）に二重化されているため、SKILL 本文（高リスク）が消えても核心の「ガイドを参照させる」挙動はある程度残る。

---

## 5. 再生成（再インストール）時の運用手順チェックリスト

`npx cc-sdd` を再実行する必要が生じたときは、以下を順守する。

1. **事前バックアップ（必須）**
   ```bash
   cp -r .kiro   .kiro.backup
   cp -r .claude .claude.backup
   ```
2. **作業ツリーをクリーンに**（未コミット変更があれば先にコミット or stash）。再生成差分だけを後で見分けられるようにする。
3. **再インストール実行**
   ```bash
   npx cc-sdd@latest --claude-skills   # 本プロジェクトのモード
   ```
4. **インストーラのプロンプトで keep / append を選ぶ**（overwrite を避ける）。特に下記は keep 推奨:
   - `.kiro/steering/*`（保持）
   - `.kiro/specs/*`（保持）
   - カスタム済みの `.kiro/settings/templates/specs/*`、`.claude/skills/kiro-spec-*/*`
5. **差分レビュー**
   ```bash
   git status
   git diff
   ```
   上書きされたファイルを特定する。
6. **再適用** — 第6章のチェックリストを使い、各ファイルのカスタマイズが残っているか **grep の「目印」文字列**で確認。消えていれば該当コミットから復元:
   ```bash
   # 例: design.md テンプレのカスタマイズを丸ごと戻す
   git checkout bb3476e -- .kiro/settings/templates/specs/design.md
   ```
   ※新版テンプレに構造変更がある場合は、単純 checkout ではなく**新版へ手で移植**する（cc-sdd 公式の推奨）。
7. **フック有効化の確認** — 新セッションで `.claude/hooks/inject-cc-sdd-refs.sh` が発火するか確認（フックはセッション開始時ロード）。
8. **動作確認** — `/kiro:spec-design` 等を1回流し、(a) HTMX/DB 参照が注入される、(b) design.md に View/Template Contract が出る、(c) tasks が逐次（(P)なし）で出る、を確認。

---

## 6. 【完全版】カスタマイズ目録 ＝ 再適用チェックリスト

本セッション（2026-06-06）で加えた全カスタマイズ。**「目印」= 再生成後にそのファイルへ `grep` して存在を確認する文字列。** 消えていたら「関連コミット」から復元・再移植する。

### A. ハーネス層（リスク: 対象外・消えない）

- [ ] **`.claude/hooks/inject-cc-sdd-refs.sh`**
  - 内容: UserPromptSubmit フック。`/kiro-spec-{requirements,design,quick,tasks}` 実行時に additionalContext で **(A) HTMX実装ガイド参照** と **(B) DBスナップショット参照** を自動注入。常に exit 0（ブロックしない）。
  - 目印: ファイル存在 ＋ `cc-sdd 参照必須リソース`
  - 関連コミット: `73e3a2d`（作成 `inject-htmx-guide-ref.sh`）→ `796928e`（DB 追加・`inject-cc-sdd-refs.sh` へ統合改称）

- [ ] **`.claude/settings.json`**
  - 内容: 上記フックを UserPromptSubmit に登録。
  - 目印: `inject-cc-sdd-refs.sh`
  - 関連コミット: `73e3a2d` / `796928e`

### B. steering 層（リスク: 低・保持されやすい）

- [ ] **`.kiro/steering/tech.md`**
  - 内容: 「## HTMX/templ 動的実装の正典（cc-sdd 必読・落とし穴回避）」節 ＋ 「## DBスキーマ現状の参照（cc-sdd 必読・存在しないカラム/型の防止）」節。強制手段（フック名）も記載。
  - 目印: `HTMX/templ 動的実装の正典` ／ `DBスキーマ現状の参照`
  - 関連コミット: `73e3a2d` / `796928e`

### C. テンプレート層（リスク: 中・overwrite で消える）

- [ ] **`.kiro/settings/templates/specs/design.md`**
  - 内容: Contracts チェックリストに **`View/Template` 種別を新設**＋`##### View / Template Contract` 詳細ブロック追加。`API Contract`→`API (JSON) Contract`（デバイス取込API・ドキュメント限定と明記、例を `/api/v1/ingest` Bearer に）。UIコンポーネント例と Data Transfer を templ 化。Service Interface に `repository.Querier`（唯一の DB ポート）注記。Logical Data Model に no-FK 注記。Testing Strategy を Go（table-driven / httptest / Querier モック）化。
  - 目印: `View / Template Contract` ／ `API (JSON) Contract` ／ `repository.Querier`
  - 関連コミット: `bb3476e`

- [ ] **`.kiro/settings/templates/specs/research.md`**
  - 内容: Architecture Pattern Evaluation の例を `Hexagonal`→`Layered-lite（本プロジェクト採用）`。Sources Consulted に正典（snapshot / HTMXガイド / steering）を追記。
  - 目印: `Layered-lite（本プロジェクト採用）`
  - 関連コミット: `bb3476e`

- [ ] **`.kiro/settings/templates/specs/tasks.md`**
  - 内容: 並列マーカー注記を「**既定では ` (P)` を付けない**（逐次・`--parallel` 時のみ付与）」に変更。
  - 目印: `既定では ` + "` (P)` を付けない"
  - 関連コミット: `08e8ee1`

### D. スキル本体＋ルール層（リスク: 高・丸ごと再生成。cc-sdd 公式は直接編集を非推奨）

- [ ] **`.claude/skills/kiro-spec-requirements/SKILL.md`**
  - 内容: Step1 に「【本プロジェクト固有・必須】HTMX実装ガイド参照」「DBスキーマ現状参照」ブロック（WHAT/HOW 分離付き）。
  - 目印: `本プロジェクト固有・必須】HTMX実装ガイド参照`
  - 関連コミット: `73e3a2d` / `796928e`

- [ ] **`.claude/skills/kiro-spec-design/SKILL.md`**
  - 内容: 上記 HTMX/DB 必須ブロック ＋ Critical Constraints の Type Safety を **Go 第一級**（`repository.Querier` ポート、binding バリデーション）に。
  - 目印: `Go（本プロジェクト）`
  - 関連コミット: `73e3a2d` / `796928e` / `bb3476e`

- [ ] **`.claude/skills/kiro-spec-tasks/SKILL.md`**
  - 内容: HTMX/DB 必須ブロック ＋ **実行モード逐次既定化**（`parallel = (--parallel flag is present)`、既定 sequential）＋ `argument-hint` を `[--parallel]` に ＋ 各サブタスクを `/tdd` 1サイクル粒度・単一責務に。
  - 目印: `parallel = (--parallel flag is present)`
  - 関連コミット: `73e3a2d` / `796928e` / `08e8ee1`

- [ ] **`.claude/skills/kiro-spec-quick/SKILL.md`**
  - 内容: HTMX/DB 必須ブロック ＋ Final Summary の `endpoints`→`routes/handlers` ＋ Phase4 に逐次既定の注記。
  - 目印: `routes/handlers`
  - 関連コミット: `73e3a2d` / `796928e` / `bb3476e` / `08e8ee1`

- [ ] **`.claude/skills/kiro-spec-design/rules/design-principles.md`**
  - 内容: Type Safety（§1）と「Shared Interfaces & Props」を **TS/React 語彙 → Go/templ**（`any`/discriminated unions/`BaseUIPanelProps` を排除し、Go の `error` 値・templ 共通パラメータ struct へ）。Dependency Direction を Layered-lite（structure.md）委譲。Contracts に View/Template を追加。
  - 目印: `Shared UI Component Contracts（templ）`
  - 関連コミット: `bb3476e`

- [ ] **`.claude/skills/kiro-spec-requirements/rules/ears-format.md`**
  - 内容: EARS の例文を EC/車/携帯 → **農業IoT/センサー/アラート/デバイス取込API** ドメインに差し替え。
  - 目印: `sensor reading exceeds its alert threshold`
  - 関連コミット: `bb3476e`

- [ ] **`.claude/skills/kiro-spec-requirements/rules/requirements-review-gate.md`**
  - 内容: Mechanical Check「No implementation language」に、**観測可能なインターフェース種別**（画面表示か機械間 API か、Bearer 認証の有無）は要件に書いてよい旨を追記。
  - 目印: `観測可能なインターフェース種別`
  - 関連コミット: `bb3476e`

- [ ] **`.claude/skills/kiro-spec-tasks/rules/tasks-generation.md`**
  - 内容: Foundation の生成順（goose→sqlc→templ）、Boundary 名（templ/handler/Querier/authz）、Web UI 完了条件を HTMX 観点、OpenAPI を実装タスクに含める旨、**Parallel Analysis を既定無効（逐次）化**、各サブタスクを `/tdd` 1サイクル粒度に。
  - 目印: `Parallel Analysis（本プロジェクトは既定で無効＝逐次）`
  - 関連コミット: `bb3476e` / `08e8ee1`

- [ ] **`.claude/skills/kiro-spec-tasks/rules/tasks-parallel-analysis.md`**
  - 内容: 「本ルールは `--parallel` 時のみ適用。既定は逐次で `(P)` を付けない」注記を冒頭に追加。
  - 目印: `本プロジェクトの既定は逐次（sequential）`
  - 関連コミット: `08e8ee1`

### 一括復元のショートカット

特定コミット時点の machinery を丸ごと戻したい場合:

```bash
# 例: bb3476e（Go最適化）で変更した全テンプレ/rules を当時の内容に戻す
git show --stat bb3476e                 # 対象ファイル確認
git checkout bb3476e -- <path>          # ファイル単位で復元

# 関連コミット一覧（このプロジェクトのカスタマイズ）
#   73e3a2d  HTMX実装ガイド参照の多層強制
#   796928e  DBスキーマ現状を参照必須に追加＋フック統合
#   bb3476e  cc-sdd machinery を Gin+templ+HTMX に最適化
#   08e8ee1  タスク生成を逐次(sequential)既定に変更（1行ずつTDD用）
```

---

## 7. 堅牢化の選択肢（任意・将来）

再生成耐性を上げたい場合の手段（トレードオフあり）:

1. **高リスク帯を steering へ寄せる** — SKILL/rules に直書きしているプロジェクト方針のうち、コマンド非依存のものを `.kiro/steering/*.md` へ移すと保持されやすくなる。ただし steering は全コマンド共通で読まれるため、コマンド固有の細かい手順までは寄せきれない。
2. **カスタマイズをパッチとして保存** — `git format-patch` や `git diff` を `docs/` 配下に保存しておき、再生成後に `git apply` で再適用する。新版テンプレに構造変更があると当たらない点に注意。
3. **再生成しない運用に固定** — `npx cc-sdd@<version>` のようにバージョンを固定し、原則再実行しない。本書＋git で十分回せるなら最も低コスト。

---

## 8. 参考資料

- cc-sdd 公式ドキュメント（バックアップ）: `bk/cc-sdd-doc/`
  - `migration-guide.md` — 再インストール手順・overwrite/append/keep・「変わらないもの」
  - `customization-guide.md` — templates/ と rules/ のカスタマイズ法・「絶対に維持すべき構造」
  - `command-reference.md` / `skill-reference.md` — コマンド/スキル一覧
- 関連 steering: `.kiro/steering/tech.md`（HTMX/templ 正典・DBスキーマ現状の節）
- 関連メモリ（AI 用）: `project_htmx_guide_enforcement` / `project_ccsdd_machinery_go_opt`

---

## 9. 改訂履歴

| 日付 | 内容 |
|---|---|
| 2026-06-06 | 初版作成。HTMXガイド/DBスナップショット参照の多層強制、cc-sdd machinery の Go/templ/HTMX 最適化、タスク逐次既定化までを目録化（コミット 73e3a2d / 796928e / bb3476e / 08e8ee1）。 |
