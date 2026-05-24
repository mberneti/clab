---
name: clab
description: "AI-powered review of self-hosted GitLab merge requests. Fetches MR diff, applies project-specific rules, posts inline comments. Can analyze past MRs to generate review rules."
---

# /clab-review

AI review of a self-hosted GitLab MR. Fetches diff, runs rules, posts comments.

## Usage

```
/clab-review <MR_URL>
/clab-review <MR_IID>                     # uses GITLAB_PROJECT from env or .claude/gitlab-mr-review.env
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

The script prints the MR header line. Read `/tmp/gl_mr_data.json` for the structured result:
- `meta` — iid, title, branches, author, state, shas (base/start/head)
- `files[]` — path, added_lines, deleted_lines, diff (raw), annotated (line-numbered added/context lines)

### Step 3 — Run automated lint rules

```bash
clab-lint-rules /tmp/gl_mr_data.json /tmp/gl_mr_auto_findings.json
```

Read `/tmp/gl_mr_auto_findings.json`. These findings are already confirmed — **do not re-check them**. The script covers:
- `critical` — no-secrets, no-console-log
- `major` — type-safety (any casts), dead-code (commented blocks >3 lines)
- `minor` — todo-comment (no ticket ref), large-file (>400 changed lines)

### Step 4 — Semantic review

Read `files[].annotated` from `/tmp/gl_mr_data.json`. For each file, review only added lines (`added: true`) for issues the lint script cannot catch:

- Logic bugs, off-by-one errors, incorrect conditionals
- Stale closures, missing dep-array entries, React hook misuse
- API contract violations, type narrowing errors, missing null guards
- UX/semantic issues (wrong color token, misleading label, missing loading state)
- Inconsistencies between files in the same PR

**Do not re-flag anything already in `/tmp/gl_mr_auto_findings.json`.**

Format each finding:
```
<severity>: <file_path>:<line> — <rule-id>: <one-line description>. <fix hint>.
```

Group by severity: critical → major → minor. Apply `--severity` filter if given.

Merge semantic findings with auto findings. If total is zero: print `No issues found.` and skip Step 5.

### Step 5 — Post comments (skip if --dry-run)

**Line anchoring rule:** GitLab only accepts inline comments on lines present in the diff. Before writing findings, verify each finding's `line` is an added line (`added: true`) in `files[].annotated`. If the target line is a context line, move it to the nearest added line in the same file hunk and note the original line in the description. If no added line exists in the file, drop the inline line and set `"line": null` — the posting script will fall back to a general MR note.

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

Then invoke the posting script (SHAs are fetched automatically by the script):

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
GITLAB_MR_IID="$MR_IID" \
  clab-post-comments /tmp/gl_mr_findings.json
```

`GITLAB_PROJECT_ID` must be the **decoded** project path (e.g. `supernova/digikala-now-react`, not `supernova%2Fdigikala-now-react`).

If the script reports an error for a finding, print it and continue.

### Step 6 — Summary

```
Review complete: !<iid> — <N> finding(s) posted
  critical: N
  major:    N
  minor:    N
```

Append `(dry run — no comments posted)` if `--dry-run`.

---

---

# /clab-prepare-rules

Analyze past MR reviews to generate or update `.claude/gitlab-mr-review-rules.md`. Identifies recurring issues across multiple MRs and converts them into named, actionable rules.

## Usage

```
/clab-prepare-rules --last 20
/clab-prepare-rules --mr-ids 123,456,789
/clab-prepare-rules --since 2026-01-01
/clab-prepare-rules --since 2026-01-01 --until 2026-03-31
/clab-prepare-rules --last 10 --dry-run        # print suggested rules, don't write file
/clab-prepare-rules --last 10 --append         # append to existing rules file instead of replacing
```

## What You Must Do When Invoked

### Step 0 — Resolve config

Same as `/clab-review` Step 0. Read `.claude/gitlab-mr-review.env` and overlay with env. Required: `GITLAB_TOKEN`, `GITLAB_HOST`, `GITLAB_PROJECT`.

