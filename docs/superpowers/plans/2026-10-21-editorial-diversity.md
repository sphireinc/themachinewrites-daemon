# Editorial Diversity, Self-Awareness, and Topic Exploration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` or `superpowers:subagent-driven-development` to implement this plan task-by-task after approval. Steps use checkbox syntax for tracking.

**Goal:** Extend The Machine Writes with recoverable subject identity and bounded, evidence-aware editorial history analysis across the daemon, public contract, and private website API.

**Architecture:** Keep the existing Go editorial cycle, SQLite store, Git publication corpus, and three independent repositories. Add a subject registry and separate append-only administrative amendment records, derive diversity profiles from canonical editorial publications using effective identities, then pass compact profiles into the existing bounded research bundle without changing mode selection or publication decisions. Deliver the public contract first, pin it in the daemon and private repos, and retain compatibility for older records.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), YAML frontmatter, JSON Schema 2020-12, Node.js validation scripts, TypeScript, Vitest/Vite.

**Spec:** `../../../_plan/features/05-agent/FEAT-035-editorial-diversity-self-awareness-topic-exploration.md`

## Global Constraints

- Preserve the three independent Git histories; never initialize or commit in the parent project root.
- Canonical public publication files remain the authority; SQLite is a reconstructable working index.
- Every new publication requires a random `subject_id` in `sub_` plus 32 lowercase hexadecimal characters; never derive it from title, URL, topic, or model output.
- Historical records without `subject_id` remain readable and are never rewritten to backfill identity.
- Subject amendments are append-only administrative records under `subject-amendments/`; they never appear as editorial posts, feeds, publication counts, or diversity samples. Replay them in stable `(amended_at, amendment_id)` order when resolving effective assignments.
- Topic classification is versioned; raw publication tags remain unchanged, aliases normalize deterministically, and unknown valid tags remain open categories.
- No vector database, external semantic service, new database, topic quota, forced publication, engagement optimization, or invented source material.
- Default minimum sample is 10; valid configured range is 2 through 100. Empty history is `no_history`; below minimum is `sparse_history`; both have `statistical_status: insufficient_history` and no model-facing imbalance claims. The public post/feed contract remains schema v1 with additive optional fields; amendment records use `subject-identity-amendment.v1.schema.json`.
- Concentration is `1 - H/ln(K)` over normalized primary topics only; `K=1` yields 1, and `N=0` yields null.
- Explicit editorial mode stays authoritative. Diversity cannot remove candidates, suppress warranted follow-ups, change conclusions, or prevent abstention. Promotion budget is zero for fraction zero or fewer than two candidates; otherwise `min(n-1, max(1, floor(n*fraction)))`, capped at one candidate for small pools and by the configured fraction for larger pools.
- Preserve the approved website layout and do not publish fixture or fake content.
- Do not modify the private repository's pre-existing staged `pnpm-lock.yaml` or `pnpm-workspace.yaml`; stage only task-owned paths.
- Tests must not use live Codex, source feeds, publisher credentials, production Git remotes, or deployment.

## Review Focus

- A lost, empty, or corrupt SQLite database must reconstruct exact published `subject_id` values and amendment overlays from the canonical corpus; test from a fresh database and reject malformed conflicting amendments.
- Similar titles or broad topic overlap must not merge distinct subjects; distinct titles for the same explicit follow-up must reuse an offered subject ID; test positive and negative matches.
- Sparse data must not be mistaken for measured imbalance; test 0, 1, minimum-minus-one, minimum, one-category, and multiple-category entropy cases.
- Multi-topic coverage can overlap while primary-topic concentration stays exclusive; test alias de-duplication within a post and exact primary-topic probabilities.
- Warranted follow-ups must survive repetition penalties, and diversity must never change an explicit mode, remove candidates, or convert a defer into publication; test all three at the runtime boundary.

## Repository File Map

### `themachinewrites-public` (canonical contract and append-only corpus)

