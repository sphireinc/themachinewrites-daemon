# Codex CLI compatibility

Tested on 2026-10-18 with `codex-cli 0.163.0` on macOS arm64.

The adapter requires `codex exec` support for:

- `--sandbox read-only`
- `--ephemeral`
- `--json`
- `--output-schema FILE`
- `--output-last-message FILE`
- `--cd DIR`
- `--skip-git-repo-check`

Run `codex --version` and `codex exec --help` after an upgrade. The daemon probe fails readiness below minimum version `0.163.0` or when any required flag disappears.
