# themachinewrites-daemon

Private Go supervisor for The Machine Writes. It runs on an operator-controlled laptop or home host, ingests research feeds, invokes bounded Codex editorial work, persists local operational state and publishes validated records to the separate public Git repository.

The host makes outbound requests; it does not expose an inbound public service. The model subprocess does not receive the publisher credential or write access to source and infrastructure.

This is an independent Git repository on `main`. When checked out in the shared project workspace, the sibling `_plan` directory supplies the implementation backlog and execution order.

## Local configuration

Copy `config.example.yaml` to a private `config.yaml` and adjust the runtime paths. Unknown YAML keys are rejected. `codex_home` must be an absolute path to an existing real directory outside the paths denied by the daemon's outer sandbox. Config paths do not expand `~`; use an absolute `public_repo_path` to point at a repository under the user's home.

Credentials never belong in YAML. Dry-run mode does not require the publisher token, but the dedicated `codex_home` must be authenticated for Codex to run. Keep both `codex_home` and `runtime_dir` outside the denied tree. When `public_repo_path` is under this project root, the outer sandbox denies file reads under the project root; a runtime directory below the daemon checkout then prevents Codex from reading its temporary output schema, and a Codex home there prevents it from reading its login state. Use external paths such as `$HOME/.local/share/themachinewrites-codex` and `$HOME/Library/Application Support/TheMachineWrites/runtime`. The current operator run's `structured_output_schema` category is broad; a direct probe using the same model and schema succeeded, so that category alone is not proof of an invalid schema. See [Codex login and dry-run setup](docs/CODEX-COMPATIBILITY.md#dedicated-codex-home-login-and-dry-run). A live publication process reads `THEMACHINEWRITES_GIT_PUBLISH_TOKEN` from the environment; an optional research credential uses `THEMACHINEWRITES_RESEARCH_TOKEN`. Secret values are redacted from formatting, logs and validation errors.

Run `go test ./...`, `go vet ./...`, and `go run ./cmd/themachinewrites --help`. Validate a configuration with `go run ./cmd/themachinewrites validate-config --config ./config.yaml`. Rebuild local publication acknowledgements after restoring an old backup with `go run ./cmd/themachinewrites reconcile-public --config ./config.yaml`; use `divergence` for its read-only discrepancy report; see `docs/BACKUP.md` for the clean-host procedure.

`once` prints coarse progress milestones to stdout and structured JSON stage events to stderr. Events cover source fetches and ingestion, candidate selection, Codex execution, dry-run decisions, and confirmed publication. Codex failures include only a fixed diagnostic category (`authentication`, `native_sandbox_apply`, `sandbox_or_filesystem`, `structured_output_schema`, `network`, `rate_limit_or_quota`, or `unknown`); raw Codex stdout/stderr, prompts, article text, and credentials are not logged.
