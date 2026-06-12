# AI Agent Guidance - Agentic Task Executor

**Purpose:** This document provides context and guidance for AI assistants (like Claude) working on this codebase.

**Project:** Agentic Task Executor - AI-powered repository automation service  
**Primary Use Case:** SRE automation patterns and quality gates enforcement  
**Tech Stack:** Go, Temporal, Claude AI (via Vertex), GitHub API

---

## Project Overview

This is a **production service** that executes AI-assisted tasks against GitHub repositories using:
- **Claude AI** for intelligent reasoning and decision-making
- **Temporal** for durable, long-running workflow orchestration
- **Go** for type-safe, performant service implementation

The service clones repositories, analyzes them using Claude with custom tools, makes changes, and creates pull requests - all while handling failures gracefully through Temporal's retry mechanisms.

---

## Architecture Mental Model

Think of this as **three layers**:

```
API Layer (HTTP) → Temporal Workflows → Activities (Git, Agent, GitHub)
```

1. **API Layer** (`internal/api/`): HTTP endpoints that start Temporal workflows
2. **Workflows** (`internal/workflows/`): Durable orchestration logic (survives restarts)
3. **Activities** (`internal/activities/`): Individual units of work (clone, agent reasoning, PR creation)

**Critical concept**: Workflows are **deterministic** and **replayable**. Never use `time.Now()` or random values directly in workflows - use Temporal's workflow-safe alternatives.

---

## Code Conventions

### Go Style
- **Struct-based dependency injection** for activities (see `GitActivities`, `AgentActivities`)
- **Function-based workflows** (Temporal Go SDK pattern)
- **Exported methods** on activity structs automatically become Temporal activities
- **Error wrapping** with `fmt.Errorf` and `%w` for error chains
- **Context handling**: Always pass and respect `context.Context`

### Naming Patterns
- **Workflows**: `{Domain}Workflow` (e.g., `AgenticTaskWorkflow`)
- **Activities**: Verb-noun format (e.g., `CloneRepository`, `CreatePullRequest`)
- **Structs**: Descriptive names with `Activities` suffix for activity containers
- **Request/Response**: Suffix with `Input`, `Result`, `Params`, `Response`

### File Organization
```
cmd/           # Entrypoints (api/main.go, worker/main.go)
internal/      # Private application code
├── api/       # HTTP layer
├── workflows/ # Temporal workflows
├── activities/# Temporal activities
├── agent/     # Claude AI integration
└── config/    # Configuration loading
docs/          # Comprehensive documentation
```

---

## Common Tasks

### Adding a New Tool for Claude

1. **Define schema** in `internal/agent/tools.go`:
   ```go
   {
       Name:        "my_tool",
       Description: "What it does",
       InputSchema: map[string]any{...},
   }
   ```

2. **Implement handler** in `internal/activities/agent_runtime.go`:
   ```go
   case "my_tool":
       // Extract parameters
       // Execute operation
       // Return result as JSON
   ```

3. **Test** with a sample workflow execution

### Adding a New Activity

1. **Create method** on appropriate activities struct:
   ```go
   func (a *MyActivities) DoSomething(ctx context.Context, input MyInput) (MyResult, error) {
       // Implementation
   }
   ```

2. **Register** in worker (`cmd/worker/main.go`):
   ```go
   w.RegisterActivity(&activities.MyActivities{...})
   ```

3. **Call from workflow** with activity options:
   ```go
   var result MyResult
   err := workflow.ExecuteActivity(ctx, activities.DoSomething, input).Get(ctx, &result)
   ```

### Adding a New API Endpoint

1. **Define models** in `internal/api/models.go`:
   ```go
   type MyRequest struct { ... }
   type MyResponse struct { ... }
   ```

2. **Implement handler** in `internal/api/handler.go`:
   ```go
   func (h *Handler) MyHandler(w http.ResponseWriter, r *http.Request) {
       // Parse request
       // Start workflow or query
       // Return response
   }
   ```

