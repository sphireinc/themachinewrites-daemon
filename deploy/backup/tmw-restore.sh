#!/bin/sh
set -eu

usage() { printf '%s\n' 'usage: tmw-restore.sh --input FILE --output FILE --passphrase-file PATH' >&2; exit 2; }
input= output= passphrase=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --input) input=$2; shift 2 ;;
    --output) output=$2; shift 2 ;;
    --passphrase-file) passphrase=$2; shift 2 ;;
    *) usage ;;
  esac
done
[ -n "$input" ] && [ -n "$output" ] && [ -n "$passphrase" ] || usage
[ -f "$input" ] && [ -f "$passphrase" ] || { printf '%s\n' 'input or passphrase file missing' >&2; exit 1; }
[ ! -e "$output" ] || { printf '%s\n' 'restore output already exists' >&2; exit 1; }
mode=$(stat -f '%Lp' "$passphrase" 2>/dev/null || stat -c '%a' "$passphrase")
[ "$mode" = 600 ] || { printf '%s\n' 'passphrase file must have mode 0600' >&2; exit 1; }
mkdir -p "$(dirname -- "$output")"
openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -in "$input" -out "$output" -pass "file:$passphrase"
chmod 600 "$output"
printf '%s\n' "$output"
