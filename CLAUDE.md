# AI Agent Development Guide

This document provides guidance for AI agents (like Claude) working with the Agentic Task Executor codebase.

## Project Context

**Name:** Agentic Task Executor  
**Type:** Go microservice with Temporal workflow orchestration  
**Purpose:** AI-powered task execution service for repository automation and analysis  
**Language:** Go 1.23+  
**Key Frameworks:** Temporal, Chi router, Anthropic SDK (Claude AI)

## Quick Architecture Overview

```
API Layer (Chi HTTP) 
  ↓
Temporal Workflows (Orchestration)
  ↓
Activities (Git, Agent Runtime, GitHub)
  ↓
Claude AI Agent (Tool-based reasoning)
```

### Key Components
- **API Server** (`cmd/api/`): HTTP REST API for task execution
- **Worker** (`cmd/worker/`): Temporal worker executing workflows and activities
- **Workflows** (`internal/workflows/`): Durable task orchestration
- **Activities** (`internal/activities/`): Git ops, agent runtime, GitHub integration
- **Agent** (`internal/agent/`): Claude AI client with tool definitions

## Code Structure

```
agentic-task-executor/
├── cmd/
│   ├── api/          # API server entrypoint
│   └── worker/       # Temporal worker entrypoint
├── internal/
│   ├── api/          # HTTP handlers, models, server setup
│   ├── workflows/    # Temporal workflow definitions
│   ├── activities/   # Temporal activity implementations
│   ├── agent/        # Claude AI integration
│   └── config/       # Configuration loading
├── docs/             # Documentation
├── .github/          # CI/CD workflows
└── [config files]    # .golangci.yml, .pre-commit-config.yaml, etc.
```

## Development Workflow

### Building and Testing
```bash
make build    # Build API and worker binaries
make test     # Run tests
make lint     # Run golangci-lint
make clean    # Clean build artifacts
```

### Running Locally
```bash
# 1. Start Temporal server
make docker-up

# 2. Start worker (terminal 1)
make run-worker

# 3. Start API server (terminal 2)
make run-api

# 4. Temporal UI: http://localhost:8080
# 5. API: http://localhost:8000
```

### Making Changes

**Before editing code:**
1. Ensure you understand the component's role (see Architecture docs)
2. Check existing patterns in similar files
3. Verify dependencies are available

**After editing code:**
1. Run `make lint` to check code quality
2. Run `make test` to ensure tests pass
3. Run `make build` to verify compilation
4. Test manually if adding new features

**Pre-commit checks:**
- Linting with golangci-lint (see `.golangci.yml`)
- Format with gofmt/goimports
- Secret detection with gitleaks
- Tests must pass

## Code Conventions

### Go Style
- Follow standard Go conventions (effective Go, Go proverbs)
- Use `gofmt` and `goimports` for formatting
- Error handling: always check errors, wrap with context
- Naming: camelCase for private, PascalCase for exported

### Temporal Patterns

**Workflows** (`internal/workflows/`):
- Must be deterministic (no random, time.Now, etc.)
- Use `workflow.Now()` instead of `time.Now()`
- Use `workflow.Sleep()` instead of `time.Sleep()`
- All I/O must go through activities
- Input/output types must be serializable

**Activities** (`internal/activities/`):
- Can perform I/O, call external APIs
- Should be idempotent when possible
- Use heartbeats for long-running operations
- Return detailed errors for debugging
- Struct-based registration for dependency injection

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to clone repository: %w", err)
}

// Activity errors should be descriptive
if err := activity.RecordHeartbeat(ctx, progress); err != nil {
    return nil, fmt.Errorf("heartbeat failed: %w", err)
}
```

### Logging
- Use `log/slog` (stdlib structured logging)
- Log levels: Debug, Info, Warn, Error
- Include relevant context (workflow ID, activity name, etc.)

## Key Dependencies

| Package | Purpose | Import Path |
|---------|---------|-------------|
| Chi | HTTP routing | `github.com/go-chi/chi/v5` |
| Temporal SDK | Workflow orchestration | `go.temporal.io/sdk` |
| Anthropic SDK | Claude API | `github.com/anthropics/anthropic-sdk-go` |
| go-github | GitHub API | `github.com/google/go-github/v68` |
| go-git | Git operations | `github.com/go-git/go-git/v5` |
| env | Config loading | `github.com/caarlos0/env/v11` |

## Testing Guidelines

### Unit Tests
- Test file naming: `*_test.go`
- Table-driven tests preferred
- Mock external dependencies (Temporal, GitHub, Claude)
- Use `t.Parallel()` for independent tests

### Integration Tests
- Tag with `// +build integration`
- Require running Temporal server
- Clean up resources in `t.Cleanup()`

