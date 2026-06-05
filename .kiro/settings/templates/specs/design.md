# Design Document Template

---
**Purpose**: Provide sufficient detail to ensure implementation consistency across different implementers, preventing interpretation drift.

**Approach**:
- Include essential sections that directly inform implementation decisions
- Omit optional sections unless critical to preventing implementation errors
- Match detail level to feature complexity
- Use diagrams and tables over lengthy prose

**Warning**: Approaching 1000 lines indicates excessive feature complexity that may require design simplification or splitting into multiple specs.
---

> Sections may be reordered (e.g., surfacing Requirements Traceability earlier or moving Data Models nearer Architecture) when it improves clarity. Within each section, keep the flow **Summary → Scope → Decisions → Impacts/Risks** so reviewers can scan consistently.

## Overview 
2-3 paragraphs max
**Purpose**: This feature delivers [specific value] to [target users].
**Users**: [Target user groups] will utilize this for [specific workflows].
**Impact** (if applicable): Changes the current [system state] by [specific modifications].


### Goals
- Primary objective 1
- Primary objective 2  
- Success criteria

### Non-Goals
- Explicitly excluded functionality
- Future considerations outside current scope
- Integration points deferred

## Boundary Commitments

State the responsibility boundary of this spec in concrete terms. Treat this as the anchor for architecture, tasks, and later validation.

### This Spec Owns
- Capabilities and behaviors this spec is responsible for
- Data it owns or is authoritative for
- Interfaces or contracts it defines or stabilizes

### Out of Boundary
- Related concerns this spec explicitly does NOT own
- Work deferred to another spec, existing subsystem, or later phase
- Changes this spec must not absorb as "just one more thing"

### Allowed Dependencies
- Upstream systems/specs/components this design may depend on
- Shared infrastructure this design may use
- Dependency constraints that must not be violated

### Revalidation Triggers
List the kinds of changes that should force dependent specs or consumers to re-check integration.

- Contract shape changes
- Data ownership changes
- Dependency direction changes
- Startup/runtime prerequisite changes

## Architecture

> Reference detailed discovery notes in `research.md` only for background; keep design.md self-contained for reviewers by capturing all decisions and contracts here.
> Capture key decisions in text and let diagrams carry structural detail—avoid repeating the same information in prose.
> Supporting sections below should remain as light as possible unless they materially clarify the responsibility boundary, dependency rules, or integration seams.

### Existing Architecture Analysis (if applicable)
When modifying existing systems:
- Current architecture patterns and constraints
- Existing domain boundaries to be respected
- Integration points that must be maintained
- Technical debt addressed or worked around

### Architecture Pattern & Boundary Map
**RECOMMENDED**: Include Mermaid diagram showing the chosen architecture pattern and system boundaries (required for complex features, optional for simple additions)

**Architecture Integration**:
- Selected pattern: [name and brief rationale]
- Domain/feature boundaries: [how responsibilities are separated to avoid conflicts]
- Existing patterns preserved: [list key patterns]
- New components rationale: [why each is needed]
- Steering compliance: [principles maintained]

### Technology Stack

| Layer | Choice / Version | Role in Feature | Notes |
|-------|------------------|-----------------|-------|
| Frontend / CLI | | | |
| Backend / Services | | | |
| Data / Storage | | | |
| Messaging / Events | | | |
| Infrastructure / Runtime | | | |

> Keep rationale concise here and, when more depth is required (trade-offs, benchmarks), add a short summary plus pointer to the Supporting References section and `research.md` for raw investigation notes.

## File Structure Plan

Map the directory structure and file responsibilities for this feature. This section directly drives task `_Boundary:_` annotations and implementation Task Briefs. Use the appropriate level of detail:

- **Small features**: List individual files with responsibilities
- **Large features**: Describe directory-level structure + per-domain/module pattern, list only non-obvious files individually

