---
name: clab-prepare-rules
description: "Analyze past GitLab MRs to generate or update project-specific review rules in .claude/gitlab-mr-review-rules.md."
---

# /clab-prepare-rules

Analyze past MR reviews to generate or update `.claude/gitlab-mr-review-rules.md`. Identifies recurring issues across multiple MRs and converts them into named, actionable rules ready for `/clab-review`.

## Usage

```
/clab-prepare-rules --last 20
/clab-prepare-rules --mr-ids 123,456,789
/clab-prepare-rules --since 2026-01-01
/clab-prepare-rules --since 2026-01-01 --until 2026-03-31
/clab-prepare-rules --last 10 --dry-run   # print suggested rules, don't write file
/clab-prepare-rules --last 10 --append    # append to existing rules instead of replacing
```

| Flag | Description |
|------|-------------|
| `--last N` | Analyze the N most recent MRs (any state) |
| `--mr-ids IID,...` | Analyze specific MRs by IID, comma-separated |
| `--since YYYY-MM-DD` | Analyze MRs created on or after this date |
| `--until YYYY-MM-DD` | Upper bound for `--since` (optional, defaults to today) |
| `--dry-run` | Print rule candidates without writing the file |
| `--append` | Append new rules to existing file instead of replacing the rules section |

Exactly one of `--last`, `--mr-ids`, or `--since` is required.

## What You Must Do When Invoked

### Step 0 — Resolve config

Read `.claude/gitlab-mr-review.env` if it exists (key=value, one per line). Overlay with shell env. Required vars:

| Var | Description |
|-----|-------------|
| `GITLAB_TOKEN` | Personal access token (api scope) |
| `GITLAB_HOST` | e.g. `git.digikala.com` |
| `GITLAB_PROJECT` | URL-encoded project path e.g. `frontend%2Fdigikala-now-react` |

If any required var is missing, stop and tell the user which one is missing.

`GITLAB_PROJECT_ID` used in binary calls is the **URL-decoded** value of `GITLAB_PROJECT` (e.g. `frontend/digikala-now-react`, not `frontend%2Fdigikala-now-react`).

### Step 1 — List MRs

Run the list binary with the flags passed by the user:

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
  clab-list-mrs --last N /tmp/gl_mr_list.json
# or: --mr-ids 123,456,789
# or: --since 2026-01-01 [--until 2026-03-31]
```

Read `/tmp/gl_mr_list.json`. Each entry has: `iid`, `title`, `author`, `state`, `source_branch`, `target_branch`, `created_at`.

If 0 MRs returned, stop and tell the user no MRs matched the criteria.

### Step 2 — Fetch diffs + lint for each MR

For each MR in the list, run fetch-diff then lint-rules:

```bash
GITLAB_TOKEN="$GITLAB_TOKEN" \
GITLAB_HOST="$GITLAB_HOST" \
GITLAB_PROJECT_ID="<URL-decoded $GITLAB_PROJECT>" \
GITLAB_MR_IID="<iid>" \
  clab-fetch-diff /tmp/gl_mr_data_<iid>.json

clab-lint-rules /tmp/gl_mr_data_<iid>.json /tmp/gl_mr_auto_findings_<iid>.json
```

Collect all findings across all MRs into a single working list, each tagged with its source `iid`.

**Performance note:** For lists larger than 30 MRs, process in batches of 10 and print progress (`Processing MRs 1–10 of 47...`).

### Step 3 — Semantic analysis across MRs

Read `files[].annotated` from each `/tmp/gl_mr_data_<iid>.json`. For each MR, identify issues in added lines that are NOT already captured in the auto-findings (same scope as `/clab-review` Step 4: logic bugs, hook misuse, API contract violations, UX issues — not re-checking lint-covered rules).

Aggregate all findings (auto + semantic) and identify **patterns**:
- Issues appearing in **2 or more** MRs → strong rule candidate
- Issues appearing once but representing a clear team-wide anti-pattern → weak candidate (flag with lower confidence)
- Single-occurrence one-off mistakes → discard

For each pattern, synthesize a rule candidate with evidence:

```
RULE[<severity>] <id> — <description>
# Seen in: !<iid1>, !<iid2>, ... (<N> occurrences)
# Example: <one-line concrete example from the diff>
```

**Severity assignment:**
- `critical` — security risk, data loss, broken functionality
- `major` — correctness issues, API violations, significant UX regressions
- `minor` — style inconsistency, naming, missing quality-of-life patterns

**Deduplication:** Check `.claude/gitlab-mr-review-rules.md` if it exists. If an existing rule already covers the pattern (same intent, even if differently named), skip the candidate — note it as "already covered by `<existing-rule-id>`".

### Step 4 — Present rule candidates

Print all candidates grouped by severity, with evidence. Always print this before writing the file, even when not `--dry-run`:

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

Skipped (already covered by existing rules):
  - no-console-log already covers: console-log-debug
```

If no patterns found: print `No recurring patterns found across the analyzed MRs.` and stop.

### Step 5 — Write rules file (skip if --dry-run)

Strip the `# Seen in:` / `# Example:` evidence lines — those are display-only, not written to the file.

**Default (replace mode):** Replace the project-specific rules section in `.claude/gitlab-mr-review-rules.md`, preserving the META header block at the top of the file intact.

**`--append` mode:** Append new rules after the last existing rule in the file. Never write a rule whose `id` already exists in the file.

Write each rule as an active rule (not commented out):
```
RULE[<severity>] <id> — <description>
```

Print:
```
Rules written to .claude/gitlab-mr-review-rules.md
  Added:   N rule(s)
  Skipped: N rule(s) (already covered)
```

If the file does not exist yet, create it using the standard template header before writing the rules.

---

## Config (.claude/gitlab-mr-review.env)

```env
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_HOST=git.digikala.com
GITLAB_PROJECT=frontend%2Fdigikala-now-react
```

**Never commit this file.** Add `.claude/gitlab-mr-review.env` to `.gitignore`.

## Workflow tip

Run `--dry-run` first to review candidates, then run without it to write:

```
/clab-prepare-rules --last 30 --dry-run
/clab-prepare-rules --last 30
```

Then run `/clab-review` on new MRs — the generated rules are picked up automatically.
