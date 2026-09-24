---
name: publish-guard
description: Use before EVERY git push (and before opening a PR) in edgex-foundry-mcp, which is a public repository. Scans tracked files and the commits about to be pushed for secrets and confidential data (tokens, JWTs, private keys, cloud keys and account IDs, RFC1918 IPs, email addresses, internal hostnames) plus an optional private deny-list. Reports findings with masked matches and STOPS; never auto-fixes or rewrites history.
---

# Publish guard (pre-push confidentiality scan)

## Run it

```sh
scripts/publish_guard.sh              # tracked files + commits in @{upstream}..HEAD (or origin/main..HEAD)
scripts/publish_guard.sh --base REF   # scan commits in REF..HEAD instead
scripts/publish_guard.sh --files-only # skip the commit-range scan
scripts/publish_guard.sh --head SHA   # end the commit range at SHA instead of HEAD
```

Exit code `0` means clean, `1` means findings, and `2` means a usage or environment
error.

## What it checks

- **Generic rules** (also run in CI by `.github/workflows/ci.yml`): JWTs and bearer
  tokens, PEM private keys, AWS-style access keys and account IDs, GitHub/Slack/API-style
  tokens, credential assignments with non-placeholder values, private IPv4 ranges
  (RFC 1918), email addresses, and hostnames under internal-only TLDs.
- **Commit identities** (commit-range scan, local and CI): every author and committer
  email must be a GitHub noreply address (`*@users.noreply.github.com`) or equal to
  `git config user.email`. Corporate domains, personal webmail and bot addresses such
  as the Claude bot address (noreply at anthropic.com) are reported. To fix one, the user sets the repo-local
  identity (`git config --local user.email ...`) and decides whether to rewrite the
  unpushed commits.
- **Private deny-list** (local only): `.claude/publish-guard.local.txt`. It is gitignored
  and holds one term per line (`#` comments allowed). Put organization, customer, site or
  hostname terms here that must never reach the public repo. Matches are printed
  MASKED, and you must never echo the full term in chat, commits or PRs. Claude's `Read`
  tool is denied on this file in `.claude/settings.json`. Only the script reads it.
- Accepted placeholders (RFC 5737 IPs, `example.com`, RFC 2606 reserved TLDs such as
  `.test`/`.example`/`.invalid`/`.localhost`, values containing
  `example`/`placeholder`/`redacted`) are allowed. Project-wide exceptions live in
  `scripts/publish_guard.allow`, one ERE per line, matched against the finding text.

## When it reports findings

1. **STOP. Do not push.**
2. Show the user the report as printed (it is already masked). For each finding, say
   whether it looks like a real leak or a false positive.
3. Do NOT edit files, amend, rebase, or rewrite history on your own. Ask the user how to
   proceed:
   - real leak in the working tree only: user decides on the edit
   - real leak already in a commit: user decides whether to rewrite unpushed history
     (never on a pushed shared branch without explicit approval), and whether to rotate
     the credential
   - false positive: user approves adding a narrow pattern to
     `scripts/publish_guard.allow`
4. Re-run until the scan is clean, then push.
