# Agentic Task Executor

AI-powered task execution service for repository automation and analysis.

## Overview

This service provides an API for executing AI-assisted tasks against GitHub repositories using Claude and Temporal for workflow orchestration.

**Built for:** ROSAENG-59415 (SRE Automation Pattern - AI-Assisted Operational Workflows)

## Architecture

- **API Layer:** FastAPI REST API
- **Orchestration:** Temporal workflows for long-running, durable execution
- **Agent Runtime:** Claude SDK with custom tool definitions
- **State Management:** Temporal server

## Features

- Execute AI-assisted tasks against GitHub repositories
- Durable workflow execution (survives service restarts)
- Long-running workflows with external event handling
- Real-time status tracking via Temporal UI
- Automatic retries for transient failures

## Documentation

- **[Quick Reference](docs/QUICK_REFERENCE.md)** - Commands and common operations
- **[Getting Started](docs/GETTING_STARTED.md)** - Detailed setup guide with examples
- **[Architecture](docs/ARCHITECTURE.md)** - System design and component details
- **[Project Status](docs/PROJECT_STATUS.md)** - Current status and roadmap

## Quick Start

### Prerequisites

- Python 3.11+
- Docker and Docker Compose (for Temporal server)
- GitHub access token
- Google Cloud credentials (for Vertex AI)

### Local Development Setup

1. **Start Temporal server:**
   ```bash
   docker-compose up -d
   ```

2. **Install dependencies:**
   ```bash
   pip install -r requirements.txt
   ```

3. **Set environment variables:**
   ```bash
   export ANTHROPIC_API_KEY=your_key_here
   export GITHUB_TOKEN=your_token_here
   ```

4. **Start the Temporal worker:**
   ```bash
   python -m src.worker
   ```

5. **Start the API server:**
   ```bash
   uvicorn src.api:app --reload
   ```

6. **Access services:**
   - API: http://localhost:8000
   - API Docs: http://localhost:8000/docs
   - Temporal UI: http://localhost:8080

## API Usage

### Execute a Task

```bash
curl -X POST http://localhost:8000/execute-task \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/openshift/managed-cluster-validating-webhooks",
    "task_description": "Audit repository for agentic SDLC readiness",
    "checklist": [
      "Check for .golangci.yml",
      "Check for .pre-commit-config.yaml with lint/format/secrets",
      "Verify Makefile has test target",
      "Check for claude.md or agents.md",
      "Verify .claude/settings.json exists"
    ],
    "context": {
      "repo_type": "operator",
      "language": "go"
    }
  }'
```

### Check Task Status

```bash
curl http://localhost:8000/task/{workflow_id}/status
```

## Project Structure

```
agentic-task-executor/
├── src/
│   ├── api/                    # FastAPI application
│   │   ├── __init__.py
│   │   ├── routes.py          # API endpoints
│   │   └── models.py          # Request/response models
│   ├── workflows/             # Temporal workflows
│   │   ├── __init__.py
│   │   └── task_workflow.py  # Main workflow definitions
│   ├── activities/            # Temporal activities
│   │   ├── __init__.py
│   │   ├── git_operations.py # Clone, branch, commit, push
│   │   ├── agent_runtime.py  # Claude SDK integration
│   │   └── github_operations.py # PR creation, status checks
│   ├── agent/                 # Agent runtime components
│   │   ├── __init__.py
│   │   ├── claude_client.py  # Claude SDK wrapper
│   │   ├── tools.py          # Tool definitions for Claude
│   │   └── prompts.py        # System prompts and templates
│   ├── worker.py             # Temporal worker process
│   └── config.py             # Configuration management
├── tests/
│   ├── test_workflows.py
│   ├── test_activities.py
│   └── test_agent.py
├── docker-compose.yml        # Temporal server setup
├── Dockerfile               # Container image for deployment
├── requirements.txt
├── pyproject.toml
└── README.md
```

## Development

### Running Tests

```bash
pytest tests/
```

### Temporal UI

Monitor running workflows at http://localhost:8080

### Debugging

Temporal provides time-travel debugging - you can replay workflows with the exact same inputs and state.

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment instructions.

## References

- [ROSAENG-59415](https://redhat.atlassian.net/browse/ROSAENG-59415) - SRE Automation Pattern
- [ROSAENG-59414](https://redhat.atlassian.net/browse/ROSAENG-59414) - Quality Gates Tooling
- [Agentic SDLC Best Practices](https://gitlab.cee.redhat.com/global-engineering/wg-agentic-sdlc/-/blob/main/best-practices/repo-scaffolding/README.md)
