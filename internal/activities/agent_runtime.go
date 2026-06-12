package activities

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexasmi/agentic-task-executor/internal/agent"
	"github.com/alexasmi/agentic-task-executor/internal/config"
)

type AgentActivities struct {
	Config *config.Config
}

type AgentResult struct {
	Success        bool           `json:"success"`
	ChangesMade    bool           `json:"changes_made"`
	Summary        string         `json:"summary"`
	Reasoning      string         `json:"reasoning"`
	ActionTaken    string         `json:"action_taken"`
	NextSteps      []string       `json:"next_steps"`
	ContextUpdates map[string]any `json:"context_updates"`
	FilesModified  []string       `json:"files_modified"`
	CommitMessage  string         `json:"commit_message,omitempty"`
	Error          string         `json:"error,omitempty"`
}

func (a *AgentActivities) AgentReasoningStep(ctx context.Context, workspace, taskDescription string, checklist []string, taskContext map[string]any) (AgentResult, error) {
	slog.Info("Starting agent reasoning step",
		"task", taskDescription, "workspace", workspace, "checklist_items", len(checklist))

	claudeClient := agent.NewClaudeClient(
		ctx,
		a.Config.GCPProjectID,
		a.Config.GCPRegion,
		"claude-sonnet-4-5@20250929",
		8192,
	)

	tools := agent.GetToolDefinitions()
	systemPrompt := agent.BuildAgentSystemPrompt(taskDescription, checklist)
	initialPrompt := agent.BuildInitialPrompt(workspace, taskDescription, checklist)
	executor := NewToolExecutor(workspace)

	result, err := claudeClient.RunAgentLoop(ctx, initialPrompt, systemPrompt, tools, executor, 100)
	if err != nil {
		return AgentResult{
			Success: false,
			Error:   fmt.Sprintf("Agent reasoning step failed: %v", err),
		}, nil
	}

	slog.Info("Agent completed", "iterations", result.Iterations, "tool_calls", len(result.ToolCalls))

	filesModified := extractModifiedFiles(result.ToolCalls)

	return AgentResult{
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

func extractModifiedFiles(toolCalls []agent.ToolCall) []string {
	seen := make(map[string]bool)
	var files []string
	for _, tc := range toolCalls {
		if tc.Name == "write_file" || tc.Name == "read_file" {
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

// ToolExecutor executes tools for the Claude agent, sandboxed to a workspace.
type ToolExecutor struct {
	workspace string
}

func NewToolExecutor(workspace string) *ToolExecutor {
	return &ToolExecutor{workspace: workspace}
}

func (e *ToolExecutor) ExecuteTool(name string, input map[string]any) string {
	slog.Info("Executing tool", "name", name, "input", input)

	switch name {
	case "read_file":
		fp, _ := input["file_path"].(string)
		return e.readFile(fp)
	case "list_files":
		dir, _ := input["directory"].(string)
		return e.listFiles(dir)
	case "execute_command":
		cmd, _ := input["command"].(string)
		return e.executeCommand(cmd)
	case "search_files":
		pattern, _ := input["pattern"].(string)
		filePattern, _ := input["file_pattern"].(string)
		return e.searchFiles(pattern, filePattern)
	default:
		return fmt.Sprintf("Error: Unknown tool: %s", name)
	}
}

func (e *ToolExecutor) validatePath(relPath string) (string, error) {
	fullPath := filepath.Join(e.workspace, relPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	absWorkspace, err := filepath.Abs(e.workspace)
	if err != nil {
		return "", fmt.Errorf("resolving workspace: %w", err)
	}
	if !strings.HasPrefix(absPath, absWorkspace) {
		return "", fmt.Errorf("path %s is outside workspace", relPath)
	}
	return absPath, nil
}

func (e *ToolExecutor) readFile(filePath string) string {
	absPath, err := e.validatePath(filePath)
	if err != nil {
		return "Error: " + err.Error()
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("Error: File not found: %s", filePath)
	}
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if info.IsDir() {
		return fmt.Sprintf("Error: Not a file: %s", filePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	return string(data)
}

func (e *ToolExecutor) listFiles(directory string) string {
	absPath, err := e.validatePath(directory)
	if err != nil {
		return "Error: " + err.Error()
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Sprintf("Error: Directory not found: %s", directory)
	}
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if !info.IsDir() {
		return fmt.Sprintf("Error: Not a directory: %s", directory)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Sprintf("Error listing directory: %v", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	var items []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		items = append(items, name)
	}
	return strings.Join(items, "\n")
}

func (e *ToolExecutor) executeCommand(command string) string {
	slog.Info("Executing command", "command", command)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = e.workspace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var parts []string
	if stdout.Len() > 0 {
		parts = append(parts, "STDOUT:\n"+stdout.String())
	}
	if stderr.Len() > 0 {
		parts = append(parts, "STDERR:\n"+stderr.String())
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Error: Command timed out after 30 seconds"
		}
		parts = append(parts, fmt.Sprintf("Exit code: %v", err))
	}

	if len(parts) == 0 {
		return "Command completed with no output"
	}
	return strings.Join(parts, "\n")
}

func (e *ToolExecutor) searchFiles(pattern, filePattern string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var args []string
	args = append(args, "-r")
	if filePattern != "" {
		args = append(args, "--include="+filePattern)
	}
	args = append(args, pattern, ".")

	cmd := exec.CommandContext(ctx, "grep", args...)
	cmd.Dir = e.workspace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "Error: Search timed out after 30 seconds"
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "No matches found"
		}
		return fmt.Sprintf("Search failed: %s", stderr.String())
	}

	if stdout.Len() == 0 {
		return "No matches found"
	}
	return stdout.String()
}