- `schemas/post.v1.schema.json`: optional `subject_id` and `subject_label` for legacy-compatible schema v1 records.
- `schemas/latest-feed.v1.schema.json` and `schemas/month-feed.v1.schema.json`: optional card `subject_id` for old records and generated cards.
- `schemas/subject-identity-amendment.v1.schema.json`: separate administrative, append-only amendment record contract; exclude these records from post/feed manifests and statistics.
- `scripts/validate.mjs`, fixtures, and `tests/validate.test.mjs`: validate ID shape, amendment references/order, backward compatibility, admin-record exclusion, and invalid amendment cases.
- `docs/URL-CONTRACT.md` or a new focused subject-identity contract document: contract and correction semantics.
- Existing `feed/*.json`: regenerate from the empty canonical corpus; do not fabricate a publication.

### `themachinewrites-daemon` (identity, profile, and runtime integration)

- `internal/publication/corpus.go`, `render.go`, `validate.go`, `feeds.go`, and associated tests: parse/require new `subject_id`, render additive labels, load and replay separate administrative amendments, and keep amendments out of editorial feeds/counts.
- `internal/engine/proposal.schema.json` and `internal/engine` proposal structures: request identity reuse/new subject label and amendment proposal data.
- `internal/store/migrations/011_subject_registry.sql`, `internal/store/subjects.go`, and tests: registry, aliases, effective post assignment overlay, and amendment index. Published IDs/amendments rebuild from canonical records; unpublished subjects/aliases are SQLite-only and are lost with the DB; pending writes use the existing outbox recovery rules.
- `internal/editorial/topics.go`, `history.go`, `diversity.go`, and tests: taxonomy, profiles, entropy, repetition, and bounded ordering.
- `internal/config/config.go`, `validate.go`, `config.example.yaml`, and tests: settings/defaults/range checks.
- `internal/observations` bundle types/build logic and tests (confirm exact current package path during execution): bounded profile/subject context.
- `internal/daemon/runtime.go`, focused runtime tests, `docs/EDITORIAL-MODES.md`, `docs/RECOVERY.md`, and README: connect the profile and registry to existing cycle and document operation.
- `testdata/contracts/*`, `testdata/contracts/PIN.json`, and `tests/fixtures/public-repository/schemas/*`: mirror/pin exact public contract revision and hashes.

### `themachinewrites-private` (read-only website/API compatibility)

- `functions/_lib/types.ts`, `functions/api/feed.ts`, `functions/_lib/post.ts` and tests: carry/validate optional `subject_id` and amendment metadata where exposed by API.
- `src/api/feed.ts` and tests: preserve the additive field in typed feed records.
- `tests/contracts/*.schema.json`, `tests/contracts/PIN.json`, compatibility tests: pin and validate the exact public schema revision.
- No visual layout changes are planned; verify this with the existing UI suite and a diff limited to contract/data plumbing.

## Dependency Order

```text
Task 1 contract semantics and fixtures
  -> Task 2 daemon public-contract pin and publication metadata round-trip
  -> Task 3 SQLite registry, canonical rebuild, and assignment amendment replay
  -> Task 4 topic taxonomy and window/profile data model
  -> Task 5 reproducible distributions, entropy, sparse-history states, and continuity metrics
  -> Task 6 configuration and bounded diversity ordering
  -> Task 7 proposal/context/runtime integration and subject continuity
  -> Task 8 private website/API contract update and cross-repository compatibility
  -> Task 9 operational docs, recovery/rollback procedure, task evidence, acceptance review
```

Each task lands as a reviewable commit in its owning repository when practical. The public contract commit precedes daemon/private pin updates. Do not integrate branches until each child repo passes its task checks and the cross-repository contract test.

### Required independent-review gates

- After Tasks 1–2: review public contract compatibility, administrative amendment exclusion, additive schema-v1 behavior, and exact public schema pins.
- After Task 3: review SQLite persistence, fresh-database reconstruction, amendment replay, and unpublished/pending recovery boundaries.
- After Task 7: review runtime mode preservation, effective identity use, conservative matching, bounded ordering, follow-up eligibility, and abstention.
- After Tasks 8–9: review cross-repository pins, API compatibility, security/privacy, recovery/rollback, website appearance, and regression evidence.

Resolve blocking findings with regression tests and focused commits before continuing past that gate.

---

### Task 1: Pin public subject and amendment semantics

**Repository:** `themachinewrites-public`

