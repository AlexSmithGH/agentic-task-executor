package api

type TaskParams struct {
	RepoURL         string         `json:"repo_url"`
	TaskDescription string         `json:"task_description"`
	Checklist       []string       `json:"checklist,omitempty"`
	Context         map[string]any `json:"context,omitempty"`
}

type TaskResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Status     string `json:"status"`
}

type TaskStatus struct {
	WorkflowID string         `json:"workflow_id"`
	RunID      string         `json:"run_id"`
	Status     string         `json:"status"`
	Result     map[string]any `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type SignalRequest struct {
	SignalName string         `json:"signal_name"`
	SignalArgs map[string]any `json:"signal_args,omitempty"`
}

type ErrorResponse struct {
	Detail string `json:"detail"`
}
