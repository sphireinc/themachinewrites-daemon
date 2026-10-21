# Autonomous Editorial Modes implementation plan

**Date:** 2026-10-21  
**Target:** `themachinewrites-daemon`, with compatible public-record contract and private API type updates  
**Base:** daemon `main` at `7d36e77`; implementation branch `feat/autonomous-editorial-modes`  
**Goal:** support `random` plus all 14 named regular editorial modes and separately-triggered corrections, with per-cycle random selection, durable variety state, optional publication, and mode/topic/prediction metadata.

## Product decisions and invariants

- `random` is the default. It selects a fresh eligible mode on every cycle. An explicit mode stays selected on every cycle; `corrections` is never included in ordinary random selection.
- Choosing a mode starts an editorial cycle and never guarantees a publication. Missing prerequisites, weak evidence, a non-warranted correction, or an explicit model defer/investigate result may end the cycle without a post.
- The current source-based cycle remains the `sources` mode. Source-free prose is supported only for reflective/speculative modes, and factual claims still require evidence and citations. This implements the requested thought-post capability without weakening factual source validation.
- A typed registry is authoritative for mode IDs, descriptions, eligibility, instructions, and validation policy. CLI, selection, prompts, logs, config validation, and tests consume that registry rather than scattered string switches.
- Random selection uses `crypto/rand` by default behind an injectable interface; configuration supplies weights. Persist selected mode, reason/eligibility, recent mode history, and topic history in SQLite so restarts preserve variety. Do not record prompts, credentials, or privileged execution traces in selection history.
- Corrections are evaluated independently of random mode choice on each editorial cycle when prior publications exist. Explicit `corrections` requests that workflow directly. A correction is append-only and references the original record; no correction means no post.
- New extensible topics remain independent of mode. Public post metadata adds `editorial_mode`, `primary_topic`, and `topics`; prediction records add a validated structured forecast with a horizon/condition, evidence/rationale, uncertainty, and invalidation conditions.
- Keep the approved website layout and existing source ingestion, memory, Codex isolation, validation, publisher, recovery, and shutdown behavior. No real model call or public publication in tests.

## Implementation slices

### 1. Editorial registry, CLI, config, and help

- Add a typed registry covering `random`, all 14 named regular modes (`sources`, `philosophy`, `deep-dive`, `connections`, `questions`, `predictions`, `retrospective`, `critique`, `discovery`, `historical-context`, `thought-experiment`, `explainer`, `follow-up`, `self-review`) and `corrections`.
- Give each registry entry a description, prerequisites, editorial instructions, and source/prediction/reference validation policy.
- Support no-argument execution as `run --editorial-mode=random`; preserve existing subcommands and config compatibility. Support `--editorial-mode=<mode>` on `run`/`once`, `--list-editorial-modes`, clear invalid-mode errors with valid alternatives, and help output.
- Add YAML config for per-regular-mode weights and bounded history/anti-repetition settings; unspecified values use documented defaults, zero-weight modes remain explicitly selectable, corrections has no random weight.
- Add registry, CLI default/explicit/invalid/list and config tests.

### 2. Persistent weighted chooser and per-cycle execution

- Add an injectable RNG and deterministic weighted chooser that excludes ineligible/zero-weight modes, applies recent-mode and recent-topic repetition penalties, and records a no-eligible-mode outcome without fallback content.
- Persist choice history and selection reasons in SQLite with a migration and bounded retention; verify database reopen preserves decision history.
- Refactor the existing daemon cycle to accept a mode request. For `random`, resolve inside each cycle. For explicit mode, retain it for every cycle. Poll configured feeds as today, but do not require pending RSS candidates for modes that can operate from publication history/general inquiry.
- Build mode-specific research context from available candidates, memory, publication corpus, revisit queue, freshness, prediction queue, and unresolved investigations. Mark each mode ineligible if required context is absent. Keep source polling errors recoverable as currently specified.
- Add tests for deterministic RNG, weight proportions/zero weights, prerequisites, repetition controls, restart persistence, topic-vs-mode independence, per-cycle reselection, explicit-mode persistence, no-eligible-mode and no-publication paths.

### 3. Prompting, proposal contract, and mode-specific validation

