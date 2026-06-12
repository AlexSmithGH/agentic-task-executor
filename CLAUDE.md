# Claude & AI Agent Guide

This document provides guidance for AI assistants (like Claude) working with the Agentic Task Executor codebase. It complements the existing documentation with AI-specific context, patterns, and conventions.

## Quick Context

**What this project does:** An AI-powered task execution service that uses Claude AI and Temporal workflows to automate repository operations like audits, PR creation, and CI failure analysis.

**Primary languages/frameworks:**
- Go 1.23+ (Temporal SDK, Chi router)
- Temporal for durable workflow orchestration
- Claude AI via Vertex AI for agentic reasoning
- GitHub API for repository operations

**Key documentation:**
- [Architecture](docs/ARCHITECTURE.md) - System design and component details
- [Getting Started](docs/GETTING_STARTED.md) - Setup and usage guide
- [Quick Reference](docs/QUICK_REFERENCE.md) - Commands and operations
- [Project Status](docs/PROJECT_STATUS.md) - Current status and roadmap

## Project Structure

```
agentic-task-executor/
├── cmd/                    # Application entry points
│   ├── api/               # HTTP API server (port 8000)
│   └── worker/            # Temporal worker process
├── internal/              # Private application code
│   ├── activities/        # Temporal activities (git, agent, github)
│   ├── agent/            # Claude AI client and tools
│   ├── api/              # HTTP handlers and models
│   ├── config/           # Environment configuration
│   └── workflows/        # Temporal workflow definitions
├── docs/                 # Project documentation
├── .env.example          # Environment variable template
├── Makefile              # Build and development commands
├── docker-compose.yml    # Temporal local development
└── Dockerfile            # Multi-stage container build
```

## Code Patterns and Conventions

### 1. Temporal Activities Pattern

Activities use **struct-based registration** for dependency injection:

```go
// activities/git_operations.go
type GitActivities struct {
    config *config.Config
}

func NewGitActivities(cfg *config.Config) *GitActivities {
    return &GitActivities{config: cfg}
}

// All exported methods become Temporal activities
func (a *GitActivities) CloneRepository(ctx context.Context, input CloneInput) (*CloneResult, error) {
    // Implementation
}
```

**When adding a new activity:**
1. Add method to existing activity struct (if related) OR create new struct
2. Register in `cmd/worker/main.go` using `workflow.RegisterActivity()`
3. Define input/output structs with JSON tags
4. Use activity logger: `activity.GetLogger(ctx)`
5. Add heartbeats for long operations: `activity.RecordHeartbeat(ctx, progress)`

### 2. Workflow Pattern

Workflows are **functions** (not classes like Python Temporal):

```go
// workflows/task_workflow.go
func AgenticTaskWorkflow(ctx workflow.Context, input WorkflowInput) (*WorkflowResult, error) {
    logger := workflow.GetLogger(ctx)
    
    // Configure activity options
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 10 * time.Minute,
        HeartbeatTimeout:    30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            MaximumAttempts: 3,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)
    
    // Execute activities
    var cloneResult activities.CloneResult
    err := workflow.ExecuteActivity(ctx, "CloneRepository", cloneInput).Get(ctx, &cloneResult)
    
    return &WorkflowResult{...}, nil
}
```

**Key workflow rules:**
- ✅ Use `workflow.ExecuteActivity()` for side effects
- ✅ Use `workflow.Sleep()` for delays (NOT `time.Sleep()`)
- ✅ Use `workflow.Now()` for current time (NOT `time.Now()`)
- ✅ Use `workflow.GetLogger()` for logging
- ❌ NEVER use `time.Sleep()`, `time.Now()`, or `math.Rand()` directly
- ❌ NEVER do I/O operations directly in workflow code

### 3. API Handler Pattern

Handlers use Chi router with struct-based registration:

```go
// api/handler.go
type Handler struct {
    client temporal.Client
    config *config.Config
}

func (h *Handler) ExecuteTask(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var params TaskParams
    if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request body")
        return
    }
    
    // 2. Start workflow
    workflowOptions := client.StartWorkflowOptions{
        ID:        "task-" + uuid.New().String(),
        TaskQueue: h.config.TaskQueue,
    }
    
    // 3. Return response
    respondJSON(w, http.StatusOK, response)
}
```

### 4. Configuration Pattern

Environment variables loaded via struct tags:

```go
// config/config.go
type Config struct {
    GCPProjectID string `env:"GCP_PROJECT_ID,required"`
    GCPRegion    string `env:"GCP_REGION" envDefault:"us-east5"`
    GitHubToken  string `env:"GITHUB_TOKEN,required"`
    APIPort      string `env:"API_PORT" envDefault:"8000"`
}

func Load() (*Config, error) {
    godotenv.Load() // Load .env file
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

### 5. Agent Tools Pattern

Tools are defined with JSON Schema and executed via type switching:

```go
// agent/tools.go
func GetTools() []anthropic.ToolDefinition {
    return []anthropic.ToolDefinition{
        {
            Name:        "read_file",
            Description: "Read contents of a file in the workspace",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "file_path": map[string]interface{}{
                        "type":        "string",
                        "description": "Path to file relative to workspace root",
                    },
                },
                "required": []string{"file_path"},
            },
        },
    }
}

