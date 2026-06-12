# Agentic Task Executor - Quick Reference

## Starting the System

### 1. Start Infrastructure (Docker)
```bash
make docker-up
```

This starts:
- PostgreSQL (port 5432)
- Temporal Server (port 7233)
- Temporal UI (port 8080) — http://localhost:8080

### 2. Start Worker (Terminal 1)
```bash
make run-worker
```

### 3. Start API (Terminal 2)
```bash
make run-api
```

## Using the System

### Send a Task
```bash
curl -s -X POST http://localhost:8000/api/v1/execute-task \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/openshift/managed-cluster-validating-webhooks",
    "task_description": "Audit this repository for agentic SDLC readiness",
    "checklist": [
      "Check for .golangci.yml",
      "Check for .pre-commit-config.yaml",
      "Check for Makefile with test target"
    ]
  }' | jq .
```

Returns:
```json
{
  "workflow_id": "task-abc123...",
  "run_id": "xyz789...",
  "status": "running"
}
```

### Check Task Status
```bash
curl -s http://localhost:8000/api/v1/task/<workflow_id>/status | jq .
```

### Monitor via Temporal UI
Open in browser: http://localhost:8080

## Monitoring

### API Health
```bash
curl -s http://localhost:8000/health | jq .
```

### Worker Logs
Watch Terminal 1 for activity execution logs

### Temporal Server Status
```bash
docker-compose ps
```

### View Temporal Logs
```bash
docker-compose logs -f temporal
```

## Stopping the System

### Stop Worker/API
Press `Ctrl+C` in each terminal

### Stop Infrastructure
```bash
make docker-down
```

### Stop Everything (including volumes)
```bash
docker-compose down -v
```

## Troubleshooting

### Worker won't start
- Check if Temporal is running: `docker-compose ps`
- Restart Temporal: `docker-compose restart temporal`

### API returns errors
- Ensure Temporal server is running
- Check worker is running and registered

### Task fails immediately
- Check worker logs for errors
- View in Temporal UI: http://localhost:8080

## Key URLs

- **API Server**: http://localhost:8000
- **Health Check**: http://localhost:8000/health
- **Temporal UI**: http://localhost:8080
- **Temporal gRPC**: localhost:7233
- **PostgreSQL**: localhost:5432

## Configuration

All settings via environment variables (`.env` file):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GCP_PROJECT_ID` | Yes | — | Google Cloud project for Vertex AI |
| `GCP_REGION` | No | `us-east5` | Region for Vertex AI |
| `GITHUB_TOKEN` | Yes | — | GitHub personal access token |
| `TEMPORAL_HOST` | No | `localhost:7233` | Temporal server address |
| `TEMPORAL_NAMESPACE` | No | `default` | Temporal namespace |
| `TEMPORAL_TASK_QUEUE` | No | `agentic-tasks` | Temporal task queue |
| `API_HOST` | No | `0.0.0.0` | API listen address |
| `API_PORT` | No | `8000` | API listen port |
| `LOG_LEVEL` | No | `INFO` | Log level |
| `WORKSPACE_DIR` | No | `/tmp/agentic-workspaces` | Workspace directory |

## Build Commands

```bash
make build        # Build both binaries
make run-api      # Build and run API server
make run-worker   # Build and run Temporal worker
make test         # Run tests
make lint         # Run golangci-lint
make clean        # Remove build artifacts
make docker-up    # Start Temporal infrastructure
make docker-down  # Stop Temporal infrastructure
```
