#!/usr/bin/env bash
# ============================================================================
# UserPromptSubmit フック
#   /kiro-spec-{requirements,design,quick,tasks} および /tdd・/kiro-impl 実行時に、
#   cc-sdd で requirements / design / tasks の生成・実装の際に必ず参照すべき
#   「権威ある資料」への参照を必須化する追加コンテキストを注入する。
#
#   注入する3資料:
#     (A) 2cc_sdd/HTMX実装ガイド(動的).md  … templ+HTMX+Alpine.js の落とし穴回避
#     (B) docs/database_snapshot/          … 存在しないカラム/型の選択を防ぐ現状スキーマ
#     (C) 2cc_sdd/テストガイダンス集.md     … Go(Gin/templ/HTMX/sqlc) テストの定石・落とし穴
#
#   設計: HTMXガイドは約288KBあり丸読み非現実的なので「冒頭の索引→該当節のみ」、
#   DBスナップショットは約190行と小さいので「全読み可・実在するものに限定」を指示する。
#
#   入力 : stdin に UserPromptSubmit イベントの JSON（.prompt に生プロンプト）
#   出力 : マッチ時のみ hookSpecificOutput.additionalContext を JSON で返す
#   方針 : 決してブロックしない（exit 2 を使わない）。常に exit 0。
# ============================================================================

# stdin を読み、.prompt を取り出す（JSON 不正でも落とさない）
input="$(cat)"
prompt="$(printf '%s' "$input" | jq -r '.prompt // ""' 2>/dev/null)"

# 対象コマンドにマッチするか判定（先頭の空白を許容、コマンド名の直後は空白か行末）
if printf '%s' "$prompt" | grep -qiE '^[[:space:]]*/(kiro-spec-(requirements|design|quick|tasks)|kiro-impl|tdd)([[:space:]]|$)'; then
  context="$(cat <<'CTX'
【必須・本プロジェクト固有】cc-sdd 参照必須リソース（既知の落とし穴回避）

本コマンドの実行フェーズ（要件/設計/タスク生成、または /tdd・/kiro-impl の実装）に応じて、以下の権威ある資料を必ず参照すること（フェーズ別の使い分けは末尾参照）。

────────────────────────────────
(A) HTMX実装ガイド — templ+HTMX+Alpine.js の落とし穴回避
────────────────────────────────
正典: `2cc_sdd/HTMX実装ガイド(動的).md`（約288KB）
1. まず冒頭の `## cc-sdd参照ガイド`（優先度★付き索引・約60行）を読む。
2. 対象画面の `2cc_sdd/spec-init-prompts/session-*.md` / `.kiro/specs/{feature}/brief.md` が
   参照節を行番号付きで列挙していれば、その節を読む。
3. なければ索引から該当 ★★★ 節を読む（§2 変換ルール/templ分割/命名規約、§3 id属性一覧、
   §4 画面別HTMX操作仕様、§7 バリデーション、§8 CSRF）。Tom Select 画面は §16・C12。
4. ガイド全体（約288KB）の丸読み禁止。索引 → 該当節に絞ること。

────────────────────────────────
(B) DBスキーマ現状 — 存在しないカラム/型の選択を防ぐ
────────────────────────────────
正典: `docs/database_snapshot/table_definitions.md`（約190行・全読み可）
      ＋ `docs/database_snapshot/er_diagram.mmd`（論理リレーション）
- これが**権威ある現状スキーマ**。テーブル・カラム・型・NULL・デフォルト・索引・
  CHECK 制約（enum 許容値）が記載されている。
- 設計・タスクで参照するテーブル/カラム/型は、必ず本ファイルに**実在する**ものに限る。
  スナップショットに無いカラム・型・テーブルを勝手に発明しない。
- enum 的な値は CHECK 制約の許容リストに従う
  （例: metric=temperature/humidity、operator=>,<,>=,<=）。
- 新規カラム/型/テーブルが必要な場合は、それを既存前提にせず
  **migration 追加（db/migrations/）を明示的なタスク/設計判断として記述**する
  （変更後の再生成は `make db-snapshot`）。
- スナップショットは自動生成（手動編集しない）。マイグレーション変更後は要再生成。

────────────────────────────────
(C) テストガイダンス集 — Go(Gin+templ+HTMX+sqlc/pgx) テストの落とし穴と定石
────────────────────────────────
正典: `2cc_sdd/テストガイダンス集.md`（全50節）
- Go テストの定石と既知の落とし穴を集約: Querier 手書きモックで DB 非依存検証、httptest+gin、
  templ は Render→bytes.Buffer→strings.Contains、gorilla/csrf は GET→トークン往復＋dev は
  csrf.PlaintextHTTPRequest、scs を sm.Load(ctx,"") で in-memory 検証、go-playground/validator 単体、
  カバレッジ80%設計、ユーザー列挙防止、302/303 使い分け 等。
1. まず冒頭の `## Go テーマ別索引` を読み、対象テーマ（DB / HTTP / templ / HTMX /
   認証・認可・CSRF / バリデーション / クライアントサイド / CRUD・CSV / データ整合性）の節に絞る。
2. 約370KB の丸読み禁止。索引 → 該当節に絞ること。

- requirements フェーズ: (A)(B) のみ。「ユーザー観測可能な振る舞い・境界」と「実在するデータ項目の範囲」の
  把握に留め、カラム/型の選定や実装詳細・テスト方式は持ち込まない（WHAT/HOW 分離）。(C) は不要。
- design / tasks フェーズ: (A) は設計判断・タスク粒度の根拠、(B) はデータモデル設計の現状制約、
  (C) は design の Testing Strategy 導出と tasks のテストタスク粒度（/tdd 1サイクルで書ける単位）の根拠。
- implementation フェーズ（/tdd・/kiro-impl）: (C) を第一参照に RED→GREEN でテストを書く。
  (A) は templ/HTMX 実装の落とし穴、(B) はクエリ/カラムの整合に用いる。
CTX
)"
  jq -n --arg ctx "$context" \
    '{hookSpecificOutput:{hookEventName:"UserPromptSubmit",additionalContext:$ctx}}'
fi

exit 0
