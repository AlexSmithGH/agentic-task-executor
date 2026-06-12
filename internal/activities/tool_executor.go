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
)

var blockedFilePatterns = []string{
	"_REPORT", "_SUMMARY", "_AUDIT", "_FINDINGS", "_ANALYSIS",
	"REPORT.md", "SUMMARY.md", "AUDIT.md", "FINDINGS.md",
	"validate_", "check_", "verify_",
}

func isBlockedFile(filePath string) bool {
	base := strings.ToUpper(filepath.Base(filePath))
	for _, pattern := range blockedFilePatterns {
		if strings.Contains(base, pattern) {
			return true
		}
	}
	return false
}

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
	case "write_file":
		fp, _ := input["file_path"].(string)
		content, _ := input["content"].(string)
		return e.writeFile(fp, content)
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

func (e *ToolExecutor) writeFile(filePath, content string) string {
	if isBlockedFile(filePath) {
		return fmt.Sprintf("Error: Refused to create %s — do not create report, summary, audit, or validation script files. Only create files the project actually needs.", filePath)
	}

	absPath, err := e.validatePath(filePath)
	if err != nil {
		return "Error: " + err.Error()
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("Error creating directory: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Sprintf("Error writing file: %v", err)
	}
	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), filePath)
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

	args := []string{"-r"}
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