3. **Register route** in `internal/api/server.go`:
   ```go
   r.Post("/api/v1/my-endpoint", h.MyHandler)
   ```

---

## Critical Patterns

### Temporal Workflow Patterns

**✅ DO:**
```go
// Use workflow.Now() for time
now := workflow.Now(ctx)

// Use workflow.Sleep() for delays
workflow.Sleep(ctx, 5*time.Minute)

// Use workflow.ExecuteActivity with options
ao := workflow.ActivityOptions{
    StartToCloseTimeout: 5 * time.Minute,
}
ctx = workflow.WithActivityOptions(ctx, ao)
```

**❌ DON'T:**
```go
// Never use stdlib time directly
now := time.Now()  // ❌ Non-deterministic!

// Never use random values
id := uuid.New()   // ❌ Non-deterministic!

// Never do I/O in workflows
http.Get(...)      // ❌ Move to activity!
```

### Error Handling

**Activity errors:**
```go
if err != nil {
    return MyResult{}, fmt.Errorf("failed to clone: %w", err)
}
```

**Workflow error handling:**
```go
err := workflow.ExecuteActivity(ctx, MyActivity, input).Get(ctx, &result)
if err != nil {
    return WorkflowResult{}, fmt.Errorf("activity failed: %w", err)
}
```

### Claude Agent Integration

**Multi-turn reasoning loop** (`internal/agent/client.go`):
```go
for iteration < maxIterations {
    message := CreateMessage(...)
    if !hasToolUse(message) {
        break  // Agent is done
    }
    toolResults := executeTools(message)
    messages = append(messages, toolResults...)
}
```

**Workspace sandboxing** (all tool paths validated):
```go
absPath, _ := filepath.Abs(path)
if !strings.HasPrefix(absPath, workspace) {
    return "", fmt.Errorf("access denied")
}
```

---

## Testing Strategy

### Current State
- ✅ Makefile has `make test` target
- ✅ Go test structure (`go test ./...`)
- ⚠️ **Limited test coverage** (needs expansion)

### When Writing Tests

**Unit tests:**
- Mock Temporal client for API tests
- Mock GitHub client for activity tests
- Mock Claude client for agent tests

**Integration tests:**
- Use Temporal test server (`testsuite` package)
- Test full workflows end-to-end
- Use real filesystem for workspace tests

**Example test structure:**
```go
func TestMyActivity(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestActivityEnvironment()
    
    // Register activity
    env.RegisterActivity(MyActivity)
    
    // Execute
    result, err := env.ExecuteActivity(MyActivity, input)
    
    // Assert
    require.NoError(t, err)
    var output MyOutput
    result.Get(&output)
    assert.Equal(t, expected, output)
}
```

---

## Environment Variables

**Required:**
- `GCP_PROJECT_ID` - Google Cloud project for Vertex AI
- `GCP_REGION` - Region for Vertex AI (e.g., `us-east5`)
- `GITHUB_TOKEN` - GitHub personal access token (repo scope)
- `TEMPORAL_HOST` - Temporal server address
- `TEMPORAL_TASK_QUEUE` - Task queue name

**Optional:**
- `GOOGLE_APPLICATION_CREDENTIALS` - Path to service account JSON (uses ADC if not set)
- `API_HOST` / `API_PORT` - API server binding
- `LOG_LEVEL` - Logging verbosity
- `WORKSPACE_DIR` - Base directory for cloned repos

See `.env.example` for complete list with defaults.

---

## Known Limitations & Gotchas

### 1. Temporal Determinism
**Issue:** Workflows must be deterministic for replay  
**Impact:** Can't use `time.Now()`, random values, or I/O directly  
**Solution:** Use `workflow.*` alternatives and move I/O to activities

### 2. Go Generics in Temporal
**Issue:** Temporal SDK doesn't work well with generic types  
**Impact:** Must define concrete types for all workflow inputs/outputs  
**Solution:** Avoid generics in workflow/activity signatures

### 3. Workspace Cleanup
**Issue:** Failed workflows may leave workspace directories behind  
**Impact:** Disk space can fill up over time  
**Solution:** Defer cleanup in activities (in progress)

