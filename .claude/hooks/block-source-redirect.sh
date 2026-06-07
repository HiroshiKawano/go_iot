#!/usr/bin/env bash
# ============================================================================
# PreToolUse(Bash) ガードフック
#   追跡ソース配下 (internal/ ・ cmd/) へのシェルリダイレクト (> / >>) を拒否する。
#
# 背景:
#   監査中のサブエージェントが `cat /tmp/x >> internal/handler/device_test.go` を
#   実行し、既存ファイル末尾に 2 つ目の `package handler` を追記してコンパイル不能化
#   させた事故があった。`Bash(cat:*)` が allowlist 済みのため承認なしで通ってしまう。
#   権限ルール (前方一致) では「cat は許可するが >> internal/ は拒否」を表現できない
#   ため、本フックで「リダイレクト先が追跡ソースのパス」のときだけ拒否する。
#
# 方針:
#   - ソースの編集・新規生成は Edit / Write ツール、生成物は templ generate / make を使う。
#   - 2>&1 等の fd 複製、/tmp への出力、引数に internal/ を含むだけのコマンドは対象外。
#   - 判定不能 (jq 失敗等) のときは安全側に倒さず素通し (false positive で開発を止めない)。
# ============================================================================

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || true)"

# > または >> のリダイレクト先が internal/ あるいは cmd/ で始まるパス (相対・絶対とも) を
# 指す場合に一致する。path セグメント境界 (先頭 or `/` 直後) の internal//cmd/ のみ拾い、
# `somecmd/` のような部分一致は除外する。`;` `|` `&` でトークンを区切り 2>&1 等を弾く。
pattern='>>?[[:space:]]*(\.?/)?([^[:space:];|&]*/)?(internal|cmd)/'

if printf '%s' "$cmd" | grep -Eq "$pattern"; then
  reason='追跡ソース(internal/・cmd/)へのシェルリダイレクト(> / >>)は禁止です。ソースの編集・新規作成は Edit/Write ツール、生成物は templ generate / make を使ってください。これは cat >> による device_test.go 破損(package 宣言重複)の再発防止ガードです。一時ファイルが必要なら /tmp 配下へ出力してください。'
  # PreToolUse の deny を JSON で返す。reason はサニタイズ済みの固定文言。
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$reason"
  exit 0
fi

exit 0
