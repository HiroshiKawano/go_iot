#!/bin/bash
# DBスナップショット (テーブル定義 Markdown + Mermaid ER図) を一括更新する。
#
# 目的: 実DBに接続しなくても、生成済みファイルを読むだけで
#       テーブル・カラム・制約・リレーションを把握できるようにする
#       (AI エージェント・新規参入者向けのドキュメント資産)。
#
# 前提: PostgreSQL が起動済み (make up) かつマイグレーション適用済み (make migrate-up)。
#       スキーマを変更したら本スクリプトを再実行してスナップショットを更新すること。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${PROJECT_ROOT}/docs/database_snapshot"

cd "$PROJECT_ROOT"

# .env から DATABASE_URL を読み込む (make 経由でない直接実行にも対応)。
set -a
# shellcheck disable=SC1091
[ -f .env ] && . ./.env
set +a

if [ -z "${DATABASE_URL:-}" ]; then
  echo "エラー: DATABASE_URL が未設定です (.env を作成してください)" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

echo "=== DBスナップショット更新 ==="

echo "[1/2] table_definitions.md を生成中..."
go run ./cmd/db-snapshot -format=markdown \
  > "${OUTPUT_DIR}/table_definitions.md"

echo "[2/2] er_diagram.mmd を生成中..."
go run ./cmd/db-snapshot -format=mermaid \
  > "${OUTPUT_DIR}/er_diagram.mmd"

echo "=== 完了: ${OUTPUT_DIR}/ ==="
