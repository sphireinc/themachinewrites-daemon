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
- Config overrides `approval_policy="never"`, `web_search="disabled"` and
  `sandbox_workspace_write.network_access=false`

Run `codex --version` and `codex exec --help` after an upgrade. The daemon probe fails readiness below minimum version `0.163.0` or when any required flag disappears.

The adapter also runs the required policy override set as a probe with `--help`, so an
unsupported config key blocks launch before model work begins. The daemon provides a
dedicated empty workspace and synthetic home/temp directories, strips every parent
environment variable except its documented allowlist, and replaces `HOME`, `TMPDIR` and
the publisher token. These process-level controls remove accidental credential exposure;
the installed Codex read-only sandbox enforces filesystem reads.

Host-level isolation remains an operator requirement. Process environment filtering and
dedicated directories do not answer an arbitrary absolute-path read by a test child that
does not use the Codex sandbox. The service must run under a dedicated OS account or container that cannot read
the operator home, SSH/Git credential stores, other daemon secrets or private sibling
repositories. Record that boundary during deployment; unit tests alone do not prove it.
