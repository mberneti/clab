// lint-rules: run regex-based review rules against a structured MR diff.
// Usage: lint-rules [input.json] [output.json]
//
//	input  default: /tmp/gl_mr_data.json
//	output default: /tmp/gl_mr_auto_findings.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/mberneti/clab/internal/gitlab"
)

var (
	reSecrets = regexp.MustCompile(`(?i)\b(password|passwd|secret|api_?key|access_?token|private_?key|auth_?token)\s*[:=]\s*['` + "`" + `"][^'` + "`" + `"\s]{8,}['` + "`" + `"]`)
	reTodo    = regexp.MustCompile(`\b(TODO|FIXME)\b`)
	reTicket  = regexp.MustCompile(`([A-Z]+-\d+|#\d+)`)
	reComment = regexp.MustCompile(`^\s*(//|/\*|\*|#|<!--)`)
)

var severityOrder = map[string]int{"critical": 0, "major": 1, "minor": 2}

func main() {
	inPath := "/tmp/gl_mr_data.json"
	outPath := "/tmp/gl_mr_auto_findings.json"
	if len(os.Args) > 1 {
		inPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		outPath = os.Args[2]
	}

	raw, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ read %s: %v\n", inPath, err)
		os.Exit(1)
	}
	var data gitlab.MRData
	if err := json.Unmarshal(raw, &data); err != nil {
		fmt.Fprintf(os.Stderr, "❌ parse %s: %v\n", inPath, err)
		os.Exit(1)
	}

	findings := lint(data.Files)

	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if severityOrder[a.Severity] != severityOrder[b.Severity] {
			return severityOrder[a.Severity] < severityOrder[b.Severity]
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		al, bl := 0, 0
		if a.Line != nil {
			al = *a.Line
		}
		if b.Line != nil {
			bl = *b.Line
		}
		return al < bl
	})

	out, _ := json.MarshalIndent(findings, "", "  ")
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	counts := map[string]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	fmt.Printf("Lint complete: %d finding(s)\n", len(findings))
	fmt.Printf("  critical: %d\n", counts["critical"])
	fmt.Printf("  major:    %d\n", counts["major"])
	fmt.Printf("  minor:    %d\n", counts["minor"])
	fmt.Printf("✅ Written to %s\n", outPath)
}

func lint(files []gitlab.FileDiff) []gitlab.Finding {
	var findings []gitlab.Finding
	push := func(severity, rule, path string, line *int, desc, fix string) {
		findings = append(findings, gitlab.Finding{
			Severity: severity, Rule: rule, Path: path,
			Line: line, Description: desc, Fix: fix,
		})
	}
	intPtr := func(n int) *int { return &n }

	for _, file := range files {
		if file.DeletedFile {
			continue
		}

		// large-file
		if file.AddedLines+file.DeletedLines > 400 {
			l := intPtr(1)
			push("minor", "large-file", file.Path, l,
				fmt.Sprintf("%d lines changed (%d+ %d−).", file.AddedLines+file.DeletedLines, file.AddedLines, file.DeletedLines),
				"Consider splitting into smaller, focused commits or files.")
		}

		var consecStart, consecCount int

		for _, al := range file.Annotated {
			isComment := reComment.MatchString(strings.TrimSpace(al.Content))

			if !isComment {
				if consecCount > 3 {
					l := intPtr(consecStart)
					push("major", "dead-code", file.Path, l,
						fmt.Sprintf("%d consecutive comment lines, likely commented-out code.", consecCount),
						"Remove dead code; use version control to recover it if needed.")
				}
				consecCount = 0
			} else {
				if consecCount == 0 {
					consecStart = al.Line
				}
				consecCount++
			}

			if !al.Added {
				continue
			}

			// no-secrets
			if reSecrets.MatchString(al.Content) {
				l := intPtr(al.Line)
				push("critical", "no-secrets", file.Path, l,
					"Possible hardcoded credential.",
					"Move to environment variable.")
			}

			// todo-comment
			if reTodo.MatchString(al.Content) && !reTicket.MatchString(al.Content) {
				l := intPtr(al.Line)
				push("minor", "todo-comment", file.Path, l,
					"TODO/FIXME without a ticket reference.",
					"Add a ticket ID (e.g. JIRA-123 or #456).")
			}
		}

		// flush trailing comment block
		if consecCount > 3 {
			l := intPtr(consecStart)
			push("major", "dead-code", file.Path, l,
				fmt.Sprintf("%d consecutive comment lines — likely commented-out code.", consecCount),
				"Remove dead code; use version control to recover it if needed.")
		}
	}

	return findings
}
