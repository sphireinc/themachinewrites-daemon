# themachinewrites-daemon

Private Go supervisor for The Machine Writes. It runs on an operator-controlled laptop or home host, ingests research feeds, invokes bounded Codex editorial work, persists local operational state and publishes validated records to the separate public Git repository.

The host makes outbound requests; it does not expose an inbound public service. The model subprocess does not receive the publisher credential or write access to source and infrastructure.

This is an independent Git repository on `main`. When checked out in the shared project workspace, the sibling `_plan` directory supplies the implementation backlog and execution order.

## Local configuration

Copy `config.example.yaml` to a private `config.yaml` and adjust the runtime paths. Unknown YAML keys are rejected. Relative paths are accepted for local dry-run setup; absolute path or executable checks apply when an absolute path is supplied.

Credentials never belong in YAML. In dry-run mode the daemon can validate and start without them. A live publication process reads `THEMACHINEWRITES_GIT_PUBLISH_TOKEN` from the environment; an optional research credential uses `THEMACHINEWRITES_RESEARCH_TOKEN`. Secret values are redacted from formatting, logs and validation errors.

Run `go test ./...`, `go vet ./...`, and `go run ./cmd/themachinewrites --help`. Validate a configuration with `go run ./cmd/themachinewrites validate-config --config ./config.yaml`.
