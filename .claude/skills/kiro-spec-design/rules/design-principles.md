# Technical Design Rules and Principles

## Core Design Principles

### 0. Boundary First
- **Boundary is mandatory; owner is optional**
- A design is not ready when it explains components but leaves responsibility seams ambiguous
- Define what the spec owns before elaborating how it works
- Explicitly record what is out of boundary
- Do not leak downstream-specific behavior or assumptions into upstream boundaries

### 1. Type Safety is Mandatory
- 公開シグネチャ（関数・メソッド・interface）は明示的な型を持つ。Go では `interface{}` / `any` への退避を避け、最小の interface を定義する。
- すべてのパラメータと戻り値に明示的な型を定義する。
- エラーは Go の `error` 値（センチネル / 型付きエラー + `errors.Is`/`errors.As`）で判別可能に表現する。
- ジェネリクスを使う場合は型制約を明記する。

### 2. Design vs Implementation
- **Focus on WHAT, not HOW**
- Define interfaces and contracts, not code
- Specify behavior through pre/post conditions
- Document architectural decisions, not algorithms

### 3. Visual Communication
- **Simple features**: Basic component diagram or none
- **Medium complexity**: Architecture + data flow
- **High complexity**: Multiple diagrams (architecture, sequence, state)
- **Always pure Mermaid**: No styling, just structure

### 4. Component Design Rules
- **Single Responsibility**: One clear purpose per component
- **Clear Boundaries**: Explicit domain ownership
- **Boundary Commitments First**: Before detailing components, state the responsibility boundary this design commits to
- **Dependency Direction**: Follow architectural layers
- **Interface Segregation**: Minimal, focused interfaces
- **Team-safe Interfaces**: Design boundaries that allow parallel implementation without merge conflicts
- **No Hidden Shared Ownership**: If two areas appear to co-own the same behavior or data, the design is incomplete
- **Research Traceability**: Record boundary decisions and rationale in `research.md`

### 5. Data Modeling Standards
- **Domain First**: Start with business concepts
- **Consistency Boundaries**: Clear aggregate roots
- **Normalization**: Balance between performance and integrity
- **Evolution**: Plan for schema changes

### 6. Error Handling Philosophy
- **Fail Fast**: Validate early and clearly
- **Graceful Degradation**: Partial functionality over complete failure
- **User Context**: Actionable error messages
- **Observability**: Comprehensive logging and monitoring

### 7. Integration Patterns
- **Loose Coupling**: Minimize dependencies
- **Contract First**: Define interfaces before implementation
- **Versioning**: Plan for API evolution
- **Idempotency**: Design for retry safety
- **Contract Visibility**: Surface API and event contracts in design.md while linking extended details from `research.md`

### 8. Dependency Direction
- **Define and enforce the dependency direction** in the architecture section of design.md（本プロジェクトは structure.md steering のレイヤ方向に従う。例: `handler → service → repository.Querier`（sqlc ポート）、`domain` は上位から参照のみ、認可は `internal/authz` に集約。厳格 Clean / Hexagonal ではなく実務的 Layered-lite）
- Each layer imports only from layers to its left — never upward
- This constraint is not a suggestion; implementation and review should treat violations as errors
- When the File Structure Plan maps files to components, the dependency direction determines which imports are allowed

## Documentation Standards

### Language and Tone
- **Declarative**: "The system authenticates users" not "The system should authenticate"
- **Precise**: Specific technical terms over vague descriptions
- **Concise**: Essential information only
- **Formal**: Professional technical writing

### Structure Requirements
- **Hierarchical**: Clear section organization
- **Traceable**: Requirements to components mapping
- **Complete**: All aspects covered for implementation
- **Consistent**: Uniform terminology throughout
- **Focused**: Keep design.md centered on architecture and contracts; move investigation logs and lengthy comparisons to `research.md`

## Section Authoring Guidance

