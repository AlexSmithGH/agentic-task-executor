package workflows

import (
	"fmt"
	"strings"

	"github.com/alexasmi/agentic-task-executor/internal/models"
)

func buildImplementationPrompt(auditReport, originalTask, branchName string) string {
	return fmt.Sprintf(`You are an expert software engineer implementing changes based on an audit report.

Original task: %s

Audit findings:
%s

INSTRUCTIONS:
1. Create and checkout a new branch: git checkout -b %s
2. Make the recommended changes using write_file
3. Stage ONLY the files you intentionally changed (use git add <specific files>, NOT git add .)
4. Commit with a clear, concise commit message (use execute_command to run git commit)
5. Do NOT push — the system will handle pushing after validating your commit

STRICTLY FORBIDDEN — do NOT create any of these:
- Report files (AUDIT.md, REPORT.md, SUMMARY.md, FINDINGS.md, etc.)
- Validation or check scripts (validate_*.sh, check_*.sh, verify_*.sh)
- Any file that documents YOUR process, findings, or analysis
- Any file whose purpose is to summarize what you did

The ONLY files you should create or modify are files that the project itself needs to function:
configuration files (.golangci.yml, .pre-commit-config.yaml, etc.), source code, CI workflows,
Dockerfiles, Makefiles, and similar. If the audit says "add .golangci.yml", create .golangci.yml.
If the audit says "add CI workflow", create .github/workflows/ci.yml. Do NOT create a report
about what you did.

Focus on actionable, high-priority items from the audit.
Skip findings that require external action (e.g., enabling GitHub settings).
Verify your changes compile or pass basic validation before committing.`, originalTask, auditReport, branchName)
}

func buildFeedbackTaskDescription(originalTask string, event models.PREvent) string {
	switch event.Type {
	case models.PREventCIFailure:
		return fmt.Sprintf(
			`Fix CI failure for: %s

CI failure details:
%s

INSTRUCTIONS:
1. Analyze the failure and fix the code
2. Stage ONLY the files you changed (git add <specific files>)
3. Commit with a message describing the fix
4. Do NOT push — the system handles that
5. Do NOT create any report, summary, or analysis files — only fix the actual code`,
			originalTask, event.CIDetails,
		)
	case models.PREventReviewFeedback:
		var lines []string
		for _, c := range event.Comments {
			line := fmt.Sprintf("- %s: %s", c.Author, c.Body)
			if c.Path != "" {
				line += fmt.Sprintf(" (file: %s, line: %d)", c.Path, c.Line)
			}
			lines = append(lines, line)
		}
		return fmt.Sprintf(
			`Address review feedback for: %s

Review comments:
%s

INSTRUCTIONS:
1. Address each review comment by modifying the appropriate files
2. Stage ONLY the files you changed (git add <specific files>)
3. Commit with a message describing what you addressed
4. Do NOT push — the system handles that
5. Do NOT create any report, summary, or analysis files — only modify the actual code`,
			originalTask, strings.Join(lines, "\n"),
		)
	default:
		return originalTask
	}
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

func updateSeenCommentIDs(existing []int64, comments []models.Comment) []int64 {
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
