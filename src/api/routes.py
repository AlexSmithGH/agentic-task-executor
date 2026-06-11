"""FastAPI routes for task execution and workflow management."""

import logging
import uuid
from typing import Any, Dict

from fastapi import APIRouter, HTTPException, status
from temporalio.client import Client, WorkflowExecutionStatus, WorkflowHandle

from src.api.models import TaskParams, TaskResponse, TaskStatus
from src.config import settings
from src.workflows import AgenticTaskWorkflow, WorkflowInput

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/v1", tags=["tasks"])

# Global Temporal client instance
_temporal_client: Client | None = None


async def get_temporal_client() -> Client:
    """Get or create Temporal client connection.

    Returns:
        Client: Connected Temporal client instance

    Raises:
        HTTPException: If connection to Temporal fails
    """
    global _temporal_client

    if _temporal_client is None:
        try:
            logger.info(f"Connecting to Temporal at {settings.temporal_host}")
            _temporal_client = await Client.connect(
                settings.temporal_host,
                namespace=settings.temporal_namespace,
            )
            logger.info("Successfully connected to Temporal")
        except Exception as e:
            logger.error(f"Failed to connect to Temporal: {e}")
            raise HTTPException(
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
                detail=f"Failed to connect to Temporal service: {str(e)}",
            )

    return _temporal_client


@router.post("/execute-task", response_model=TaskResponse, status_code=status.HTTP_202_ACCEPTED)
async def execute_task(params: TaskParams) -> TaskResponse:
    """Start a new agentic task execution workflow.

    Args:
        params: Task parameters including repo URL and description

    Returns:
        TaskResponse: Workflow ID and run ID for tracking

    Raises:
        HTTPException: If workflow execution fails to start
    """
    try:
        client = await get_temporal_client()

        # Generate unique workflow ID
        workflow_id = f"task-{uuid.uuid4()}"

        # Convert API model to workflow input
        workflow_input = WorkflowInput(
            repo_url=str(params.repo_url),
            task_description=params.task_description,
            checklist=params.checklist,
            context=params.context or {},
        )

        logger.info(f"Starting workflow {workflow_id} for repo {params.repo_url}")

        # Start workflow execution
        handle = await client.start_workflow(
            AgenticTaskWorkflow.run,
            workflow_input,
            id=workflow_id,
            task_queue=settings.temporal_task_queue,
        )

        logger.info(
            f"Workflow started successfully: {workflow_id}, run_id: {handle.result_run_id}"
        )

        return TaskResponse(
            workflow_id=workflow_id,
            run_id=handle.result_run_id,
            status="running",
        )

    except Exception as e:
        logger.error(f"Failed to start workflow: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to start task execution: {str(e)}",
        )


@router.get("/task/{workflow_id}/status", response_model=TaskStatus)
async def get_task_status(workflow_id: str) -> TaskStatus:
    """Get the current status of a running or completed workflow.

    Args:
        workflow_id: The workflow ID to query

    Returns:
        TaskStatus: Current workflow status and result if completed

    Raises:
        HTTPException: If workflow is not found or query fails
    """
    try:
        client = await get_temporal_client()

        # Get workflow handle
        handle: WorkflowHandle = client.get_workflow_handle(workflow_id)

        # Query workflow status
        describe = await handle.describe()

        # Map Temporal status to our status string
        status_map = {
            WorkflowExecutionStatus.RUNNING: "running",
            WorkflowExecutionStatus.COMPLETED: "completed",
            WorkflowExecutionStatus.FAILED: "failed",
            WorkflowExecutionStatus.CANCELED: "canceled",
            WorkflowExecutionStatus.TERMINATED: "terminated",
            WorkflowExecutionStatus.CONTINUED_AS_NEW: "running",
            WorkflowExecutionStatus.TIMED_OUT: "timed_out",
        }

        workflow_status = status_map.get(describe.status, "unknown")

        result: Dict[str, Any] | None = None
        error: str | None = None

        # If workflow is completed, try to get result
        if describe.status == WorkflowExecutionStatus.COMPLETED:
            try:
                workflow_result = await handle.result()
                # Temporal returns results as dicts, not dataclass objects
                if isinstance(workflow_result, dict):
                    result = workflow_result
                else:
                    result = {
                        "success": workflow_result.get("success", False),
                        "summary": workflow_result.get("summary", ""),
                        "details": workflow_result.get("details", {}),
                        "pr_url": workflow_result.get("pr_url"),
                    }
            except Exception as e:
                logger.warning(f"Could not retrieve workflow result: {e}")
                logger.exception(e)  # Log full traceback for debugging

        # If workflow failed, get error message
        elif describe.status == WorkflowExecutionStatus.FAILED:
            if describe.raw_description.get("failure"):
                failure = describe.raw_description["failure"]
                error = failure.get("message", "Unknown error")

        logger.info(f"Workflow {workflow_id} status: {workflow_status}")

        return TaskStatus(
            workflow_id=workflow_id,
            run_id=describe.run_id,
            status=workflow_status,
            result=result,
            error=error,
        )

    except Exception as e:
        logger.error(f"Failed to get workflow status for {workflow_id}: {e}")
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail=f"Workflow not found or query failed: {str(e)}",
        )