func ExecuteTool(ctx context.Context, toolName string, toolInput map[string]interface{}, workspaceDir string) (string, error) {
    switch toolName {
    case "read_file":
        return executeReadFile(toolInput, workspaceDir)
    // ... other tools
    }
}
```

## Adding New Features

### Adding a New Activity

1. **Decide where it belongs:**
   - Git operations → `activities/git_operations.go`
   - GitHub operations → `activities/github_operations.go`
   - Agent/Claude operations → `activities/agent_runtime.go`
   - New category → Create new file

2. **Add method to struct:**
```go
func (a *GitActivities) YourNewActivity(ctx context.Context, input YourInput) (*YourResult, error) {
    logger := activity.GetLogger(ctx)
    logger.Info("Starting your activity", "input", input)
    
    // Implementation
    
    return &YourResult{}, nil
}
```

3. **Define types:**
```go
type YourInput struct {
    Field1 string `json:"field1"`
    Field2 int    `json:"field2"`
}

type YourResult struct {
    Output string `json:"output"`
}
```

4. **Register in worker:**
```go
// cmd/worker/main.go
gitActivities := activities.NewGitActivities(cfg)
w.RegisterActivity(gitActivities)  // All methods auto-registered
```

5. **Use in workflow:**
```go
var result activities.YourResult
err := workflow.ExecuteActivity(ctx, "YourNewActivity", input).Get(ctx, &result)
```

### Adding a New API Endpoint

1. **Add handler method:**
```go
// api/handler.go
func (h *Handler) YourEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

2. **Register route:**
```go
// api/server.go
func (h *Handler) SetupRoutes() *chi.Mux {
    r.Get("/api/v1/your-endpoint", h.YourEndpoint)
}
```

3. **Add request/response types:**
```go
// api/models.go
type YourRequest struct {
    Field string `json:"field"`
}

type YourResponse struct {
    Result string `json:"result"`
}
```

### Adding a New Agent Tool

1. **Define tool in agent/tools.go:**
```go
{
    Name:        "your_tool",
    Description: "What your tool does",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param": map[string]interface{}{
                "type":        "string",
                "description": "Parameter description",
            },
        },
        "required": []string{"param"},
    },
}
```

2. **Implement execution:**
```go
func executeYourTool(input map[string]interface{}, workspaceDir string) (string, error) {
    // Extract parameters
    param, ok := input["param"].(string)
    if !ok {
        return "", fmt.Errorf("invalid param")
    }
    
    // Implementation
    
    return result, nil
}
```

3. **Add to switch statement:**
```go
func ExecuteTool(ctx context.Context, toolName string, toolInput map[string]interface{}, workspaceDir string) (string, error) {
    switch toolName {
    case "your_tool":
        return executeYourTool(toolInput, workspaceDir)
    // ...
    }
}
```

## Testing Requirements

### Current State
⚠️ **Note:** The project currently has limited test coverage. This is a known gap (see `docs/PROJECT_STATUS.md`).

### When Adding Tests

1. **Unit tests** - Test pure logic without dependencies:
```go
// internal/activities/git_operations_test.go
func TestCloneRepository(t *testing.T) {
    // Use test environment or mocks
}
```

2. **Integration tests** - Test with real Temporal:
```go
func TestAgenticTaskWorkflow(t *testing.T) {
    testSuite := &testsuite.WorkflowTestSuite{}
    env := testSuite.NewTestWorkflowEnvironment()
    
    // Register activities and workflows
    // Execute and assert
}
```

3. **Run tests:**
```bash
make test              # Run all tests
go test ./...          # Direct go test
go test -v ./internal/activities  # Verbose, specific package
```

## Common Tasks

### 1. Adding Support for a New Repository Type

**Files to modify:**
- `internal/agent/prompts.go` - Add new prompt template
- `internal/workflows/task_workflow.go` - Add conditional logic if needed
- `api/models.go` - Update TaskParams if new fields needed

### 2. Adding a New Workflow

```go
// internal/workflows/your_workflow.go
package workflows

func YourWorkflow(ctx workflow.Context, input YourInput) (*YourResult, error) {
    // Implementation
}

// Register in cmd/worker/main.go
w.RegisterWorkflow(workflows.YourWorkflow)

// Add API endpoint in api/handler.go
we, err := h.client.ExecuteWorkflow(ctx, workflowOptions, workflows.YourWorkflow, input)
```

### 3. Modifying Agent Behavior

**Prompt templates:** `internal/agent/prompts.go`
- `AuditRepositoryPrompt` - Repository analysis
- `CreatePRPrompt` - PR creation
- `AnalyzeCIFailurePrompt` - CI debugging

**Tool definitions:** `internal/agent/tools.go`
- Add/modify available tools
- Update tool descriptions
- Change parameters

