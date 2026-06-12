package models

type AgentResult struct {
	Success        bool           `json:"success"`
	ChangesMade    bool           `json:"changes_made"`
	Summary        string         `json:"summary"`
	Reasoning      string         `json:"reasoning,omitempty"`
	ActionTaken    string         `json:"action_taken,omitempty"`
	NextSteps      []string       `json:"next_steps,omitempty"`
	ContextUpdates map[string]any `json:"context_updates,omitempty"`
	FilesModified  []string       `json:"files_modified"`
	CommitMessage  string         `json:"commit_message,omitempty"`
	Error          string         `json:"error,omitempty"`
}

type ToolCall struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type AgentLoopResult struct {
	FinalResponse string     `json:"final_response"`
	Iterations    int        `json:"iterations"`
	ToolCalls     []ToolCall `json:"tool_calls"`
}
