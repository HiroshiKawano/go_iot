#!/usr/bin/env bash
# ============================================================================
# PreToolUse(Bash) ガードフック (force-push 禁止)
#   git の強制プッシュを permissionDecision: "deny" で完全に拒否する。
#   対象: --force / -f / --force-with-lease / --force-if-includes /
#         単ハイフンの f クラスタ(-fu 等) / +refspec 形式 (git push origin +main)。
#
# 背景:
#   force-push はリモート履歴を破壊し他者のコミットを消し得る不可逆操作のため、
#   利用者の依頼で全面禁止 (2026-06-08)。bash-rm-guard.sh が非rmを allow する中、
#   deny は allow に優先するため本フックで確実にブロックできる
#   (既存 block-source-redirect.sh と同じ仕組み)。
#
# 方針:
#   - コマンドを ; | & で分割し、git push セグメントに force 指定があれば deny。
#   - 判定不能 (jq 失敗・コマンド空) のときは素通し (force でない push を誤って
#     止めない=ここでは「止めすぎない」方向)。force-push の検出に確信があるときだけ deny。
# ============================================================================

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || true)"
[ -z "$cmd" ] && exit 0

deny() {
  reason='force-push (git push --force / -f / --force-with-lease / +refspec) はリモート履歴を破壊する不可逆操作のため禁止です。履歴を書き換えたい場合は新しいブランチへ通常 push するか、人手で実行してください。'
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n' "$reason"
  exit 0
}

# 区切り文字 ; | & を改行へ置換してセグメント分割し、git push セグメントだけを検査する。
segments="$(printf '%s' "$cmd" | tr ';|&' '\n\n\n')"
while IFS= read -r seg; do
  # git push セグメントか (git ... push の形)。
  printf '%s' "$seg" | grep -Eq '(^|[^a-zA-Z0-9._-])git([[:space:]].*)?[[:space:]]push([[:space:]]|$)' || continue
  # force 指定があるか:
  #   --force / --force-with-lease / --force-if-includes / --force=...
  #   単ハイフンの f を含むクラスタ (-f / -fu / -vf 等)
  #   +refspec (git push origin +main の先頭 + 付きトークン)
  if printf '%s' "$seg" | grep -Eq '(--force([-=a-zA-Z]*)?|(^|[[:space:]])-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|$)|[[:space:]]\+[^[:space:]])'; then
    deny
  fi
done <<EOF
$segments
EOF

exit 0
