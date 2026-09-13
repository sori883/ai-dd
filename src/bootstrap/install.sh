#!/bin/sh
# Initial acquisition only. The Go installer validates the complete bundle.
set -eu
fail() { printf '%s\n' "AI-DD: $*" >&2; exit 1; }
[ "$#" -eq 2 ] || fail 'usage: install.sh VERSION EXISTING_PROJECT_DIRECTORY'
version=$1
case "$version" in ''|*[!a-zA-Z0-9._-]*|*..*|[!a-zA-Z0-9]*) fail 'invalid version' ;; esac
[ "${#version}" -le 128 ] || fail 'invalid version'
[ -d "$2" ] || fail 'project directory does not exist'
project=$(CDPATH= cd -- "$2" && pwd -P) || fail 'cannot resolve project directory'
for tool in curl tar mktemp uname head tail cat wc awk chmod rm; do command -v "$tool" >/dev/null 2>&1 || fail "missing required tool: $tool"; done
case "$(uname -s)" in Darwin) os=darwin ;; Linux) os=linux ;; *) fail 'unsupported operating system' ;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) fail 'unsupported process architecture' ;; esac
if command -v sha256sum >/dev/null 2>&1; then hash_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then hash_tool=shasum
else fail 'missing SHA-256 tool'; fi
name=ai-dd_${version}_${os}_${arch}.tar.gz
tmp=$(mktemp -d "${TMPDIR:-/tmp}/ai-dd-bootstrap.XXXXXXXX") || fail 'cannot create temporary directory'
trap 'rm -rf -- "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
fetch() {
 (set +e; curl --disable --proto '=https' --proto-redir '=https' --location --fail --silent --show-error --connect-timeout 30 --max-time 120 --max-filesize "$2" --output - "https://github.com/sori883/ai-dd/releases/download/$version/$1"; printf '%s' "$?" > "$tmp/curl-status") | head -c "$(( $2 + 1 ))" > "$tmp/$1"
 [ -f "$tmp/curl-status" ] && [ "$(cat "$tmp/curl-status")" = 0 ] || fail "download failed: $1"
 [ "$(wc -c < "$tmp/$1")" -le "$2" ] || fail 'download exceeds limit'
 rm -f "$tmp/curl-status"
}
fetch SHA256SUMS 4096
# Require the full, sorted six-line table, even when only one archive is cached.
awk -v version="$version" '
 BEGIN { split("darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64 windows_arm64", targets, " ") }
 { suffix=(NR>4 ? ".zip" : ".tar.gz"); expected="ai-dd_" version "_" targets[NR] suffix;
   if (NR>6 || length($0)!=66+length(expected) || substr($0,65)!="  " expected || length($1)!=64 || $1 ~ /[^0-9a-f]/) bad=1 }
 END { if (NR!=6 || bad) exit 1 }
' "$tmp/SHA256SUMS" || fail 'invalid SHA256SUMS'
[ "$(tail -c 1 "$tmp/SHA256SUMS" | wc -l)" -eq 1 ] || fail 'invalid SHA256SUMS newline'
expected=$(awk -v name="$name" '$2==name {print $1}' "$tmp/SHA256SUMS")
fetch "$name" 536870912
if [ "$hash_tool" = sha256sum ]; then actual=$(sha256sum "$tmp/$name"); else actual=$(shasum -a 256 "$tmp/$name"); fi
actual=${actual%% *}
[ "$actual" = "$expected" ] || fail 'archive checksum mismatch'
# List only the selected member. A regular file must occur exactly once.
LC_ALL=C tar -tzf "$tmp/$name" aidlc-install > "$tmp/member-names" || fail 'cannot locate installer'
[ "$(wc -l < "$tmp/member-names")" -eq 1 ] && [ "$(cat "$tmp/member-names")" = aidlc-install ] || fail 'installer must be one root member'
LC_ALL=C tar -tvzf "$tmp/$name" aidlc-install > "$tmp/member" || fail 'cannot inspect installer'
[ "$(wc -l < "$tmp/member")" -eq 1 ] || fail 'installer must occur exactly once'
case "$(cat "$tmp/member")" in -*) ;; *) fail 'installer must be a regular file' ;; esac
# Limit output while retaining tar status without requiring shell pipefail.
(tar -xOzf "$tmp/$name" aidlc-install; printf '%s' "$?" > "$tmp/tar-status") | head -c 67108865 > "$tmp/aidlc-install"
[ -f "$tmp/tar-status" ] && [ "$(cat "$tmp/tar-status")" = 0 ] || fail 'installer extraction failed'
[ "$(wc -c < "$tmp/aidlc-install")" -le 67108864 ] || fail 'installer exceeds limit'
chmod 700 "$tmp/aidlc-install"
set +e
"$tmp/aidlc-install" codex --release-version "$version" --project-dir "$project" --release-dir "$tmp"
status=$?
exit "$status"
