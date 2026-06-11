"""Temporal workflow for orchestrating agentic task execution.

This workflow coordinates the full lifecycle of an agentic task:
1. Clone repository to isolated workspace
2. Execute agent reasoning loop with Claude
3. Create pull request if changes made
4. Wait for CI results (optional)
5. Handle failures with retry logic
"""

from dataclasses import dataclass
from datetime import timedelta
from typing import Any, Dict, List, Optional

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from anthropic import Anthropic


@dataclass
class WorkflowInput:
    """Input parameters for the agentic task workflow.

    Attributes:
        repo_url: GitHub repository URL to clone
        task_description: High-level description of what the agent should accomplish
        checklist: Optional list of verification items for the agent to check
        context: Additional context dictionary for the agent (env vars, constraints, etc)
        wait_for_ci: Whether to wait for CI completion after creating PR
        branch_name: Optional custom branch name (auto-generated if not provided)
    """

    repo_url: str
    task_description: str
    checklist: Optional[List[str]] = None
    context: Optional[Dict[str, Any]] = None
    wait_for_ci: bool = False
    branch_name: Optional[str] = None


@dataclass
class WorkflowResult:
    """Result of the agentic task workflow.

    Attributes:
        success: Whether the task completed successfully
        summary: Human-readable summary of what was accomplished
        details: Detailed execution information (commits, files changed, etc)
        pr_url: URL of created pull request (if any)
        ci_status: CI check status if wait_for_ci was enabled
        workspace_path: Path to the workspace directory
        error_message: Error description if task failed
    """

    success: bool
    summary: str
    details: Dict[str, Any]
    pr_url: Optional[str] = None
    ci_status: Optional[str] = None
    workspace_path: Optional[str] = None
    error_message: Optional[str] = None


@dataclass
class CloneResult:
    """Result from repository clone activity."""

    workspace_path: str
    repo_name: str
    default_branch: str


@dataclass
class AgentResult:
    """Result from agent reasoning loop activity."""

    success: bool
    changes_made: bool
    summary: str
    files_modified: List[str]
    commit_message: Optional[str] = None
    error: Optional[str] = None


@dataclass
class PRResult:
    """Result from PR creation activity."""

    pr_url: str
    pr_number: int
    branch_name: str


