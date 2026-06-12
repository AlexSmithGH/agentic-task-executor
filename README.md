# Agentic Task Executor

AI-powered task execution service for repository automation and analysis.

## Overview

This service provides an API for executing AI-assisted tasks against GitHub repositories using Claude (via Vertex AI) and Temporal for workflow orchestration.

**Built for:** ROSAENG-59415 (SRE Automation Pattern - AI-Assisted Operational Workflows)

## Architecture

- **API Layer:** Go HTTP server (Chi router)
- **Orchestration:** Temporal workflows for long-running, durable execution
- **Agent Runtime:** Anthropic Go SDK with custom tool definitions
- **State Management:** Temporal server with PostgreSQL

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

- Go 1.23+
- Docker and Docker Compose (for Temporal server)
- GitHub access token
- Google Cloud credentials (for Vertex AI)

### Local Development Setup

1. **Start Temporal server:**
   ```bash
   docker-compose up -d
   ```

2. **Configure environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```

3. **Start the Temporal worker:**
   ```bash
   make run-worker
   ```

4. **Start the API server (separate terminal):**
   ```bash
   make run-api
   ```

5. **Access services:**
   - API: http://localhost:8000
   - Health: http://localhost:8000/health
   - Temporal UI: http://localhost:8080

## API Usage

### Execute a Task

```bash
curl -s -X POST http://localhost:8000/api/v1/execute-task \
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
  }' | jq .
```

### Check Task Status

```bash
curl -s http://localhost:8000/api/v1/task/{workflow_id}/status | jq .
```

## Project Structure

```
agentic-task-executor/
├── cmd/
│   ├── api/main.go              # API server entrypoint
│   └── worker/main.go           # Temporal worker entrypoint
├── internal/
│   ├── config/config.go         # Configuration (env loading)
│   ├── api/                     # HTTP handlers and router
│   │   ├── handler.go           # Route handlers
│   │   ├── models.go            # Request/response structs
│   │   └── server.go            # Server setup and middleware
│   ├── workflows/
│   │   └── task_workflow.go     # Temporal workflow definition
│   ├── activities/              # Temporal activity implementations
│   │   ├── git_operations.go    # Clone, branch, commit, push
│   │   ├── agent_runtime.go     # Claude agent reasoning loop
│   │   └── github_operations.go # PR creation, CI status
│   └── agent/                   # Claude AI integration
│       ├── client.go            # Anthropic SDK wrapper (Vertex AI)
│       ├── tools.go             # Tool definitions for Claude
│       └── prompts.go           # System prompt templates
├── docker-compose.yml           # Temporal server setup
├── Dockerfile                   # Multi-stage container build
├── Makefile                     # Build and dev commands
├── go.mod / go.sum
└── .env.example
```

## Development

### Building

```bash
make build
```

### Running Tests

```bash
make test
```

### Temporal UI

Monitor running workflows at http://localhost:8080

### Debugging

Temporal provides time-travel debugging - you can replay workflows with the exact same inputs and state.

## References

- [ROSAENG-59415](https://redhat.atlassian.net/browse/ROSAENG-59415) - SRE Automation Pattern
- [ROSAENG-59414](https://redhat.atlassian.net/browse/ROSAENG-59414) - Quality Gates Tooling
- [Temporal Go SDK Docs](https://docs.temporal.io/dev-guide/go)
- [Anthropic Go SDK](https://github.com/anthropics/anthropic-sdk-go)
