package activities

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/go-github/v68/github"
)

type CIStatus string

const (
	CIStatusPending CIStatus = "pending"
	CIStatusSuccess CIStatus = "success"
	CIStatusFailure CIStatus = "failure"
	CIStatusError   CIStatus = "error"
	CIStatusUnknown CIStatus = "unknown"
)

type GitHubActivities struct {
	Client *github.Client
}

type PRCreateResult struct {
	PRURL    string `json:"pr_url"`
	PRNumber int    `json:"pr_number"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type CIStatusResult struct {
	Status  CIStatus `json:"status"`
	Details string   `json:"details"`
	Success bool     `json:"success"`
	Error   string   `json:"error,omitempty"`
}

type Comment struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

type CommentsResult struct {
	Comments []Comment `json:"comments"`
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
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

func (a *GitHubActivities) CreatePullRequest(ctx context.Context, input PRCreateInput) (PRCreateResult, error) {
	repo := input.Repo
	branch := input.BranchName
	baseBranch := input.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	title := input.CommitMessage
	body := fmt.Sprintf("## Task\n%s\n\n## Files Modified\n%s",
		input.TaskDescription, strings.Join(input.FilesModified, "\n"))

	slog.Info("Creating pull request", "repo", repo, "branch", branch, "base", baseBranch)

	owner, repoName, err := parseRepoString(repo)
	if err != nil {
		return PRCreateResult{}, err
	}

	pr, _, err := a.Client.PullRequests.Create(ctx, owner, repoName, &github.NewPullRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(body),
		Head:  github.Ptr(branch),
		Base:  github.Ptr(baseBranch),
	})
	if err != nil {
		return PRCreateResult{}, fmt.Errorf("creating pull request: %w", err)
	}

	slog.Info("Pull request created", "number", pr.GetNumber(), "url", pr.GetHTMLURL())
	return PRCreateResult{
		PRURL:    pr.GetHTMLURL(),
		PRNumber: pr.GetNumber(),
		Success:  true,
	}, nil
}

func (a *GitHubActivities) GetCIStatus(ctx context.Context, prURL string) (CIStatusResult, error) {
	slog.Info("Checking CI status", "pr_url", prURL)

	owner, repo, prNumber, err := parsePRURL(prURL)
	if err != nil {
		return CIStatusResult{}, err
	}

	pr, _, err := a.Client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return CIStatusResult{}, fmt.Errorf("getting pull request: %w", err)
	}

	commits, _, err := a.Client.PullRequests.ListCommits(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return CIStatusResult{}, fmt.Errorf("listing commits: %w", err)
	}

	if len(commits) == 0 {
		return CIStatusResult{
			Status:  CIStatusUnknown,
			Details: "No commits found in PR",
			Success: true,
		}, nil
	}

	_ = pr
	latestSHA := commits[len(commits)-1].GetSHA()

	combinedStatus, _, err := a.Client.Repositories.GetCombinedStatus(ctx, owner, repo, latestSHA, nil)
	if err != nil {
		return CIStatusResult{}, fmt.Errorf("getting combined status: %w", err)
	}

	statusMap := map[string]CIStatus{
		"pending": CIStatusPending,
		"success": CIStatusSuccess,
		"failure": CIStatusFailure,
		"error":   CIStatusError,
	}

	ciStatus, ok := statusMap[combinedStatus.GetState()]
	if !ok {
		ciStatus = CIStatusUnknown
	}

	var details []string
	for _, s := range combinedStatus.Statuses {
		details = append(details, fmt.Sprintf("%s: %s - %s",
			s.GetContext(), s.GetState(), s.GetDescription()))
	}
	detailsStr := "No status checks found"
	if len(details) > 0 {
		detailsStr = strings.Join(details, "\n")
	}

	slog.Info("CI status retrieved", "pr_number", prNumber, "status", ciStatus)
	return CIStatusResult{
		Status:  ciStatus,
		Details: detailsStr,
		Success: true,
	}, nil
}

func (a *GitHubActivities) GetReviewComments(ctx context.Context, prURL string) (CommentsResult, error) {
	slog.Info("Fetching review comments", "pr_url", prURL)

	owner, repo, prNumber, err := parsePRURL(prURL)
	if err != nil {
		return CommentsResult{}, err
	}

	var comments []Comment

	// Review comments (inline code comments)
	reviewComments, _, err := a.Client.PullRequests.ListComments(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return CommentsResult{}, fmt.Errorf("listing review comments: %w", err)
	}
	for _, rc := range reviewComments {
		c := Comment{
			ID:     rc.GetID(),
			Author: rc.GetUser().GetLogin(),
			Body:   rc.GetBody(),
			Path:   rc.GetPath(),
			Line:   rc.GetLine(),
		}
		if rc.CreatedAt != nil {
			c.CreatedAt = rc.CreatedAt.Format("2006-01-02T15:04:05Z")
		}
		comments = append(comments, c)
	}

	// Issue comments (general PR comments)
	issueComments, _, err := a.Client.Issues.ListComments(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return CommentsResult{}, fmt.Errorf("listing issue comments: %w", err)
	}
	for _, ic := range issueComments {
		c := Comment{
			ID:     ic.GetID(),
			Author: ic.GetUser().GetLogin(),
			Body:   ic.GetBody(),
		}
		if ic.CreatedAt != nil {
			c.CreatedAt = ic.CreatedAt.Format("2006-01-02T15:04:05Z")
		}
		comments = append(comments, c)
	}

	// Reviews
	reviews, _, err := a.Client.PullRequests.ListReviews(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return CommentsResult{}, fmt.Errorf("listing reviews: %w", err)
	}
	for _, r := range reviews {
		if r.GetBody() != "" {
			c := Comment{
				ID:     r.GetID(),
				Author: r.GetUser().GetLogin(),
				Body:   fmt.Sprintf("[Review: %s] %s", r.GetState(), r.GetBody()),
			}
			if r.SubmittedAt != nil {
				c.CreatedAt = r.SubmittedAt.Format("2006-01-02T15:04:05Z")
			}
			comments = append(comments, c)
		}
	}

	slog.Info("Comments retrieved", "pr_number", prNumber, "count", len(comments))
	return CommentsResult{
		Comments: comments,
		Success:  true,
	}, nil
}

func parseRepoString(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo format: %q (expected owner/repo)", repo)
	}
	return parts[0], parts[1], nil
}

func parsePRURL(prURL string) (owner, repo string, number int, err error) {
	prURL = strings.TrimRight(prURL, "/")
	parts := strings.Split(prURL, "/")
	if len(parts) < 7 || parts[5] != "pull" {
		return "", "", 0, fmt.Errorf("invalid PR URL format: %s", prURL)
	}
	num, err := strconv.Atoi(parts[6])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number in URL: %s", prURL)
	}
	return parts[3], parts[4], num, nil
}