**Files:**
- Modify: `schemas/post.v1.schema.json`, `schemas/latest-feed.v1.schema.json`, `schemas/month-feed.v1.schema.json`, `schemas/subject-identity-amendment.v1.schema.json`
- Modify: `scripts/validate.mjs`, `docs/URL-CONTRACT.md` or create `docs/SUBJECT-IDENTITY.md`
- Test: `tests/fixtures/post.valid.json`, add subject amendment fixtures, `tests/validate.test.mjs`

**Interfaces:**
- Produces the exact schema-v1 publication/feed names and the separate `subject-identity-amendment.v1.schema.json` fields consumed by Tasks 2–9. Amendment records use `amendment_id` (ULID), `post_id`, nullable `previous_subject_id`, `replacement_subject_id`, `reason`, and `amended_at`; files live under `subject-amendments/YYYY/MM/DD/<amendment_id>.json` and are administrative records, not editorial publications.

- [ ] **Step 1: Add failing validator tests** for legacy post without `subject_id`, ID shape, amendment schema/reference validity, conflicting amendments, feed/count exclusion, and append-only behavior that leaves target publication bytes unchanged.
- [ ] **Step 2: Run** `npm test` from `themachinewrites-public`; confirm the new cases fail against current schemas/validator.
- [ ] **Step 3: Implement additive schema/validator rules** and document that `subject_id` is optional for historical records but required by daemon creation; a separate administrative amendment under `subject-amendments/` overlays a target post's effective identity in stable timestamp/ID order and never enters the editorial corpus or publication metrics.
- [ ] **Step 4: Run** `npm test && npm run validate`; verify old fixtures remain valid, invalid amendment fixtures are rejected, and no actual post is added.
- [ ] **Step 5: Commit only public docs/schemas/validator/fixtures/tests** with message `feat: define subject identity amendment contract`.

### Task 2: Pin the exact public contract in daemon and implement metadata round-trip

**Repository:** `themachinewrites-daemon`

**Files:**
- Modify: `internal/publication/corpus.go`, `render.go`, `validate.go`, `feeds.go`
- Modify: `testdata/contracts/post.v1.schema.json`, `testdata/contracts/latest-feed.v1.schema.json`, `tests/fixtures/public-repository/schemas/*`, existing contract pin manifest
- Test: `internal/publication/corpus_test.go`, `render_test.go`, `contract_test.go`, `feeds_test.go`

**Interfaces:**
- Consumes public contract committed in Task 1.
- Produces `PostRecord.SubjectID string`, `PostRecord.SubjectLabel string`, and `Corpus.Amendments []SubjectIdentityAmendment` loaded separately from `subject-amendments/`; feed cards carry optional `SubjectID`. Post/feed schema versions remain 1; pins name the same public commit and schema hashes. Post and feed `schema_version` stay 1; pins include public revision SHA and consumed schema hashes.

- [ ] **Step 1: Add failing Go round-trip tests** for legacy post without ID, new post with ID, amendment directory parse/replay, amendment exclusion from generated feeds/stats, and feed card metadata.
- [ ] **Step 2: Run** `GOCACHE=/private/tmp/tmw-go-cache go test ./internal/publication/...`; verify expected failures.
- [ ] **Step 3: Copy/pin consumed public schemas** by exact commit SHA and SHA-256; retain `schema_version: 1`; add a separate amendment loader/replayer; enforce IDs on all daemon-created publications, retain legacy parsing, and exclude amendment files from all post counts and feeds.
- [ ] **Step 4: Run** targeted publication tests and the contract-pin verification command; verify both schema bytes/hashes match the Task 1 public commit.
- [ ] **Step 5: Commit** daemon contract files only with message `feat: round-trip publication subject identities`.

### Task 3: Add SQLite registry and lossless canonical reconstruction

**Repository:** `themachinewrites-daemon`

**Files:**
- Create: `internal/store/migrations/011_subject_registry.sql`, `internal/store/subjects.go`, `internal/store/subjects_test.go`
- Modify: `internal/store/migrations.go`, `internal/store/migrations_test.go`, `internal/publication/corpus.go` integration tests

