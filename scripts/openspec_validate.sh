#!/usr/bin/env bash
# openspec_validate.sh - strict OpenSpec validation that tolerates roadmap stubs.
#
# `openspec validate --all --strict` rejects changes without spec deltas, but
# roadmap stubs intentionally contain only proposal.md. This script validates
# every spec and every change that has a specs/ directory in strict mode, and
# lists the stub changes it skipped. A change stops being a stub as soon as it
# gets specs/, and from then on it is validated strictly.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
export OPENSPEC_TELEMETRY=0

status=0
if compgen -G "openspec/specs/*/spec.md" >/dev/null; then
  openspec validate --specs --strict --no-interactive || status=1
else
  echo "openspec: no main specs yet"
fi

for dir in openspec/changes/*/; do
  name=$(basename "$dir")
  [[ "$name" == "archive" ]] && continue
  if [[ -d "$dir/specs" ]]; then
    openspec validate "$name" --type change --strict --no-interactive || status=1
  else
    [[ -f "$dir/proposal.md" ]] || { echo "openspec: $name has neither specs/ nor proposal.md" >&2; status=1; continue; }
    echo "openspec: $name is a roadmap stub (proposal only): skipped"
  fi
done
exit $status