### Current State
⚠️ **Note:** Test suite is currently minimal. When adding tests:
1. Start with unit tests for business logic
2. Add integration tests for workflows
3. Mock external services (GitHub, Claude API)

## Environment Configuration

See `.env.example` for all environment variables. Required:
- `GCP_PROJECT_ID`, `GCP_REGION` - Vertex AI access
- `GITHUB_TOKEN` - GitHub API authentication
- `TEMPORAL_HOST`, `TEMPORAL_NAMESPACE`, `TEMPORAL_TASK_QUEUE` - Temporal connection
- `WORKSPACE_DIR` - Location for cloned repositories

## Common Operations

### Adding a New Activity
1. Create method on appropriate activities struct (`internal/activities/`)
2. Method signature: `func (a *MyActivities) MyActivity(ctx context.Context, input MyInput) (MyOutput, error)`
3. Register in worker (`cmd/worker/main.go`)
4. Call from workflow using `workflow.ExecuteActivity()`

### Adding a New Workflow
1. Create workflow function in `internal/workflows/`
2. Signature: `func MyWorkflow(ctx workflow.Context, input MyInput) (MyOutput, error)`
3. Register in worker (`cmd/worker/main.go`)
4. Start from API handler using `client.ExecuteWorkflow()`

### Adding a New API Endpoint
1. Define request/response models in `internal/api/models.go`
2. Add handler method in `internal/api/handler.go`
3. Register route in `internal/api/server.go`

### Adding a New Tool for Claude
1. Define tool schema in `internal/agent/tools.go`
2. Implement executor logic in tool executor
3. Add tool to agent client tool list
4. Update prompts if needed

## Debugging

### Temporal UI
- View running/completed workflows: http://localhost:8080
- Inspect workflow history, inputs, outputs
- See activity retry attempts and errors
- Time-travel debugging capabilities

### Logs
- API server logs to stdout (structured JSON in production)
- Worker logs to stdout
- Temporal server logs via `docker-compose logs temporal`

### Common Issues
1. **Workflow non-determinism**: Ensure workflows use `workflow.*` functions
2. **Activity timeouts**: Increase timeout or add heartbeats
3. **Authentication failures**: Check env vars and credentials
4. **Workspace permissions**: Ensure `WORKSPACE_DIR` is writable

## Security Considerations

### Workspace Isolation
- All file operations validated to stay within workspace
- Commands executed with timeout (30s default)
- No shell expansion in commands

### Credentials
- Never commit `.env` file (in `.gitignore`)
- Use environment variables for all secrets
- GCP uses Application Default Credentials

### Code Review Checklist
- [ ] No hardcoded secrets or tokens
- [ ] Error messages don't leak sensitive data
- [ ] File paths validated against workspace
- [ ] External inputs sanitized
- [ ] Timeouts set on external calls

## AI-Specific Guidance

### When Analyzing Code
1. Start with `README.md` and `docs/ARCHITECTURE.md`
2. Understand the workflow before activities
3. Check error handling patterns
4. Look for similar existing code

### When Making Changes
1. Preserve existing patterns and conventions
2. Update related documentation
3. Consider backward compatibility
4. Think about failure modes

### When Adding Features
1. Check if workflow or activity is appropriate
2. Consider durability and retry semantics
3. Add proper error handling
4. Update API models if needed

### Tools Available to You
- `read_file` - Read any file in the repository
- `list_files` - List directory contents
- `execute_command` - Run commands (make, go, git, etc.)
- `search_files` - Search for patterns in code

## Documentation

### Existing Documentation
- **README.md** - Quick start and overview
- **docs/ARCHITECTURE.md** - Detailed architecture and design decisions
- **docs/GETTING_STARTED.md** - Step-by-step setup guide
- **docs/QUICK_REFERENCE.md** - Command reference and env vars
- **docs/PROJECT_STATUS.md** - Current status and roadmap

### When to Update Docs
- New features or API changes → Update README.md
- Architecture changes → Update ARCHITECTURE.md
- New env vars → Update QUICK_REFERENCE.md and .env.example
- Breaking changes → Add migration notes

## Related Resources

- [Temporal Go SDK Docs](https://docs.temporal.io/dev-guide/go)
- [Anthropic API Reference](https://docs.anthropic.com/)
- [Chi Router Docs](https://go-chi.io/)
- [Effective Go](https://golang.org/doc/effective_go)

## JIRA Tickets
- **ROSAENG-59415** - SRE Automation Pattern (main ticket)
- **ROSAENG-59414** - Quality Gates Tooling (related)

## Getting Help

1. Check existing documentation in `docs/`
2. Review Temporal UI for workflow issues
3. Check logs for error details
4. Review similar code in the repository
5. Consult Go and Temporal documentation

---

**Last Updated:** 2024  
**Maintained by:** SRE Team
