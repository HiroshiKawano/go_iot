#!/usr/bin/env bash
# ============================================================================
# PreToolUse(Bash) 自動承認フック (rm ガード)
#   削除コマンド rm を含むかどうかで承認方針を切り替える。
#     - rm を含まない                  -> 自動承認 (permissionDecision: "allow")
#     - rm を含むが対象が /tmp 配下のみ -> 自動承認 ("allow")
#     - それ以外の rm                   -> 確認 ("ask")
#
# 背景:
#   読み取り系・ビルド系コマンドの逐次承認が業務の妨げになっていたため、
#   利用者の明示的依頼により「rm 以外は一括自動承認」に切り替える。
#   ただし一時ファイル(/tmp)の掃除だけは摩擦が出ないよう自動承認に残す。
#
# 安全に関する注意:
#   - 既存 block-source-redirect.sh が "deny" を返す場合は deny が allow に
#     優先するため、追跡ソース(internal/・cmd/)へのリダイレクト禁止ガードは
#     本フックと併存しても引き続き有効。
#   - 旧 allowlist の `rm -rf /tmp/:*` は前方一致のため
#     `rm -rf /tmp/foo; rm /etc/x` のような複合コマンドも素通ししてしまう穴が
#     あった。本フックはコマンドを区切り(; | &)でセグメント分割し、rm を含む
#     全セグメントが /tmp 限定であることを厳格パターンで検証する。1 つでも
#     /tmp 限定でない rm があれば確認に倒す。
#   - 判定不能 (jq 失敗・コマンド空) のときは安全側に倒し "ask" を返す。
#   - クォート付き tmp パスや `..` を含む対象、rm への出力リダイレクト等は
#     誤判定を避けるため確認 (ask) に倒す (false negative は許容、安全方向)。
# ============================================================================

input="$(cat)"
cmd="$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || true)"

emit() {
  # $1: allow|ask, $2: 理由 (JSON 文字列を壊す " や \ を含めないこと)
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"%s","permissionDecisionReason":"%s"}}\n' "$1" "$2"
  exit 0
}

# コマンドが取得できなければ確認に倒す。
[ -z "$cmd" ] && emit ask "コマンド内容を判定できないため確認します。"

# rm をコマンドトークンとして含むか (語境界判定)。
# 先頭または非識別子文字(空白 / ; | & 等)の直後の rm で、後続が空白か行末のもの。
# terraform / format / confirm / perm / chmod 等の部分一致は除外する。大小無視(-i)。
rm_re='(^|[^a-zA-Z0-9._-])rm([[:space:]]|$)'
if ! printf '%s' "$cmd" | grep -Eiq "$rm_re"; then
  emit allow "削除(rm)を含まないため自動承認しました。"
fi

# ここから rm を含むケース。全 rm セグメントが /tmp 限定か検証する。
# 区切り文字 ; | & を改行へ置換してセグメント分割 (&& も || も & | に含まれる)。
# クォート内区切りでも過剰分割になるだけで安全側に倒れる。
safe=1
segments="$(printf '%s' "$cmd" | tr ';|&' '\n\n\n')"
while IFS= read -r seg; do
  # rm を含まないセグメントは rm 安全性に無関係なので無視。
  printf '%s' "$seg" | grep -Eiq "$rm_re" || continue
  # パストラバーサル(..)を含む rm は確認に倒す。
  if printf '%s' "$seg" | grep -Eq '\.\.'; then safe=0; break; fi
  # 厳格パターン: [空白] [/bin/|/usr/bin/]rm [空白] (オプション)* (/tmp ターゲット)+ [行末]
  #   ターゲットは全て /tmp/ または /private/tmp/ で始まること。
  if ! printf '%s' "$seg" | grep -Eq '^[[:space:]]*(/usr/bin/|/bin/)?rm[[:space:]]+(-[-A-Za-z]+[[:space:]]+)*((/tmp/|/private/tmp/)[^[:space:]]*[[:space:]]*)+$'; then
    safe=0; break
  fi
done <<EOF
$segments
EOF

if [ "$safe" -eq 1 ]; then
  emit allow "rm の対象が /tmp 配下のみのため自動承認しました。"
fi
emit ask "rm を含み /tmp 限定でないため確認が必要です。"