**Interfaces:**
- Produces `SubjectRegistry` methods: `CreateSubject(ctx, label, randomReader) (Subject, error)`, `GetSubject(ctx, subjectID) (Subject, bool, error)`, `AddSubjectAlias(ctx, subjectID, alias) error`, `RebuildSubjects(ctx, records []CanonicalSubjectRecord) error`, `RecordSubjectAmendment(ctx, amendment SubjectIdentityAmendment) error`, and `ResolvePostSubject(ctx, postID) (subjectID string, found bool, err error)`. `CanonicalSubjectRecord` contains post ID, published subject ID, display label/title, timestamp, and an optional amendment; it avoids coupling the store to publication parsing.
- Subject IDs are generated as `sub_` plus 32 lowercase hex chars from `crypto/rand`; generation is injectable for deterministic tests. Reject collision rather than changing an existing ID.

- [ ] **Step 1: Write failing store tests** for create/reuse, ID uniqueness/shape, alias de-duplication, reopen persistence, foreign-key integrity, failed transaction rollback, and reconstruction into a fresh database.
- [ ] **Step 2: Run** `GOCACHE=/private/tmp/tmw-go-cache go test ./internal/store -run 'Subject|Migration'`; verify missing methods/schema failures.
- [ ] **Step 3: Add versioned migration** with subjects, aliases, effective post assignments, and amendment index; published posts plus separate administrative amendments remain the recovery authority. Unpublished subjects/aliases exist only in SQLite and may be lost; the existing outbox handles pending attempts, and only Git-confirmed posts are rebuilt after a DB loss.
- [ ] **Step 4: Implement deterministic rebuild** from canonical posts plus admin amendment files: preserve every embedded ID, replay amendments in `(amended_at, amendment_id)` order, derive labels from canonical publication labels/titles, and reject malformed/conflicting references. Never assign replacement IDs to existing records.
- [ ] **Step 5: Run** targeted store tests plus `go test -race ./internal/store`; verify a fresh DB recovers all published IDs/effective mappings/amendments exactly; unpublished rows are absent and never assigned substitute identities; pending attempts follow existing outbox rules.
- [ ] **Step 6: Commit** store migration/registry code and tests only with message `feat: persist and rebuild editorial subject registry`.

### Task 4: Implement a versioned extensible topic taxonomy

**Repository:** `themachinewrites-daemon`

**Files:**
- Create: `internal/editorial/topics.go`, `internal/editorial/topics_test.go`
- Modify: `internal/editorial/modes.go` only if current topic helpers belong there; do not change mode selection

**Interfaces:**
- Produces `NormalizeTopic(raw string, taxonomyVersion string) (NormalizedTopic, error)`, where `NormalizedTopic` contains `Canonical`, `Parent`, and `Original`.
- Produces `NormalizeTopics(raw []string, taxonomyVersion string) ([]NormalizedTopic, error)` with canonical duplicate removal per publication while retaining original tags on publication records.

- [ ] **Step 1: Add failing tests** for true aliases (`ai` and `artificial-intelligence` canonicalize together), hierarchy (machine-learning remains distinct under artificial-intelligence; large-language-models remains distinct under machine-learning), overlapping topics, unknown valid topics, invalid tags, and stable version-specific classification.
- [ ] **Step 2: Run** `GOCACHE=/private/tmp/tmw-go-cache go test ./internal/editorial -run Topic`; confirm failures.
- [ ] **Step 3: Implement a small embedded taxonomy** with explicit version ID and pass-through behavior for unknown valid categories; no network lookup or runtime mutation of old taxonomy versions.
- [ ] **Step 4: Run** topic tests and ensure input tag order/original strings are preserved outside normalized metrics.
- [ ] **Step 5: Commit** taxonomy and tests with message `feat: normalize editorial topics by taxonomy version`.

### Task 5: Derive reproducible editorial history profiles

**Repository:** `themachinewrites-daemon`

**Files:**
- Create: `internal/editorial/history.go`, `internal/editorial/history_test.go`, `internal/editorial/diversity.go`, `internal/editorial/diversity_test.go`
- Modify: `internal/publication/corpus.go` only for explicit correction/amendment lookup helpers, if necessary