@workflow.defn
class AgenticTaskWorkflow:
    """Temporal workflow for executing agentic tasks with Claude.

    This workflow orchestrates the complete lifecycle of an autonomous task:
    - Repository setup and isolation
    - Agent execution with retry logic
    - PR creation and CI monitoring
    - Cleanup and error handling
    """

    def __init__(self) -> None:
        """Initialize workflow state."""
        self._workspace_path: Optional[str] = None
        self._pr_url: Optional[str] = None
        self._ci_status: Optional[str] = None

    @workflow.run
    async def run(self, input_data: WorkflowInput) -> WorkflowResult:
        """Execute the agentic task workflow.

        Args:
            input_data: Workflow input parameters

        Returns:
            WorkflowResult containing execution outcome and details

        Raises:
            ApplicationError: For unrecoverable failures
        """
        workflow.logger.info(
            f"Starting agentic task workflow for repo: {input_data.repo_url}",
            extra={"task": input_data.task_description}
        )

        try:
            # Step 1: Clone repository to isolated workspace
            # Generate unique workspace directory based on workflow ID
            # Note: Using hardcoded path because os.getenv() is non-deterministic in Temporal workflows
            workspace_dir = f"/tmp/agentic-workspaces/{workflow.info().workflow_id}"

            clone_result = await workflow.execute_activity(
                "clone_repository",
                args=[input_data.repo_url, workspace_dir],
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=RetryPolicy(
                    maximum_attempts=3,
                    initial_interval=timedelta(seconds=1),
                    maximum_interval=timedelta(seconds=10),
                ),
            )
            # Temporal returns results as dicts, not dataclass objects
            self._workspace_path = clone_result["workspace_path"]

            workflow.logger.info(
                f"Repository cloned to {clone_result['workspace_path']}"
            )

            # Step 2: Execute agent reasoning loop
            agent_result = await workflow.execute_activity(
                "agent_reasoning_step",
                args=[
                    clone_result["workspace_path"],
                    input_data.task_description,
                    input_data.checklist or [],
                    input_data.context or {},
                ],
                start_to_close_timeout=timedelta(minutes=30),
                retry_policy=RetryPolicy(
                    maximum_attempts=2,  # Limited retries for agent execution
                    initial_interval=timedelta(seconds=5),
                    non_retryable_error_types=["ValidationError"],
                ),
                heartbeat_timeout=timedelta(minutes=5),
            )

            if not agent_result.get("success", False):
                return WorkflowResult(
                    success=False,
                    summary="Agent execution failed",
                    details={"error": agent_result.get("error", "Unknown error")},
                    workspace_path=self._workspace_path,
                    error_message=agent_result.get("error", "Unknown error"),
                )

            workflow.logger.info(
                f"Agent completed: {agent_result.get('summary', 'No summary')}"
            )

            # Step 3: Create PR if changes were made
            if agent_result.get("changes_made", False):
                pr_result = await workflow.execute_activity(
                    "create_pull_request",
                    {
                        "workspace_path": clone_result["workspace_path"],
                        "branch_name": input_data.branch_name,
                        "commit_message": agent_result.get("commit_message", "Agent changes"),
                        "task_description": input_data.task_description,
                        "files_modified": agent_result.get("files_modified", []),
                    },
                    start_to_close_timeout=timedelta(minutes=3),
                    retry_policy=RetryPolicy(
                        maximum_attempts=3,
                        initial_interval=timedelta(seconds=2),
                    ),
                )
                self._pr_url = pr_result.get("pr_url")

                workflow.logger.info(
                    f"Pull request created: {pr_result.get('pr_url')}"
                )

                # Step 4: Wait for CI results if requested
                if input_data.wait_for_ci:
                    ci_result = await workflow.execute_activity(
                        "wait_for_ci_completion",
                        {
                            "pr_number": pr_result.pr_number,
                            "repo_url": input_data.repo_url,
                        },
                        start_to_close_timeout=timedelta(minutes=20),
                        retry_policy=RetryPolicy(
                            maximum_attempts=1,  # No retries for CI wait
                        ),
                    )
                    self._ci_status = ci_result.get("status", "unknown")

                    workflow.logger.info(
                        f"CI status: {self._ci_status}",
                        extra={"checks": ci_result.get("checks", [])}
                    )

            # Build final result
            return WorkflowResult(
                success=True,
                summary=agent_result.get("summary", "Task completed"),
                details={
                    "files_modified": agent_result.get("files_modified", []),
                    "changes_made": agent_result.get("changes_made", False),
                    "reasoning": agent_result.get("reasoning", ""),
                },
                pr_url=self._pr_url,
                ci_status=self._ci_status,
                workspace_path=self._workspace_path,
            )

        except Exception as e:
            workflow.logger.error(
                f"Workflow failed with error: {str(e)}",
                exc_info=True
            )

            return WorkflowResult(
                success=False,
                summary=f"Task failed: {type(e).__name__}",
                details={"exception": str(e)},
                workspace_path=self._workspace_path,
                error_message=str(e),
            )

    @workflow.signal
    async def cancel_task(self) -> None:
        """Signal handler to cancel the running task.

        This allows external cancellation of long-running tasks.
        """
        workflow.logger.warning("Task cancellation requested")
        raise workflow.CancelledError("Task cancelled by user")

    @workflow.query
    def get_status(self) -> Dict[str, Any]:
        """Query handler to get current workflow status.

        Returns:
            Dictionary with current state information
        """
        return {
            "workspace_path": self._workspace_path,
            "pr_url": self._pr_url,
            "ci_status": self._ci_status,
        }
