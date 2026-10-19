#!/bin/sh
set -eu

usage() {
  printf '%s\n' 'usage: tmw-backup.sh --config PATH --output DIR --passphrase-file PATH [--retain N]' >&2
  exit 2
}

config_file=
output_dir=
passphrase_file=
retain=7
while [ "$#" -gt 0 ]; do
  case "$1" in
    --config) [ "$#" -ge 2 ] || usage; config_file=$2; shift 2 ;;
    --output) [ "$#" -ge 2 ] || usage; output_dir=$2; shift 2 ;;
    --passphrase-file) [ "$#" -ge 2 ] || usage; passphrase_file=$2; shift 2 ;;
    --retain) [ "$#" -ge 2 ] || usage; retain=$2; shift 2 ;;
    *) usage ;;
  esac
done

[ -n "$config_file" ] && [ -n "$output_dir" ] && [ -n "$passphrase_file" ] || usage
case "$retain" in ''|*[!0-9]*) usage ;; esac
[ "$retain" -ge 1 ] && [ "$retain" -le 365 ] || usage
[ -f "$passphrase_file" ] || { printf '%s\n' 'passphrase file does not exist' >&2; exit 1; }
mode=$(stat -f '%Lp' "$passphrase_file" 2>/dev/null || stat -c '%a' "$passphrase_file")
[ "$mode" = 600 ] || { printf '%s\n' 'passphrase file must have mode 0600' >&2; exit 1; }
command -v openssl >/dev/null 2>&1 || { printf '%s\n' 'openssl is required' >&2; exit 1; }

mkdir -p "$output_dir"
chmod 700 "$output_dir"
plain=$(mktemp "${TMPDIR:-/tmp}/tmw-backup.XXXXXX.sqlite")
encrypted=$(mktemp "${TMPDIR:-/tmp}/tmw-backup.XXXXXX")
trap 'rm -f "$plain" "$encrypted"' EXIT HUP INT TERM

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
if command -v themachinewrites >/dev/null 2>&1; then binary=themachinewrites
elif [ -x "$script_dir/bin/themachinewrites" ]; then binary=$script_dir/bin/themachinewrites
else printf '%s\n' 'themachinewrites binary not found' >&2; exit 1; fi
"$binary" backup --config "$config_file" --output "$plain"
openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -in "$plain" -out "$encrypted" -pass "file:$passphrase_file"
openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -in "$encrypted" -pass "file:$passphrase_file" | cmp -s - "$plain" || {
  printf '%s\n' 'encrypted backup verification failed' >&2
  exit 1
}
stamp=$(date -u '+%Y%m%dT%H%M%SZ')
final="$output_dir/tmw-runtime-$stamp.sqlite.enc"
[ ! -e "$final" ] || { printf '%s\n' 'backup already exists' >&2; exit 1; }
install -m 600 "$encrypted" "$final"

count=$(find "$output_dir" -type f -name 'tmw-runtime-*.sqlite.enc' | wc -l | tr -d ' ')
if [ "$count" -gt "$retain" ]; then
  find "$output_dir" -type f -name 'tmw-runtime-*.sqlite.enc' | sort | head -n "$((count-retain))" | while IFS= read -r old; do rm -f "$old"; done
fi
printf '%s\n' "$final"
