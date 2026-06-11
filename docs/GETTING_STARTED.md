# Getting Started with Agentic Task Executor

This guide will walk you through setting up and running your first agentic task.

## Prerequisites

- Python 3.11 or higher
- Docker and Docker Compose
- GitHub personal access token
- Anthropic API key (Claude)

## Installation

### 1. Clone and Navigate

```bash
cd agentic-task-executor
```

### 2. Create Virtual Environment

```bash
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

### 3. Install Dependencies

```bash
pip install -r requirements.txt
```

### 4. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and add your credentials:

```bash
ANTHROPIC_API_KEY=sk-ant-api03-...
GITHUB_TOKEN=ghp_...
```

### 5. Start Temporal Server

```bash
docker-compose up -d
```

Verify it's running:
- Temporal UI: http://localhost:8080
- PostgreSQL: localhost:5432

## Running the Service

You need to run two processes:

### Terminal 1: Start the Worker

```bash
python -m src.worker
```

You should see:
```
Starting Temporal worker...
Connected to Temporal at localhost:7233
Loaded workflows: AgenticTaskWorkflow
Loaded activities: clone_repository, create_branch, ...
Worker started successfully. Press Ctrl+C to stop.
```

### Terminal 2: Start the API Server

```bash
uvicorn src.api:app --reload
```

You should see:
```
INFO:     Uvicorn running on http://127.0.0.1:8000 (Press CTRL+C to quit)
INFO:     Started reloader process
```

### Alternative: Use Make

```bash
# Start Temporal
make docker-up

# In separate terminals:
make run-worker
make run-api
```

## Your First Task

### 1. Check API Health

```bash
curl http://localhost:8000/health
```

Expected response:
```json
{"status": "healthy"}
```

### 2. Execute a Repository Audit Task

```bash
curl -X POST http://localhost:8000/api/v1/execute-task \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/openshift/managed-cluster-validating-webhooks",
    "task_description": "Audit repository for agentic SDLC readiness",
    "checklist": [
      "Check for .golangci.yml presence",
      "Check for .pre-commit-config.yaml with lint, format, and secret detection",
      "Verify Makefile has a test target",
      "Check for claude.md or agents.md documentation",
      "Verify .claude/settings.json exists with hooks configuration"
    ],
    "context": {
      "repo_type": "operator",
      "language": "go"
    }
  }'
```

Expected response:
```json
{
  "workflow_id": "audit-task-abc123",
  "run_id": "xyz789",
  "status": "running"
}
```

### 3. Check Task Status

Using the `workflow_id` from the response:

```bash
curl http://localhost:8000/api/v1/task/audit-task-abc123/status
```

Response while running:
```json
{
  "workflow_id": "audit-task-abc123",
  "run_id": "xyz789",
  "status": "running",
  "result": null,
  "error": null
}
```

Response when complete:
```json
{
  "workflow_id": "audit-task-abc123",
  "run_id": "xyz789",
  "status": "completed",
  "result": {
    "success": true,
    "summary": "Repository audit completed",
    "details": {
      "golangci_yml": "present",
      "pre_commit_config": "present with required hooks",
      "makefile_test": "present",
      "agent_docs": "claude.md found",
      "claude_settings": "not found"
    },
    "pr_url": null
  },
  "error": null
}
```

### 4. Monitor in Temporal UI

1. Open http://localhost:8080
2. Click on "Workflows"
3. Find your workflow by ID
4. Click to see detailed execution history

## Common Tasks

### Check Running Workflows

```bash
curl http://localhost:8000/api/v1/tasks
```

### Cancel a Running Task

```bash
curl -X POST http://localhost:8000/api/v1/task/{workflow_id}/cancel
```

### Send Signal to Workflow

```bash
curl -X POST http://localhost:8000/api/v1/task/{workflow_id}/signal \
  -H "Content-Type: application/json" \
  -d '{
    "signal_name": "ci_completed",
    "signal_args": {"status": "passed"}
  }'
```

## Example Use Cases

### 1. Repository Audit (ROSAENG-59414)

Audit a repository for agentic SDLC readiness:

```json
{
  "repo_url": "https://github.com/your-org/your-repo",
  "task_description": "Comprehensive audit for agentic SDLC enablement",
  "checklist": [
    "Check linting configuration",
    "Check pre-commit hooks",
    "Verify test infrastructure",
    "Check agent documentation",
    "Verify CI/CD integration"
  ]
}
```

### 2. Automated Boilerplate Update (ROSAENG-59415)

Create a PR to update boilerplate:

```json
{
  "repo_url": "https://github.com/your-org/your-operator",
  "task_description": "Update boilerplate files to latest version",
  "context": {
    "boilerplate_repo": "https://github.com/openshift/boilerplate",
    "boilerplate_version": "v1.2.3",
    "create_pr": true,
    "wait_for_ci": true
  }
}
```

### 3. CI Failure Analysis

Analyze and fix CI failures:

```json
{
  "repo_url": "https://github.com/your-org/your-repo",
  "task_description": "Analyze CI failure and propose fixes",
  "context": {
    "pr_url": "https://github.com/your-org/your-repo/pull/123",
    "ci_logs": "link-to-logs"
  }
}
```

## Troubleshooting

### Worker Not Starting

**Error:** `Failed to connect to Temporal server`

**Solution:**
1. Check Temporal is running: `docker-compose ps`
2. Check logs: `docker-compose logs temporal`
3. Restart: `docker-compose restart temporal`

### API Returns 503

**Error:** `{"detail": "Temporal client not available"}`

**Solution:**
1. Ensure worker is running
2. Check Temporal server status
3. Verify `TEMPORAL_HOST` in `.env`

### Task Stays in "Running" State

**Possible causes:**
1. Worker crashed - check worker logs
2. Activity timeout - check Temporal UI for details
3. Claude API rate limit - wait and retry

**Debug:**
1. Check worker terminal for errors
2. Open Temporal UI and find the workflow
3. Look at activity execution history
4. Check for error messages

### GitHub Authentication Errors

**Error:** `Bad credentials` or `401 Unauthorized`

**Solution:**
1. Verify `GITHUB_TOKEN` in `.env`
2. Ensure token has `repo` scope
3. Test token: `curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user`

### Claude API Errors

**Error:** `401 Authentication Error`

**Solution:**
1. Verify `ANTHROPIC_API_KEY` in `.env`
2. Check API key is valid at https://console.anthropic.com
3. Ensure sufficient API credits

## Next Steps

1. **Implement Agent Runtime** - Complete Claude SDK integration in `src/agent/`
2. **Add Tests** - Write tests for workflows and activities
3. **Customize Prompts** - Adjust system prompts for your use cases
4. **Add More Tools** - Extend agent capabilities with custom tools
5. **Deploy** - See DEPLOYMENT.md for production setup

## Resources

- [Temporal Python SDK Docs](https://docs.temporal.io/dev-guide/python)
- [Anthropic API Docs](https://docs.anthropic.com)
- [FastAPI Docs](https://fastapi.tiangolo.com)
- [ROSAENG-59415](https://redhat.atlassian.net/browse/ROSAENG-59415)
- [ROSAENG-59414](https://redhat.atlassian.net/browse/ROSAENG-59414)
