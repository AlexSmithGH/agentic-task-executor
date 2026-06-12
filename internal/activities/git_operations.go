package activities

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type GitActivities struct {
	GitHubToken string
}

type CloneResult struct {
	WorkspacePath string `json:"workspace_path"`
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

func (a *GitActivities) CloneRepository(ctx context.Context, repoURL, workspaceDir string) (CloneResult, error) {
	slog.Info("Cloning repository", "url", repoURL, "dir", workspaceDir)

	parentDir := filepath.Dir(workspaceDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return CloneResult{}, fmt.Errorf("creating parent directory: %w", err)
	}

	cloneOpts := &git.CloneOptions{
		URL: repoURL,
	}
	if a.GitHubToken != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: a.GitHubToken,
		}
	}

	_, err := git.PlainCloneContext(ctx, workspaceDir, false, cloneOpts)
	if err != nil {
		return CloneResult{}, fmt.Errorf("cloning repository: %w", err)
	}

	absPath, _ := filepath.Abs(workspaceDir)
	slog.Info("Repository cloned", "path", absPath)

	return CloneResult{
		WorkspacePath: absPath,
		Success:       true,
	}, nil
}

func (a *GitActivities) CreateBranch(ctx context.Context, workspace, branchName string) (BranchResult, error) {
	slog.Info("Creating branch", "branch", branchName, "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return BranchResult{}, fmt.Errorf("opening repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return BranchResult{}, fmt.Errorf("getting worktree: %w", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})
	if err != nil {
		return BranchResult{}, fmt.Errorf("creating branch: %w", err)
	}

	slog.Info("Branch created", "branch", branchName)
	return BranchResult{
		BranchName: branchName,
		Success:    true,
	}, nil
}

func (a *GitActivities) CommitChanges(ctx context.Context, workspace, message string) (CommitResult, error) {
	slog.Info("Committing changes", "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return CommitResult{}, fmt.Errorf("opening repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return CommitResult{}, fmt.Errorf("getting worktree: %w", err)
	}

	_, err = wt.Add(".")
	if err != nil {
		return CommitResult{}, fmt.Errorf("staging changes: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return CommitResult{}, fmt.Errorf("getting status: %w", err)
	}

	if status.IsClean() {
		slog.Info("No changes to commit")
		return CommitResult{
			Success: true,
			Error:   "No changes to commit",
		}, nil
	}

	hash, err := wt.Commit(message, &git.CommitOptions{})
	if err != nil {
		return CommitResult{}, fmt.Errorf("committing changes: %w", err)
	}

	slog.Info("Changes committed", "sha", hash.String())
	return CommitResult{
		CommitSHA: hash.String(),
		Success:   true,
	}, nil
}

func (a *GitActivities) PushChanges(ctx context.Context, workspace string) (PushResult, error) {
	slog.Info("Pushing changes", "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return PushResult{}, fmt.Errorf("opening repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return PushResult{}, fmt.Errorf("getting HEAD: %w", err)
	}

	pushOpts := &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(head.Name() + ":" + head.Name())},
	}
	if a.GitHubToken != "" {
		pushOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: a.GitHubToken,
		}
	}

	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		return PushResult{}, fmt.Errorf("pushing changes: %w", err)
	}

	slog.Info("Changes pushed")
	return PushResult{Success: true}, nil
}
