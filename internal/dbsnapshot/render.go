package dbsnapshot

import (
	"fmt"
	"strings"
)

// snapshotNote はスナップショット両形式の冒頭に付ける共通注記。
const snapshotNote = "> このファイルは `make db-snapshot` で自動生成される。手動編集しない（スキーマ変更後に再生成すること）。\n" +
	"> 実DBへ接続しなくても、本ファイルを読むだけでテーブル・カラム・制約・リレーションを把握できることを目的とする。\n" +
	"> ※ 外部キー制約は張らない方針（参照整合性はアプリ層で担保）。Mermaid の関連は `<table>_id` 命名から推論した論理リレーション。"

// RenderMarkdown は Schema を人間/AI 可読な Markdown テーブル定義に整形する。
func RenderMarkdown(s *Schema) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# DBスナップショット — テーブル定義\n\n")
	fmt.Fprintf(&b, "%s\n\n", snapshotNote)
	fmt.Fprintf(&b, "**テーブル数:** %d\n\n", len(s.Tables))

	// 目次
	fmt.Fprintf(&b, "## 目次\n\n")
	for _, t := range s.Tables {
		fmt.Fprintf(&b, "- [%s](#%s)%s\n", t.Name, t.Name, commentSuffix(t.Comment))
	}
	b.WriteString("\n---\n\n")

	for _, t := range s.Tables {
		renderTableMarkdown(&b, t)
	}

	// 論理リレーション一覧
	rels := s.Relationships()
	if len(rels) > 0 {
		b.WriteString("## 論理リレーション\n\n")
		b.WriteString("外部キー制約は張らないため、以下は `<table>_id` カラム名から推論した参照関係。\n\n")
		b.WriteString("| 子テーブル | カラム | → | 親テーブル |\n")
		b.WriteString("|------------|--------|---|------------|\n")
		for _, r := range rels {
			fmt.Fprintf(&b, "| %s | %s | → | %s |\n", r.FromTable, r.FromCol, r.ToTable)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderTableMarkdown(b *strings.Builder, t Table) {
	fmt.Fprintf(b, "## %s\n\n", t.Name)
	if t.Comment != "" {
		fmt.Fprintf(b, "%s\n\n", t.Comment)
	}

	b.WriteString("| カラム | 型 | NULL | デフォルト | 説明 |\n")
	b.WriteString("|--------|----|------|-----------|------|\n")
	for _, c := range t.Columns {
		null := "NO"
		if c.Nullable {
			null = "YES"
		}
		desc := c.Comment
		if c.IsPK {
			desc = strings.TrimSpace("PK " + desc)
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n",
			c.Name, mdCell(c.Type), null, mdCell(c.Default), mdCell(desc))
	}
	b.WriteString("\n")

	if len(t.Indexes) > 0 {
		b.WriteString("**索引**\n\n")
		for _, idx := range t.Indexes {
			fmt.Fprintf(b, "- `%s`: `%s`\n", idx.Name, idx.Def)
		}
		b.WriteString("\n")
	}

	if len(t.Checks) > 0 {
		b.WriteString("**CHECK 制約**\n\n")
		for _, ck := range t.Checks {
			fmt.Fprintf(b, "- `%s`: `%s`\n", ck.Name, ck.Expr)
		}
		b.WriteString("\n")
	}

	b.WriteString("---\n\n")
}

// RenderMermaid は Schema を Mermaid の erDiagram に整形する。
func RenderMermaid(s *Schema) string {
	var b strings.Builder

	// erDiagram をファイル先頭に置く (Mermaid はダイアグラム宣言を先頭に要求する)。
	// 注記は %% 行コメントとして宣言直後に置く (レンダリングされない)。
	b.WriteString("erDiagram\n")
	b.WriteString("%% DBスナップショット — ER図 (Mermaid)。make db-snapshot で自動生成。手動編集しない。\n")
	b.WriteString("%% 外部キー制約は張らない方針のため、関連は <table>_id 命名からの推論（論理リレーション）。\n")

	// エンティティ (テーブル + カラム)
	for _, t := range s.Tables {
		fmt.Fprintf(&b, "    %s {\n", t.Name)
		fkCols := relationshipColumns(s, t.Name)
		for _, c := range t.Columns {
			marker := ""
			switch {
			case c.IsPK:
				marker = "PK"
			case fkCols[c.Name]:
				marker = "FK"
			}
			line := fmt.Sprintf("        %s %s", mermaidType(c.Type), c.Name)
			if marker != "" {
				line += " " + marker
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("    }\n")
	}

	// リレーション (親 ||--o{ 子 : ラベル)
	for _, r := range s.Relationships() {
		fmt.Fprintf(&b, "    %s ||--o{ %s : %s\n", r.ToTable, r.FromTable, r.FromCol)
	}

	return b.String()
}

// relationshipColumns はテーブル table が持つ「FK相当 (_id 推論)」カラム集合を返す。
func relationshipColumns(s *Schema, table string) map[string]bool {
	cols := map[string]bool{}
	for _, r := range s.Relationships() {
		if r.FromTable == table {
			cols[r.FromCol] = true
		}
	}
	return cols
}

// mermaidType は PostgreSQL の型名を Mermaid の属性型トークン
// (空白・括弧を含まない単一語) に正規化する。
func mermaidType(pgType string) string {
	base := pgType
	if i := strings.IndexByte(base, '('); i >= 0 { // 精度・長さ (255 等) を除去
		base = base[:i]
	}
	base = strings.TrimSpace(base)

	switch base {
	case "character varying":
		return "varchar"
	case "character":
		return "char"
	case "timestamp with time zone":
		return "timestamptz"
	case "timestamp without time zone":
		return "timestamp"
	case "time with time zone":
		return "timetz"
	case "double precision":
		return "double"
	}
	// それ以外で空白が残る型は安全側でアンダースコア化。
	return strings.ReplaceAll(base, " ", "_")
}

// mdCell は Markdown 表セル用にエスケープする (パイプと改行を無害化)。空なら "-"。
func mdCell(s string) string {
	if s == "" {
		return "-"
	}
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func commentSuffix(comment string) string {
	if comment == "" {
		return ""
	}
	return " — " + comment
}
