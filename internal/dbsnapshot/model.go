// Package dbsnapshot は SQLite のスキーマを内省し、
// テーブル定義 (Markdown) と ER 図 (Mermaid) のスナップショットを生成する。
//
// 主目的: 実DBに接続しなくても、生成済みスナップショットを読むだけで
// テーブル・カラム・制約・リレーションを把握できるようにすること
// (AI エージェント・新規参入者向けのドキュメント資産)。
//
// 設計方針: スキーマ取得 (introspect.go: DB依存・非純粋・sqlite_master+PRAGMA) と
// 描画 (render.go: 純粋・テスト可能) を分離する。
package dbsnapshot

// Schema は1つのデータベースから取得した全テーブルのスナップショット。
type Schema struct {
	Tables []Table
}

// Table は1テーブルの定義 (カラム・索引・CHECK制約・コメント)。
type Table struct {
	Name    string
	Comment string
	Columns []Column
	Indexes []Index
	Checks  []Check
}

// Column は1カラムの定義。
type Column struct {
	Name     string // カラム名
	Type     string // SQLite の宣言型 (例: INTEGER / REAL / DATETIME / VARCHAR(20) / BLOB / json)
	Nullable bool   // NULL 許容なら true
	Default  string // デフォルト式 (無ければ空文字)
	IsPK     bool   // 主キー構成カラムなら true
	Comment  string // SQLite はカラムコメント非対応のため常に空 (PK 表示は IsPK で補う)
}

// Index は1索引の定義。Def は sqlite_master.sql の CREATE INDEX 文
// (UNIQUE や部分索引の WHERE 句を含む)。
type Index struct {
	Name     string
	Def      string
	IsUnique bool
}

// Check は1つの CHECK 制約。Expr は CREATE TABLE 文から抽出した CHECK 式
// (例: CHECK (metric IN ('temperature', 'humidity')))。
type Check struct {
	Name string
	Expr string
}

// Relationship は <table>_id 命名規約から推論した論理リレーション。
// 本プロジェクトは外部キー制約を張らない方針のため、DB からは取得できず
// カラム名から推論する (参照整合性はアプリ層で担保)。
type Relationship struct {
	FromTable string // 子テーブル (<x>_id カラムを持つ側)
	FromCol   string // 参照元カラム (例: user_id)
	ToTable   string // 親テーブル (例: users)
}

// Relationships は Schema 内の全テーブルを走査し、
// 「<base>_id」カラム → テーブル「<base>s」への論理リレーションを推論して返す。
// 主キー (id) や、対応する親テーブルが存在しない _id カラムは対象外。
func (s *Schema) Relationships() []Relationship {
	exists := make(map[string]bool, len(s.Tables))
	for _, t := range s.Tables {
		exists[t.Name] = true
	}

	var rels []Relationship
	for _, t := range s.Tables {
		for _, c := range t.Columns {
			parent, ok := inferParentTable(c)
			if !ok || !exists[parent] {
				continue
			}
			rels = append(rels, Relationship{
				FromTable: t.Name,
				FromCol:   c.Name,
				ToTable:   parent,
			})
		}
	}
	return rels
}

// inferParentTable は「<base>_id」カラムから親テーブル名「<base>s」を推論する。
// 主キー (IsPK) や末尾が _id でないカラムは対象外 (ok=false)。
func inferParentTable(c Column) (table string, ok bool) {
	const suffix = "_id"
	if c.IsPK || len(c.Name) <= len(suffix) || c.Name[len(c.Name)-len(suffix):] != suffix {
		return "", false
	}
	base := c.Name[:len(c.Name)-len(suffix)]
	// 単純複数形 (base + "s")。user→users / device→devices / alert_rule→alert_rules。
	return base + "s", true
}