**Interfaces:**
- Produces `BuildProfile(posts []publication.PostRecord, now time.Time, window Window, taxonomyVersion string, minimumSample int) Profile`; `now` is explicit so time-window tests are deterministic.
- `Window` supports `last_10`, `last_30`, `last_100`, `last_7_days`, `last_30_days`, `last_90_days`, and `all`.
- `Profile` exposes `HistoryState` (`no_history`, `sparse_history`, `sufficient_history`), `StatisticalStatus` (`insufficient_history`, `ready`), sample count, topic coverage, primary-topic distribution, mode distribution, concentration pointer, exploration numerator/denominator/rate, repeated-subject and meaningful-follow-up counts, unresolved question/prediction/follow-up counts, dominant and evidence-backed underexplored topic candidates, and taxonomy version.

- [ ] **Step 1: Add table-driven failing tests** for empty corpus; sample sizes 1, min-1, min; each count/time window; sorting/tie boundaries; alias dedupe; overlapping topic coverage; mode and primary-topic distributions; exploration denominator; effective subject repetition after replaying amendments vs follow-up; admin-record exclusion from every metric; and no fabricated history from source categories.
- [ ] **Step 2: Add exact entropy tests**: for `N=0`, concentration absent; `K=1`, concentration 1; equal two-category, concentration 0; 3:1 two-category, `1 - [-.75 ln .75 - .25 ln .25]/ln 2`; and sparse sample, no model-facing concentration/imbalance.
- [ ] **Step 3: Run** `GOCACHE=/private/tmp/tmw-go-cache go test ./internal/editorial -run 'Profile|Entropy|Window|History'`; confirm failures.
- [ ] **Step 4: Implement pure deterministic profile functions** over canonical records; normalize `primary_topic` only for entropy, use multi-topic tags only for coverage, and preserve history state/sample counts.
- [ ] **Step 5: Run** targeted tests and `go test -race ./internal/editorial`; compare profile output across repeated builds and taxonomy-version selection.
- [ ] **Step 6: Commit** profile/metric code and tests with message `feat: derive editorial diversity profiles from publications`.

### Task 6: Configure sample threshold and bounded diversity policy

**Repository:** `themachinewrites-daemon`

**Files:**
- Modify: `internal/config/config.go`, `internal/config/validate.go`, `config.example.yaml`
- Create/modify: `internal/config/editorial_diversity_test.go`, `internal/editorial/diversity.go` and tests

**Interfaces:**
- Config section `editorial_diversity` has `enabled`, `history_windows`, `minimum_sample` (default 10, range 2–100), `exploration.enabled`, `exploration.probability` (default 0.25, range 0–1), `repetition.enabled`, `repetition.subject_cooldown` (default 24h), `diversity_ordering_fraction` (default 0.15, range 0–0.25), and `diagnostics` level.
- Produces `OrderCandidates(candidates []Candidate, profile Profile, policy Policy, rng Random) (ordered []Candidate, Decision, error)`; preserves all candidates and uses `0` promotions when fraction is zero or `n < 2`, otherwise `min(n-1, max(1, floor(n*fraction)))`; this allows one promotion in small nontrivial pools and respects the configured fraction for larger pools. RNG is injectable; stable ties use candidate ID.

- [ ] **Step 1: Add failing config tests** for defaults, minimum sample boundaries, invalid probability/fraction, negative windows/cooldown, zero-weight/no-reorder settings, and strict YAML unknown-key rejection.
- [ ] **Step 2: Add failing policy tests** with seeded RNG for exploration/exploitation, disabled baseline order, soft cooldown, stable ties, exact promotion counts for 0/1/2/large pools, all-candidate retention, reproducibility, and follow-up eligibility.
- [ ] **Step 3: Run** focused config/editorial tests and verify failures.
- [ ] **Step 4: Implement config validation and ranking**; diversity contributes only bounded candidate ordering and never creates/removes evidence candidates or changes mode selection.
- [ ] **Step 5: Run** focused tests, `go test -race ./internal/config ./internal/editorial`, and config example decode/validation.
- [ ] **Step 6: Commit** configuration and selection logic/tests with message `feat: configure bounded editorial diversity`.

### Task 7: Integrate subject continuity and compact profiles into the editorial cycle

**Repository:** `themachinewrites-daemon`

