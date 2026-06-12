package activities

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"go.temporal.io/sdk/activity"

	"github.com/alexasmi/agentic-task-executor/internal/models"
)

type GitHubActivities struct {
	Client *github.Client
}

type watcherState struct {
	LastSeenCommentIDs []int64 `json:"last_seen_comment_ids"`
	LastKnownCommitSHA string  `json:"last_known_commit_sha"`
}

func (a *GitHubActivities) CreatePullRequest(ctx context.Context, input models.PRCreateInput) (models.PRCreateResult, error) {
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
		return models.PRCreateResult{}, err
	}

	pr, _, err := a.Client.PullRequests.Create(ctx, owner, repoName, &github.NewPullRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(body),
		Head:  github.Ptr(branch),
		Base:  github.Ptr(baseBranch),
	})
	if err != nil {
		return models.PRCreateResult{}, fmt.Errorf("creating pull request: %w", err)
	}

	slog.Info("Pull request created", "number", pr.GetNumber(), "url", pr.GetHTMLURL())
	return models.PRCreateResult{
		PRURL:    pr.GetHTMLURL(),
		PRNumber: pr.GetNumber(),
		HeadSHA:  pr.GetHead().GetSHA(),
		Success:  true,
	}, nil
}

func (a *GitHubActivities) GetCIStatus(ctx context.Context, prURL string) (models.CIStatusResult, error) {
	slog.Info("Checking CI status", "pr_url", prURL)

	owner, repo, prNumber, err := parsePRURL(prURL)
	if err != nil {
		return models.CIStatusResult{}, err
	}

	commits, _, err := a.Client.PullRequests.ListCommits(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return models.CIStatusResult{}, fmt.Errorf("listing commits: %w", err)
	}

	if len(commits) == 0 {
		return models.CIStatusResult{
			Status:  models.CIStatusUnknown,
			Details: "No commits found in PR",
			Success: true,
		}, nil
	}

	latestSHA := commits[len(commits)-1].GetSHA()

	combinedStatus, _, err := a.Client.Repositories.GetCombinedStatus(ctx, owner, repo, latestSHA, nil)
	if err != nil {
		return models.CIStatusResult{}, fmt.Errorf("getting combined status: %w", err)
	}

	statusMap := map[string]models.CIStatus{
		"pending": models.CIStatusPending,
		"success": models.CIStatusSuccess,
		"failure": models.CIStatusFailure,
		"error":   models.CIStatusError,
	}

	ciStatus, ok := statusMap[combinedStatus.GetState()]
	if !ok {
		ciStatus = models.CIStatusUnknown
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
	return models.CIStatusResult{
		Status:  ciStatus,
		Details: detailsStr,
		Success: true,
	}, nil
}

func (a *GitHubActivities) GetReviewComments(ctx context.Context, prURL string) (models.CommentsResult, error) {
	slog.Info("Fetching review comments", "pr_url", prURL)

	owner, repo, prNumber, err := parsePRURL(prURL)
	if err != nil {
		return models.CommentsResult{}, err
	}

	var comments []models.Comment

	reviewComments, _, err := a.Client.PullRequests.ListComments(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return models.CommentsResult{}, fmt.Errorf("listing review comments: %w", err)
	}
	for _, rc := range reviewComments {
		c := models.Comment{
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

	issueComments, _, err := a.Client.Issues.ListComments(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return models.CommentsResult{}, fmt.Errorf("listing issue comments: %w", err)
	}
	for _, ic := range issueComments {
		c := models.Comment{
			ID:     ic.GetID(),
			Author: ic.GetUser().GetLogin(),
			Body:   ic.GetBody(),
		}
		if ic.CreatedAt != nil {
			c.CreatedAt = ic.CreatedAt.Format("2006-01-02T15:04:05Z")
		}
		comments = append(comments, c)
	}

	reviews, _, err := a.Client.PullRequests.ListReviews(ctx, owner, repo, prNumber, nil)
	if err != nil {
		return models.CommentsResult{}, fmt.Errorf("listing reviews: %w", err)
	}
	for _, r := range reviews {
		if r.GetBody() != "" {
			c := models.Comment{
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
	return models.CommentsResult{Comments: comments, Success: true}, nil
}

func (a *GitHubActivities) WatchPR(ctx context.Context, input models.PRWatchInput) (models.PREvent, error) {
	owner, repo, prNumber, err := parsePRURL(input.PRURL)
	if err != nil {
		return models.PREvent{}, err
	}

	pollInterval := 30 * time.Second
	if input.PollInterval != "" {
		if d, err := time.ParseDuration(input.PollInterval); err == nil {
			pollInterval = d
		}
	}

	lastCommitSHA := input.LastKnownCommitSHA
	seenIDs := make(map[int64]bool, len(input.LastSeenCommentIDs))
	for _, id := range input.LastSeenCommentIDs {
		seenIDs[id] = true
	}

	if activity.HasHeartbeatDetails(ctx) {
		var saved watcherState
		if err := activity.GetHeartbeatDetails(ctx, &saved); err == nil {
			lastCommitSHA = saved.LastKnownCommitSHA
			for _, id := range saved.LastSeenCommentIDs {
				seenIDs[id] = true
			}
		}
	}

	slog.Info("Watching PR", "url", input.PRURL, "poll_interval", pollInterval)

	for {
		activity.RecordHeartbeat(ctx, watcherState{
			LastSeenCommentIDs: mapKeys(seenIDs),
			LastKnownCommitSHA: lastCommitSHA,
		})

		if ctx.Err() != nil {
			return models.PREvent{}, ctx.Err()
		}

		pr, _, err := a.Client.PullRequests.Get(ctx, owner, repo, prNumber)
		if err != nil {
			slog.Warn("Failed to get PR", "error", err)
			time.Sleep(pollInterval)
			continue
		}

		if pr.GetMerged() {
			return models.PREvent{Type: models.PREventMerged, PRState: "merged"}, nil
		}
		if pr.GetState() == "closed" {
			return models.PREvent{Type: models.PREventClosed, PRState: "closed"}, nil
		}

		if lastCommitSHA != "" {
			combinedStatus, _, err := a.Client.Repositories.GetCombinedStatus(ctx, owner, repo, lastCommitSHA, nil)
			if err == nil {
				state := combinedStatus.GetState()
				if state == "failure" || state == "error" {
					var details []string
					for _, s := range combinedStatus.Statuses {
						details = append(details, fmt.Sprintf("%s: %s - %s",
							s.GetContext(), s.GetState(), s.GetDescription()))
					}
					return models.PREvent{
						Type:      models.PREventCIFailure,
						CIDetails: strings.Join(details, "\n"),
						PRState:   "open",
					}, nil
				}
			}

			checkRuns, _, err := a.Client.Checks.ListCheckRunsForRef(ctx, owner, repo, lastCommitSHA, nil)
			if err == nil && checkRuns != nil {
				for _, cr := range checkRuns.CheckRuns {
					if cr.GetStatus() == "completed" && cr.GetConclusion() == "failure" {
						detail := fmt.Sprintf("%s: failure", cr.GetName())
						if cr.Output != nil && cr.Output.Summary != nil {
							detail += " - " + cr.Output.GetSummary()
						}
						return models.PREvent{
							Type:      models.PREventCIFailure,
							CIDetails: detail,
							PRState:   "open",
						}, nil
					}
				}
			}
		}

		reviews, _, err := a.Client.PullRequests.ListReviews(ctx, owner, repo, prNumber, nil)
		if err == nil {
			hasChangesRequested := false
			for _, r := range reviews {
				if r.GetState() == "CHANGES_REQUESTED" {
					hasChangesRequested = true
					break
				}
			}

			if hasChangesRequested {
				var newComments []models.Comment

				reviewComments, _, _ := a.Client.PullRequests.ListComments(ctx, owner, repo, prNumber, nil)
				for _, rc := range reviewComments {
					if !seenIDs[rc.GetID()] {
						c := models.Comment{
							ID:     rc.GetID(),
							Author: rc.GetUser().GetLogin(),
							Body:   rc.GetBody(),
							Path:   rc.GetPath(),
							Line:   rc.GetLine(),
						}
						if rc.CreatedAt != nil {
							c.CreatedAt = rc.CreatedAt.Format("2006-01-02T15:04:05Z")
						}
						newComments = append(newComments, c)
						seenIDs[rc.GetID()] = true
					}
				}

				issueComments, _, _ := a.Client.Issues.ListComments(ctx, owner, repo, prNumber, nil)
				for _, ic := range issueComments {
					if !seenIDs[ic.GetID()] {
						c := models.Comment{
							ID:     ic.GetID(),
							Author: ic.GetUser().GetLogin(),
							Body:   ic.GetBody(),
						}
						if ic.CreatedAt != nil {
							c.CreatedAt = ic.CreatedAt.Format("2006-01-02T15:04:05Z")
						}
						newComments = append(newComments, c)
						seenIDs[ic.GetID()] = true
					}
				}

				for _, r := range reviews {
					if r.GetBody() != "" && !seenIDs[r.GetID()] {
						c := models.Comment{
							ID:     r.GetID(),
							Author: r.GetUser().GetLogin(),
							Body:   fmt.Sprintf("[Review: %s] %s", r.GetState(), r.GetBody()),
						}
						if r.SubmittedAt != nil {
							c.CreatedAt = r.SubmittedAt.Format("2006-01-02T15:04:05Z")
						}
						newComments = append(newComments, c)
						seenIDs[r.GetID()] = true
					}
				}

				if len(newComments) > 0 {
					return models.PREvent{
						Type:     models.PREventReviewFeedback,
						Comments: newComments,
						PRState:  "open",
					}, nil
				}
			}
		}

		time.Sleep(pollInterval)
	}
}

func mapKeys(m map[int64]bool) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
