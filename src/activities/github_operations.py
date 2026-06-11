"""GitHub operations activities for Temporal workflows."""

import logging
import os
from dataclasses import dataclass
from enum import Enum
from typing import List, Optional

from github import Github, GithubException
from temporalio import activity

logger = logging.getLogger(__name__)


class CIStatus(Enum):
    """CI/CD pipeline status."""
    PENDING = "pending"
    SUCCESS = "success"
    FAILURE = "failure"
    ERROR = "error"
    UNKNOWN = "unknown"


@dataclass
class Comment:
    """A review comment on a pull request."""
    id: int
    author: str
    body: str
    path: Optional[str] = None
    line: Optional[int] = None
    created_at: Optional[str] = None


@dataclass
class PRResult:
    """Result of a pull request creation."""
    pr_url: str
    pr_number: int
    success: bool
    error: Optional[str] = None


@dataclass
class CIStatusResult:
    """Result of CI status check."""
    status: CIStatus
    details: str
    success: bool
    error: Optional[str] = None


@dataclass
class CommentsResult:
    """Result of fetching review comments."""
    comments: List[Comment]
    success: bool
    error: Optional[str] = None


def _get_github_client() -> Github:
    """
    Create and return an authenticated GitHub client.

    Returns:
        Authenticated PyGithub client instance

    Raises:
        ValueError: If GITHUB_TOKEN environment variable is not set
    """
    token = os.getenv("GITHUB_TOKEN")
    if not token:
        raise ValueError("GITHUB_TOKEN environment variable is required")

    return Github(token)


@activity.defn
async def create_pull_request(
    repo: str,
    branch: str,
    title: str,
    body: str,
    base_branch: str = "main"
) -> PRResult:
    """
    Create a pull request on GitHub.

    Args:
        repo: The repository in format 'owner/repo'
        branch: The branch to create the PR from
        title: The pull request title
        body: The pull request description
        base_branch: The base branch to merge into (default: 'main')

    Returns:
        PRResult with PR URL, number, and operation status
    """
    try:
        logger.info(f"Creating pull request in {repo} from {branch} to {base_branch}")

        # Get GitHub client
        gh = _get_github_client()

        # Get the repository
        repository = gh.get_repo(repo)

        # Create the pull request
        pr = repository.create_pull(
            title=title,
            body=body,
            head=branch,
            base=base_branch
        )

        logger.info(f"Successfully created PR #{pr.number}: {pr.html_url}")
        return PRResult(
            pr_url=pr.html_url,
            pr_number=pr.number,
            success=True
        )

    except GithubException as e:
        error_msg = f"GitHub API error: {e.status} - {e.data.get('message', str(e))}"
        logger.error(error_msg)
        return PRResult(
            pr_url="",
            pr_number=0,
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to create pull request: {e}"
        logger.error(error_msg)
        return PRResult(
            pr_url="",
            pr_number=0,
            success=False,
            error=error_msg
        )


@activity.defn
async def get_ci_status(pr_url: str) -> CIStatusResult:
    """
    Get the CI/CD status for a pull request.

    Args:
        pr_url: The URL of the pull request

    Returns:
        CIStatusResult with status and details
    """
    try:
        logger.info(f"Checking CI status for PR: {pr_url}")

        # Parse the PR URL to get repo and PR number
        # Expected format: https://github.com/owner/repo/pull/123
        parts = pr_url.rstrip('/').split('/')
        if len(parts) < 7 or parts[5] != 'pull':
            raise ValueError(f"Invalid PR URL format: {pr_url}")

        repo_name = f"{parts[3]}/{parts[4]}"
        pr_number = int(parts[6])

        # Get GitHub client
        gh = _get_github_client()

        # Get the repository and pull request
        repository = gh.get_repo(repo_name)
        pr = repository.get_pull(pr_number)

        # Get the latest commit
        commits = list(pr.get_commits())
        if not commits:
            return CIStatusResult(
                status=CIStatus.UNKNOWN,
                details="No commits found in PR",
                success=True
            )

        latest_commit = commits[-1]

        # Get the combined status
        combined_status = latest_commit.get_combined_status()

        # Map GitHub status to our CIStatus enum
        status_mapping = {
            "pending": CIStatus.PENDING,
            "success": CIStatus.SUCCESS,
            "failure": CIStatus.FAILURE,
            "error": CIStatus.ERROR,
        }

        ci_status = status_mapping.get(combined_status.state, CIStatus.UNKNOWN)

        # Collect status details
        details = []
        for status in combined_status.statuses:
            details.append(f"{status.context}: {status.state} - {status.description}")

        details_str = "\n".join(details) if details else "No status checks found"

        logger.info(f"CI status for PR #{pr_number}: {ci_status.value}")
        return CIStatusResult(
            status=ci_status,
            details=details_str,
            success=True
        )

    except GithubException as e:
        error_msg = f"GitHub API error: {e.status} - {e.data.get('message', str(e))}"
        logger.error(error_msg)
        return CIStatusResult(
            status=CIStatus.UNKNOWN,
            details="",
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to get CI status: {e}"
        logger.error(error_msg)
        return CIStatusResult(
            status=CIStatus.UNKNOWN,
            details="",
            success=False,
            error=error_msg
        )


@activity.defn
async def get_review_comments(pr_url: str) -> CommentsResult:
    """
    Get all review comments for a pull request.

    Args:
        pr_url: The URL of the pull request

    Returns:
        CommentsResult with list of comments and operation status
    """
    try:
        logger.info(f"Fetching review comments for PR: {pr_url}")

        # Parse the PR URL to get repo and PR number
        parts = pr_url.rstrip('/').split('/')
        if len(parts) < 7 or parts[5] != 'pull':
            raise ValueError(f"Invalid PR URL format: {pr_url}")

        repo_name = f"{parts[3]}/{parts[4]}"
        pr_number = int(parts[6])

        # Get GitHub client
        gh = _get_github_client()

        # Get the repository and pull request
        repository = gh.get_repo(repo_name)
        pr = repository.get_pull(pr_number)

        # Collect all comments
        comments = []

        # Get review comments (inline comments on code)
        for review_comment in pr.get_review_comments():
            comments.append(Comment(
                id=review_comment.id,
                author=review_comment.user.login,
                body=review_comment.body,
                path=review_comment.path,
                line=review_comment.line,
                created_at=review_comment.created_at.isoformat() if review_comment.created_at else None
            ))

        # Get issue comments (general PR comments)
        for issue_comment in pr.get_issue_comments():
            comments.append(Comment(
                id=issue_comment.id,
                author=issue_comment.user.login,
                body=issue_comment.body,
                created_at=issue_comment.created_at.isoformat() if issue_comment.created_at else None
            ))

        # Get review comments (from reviews)
        for review in pr.get_reviews():
            if review.body:  # Only include reviews with body text
                comments.append(Comment(
                    id=review.id,
                    author=review.user.login,
                    body=f"[Review: {review.state}] {review.body}",
                    created_at=review.submitted_at.isoformat() if review.submitted_at else None
                ))

        logger.info(f"Found {len(comments)} comments for PR #{pr_number}")
        return CommentsResult(
            comments=comments,
            success=True
        )

    except GithubException as e:
        error_msg = f"GitHub API error: {e.status} - {e.data.get('message', str(e))}"
        logger.error(error_msg)
        return CommentsResult(
            comments=[],
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to get review comments: {e}"
        logger.error(error_msg)
        return CommentsResult(
            comments=[],
            success=False,
            error=error_msg
        )
