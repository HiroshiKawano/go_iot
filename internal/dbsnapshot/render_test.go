package dbsnapshot

import (
	"strings"
	"testing"
)

// fixtureSchema は本番の go_iot スキーマを縮約した検証用スキーマ。
// users ← devices(user_id) / device_tokens(user_id)、devices ← sensor_readings(device_id) の論理関連を持つ。
// orphan_id は対応する親テーブルが無い _id カラム (リレーション対象外の検証用)。
func fixtureSchema() *Schema {
	return &Schema{
		Tables: []Table{
			{
				Name:    "users",
				Comment: "ユーザー (Web UI の Session 認証対象)",
				Columns: []Column{
					{Name: "id", Type: "bigint", Nullable: false, Default: "nextval('users_id_seq'::regclass)", IsPK: true},
					{Name: "email", Type: "character varying(255)", Nullable: false, Comment: "ログインID"},
					{Name: "created_at", Type: "timestamp with time zone", Nullable: false, Default: "now()"},
				},
				Indexes: []Index{
					{Name: "users_email_unique", Def: "CREATE UNIQUE INDEX users_email_unique ON public.users USING btree (email)", IsUnique: true},
				},
			},
			{
				Name:    "devices",
				Comment: "ESP8266デバイス管理",
				Columns: []Column{
					{Name: "id", Type: "bigint", Nullable: false, IsPK: true},
					{Name: "user_id", Type: "bigint", Nullable: false},
					{Name: "temperature", Type: "numeric(5,2)", Nullable: true},
					{Name: "orphan_id", Type: "bigint", Nullable: true}, // 親テーブル orphans は存在しない
				},
				Checks: []Check{
					{Name: "devices_mac_address_format", Expr: "CHECK ((mac_address ~ '^...$'::text))"},
				},
			},
			{
				Name: "device_tokens",
				Columns: []Column{
					{Name: "id", Type: "bigint", Nullable: false, IsPK: true},
					{Name: "user_id", Type: "bigint", Nullable: false},
				},
			},
			{
				Name: "sensor_readings",
				Columns: []Column{
					{Name: "id", Type: "bigint", Nullable: false, IsPK: true},
					{Name: "device_id", Type: "bigint", Nullable: false},
				},
			},
		},
	}
}

func TestRelationships(t *testing.T) {
	rels := fixtureSchema().Relationships()

	want := map[string]string{ // FromTable.FromCol -> ToTable
		"devices.user_id":           "users",
		"device_tokens.user_id":     "users",
		"sensor_readings.device_id": "devices",
	}
	if len(rels) != len(want) {
		t.Fatalf("リレーション数が想定外: got %d (%v), want %d", len(rels), rels, len(want))
	}
	for _, r := range rels {
		key := r.FromTable + "." + r.FromCol
		parent, ok := want[key]
		if !ok {
			t.Errorf("想定外のリレーション: %s -> %s", key, r.ToTable)
			continue
		}
		if r.ToTable != parent {
			t.Errorf("%s の参照先が不正: got %s, want %s", key, r.ToTable, parent)
		}
	}
}

func TestRelationships_ExcludesPKAndOrphan(t *testing.T) {
	for _, r := range fixtureSchema().Relationships() {
		if r.FromCol == "id" {
			t.Errorf("主キー id をリレーション元にしてはならない: %+v", r)
		}
		if r.FromCol == "orphan_id" {
			t.Errorf("親テーブルが存在しない orphan_id を関連にしてはならない: %+v", r)
		}
	}
}

func TestRenderMarkdown(t *testing.T) {
	md := RenderMarkdown(fixtureSchema())

	mustContain := []string{
		"## users", // テーブル見出し
		"ユーザー (Web UI の Session 認証対象)",    // テーブルコメント
		"| カラム | 型 | NULL | デフォルト | 説明 |", // カラム表ヘッダ
		"email",                      // カラム名
		"character varying(255)",     // 生の型を保持
		"ログインID",                     // カラムコメント
		"users_email_unique",         // 索引定義
		"devices_mac_address_format", // CHECK制約名
		"外部キー制約",                     // FK非採用の注記
	}
	for _, s := range mustContain {
		if !strings.Contains(md, s) {
			t.Errorf("Markdown 出力に %q が含まれていない", s)
		}
	}

	// 主キーカラムの行には PK マーカーが付く
	if !strings.Contains(md, "PK") {
		t.Error("主キーに PK マーカーが付いていない")
	}
}

func TestRenderMermaid(t *testing.T) {
	mmd := RenderMermaid(fixtureSchema())

	if !strings.HasPrefix(strings.TrimSpace(mmd), "erDiagram") {
		t.Errorf("Mermaid は erDiagram で始まる必要がある:\n%s", mmd)
	}

	mustContain := []string{
		"users {",                        // エンティティブロック
		"devices ||--o{",                 // 親が複数の子を持つ記法 (実際は users ||--o{ devices)
		"users ||--o{ devices : user_id", // 推論した論理リレーション
		"varchar",                        // 型サニタイズ: character varying(255) -> varchar
		"timestamptz",                    // timestamp with time zone -> timestamptz
		"numeric",                        // numeric(5,2) -> numeric (精度除去)
	}
	for _, s := range mustContain {
		if !strings.Contains(mmd, s) {
			t.Errorf("Mermaid 出力に %q が含まれていない:\n%s", s, mmd)
		}
	}

	// Mermaid のエンティティ属性行に丸括弧/空白付きの生型が漏れていないこと
	// (character varying(255) のような型は Mermaid 構文を壊す)
	if strings.Contains(mmd, "character varying") {
		t.Error("Mermaid に未サニタイズの型 'character varying' が混入している")
	}
}

func TestMermaidType(t *testing.T) {
	cases := map[string]string{
		"bigint":                      "bigint",
		"character varying(255)":      "varchar",
		"numeric(5,2)":                "numeric",
		"timestamp with time zone":    "timestamptz",
		"timestamp without time zone": "timestamp",
		"boolean":                     "boolean",
		"jsonb":                       "jsonb",
		"double precision":            "double",
	}
	for in, want := range cases {
		if got := mermaidType(in); got != want {
			t.Errorf("mermaidType(%q) = %q, want %q", in, got, want)
		}
		if strings.ContainsAny(mermaidType(in), " ()") {
			t.Errorf("mermaidType(%q) に空白/括弧が残っている: %q", in, mermaidType(in))
		}
	}
}
