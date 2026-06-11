# Temporal Activities

This directory contains Temporal activity definitions for the agentic task execution system.

## Activity Modules

### 1. git_operations.py

Git-related activities for repository management:

- **clone_repository(repo_url, workspace_dir)** - Clone a Git repository
  - Returns: `CloneResult` with workspace path
  
- **create_branch(workspace, branch_name)** - Create and checkout a new branch
  - Returns: `BranchResult` with branch name
  
- **commit_changes(workspace, message)** - Stage and commit all changes
  - Returns: `CommitResult` with commit SHA
  
- **push_changes(workspace)** - Push commits to remote
  - Returns: `PushResult` with success status

**Dependencies**: GitPython

**Example**:
```python
from temporalio.client import Client
from src.activities import clone_repository, create_branch

# In a workflow
result = await workflow.execute_activity(
    clone_repository,
    args=["https://github.com/org/repo.git", "/tmp/workspace"],
    start_to_close_timeout=timedelta(minutes=5)
)
```

### 2. agent_runtime.py

Agent reasoning activities using Claude SDK:

- **agent_reasoning_step(workspace, task_description, checklist, context)** - Execute a reasoning step
  - Returns: `AgentResult` with reasoning, actions, and next steps
  - **Status**: Skeleton implementation - Claude SDK integration pending

**Dependencies**: anthropic (Claude SDK)

**Planned Features**:
- Tool definitions for file operations, bash commands, etc.
- Reasoning loop with tool use
- Structured result parsing
- Context management

**Example** (future implementation):
```python
result = await workflow.execute_activity(
    agent_reasoning_step,
    args=[
        "/tmp/workspace",
        "Fix authentication bug in login module",
        ["Review login.py", "Write tests", "Fix bug"],
        {"previous_attempts": 0}
    ],
    start_to_close_timeout=timedelta(minutes=10)
)
```

### 3. github_operations.py

GitHub API activities for PR management:

- **create_pull_request(repo, branch, title, body, base_branch)** - Create a PR
  - Returns: `PRResult` with PR URL and number
  
- **get_ci_status(pr_url)** - Check CI/CD pipeline status
  - Returns: `CIStatusResult` with status enum and details
  
- **get_review_comments(pr_url)** - Fetch all PR comments
  - Returns: `CommentsResult` with list of comments

**Dependencies**: PyGithub

**Environment Variables**:
- `GITHUB_TOKEN` - GitHub personal access token (required)

**Example**:
```python
# Create PR
pr_result = await workflow.execute_activity(
    create_pull_request,
    args=["org/repo", "feature-branch", "Add new feature", "This PR adds..."],
    start_to_close_timeout=timedelta(minutes=2)
)

# Check CI status
ci_result = await workflow.execute_activity(
    get_ci_status,
    args=[pr_result.pr_url],
    start_to_close_timeout=timedelta(minutes=1)
)
```

## Installation

Install required dependencies:

```bash
pip install -r requirements-temporal.txt
```

Or install from pyproject.toml:

```bash
pip install -e .
```

## Configuration

### GitHub Token

Set the `GITHUB_TOKEN` environment variable:

```bash
export GITHUB_TOKEN=ghp_your_token_here
```

### Git Credentials

Ensure Git is configured with proper credentials for push operations:

```bash
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"
```

For HTTPS repositories, consider using a credential helper:

```bash
git config --global credential.helper store
```

## Error Handling

All activities follow a consistent error handling pattern:

1. Return structured result objects with `success` boolean
2. Include `error` field with detailed error messages
3. Log errors using Python logging
4. Catch and handle library-specific exceptions (GitCommandError, GithubException)

## Testing

Run tests with pytest:

```bash
pytest tests/activities/
```

## Next Steps

### Agent Runtime Implementation

The `agent_runtime.py` module is currently a skeleton. To implement:

1. Set up Claude SDK client with API key
2. Implement tool execution functions
3. Create reasoning loop with tool use
4. Add structured result parsing
5. Implement context management

See the inline TODOs in `agent_runtime.py` for details.