**Reasoning loop:** `internal/agent/client.go`
- `RunAgentLoop()` - Multi-turn conversation logic
- Modify max iterations, stopping conditions

### 4. Changing Workflow Timeouts

```go
// internal/workflows/task_workflow.go
activityOptions := workflow.ActivityOptions{
    StartToCloseTimeout: 30 * time.Minute,  // Total execution time
    HeartbeatTimeout:    1 * time.Minute,   // Time between heartbeats
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,
        MaximumInterval:    time.Minute,
        MaximumAttempts:    3,
    },
}
```

## Build and Development

### Essential Commands

```bash
# Build
make build              # Build API and worker binaries

# Run
make dev               # Start full dev environment (Temporal + worker + API)
make run-api           # Run API server only
make run-worker        # Run worker only

# Test and Quality
make test              # Run tests
make lint              # Run golangci-lint

# Docker
make docker-up         # Start Temporal server
make docker-down       # Stop Temporal server
make docker-logs       # View Temporal logs

# Cleanup
make clean             # Remove build artifacts
```

### Development Workflow

1. **Start Temporal:** `make docker-up`
2. **Run worker:** `make run-worker` (in one terminal)
3. **Run API:** `make run-api` (in another terminal)
4. **Make changes** to code
5. **Restart** affected service (API or worker)
6. **Test** via curl or Temporal UI

### Debugging

**View workflow execution:**
- Temporal UI: http://localhost:8080
- Find workflow by ID
- View event history, activity results, errors

**Check logs:**
- API server logs: stdout from `make run-api`
- Worker logs: stdout from `make run-worker`
- Temporal logs: `make docker-logs`

**Common issues:**
- Activity timeout → Check `StartToCloseTimeout` in workflow
- GitHub auth error → Verify `GITHUB_TOKEN` in `.env`
- Claude API error → Check GCP credentials and Vertex AI enablement

## Code Style and Conventions

### Naming
- **Exported functions:** `PascalCase`
- **Unexported functions:** `camelCase`
- **Constants:** `PascalCase` or `SCREAMING_SNAKE_CASE`
- **Interfaces:** Usually end with `-er` (e.g., `Reader`, `Writer`)

### Error Handling
```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to clone repository: %w", err)
}

// Check specific errors
if errors.Is(err, ErrNotFound) {
    // Handle
}
```

### Logging
```go
// In activities
logger := activity.GetLogger(ctx)
logger.Info("Message", "key1", value1, "key2", value2)

// In workflows
logger := workflow.GetLogger(ctx)
logger.Info("Message", "key", value)

// In API
slog.Info("Message", "key", value)
```

### Comments
- Package-level doc comment on every package
- Exported functions/types must have doc comments
- Complex logic should have inline comments

## Security Considerations

### Workspace Isolation
All file operations **must** validate paths stay within workspace:

```go
absPath, err := filepath.Abs(filepath.Join(workspaceDir, filePath))
if err != nil {
    return "", fmt.Errorf("invalid file path: %w", err)
}
if !strings.HasPrefix(absPath, workspaceDir) {
    return "", fmt.Errorf("path escape attempt detected")
}
```

### Command Execution
Commands are sandboxed to workspace with timeout:

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
cmd := exec.CommandContext(ctx, "bash", "-c", command)
cmd.Dir = workspaceDir  // Execute in workspace
```

### Secrets
- Never log tokens or credentials
- Load from environment variables only
- Use `.gitignore` to exclude `.env`

## Dependencies

**When adding dependencies:**
```bash
go get github.com/some/package@version
go mod tidy
```

**Check for updates:**
```bash
go list -u -m all
go get -u ./...
go mod tidy
```

## References

- **Temporal Go SDK:** https://docs.temporal.io/dev-guide/go
- **Anthropic Go SDK:** https://github.com/anthropics/anthropic-sdk-go
- **Chi Router:** https://github.com/go-chi/chi
- **go-git:** https://github.com/go-git/go-git
- **Effective Go:** https://golang.org/doc/effective_go

## Contributing

When making changes:
1. ✅ Run `make lint` before committing
2. ✅ Add tests for new functionality
3. ✅ Update documentation if adding features
4. ✅ Follow existing code patterns
5. ✅ Validate workspace path isolation for file operations
6. ✅ Use activity options for timeouts and retries

## AI-Specific Notes

**When Claude or other AI agents work on this codebase:**

1. **Always validate Go syntax** - Go is strict about types, imports, and structure
2. **Respect Temporal patterns** - Don't use time.Now() or time.Sleep() in workflows
3. **Check workspace isolation** - File operations must stay in workspace directory
4. **Test changes locally** - Build and run before committing
5. **Update documentation** - Keep CLAUDE.md and other docs in sync with code changes
6. **Follow the existing patterns** - Don't introduce new paradigms without discussion

**Before proposing changes:**
- Read relevant documentation files
- Understand the Temporal workflow/activity model
- Check existing implementations for similar features
- Consider impact on running workflows (breaking changes)
