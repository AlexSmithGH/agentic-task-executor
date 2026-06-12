# Getting Started with Agentic Task Executor

This guide walks you through setting up and running your first agentic task.

## Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose
- GitHub personal access token
- Google Cloud credentials (for Vertex AI / Claude)

## Installation

### 1. Clone and Navigate

```bash
cd agentic-task-executor
```

### 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and set your credentials:

```bash
GCP_PROJECT_ID=your-gcp-project
GCP_REGION=us-east5
GITHUB_TOKEN=ghp_...
```

### 3. Start Temporal Server

```bash
make docker-up
```

Verify it's running:
- Temporal UI: http://localhost:8080
- PostgreSQL: localhost:5432

### 4. Build

```bash
make build
```

## Running the Service

You need to run two processes:

### Terminal 1: Start the Worker

```bash
make run-worker
```

You should see:
```
INFO Starting Temporal worker host=localhost:7233 namespace=default task_queue=agentic-tasks
INFO Worker configured workflows=["AgenticTaskWorkflow"] task_queue=agentic-tasks
```

### Terminal 2: Start the API Server

```bash
make run-api
```

You should see:
```
INFO Connecting to Temporal host=localhost:7233 namespace=default
INFO API server starting addr=0.0.0.0:8000
```

## Your First Task

### 1. Check API Health

```bash
curl -s http://localhost:8000/health | jq .
```

Expected response:
```json
{"status": "healthy"}
```

### 2. Execute a Repository Audit Task

```bash
curl -s -X POST http://localhost:8000/api/v1/execute-task \
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
  }' | jq .
```

Expected response:
```json
{
  "workflow_id": "task-abc123...",
  "run_id": "xyz789...",
  "status": "running"
}
```

### 3. Check Task Status

Using the `workflow_id` from the response:

```bash
curl -s http://localhost:8000/api/v1/task/task-abc123.../status | jq .
```

Response while running:
```json
{
  "workflow_id": "task-abc123...",
  "run_id": "xyz789...",
  "status": "running"
}
```

Response when complete:
```json
{
  "workflow_id": "task-abc123...",
  "run_id": "xyz789...",
  "status": "completed",
  "result": {
    "success": true,
    "summary": "Repository audit completed",
    "details": { ... }
  }
}
```

### 4. Monitor in Temporal UI

1. Open http://localhost:8080
2. Click on "Workflows"
3. Find your workflow by ID
4. Click to see detailed execution history

## Common Operations

### Check Running Workflows

```bash
curl -s http://localhost:8000/api/v1/tasks | jq .
```

### Cancel a Running Task

```bash
curl -s -X POST http://localhost:8000/api/v1/task/{workflow_id}/cancel | jq .
```

### Send Signal to Workflow

```bash
curl -s -X POST http://localhost:8000/api/v1/task/{workflow_id}/signal \
  -H "Content-Type: application/json" \
  -d '{
    "signal_name": "ci_completed",
    "signal_args": {"status": "passed"}
  }' | jq .
```

## Example Use Cases

### 1. Repository Audit (ROSAENG-59414)

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

**Error:** `Failed to connect to Temporal`

**Solution:**
1. Check Temporal is running: `docker-compose ps`
2. Check logs: `docker-compose logs temporal`
3. Restart: `docker-compose restart temporal`

### API Returns 503

**Solution:**
1. Ensure Temporal server is running
2. Verify `TEMPORAL_HOST` in `.env`

### Task Stays in "Running" State

**Possible causes:**
1. Worker crashed - check worker terminal
2. Activity timeout - check Temporal UI for details
3. Claude API rate limit - wait and retry

**Debug:**
1. Check worker terminal for errors
2. Open Temporal UI and find the workflow
3. Look at activity execution history

### GitHub Authentication Errors

**Solution:**
1. Verify `GITHUB_TOKEN` in `.env`
2. Ensure token has `repo` scope
3. Test token: `curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user`

### Claude / Vertex AI Errors

**Solution:**
1. Verify `GCP_PROJECT_ID` and `GCP_REGION` in `.env`
2. Authenticate: `gcloud auth application-default login`
3. Ensure Vertex AI API is enabled in your GCP project

## Resources

- [Temporal Go SDK Docs](https://docs.temporal.io/dev-guide/go)
- [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go)
- [Chi Router](https://github.com/go-chi/chi)
- [go-git](https://github.com/go-git/go-git)
- [ROSAENG-59415](https://redhat.atlassian.net/browse/ROSAENG-59415)
- [ROSAENG-59414](https://redhat.atlassian.net/browse/ROSAENG-59414)
