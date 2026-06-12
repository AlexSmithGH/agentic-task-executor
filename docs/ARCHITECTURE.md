# Agentic Task Executor - Architecture

## Overview

This service provides a general-purpose agentic task execution platform for repository automation using Claude AI and Temporal workflows.

**Built for:** ROSAENG-59415 (SRE Automation Pattern) and ROSAENG-59414 (Quality Gates Tooling)

## Architecture Layers

```
┌─────────────────────────────────────────────────┐
│ API Layer (Go / Chi)                           │
│ - REST endpoints for task execution             │
│ - Status queries and workflow control           │
│ - Signal handling for external events           │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ Orchestration Layer (Temporal)                  │
│ - Durable workflow execution                    │
│ - Long-running task management                  │
│ - State persistence across restarts             │
│ - External event handling (webhooks)            │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ Activities Layer                                │
│ ├─ Git Operations (clone, branch, commit, push)│
│ ├─ Agent Runtime (Claude reasoning loop)       │
│ └─ GitHub Operations (PRs, CI status, reviews) │
└─────────────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│ Agent Runtime Layer                             │
│ ├─ Claude Client (multi-turn reasoning)        │
│ ├─ Tool Definitions (file ops, commands)       │
│ └─ Prompt Templates (task-specific prompts)    │
└─────────────────────────────────────────────────┘
```

## Component Details

### API Layer (`internal/api/`)