### Directory Structure
```
internal/
├── domain_a/              # Domain A responsibility
│   ├── handler.go         # Gin ハンドラ（HTTP 境界）
│   ├── service.go         # ビジネスロジック
│   ├── repository.go      # 永続化境界（sqlc 呼び出し）
│   └── types.go           # ドメイン型
├── domain_b/              # Domain B (domain_a と同じパターン)
└── shared/
    └── cross_cutting.go   # 非自明: なぜここにあるか
cmd/
└── server/
    └── main.go            # 組立・起動のみ
db/
├── migrations/            # goose マイグレーション
└── queries/               # sqlc 入力 SQL
```

> 繰り返しの構造は「domain_b は domain_a と同じパターン」と一度だけ記述する。パスから責務が自明でないファイルだけ個別列挙する。

### Modified Files
- `internal/path/to/existing.go` — 何をどう変え、なぜか

> Each file should have one clear responsibility. Group files that change together. For repeated structures, describe the pattern once rather than listing every file.
> Avoid duplicating what Components and Interfaces already describes — focus on the physical file layout that Components maps to.

## System Flows

Provide only the diagrams needed to explain non-trivial flows. Use pure Mermaid syntax. Common patterns:
- Sequence (multi-party interactions)
- Process / state (branching logic or lifecycle)
- Data / event flow (pipelines, async messaging)

Skip this section entirely for simple CRUD changes.
> Describe flow-level decisions (e.g., gating conditions, retries) briefly after the diagram instead of restating each step.

## Requirements Traceability

Use this section for complex or compliance-sensitive features where requirements span multiple domains. Straightforward 1:1 mappings can rely on the Components summary table.

Map each requirement ID (e.g., `2.1`) to the design elements that realize it.

| Requirement | Summary | Components | Interfaces | Flows |
|-------------|---------|------------|------------|-------|
| 1.1 | | | | |
| 1.2 | | | | |

> Omit this section only when a single component satisfies a single requirement without cross-cutting concerns.

## Components and Interfaces

Provide a quick reference before diving into per-component details.

- Summaries can be a table or compact list. Example table:
  | Component | Domain/Layer | Intent | Req Coverage | Key Dependencies (P0/P1) | Contracts |
  |-----------|--------------|--------|--------------|--------------------------|-----------|
  | DeviceListView | View (templ) | デバイス一覧テーブルを描画 | 1, 2 | LayoutBase (P0), DeviceCardsPartial (P1) | View/Template |
  | DeviceIngestAPI | Backend | 取込API（JSON）を受理 | 3 | repository.Querier (P0) | API (JSON) |
- Only components introducing new boundaries (e.g., HTMX フラグメントを返すハンドラ, external integrations, persistence) require full detail blocks. Simple presentation components（静的な templ 部品）can rely on the summary row plus a short Implementation Note.

Group detailed blocks by domain or architectural layer. For each detailed component, list requirement IDs as `2.1, 2.3` (omit “Requirement”). 本プロジェクトの UI は **templ コンポーネント**（typed な Go struct / 明示パラメータで受け取る）であり、クライアント状態は Alpine.js、サーバ部分更新は HTMX で扱う。複数の View が同じ構造を共有する場合は、共通レイアウト templ または共有パラメータ struct を参照して重複記述を避ける（React の props 継承ではない）。

### [Domain / Layer]

#### [Component Name]

| Field | Detail |
|-------|--------|
| Intent | 1-line description of the responsibility |
| Requirements | 2.1, 2.3 |
| Owner / Reviewers | (optional) |

**Responsibilities & Constraints**
- Primary responsibility
- Domain boundary and transaction scope
- Data ownership / invariants

**Dependencies**
- Inbound: Component/service name — purpose (Criticality)
- Outbound: Component/service name — purpose (Criticality)
- External: Service/library — purpose (Criticality)

Summarize external dependency findings here; deeper investigation (API signatures, rate limits, migration notes) lives in `research.md`.

