# Deployment artifacts and operator checks

The deployment recipes are reviewable starting points, not proof that a service or account has
been provisioned.

| Artifact | Local check | External verification still required |
|---|---|---|
| `deploy/systemd/themachinewrites.service` | `systemd-analyze verify` on Linux | install under dedicated account, permissions, start/restart, no inbound listener |
| `deploy/launchd/com.themachinewrites.daemon.plist` | `plutil -lint` | bootstrap as dedicated identity, protected secret injection, restart/log verification |
| `docs/SECRETS.md` | placeholders only; permission commands | actual `0600` ownership, provider scope, rotation/revocation |
| `docs/LINUX.md`, `docs/MACOS.md` | command/document review | host-specific paths, account policy, sandbox deny canaries |
| `themachinewrites-private/docs/DEPLOY.md` | local Pages build/headers | Pages project bind, production branch and variables |
| `themachinewrites-private/docs/DOMAIN.md` | prose/command review | DNS records, certificate, apex/www redirect |
| `_plan/checklists/DEPLOY-SMOKE.md` | checklist completeness | actual dry-run/publication/origin/rollback run |

The service recipes intentionally contain no credential values and no inbound listen directive.
Operator edges must use an outbound-only design and keep publisher credentials outside the Codex
child environment.
