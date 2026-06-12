package models

type CIStatus string

const (
	CIStatusPending CIStatus = "pending"
	CIStatusSuccess CIStatus = "success"
	CIStatusFailure CIStatus = "failure"
	CIStatusError   CIStatus = "error"
	CIStatusUnknown CIStatus = "unknown"
)

type PRCreateInput struct {
	WorkspacePath   string   `json:"workspace_path"`
	BranchName      string   `json:"branch_name,omitempty"`
	CommitMessage   string   `json:"commit_message"`
	TaskDescription string   `json:"task_description"`
	FilesModified   []string `json:"files_modified"`
	Repo            string   `json:"repo,omitempty"`
	BaseBranch      string   `json:"base_branch,omitempty"`
}

type PRCreateResult struct {
	PRURL    string `json:"pr_url"`
	PRNumber int    `json:"pr_number"`
	HeadSHA  string `json:"head_sha,omitempty"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type PREventType string

const (
	PREventCIFailure      PREventType = "ci_failure"
	PREventReviewFeedback PREventType = "review_feedback"
	PREventMerged         PREventType = "merged"
	PREventClosed         PREventType = "closed"
)

type PRWatchInput struct {
	PRURL              string  `json:"pr_url"`
	LastKnownCommitSHA string  `json:"last_known_commit_sha"`
	LastSeenCommentIDs []int64 `json:"last_seen_comment_ids"`
	PollInterval       string  `json:"poll_interval,omitempty"`
}

type PREvent struct {
	Type      PREventType `json:"type"`
	CIDetails string      `json:"ci_details,omitempty"`
	Comments  []Comment   `json:"comments,omitempty"`
	PRState   string      `json:"pr_state"`
}

type Comment struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type CIStatusResult struct {
	Status  CIStatus `json:"status"`
	Details string   `json:"details"`
	Success bool     `json:"success"`
	Error   string   `json:"error,omitempty"`
}

type CommentsResult struct {
	Comments []Comment `json:"comments"`
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
}
