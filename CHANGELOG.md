# Changelog

All notable changes to this project will be documented in this file.

## [v1.1.0] - 2026-05-24

### Added
- `clab-list-mrs`: list MR IIDs by `--last N`, `--mr-ids 1,2,3`, or `--since`/`--until` date range
- `/clab-prepare-rules` skill: analyze past MRs to generate project-specific review rules; supports `--dry-run` and `--append`

## [v1.0.1] - 2026-05-24

### Fixed
- `clab-post-comments`: handle GitLab instances that return discussion `id` as a string instead of an integer (fixes unmarshal error on self-hosted GitLab)

## [v1.0.0] - 2026-05-24

### Added
- `clab-fetch-diff`: fetch GitLab MR metadata and full paginated diff
- `clab-lint-rules`: language-agnostic regex lint rules (no-secrets, dead-code, todo-comment, large-file)
- `clab-post-comments`: post findings as inline GitLab discussions
- Claude Code skill (`SKILL.md`) for `/clab-review` command
- Cross-platform release archives (Linux, macOS, Windows — amd64 + arm64)
- `install.sh` one-liner installer
