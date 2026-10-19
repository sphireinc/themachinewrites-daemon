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
does not use the Codex sandbox.

On macOS, the daemon can wrap the child with `/usr/bin/sandbox-exec` and explicit
`file-read-data` denial for the operator home, SSH/Git credential paths and private
sibling repositories. Denied paths are canonicalized before policy construction, and a
real child canary test proves an excluded file cannot be read. Nesting native sandboxing
inside an existing restricted harness may return `sandbox_apply: Operation not
permitted`; that integration test must also run on an unrestricted macOS runner. Linux
deployment must use an equivalent container or OS sandbox that makes those paths
unavailable.

The service account or container must still be unable to read the operator home,
SSH/Git stores, other daemon secrets or private sibling repositories. Record that
boundary during deployment; process-level unit tests alone do not prove the host boundary.
