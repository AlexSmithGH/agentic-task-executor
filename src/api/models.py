"""API request and response models."""

from typing import Any, Dict, List, Optional

from pydantic import BaseModel, Field, HttpUrl


class TaskParams(BaseModel):
    """Parameters for executing an agentic task."""

    repo_url: HttpUrl = Field(..., description="GitHub repository URL")
    task_description: str = Field(..., description="High-level description of the task")
    checklist: Optional[List[str]] = Field(
        default=None, description="Optional checklist of items to verify"
    )
    context: Optional[Dict[str, Any]] = Field(
        default=None, description="Additional context for the agent"
    )


class TaskResponse(BaseModel):
    """Response from task execution request."""

    workflow_id: str = Field(..., description="Temporal workflow ID for tracking")
    run_id: str = Field(..., description="Temporal run ID")
    status: str = Field(default="running", description="Initial status")


class TaskStatus(BaseModel):
    """Status of a running or completed task."""

    workflow_id: str
    run_id: str
    status: str = Field(..., description="Workflow status: running, completed, failed")
    result: Optional[Dict[str, Any]] = Field(
        default=None, description="Task result if completed"
    )
    error: Optional[str] = Field(default=None, description="Error message if failed")


class TaskResult(BaseModel):
    """Final result from a completed task."""

    success: bool
    summary: str = Field(..., description="Summary of what was accomplished")
    details: Dict[str, Any] = Field(..., description="Detailed results")
    pr_url: Optional[str] = Field(default=None, description="Pull request URL if created")