### Step 1 — List MRs

Run the list binary:

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
  clab-list-mrs [--last N | --mr-ids IID,IID,... | --since DATE [--until DATE]] /tmp/gl_mr_list.json
```

Read `/tmp/gl_mr_list.json`. It contains an array of MR summaries with fields: `iid`, `title`, `author`, `state`, `source_branch`, `target_branch`, `created_at`.

If 0 MRs returned, stop and tell the user.

### Step 2 — Fetch diffs + lint for each MR

For each MR in the list, run fetch-diff and lint-rules:

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
GITLAB_MR_IID="<iid>" \
  clab-fetch-diff /tmp/gl_mr_data_<iid>.json

clab-lint-rules /tmp/gl_mr_data_<iid>.json /tmp/gl_mr_auto_findings_<iid>.json
```

Collect all findings across all MRs into a single in-memory list, tagged with the source MR IID.

**Performance note:** If the list is large (>30 MRs), process in batches of 10 and print progress.

### Step 3 — Semantic analysis across MRs

Read all `files[].annotated` from each `/tmp/gl_mr_data_<iid>.json`. For each MR, note issues that appear in the diff but are NOT already in the auto-findings (same semantic review scope as `/clab-review` Step 4, but across multiple MRs).

Then aggregate all findings (auto + semantic) and identify **patterns**: issues that appear in 2 or more MRs, or that represent a clear team/codebase anti-pattern even if seen once.

For each pattern, synthesize a rule candidate:

```
RULE[<severity>] <id> — <description>
# Seen in: !<iid1>, !<iid2>, ... (<N> occurrences)
# Example: <one-line concrete example from the diff>
```

**Severity assignment:**
- `critical` — security, data loss, broken functionality
- `major` — correctness issues, API violations, significant UX problems
- `minor` — style, naming, missing convenience features

**Deduplication:** If an existing rule in `.claude/gitlab-mr-review-rules.md` already covers the pattern, skip it — don't create duplicates.

**Noise filter:** Discard single-occurrence patterns that are clearly one-off mistakes, not team-wide habits.

### Step 4 — Present rule candidates

Print the suggested rules grouped by severity, with their evidence:

```
Suggested rules from analysis of <N> MR(s):

[critical]
  RULE[critical] no-raw-sql — never use raw SQL string interpolation; use parameterized queries
  # Seen in: !34, !41 (2 occurrences)
  # Example: db.Query("SELECT * FROM users WHERE id=" + userId)

[major]
  RULE[major] missing-error-boundary — async component fetch without error boundary
  # Seen in: !28, !31, !35 (3 occurrences)

[minor]
  RULE[minor] console-log-debug — console.log left in non-error paths
  # Seen in: !28, !30, !33, !38 (4 occurrences)
```

If no patterns found: print `No recurring patterns found across the analyzed MRs.` and stop.

### Step 5 — Write rules file (skip if --dry-run)

If `--append`: append the new rules (without duplicates) to the existing `.claude/gitlab-mr-review-rules.md`, after the last existing rule.

Otherwise (default): **replace** the project-specific rules section in `.claude/gitlab-mr-review-rules.md`, preserving the META header block.

Rules are written as active rules (not commented out). Strip the `# Seen in:` / `# Example:` evidence lines from the written file — those are for display only.

Format each written rule:
```
RULE[<severity>] <id> — <description>
```

Print:
```
Rules written to .claude/gitlab-mr-review-rules.md
  Added: N rule(s)
  Skipped (already covered): N rule(s)
```

---

## Config file (.claude/gitlab-mr-review.env)

```env
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_HOST=git.digikala.com
GITLAB_PROJECT=frontend%2Fdigikala-now-react
```

**Never commit this file.** Add `.claude/gitlab-mr-review.env` to `.gitignore`.

## Rules file (.claude/gitlab-mr-review-rules.md)

See the template at `.claude/gitlab-mr-review-rules.md`. Format:

```
RULE[<severity>] <id> — <description>
```

Severity values: `critical`, `major`, `minor`
