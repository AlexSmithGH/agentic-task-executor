package models

type WorkflowInput struct {
	RepoURL         string         `json:"repo_url"`
	TaskDescription string         `json:"task_description"`
	Checklist       []string       `json:"checklist,omitempty"`
	Context         map[string]any `json:"context,omitempty"`
	WaitForCI       bool           `json:"wait_for_ci"`
	BranchName      string         `json:"branch_name,omitempty"`
}

type WorkflowResult struct {
	Success            bool           `json:"success"`
	Summary            string         `json:"summary"`
	Details            map[string]any `json:"details"`
	PRURL              string         `json:"pr_url,omitempty"`
	CIStatus           string         `json:"ci_status,omitempty"`
	WorkspacePath      string         `json:"workspace_path,omitempty"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	FeedbackIterations int            `json:"feedback_iterations,omitempty"`
}