**Files:**
- Modify: `internal/engine/proposal.schema.json` and proposal/parser files
- Modify: existing observations bundle package and its tests
- Modify: `internal/daemon/runtime.go`, `internal/daemon/runtime_test.go`, `internal/daemon/editorial_modes_test.go`
- Modify: `internal/publication/validate.go`, `write.go`, tests

**Interfaces:**
- New publication proposal carries `subject_id` if selecting a supplied registry subject, or `subject_label` if proposing a new one; a new ID is allocated by Go only after validation, not supplied/generated by the model.
- Nonpublication proposal carries neither identity field; publish validation requires final `subject_id`.
- Bundle carries profile state/sample/taxonomy, capped subject candidates, explicit evidence/repetition/follow-up diagnostics, and exploration branch; never prompts with hidden reasoning or private execution traces.

- [ ] **Step 1: Add mocked Codex tests** for existing subject selection, new identity proposal, unknown/unoffered subject rejection, close title but distinct subject, defer with null identity fields, follow-up reuse with new evidence, correction retaining subject ID, and no publication when evidence is insufficient.
- [ ] **Step 2: Run** focused engine/daemon/publication tests and verify failures.
- [ ] **Step 3: Build bounded context** from the registry and canonical corpus; rebuild the registry at startup, use deterministic exact/alias/source/related/narrow-topic retrieval, with stable tie order and conservative distinct-subject fallback.
- [ ] **Step 4: Integrate profile and policy** in the existing runtime path while leaving explicit weighted mode selection unchanged; validate candidate IDs and final generated ID in Go before publication transaction.
- [ ] **Step 5: Add structured diagnostics** for history state/sample, taxonomy/config version, candidate IDs, match/repetition/follow-up signals, exploration branch, reorder change, selected subject, and publish/defer result; assert logs omit prompts, body text, credentials, and hidden reasoning.
- [ ] **Step 6: Run** targeted packages, then `GOCACHE=/private/tmp/tmw-go-cache go test ./...`; verify no real Codex/network or publisher credential is used.
- [ ] **Step 7: Commit** runtime integration and documentation prompt/schema assets with message `feat: use subject history in editorial research`.

### Task 8: Update private website/API types and contract pins

**Repository:** `themachinewrites-private`

**Files:**
- Modify: `functions/_lib/types.ts`, `functions/api/feed.ts`, `functions/_lib/post.ts`, `src/api/feed.ts`
- Modify: `tests/contracts/post.v1.schema.json`, `latest-feed.v1.schema.json`, `PIN.json`
- Test: `functions/_lib/post.test.ts`, `src/api/feed.test.ts`, `tests/contracts/compatibility.test.ts`

**Interfaces:**
- Consumes exact public schema revision from Task 1 and daemon-compatible feed payloads from Task 2.
- Exposes optional `subject_id?: string` in old/new card types without changing component layout or filtering.

- [ ] **Step 1: Add failing compatibility tests** for old card without subject ID, new card with valid ID, malformed ID, and public schema pin mismatch.
- [ ] **Step 2: Run** `npm test -- --run`; verify expected contract/type failure.
- [ ] **Step 3: Update types, validators, and pinned schema hashes** using exact public commit; keep the field data-only with no new visual behavior.
- [ ] **Step 4: Run** `npm test -- --run && npm run typecheck && npm run lint && npm run build`; confirm staged pnpm workspace files remain unchanged and unstaged by this task.
- [ ] **Step 5: Commit** only owned website/API/test contract files with message `feat: accept publication subject identities`.

### Task 9: Cross-repository recovery, rollback, docs, and acceptance

**Repositories:** all three child repositories plus parent `_plan` evidence only.

**Files:**
- Modify: daemon `docs/EDITORIAL-MODES.md`, `docs/RECOVERY.md`, README/config example as needed
- Modify: public contract documentation from Task 1 if test learnings require it
- Modify: private `README.md` only if API contract documentation belongs there
- Create: `_plan/evidence/FEAT-035/acceptance.md`, task-level evidence as ledger tasks are defined
- Update: `_plan/PROGRESS.md` and `tasks.html` only when implementation tasks actually change status

