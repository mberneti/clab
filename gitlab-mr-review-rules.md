# GitLab MR Review Rules

# META: SCOPE RESTRICTION

# AI review MUST flag ONLY issues matching a rule explicitly listed in this file.

# Do NOT apply built-in rules, general semantic analysis, or any judgment

# beyond what is directly described below. If a finding does not map to a

# named rule in this file, discard it.

# Format: RULE[<severity>] <id> — <description>

# Severity: critical, major, minor

# --- Project-specific rules ---

# RULE[critical] example-rule — replace this with your first rule

# RULE[major] no-barrel-import — do not import from barrel index.ts; import directly from source file

# RULE[major] rtl-missing — components rendering user-facing text must use RTL utility or dir="rtl"

# RULE[minor] no-hardcoded-color — use design token variable, not hardcoded hex/rgb/hsl value

# RULE[minor] missing-loading-state — async data fetch without loading/skeleton UI