### Global Ordering
- Default flow: Overview → Goals/Non-Goals → Boundary Commitments → Architecture → File Structure Plan → Components & Interfaces → Optional sections.
- Teams may swap Traceability earlier or place Data Models nearer Architecture when it improves clarity, but keep section headings intact.
- Within each section, follow **Summary → Scope → Decisions → Impacts/Risks** so reviewers can scan consistently.

### Requirement IDs
- Reference requirements as `2.1, 2.3` without prefixes (no “Requirement 2.1”).
- All requirements MUST have numeric IDs. If a requirement lacks a numeric ID, stop and fix `requirements.md` before continuing.
- Use `N.M`-style numeric IDs where `N` is the top-level requirement number from requirements.md (for example, Requirement 1 → 1.1, 1.2; Requirement 2 → 2.1, 2.2).
- Every component, task, and traceability row must reference the same canonical numeric ID.

### Technology Stack
- Include ONLY layers impacted by this feature (frontend, backend, data, messaging, infra).
- For each layer specify tool/library + version + the role it plays; push extended rationale, comparisons, or benchmarks to `research.md`.
- When extending an existing system, highlight deviations from the current stack and list new dependencies.

### System Flows
- Add diagrams only when they clarify behavior:  
  - **Sequence** for multi-step interactions  
  - **Process/State** for branching rules or lifecycle  
  - **Data/Event** for pipelines or async patterns
- Always use pure Mermaid. If no complex flow exists, omit the entire section.

### Requirements Traceability
- Use the standard table (`Requirement | Summary | Components | Interfaces | Flows`) to prove coverage.
- Collapse to bullet form only when a single requirement maps 1:1 to a component.
- Prefer the component summary table for simple mappings; reserve the full traceability table for complex or compliance-sensitive requirements.
- Re-run this mapping whenever requirements or components change to avoid drift.

### Components & Interfaces Authoring
- Boundary Commitments should already make the ownership seam explicit before this section begins.
- Group components by domain/layer and provide one block per component.
- Begin with a summary table listing Component, Domain, Intent, Requirement coverage, key dependencies, and selected contracts.
- Table fields: Intent (one line), Requirements (`2.1, 2.3`), Owner/Reviewers (optional).
- Dependencies table must mark each entry as Inbound/Outbound/External and assign Criticality (`P0` blocking, `P1` high-risk, `P2` informational).
- Summaries of external dependency research stay here; detailed investigation (API signatures, rate limits, migration notes) belongs in `research.md`.
- design.md must remain a self-contained reviewer artifact. Reference `research.md` only for background, and restate any conclusions or decisions here.
- Contracts: tick only the relevant types (View/Template / Service / API(JSON) / Event / Batch / State). Web UI 画面は **View/Template**（templ 返却・HTMX 部分更新）で記述し、`API(JSON)` はデバイス取込API・ドキュメント専用とする。Unchecked types should not appear later in the component section.
- Service interfaces must declare method signatures, inputs/outputs, and error envelopes. API/Event/Batch contracts require schema tables or bullet lists covering trigger, payload, delivery, idempotency.
- Use **Integration & Migration Notes**, **Validation Hooks**, and **Open Questions / Risks** to document rollout strategy, observability, and unresolved decisions.
- Detail density rules:
  - **Full block**: components introducing new boundaries (logic hooks, shared services, external integrations, data layers).
  - **Summary-only**: presentational/UI components with no new boundaries (plus a short Implementation Note if needed).
- Implementation Notes must combine Integration / Validation / Risks into a single bulleted subsection to reduce repetition.
- Prefer lists or inline descriptors for short data (dependencies, contract selections). Use tables only when comparing multiple items.

