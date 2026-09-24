#!/usr/bin/env bash
# publish_guard.sh - pre-push confidentiality and secret scan.
#
# Scans (1) tracked files in the working tree and (2) the commits about to be
# pushed (messages, added lines, author and committer emails) for secrets and
# confidential data. Matches are always printed masked. Never modifies anything.
#
# Commit identities: author and committer emails must be a GitHub noreply
# address (*@users.noreply.github.com) or equal to `git config user.email`.
# Anything else (corporate domains, personal webmail, bot addresses such as
# noreply@anthropic.com) is reported.
#
# Usage:
#   scripts/publish_guard.sh [--base REF] [--head REF] [--files-only] [--ci]
#
#   --base REF    scan commits in REF..HEAD (default: @{upstream}, else origin/main)
#   --head REF    end of the commit range instead of HEAD (e.g. a PR head SHA)
#   --files-only  skip the commit-range scan
#   --ci          generic rules only; ignore the local deny-list
#
# Exit codes: 0 clean, 1 findings, 2 usage/environment error.
# Requires GNU grep (-P) and git >= 2.19.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "publish-guard: not a git repository" >&2; exit 2; }
cd "$ROOT"

BASE=""
HEAD_REF="HEAD"
FILES_ONLY=0
CI_MODE=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base) BASE="${2:-}"; shift 2 ;;
    --files-only) FILES_ONLY=1; shift ;;
    --ci) CI_MODE=1; shift ;;
    --head) HEAD_REF="${2:-}"; shift 2 ;;
    -h|--help) sed -n '2,24p' "$0"; exit 0 ;;
    *) echo "publish-guard: unknown argument: $1" >&2; exit 2 ;;
  esac
done

echo "x" | grep -qP 'x' 2>/dev/null || { echo "publish-guard: GNU grep with -P is required" >&2; exit 2; }

ALLOW_FILE="scripts/publish_guard.allow"
DENY_FILE=".claude/publish-guard.local.txt"
# Files that legitimately describe the patterns themselves.
EXCLUDES=(":(exclude)scripts/publish_guard.sh" ":(exclude)$ALLOW_FILE")

# name|PCRE. Keep rules generic: no organization-specific terms here.
RULES=(
  'jwt|eyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}'
  'bearer-token|(?i)bearer\s+[A-Za-z0-9._~+/-]{20,}=*'
  'private-key|-----BEGIN (RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY( BLOCK)?-----'
  'aws-access-key|\b(AKIA|ASIA|AGPA|AIDA|AROA)[0-9A-Z]{16}\b'
  'aws-account-id|(?i)(arn:aws[a-z-]*:[a-z0-9-]*:[a-z0-9-]*:[0-9]{12}|aws.?account.?id["'"'"'\s:=]+[0-9]{12})'
  'github-token|\b(ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{30,}'
  'slack-token|\bxox[abposr]-[A-Za-z0-9-]{10,}'
  'api-key|\b(sk-ant-[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9]{32,}|AIza[0-9A-Za-z_-]{35})\b'
  'credential-assignment|(?i)\b(password|passwd|pwd|secret|api[_-]?key|access[_-]?token|client[_-]?secret)\b["'"'"']?\s*[:=]\s*["'"'"'][^"'"'"'\s]{6,}["'"'"']'
  'private-ipv4|\b(10\.(25[0-5]|2[0-4]\d|1?\d?\d)\.(25[0-5]|2[0-4]\d|1?\d?\d)\.(25[0-5]|2[0-4]\d|1?\d?\d)|172\.(1[6-9]|2\d|3[01])\.(25[0-5]|2[0-4]\d|1?\d?\d)\.(25[0-5]|2[0-4]\d|1?\d?\d)|192\.168\.(25[0-5]|2[0-4]\d|1?\d?\d)\.(25[0-5]|2[0-4]\d|1?\d?\d))\b'
  'email|\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b'
  'internal-hostname|\b[a-z0-9-]+(\.[a-z0-9-]+)*\.(internal|intranet|intra|corp|lan|localdomain|home\.arpa)(?![\w.-])'
)

# Built-in placeholders that are always acceptable (ERE, matched on the finding text).
BUILTIN_ALLOW=(
  '@(example\.(com|org|net)|[a-z0-9.-]+\.example)$'
  '@users\.noreply\.github\.com$'
  '(?i)(example|placeholder|redacted|changeme|dummy|fake|xxxx|\*\*\*)'
)

