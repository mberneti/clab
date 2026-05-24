package gitlab

// MRMeta holds the merge request header information.
type MRMeta struct {
	IID          int         `json:"iid"`
	Title        string      `json:"title"`
	SourceBranch string      `json:"source_branch"`
	TargetBranch string      `json:"target_branch"`
	Author       string      `json:"author"`
	State        string      `json:"state"`
	SHAs         DiffRefs    `json:"shas"`
}

// DiffRefs holds the three SHAs GitLab needs for inline comments.
type DiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	StartSHA string `json:"start_sha"`
	HeadSHA  string `json:"head_sha"`
}

// AnnotatedLine is one line from a file diff with its new-file line number.
type AnnotatedLine struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
	Added   bool   `json:"added"`
}

// FileDiff is one file's diff data, fully annotated.
type FileDiff struct {
	Path         string          `json:"path"`
	OldPath      string          `json:"old_path,omitempty"`
	NewFile      bool            `json:"new_file"`
	DeletedFile  bool            `json:"deleted_file"`
	RenamedFile  bool            `json:"renamed_file"`
	AddedLines   int             `json:"added_lines"`
	DeletedLines int             `json:"deleted_lines"`
	Diff         string          `json:"diff"`
	Annotated    []AnnotatedLine `json:"annotated"`
}

// MRData is the full output of fetch-diff, consumed by lint-rules and post-comments.
type MRData struct {
	Meta  MRMeta     `json:"meta"`
	Files []FileDiff `json:"files"`
}

// Finding is one review finding emitted by lint-rules and consumed by post-comments.
type Finding struct {
	Severity    string `json:"severity"`
	Rule        string `json:"rule"`
	Path        string `json:"path"`
	Line        *int   `json:"line"` // nil → general MR note
	Description string `json:"description"`
	Fix         string `json:"fix"`
}