**Interfaces:**
- Recovery authority: canonical posts plus append-only identity amendments; SQLite registry is rebuildable.
- Rollback: revert code/config and regenerate DB from canonical corpus; never remove/alter public identity metadata/amendments or rewrite old posts.

- [ ] **Step 1: Add disposable-repository integration test**: publish fixture posts with assigned IDs, create fresh SQLite, rebuild, apply amendment, reopen, and assert original markdown bytes unchanged plus effective assignment correct.
- [ ] **Step 2: Add cross-repository compatibility test** that validates the same subject and amendment fixtures against public validator, daemon parser/schema pin, and private feed/API validator.
- [ ] **Step 3: Document migration and rollback** including schema version bump, additive reader compatibility, database loss recovery, malformed amendment handling, and rollback preserving canonical records.
- [ ] **Step 4: Run public** `npm test && npm run validate`; **daemon** `GOCACHE=/private/tmp/tmw-go-cache go test -race ./... && go vet ./... && go build ./cmd/themachinewrites`; **private** `npm test -- --run && npm run typecheck && npm run lint && npm run build`; and all contract-pin checks.
- [ ] **Step 5: Review diffs and Git state** in each repository; confirm the private staged workspace files were not changed or included, public corpus remains empty, and no deployment/live publication occurred.
- [ ] **Step 6: Record exact commands, SHAs, and limitations** in FEAT-035 evidence and the project progress ledger; regenerate `tasks.html` and run generator `--check` after ledger status updates.
- [ ] **Step 7: Commit** docs/evidence only in their owning repositories where appropriate; parent `_plan` remains a non-Git ledger.

## Acceptance Criteria

| Requirement | Acceptance evidence |
|---|---|
| Stable identity | Random ID shape/uniqueness tests; same subject reuse via explicit match; DB reopen preserves exact ID; near-title distinct subjects stay separate. |
| Recovery | Rebuild a fresh SQLite DB and recover exact published IDs/effective assignments/amendments from canonical posts plus admin records; unpublished local state is explicitly not recoverable; original bytes unchanged. |
| Corrections | Correction/follow-up `related_ids` remain independent of subject identity; admin amendment is auditable, excluded from post counts/feeds, and never rewrites its target. |
| Taxonomy | True aliases map to one category; hierarchy retains distinct children; multi-topic, raw-tag, version, and unknown-topic tests pass. |
| Cold start and sparse history | Empty corpus reports `no_history` plus `insufficient_history`; 1 through min-1 report `sparse_history`; configured source categories do not appear as measured history. |
| Entropy | Exact formula and deterministic edge cases pass for no history, one category, equal categories, skewed categories, aliases, and sparse samples. |
| Diversity policy | Seeded exploration reproducible; ordering promotion is bounded and soft; explicit mode and candidate set remain unchanged; relevant follow-up remains eligible; abstention remains valid. |
| Contracts | Post/feed schema v1 remains additive; separate amendment v1 is strictly validated; all repos pin the same exact public revision and consumed schema hashes; legacy/new records work and malformed metadata is rejected. |
| Operations | Migration, DB rebuild, rollback, diagnostics, privacy boundary, and no-live-test constraints documented and verified. |
| Regression | Existing publishing, source ingestion, correction, feed, runtime, and website suites pass; public corpus remains unchanged except explicit approved publications. |

## Plan Self-Review

- **Spec coverage:** Subject identity, registry, matching, corrections, topic taxonomy, all requested windows, concentration, sparse states, repetition/follow-up continuity, exploration, config, diagnostics, three-repository contract compatibility, docs, recovery, and regression acceptance are assigned to Tasks 1–9.
- **Step granularity:** Each task has failing tests, a focused implementation boundary, verification, and a repository-scoped commit. Contract and runtime interfaces are listed where later tasks depend on them.
- **Type consistency:** Registry methods, profile/status fields, ID format, proposal fields, and amendment field names are defined once and reused in later tasks.
- **Review focus:** Database-loss recovery, conservative matching, sparse history, multi-topic counting, follow-up eligibility, explicit modes, and abstention each have specific tests.
- **Scope/proportion:** The work is split by contract, daemon capabilities, website compatibility, and integrated acceptance; no new orchestration framework, database, vector search, or visual redesign is included.