mask() {
  local s="$1" n=${#1}
  if (( n <= 6 )); then printf '%s' "${s:0:1}***"; else printf '%s' "${s:0:3}***${s: -2}"; fi
}

allowed() {
  local text="$1" re
  for re in "${BUILTIN_ALLOW[@]}"; do
    if [[ "$re" == '(?i)'* ]]; then
      grep -qiE -- "${re#'(?i)'}" <<<"$text" && return 0
    else
      grep -qE -- "$re" <<<"$text" && return 0
    fi
  done
  if [[ -f "$ALLOW_FILE" ]]; then
    while IFS= read -r re || [[ -n "$re" ]]; do
      [[ -z "$re" || "$re" == \#* ]] && continue
      grep -qE -- "$re" <<<"$text" && return 0
    done <"$ALLOW_FILE"
  fi
  return 1
}

FINDINGS=0
report() { # where rule match
  FINDINGS=$((FINDINGS + 1))
  local shown
  if [[ "$2" == "deny-list" ]]; then shown="${3:0:1}*** (${#3} chars)"; else shown=$(mask "$3"); fi
  printf '  %-45s %-22s %s\n' "$1" "$2" "$shown"
}

scan_files() {
  local rule name re line loc match
  for rule in "${RULES[@]}"; do
    name=${rule%%|*}; re=${rule#*|}
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      loc=${line%%:*}; line=${line#*:}; loc="$loc:${line%%:*}"; match=${line#*:}
      allowed "$match" && continue
      report "$loc" "$name" "$match"
    done < <(git grep -n -I -o -P -e "$re" -- . "${EXCLUDES[@]}" 2>/dev/null || true)
  done
}

scan_commits() {
  local range="$1" c rule name re match
  for c in $(git rev-list "$range"); do
    local text
    text=$( { git log -1 --format=%B "$c"; git show --format= --unified=0 "$c" -- . "${EXCLUDES[@]}" | grep -E '^\+' | grep -vE '^\+\+\+ ' || true; } )
    for rule in "${RULES[@]}"; do
      name=${rule%%|*}; re=${rule#*|}
      while IFS= read -r match; do
        [[ -z "$match" ]] && continue
        allowed "$match" && continue
        report "commit ${c:0:10}" "$name" "$match"
      done < <(grep -oP -e "$re" <<<"$text" || true)
    done
  done
}

# Author/committer identity check. The only non-generic allowed address is the
# one configured locally for this repository; nothing personal is hard-coded.
scan_identities() {
  local range="$1" own c who email
  own=$(git config --get user.email 2>/dev/null || true)
  while IFS=$'\t' read -r c who email; do
    [[ -z "$c" ]] && continue
    [[ "$email" =~ @users\.noreply\.github\.com$ ]] && continue
    [[ -n "$own" && "${email,,}" == "${own,,}" ]] && continue
    report "commit ${c:0:10}" "$who-email" "${email:-<empty>}"
  done < <(git log --format=$'%H\tauthor\t%ae%n%H\tcommitter\t%ce' "$range")
}

scan_denylist() {
  [[ -f "$DENY_FILE" ]] || return 0
  local term line loc
  while IFS= read -r term || [[ -n "$term" ]]; do
    term=${term%$'\r'}
    [[ -z "$term" || "$term" == \#* ]] && continue
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      loc=${line%%:*}; line=${line#*:}; loc="$loc:${line%%:*}"
      report "$loc" "deny-list" "$term"
    done < <(git grep -n -I -i -F -e "$term" -- . 2>/dev/null | cut -d: -f1,2 || true)
    if [[ -n "${RANGE:-}" ]]; then
      local c
      for c in $(git rev-list "$RANGE"); do
        if { git log -1 --format=%B "$c"; git show --format= --unified=0 "$c" | grep -E '^\+' || true; } | grep -qiF -e "$term"; then
          report "commit ${c:0:10}" "deny-list" "$term"
        fi
      done
    fi
  done <"$DENY_FILE"
}

RANGE=""
if (( ! FILES_ONLY )); then
  if [[ -z "$BASE" ]]; then
    BASE=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)
    [[ -z "$BASE" ]] && git rev-parse -q --verify origin/main >/dev/null && BASE=origin/main
  fi
  if [[ -n "$BASE" ]]; then
    git rev-parse -q --verify "$BASE" >/dev/null || { echo "publish-guard: unknown base: $BASE" >&2; exit 2; }
    git rev-parse -q --verify "$HEAD_REF" >/dev/null || { echo "publish-guard: unknown head: $HEAD_REF" >&2; exit 2; }
    RANGE="$BASE..$HEAD_REF"
  fi
fi

echo "publish-guard: scanning tracked files${RANGE:+ and commits in $RANGE}$( (( CI_MODE )) && echo ' (CI mode: generic rules only)')"
scan_files
if [[ -n "$RANGE" ]]; then
  scan_commits "$RANGE"
  scan_identities "$RANGE"
fi
if (( ! CI_MODE )); then
  if [[ -f "$DENY_FILE" ]]; then scan_denylist; else echo "publish-guard: no $DENY_FILE (local deny-list skipped)"; fi
fi

if (( FINDINGS > 0 )); then
  echo "publish-guard: $FINDINGS finding(s). Do not push; review each one (matches are masked)." >&2
  exit 1
fi
echo "publish-guard: clean"
