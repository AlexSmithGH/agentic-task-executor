package models

type CloneResult struct {
	WorkspacePath string `json:"workspace_path"`
	RepoName      string `json:"repo_name,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
}

type BranchResult struct {
	BranchName string `json:"branch_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

type CommitResult struct {
	CommitSHA string `json:"commit_sha"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

type PushResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}
