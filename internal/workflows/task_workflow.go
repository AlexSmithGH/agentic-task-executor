package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type WorkflowInput struct {
	RepoURL         string         `json:"repo_url"`
	TaskDescription string         `json:"task_description"`
	Checklist       []string       `json:"checklist,omitempty"`
	Context         map[string]any `json:"context,omitempty"`
	WaitForCI       bool           `json:"wait_for_ci"`
	BranchName      string         `json:"branch_name,omitempty"`
}

type WorkflowResult struct {
	Success      bool           `json:"success"`
	Summary      string         `json:"summary"`
	Details      map[string]any `json:"details"`
	PRURL        string         `json:"pr_url,omitempty"`
	CIStatus     string         `json:"ci_status,omitempty"`
	WorkspacePath string        `json:"workspace_path,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

type CloneResult struct {
	WorkspacePath string `json:"workspace_path"`
	RepoName      string `json:"repo_name"`
	DefaultBranch string `json:"default_branch"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
}

type AgentResult struct {
	Success       bool     `json:"success"`
	ChangesMade   bool     `json:"changes_made"`
	Summary       string   `json:"summary"`
	FilesModified []string `json:"files_modified"`
	CommitMessage string   `json:"commit_message,omitempty"`
	Reasoning     string   `json:"reasoning,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type PRCreateInput struct {
	WorkspacePath   string   `json:"workspace_path"`
	BranchName      string   `json:"branch_name,omitempty"`
	CommitMessage   string   `json:"commit_message"`
	TaskDescription string   `json:"task_description"`
	FilesModified   []string `json:"files_modified"`
}

type PRResult struct {
	PRURL      string `json:"pr_url"`
	PRNumber   int    `json:"pr_number"`
	BranchName string `json:"branch_name"`
}

type CIWaitInput struct {
	PRNumber int    `json:"pr_number"`
	RepoURL  string `json:"repo_url"`
}

type CIResult struct {
	Status string `json:"status"`
	Checks []any  `json:"checks,omitempty"`
}

func AgenticTaskWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	logger := workflow.GetLogger(ctx)

	var workspacePath string
	var prURL string
	var ciStatus string

	// Query handler for status checks
	err := workflow.SetQueryHandler(ctx, "get_status", func() (map[string]any, error) {
		return map[string]any{
			"workspace_path": workspacePath,
			"pr_url":         prURL,
			"ci_status":      ciStatus,
		}, nil
	})
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("setting query handler: %w", err)
	}

	logger.Info("Starting agentic task workflow", "repo", input.RepoURL, "task", input.TaskDescription)

	// Step 1: Clone repository
	workspaceDir := fmt.Sprintf("/tmp/agentic-workspaces/%s", workflow.GetInfo(ctx).WorkflowExecution.ID)

	cloneCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
			InitialInterval: 1 * time.Second,
			MaximumInterval: 10 * time.Second,
		},
	})

	var cloneResult CloneResult
	err = workflow.ExecuteActivity(cloneCtx, "CloneRepository", input.RepoURL, workspaceDir).Get(ctx, &cloneResult)
	if err != nil {
		return WorkflowResult{
			Success:      false,
			Summary:      "Failed to clone repository",
			Details:      map[string]any{"error": err.Error()},
			ErrorMessage: err.Error(),
		}, nil
	}
	workspacePath = cloneResult.WorkspacePath
	logger.Info("Repository cloned", "path", workspacePath)

	// Step 2: Execute agent reasoning loop
	agentCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        2,
			InitialInterval:        5 * time.Second,
			NonRetryableErrorTypes: []string{"ValidationError"},
		},
	})

	checklist := input.Checklist
	if checklist == nil {
		checklist = []string{}
	}
	taskContext := input.Context
	if taskContext == nil {
		taskContext = map[string]any{}
	}

	var agentResult AgentResult
	err = workflow.ExecuteActivity(agentCtx, "AgentReasoningStep",
		workspacePath, input.TaskDescription, checklist, taskContext,
	).Get(ctx, &agentResult)
	if err != nil {
		return WorkflowResult{
			Success:       false,
			Summary:       "Agent execution failed",
			Details:       map[string]any{"error": err.Error()},
			WorkspacePath: workspacePath,
			ErrorMessage:  err.Error(),
		}, nil
	}

	if !agentResult.Success {
		return WorkflowResult{
			Success:       false,
			Summary:       "Agent execution failed",
			Details:       map[string]any{"error": agentResult.Error},
			WorkspacePath: workspacePath,
			ErrorMessage:  agentResult.Error,
		}, nil
	}

	logger.Info("Agent completed", "summary", agentResult.Summary)

	// Step 3: Create PR if changes were made
	if agentResult.ChangesMade {
		prCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 3 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 3,
				InitialInterval: 2 * time.Second,
			},
		})

		commitMsg := agentResult.CommitMessage
		if commitMsg == "" {
			commitMsg = "Agent changes"
		}

		var prResult PRResult
		err = workflow.ExecuteActivity(prCtx, "CreatePullRequest", PRCreateInput{
			WorkspacePath:   workspacePath,
			BranchName:      input.BranchName,
			CommitMessage:   commitMsg,
			TaskDescription: input.TaskDescription,
			FilesModified:   agentResult.FilesModified,
		}).Get(ctx, &prResult)
		if err != nil {
			return WorkflowResult{
				Success:       false,
				Summary:       "Failed to create pull request",
				Details:       map[string]any{"error": err.Error()},
				WorkspacePath: workspacePath,
				ErrorMessage:  err.Error(),
			}, nil
		}

		prURL = prResult.PRURL
		logger.Info("Pull request created", "url", prURL)

		// Step 4: Wait for CI results if requested
		if input.WaitForCI {
			ciCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
				StartToCloseTimeout: 20 * time.Minute,
				RetryPolicy: &temporal.RetryPolicy{
					MaximumAttempts: 1,
				},
			})

			var ciResult CIResult
			err = workflow.ExecuteActivity(ciCtx, "WaitForCICompletion", CIWaitInput{
				PRNumber: prResult.PRNumber,
				RepoURL:  input.RepoURL,
			}).Get(ctx, &ciResult)
			if err != nil {
				logger.Warn("CI wait failed", "error", err)
			} else {
				ciStatus = ciResult.Status
				logger.Info("CI status", "status", ciStatus)
			}
		}
	}

	return WorkflowResult{
		Success: true,
		Summary: agentResult.Summary,
		Details: map[string]any{
			"files_modified": agentResult.FilesModified,
			"changes_made":   agentResult.ChangesMade,
			"reasoning":      agentResult.Reasoning,
		},
		PRURL:         prURL,
		CIStatus:      ciStatus,
		WorkspacePath: workspacePath,
	}, nil
}
