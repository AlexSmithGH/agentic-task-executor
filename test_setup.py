#!/usr/bin/env python3
"""Test script to verify the agentic task executor setup."""

import asyncio
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


async def test_configuration():
    """Test that configuration loads correctly."""
    logger.info("Testing configuration...")
    from src.config import settings

    logger.info(f"  GCP Project: {settings.gcp_project_id}")
    logger.info(f"  GCP Region: {settings.gcp_region}")
    logger.info(f"  Temporal Host: {settings.temporal_host}")
    logger.info(f"  GitHub Token: {'✓ Set' if settings.github_token and len(settings.github_token) > 10 else '✗ NOT SET'}")
    logger.info("✓ Configuration OK\n")


async def test_temporal_connection():
    """Test connection to Temporal server."""
    logger.info("Testing Temporal connection...")
    from temporalio.client import Client
    from src.config import settings

    try:
        client = await Client.connect(settings.temporal_host)
        logger.info(f"  Connected to: {settings.temporal_host}")
        logger.info("✓ Temporal connection OK\n")
        return client
    except Exception as e:
        logger.error(f"✗ Temporal connection FAILED: {e}\n")
        raise


async def test_vertex_ai_client():
    """Test Vertex AI Claude client initialization."""
    logger.info("Testing Vertex AI Claude client...")
    from src.agent.claude_client import ClaudeClient
    from src.config import settings

    try:
        client = ClaudeClient(
            project_id=settings.gcp_project_id,
            region=settings.gcp_region
        )
        logger.info(f"  Project: {client.project_id}")
        logger.info(f"  Region: {client.region}")
        logger.info(f"  Model: {client.model}")
        logger.info("✓ Vertex AI client OK\n")
        return client
    except Exception as e:
        logger.error(f"✗ Vertex AI client FAILED: {e}\n")
        raise


async def test_activity_imports():
    """Test that all activities can be imported."""
    logger.info("Testing activity imports...")
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
        activities = [
            clone_repository,
            create_branch,
            commit_changes,
            push_changes,
            agent_reasoning_step,
            create_pull_request,
            get_ci_status,
            get_review_comments,
        ]
        logger.info(f"  Loaded {len(activities)} activities")
        for act in activities:
            logger.info(f"    - {act.__name__}")
        logger.info("✓ Activity imports OK\n")
    except Exception as e:
        logger.error(f"✗ Activity imports FAILED: {e}\n")
        raise


async def test_workflow_import():
    """Test that workflow can be imported."""
    logger.info("Testing workflow import...")
    try:
        from src.workflows import AgenticTaskWorkflow
        logger.info(f"  Loaded workflow: {AgenticTaskWorkflow.__name__}")
        logger.info("✓ Workflow import OK\n")
    except Exception as e:
        logger.error(f"✗ Workflow import FAILED: {e}\n")
        raise


async def main():
    """Run all tests."""
    logger.info("=" * 60)
    logger.info("Agentic Task Executor - Setup Test")
    logger.info("=" * 60 + "\n")

    try:
        await test_configuration()
        await test_temporal_connection()
        await test_vertex_ai_client()
        await test_activity_imports()
        await test_workflow_import()

        logger.info("=" * 60)
        logger.info("✓ ALL TESTS PASSED - System is ready!")
        logger.info("=" * 60)
        logger.info("\nNext steps:")
        logger.info("  1. Start worker: python -m src.worker")
        logger.info("  2. Start API: uvicorn src.api:app --reload")
        logger.info("  3. Test API: curl http://localhost:8000/health")

    except Exception as e:
        logger.error("\n" + "=" * 60)
        logger.error("✗ TESTS FAILED")
        logger.error("=" * 60)
        raise


if __name__ == "__main__":
    asyncio.run(main())