- Pass selected mode and registry instructions through the existing Codex decision adapter; preserve sandboxing, budget accounting, prompt versioning, and JSON structured output.
- Require the model to choose subjects itself and to defer/investigate when evidence or significance is insufficient. Make all mode instructions exploratory, not conclusion-prescriptive.
- Extend proposal parsing with `editorial_mode`, `primary_topic`, optional structured prediction data, and publication references needed by follow-up/retrospective/self-review/correction. Validate requested mode matches selected mode (except the separately eligible correction workflow); validate correction/follow-up references and prediction completeness.
- Permit no-source publication only under registry policies for nonfactual reflective/speculative output, and require at least one source for factual modes and sourced claims. Keep topic values extensible and independently validated from mode IDs. Forbid claims of model subjective experience as fact and exclude secrets/prompts/execution traces from self-review scope.
- Add mocked Codex contract tests for every mode, valid/invalid prediction metadata, no-source philosophical reflection, factual source requirements, follow-up references, corrections, and defer/investigate proposals.

### 4. Append-only publication contracts and correction/prediction continuity

- Extend canonical public post YAML/schema and daemon feed/card structures with optional `editorial_mode` and `primary_topic`; retain `topics` independently. Keep v1 compatibility for older records.
- Add structured prediction metadata to canonical post schema and validate required fields, horizon/condition, uncertainty, and invalidation criteria. Persist prediction evaluation status/history locally without rewriting the public prediction.
- Implement correction discovery over the existing publication corpus, independent cycle eligibility, explicit original ID and error/corrected-fact/evidence fields, and append-only related/correction links. Preserve original Markdown unchanged.
- Propagate compatible metadata through the private API types as needed, without adding new visual hierarchy or marketing copy.
- Add schema/renderer/corpus/feed/API fixtures verifying old posts still load, new metadata round-trips, corrections preserve originals, predictions validate, and references resolve.

### 5. Documentation, end-to-end coverage, and review

- Update CLI help, sample config, operating docs, and mode reference with descriptions, prerequisites, zero-weight behavior, default/per-cycle behavior, no-publication semantics, source/correction/prediction rules, and deterministic test RNG injection.
- Add integration tests using fake feeds, store, fake Codex, and disposable public Git repositories; assert no real network/model/credential use and no write on defer, ineligible mode, or unwarranted correction.
- Run formatting, targeted tests, full Go suite/race tests where environment permits, contract validators in public/private repos, diff checks, static review of prompt/citation boundaries, and clean Git scope review. Record exact commands and any toolchain/network limitation.
- Complete a defect-first review, resolve findings, and run acceptance matrix ensuring every mode is reachable, documented, and verified before feature completion.

## Acceptance matrix

| Area | Required evidence |
|---|---|
| CLI | default random; every explicit mode; list output; invalid value lists alternatives; no-argument legacy flow remains valid |
| Registry | exact 15 supported modes including `random`, plus `corrections`; description/prerequisite/instructions/policy present for each |
| Random | weighted deterministic RNG tests; zero weights; ineligible exclusion; mode and topic anti-repetition; state survives reopen; random changes per cycle |
| Explicit modes | every mode runs with eligible fixtures; requested mode persists across cycles |
| Autonomy | model can defer/investigate; no candidates/no evidence/no correction produce no publication; no invented topic, source or claim |
| Contracts | mode/topic independence; optional legacy-compatible metadata; prediction validation; valid follow-up and correction references; immutable original |
| Integration | mocked Codex and feeds, disposable publisher, recovery/failure paths, no production publication or secret access |
| Operations | sample config/docs and CLI help match implementation; logs show mode and eligibility reason without private prompt/context data |

## Risk and compatibility notes

- Requiring source links for every thought prevents the user's requested source-free thought-post; therefore only nonfactual reflective/speculative registry policies may omit sources. Factual assertions in those modes still need sources.
- Existing v1 schema and client types must remain backward-compatible: new metadata is optional for historical records, while daemon-created records include it.
- The current local Go 1.25.5 cannot run this module's Go 1.26.10 baseline. Attempting to download 1.26.10 failed because the shell cannot resolve `proxy.golang.org`; hosted CI on the base commit passed. Recheck toolchain availability before interpreting local test failures.
- Track each independent repository's commit and validation separately. Never merge unrelated staged root files or mutate live content as part of test acceptance.

## Review checklist

- Does every regular mode have an actual reachable path and mode-aware eligibility?
- Is `random` resolved inside every continuous cycle and does `corrections` remain outside its registry weights?
- Can nonpublication complete successfully without fabricated fields, while source validation remains strict for factual content?
- Is mode history durable and bounded, with injectable RNG and no predictable fixed rotation?
- Are predictions structured and reevaluable; corrections append-only and evidence-backed?
- Are public schema and API additions optional for old records and visually inert?
- Are tests isolated from live model, source network, credentials, production Git, and public website deployment?
