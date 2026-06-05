#!/usr/bin/env bash
# ============================================================================
# UserPromptSubmit フック
#   /kiro-spec-{requirements,design,quick,tasks} 実行時に、
#   2cc_sdd/HTMX実装ガイド(動的).md の参照を必須化する追加コンテキストを注入する。
#
#   目的: cc-sdd（spec-driven development）で requirements / design / tasks を
#   生成する際、templ + HTMX + Alpine.js の既知の落とし穴を回避するため、
#   ガイドの該当節を確実に参照させる（多層強制の「ハーネス層」）。
#
#   設計: ガイドは約288KBあり丸読みは非現実的なので、注入文では
#   「冒頭の cc-sdd参照ガイド索引 → 対象画面の該当★★★節のみ」を読ませる。
#
#   入力 : stdin に UserPromptSubmit イベントの JSON（.prompt に生プロンプト）
#   出力 : マッチ時のみ hookSpecificOutput.additionalContext を JSON で返す
#   方針 : 決してブロックしない（exit 2 を使わない）。常に exit 0。
# ============================================================================

# stdin を読み、.prompt を取り出す（JSON 不正でも落とさない）
input="$(cat)"
prompt="$(printf '%s' "$input" | jq -r '.prompt // ""' 2>/dev/null)"

# 対象コマンドにマッチするか判定（先頭の空白を許容、コマンド名の直後は空白か行末）
if printf '%s' "$prompt" | grep -qiE '^[[:space:]]*/kiro-spec-(requirements|design|quick|tasks)([[:space:]]|$)'; then
  context="$(cat <<'CTX'
【必須・本プロジェクト固有】HTMX実装ガイド参照ルール（cc-sdd 既知の落とし穴回避）

templ + HTMX + Alpine.js の動的実装には既知の落とし穴が多数あり、
`2cc_sdd/HTMX実装ガイド(動的).md` に集約されている。
本コマンドで requirements / design / tasks を生成する前に、必ず次を実施すること:

1. まず同書冒頭の `## cc-sdd参照ガイド`（優先度★付きセクション索引・約60行）を読む。
2. 対象画面の `2cc_sdd/spec-init-prompts/session-*.md` または
   `.kiro/specs/{feature}/brief.md` が参照すべき節を行番号付きで列挙していれば、その節を読む。
3. 列挙がなければ、索引から対象画面に該当する ★★★ 節を選んで読む
   （§2 モック→templ+HTMX 変換ルール / templ コンポーネント分割 / 命名規約、
    §3 id属性一覧、§4 画面別HTMX操作仕様、§7 バリデーションエラー表示、§8 CSRF）。
   Tom Select を使う画面は §16・C12 も読む。
4. ガイド全体（約288KB）は丸読みしない。索引 → 該当節に絞ること。

- requirements フェーズ: 「ユーザーに見える振る舞い・境界」
  （どの操作が HTMX 部分更新か / フルページ遷移か、バリデーション表示方式が体験に与える影響）
  の把握に留め、実装詳細は requirements に持ち込まない（WHAT/HOW 分離を維持）。
- design / tasks フェーズ: ガイドの該当節を、設計判断・タスク粒度
  （部分コンポーネント返却、OOB エンドポイント、Tom Select 再初期化、422+部分返却 等）
  の根拠として明示的に使う。
CTX
)"
  jq -n --arg ctx "$context" \
    '{hookSpecificOutput:{hookEventName:"UserPromptSubmit",additionalContext:$ctx}}'
fi

exit 0
