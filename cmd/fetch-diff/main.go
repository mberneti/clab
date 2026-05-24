// fetch-diff: fetch MR metadata + full paginated diff from GitLab.
// Env: GITLAB_TOKEN, GITLAB_HOST, GITLAB_PROJECT_ID, GITLAB_MR_IID
// Usage: fetch-diff [output.json]   (default: /tmp/gl_mr_data.json)
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
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

	outPath := "/tmp/gl_mr_data.json"
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	base := fmt.Sprintf("https://%s/api/v4", host)
	enc := url.PathEscape(projectID)

	client := &http.Client{}

	fmt.Fprintf(os.Stderr, "📥 Fetching MR !%s from %s...\n", mrIID, host)

	meta, err := fetchMeta(client, base, enc, mrIID, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ fetch meta: %v\n", err)
		os.Exit(1)
	}

	rawDiffs, err := fetchAllDiffs(client, base, enc, mrIID, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ fetch diffs: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "MR !%d: %s (%s → %s)\n", meta.IID, meta.Title, meta.SourceBranch, meta.TargetBranch)
	fmt.Fprintf(os.Stderr, "Author: %s | State: %s | Files: %d\n", meta.Author, meta.State, len(rawDiffs))

	files := make([]gitlab.FileDiff, 0, len(rawDiffs))
	for _, raw := range rawDiffs {
		diff := strOrEmpty(raw["diff"])
		added, deleted := countDiffLines(diff)
		fd := gitlab.FileDiff{
			Path:         strOrEmpty(raw["new_path"]),
			NewFile:      boolField(raw["new_file"]),
			DeletedFile:  boolField(raw["deleted_file"]),
			RenamedFile:  boolField(raw["renamed_file"]),
			AddedLines:   added,
			DeletedLines: deleted,
			Diff:         diff,
			Annotated:    annotate(diff),
		}
		if op := strOrEmpty(raw["old_path"]); op != fd.Path {
			fd.OldPath = op
		}
		files = append(files, fd)
	}

	out := gitlab.MRData{Meta: meta, Files: files}
	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✅ Written to %s\n", outPath)
}

func fetchMeta(c *http.Client, base, enc, mrIID, token string) (gitlab.MRMeta, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests/%s", base, enc, mrIID)
	var raw map[string]any
	if err := apiGet(c, u, token, nil, &raw); err != nil {
		return gitlab.MRMeta{}, err
	}
	author := ""
	if a, ok := raw["author"].(map[string]any); ok {
		author = strOrEmpty(a["name"])
	}
	var refs gitlab.DiffRefs
	if dr, ok := raw["diff_refs"].(map[string]any); ok {
		refs = gitlab.DiffRefs{
			BaseSHA:  strOrEmpty(dr["base_sha"]),
			StartSHA: strOrEmpty(dr["start_sha"]),
			HeadSHA:  strOrEmpty(dr["head_sha"]),
		}
	}
	iid := 0
	if v, ok := raw["iid"].(float64); ok {
		iid = int(v)
	}
	return gitlab.MRMeta{
		IID:          iid,
		Title:        strOrEmpty(raw["title"]),
		SourceBranch: strOrEmpty(raw["source_branch"]),
		TargetBranch: strOrEmpty(raw["target_branch"]),
		Author:       author,
		State:        strOrEmpty(raw["state"]),
		SHAs:         refs,
	}, nil
}

func fetchAllDiffs(c *http.Client, base, enc, mrIID, token string) ([]map[string]any, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests/%s/diffs", base, enc, mrIID)
	var all []map[string]any
	page := 1
	for {
		params := map[string]string{"unidiff": "true", "page": strconv.Itoa(page), "per_page": "100"}
		var page_data []map[string]any
		totalPages, err := apiGetPaged(c, u, token, params, &page_data)
		if err != nil {
			return nil, err
		}
		all = append(all, page_data...)
		if page >= totalPages {
			break
		}
		page++
	}
	return all, nil
}

func apiGet(c *http.Client, u, token string, params map[string]string, out any) error {
	_, err := apiGetPaged(c, u, token, params, out)
	return err
}

func apiGetPaged(c *http.Client, rawURL, token string, params map[string]string, out any) (int, error) {
	req, _ := http.NewRequest("GET", rawURL, nil)
	req.Header.Set("PRIVATE-TOKEN", token)
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := c.Do(req)
	if err != nil {
		return 1, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return 1, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return 1, err
	}
	total := 1
	if v := resp.Header.Get("X-Total-Pages"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			total = n
		}
	}
	return total, nil
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func annotate(diffText string) []gitlab.AnnotatedLine {
	var result []gitlab.AnnotatedLine
	newLine := 0
	for _, raw := range strings.Split(diffText, "\n") {
		if m := hunkRe.FindStringSubmatch(raw); m != nil {
			n, _ := strconv.Atoi(m[1])
			newLine = n - 1
			continue
		}
		if strings.HasPrefix(raw, "---") || strings.HasPrefix(raw, "+++") {
			continue
		}
		if strings.HasPrefix(raw, "-") {
			continue
		}
		newLine++
		added := strings.HasPrefix(raw, "+")
		content := raw
		if len(raw) > 0 {
			content = raw[1:]
		}
		result = append(result, gitlab.AnnotatedLine{Line: newLine, Content: content, Added: added})
	}
	return result
}

func countDiffLines(diff string) (added, deleted int) {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			deleted++
		}
	}
	return
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

func boolField(v any) bool {
	b, _ := v.(bool)
	return b
}
