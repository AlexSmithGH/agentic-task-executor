# Agentic Task Executor - Quick Reference

## 🚀 Starting the System

### 1. Start Infrastructure (Docker)
```bash
cd ~/git/alexasmi/agentic-task-executor
docker-compose up -d
```

This starts:
- ✅ PostgreSQL (port 5432)
- ✅ Temporal Server (port 7233)
- ✅ Temporal UI (port 8080) - **http://localhost:8080**

### 2. Start Worker (Terminal 1)
```bash
source venv/bin/activate
python -m src.worker
```

### 3. Start API (Terminal 2)
```bash
source venv/bin/activate
uvicorn src.api:app --reload
```

## 📡 Using the System

### Send a Task
```bash
curl -X POST http://localhost:8000/api/v1/execute-task \
  -H "Content-Type: application/json" \
  -d '{
    "repo_url": "https://github.com/openshift/managed-cluster-validating-webhooks",
    "task_description": "Audit this repository for agentic SDLC readiness",
    "checklist": [
      "Check for .golangci.yml",
      "Check for .pre-commit-config.yaml",
      "Check for Makefile with test target"
    ]
  }'
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
curl http://localhost:8000/api/v1/task/<workflow_id>/status | jq
```

### Monitor via Temporal UI
Open in browser: **http://localhost:8080**

- Click "Workflows" to see all executions
- Click a workflow ID to see detailed execution history
- View activity timeline, inputs, outputs, and errors

## 🔍 Monitoring

### API Health
```bash
curl http://localhost:8000/health
```

### Worker Logs
Watch Terminal 1 for activity execution logs

### Temporal Server Status
```bash
docker-compose ps
```

### View Temporal Logs
```bash
docker logs agentic-task-executor-temporal-1
```

## 🛑 Stopping the System

### Stop Worker/API
Press `Ctrl+C` in each terminal

### Stop Infrastructure
```bash
docker-compose down
```

### Stop Everything (including volumes)
```bash
docker-compose down -v
```

## 🐛 Troubleshooting

### Worker won't start
- Check if Temporal is running: `docker-compose ps`
- Check connectivity: `curl http://localhost:7233`
- Restart Temporal: `docker-compose restart temporal`

### API returns 503
- Ensure worker is running
- Check Temporal server: `docker-compose logs temporal`

### Task fails immediately
- Check worker logs for errors
- View in Temporal UI: http://localhost:8080
- Check task status via API

### Temporal UI not loading
- Verify container is running: `docker ps | grep temporal-ui`
- Check logs: `docker logs agentic-task-executor-temporal-ui-1`
- Restart: `docker-compose restart temporal-ui`

## 📍 Key URLs

- **API Server**: http://localhost:8000
- **API Docs**: http://localhost:8000/docs
- **Temporal UI**: http://localhost:8080
- **Temporal gRPC**: localhost:7233
- **PostgreSQL**: localhost:5432

## 📝 Configuration

- Environment: `.env`
- Docker setup: `docker-compose.yml`
- Python deps: `requirements.txt`
- Worker config: `src/config.py`

## 🔑 Environment Variables

Required in `.env`:
- `GCP_PROJECT_ID` - Google Cloud project for Vertex AI
- `GCP_REGION` - Region for Vertex AI (default: us-east5)
- `GITHUB_TOKEN` - GitHub personal access token
- `TEMPORAL_HOST` - Temporal server address (default: localhost:7233)

## ⚡ Quick Test

Complete end-to-end test:
```bash
# 1. Start everything
docker-compose up -d
source venv/bin/activate
python -m src.worker &
WORKER_PID=$!
uvicorn src.api:app &
API_PID=$!

# 2. Wait for startup
sleep 5

# 3. Send test task
curl -X POST http://localhost:8000/api/v1/execute-task \
  -H "Content-Type: application/json" \
  -d '{"repo_url": "https://github.com/openshift/managed-cluster-validating-webhooks", "task_description": "Test audit"}' \
  > /tmp/task.json

# 4. Get workflow ID and check status
WORKFLOW_ID=$(jq -r '.workflow_id' /tmp/task.json)
sleep 3
curl http://localhost:8000/api/v1/task/$WORKFLOW_ID/status | jq

# 5. Open Temporal UI
open http://localhost:8080

# 6. Cleanup
kill $WORKER_PID $API_PID
```

## 📚 Next Steps

1. ✅ System is running
2. ⏭️ Implement Claude SDK integration in `src/activities/agent_runtime.py`
3. ⏭️ Test with real repository audits
4. ⏭️ Add PR creation workflow
5. ⏭️ Integrate with CI monitoring

## 🆘 Getting Help

- Check logs in worker/API terminals
- View execution in Temporal UI
- Review `ARCHITECTURE.md` for design details
- See `GETTING_STARTED.md` for detailed setup
