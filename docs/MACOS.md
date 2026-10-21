# macOS LaunchAgent operation

Install the compiled daemon and reviewed configuration outside the operator checkout. The example
LaunchAgent starts the service at login and restarts an unsuccessful process after a throttle.

1. Build and test:

   ```sh
   go test -race ./...
   go vet ./...
   go build -trimpath -o /tmp/themachinewrites ./cmd/themachinewrites
   sudo install -d -m 0755 /usr/local/bin /usr/local/etc/themachinewrites /usr/local/var/themachinewrites /usr/local/var/log
   sudo install -m 0755 /tmp/themachinewrites /usr/local/bin/themachinewrites
   ```

2. Install reviewed `config.yaml` and `sources.yaml` with restrictive permissions. Do not put
   secrets in either file.

   Place the configured dedicated `CODEX_HOME` outside the project root and denied-read paths.
   Before loading the LaunchAgent, authenticate it as the same macOS user that will run the
   agent and verify it with `codex login status`. Follow
   [Codex login and dry-run setup](CODEX-COMPATIBILITY.md#dedicated-codex-home-login-and-dry-run).

3. Make secrets available to launchd through a protected operator-owned wrapper or a dedicated
   service account/keychain policy. Do not paste secrets into the plist. The required environment
   key is `THEMACHINEWRITES_GIT_PUBLISH_TOKEN`; see `SECRETS.md`.

4. Install and validate the plist:

   ```sh
   cp deploy/launchd/com.themachinewrites.daemon.plist "$HOME/Library/LaunchAgents/"
   plutil -lint "$HOME/Library/LaunchAgents/com.themachinewrites.daemon.plist"
   launchctl bootstrap gui/"$(id -u)" "$HOME/Library/LaunchAgents/com.themachinewrites.daemon.plist"
   launchctl print gui/"$(id -u)"/com.themachinewrites.daemon
   ```

5. Stop and unload:

   ```sh
   launchctl bootout gui/"$(id -u)" "$HOME/Library/LaunchAgents/com.themachinewrites.daemon.plist"
   ```

Review stdout/stderr under `/usr/local/var/log`; the installed daemon also writes JSON health to
the configured runtime path. Record whether a dedicated account/container or native macOS sandbox
deny list is active before live publication.
