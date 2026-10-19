#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/tmw-backup-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin" "$tmp/runtime" "$tmp/codex-home" "$tmp/public" "$tmp/backups"
cat > "$tmp/config.yaml" <<EOF
dry_run: true
sources_path: $tmp/sources.yaml
runtime_dir: $tmp/runtime
codex_model: fixture
codex_home: $tmp/codex-home
public_repo_path: $tmp/public
public_branch: main
public_remote: https://github.com/example/public.git
EOF
printf '%s\n' 'sources: []' > "$tmp/sources.yaml"
printf '%s\n' 'fixture backup content' > "$tmp/fixture.sqlite"
cat > "$tmp/bin/themachinewrites" <<EOF
#!/bin/sh
set -eu
output=
while [ "\$#" -gt 0 ]; do
  case "\$1" in
    backup) shift ;;
    --output) output=\$2; shift 2 ;;
    --config) shift 2 ;;
    *) exit 2 ;;
  esac
done
[ -n "\$output" ] || exit 2
cp "$tmp/fixture.sqlite" "\$output"
EOF
chmod +x "$tmp/bin/themachinewrites"
printf '%s\n' 'fixture-test-passphrase' > "$tmp/passphrase"
chmod 600 "$tmp/passphrase"
(
  cd "$repo"
  PATH="$tmp/bin:$PATH" deploy/backup/tmw-backup.sh --config "$tmp/config.yaml" --output "$tmp/backups" --passphrase-file "$tmp/passphrase" --retain 1 >/dev/null
  encrypted=$(find "$tmp/backups" -name '*.sqlite.enc' -type f)
  deploy/backup/tmw-restore.sh --input "$encrypted" --output "$tmp/restored.sqlite" --passphrase-file "$tmp/passphrase" >/dev/null
  cmp "$tmp/fixture.sqlite" "$tmp/restored.sqlite"
)
