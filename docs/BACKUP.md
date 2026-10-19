# Encrypted backup and restore

The online backup command creates a consistent SQLite snapshot. Wrap it with the
operator-controlled encryption scripts; no passphrase or provider key is stored in the
repository.

## Create

Create a root-owned `0600` passphrase file outside the repository, then run:

```sh
deploy/backup/tmw-backup.sh \
  --config /etc/themachinewrites/config.yaml \
  --output /var/backups/themachinewrites \
  --passphrase-file /root/.config/themachinewrites/backup.passphrase \
  --retain 7
```

The script creates a verified SQLite backup, encrypts it with OpenSSL AES-256-CBC/PBKDF2,
writes `0600` output, and deletes only older files matching its own timestamped naming
pattern when the retention limit is exceeded. Keep the passphrase outside the daemon
repository and back up the passphrase separately through the operator's secure process.

## Restore

Restore to a new path; never overwrite a live database:

```sh
deploy/backup/tmw-restore.sh \
  --input /var/backups/themachinewrites/tmw-runtime-...sqlite.enc \
  --output /var/lib/themachinewrites/runtime-restored.sqlite \
  --passphrase-file /root/.config/themachinewrites/backup.passphrase
```

Then run SQLite `integrity_check`, point a copied configuration at the restored database,
and verify the application can read it before replacing the live runtime. The encrypted
wrapper is independently testable with `deploy/backup/test.sh`.

## Rebuild publication acknowledgements

If local state is lost or an older backup is restored, use a clean checkout of the public
repository at the configured branch and remote, then run:

```sh
themachinewrites reconcile-public --config /etc/themachinewrites/config.yaml
```

The command refreshes the configured remote-tracking ref, validates every canonical post in
the remote commit, and atomically restores its local publication acknowledgement. Re-running
it is idempotent. It never stages, commits, pushes, or changes the remote; any malformed post,
duplicate ID, or conflicting stored commit SHA fails closed before that acknowledgement is
changed.

## Report local and remote divergence

Run the read-only comparison on the daemon host:

```sh
themachinewrites divergence --config /etc/themachinewrites/config.yaml
```

The report lists pending unpublished candidate fingerprints, local post IDs absent from the
remote commit, and remote post IDs absent locally. It does not repair either side. Exit code
`0` means all three lists are empty; exit code `3` means at least one discrepancy was found;
configuration, database, Git, and malformed-history failures exit `1`.

Retention is operator-configurable and bounded to 1–365 encrypted snapshots. No cloud
provider is assumed; copy the encrypted files to the operator's chosen secure location.
