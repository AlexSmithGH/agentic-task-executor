# Agentic Task Executor - Architecture

## Overview

This service provides a general-purpose agentic task execution platform for repository automation using Claude AI and Temporal workflows.

**Built for:** ROSAENG-59415 (SRE Automation Pattern) and ROSAENG-59414 (Quality Gates Tooling)

## Architecture Layers

```
┌─────────────────────────────────────────────────┐
│ API Layer (FastAPI)                             │
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
│ ├─ Agent Runtime (Claude SDK reasoning loop)   │
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

### API Layer (FastAPI)

**Endpoints:**
- `POST /api/v1/execute-task` - Start a new agentic task
- `GET /api/v1/task/{workflow_id}/status` - Query task status
- `POST /api/v1/task/{workflow_id}/signal` - Send signals to running tasks
- `POST /api/v1/task/{workflow_id}/cancel` - Cancel a running task
- `GET /api/v1/tasks` - List recent tasks

**Models:**
- `TaskParams` - Input parameters for task execution
- `TaskResponse` - Initial response with workflow tracking info
- `TaskStatus` - Current status of a workflow
- `TaskResult` - Final results from completed tasks

### Orchestration Layer (Temporal)

**Workflows:**
- `AgenticTaskWorkflow` - Main workflow orchestrating the full task lifecycle

**Workflow Steps:**
1. Clone repository to workspace
2. Execute agent reasoning loop
3. Create pull request (if changes made)
4. Wait for CI results (optional)
5. Handle failures and iterate

**Features:**
- Durable execution (survives service restarts)
- Long waits (hours/days for CI, code review)
- Signal handlers (external events like webhooks)
- Query handlers (status checks)
- Automatic retries with backoff

### Activities Layer

**Git Operations (`git_operations.py`):**
- `clone_repository` - Clone GitHub repos to workspace
- `create_branch` - Create and checkout new branches
- `commit_changes` - Stage and commit all changes
- `push_changes` - Push commits to remote

**Agent Runtime (`agent_runtime.py`):**
- `agent_reasoning_step` - Execute Claude reasoning loop with tools
- Tool execution (read files, run commands, search code)
- Returns structured results with actions taken

**GitHub Operations (`github_operations.py`):**
- `create_pull_request` - Create PRs on GitHub
- `get_ci_status` - Check CI/CD pipeline status
- `get_review_comments` - Fetch PR review comments

### Agent Runtime Layer

**Claude Client (`claude_client.py`):**
- Multi-turn reasoning loops with conversation state
- Tool use integration
- Streaming support for long outputs

**Tools (`tools.py`):**
- `read_file` - Read file contents
- `list_files` - List directory contents
- `run_command` - Execute shell commands
- `search_code` - Search for code patterns

**Prompts (`prompts.py`):**
- `AUDIT_REPOSITORY_PROMPT` - Repository analysis tasks
- `CREATE_PR_PROMPT` - PR creation tasks
- `ANALYZE_CI_FAILURE_PROMPT` - CI failure debugging

## Data Flow

### Typical Task Execution Flow

1. **API Request**
   ```
   POST /execute-task
   {
     "repo_url": "https://github.com/org/repo",
     "task_description": "Audit for agentic SDLC readiness",
     "checklist": ["Check .golangci.yml", "Check Makefile"],
     "context": {"repo_type": "operator"}
   }
   ```

2. **Workflow Initialization**
   - Temporal workflow started with unique ID
   - Workflow parameters stored in Temporal

3. **Repository Clone**
   - Activity: `clone_repository`
   - Creates workspace directory
   - Clones repo to local filesystem

4. **Agent Reasoning**
   - Activity: `agent_reasoning_step`
   - Claude analyzes repository structure
   - Uses tools to read files, run commands
   - Generates findings and recommendations

5. **PR Creation (if changes made)**
   - Activity: `create_pull_request`
   - Creates GitHub PR with changes
   - Returns PR URL for tracking

6. **CI Monitoring (optional)**
   - Workflow waits for CI signal
   - Activity: `get_ci_status`
   - If failed, agent analyzes and fixes

7. **Result Return**
   - Workflow completes with TaskResult
   - API returns structured response

## State Management

### Temporal State Persistence

- **Workflow State**: Persisted in PostgreSQL by Temporal
- **Activity Results**: Stored in workflow history
- **Signals**: Queued in Temporal and delivered to workflows
- **Workspace Files**: Ephemeral, cleaned up after completion

### Conversation State

- **Agent Reasoning**: Maintained in memory during workflow execution
- **Tool Results**: Passed between Claude turns
- **Context**: Preserved across agent iterations

## Deployment Architecture

### Local Development
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   FastAPI   │────▶│  Temporal   │────▶│  Worker     │
│   (Port     │     │  Server     │     │  Process    │
│    8000)    │     │ (Docker)    │     │             │
└─────────────┘     └─────────────┘     └─────────────┘
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
│  API (ECS/  │────▶│  Temporal   │────▶│  Workers    │
│  K8s)       │     │  Cloud      │     │  (ECS/K8s)  │
└─────────────┘     └─────────────┘     └─────────────┘
      │                                        │
      ▼                                        ▼
┌─────────────┐                        ┌─────────────┐
│  CloudWatch │                        │  Ephemeral  │
│  Logs       │                        │  Workspaces │
└─────────────┘                        └─────────────┘
```

## Design Decisions

### Why Temporal?

1. **Long-running workflows** - PR workflows can wait hours/days for CI and reviews
2. **Durability** - Service restarts don't lose workflow state
3. **External events** - Built-in signal handling for webhooks
4. **Observability** - Temporal UI shows workflow execution history
5. **Retry logic** - Automatic retries for transient failures

### Why NOT Bedrock Agents?

1. **POC simplicity** - Direct Claude SDK gives more control
2. **Debugging** - Easier to iterate on prompts and tools
3. **Flexibility** - Can migrate to Bedrock later if needed
4. **Cost** - Bedrock Agents add overhead for simple tasks

### Why Claude SDK Direct?

1. **Full control** - Complete visibility into reasoning process
2. **Tool customization** - Define exactly the tools needed
3. **Extended thinking** - Access to Claude's advanced reasoning modes
4. **Streaming** - Real-time output for long-running tasks

## Security Considerations

### Credentials Management
- GitHub tokens stored in environment variables
- Anthropic API keys in environment variables
- Temporal uses mTLS for production deployments

### Workspace Isolation
- Each task gets isolated workspace directory
- Workspaces cleaned up after completion
- Commands restricted to workspace root

### Tool Safety
- File operations restricted to workspace
- Command execution sandboxed
- GitHub operations require proper authentication

## Observability

### Logging
- Structured logging with context throughout
- Activity-level logging in Temporal
- API request/response logging

### Monitoring
- Temporal UI for workflow visualization
- FastAPI metrics endpoint
- Worker health checks

### Debugging
- Temporal time-travel debugging
- Full conversation history in logs
- Tool execution traces

## Future Enhancements

### Near-term
1. Implement full Claude SDK integration in agent runtime
2. Add webhook receiver for GitHub CI events
3. Implement human-in-the-loop approval steps
4. Add metrics dashboard integration

### Long-term
1. Multi-agent workflows (parallel analysis)
2. Learned preferences from feedback
3. Integration with monitoring systems (osde2e)
4. Container-based isolation for workspaces
