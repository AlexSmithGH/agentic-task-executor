"""Temporal worker process for agentic task execution.

This module implements the Temporal worker that executes workflows and activities.
Run with: python -m src.worker
"""

import asyncio
import logging
import signal
import sys
from typing import Optional

from temporalio.client import Client
from temporalio.worker import Worker

from src.config import settings

# Configure logging
logging.basicConfig(
    level=settings.log_level,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)

# Global worker instance for graceful shutdown
_worker: Optional[Worker] = None
_shutdown_event: Optional[asyncio.Event] = None


def signal_handler(sig: int, frame) -> None:
    """Handle shutdown signals gracefully."""
    logger.info(f"Received signal {sig}, initiating graceful shutdown...")
    if _shutdown_event:
        _shutdown_event.set()


async def run_worker() -> None:
    """Initialize and run the Temporal worker.

    This function:
    1. Connects to the Temporal server
    2. Registers workflows and activities
    3. Starts the worker
    4. Waits for shutdown signal
    5. Gracefully shuts down
    """
    global _worker, _shutdown_event

    logger.info("Starting Temporal worker...")
    logger.info(f"Temporal host: {settings.temporal_host}")
    logger.info(f"Temporal namespace: {settings.temporal_namespace}")
    logger.info(f"Task queue: {settings.temporal_task_queue}")

    # Create shutdown event
    _shutdown_event = asyncio.Event()

    try:
        # Connect to Temporal server
        logger.info(f"Connecting to Temporal server at {settings.temporal_host}...")
        client = await Client.connect(
            settings.temporal_host,
            namespace=settings.temporal_namespace,
        )
        logger.info("Successfully connected to Temporal server")

        # Import workflows and activities
        # These imports are done here to avoid circular dependencies
        # and to ensure they're only loaded when the worker starts
        try:
            from src.workflows import AgenticTaskWorkflow
            logger.info("Loaded workflow: AgenticTaskWorkflow")
        except ImportError as e:
            logger.error(f"Failed to import workflows: {e}")
            logger.warning("Worker will start but workflows may not be available")
            AgenticTaskWorkflow = None

        # Try to import activities if they exist
        activities_list = []
        try:
            from src.activities import (
                clone_repository,
                create_branch,
                commit_changes,
                push_changes,
                agent_reasoning_step,
                create_pull_request,
                get_ci_status,
                get_review_comments,
            )
            activities_list = [
                clone_repository,
                create_branch,
                commit_changes,
                push_changes,
                agent_reasoning_step,
                create_pull_request,
                get_ci_status,
                get_review_comments,
            ]
            logger.info(f"Loaded {len(activities_list)} activities")
        except ImportError as e:
            logger.warning(f"Activities module not found or incomplete: {e}")
            logger.warning("Worker will start but activities may not be available")

        # Collect workflows to register
        workflows = []
        if AgenticTaskWorkflow:
            workflows.append(AgenticTaskWorkflow)

        if not workflows and not activities_list:
            logger.error("No workflows or activities available to register")
            logger.error("Please ensure src/workflows and src/activities are implemented")
            sys.exit(1)

        # Create and start worker
        logger.info(f"Creating worker on task queue: {settings.temporal_task_queue}")
        _worker = Worker(
            client,
            task_queue=settings.temporal_task_queue,
            workflows=workflows,
            activities=activities_list,
        )

        logger.info("Worker created successfully")
        logger.info("=" * 60)
        logger.info("Worker configuration:")
        logger.info(f"  Workflows: {[w.__name__ for w in workflows]}")
        logger.info(f"  Activities: {[a.__name__ for a in activities_list]}")
        logger.info(f"  Task queue: {settings.temporal_task_queue}")
        logger.info("=" * 60)
        logger.info("Worker is running. Press Ctrl+C to stop.")

        # Run worker until shutdown signal
        async with _worker:
            await _shutdown_event.wait()

        logger.info("Worker stopped gracefully")

    except asyncio.CancelledError:
        logger.info("Worker task cancelled")
    except Exception as e:
        logger.error(f"Worker error: {e}", exc_info=True)
        raise
    finally:
        logger.info("Worker shutdown complete")


def main() -> None:
    """Entry point for the worker process."""
    # Register signal handlers for graceful shutdown
    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    logger.info("=" * 60)
    logger.info("Agentic Task Executor - Temporal Worker")
    logger.info("=" * 60)

    # Run the worker
    try:
        asyncio.run(run_worker())
    except KeyboardInterrupt:
        logger.info("Shutdown initiated by user")
    except Exception as e:
        logger.error(f"Fatal error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
