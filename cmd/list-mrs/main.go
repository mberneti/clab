// list-mrs: list MR IIDs for prepare-rules aggregation.
// Env: GITLAB_TOKEN, GITLAB_HOST, GITLAB_PROJECT_ID
// Usage: list-mrs [--last N] [--mr-ids 1,2,3] [--since DATE] [--until DATE] [output.json]
//
//	output default: /tmp/gl_mr_list.json
//	DATE format: YYYY-MM-DD
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var version = "dev"

type MRSummary struct {
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	CreatedAt    string `json:"created_at"`
	MergedAt     string `json:"merged_at,omitempty"`
}

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

	var (
		lastN    int
		mrIDs    []int
		since    string
		until    string
		outPath  = "/tmp/gl_mr_list.json"
	)

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--last":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "❌ --last requires a number")
				os.Exit(1)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "❌ --last value must be a positive integer, got: %s\n", args[i])
				os.Exit(1)
			}
			lastN = n
		case "--mr-ids":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "❌ --mr-ids requires a comma-separated list of IIDs")
				os.Exit(1)
			}
			for _, part := range strings.Split(args[i], ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				n, err := strconv.Atoi(part)
				if err != nil {
					fmt.Fprintf(os.Stderr, "❌ invalid MR IID: %s\n", part)
					os.Exit(1)
				}
				mrIDs = append(mrIDs, n)
			}
		case "--since":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "❌ --since requires a date (YYYY-MM-DD)")
				os.Exit(1)
			}
			since = args[i]
			if !isValidDate(since) {
				fmt.Fprintf(os.Stderr, "❌ --since date must be YYYY-MM-DD, got: %s\n", since)
				os.Exit(1)
			}
		case "--until":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "❌ --until requires a date (YYYY-MM-DD)")
				os.Exit(1)
			}
			until = args[i]
			if !isValidDate(until) {
				fmt.Fprintf(os.Stderr, "❌ --until date must be YYYY-MM-DD, got: %s\n", until)
				os.Exit(1)
			}
		case "--version":
			fmt.Println(version)
			os.Exit(0)
		default:
			if !strings.HasPrefix(args[i], "--") {
				outPath = args[i]
			} else {
				fmt.Fprintf(os.Stderr, "❌ Unknown flag: %s\n", args[i])
				os.Exit(1)
			}
		}
	}

	modeCount := 0
	if lastN > 0 {
		modeCount++
	}
	if len(mrIDs) > 0 {
		modeCount++
	}
	if since != "" || until != "" {
		modeCount++
	}
	if modeCount == 0 {
		fmt.Fprintln(os.Stderr, "❌ Specify one of: --last N, --mr-ids IIDs, --since/--until DATE")
		fmt.Fprintln(os.Stderr, "Usage: clab-list-mrs [--last N] [--mr-ids 1,2,3] [--since YYYY-MM-DD] [--until YYYY-MM-DD] [output.json]")
		os.Exit(1)
	}
	if modeCount > 1 {
		fmt.Fprintln(os.Stderr, "❌ Use only one of: --last, --mr-ids, --since/--until")
		os.Exit(1)
	}

	base := fmt.Sprintf("https://%s/api/v4", host)
	enc := url.PathEscape(projectID)
	client := &http.Client{}

	var mrs []MRSummary
	var err error

	switch {
	case len(mrIDs) > 0:
		fmt.Fprintf(os.Stderr, "📋 Fetching %d specific MR(s) from %s...\n", len(mrIDs), host)
		mrs, err = fetchByIIDs(client, base, enc, token, mrIDs)
	case lastN > 0:
		fmt.Fprintf(os.Stderr, "📋 Fetching last %d MR(s) from %s...\n", lastN, host)
		mrs, err = fetchLast(client, base, enc, token, lastN)
	default:
		fmt.Fprintf(os.Stderr, "📋 Fetching MRs from %s", host)
		if since != "" {
			fmt.Fprintf(os.Stderr, " since %s", since)
		}
		if until != "" {
			fmt.Fprintf(os.Stderr, " until %s", until)
		}
		fmt.Fprintln(os.Stderr)
		mrs, err = fetchByDateRange(client, base, enc, token, since, until)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Found %d MR(s)\n", len(mrs))
	for _, mr := range mrs {
		fmt.Fprintf(os.Stderr, "  !%d  %s (%s)\n", mr.IID, mr.Title, mr.State)
	}

	data, _ := json.MarshalIndent(mrs, "", "  ")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "❌ write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✅ Written to %s\n", outPath)
}

func fetchByIIDs(c *http.Client, base, enc, token string, iids []int) ([]MRSummary, error) {
	var mrs []MRSummary
	for _, iid := range iids {
		u := fmt.Sprintf("%s/projects/%s/merge_requests/%d", base, enc, iid)
		var raw map[string]any
		if err := apiGet(c, u, token, nil, &raw); err != nil {
			return nil, fmt.Errorf("fetch MR !%d: %w", iid, err)
		}
		mrs = append(mrs, parseMR(raw))
	}
	return mrs, nil
}

func fetchLast(c *http.Client, base, enc, token string, n int) ([]MRSummary, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests", base, enc)
	params := map[string]string{
		"state":    "all",
		"order_by": "created_at",
		"sort":     "desc",
		"per_page": strconv.Itoa(min(n, 100)),
		"page":     "1",
	}
	var raw []map[string]any
	if err := apiGet(c, u, token, params, &raw); err != nil {
		return nil, err
	}
	if len(raw) > n {
		raw = raw[:n]
	}
	mrs := make([]MRSummary, len(raw))
	for i, r := range raw {
		mrs[i] = parseMR(r)
	}
	return mrs, nil
}

func fetchByDateRange(c *http.Client, base, enc, token, since, until string) ([]MRSummary, error) {
	u := fmt.Sprintf("%s/projects/%s/merge_requests", base, enc)
	params := map[string]string{
		"state":    "all",
		"order_by": "created_at",
		"sort":     "desc",
		"per_page": "100",
	}
	if since != "" {
		params["created_after"] = since + "T00:00:00Z"
	}
	if until != "" {
		params["created_before"] = until + "T23:59:59Z"
	}

	var all []MRSummary
	page := 1
	for {
		params["page"] = strconv.Itoa(page)
		var raw []map[string]any
		totalPages, err := apiGetPaged(c, u, token, params, &raw)
		if err != nil {
			return nil, err
		}
		for _, r := range raw {
			all = append(all, parseMR(r))
		}
		if page >= totalPages {
			break
		}
		page++
	}
	return all, nil
}

func parseMR(raw map[string]any) MRSummary {
	author := ""
	if a, ok := raw["author"].(map[string]any); ok {
		if name, ok := a["name"].(string); ok {
			author = name
		}
	}
	iid := 0
	if v, ok := raw["iid"].(float64); ok {
		iid = int(v)
	}
	mergedAt := ""
	if v, ok := raw["merged_at"].(string); ok {
		mergedAt = v
	}
	return MRSummary{
		IID:          iid,
		Title:        strOrEmpty(raw["title"]),
		Author:       author,
		State:        strOrEmpty(raw["state"]),
		SourceBranch: strOrEmpty(raw["source_branch"]),
		TargetBranch: strOrEmpty(raw["target_branch"]),
		CreatedAt:    strOrEmpty(raw["created_at"]),
		MergedAt:     mergedAt,
	}
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

func strOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func isValidDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