### Shared UI Component Contracts（templ）
- 繰り返し使う templ コンポーネントは**共通パラメータ struct**（Go struct）を定義し、コンポーネントごとに差分フィールドを追加する。React の props 継承階層ではなく、struct 埋め込み / 共有パラメータで再利用する。
- HTMX フラグメントを返すハンドラ・ユーティリティ・外部連携アダプタが新しい契約を導入する場合は、Go の関数 / interface シグネチャで契約を明記する。
- 共通レイアウトや共有 struct を再利用する場合は、コードブロックを複製せず明示的に参照する（例: 「共通 `PageLayout` を埋め込み、`DeviceFilter` を追加」）。

### Data Models
- Domain Model covers aggregates, entities, value objects, domain events, and invariants. Add Mermaid diagrams only when relationships are non-trivial.
- Logical Data Model should articulate structure, indexing, sharding, and storage-specific considerations (event store, KV/wide-column) relevant to the change.
- Data Contracts & Integration section documents API payloads, event schemas, and cross-service synchronization patterns when the feature crosses boundaries.
- Lengthy type definitions or vendor-specific option objects should be placed in the Supporting References section within design.md, linked from the relevant section. Investigation notes stay in `research.md`.
- Supporting References usage is optional; only create it when keeping the content in the main body would reduce readability. All decisions must still appear in the main sections so design.md stands alone.

### Error/Testing/Security/Performance Sections
- Record only feature-specific decisions or deviations. Link or reference organization-wide standards (steering) for baseline practices instead of restating them.

### Diagram & Text Deduplication
- Do not restate diagram content verbatim in prose. Use the text to highlight key decisions, trade-offs, or impacts that are not obvious from the visual.
- When a decision is fully captured in the diagram annotations, a short “Key Decisions” bullet is sufficient.

### General Deduplication
- Avoid repeating the same information across Overview, Architecture, and Components. Reference earlier sections when context is identical.
- If a requirement/component relationship is captured in the summary table, do not rewrite it elsewhere unless extra nuance is added.

## Diagram Guidelines

### When to include a diagram
- **Architecture**: Use a structural diagram when 3+ components or external systems interact.
- **Sequence**: Draw a sequence diagram when calls/handshakes span multiple steps.
- **State / Flow**: Capture complex state machines or business flows in a dedicated diagram.
- **ER**: Provide an entity-relationship diagram for non-trivial data models.
- **Skip**: Minor one-component changes generally do not need diagrams.

### Mermaid requirements
```mermaid
graph TB
    Client --> ApiGateway
    ApiGateway --> ServiceA
    ApiGateway --> ServiceB
    ServiceA --> Database
```

- **Plain Mermaid only** – avoid custom styling or unsupported syntax.
- **Node IDs** – alphanumeric plus underscores only (e.g., `Client`, `ServiceA`). Do not use `@`, `/`, or leading `-`.
- **Labels** – simple words. Do not embed parentheses `()`, square brackets `[]`, quotes `"`, or slashes `/`.
  - ❌ `DnD[@dnd-kit/core]` → invalid ID (`@`).
  - ❌ `UI[KanbanBoard(React)]` → invalid label (`()`).
  - ✅ `DndKit[dnd-kit core]` → use plain text in labels, keep technology details in the accompanying description.
  - ℹ️ Mermaid strict-mode will otherwise fail with errors like `Expecting 'SQE' ... got 'PS'`; remove punctuation from labels before rendering.
- **Edges** – show data or control flow direction.
- **Groups** – using Mermaid subgraphs to cluster related components is allowed; use it sparingly for clarity.

## Quality Metrics
### Design Completeness Checklist
- All requirements addressed
- No implementation details leaked
- Clear component boundaries
- Explicit error handling
- Comprehensive test strategy
- Security considered
- Performance targets defined
- Migration path clear (if applicable)

### Common Anti-patterns to Avoid
❌ Mixing design with implementation
❌ Vague interface definitions
❌ Missing error scenarios
❌ Ignored non-functional requirements
❌ Overcomplicated architectures
❌ Tight coupling between components
❌ Missing data consistency strategy
❌ Incomplete dependency analysis
