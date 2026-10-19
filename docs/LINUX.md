# Linux host operation

Use a dedicated unprivileged service account and install the compiled daemon outside the
operator checkout. The example service reads a root-owned environment file, stores state under
systemd's private `StateDirectory`, and exposes no listen port.

1. Build the binary on the target:

   ```sh
   go test -race ./...
   go vet ./...
   go build -trimpath -o /tmp/themachinewrites ./cmd/themachinewrites
   sudo install -D -m 0755 /tmp/themachinewrites /opt/themachinewrites/bin/themachinewrites
   ```

2. Create the service account and directories:

   ```sh
   sudo useradd --system --home /var/lib/themachinewrites --shell /usr/sbin/nologin themachinewrites
   sudo install -d -m 0750 -o root -g themachinewrites /etc/themachinewrites
   sudo install -d -m 0700 -o themachinewrites -g themachinewrites /var/lib/themachinewrites
   ```

3. Install reviewed `config.yaml` and `sources.yaml`; ensure configuration contains no credential.
   Install `deploy/systemd/themachinewrites.service` under
   `/etc/systemd/system/themachinewrites.service`.

4. Create `/etc/themachinewrites/daemon.env` with mode `0600`, owner root, containing placeholder
   values replaced by an authorized operator:

   ```sh
   sudo install -m 0600 /dev/null /etc/themachinewrites/daemon.env
   sudoedit /etc/themachinewrites/daemon.env
   ```

   The required publish token key is `THEMACHINEWRITES_GIT_PUBLISH_TOKEN`; see `SECRETS.md`.

5. Validate and start:

   ```sh
   sudo systemd-analyze verify /etc/systemd/system/themachinewrites.service
   sudo systemctl daemon-reload
   sudo systemctl enable --now themachinewrites.service
   systemctl status themachinewrites.service
   journalctl -u themachinewrites.service -n 100 --no-pager
   ```

The service deliberately has no `ListenStream`/inbound port. Verify outbound feeds and Git writes
separately. If the host requires a container/OS sandbox for the Codex child, install that boundary
and rerun the filesystem canaries before enabling live publication.
