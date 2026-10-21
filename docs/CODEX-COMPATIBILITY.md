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

## Dedicated Codex home: login and dry run

`codex_home` in the daemon configuration is the absolute path used as `CODEX_HOME` by the
Codex CLI child. It must be an existing real directory outside the paths denied by the
daemon's outer sandbox, separate from the Codex executable installation and the operator's
general `~/.codex` home. Config paths do not expand `~`; use an absolute `public_repo_path`
to point at a repository under the user's home. When the public repository is under
`SphireProjects`, the daemon's outer sandbox denies reads under its parent directory. Put
both the dedicated Codex home and the daemon runtime directory outside that tree: the runtime
directory contains the temporary workspace, schema and output files that Codex must access.
The daemon does not copy or inherit the
operator's Codex login. Authenticate this dedicated home as the same OS identity that will
run the daemon.

The daemon also applies an outer macOS sandbox to the Codex CLI process, while `codex exec
--sandbox read-only` applies Codex's own sandbox. A harmless sandbox probe succeeded in the
Operator's regular Terminal. A direct `codex exec` using the same model and current proposal
schema also succeeded, returned a valid defer proposal, and wrote no publication. The daemon's
current configuration instead puts `runtime_dir: ./runtime` and `codex_home` under the project
root, while the absolute public repository path causes the outer sandbox to deny reads under
that root. This covers the temporary proposal schema and authenticated Codex home. The
`structured_output_schema` category in the `once` log is not conclusive evidence of an invalid
API schema; the actual failure is consistent with the outer read denial.

For this checkout on macOS, use `/Users/JuanSanchez/.local/share/themachinewrites-codex` for
the authenticated Codex home and `/Users/JuanSanchez/Library/Application Support/TheMachineWrites/runtime`
for runtime state. Move the existing directories to preserve login state and the SQLite
runtime database, then set both `codex_home` and `runtime_dir` in `config.yaml` to those
absolute paths. Check first that each destination does not already exist:

```sh
mkdir -p "$HOME/.local/share"
mv "$HOME/SphireProjects/TheMachineWrites/codex" "$HOME/.local/share/themachinewrites-codex"
mkdir -p "$HOME/Library/Application Support/TheMachineWrites"
mv "$HOME/SphireProjects/TheMachineWrites/themachinewrites-daemon/runtime" "$HOME/Library/Application Support/TheMachineWrites/runtime"
```

No separate Codex setting is needed: the daemon reads `codex_home` and exports it as
`CODEX_HOME` to the child process. Verify the moved login before retrying:

```sh
CODEX_HOME="$HOME/.local/share/themachinewrites-codex" codex login status
```

### Reading `once` diagnostics

`go run ./cmd/themachinewrites once --config ./config.yaml` prints human-readable phase
milestones to stdout. Structured JSON stage events go to stderr and identify whether the
failure occurred during source fetching, candidate selection, Codex execution, or publication.
For a Codex subprocess failure, the `error` field includes a fixed diagnostic category such
as `native_sandbox_apply` rather than the child's raw stderr. Prompts, fetched article content,
and credentials are not logged.

With the reviewed `config.yaml` installed, substitute its exact `codex_home` value in these
commands (do not put authentication material in YAML or command arguments):

```sh
CODEX_HOME="/absolute/path/from/config.yaml" codex login
CODEX_HOME="/absolute/path/from/config.yaml" codex login status
```

Proceed when `codex login status` confirms a login. For a local one-cycle dry run, first
ensure `dry_run: true` and that the configured `sources_path` points to a reviewed source
file. From the daemon repository, run:

```sh
go run ./cmd/themachinewrites validate-config --config ./config.yaml
go run ./cmd/themachinewrites once --config ./config.yaml
```

`once` fetches configured feeds and invokes the model, so it uses network access and may
consume model budget. With `dry_run: true`, it does not initialize the publisher or publish
to the public repository; it can still write local runtime state. Check the command output
and configured health/runtime files for the cycle result. A successful `validate-config`
alone does not prove Codex authentication or source fetch readiness.

For a persistent LaunchAgent or systemd service, prepare/authenticate the same dedicated
`CODEX_HOME` under the service's OS identity before starting the service. Ensure host sandbox
and filesystem policy allow that identity to read the Codex home. See [macOS operation](MACOS.md)
and [Linux operation](LINUX.md).

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
