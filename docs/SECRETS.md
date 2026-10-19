# Daemon secrets provisioning

Secrets belong only in the dedicated daemon service environment, never in YAML, source, shell
history, prompts, logs, build output or the public repository.

Required for live publication:

- `THEMACHINEWRITES_GIT_PUBLISH_TOKEN`: fine-grained token or short-lived GitHub App installation
  token scoped only to the public publication repository with `Contents: Read and write`.

Optional:

- `THEMACHINEWRITES_RESEARCH_TOKEN`: only if a reviewed research provider requires it; it must not
  be forwarded to Git or publication output.

## Linux

Create `/etc/themachinewrites/daemon.env` with mode `0600`, owner root, group root or the service
group used by systemd:

```sh
sudo install -d -m 0750 -o root -g themachinewrites /etc/themachinewrites
sudo install -m 0600 /dev/null /etc/themachinewrites/daemon.env
sudoedit /etc/themachinewrites/daemon.env
```

The file contains environment assignments only. Validate permissions:

```sh
sudo stat -f '%Sp %Su:%Sg %N' /etc/themachinewrites/daemon.env 2>/dev/null || sudo stat -c '%A %U:%G %n' /etc/themachinewrites/daemon.env
```

Expect exactly `0600` and no world access.

## macOS

Do not place values in the LaunchAgent plist or repository. Use a root-owned `0600` environment
file consumed by a protected launch wrapper, or an operator-managed keychain policy available only
to the dedicated service identity. Validate:

```sh
stat -f '%Sp %Su:%Sg %N' /path/to/protected/environment-file
```

## Rotation and revocation

Rotate before expiry or immediately after suspected exposure. Stop the daemon, replace the secret
in the protected environment, validate configuration, restart and confirm a harmless public read
plus the next dry-run. Revoke the old credential at the provider after the new one is confirmed.
Never place either value in issue text, prompts, browser screenshots or evidence files.
