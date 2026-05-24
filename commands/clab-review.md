---
name: clab-review
description: "AI review of a self-hosted GitLab MR. Fetches diff, runs lint rules, posts inline comments."
---

# /clab-review

AI review of a self-hosted GitLab MR. Fetches diff, runs rules, posts inline comments.

## Usage

```
/clab-review <MR_URL>
/clab-review <MR_IID>                     # uses GITLAB_PROJECT from .claude/gitlab-mr-review.env
/clab-review <MR_URL> --dry-run           # print findings, don't post
/clab-review <MR_URL> --rules-only        # print active rules and exit
/clab-review <MR_URL> --severity major+   # filter: minor, major, critical (default: minor+)
```

## What You Must Do When Invoked

### Step 0 — Resolve config

Read `.claude/gitlab-mr-review.env` if it exists (key=value, one per line). Overlay with shell env. Required vars:

| Var | Description |
|-----|-------------|
| `GITLAB_TOKEN` | Personal access token (api scope) |
| `GITLAB_HOST` | e.g. `git.digikala.com` |
| `GITLAB_PROJECT` | URL-encoded project path e.g. `frontend%2Fdigikala-now-react` |

If any required var is missing, stop and tell the user which one is missing.

If `--rules-only` was given, print active rules from Step 1 and exit.

### Step 1 — Load review rules

Read `.claude/gitlab-mr-review-rules.md` if it exists. Apply these project-specific rules in addition to the built-in rules below.

**Built-in rules (always active):**

```
RULE[critical] no-secrets        — no hardcoded tokens, passwords, API keys, private keys
RULE[critical] no-console-log    — no console.log/console.error left in production code
RULE[major]    type-safety        — no `any` casts without a justification comment
RULE[major]    dead-code          — no commented-out code blocks (>3 lines)
RULE[minor]    todo-comment       — TODO/FIXME without ticket reference
RULE[minor]    large-file         — file diff > 400 lines changed — suggest split
```

### Step 2 — Fetch MR metadata + diff

Parse `MR_IID` from URL if a full URL was given (last path segment after `/merge_requests/`).

Run the fetch binary (handles pagination automatically, strips API noise):

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
GITLAB_MR_IID="$MR_IID" \
  clab-fetch-diff /tmp/gl_mr_data.json
```

The binary prints the MR header line. Read `/tmp/gl_mr_data.json` for the structured result:
- `meta` — iid, title, branches, author, state, shas (base/start/head)
- `files[]` — path, added_lines, deleted_lines, diff (raw), annotated (line-numbered added/context lines)

### Step 3 — Run automated lint rules

```bash
clab-lint-rules /tmp/gl_mr_data.json /tmp/gl_mr_auto_findings.json
```

Read `/tmp/gl_mr_auto_findings.json`. These findings are already confirmed — **do not re-check them**. The binary covers:
- `critical` — no-secrets, no-console-log
- `major` — type-safety (any casts), dead-code (commented blocks >3 lines)
- `minor` — todo-comment (no ticket ref), large-file (>400 changed lines)

### Step 4 — Semantic review

Read `files[].annotated` from `/tmp/gl_mr_data.json`. For each file, review only added lines (`added: true`) for issues the lint binary cannot catch:

- Logic bugs, off-by-one errors, incorrect conditionals
- Stale closures, missing dep-array entries, React hook misuse
- API contract violations, type narrowing errors, missing null guards
- UX/semantic issues (wrong color token, misleading label, missing loading state)
- Inconsistencies between files in the same MR

**Do not re-flag anything already in `/tmp/gl_mr_auto_findings.json`.**

Format each finding:
```
<severity>: <file_path>:<line> — <rule-id>: <one-line description>. <fix hint>.
```

Group by severity: critical → major → minor. Apply `--severity` filter if given.

Merge semantic findings with auto findings. If total is zero: print `No issues found.` and skip Step 5.

### Step 5 — Post comments (skip if --dry-run)

**Line anchoring rule:** GitLab only accepts inline comments on lines present in the diff. Before writing findings, verify each finding's `line` is an added line (`added: true`) in `files[].annotated`. If the target line is a context line, move it to the nearest added line in the same file hunk and note the original line in the description. If no added line exists in the file, drop the inline line and set `"line": null` — the posting binary will fall back to a general MR note.

Serialize all findings into `/tmp/gl_mr_findings.json` using Bash (not the Write tool) — the file does not need a prior Read:

```bash
cat > /tmp/gl_mr_findings.json << 'FINDINGS_EOF'
[
  {
    "severity": "major",
    "rule": "<rule-id>",
    "path": "<file_path>",
    "line": <line_number_or_null>,
    "description": "<one-line description>",
    "fix": "<fix hint>"
  }
]
FINDINGS_EOF
```

Then invoke the posting binary (SHAs are fetched automatically):

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
GITLAB_MR_IID="$MR_IID" \
  clab-post-comments /tmp/gl_mr_findings.json
```

`GITLAB_PROJECT_ID` must be the **decoded** project path (e.g. `supernova/digikala-now-react`, not `supernova%2Fdigikala-now-react`).

If the binary reports an error for a finding, print it and continue.

### Step 6 — Summary

```
Review complete: !<iid> — <N> finding(s) posted
  critical: N
  major:    N
  minor:    N
```

Append `(dry run — no comments posted)` if `--dry-run`.

---

## Config (.claude/gitlab-mr-review.env)

```env
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_HOST=git.digikala.com
GITLAB_PROJECT=frontend%2Fdigikala-now-react
```

**Never commit this file.** Add `.claude/gitlab-mr-review.env` to `.gitignore`.

## Rules file (.claude/gitlab-mr-review-rules.md)

Use `/clab-prepare-rules` to generate this from past MRs, or start from the template:

```bash
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/gitlab-mr-review-rules.md \
  -o .claude/gitlab-mr-review-rules.md
```

Format:
```
RULE[<severity>] <id> — <description>
```

Severity values: `critical`, `major`, `minor`