### 4. GitHub Token Scope
**Issue:** Token must have `repo` scope for private repos  
**Impact:** PR creation fails if token lacks permissions  
**Solution:** Document token requirements clearly

### 5. Vertex AI Quota
**Issue:** Claude API calls count against GCP quota  
**Impact:** High-volume testing can hit rate limits  
**Solution:** Use mock clients for tests, monitor quotas

### 6. Long-Running Activities
**Issue:** Git clone can timeout on large repos  
**Impact:** Activity failures for repos >1GB  
**Solution:** Increase `StartToCloseTimeout` or use shallow clones

---

## Areas Requiring Human Oversight

### 1. Security
- **Credential handling**: Never log tokens or credentials
- **Workspace isolation**: Validate all file paths stay within workspace
- **Command execution**: Sanitize inputs, enforce timeouts

### 2. Claude Tool Execution
- **File modifications**: Agent can modify any file in workspace
- **Command execution**: Agent can run arbitrary bash commands
- **GitHub operations**: Agent can create PRs, branches

### 3. Production Deployment
- **Temporal server**: Ensure high availability and backups
- **GCP quotas**: Monitor Vertex AI usage
- **GitHub rate limits**: Respect API rate limits

---

## Decision Rationale

### Why Temporal Instead of Direct Orchestration?
**Reason:** Need durability for long-running workflows (PR + CI can take hours)  
**Tradeoff:** Additional complexity vs. built-in retry/durability/observability

### Why Go Instead of Python?
**Reason:** Temporal's Go SDK is most mature, strong typing for safety  
**Tradeoff:** Less ML ecosystem vs. better performance and type safety

### Why Vertex AI Instead of Direct Anthropic API?
**Reason:** Enterprise compliance, GCP integration, cost controls  
**Tradeoff:** Slightly higher latency vs. compliance and billing integration

### Why Struct-Based Activity Registration?
**Reason:** Enables constructor-based dependency injection  
**Tradeoff:** More verbose vs. explicit dependencies and testability

---

## Examples of Good PRs/Commits

### Good PR Structure
```
Title: Add timeout configuration for agent reasoning loop

Changes:
- Add MaxIterations config to AgentActivities
- Add timeout to execute_command tool (30s default)
- Update docs/ARCHITECTURE.md with timeout behavior

Tests:
- Add TestAgentTimeout integration test
- Verify timeout triggers after max iterations

Closes: ROSAENG-12345
```

### Good Commit Messages
```
✅ feat: add configurable timeout for Claude reasoning loop

Allows operators to limit max iterations and prevent runaway
agent execution. Defaults to 10 iterations with 30s per tool.

✅ fix: prevent workspace escape in read_file tool

Validates all paths resolve within workspace directory before
allowing file access.

✅ docs: update CLAUDE.md with testing patterns

Adds examples for mocking Temporal client and GitHub API
in unit tests.
```

---

## Quick Reference Commands

```bash
# Start development environment
make dev

# Run tests
make test

# Run linting
make lint

# Build binaries
make build

# Start just Temporal
make docker-up

# View Temporal UI
open http://localhost:8080

# Check API health
curl http://localhost:8000/health

# Execute a task
curl -X POST http://localhost:8000/api/v1/execute-task \
  -H "Content-Type: application/json" \
  -d @examples/audit-request.json
```

---

## Getting Help

1. **Architecture questions**: See `docs/ARCHITECTURE.md`
2. **Setup issues**: See `docs/GETTING_STARTED.md`
3. **API usage**: See `docs/QUICK_REFERENCE.md`
4. **Temporal patterns**: https://docs.temporal.io/dev-guide/go
5. **Claude API**: https://docs.anthropic.com/

---

## Version History

- **v0.1.0** (Current): Initial implementation with basic audit workflow
- Roadmap: See `docs/PROJECT_STATUS.md`

---

**Last Updated:** 2025-01-XX  
**Maintained By:** SRE Team (ROSAENG-59415)