Built with [Chi](https://github.com/go-chi/chi) router with CORS middleware. The `Handler` struct holds a Temporal client and config, connecting eagerly at startup (fail-fast).

**Endpoints:**
- `POST /api/v1/execute-task` - Start a new agentic task
- `GET /api/v1/task/{workflowID}/status` - Query task status
- `POST /api/v1/task/{workflowID}/signal` - Send signals to running tasks
- `POST /api/v1/task/{workflowID}/cancel` - Cancel a running task
- `GET /api/v1/tasks` - List recent tasks
- `GET /health` - Health check
- `GET /` - Service info

**Key files:**
- `handler.go` - Route handler methods
- `models.go` - Request/response structs (`TaskParams`, `TaskResponse`, `TaskStatus`)
- `server.go` - Router setup, CORS, middleware

### Orchestration Layer (Temporal)

**Workflow:** `AgenticTaskWorkflow` (`internal/workflows/task_workflow.go`)

Go's Temporal SDK uses function-based workflows (not class-based like Python). State is held in local variables within the workflow function scope.

**Workflow Steps:**
1. Clone repository to workspace
2. Execute agent reasoning loop
3. Create pull request (if changes made)
4. Wait for CI results (optional)

**Features:**
- Query handler via `workflow.SetQueryHandler` for status checks
- Signal channel via `workflow.GetSignalChannel` for cancellation
- Per-activity retry policies and timeouts
- Activity options: `StartToCloseTimeout`, `HeartbeatTimeout`, `RetryPolicy`

**Input/Output types:** `WorkflowInput`, `WorkflowResult`, `CloneResult`, `AgentResult`, `PRResult`

### Activities Layer (`internal/activities/`)

Activities use **struct-based registration** for dependency injection. Each struct holds its dependencies (config, GitHub client, tokens) and all exported methods become Temporal activities.

**Git Operations (`git_operations.go`) — `GitActivities` struct:**
- `CloneRepository` - Clone repos using go-git with HTTP token auth
- `CreateBranch` - Create and checkout new branches
- `CommitChanges` - Stage and commit all changes
- `PushChanges` - Push commits to remote

**Agent Runtime (`agent_runtime.go`) — `AgentActivities` struct:**
- `AgentReasoningStep` - Execute Claude reasoning loop with tools
- `ToolExecutor` - Workspace-sandboxed tool execution (read files, run commands, grep)

**GitHub Operations (`github_operations.go`) — `GitHubActivities` struct:**
- `CreatePullRequest` - Create PRs via go-github
- `GetCIStatus` - Check CI/CD pipeline status (combined commit status)
- `GetReviewComments` - Fetch inline, issue, and review comments

### Agent Runtime Layer (`internal/agent/`)

**Claude Client (`client.go`):**
- Uses `anthropic-sdk-go` with `vertex.WithGoogleAuth` for Vertex AI
- `CreateMessage` - Single message with optional system prompt and tools
- `RunAgentLoop` - Multi-turn reasoning loop: send messages → check for tool_use blocks → execute tools → append results → repeat until no more tool calls or max iterations

**Tools (`tools.go`):**
- `read_file` - Read file contents (workspace-sandboxed)
- `list_files` - List directory contents
- `execute_command` - Execute bash commands (30s timeout)
- `search_files` - grep-based pattern search

**Prompts (`prompts.go`):**
- `AuditRepositoryPrompt` - Repository analysis tasks
- `CreatePRPrompt` - PR creation tasks
- `AnalyzeCIFailurePrompt` - CI failure debugging
- `BuildAgentSystemPrompt` / `BuildInitialPrompt` - Dynamic prompt construction

## Data Flow

### Typical Task Execution Flow

1. **API Request** → `POST /api/v1/execute-task` with repo URL, description, checklist
2. **Workflow Start** → Temporal workflow created with unique ID (`task-{uuid}`)
3. **Clone** → `CloneRepository` activity clones repo to `/tmp/agentic-workspaces/{workflow_id}`
4. **Agent Reasoning** → `AgentReasoningStep` runs Claude with tools against the workspace
5. **PR Creation** → If agent made changes, `CreatePullRequest` creates a GitHub PR
6. **CI Wait** → Optionally polls for CI completion
7. **Result** → Workflow completes with `WorkflowResult` queryable via status endpoint

## State Management

### Temporal State Persistence
- **Workflow State**: Persisted in PostgreSQL by Temporal
- **Activity Results**: Stored in workflow history
- **Signals**: Queued in Temporal and delivered to workflows
- **Workspace Files**: Ephemeral, cleaned up after completion

### Agent Conversation State
- Maintained in memory during the `AgentReasoningStep` activity
- Full message history passed to each Claude API call
- Tool results appended as user messages between turns

## Deployment Architecture

### Local Development
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  API Server │────▶│  Temporal   │────▶│  Worker     │
│  (Go, :8000)│     │  Server     │     │  (Go)       │
└─────────────┘     │  (Docker)   │     └─────────────┘
                    └─────────────┘
                            │
                            ▼
                    ┌─────────────┐
                    │ PostgreSQL  │
                    │   (Docker)  │
                    └─────────────┘
```

### Production (Future)
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  API (K8s)  │────▶│  Temporal   │────▶│  Workers    │
│             │     │  Cloud      │     │  (K8s)      │
└─────────────┘     └─────────────┘     └─────────────┘
```

## Design Decisions

### Why Temporal?
1. **Long-running workflows** - PR workflows can wait hours/days for CI and reviews
2. **Durability** - Service restarts don't lose workflow state
3. **External events** - Built-in signal handling for webhooks
4. **Observability** - Temporal UI shows workflow execution history
5. **Go SDK** - Temporal's original and most mature SDK

### Why Go?
1. **Temporal's native language** - Most mature SDK, best documentation
2. **Single binary deployment** - No runtime dependencies
3. **Strong typing** - Compile-time safety for activity/workflow contracts
4. **Concurrency** - Natural fit for I/O-bound orchestration work

### Why Claude via Vertex AI?
1. **GCP integration** - Uses Application Default Credentials
2. **Enterprise compliance** - Data stays within GCP
3. **Official SDK** - `anthropic-sdk-go` with `vertex.WithGoogleAuth`

## Security Considerations

### Credentials Management
- GitHub token and GCP credentials loaded from environment variables
- `.env` file excluded from git via `.gitignore`

### Workspace Isolation
- Each task gets isolated workspace directory under `/tmp/agentic-workspaces/`
- File operations validated to stay within workspace via `filepath.Abs` + `strings.HasPrefix`
- Command execution sandboxed to workspace directory with 30s timeout

## Dependencies

| Concern | Go Module |
|---------|-----------|
| HTTP router | `github.com/go-chi/chi/v5` |
| Temporal SDK | `go.temporal.io/sdk` |
| Claude API | `github.com/anthropics/anthropic-sdk-go` |
| GitHub API | `github.com/google/go-github/v68` |
| Git operations | `github.com/go-git/go-git/v5` |
| Config | `github.com/caarlos0/env/v11` + `github.com/joho/godotenv` |
| Logging | `log/slog` (stdlib) |

## Future Enhancements

### Near-term
1. Add webhook receiver for GitHub CI events
2. Implement human-in-the-loop approval steps
3. Add comprehensive test suite
4. Production Kubernetes manifests

### Long-term
1. Multi-agent workflows (parallel analysis)
2. Learned preferences from feedback
3. Integration with monitoring systems (osde2e)
4. Container-based isolation for workspaces
