// post-comments: post review findings as GitLab MR discussions/notes.
// Env: GITLAB_TOKEN, GITLAB_HOST, GITLAB_PROJECT_ID, GITLAB_MR_IID
// Usage: post-comments [findings.json]   (default: /tmp/gl_mr_findings.json)
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mberneti/clab/internal/gitlab"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "❌ Missing required env var: %s\n", key)
		os.Exit(1)
	}
	return v
}

func main() {
	token := mustEnv("GITLAB_TOKEN")
	host := os.Getenv("GITLAB_HOST")
	if host == "" {
		host = "gitlab.com"
	}
	projectID := mustEnv("GITLAB_PROJECT_ID")
	mrIID := mustEnv("GITLAB_MR_IID")

	inPath := "/tmp/gl_mr_findings.json"
	if len(os.Args) > 1 {
		inPath = os.Args[1]
	}

	raw, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ read %s: %v\n", inPath, err)
		os.Exit(1)
	}
	var findings []gitlab.Finding
	if err := json.Unmarshal(raw, &findings); err != nil {
		fmt.Fprintf(os.Stderr, "❌ parse findings: %v\n", err)
		os.Exit(1)
	}

	if len(findings) == 0 {
		fmt.Println("✅ No findings to post.")
		return
	}

	base := fmt.Sprintf("https://%s/api/v4", host)
	enc := url.PathEscape(projectID)
	client := &http.Client{}

	fmt.Fprintf(os.Stderr, "📤 Fetching MR !%s SHAs...\n", mrIID)
	shas, err := fetchSHAs(client, base, enc, mrIID, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ fetch SHAs: %v\n", err)
		os.Exit(1)
	}

	counts := map[string]int{}
	posted := 0

	for _, f := range findings {
		noteID, err := postDiscussion(client, base, enc, mrIID, token, f, shas)
		if err != nil {
			line := "nil"
			if f.Line != nil {
				line = fmt.Sprintf("%d", *f.Line)
			}
			fmt.Fprintf(os.Stderr, "  ❌ [%s] %s:%s — %v\n", f.Severity, f.Path, line, err)
			continue
		}
		line := "nil"
		if f.Line != nil {
			line = fmt.Sprintf("%d", *f.Line)
		}
		fmt.Printf("  ✅ [%s] %s:%s → note #%d\n", f.Severity, f.Path, line, noteID)
		counts[f.Severity]++
		posted++
	}

	fmt.Printf("\nReview complete: !%s — %d finding(s) posted\n", mrIID, posted)
	fmt.Printf("  critical: %d\n", counts["critical"])
	fmt.Printf("  major:    %d\n", counts["major"])
	fmt.Printf("  minor:    %d\n", counts["minor"])
}

func fetchSHAs(c *http.Client, base, enc, mrIID, token string) (gitlab.DiffRefs, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests/%s", base, enc, mrIID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("PRIVATE-TOKEN", token)
	resp, err := c.Do(req)
	if err != nil {
		return gitlab.DiffRefs{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return gitlab.DiffRefs{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var raw map[string]any
	json.Unmarshal(body, &raw)
	var refs gitlab.DiffRefs
	if dr, ok := raw["diff_refs"].(map[string]any); ok {
		refs.BaseSHA = strOrEmpty(dr["base_sha"])
		refs.StartSHA = strOrEmpty(dr["start_sha"])
		refs.HeadSHA = strOrEmpty(dr["head_sha"])
	}
	return refs, nil
}

func buildCommentBody(f gitlab.Finding) string {
	sev := strings.ToUpper(f.Severity)
	return fmt.Sprintf("**[%s]** `%s`\n\n%s\n\n%s\n\n---\n*🤖 AI review — verify before acting*",
		sev, f.Rule, f.Description, f.Fix)
}

// flexInt unmarshals JSON numbers or quoted-number strings into int.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	n, _ := strconv.Atoi(s)
	*f = flexInt(n)
	return nil
}

type noteResponse struct {
	ID    flexInt `json:"id"`
	Notes []struct {
		ID flexInt `json:"id"`
	} `json:"notes"`
}

func postDiscussion(c *http.Client, base, enc, mrIID, token string, f gitlab.Finding, shas gitlab.DiffRefs) (int, error) {
	body := buildCommentBody(f)

	if f.Line == nil {
		// General MR note
		u := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes", base, enc, mrIID)
		payload := map[string]string{"body": body}
		var resp noteResponse
		if err := apiPost(c, u, token, payload, &resp); err != nil {
			return 0, err
		}
		return int(resp.ID), nil
	}

	// Inline discussion
	u := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", base, enc, mrIID)
	payload := map[string]any{
		"body": body,
		"position": map[string]any{
			"position_type": "text",
			"base_sha":      shas.BaseSHA,
			"start_sha":     shas.StartSHA,
			"head_sha":      shas.HeadSHA,
			"new_path":      f.Path,
			"new_line":      *f.Line,
		},
	}
	var resp noteResponse
	if err := apiPost(c, u, token, payload, &resp); err != nil {
		return 0, err
	}
	if len(resp.Notes) > 0 {
		return int(resp.Notes[0].ID), nil
	}
	return int(resp.ID), nil
}

func apiPost(c *http.Client, u, token string, payload any, out any) error {
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", u, bytes.NewReader(data))
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return json.Unmarshal(body, out)
}

func strOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