**Contracts**: View/Template [ ] / Service [ ] / API (JSON) [ ] / Event [ ] / Batch [ ] / State [ ]  ← check only the ones that apply.

> 本プロジェクトの分界: **Web UI 画面 = `View/Template`**（templ が HTML を直接返す。部分更新は HTMX）。**`API (JSON)` はデバイス取込API（ESP8266→Go, Bearer）と OpenAPI/Scalar ドキュメントのみ**。Web UI 画面ハンドラを `API (JSON)` 契約として書かないこと。

##### View / Template Contract
Web UI 画面ハンドラ（templ 返却・HTMX 部分更新）の契約。

| Trigger | Method | Path | 認証 | 返却モード | 返却 templ コンポーネント | 入力(binding) | エラー時 |
|---------|--------|------|------|-----------|--------------------------|---------------|----------|
| 初期表示 | GET | /devices | session | full page | `DevicesPage` | DeviceListQuery | 再レンダリング |
| hx-get | GET | /devices/cards | session | HTMX partial | `DeviceCards`(hx-target=#device-cards) | DeviceFilter | 422+同一partial |

- **返却モード**: full page templ / HTMX partial（返却する templ コンポーネント名と分割方針）。OOB 同時更新があれば対象も記す。
- **HTMX トリガ**: `hx-get/post/delete`・`hx-target`・`hx-swap`。
- **バリデーション**: 入力は binding 構造体（`go-playground/validator` タグ）。エラーは**同一テンプレに field error を載せて再レンダリング**（HTMX フォームは 422 で部分返却）。
- **CSRF**: meta tag + `htmx:configRequest`（ミューテーション時）。
- 詳細パターンは `2cc_sdd/HTMX実装ガイド(動的).md` の該当節に従う。

##### Service Interface
```go
// [ComponentName]Service はこのドメインの入口となる振る舞いを定義する
type [ComponentName]Service interface {
    MethodName(ctx context.Context, input InputType) (OutputType, error)
}
```
- 事前条件:
- 事後条件:
- 不変条件:
- 永続化は **sqlc 生成の `repository.Querier`（`emit_interface=true`、本プロジェクト唯一の DB ポート）** を DI で受け取って経由する。独自 Repository interface を新設しない。テストでは `Querier` をモック差し替えする。

##### API (JSON) Contract
> **デバイス取込API・ドキュメント専用**。Web UI 画面はここに書かず `View / Template Contract` を使う。

| Method | Endpoint | Request | Response | Auth | Errors |
|--------|----------|---------|----------|------|--------|
| POST | /api/v1/ingest | IngestRequest (JSON) | 202 Accepted | Bearer | 400, 401, 422, 500 |

##### Event Contract
- Published events:  
- Subscribed events:  
- Ordering / delivery guarantees:

##### Batch / Job Contract
- Trigger:  
- Input / validation:  
- Output / destination:  
- Idempotency & recovery:

##### State Management
- State model:  
- Persistence & consistency:  
- Concurrency strategy:

**Implementation Notes**
- Integration: 
- Validation: 
- Risks:

## Data Models

Focus on the portions of the data landscape that change with this feature.

### Domain Model
- Aggregates and transactional boundaries
- Entities, value objects, domain events
- Business rules & invariants
- Optional Mermaid diagram for complex relationships

### Logical Data Model

**Structure Definition**:
- Entity relationships and cardinality
- Attributes and their types
- Natural keys and identifiers
- Referential integrity rules
  > 本プロジェクトは **DB の外部キー制約を張らない**方針（参照整合性はアプリ層の JOIN クエリ / サービス層チェックで担保）。論理リレーションは `docs/database_snapshot/er_diagram.mmd` を参照。

**Consistency & Integrity**:
- Transaction boundaries
- Cascading rules（DB CASCADE ではなくサービス層の削除順序で表現）
- Temporal aspects (versioning, audit)

### Physical Data Model
**When to include**: When implementation requires specific storage design decisions

**For Relational Databases**:
- Table definitions with data types
- Primary/foreign keys and constraints
- Indexes and performance optimizations
- Partitioning strategy for scale

**For Document Stores**:
- Collection structures
- Embedding vs referencing decisions
- Sharding key design
- Index definitions

**For Event Stores**:
- Event schema definitions
- Stream aggregation strategies
- Snapshot policies
- Projection definitions

**For Key-Value/Wide-Column Stores**:
- Key design patterns
- Column families or value structures
- TTL and compaction strategies

### Data Contracts & Integration

**Data Transfer**
- **Web UI**: ハンドラ→templ に渡す ViewModel / パラメータ構造体（Go struct）と、フォーム入力の binding 構造体（`go-playground/validator` タグ）。HTML レンダリングのため JSON シリアライズは不要。
- **JSON API（デバイス取込 / ドキュメントのみ）**: Request/Response の JSON スキーマと `ShouldBindJSON` + binding タグ。シリアライズ形式は JSON（Protobuf 等は使わない）。
- バリデーションルール（境界値・必須・enum 許容値は `docs/database_snapshot/` の CHECK 制約に整合させる）。

**Event Schemas**
- Published event structures
- Schema versioning strategy
- Backward/forward compatibility rules

**Cross-Service Data Management**
- Distributed transaction patterns (Saga, 2PC)
- Data synchronization strategies
- Eventual consistency handling

Skip subsections that are not relevant to this feature.

## Error Handling

### Error Strategy
Concrete error handling patterns and recovery mechanisms for each error type.

### Error Categories and Responses
**User Errors** (4xx): Invalid input → field-level validation; Unauthorized → auth guidance; Not found → navigation help
**System Errors** (5xx): Infrastructure failures → graceful degradation; Timeouts → circuit breakers; Exhaustion → rate limiting  
**Business Logic Errors** (422): Rule violations → condition explanations; State conflicts → transition guidance

**Process Flow Visualization** (when complex business logic exists):
Include Mermaid flowchart only for complex error scenarios with business workflows.

### Monitoring
Error tracking, logging, and health monitoring implementation.

## Testing Strategy

### Default sections (adapt names/sections to fit the domain)
- Unit Tests: 3–5 items（table-driven テスト。Service は `repository.Querier` をモック差し替え／binding バリデーションの境界値）
- Integration Tests: 3–5 cross-component flows（`httptest` でハンドラ→templ レンダリング結果（HTML / HTMX partial）をアサート。`Querier` 差し替えで DB 境界を分離）
- E2E/UI Tests (if applicable): 3–5 critical user paths（HTMX 部分更新で `hx-target` が差し替わること、フォーム送信・リダイレクト）
- Performance/Load (if applicable): 3–4 items（pgx コネクションプール、取込API スループット）

## Optional Sections (include when relevant)

### Security Considerations
_Use this section for features handling auth, sensitive data, external integrations, or user permissions. Capture only decisions unique to this feature; defer baseline controls to steering docs._
- Threat modeling, security controls, compliance requirements
- Authentication and authorization patterns
- Data protection and privacy considerations

### Performance & Scalability
_Use this section when performance targets, high load, or scaling concerns exist. Record only feature-specific targets or trade-offs and rely on steering documents for general practices._
- Target metrics and measurement strategies
- Scaling approaches (horizontal/vertical)
- Caching strategies and optimization techniques

### Migration Strategy
Include a Mermaid flowchart showing migration phases when schema/data movement is required.
- Phase breakdown, rollback triggers, validation checkpoints

## Supporting References (Optional)
- Create this section only when keeping the information in the main body would hurt readability (e.g., very long Go interface/struct definitions, vendor option matrices, exhaustive schema tables). Keep decision-making context in the main sections so the design stays self-contained.
- Link to the supporting references from the main text instead of inlining large snippets.
- Background research notes and comparisons continue to live in `research.md`, but their conclusions must be summarized in the main design.
