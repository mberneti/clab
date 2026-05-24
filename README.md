# clab

AI-powered GitLab MR code review as a Claude Code skill.  
Fetches the diff, runs lint rules, and posts inline comments — no external services, no Node.js.

---

## How it works

```
/clab-review <MR_URL>
```

Claude Code invokes three Go binaries in sequence:

| Binary | What it does |
|--------|--------------|
| `clab-fetch-diff` | Fetches MR metadata + full paginated diff → `/tmp/gl_mr_data.json` |
| `clab-lint-rules` | Runs regex-based lint rules → `/tmp/gl_mr_auto_findings.json` |
| `clab-post-comments` | Posts findings as inline GitLab discussions |

Claude performs a semantic review on top of the lint output, then calls `clab-post-comments` with the combined findings.

---

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/install.sh | bash
```

Installs `clab-fetch-diff`, `clab-lint-rules`, and `clab-post-comments` to `/usr/local/bin`.

**Custom install directory:**

```bash
CLAB_INSTALL_DIR=~/.local/bin curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/install.sh | bash
```

**Specific version:**

```bash
CLAB_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/install.sh | bash
```

Supports Linux and macOS on amd64 and arm64. Pre-built archives are on the [Releases](../../releases) page.

---

## Setup

### 1. Create a GitLab personal access token

Go to **GitLab → User Settings → Access Tokens** and create a token with the `api` scope.

### 2. Add the skill to your Claude Code project

```bash
mkdir -p .claude
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/SKILL.md -o .claude/SKILL.md
```

### 3. Create the config file

```bash
cat > .claude/gitlab-mr-review.env << 'EOF'
GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
GITLAB_HOST=gitlab.com
GITLAB_PROJECT=your-group%2Fyour-repo
EOF
```

> **Never commit this file.** Add `.claude/gitlab-mr-review.env` to `.gitignore`.

### 4. (Optional) Project-specific rules

```bash
curl -fsSL https://raw.githubusercontent.com/mberneti/clab/main/gitlab-mr-review-rules.md \
  -o .claude/gitlab-mr-review-rules.md
```

Rules format:

```
RULE[critical] no-eval     — never use eval() in client code
RULE[major]    no-any      — no TypeScript `any` casts
RULE[minor]    rtl-missing — user-facing text must have dir="rtl"
```

---

## Usage

```
/clab-review <MR_URL>
/clab-review <MR_IID>                    # uses GITLAB_PROJECT from config
/clab-review <MR_URL> --dry-run          # print findings, don't post
/clab-review <MR_URL> --rules-only       # print active rules and exit
/clab-review <MR_URL> --severity major+  # only major and critical
```

---

## Built-in rules

| Severity | Rule ID | What it catches |
|----------|---------|-----------------|
| critical | `no-secrets` | Hardcoded tokens, passwords, API keys |
| major | `dead-code` | Commented-out code blocks (>3 consecutive lines) |
| minor | `todo-comment` | TODO/FIXME without a ticket reference |
| minor | `large-file` | File diff > 400 lines — suggests splitting |

Project-specific rules from `.claude/gitlab-mr-review-rules.md` are applied on top.

---

## Binary CLI reference

All binaries read config from environment variables. The skill sets them from `.claude/gitlab-mr-review.env`.

### `clab-fetch-diff [output.json]`

```
Env:  GITLAB_TOKEN  GITLAB_HOST  GITLAB_PROJECT_ID  GITLAB_MR_IID
Out:  /tmp/gl_mr_data.json  (default)
```

### `clab-lint-rules [input.json] [output.json]`

```
In:   /tmp/gl_mr_data.json          (default)
Out:  /tmp/gl_mr_auto_findings.json (default)
```

### `clab-post-comments [findings.json]`

```
Env:  GITLAB_TOKEN  GITLAB_HOST  GITLAB_PROJECT_ID  GITLAB_MR_IID
In:   /tmp/gl_mr_findings.json  (default)
```

---

## Building from source

```bash
git clone https://github.com/mberneti/clab.git
cd clab
make build    # ./bin/
make release  # cross-compile all platforms → ./dist/
```

---

## License

MIT
