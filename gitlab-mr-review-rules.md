# GitLab MR Review Rules

# Format: RULE[<severity>] <id> — <description>
# Severity: critical, major, minor
#
# These rules are applied IN ADDITION to built-in rules:
#   critical: no-secrets, no-console-log
#   major:    type-safety, dead-code
#   minor:    todo-comment, large-file

# --- Project-specific rules (fill these in) ---

# RULE[critical] example-rule — replace this with your first rule

# RULE[major]    no-barrel-import — do not import from barrel index.ts; import directly from source file

# RULE[major]    rtl-missing — components rendering user-facing text must use RTL utility or dir="rtl"

# RULE[minor]    no-hardcoded-color — use design token variable, not hardcoded hex/rgb/hsl value

# RULE[minor]    missing-loading-state — async data fetch without loading/skeleton UI
