package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/alexasmi/agentic-task-executor/internal/models"
)

func AgenticTaskWorkflow(ctx workflow.Context, input models.WorkflowInput) (models.WorkflowResult, error) {
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
		return models.WorkflowResult{}, fmt.Errorf("setting query handler: %w", err)
	}

	logger.Info("Starting agentic task workflow", "repo", input.RepoURL, "task", input.TaskDescription)

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

	// ===== Phase 1: Clone =====
	phase = "cloning"
	workspaceDir := fmt.Sprintf("/tmp/agentic-workspaces/%s", workflow.GetInfo(ctx).WorkflowExecution.ID)

	var cloneResult models.CloneResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, cloneOpts),
		"CloneRepository", input.RepoURL, workspaceDir,
	).Get(ctx, &cloneResult)
	if err != nil {
		return models.WorkflowResult{
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

	var agentResult models.AgentResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, agentOpts),
		"AgentReasoningStep", workspacePath, input.TaskDescription, checklist, taskContext,
	).Get(ctx, &agentResult)
	if err != nil {
		return models.WorkflowResult{
			Success: false, Summary: "Agent execution failed",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}
	if !agentResult.Success {
		return models.WorkflowResult{
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
		return models.WorkflowResult{
			Success: true, Summary: "Audit complete, implementation declined",
			Details: map[string]any{"audit_report": auditReport, "reason": reason},
			WorkspacePath: workspacePath,
		}, nil
	}

	logger.Info("Implementation approved", "reason", reason)

	// ===== Phase 4: Implementation =====
	phase = "implementing"

	implPrompt := buildImplementationPrompt(auditReport, input.TaskDescription, branchName)
	implContext := map[string]any{
		"phase":          "implementation",
		"audit_report":   auditReport,
		"branch_name":    branchName,
		"default_branch": defaultBranch,
	}

	var implResult models.AgentResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, agentOpts),
		"AgentReasoningStep", workspacePath, implPrompt, checklist, implContext,
	).Get(ctx, &implResult)
	if err != nil {
		return models.WorkflowResult{
			Success: false, Summary: "Implementation agent failed",
			Details: map[string]any{"error": err.Error(), "audit_report": auditReport},
			WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}

	if !implResult.ChangesMade {
		return models.WorkflowResult{
			Success: true, Summary: "Audit complete, no changes needed",
			Details: map[string]any{"audit_report": auditReport, "implementation": implResult.Summary},
			WorkspacePath: workspacePath,
		}, nil
	}

	// ===== Phase 5: Push and create PR =====
	phase = "creating_pr"

	var pushResult models.PushResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, gitOpts),
		"PushChanges", workspacePath,
	).Get(ctx, &pushResult)
	if err != nil {
		return models.WorkflowResult{
			Success: false, Summary: "Failed to push changes",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}

	commitMsg := implResult.CommitMessage
	if commitMsg == "" {
		commitMsg = "Implement audit recommendations"
	}

	var prResult models.PRCreateResult
	err = workflow.ExecuteActivity(
		workflow.WithActivityOptions(ctx, prOpts),
		"CreatePullRequest", models.PRCreateInput{
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
		return models.WorkflowResult{
			Success: false, Summary: "Failed to create pull request",
			Details: map[string]any{"error": err.Error()}, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
		}, nil
	}
	prURL = prResult.PRURL
	lastCommitSHA := prResult.HeadSHA
	logger.Info("Pull request created", "url", prURL)

	if !input.WaitForCI {
		return models.WorkflowResult{
			Success: true, Summary: implResult.Summary,
			Details: map[string]any{
				"audit_report": auditReport, "files_modified": implResult.FilesModified, "changes_made": true,
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

		var event models.PREvent
		err = workflow.ExecuteActivity(
			workflow.WithActivityOptions(ctx, watchOpts),
			"WatchPR", models.PRWatchInput{
				PRURL:              prURL,
				LastKnownCommitSHA: lastCommitSHA,
				LastSeenCommentIDs: seenCommentIDs,
				PollInterval:       "30s",
			},
		).Get(ctx, &event)
		if err != nil {
			logger.Error("PR watch failed", "error", err)
			return models.WorkflowResult{
				Success: false, Summary: "PR watch failed",
				PRURL: prURL, WorkspacePath: workspacePath, ErrorMessage: err.Error(),
				FeedbackIterations: feedbackIterations,
			}, nil
		}

		if event.Type == models.PREventMerged {
			logger.Info("PR merged", "iterations", feedbackIterations)
			return models.WorkflowResult{
				Success: true,
				Summary: fmt.Sprintf("PR merged after %d feedback iterations", feedbackIterations),
				Details: map[string]any{"audit_report": auditReport, "files_modified": implResult.FilesModified, "changes_made": true},
				PRURL: prURL, CIStatus: "success", WorkspacePath: workspacePath,
				FeedbackIterations: feedbackIterations,
			}, nil
		}
		if event.Type == models.PREventClosed {
			return models.WorkflowResult{
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
			"branch_name":   branchName,
		}
		feedbackTask := buildFeedbackTaskDescription(input.TaskDescription, event)

		if event.Type == models.PREventCIFailure {
			ciStatus = "failure"
			feedbackContext["ci_failure_details"] = event.CIDetails
		}
		if event.Type == models.PREventReviewFeedback {
			feedbackContext["review_comments"] = event.Comments
		}

		var fixResult models.AgentResult
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

		var fixPush models.PushResult
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
		logger.Info("Fix pushed", "iteration", feedbackIterations)
	}
}
