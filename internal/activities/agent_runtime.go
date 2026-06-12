package activities

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/alexasmi/agentic-task-executor/internal/agent"
	"github.com/alexasmi/agentic-task-executor/internal/config"
	"github.com/alexasmi/agentic-task-executor/internal/models"
)

type AgentActivities struct {
	Config *config.Config
}

func (a *AgentActivities) AgentReasoningStep(ctx context.Context, workspace, taskDescription string, checklist []string, taskContext map[string]any) (models.AgentResult, error) {
	slog.Info("Starting agent reasoning step",
		"task", taskDescription, "workspace", workspace, "checklist_items", len(checklist))

	claudeClient := agent.NewClaudeClient(
		ctx,
		a.Config.GCPProjectID,
		a.Config.GCPRegion,
		a.Config.ClaudeModel,
		a.Config.ClaudeMaxTokens,
	)

	tools := agent.GetToolDefinitions()
	systemPrompt := agent.BuildAgentSystemPrompt(taskDescription, checklist)
	initialPrompt := agent.BuildInitialPrompt(workspace, taskDescription, checklist)
	executor := NewToolExecutor(workspace)

	result, err := claudeClient.RunAgentLoop(ctx, initialPrompt, systemPrompt, tools, executor, 100)
	if err != nil {
		return models.AgentResult{
			Success: false,
			Error:   fmt.Sprintf("Agent reasoning step failed: %v", err),
		}, nil
	}

	slog.Info("Agent completed", "iterations", result.Iterations, "tool_calls", len(result.ToolCalls))

	filesModified := extractModifiedFiles(result.ToolCalls)

	return models.AgentResult{
		Success:     true,
		ChangesMade: len(filesModified) > 0,
		Summary:     result.FinalResponse,
		Reasoning:   result.FinalResponse,
		ActionTaken: fmt.Sprintf("Executed %d tool calls across %d iterations", len(result.ToolCalls), result.Iterations),
		NextSteps:   extractNextSteps(result.FinalResponse),
		ContextUpdates: map[string]any{
			"iterations": result.Iterations,
			"tool_calls": result.ToolCalls,
		},
		FilesModified: filesModified,
	}, nil
}

func extractModifiedFiles(toolCalls []models.ToolCall) []string {
	seen := make(map[string]bool)
	var files []string
	for _, tc := range toolCalls {
		if tc.Name == "write_file" {
			if fp, ok := tc.Input["file_path"].(string); ok && fp != "" {
				if !seen[fp] {
					seen[fp] = true
					files = append(files, fp)
				}
			}
		}
	}
	return files
}

func extractNextSteps(response string) []string {
	var steps []string
	prefixes := []string{"- [ ]", "TODO:", "Next:", "Action:"}
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) {
				steps = append(steps, line)
				break
			}
		}
		if len(steps) >= 5 {
			break
		}
	}
	return steps
}
