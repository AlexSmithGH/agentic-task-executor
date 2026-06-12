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

	"github.com/alexasmi/agentic-task-executor/internal/models"
)

type GitActivities struct {
	GitHubToken string
}

func (a *GitActivities) CloneRepository(ctx context.Context, repoURL, workspaceDir string) (models.CloneResult, error) {
	slog.Info("Cloning repository", "url", repoURL, "dir", workspaceDir)

	parentDir := filepath.Dir(workspaceDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return models.CloneResult{}, fmt.Errorf("creating parent directory: %w", err)
	}

	cloneOpts := &git.CloneOptions{URL: repoURL}
	if a.GitHubToken != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token",
			Password: a.GitHubToken,
		}
	}

	repo, err := git.PlainCloneContext(ctx, workspaceDir, false, cloneOpts)
	if err != nil {
		return models.CloneResult{}, fmt.Errorf("cloning repository: %w", err)
	}

	absPath, _ := filepath.Abs(workspaceDir)

	defaultBranch := "master"
	if head, err := repo.Head(); err == nil {
		defaultBranch = head.Name().Short()
	}

	slog.Info("Repository cloned", "path", absPath, "default_branch", defaultBranch)
	return models.CloneResult{
		WorkspacePath: absPath,
		DefaultBranch: defaultBranch,
		Success:       true,
	}, nil
}

func (a *GitActivities) CreateBranch(ctx context.Context, workspace, branchName string) (models.BranchResult, error) {
	slog.Info("Creating branch", "branch", branchName, "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return models.BranchResult{}, fmt.Errorf("opening repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return models.BranchResult{}, fmt.Errorf("getting worktree: %w", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})
	if err != nil {
		return models.BranchResult{}, fmt.Errorf("creating branch: %w", err)
	}

	slog.Info("Branch created", "branch", branchName)
	return models.BranchResult{BranchName: branchName, Success: true}, nil
}

func (a *GitActivities) CommitChanges(ctx context.Context, workspace, message string) (models.CommitResult, error) {
	slog.Info("Committing changes", "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return models.CommitResult{}, fmt.Errorf("opening repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return models.CommitResult{}, fmt.Errorf("getting worktree: %w", err)
	}

	if _, err = wt.Add("."); err != nil {
		return models.CommitResult{}, fmt.Errorf("staging changes: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return models.CommitResult{}, fmt.Errorf("getting status: %w", err)
	}

	if status.IsClean() {
		slog.Info("No changes to commit")
		return models.CommitResult{Success: true, Error: "No changes to commit"}, nil
	}

	hash, err := wt.Commit(message, &git.CommitOptions{})
	if err != nil {
		return models.CommitResult{}, fmt.Errorf("committing changes: %w", err)
	}

	slog.Info("Changes committed", "sha", hash.String())
	return models.CommitResult{CommitSHA: hash.String(), Success: true}, nil
}

func (a *GitActivities) PushChanges(ctx context.Context, workspace string) (models.PushResult, error) {
	slog.Info("Pushing changes", "workspace", workspace)

	repo, err := git.PlainOpen(workspace)
	if err != nil {
		return models.PushResult{}, fmt.Errorf("opening repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return models.PushResult{}, fmt.Errorf("getting HEAD: %w", err)
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

	if err = repo.PushContext(ctx, pushOpts); err != nil {
		return models.PushResult{}, fmt.Errorf("pushing changes: %w", err)
	}

	slog.Info("Changes pushed")
	return models.PushResult{Success: true}, nil
}
