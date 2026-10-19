# GitHub publication credentials

The daemon publisher must use a credential that can modify only one repository: the
public Git repository that stores The Machine Writes publications. The credential must
not authorize the daemon, private website, organization settings, issues, pull requests,
actions, packages or any other repository.

## Recommended credential

Use a fine-grained personal access token owned by the account or organization that owns
the public publication repository.

- **Repository access:** select `Only select repositories`, then select only the public
  repository, for example `themachinewrites-public`.
- **Repository permissions:** `Contents: Read and write` is the only required permission.
  Set every other permission to `No access`.
- **Organization access:** do not approve or request organization-wide access. If the
  repository owner requires a request, grant it only to the public publication repository.
- **Expiration:** choose a short operator-approved expiry and rotate before it lapses.
- **Account scope:** do not reuse a personal master token, classic token, deploy key with
  another repository, or a GitHub App installation whose selected repositories include
  the private website or daemon.

GitHub App alternative: install the App only on the public repository and grant its
installation only `Contents: Read and write`. Generate short-lived installation tokens on
the daemon host and never give the Codex subprocess access to the App private key or
installation token.

## Host wiring

Store the credential in the environment of the dedicated publisher service as
`THEMACHINEWRITES_GIT_PUBLISH_TOKEN`. Do not put it in YAML, repository files, shell history,
Codex prompt text, logs or a URL. The deterministic publisher reads it only for Git
operations; the Codex child environment explicitly excludes it.

Use the exact HTTPS remote configured for the public repository. The daemon validates
that remote against `public_remote`, then injects the token as a temporary Git HTTP
`extraheader` only into Git subprocesses created by the authenticated publisher. The token
is not written to Git config or repository files, and the Codex child environment excludes
it. Keep the credential itself restricted to the public repository as described above;
do not use a global credential helper or share this token with another repository.

## Manual permission audit

Before enabling live publication, the repository owner must inspect the credential's
settings and record all of the following:

1. Repository access is `Only select repositories`.
2. Exactly one selected repository is the intended public publication repository.
3. `Contents: Read and write` is enabled.
4. Every other repository permission is `No access`.
5. No organization-wide or all-repositories grant is present.
6. The private website and daemon repositories are absent from the selected list.
7. A direct read or write attempt against each private repository is denied while a
   harmless read/write check succeeds against the public repository.
8. Expiry, rotation owner and revocation procedure are recorded in the operator checklist.

Do not paste the token into evidence. Evidence should identify the account, repository
selection and permission matrix, not the secret value. Rotate immediately if the token is
placed in a public file, log, browser transcript or shared prompt.
