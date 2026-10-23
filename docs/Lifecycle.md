## FEAT-036 complete

Tasks 0–13 are complete. All three isolated worktrees are clean, and the independent final review passed with no blockers. Task-by-task evidence is recorded in FEAT-036 progress (_plan/evidence/FEAT-036/progress.md), with detailed Task 11 acceptance results (_plan/evidence/FEAT-036/task-11-acceptance.md) and final review evidence (_plan/evidence/FEAT-036/task-13-final-acceptance.md). tasks.html was regenerated and verified current.

### Final repository commits

| Repository | Branch | Final commit |
|---|---|---|
| Public | feat/feat036-lifecycle-public | 30697408acb9c3f8685ca895d2239ce9538a051f |
| Daemon | feat/feat036-lifecycle-daemon | e13e22d959e7538b43f7737b67494197d981eef3 |
| Private | feat/feat036-lifecycle-private | 0ddc568eb9e6d561a956aa2939812d4c92509ca4 |

The implementation commits are in the corresponding isolated histories; no branches were merged or pushed. The exact public schema source revision is 52d041ed09a3c0aab29fea209ad1c5d98a35aa3c. All three contract pins match, and all ten schema files have identical bytes and SHA-256 hashes.

### Lifecycle behavior

┌────────────────┐
│ ● initial      │
│                ├────────────────────────────────┐
└────────────────┘                                │
                                                  │
                                                  │
┌────────────────┐                                │
│ draft          │                                │
│                ├◄───────────────────────────────┘
│                ├──────────────────────────────────┐
└────────────────┘                                  │
                                                    │
                                                    │
┌────────────────┐                                  │
│ ready          │                                  │
│                ├◄─────────────────────────────────┘
│                ├────────────────────────────────────┐
│                ├◄───────────────────────────────────╪─┐
│                ├────────────────────────────────────╪─╪───┐
└────────────────┘                                    │ │   │
                                                      │ │   │
                                                      │ │   │
┌────────────────┐                                    │ │   │
│ scheduled      │                                    │ │   │
│                ├◄───────────────────────────────────┘ │   │
│                ├──cancel──────────────────────────────┘   │
│                ├──due───────────────────────────────────┐ │
└────────────────┘                                        │ │
                                                          │ │
                                                          │ │
┌────────────────┐                                        │ │
│ publishing     │                                        │ │
│                ├◄───────────────────────────────────────┘ │
│                ├◄─────────────────────────────────────────┘
│                ├──remote confirmed──────────────────────────┐
│                ├──failure or uncertain result───────────────╪─┐
│                ├◄───────────────────────────────────────────╪─╪─┐
└────────────────┘                                            │ │ │
                                                              │ │ │
                                                              │ │ │
┌────────────────┐                                            │ │ │
│ published      │                                            │ │ │
│                ├◄───────────────────────────────────────────┘ │ │
│                ├──confirmed operator event────────────────────╪─╪─┐
│                ├◄─────────────────────────────────────────────╪─╪─╪─┐
└────────────────┘                                              │ │ │ │
                                                                │ │ │ │
                                                                │ │ │ │
┌────────────────┐                                              │ │ │ │
│ publish_failed │                                              │ │ │ │
│                ├◄─────────────────────────────────────────────┘ │ │ │
│                ├──reconcile and retry───────────────────────────┘ │ │
└────────────────┘                                                  │ │
                                                                    │ │
                                                                    │ │
┌────────────────┐                                                  │ │
│ retracted      │                                                  │ │
│                ├◄─────────────────────────────────────────────────┘ │
│                ├──confirmed restoration event───────────────────────┘
└────────────────┘

Revision and supersession links remain separate from lifecycle state. A retracted replacement does not reactivate its predecessor; restoring a superseded ancestor does not make it active. A new explicit revision can establish a new terminal leaf. Published records and events remain append-only.

The daemon supports continuous autonomous operation, one-shot prompted drafts, --draft-only and explicitly authorized --publish, plus post listing, inspection, validation, editing, publishing, scheduling, retraction, restoration, revision, and history commands. Retraction and restoration require --confirm and a reason. Prompted publications retain editorial_origin=operator_prompt independently from publication_kind.

./daemon-runner
./daemon-runner --editorial-mode=philosophy
./daemon-runner --prompt "Write a short introductory Hello World post."
./daemon-runner --prompt-file ./prompts/first-post.md --draft-only
./daemon-runner --prompt "Write an introductory post." --publish

./daemon-runner posts list --status=draft
./daemon-runner posts show POST_ID
./daemon-runner posts validate POST_ID
./daemon-runner posts publish POST_ID
./daemon-runner posts schedule POST_ID --at 2026-10-24T12:00:00Z
./daemon-runner posts unschedule POST_ID
./daemon-runner posts retract POST_ID --reason "Material factual error" --confirm
./daemon-runner posts restore POST_ID --reason "Withdrawal basis resolved" --confirm
./daemon-runner posts revise POST_ID --prompt "Clarify the evidence" --request-id review-7
./daemon-runner posts history POST_ID

### Verification

- Public: npm test — 81 passed; npm run validate — 24 fixtures passed; canonical corpus remains at zero publications and zero amendments.
- Daemon: uncached go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., and go build ./... passed. Integration tests use temporary SQLite databases and disposable local bare Git remotes.
- Private: build, typecheck, lint, and tests passed — 69 passed, 1 skipped. UI and API tests cover lifecycle notices, history disclosure, revision/supersession relationships, mixed snapshot rejection, and bounded cache freshness.
- Independent reviews: Gates A–D and final acceptance passed. Gate C’s superseded-ancestor revision issue and Gate D’s combined retracted/superseded display issue were fixed and re-reviewed.

If a remote push succeeds but SQLite acknowledgment fails, the pending operation retains its stable intent. A retry checks the configured remote first and acknowledges the matching canonical event and projections without adding duplicates. Unpublished event envelopes can be rebuilt against the latest ledger head; canonical remote events cannot be changed. Public history and lifecycle projections can be reconstructed from Git without SQLite. Private drafts and unpublished local operation state require the local encrypted SQLite backup.

No hosted push, deployment, live publication, or production-state change occurred. The original private repo’s staged pnpm-lock.yaml and pnpm-workspace.yaml, the daemon’s untracked implementation plan, and the FEAT-035 integration worktrees were preserved.

The remaining verification limits are that the full private checks ran on Node 24.12.0 with dependency engine warnings, and no browser screenshot comparison was performed. The independent reviewer inspected the integration but did not rerun the suites; executable test results are recorded separately. Deployment still requires the operator’s normal coordinated rollout and verification of live Cloudflare/API cache behavior.
