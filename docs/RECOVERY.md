# Host replacement and recovery

This runbook reconstructs The Machine Writes on a clean operator host. The public Git
repository is the canonical publication record. The local SQLite database is private operational
state and may lag the public repository after host loss. Never copy a live database over an
existing one, overwrite a public post, force-push publication history, or place a credential in
YAML, a command line, a prompt, or evidence.

## Preconditions

- The public repository remote is available and the operator can make a fresh clone.
- The latest encrypted SQLite backup and its separately protected passphrase are available.
- A replacement GitHub credential is provisioned for only that public repository with
  `Contents: Read and write`.
- The daemon binary, reviewed `config.yaml`, reviewed `sources.yaml`, and service recipe are
  installed under their destination paths.
- Codex access and the host-specific native sandbox have been validated independently.

## Clean-host procedure

1. Create the replacement runtime, source, clone, and protected Codex directories with restrictive
   permissions. Keep them outside source control.
2. Clone the public repository into a new path on the new host. Do not copy the old worktree or
   reuse its Git metadata.
3. Restore the encrypted SQLite snapshot to a new database path. Run `PRAGMA integrity_check`; the
   restored file must not already exist and should be mode `0600`.
4. Rewrite the daemon configuration paths for the new runtime directory, Codex home, and public
   clone. The public remote remains the reviewed credential-free HTTPS URL.
5. Re-provision `THEMACHINEWRITES_GIT_PUBLISH_TOKEN` through the service environment or keychain
   boundary. Verify scope without printing the value, then run `validate-config`.
6. Run:

   ```sh
   themachinewrites reconcile-public --config /path/to/new/config.yaml
   ```

   This validates canonical posts from the remote commit and rebuilds missing publication
   acknowledgements. Confirm the printed commit and publication count against the remote tree.
7. Run the read-only comparison:

   ```sh
   themachinewrites divergence --config /path/to/new/config.yaml
   ```

   Exit `0` means no pending publication or local/remote ID discrepancy. Exit `3` requires an
   operator decision; this command does not repair either side.
8. Start a dry-run supervisor cycle, inspect health and logs, then enable the live service under
   the reviewed systemd or launchd identity. Confirm one bounded cycle and the next status update.

## Failure behavior

The drill must be repeated rather than assumed after a material path, OS, credential model,
database schema, service-definition, or public repository changes. A failed integrity check,
malformed public history, conflicting commit SHA, unavailable remote, missing replacement
credential, or supervisor construction failure stops the drill. Record the failing stage and
operator action; do not describe the host replacement as verified until the final live cycle and
health check have passed.

The automated replacement-host fixture is `cmd/themachinewrites/recovery_drill_test.go`. It uses a
temporary bare remote and generated SQLite state, so it proves the local recovery mechanics without
claiming a production service restart or provider credential check.
