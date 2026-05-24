---
name: clab
description: "AI-powered review of self-hosted GitLab merge requests. Fetches MR diff, applies project-specific rules, posts inline comments. Can analyze past MRs to generate review rules."
---

# clab — Claude Code Skill

Two commands. Install both by copying the files in `commands/` to your project's `.claude/commands/`:

```bash
mkdir -p .claude/commands
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/commands/clab-review.md \
  -o .claude/commands/clab-review.md
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/commands/clab-prepare-rules.md \
  -o .claude/commands/clab-prepare-rules.md
```

## Commands

| Command | Purpose |
|---------|---------|
| `/clab-review` | Review a single MR — fetch diff, lint, semantic review, post inline comments |
| `/clab-prepare-rules` | Analyze past MRs and generate project-specific review rules |

See each command file for full usage and steps.

## Required config

Create `.claude/gitlab-mr-review.env` in your project (never commit this):

```env
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_HOST=git.digikala.com
GITLAB_PROJECT=frontend%2Fdigikala-now-react
```

## Binaries

All commands depend on Go binaries installed via:

```bash
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/install.sh | bash
```

| Binary | Used by |
|--------|---------|
| `clab-fetch-diff` | `/clab-review`, `/clab-prepare-rules` |
| `clab-lint-rules` | `/clab-review`, `/clab-prepare-rules` |
| `clab-post-comments` | `/clab-review` |
| `clab-list-mrs` | `/clab-prepare-rules` |

## Typical workflow

```
# First time: generate rules from recent history
/clab-prepare-rules --last 30 --dry-run
/clab-prepare-rules --last 30

# Ongoing: review each MR
/clab-review <MR_URL>
```