@router.post("/task/{workflow_id}/signal", status_code=status.HTTP_202_ACCEPTED)
async def signal_workflow(
    workflow_id: str,
    signal_name: str,
    signal_args: Dict[str, Any] | None = None,
) -> Dict[str, str]:
    """Send a signal to a running workflow.

    Signals allow external interaction with running workflows, such as
    pausing, resuming, or providing additional input.

    Args:
        workflow_id: The workflow ID to signal
        signal_name: Name of the signal to send
        signal_args: Optional arguments for the signal

    Returns:
        Dict with confirmation message

    Raises:
        HTTPException: If workflow is not found or signal fails
    """
    try:
        client = await get_temporal_client()

        # Get workflow handle
        handle: WorkflowHandle = client.get_workflow_handle(workflow_id)

        # Send signal
        if signal_args:
            await handle.signal(signal_name, signal_args)
        else:
            await handle.signal(signal_name)

        logger.info(f"Signal '{signal_name}' sent to workflow {workflow_id}")

        return {
            "message": f"Signal '{signal_name}' sent successfully to workflow {workflow_id}",
            "workflow_id": workflow_id,
            "signal_name": signal_name,
        }

    except Exception as e:
        logger.error(f"Failed to send signal to workflow {workflow_id}: {e}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Failed to send signal: {str(e)}",
        )


@router.post("/task/{workflow_id}/cancel", status_code=status.HTTP_202_ACCEPTED)
async def cancel_workflow(workflow_id: str) -> Dict[str, str]:
    """Cancel a running workflow.

    Args:
        workflow_id: The workflow ID to cancel

    Returns:
        Dict with confirmation message

    Raises:
        HTTPException: If workflow is not found or cancellation fails
    """
    try:
        client = await get_temporal_client()

        # Get workflow handle
        handle: WorkflowHandle = client.get_workflow_handle(workflow_id)

        # Cancel workflow
        await handle.cancel()

        logger.info(f"Workflow {workflow_id} cancelled")

        return {
            "message": f"Workflow {workflow_id} cancellation requested",
            "workflow_id": workflow_id,
        }

    except Exception as e:
        logger.error(f"Failed to cancel workflow {workflow_id}: {e}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Failed to cancel workflow: {str(e)}",
        )


@router.get("/tasks", response_model=list[TaskStatus])
async def list_tasks(limit: int = 10) -> list[TaskStatus]:
    """List recent workflow executions.

    Args:
        limit: Maximum number of workflows to return (default: 10, max: 100)

    Returns:
        List of task statuses

    Raises:
        HTTPException: If query fails
    """
    try:
        if limit > 100:
            limit = 100

        client = await get_temporal_client()

        # Query for recent workflows
        # This is a simplified version - in production you'd want to use
        # Temporal's list API with proper pagination
        tasks: list[TaskStatus] = []

        logger.info(f"Listing recent workflows (limit: {limit})")

        # Note: This is a placeholder - actual implementation would use
        # client.list_workflows() or similar API
        # For now, return empty list as we don't have a proper list implementation

        return tasks

    except Exception as e:
        logger.error(f"Failed to list workflows: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to list workflows: {str(e)}",
        )
