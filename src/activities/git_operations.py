"""Git operations activities for Temporal workflows."""

import logging
import os
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

from git import Repo, GitCommandError
from temporalio import activity

logger = logging.getLogger(__name__)


@dataclass
class CloneResult:
    """Result of a repository clone operation."""
    workspace_path: str
    success: bool
    error: Optional[str] = None


@dataclass
class BranchResult:
    """Result of a branch creation operation."""
    branch_name: str
    success: bool
    error: Optional[str] = None


@dataclass
class CommitResult:
    """Result of a commit operation."""
    commit_sha: str
    success: bool
    error: Optional[str] = None


@dataclass
class PushResult:
    """Result of a push operation."""
    success: bool
    error: Optional[str] = None


@activity.defn
async def clone_repository(repo_url: str, workspace_dir: str) -> CloneResult:
    """
    Clone a Git repository to a workspace directory.

    Args:
        repo_url: The URL of the repository to clone
        workspace_dir: The directory path where the repository will be cloned

    Returns:
        CloneResult with workspace path and operation status
    """
    try:
        logger.info(f"Cloning repository {repo_url} to {workspace_dir}")

        # Ensure the parent directory exists
        workspace_path = Path(workspace_dir)
        workspace_path.parent.mkdir(parents=True, exist_ok=True)

        # Clone the repository
        repo = Repo.clone_from(repo_url, workspace_dir)

        logger.info(f"Successfully cloned repository to {workspace_dir}")
        return CloneResult(
            workspace_path=str(workspace_path.absolute()),
            success=True
        )

    except GitCommandError as e:
        error_msg = f"Git command failed: {e}"
        logger.error(error_msg)
        return CloneResult(
            workspace_path="",
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to clone repository: {e}"
        logger.error(error_msg)
        return CloneResult(
            workspace_path="",
            success=False,
            error=error_msg
        )


@activity.defn
async def create_branch(workspace: str, branch_name: str) -> BranchResult:
    """
    Create a new Git branch in the workspace.

    Args:
        workspace: The path to the Git repository workspace
        branch_name: The name of the branch to create

    Returns:
        BranchResult with branch name and operation status
    """
    try:
        logger.info(f"Creating branch {branch_name} in {workspace}")

        # Open the repository
        repo = Repo(workspace)

        # Create and checkout the new branch
        new_branch = repo.create_head(branch_name)
        new_branch.checkout()

        logger.info(f"Successfully created and checked out branch {branch_name}")
        return BranchResult(
            branch_name=branch_name,
            success=True
        )

    except GitCommandError as e:
        error_msg = f"Git command failed: {e}"
        logger.error(error_msg)
        return BranchResult(
            branch_name=branch_name,
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to create branch: {e}"
        logger.error(error_msg)
        return BranchResult(
            branch_name=branch_name,
            success=False,
            error=error_msg
        )


@activity.defn
async def commit_changes(workspace: str, message: str) -> CommitResult:
    """
    Commit all changes in the workspace.

    Args:
        workspace: The path to the Git repository workspace
        message: The commit message

    Returns:
        CommitResult with commit SHA and operation status
    """
    try:
        logger.info(f"Committing changes in {workspace}")

        # Open the repository
        repo = Repo(workspace)

        # Stage all changes
        repo.git.add(A=True)

        # Check if there are changes to commit
        if not repo.index.diff("HEAD") and not repo.untracked_files:
            logger.info("No changes to commit")
            return CommitResult(
                commit_sha="",
                success=True,
                error="No changes to commit"
            )

        # Commit the changes
        commit = repo.index.commit(message)

        logger.info(f"Successfully committed changes with SHA {commit.hexsha}")
        return CommitResult(
            commit_sha=commit.hexsha,
            success=True
        )

    except GitCommandError as e:
        error_msg = f"Git command failed: {e}"
        logger.error(error_msg)
        return CommitResult(
            commit_sha="",
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to commit changes: {e}"
        logger.error(error_msg)
        return CommitResult(
            commit_sha="",
            success=False,
            error=error_msg
        )


@activity.defn
async def push_changes(workspace: str) -> PushResult:
    """
    Push committed changes to the remote repository.

    Args:
        workspace: The path to the Git repository workspace

    Returns:
        PushResult with operation status
    """
    try:
        logger.info(f"Pushing changes from {workspace}")

        # Open the repository
        repo = Repo(workspace)

        # Get the current branch
        current_branch = repo.active_branch

        # Push to origin
        origin = repo.remote(name="origin")
        origin.push(current_branch.name)

        logger.info(f"Successfully pushed branch {current_branch.name}")
        return PushResult(success=True)

    except GitCommandError as e:
        error_msg = f"Git command failed: {e}"
        logger.error(error_msg)
        return PushResult(
            success=False,
            error=error_msg
        )
    except Exception as e:
        error_msg = f"Failed to push changes: {e}"
        logger.error(error_msg)
        return PushResult(
            success=False,
            error=error_msg
        )
