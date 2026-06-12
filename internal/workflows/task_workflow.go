package workflows

import (
	"fmt"
	"strings"
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
	Success            bool           `json:"success"`
	Summary            string         `json:"summary"`
	Details            map[string]any `json:"details"`
	PRURL              string         `json:"pr_url,omitempty"`
	CIStatus           string         `json:"ci_status,omitempty"`
	WorkspacePath      string         `json:"workspace_path,omitempty"`
	ErrorMessage       string         `json:"error_message,omitempty"`
	FeedbackIterations int            `json:"feedback_iterations,omitempty"`
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
	Repo            string   `json:"repo,omitempty"`
	BaseBranch      string   `json:"base_branch,omitempty"`
}

type PRResult struct {
	PRURL    string `json:"pr_url"`
	PRNumber int    `json:"pr_number"`
	HeadSHA  string `json:"head_sha,omitempty"`
	Success  bool   `json:"success"`
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

type BranchResult struct {
	BranchName string `json:"branch_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

// PR watcher types
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

func AgenticTaskWorkflow(ctx workflow.Context, input WorkflowInput) (WorkflowResult, error) {
	logger := workflow.GetLogger(ctx)

	var workspacePath string
	var prURL string
	var ciStatus string
	var feedbackIterations int
	var phase string
	var auditReport string

	phase = "initializing"

	err := workflow.SetQueryHandler(ctx, "get_status", func() (map[string]any, error) {
		return map[string]any{
			"workspace_path":      workspacePath,
			"pr_url":              prURL,
			"ci_status":           ciStatus,
			"feedback_iterations": feedbackIterations,
			"phase":               phase,
			"audit_report":        auditReport,
		}, nil
	})
	if err != nil {
		return WorkflowResult{}, fmt.Errorf("setting query handler: %w", err)
	}

	logger.Info("Starting agentic task workflow", "repo", input.RepoURL, "task", input.TaskDescription)

	// Activity option presets
	cloneOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 0,
			InitialInterval: 1 * time.Second,
			MaximumInterval: 30 * time.Second,
		},
	}
	agentOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 0,
			InitialInterval: 5 * time.Second,
			MaximumInterval: 1 * time.Minute,
		},
	}
	gitOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 0,
			InitialInterval: 2 * time.Second,
			MaximumInterval: 30 * time.Second,
		},
	}
	prOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 3 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 0,
			InitialInterval: 2 * time.Second,
			MaximumInterval: 30 * time.Second,
		},
	}
	watchOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 7 * 24 * time.Hour,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 0,
			InitialInterval: 10 * time.Second,
			MaximumInterval: 5 * time.Minute,
		},
	}

	// ===== Phase 1: Setup =====
	phase = "cloning"

	workspaceDir := fmt.Sprintf("/tmp/agentic-workspaces/%s", workflow.GetInfo(ctx).WorkflowExecution.ID)

	var cloneResult CloneResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, cloneOpts),
		"CloneRepository", input.RepoURL, workspaceDir,
	).Get(ctx, &cloneResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Failed to clone repository",
			Details: map[string]any{"error": err.Error()}, ErrorMessage: err.Error(),
		}, nil
	}
	workspacePath = cloneResult.WorkspacePath
	defaultBranch := cloneResult.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "master"
	}
	logger.Info("Repository cloned", "path", workspacePath, "default_branch", defaultBranch)

	branchName := input.BranchName
	if branchName == "" {
		branchName = fmt.Sprintf("agentic/%s", workflow.GetInfo(ctx).WorkflowExecution.ID)
	}

	var branchResult BranchResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, gitOpts),
		"CreateBranch", workspacePath, branchName,
	).Get(ctx, &branchResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Failed to create branch",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}

	// ===== Phase 2: Audit =====
	phase = "auditing"

	checklist := input.Checklist
	if checklist == nil {
		checklist = []string{}
	}
	taskContext := input.Context
	if taskContext == nil {
		taskContext = map[string]any{}
	}

	var agentResult AgentResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, agentOpts),
		"AgentReasoningStep", workspacePath, input.TaskDescription, checklist, taskContext,
	).Get(ctx, &agentResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Agent execution failed",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}
	if !agentResult.Success {
		return WorkflowResult{
			Success: false, Summary: "Agent execution failed",
			Details: map[string]any{"error": agentResult.Error}, WorkspacePath: workspacePath, ErrorMessage: agentResult.Error,
		}, nil
	}

	auditReport = agentResult.Summary
	logger.Info("Audit completed", "summary_length", len(auditReport))

	// ===== Phase 3: Await approval =====
	phase = "awaiting_approval"
	logger.Info("Awaiting human approval to proceed with implementation")

	approvalCh := workflow.GetSignalChannel(ctx, "approval")
	var approval map[string]any
	approvalCh.Receive(ctx, &approval)

	approved, _ := approval["approved"].(bool)
	reason, _ := approval["reason"].(string)

	if !approved {
		logger.Info("Implementation declined", "reason", reason)
		return WorkflowResult{
			Success: true, Summary: "Audit complete, implementation declined",
			Details: map[string]any{
				"audit_report": auditReport,
				"reason":       reason,
			},
			WorkspacePath: workspacePath,
		}, nil
	}

	logger.Info("Implementation approved", "reason", reason)

	// ===== Phase 4: Implementation =====
	phase = "implementing"

	implPrompt := buildImplementationPrompt(auditReport, input.TaskDescription)
	implContext := map[string]any{
		"phase":        "implementation",
		"audit_report": auditReport,
	}

	var implResult AgentResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, agentOpts),
		"AgentReasoningStep", workspacePath, implPrompt, checklist, implContext,
	).Get(ctx, &implResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Implementation agent failed",
			Details: map[string]any{"error": err.Error(), "audit_report": auditReport},
			WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}

	if !implResult.ChangesMade {
		return WorkflowResult{
			Success: true, Summary: "Audit complete, no changes needed",
			Details: map[string]any{
				"audit_report":  auditReport,
				"implementation": implResult.Summary,
			},
			WorkspacePath: workspacePath,
		}, nil
	}

	// ===== Phase 5: Commit, push, PR =====
	phase = "creating_pr"

	commitMsg := implResult.CommitMessage
	if commitMsg == "" {
		commitMsg = "Implement audit recommendations"
	}

	var commitResult CommitResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, gitOpts),
		"CommitChanges", workspacePath, commitMsg,
	).Get(ctx, &commitResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Failed to commit changes",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}
	lastCommitSHA := commitResult.CommitSHA

	if lastCommitSHA == "" {
		logger.Info("No file changes to commit, skipping PR creation")
		return WorkflowResult{
			Success: true, Summary: implResult.Summary,
			Details: map[string]any{
				"audit_report":   auditReport,
				"implementation": implResult.Summary,
				"changes_made":   false,
			},
			WorkspacePath: workspacePath,
		}, nil
	}

	var pushResult PushResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, gitOpts),
		"PushChanges", workspacePath,
	).Get(ctx, &pushResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Failed to push changes",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}

	var prResult PRResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, prOpts),
		"CreatePullRequest", PRCreateInput{
			WorkspacePath:   workspacePath,
			BranchName:      branchName,
			CommitMessage:   commitMsg,
			TaskDescription: input.TaskDescription,
			FilesModified:   implResult.FilesModified,
			Repo:            extractRepoSlug(input.RepoURL),
			BaseBranch:      defaultBranch,
		},
	).Get(ctx, &prResult)
	if err != nil {
		return WorkflowResult{
			Success: false, Summary: "Failed to create pull request",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}
	prURL = prResult.PRURL
	if prResult.HeadSHA != "" {
		lastCommitSHA = prResult.HeadSHA
	}
	logger.Info("Pull request created", "url", prURL)

	if !input.WaitForCI {
		return WorkflowResult{
			Success: true, Summary: implResult.Summary,
			Details: map[string]any{
				"audit_report":   auditReport,
				"files_modified": implResult.FilesModified,
				"changes_made":   true,
			},
			PRURL: prURL, WorkspacePath: workspacePath,
		}, nil
	}

	// ===== Phase 6: Feedback loop =====
	phase = "feedback_loop"
	var seenCommentIDs []int64

	for {
		feedbackIterations++
		logger.Info("Feedback loop iteration", "iteration", feedbackIterations)

		var event PREvent
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, watchOpts),
			"WatchPR", PRWatchInput{
				PRURL:              prURL,
				LastKnownCommitSHA: lastCommitSHA,
				LastSeenCommentIDs: seenCommentIDs,
				PollInterval:       "30s",
			},
		).Get(ctx, &event)
		if err != nil {
			logger.Error("PR watch failed", "error", err)
			return WorkflowResult{
				Success: false, Summary: "PR watch failed",
				PRURL: prURL, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
				FeedbackIterations: feedbackIterations,
			}, nil
		}

		if event.Type == PREventMerged {
			logger.Info("PR merged", "iterations", feedbackIterations)
			return WorkflowResult{
				Success: true,
				Summary: fmt.Sprintf("PR merged after %d feedback iterations", feedbackIterations),
				Details: map[string]any{
					"audit_report":   auditReport,
					"files_modified": implResult.FilesModified,
					"changes_made":   true,
				},
				PRURL: prURL, CIStatus: "success", WorkspacePath: workspacePath,
				FeedbackIterations: feedbackIterations,
			}, nil
		}
		if event.Type == PREventClosed {
			return WorkflowResult{
				Success: false, Summary: "PR was closed without merging",
				PRURL: prURL, WorkspacePath: workspacePath, ErrorMessage: "PR closed",
				FeedbackIterations: feedbackIterations,
			}, nil
		}

		feedbackContext := map[string]any{
			"feedback_type": string(event.Type),
			"iteration":     feedbackIterations,
			"pr_url":        prURL,
			"original_task": input.TaskDescription,
		}
		feedbackTask := buildFeedbackTaskDescription(input.TaskDescription, event)

		if event.Type == PREventCIFailure {
			ciStatus = "failure"
			feedbackContext["ci_failure_details"] = event.CIDetails
		}
		if event.Type == PREventReviewFeedback {
			feedbackContext["review_comments"] = event.Comments
		}

		var fixResult AgentResult
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, agentOpts),
			"AgentReasoningStep", workspacePath, feedbackTask, checklist, feedbackContext,
		).Get(ctx, &fixResult)
		if err != nil {
			logger.Error("Agent fix attempt failed", "error", err, "iteration", feedbackIterations)
			seenCommentIDs = updateSeenCommentIDs(seenCommentIDs, event.Comments)
			continue
		}

		if !fixResult.ChangesMade {
			seenCommentIDs = updateSeenCommentIDs(seenCommentIDs, event.Comments)
			continue
		}

		fixCommitMsg := fixResult.CommitMessage
		if fixCommitMsg == "" {
			fixCommitMsg = fmt.Sprintf("Address feedback (iteration %d)", feedbackIterations)
		}

		var fixCommit CommitResult
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, gitOpts),
			"CommitChanges", workspacePath, fixCommitMsg,
		).Get(ctx, &fixCommit)
		if err != nil {
			logger.Error("Commit failed", "error", err)
			continue
		}
		lastCommitSHA = fixCommit.CommitSHA

		var fixPush PushResult
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, gitOpts),
			"PushChanges", workspacePath,
		).Get(ctx, &fixPush)
		if err != nil {
			logger.Error("Push failed", "error", err)
			continue
		}

		ciStatus = "pending"
		seenCommentIDs = updateSeenCommentIDs(seenCommentIDs, event.Comments)
		logger.Info("Fix pushed", "commit", lastCommitSHA, "iteration", feedbackIterations)
	}
}

func buildImplementationPrompt(auditReport, originalTask string) string {
	return fmt.Sprintf(`Implement the recommended changes from the following audit.

Original task: %s

Audit findings:
%s

Use write_file to create or modify files. Use execute_command to verify your changes.
Focus on actionable, high-priority items. Skip findings that require external action.
After making all changes, summarize what you implemented.`, originalTask, auditReport)
}

func extractRepoSlug(repoURL string) string {
	repoURL = strings.TrimRight(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return repoURL
}

func buildFeedbackTaskDescription(originalTask string, event PREvent) string {
	switch event.Type {
	case PREventCIFailure:
		return fmt.Sprintf(
			"Fix CI failure for: %s\n\nCI failure details:\n%s",
			originalTask, event.CIDetails,
		)
	case PREventReviewFeedback:
		var lines []string
		for _, c := range event.Comments {
			line := fmt.Sprintf("- %s: %s", c.Author, c.Body)
			if c.Path != "" {
				line += fmt.Sprintf(" (file: %s, line: %d)", c.Path, c.Line)
			}
			lines = append(lines, line)
		}
		return fmt.Sprintf(
			"Address review feedback for: %s\n\nReview comments:\n%s",
			originalTask, strings.Join(lines, "\n"),
		)
	default:
		return originalTask
	}
}

func updateSeenCommentIDs(existing []int64, comments []Comment) []int64 {
	seen := make(map[int64]bool, len(existing))
	for _, id := range existing {
		seen[id] = true
	}
	for _, c := range comments {
		if !seen[c.ID] {
			existing = append(existing, c.ID)
		}
	}
	return existing
}
