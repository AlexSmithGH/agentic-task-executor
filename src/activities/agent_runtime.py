"""Agent runtime activities for Temporal workflows."""

import logging
import os
import subprocess
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Any, Dict, List, Optional

from temporalio import activity

from src.agent.claude_client import ClaudeClient
from src.config import settings

logger = logging.getLogger(__name__)


@dataclass
class AgentResult:
    """Result of an agent reasoning step."""
    success: bool
    reasoning: str
    action_taken: str
    next_steps: List[str]
    context_updates: Dict[str, Any]
    files_modified: List[str]
    error: Optional[str] = None


@activity.defn
async def agent_reasoning_step(
    workspace: str,
    task_description: str,
    checklist: List[str],
    context: Dict[str, Any]
) -> Dict[str, Any]:
    """
    Execute agent reasoning using Claude via Vertex AI.

    Args:
        workspace: The path to the Git repository workspace
        task_description: Description of the task to work on
        checklist: List of checklist items to complete
        context: Additional context for the reasoning step

    Returns:
        Dict representation of AgentResult
    """
    try:
        logger.info(f"Starting agent reasoning step for task: {task_description}")
        logger.info(f"Workspace: {workspace}")
        logger.info(f"Checklist items: {len(checklist)}")

        # Initialize Claude client with Vertex AI
        claude_client = ClaudeClient(
            project_id=settings.gcp_project_id,
            region=settings.gcp_region,
            model="claude-sonnet-4-5@20250929",
            max_tokens=8192
        )

        # Get tool definitions
        tools = _get_tool_definitions()

        # Build system prompt
        system_prompt = _build_system_prompt(task_description, checklist, context)

        # Build initial user prompt
        initial_prompt = _build_initial_prompt(workspace, task_description, checklist)

        # Execute agent loop with tools
        result = claude_client.run_agent_loop(
            initial_prompt=initial_prompt,
            system_prompt=system_prompt,
            tools=tools,
            tool_executor=ToolExecutor(workspace),
            max_iterations=100
        )

        # Parse results
        final_response = result["final_response"]
        tool_calls = result["tool_calls"]
        iterations = result["iterations"]

        logger.info(f"Agent completed after {iterations} iterations")
        logger.info(f"Tool calls made: {len(tool_calls)}")

        # Extract files modified from tool calls
        files_modified = [
            tc["input"].get("file_path", "")
            for tc in tool_calls
            if tc["name"] in ["write_file", "read_file"] and "file_path" in tc["input"]
        ]

        # Build result
        agent_result = AgentResult(
            success=True,
            reasoning=final_response,
            action_taken=f"Executed {len(tool_calls)} tool calls across {iterations} iterations",
            next_steps=_extract_next_steps(final_response),
            context_updates={
                "iterations": iterations,
                "tool_calls": tool_calls
            },
            files_modified=list(set(files_modified)),
            error=None
        )

        return asdict(agent_result)

    except Exception as e:
        error_msg = f"Agent reasoning step failed: {e}"
        logger.error(error_msg)
        logger.exception(e)

        error_result = AgentResult(
            success=False,
            reasoning="",
            action_taken="",
            next_steps=[],
            context_updates={},
            files_modified=[],
            error=error_msg
        )
        return asdict(error_result)


def _build_system_prompt(
    task_description: str,
    checklist: List[str],
    context: Dict[str, Any]
) -> str:
    """Build system prompt for the agent."""

    checklist_str = "\n".join(f"- {item}" for item in checklist) if checklist else "No specific checklist"

    return f"""You are an expert software engineer analyzing a Git repository.

Your task: {task_description}

Checklist to verify:
{checklist_str}

You have access to tools to:
- Read files in the repository
- List directory contents
- Execute commands (for testing, searching)

Your goal is to thoroughly analyze the repository and provide a comprehensive report on the checklist items.

For each checklist item:
1. Use tools to investigate the repository
2. Document what you find
3. Provide clear answers (present/absent, configured/not configured, etc.)

Be thorough and specific. Use the tools available to actually verify each item rather than making assumptions.

At the end, provide a clear summary of your findings."""


def _build_initial_prompt(
    workspace: str,
    task_description: str,
    checklist: List[str]
) -> str:
    """Build initial user prompt."""

    return f"""Analyze the repository at: {workspace}

Task: {task_description}

Please verify each of the following checklist items and provide a detailed report:

{chr(10).join(f"{i+1}. {item}" for i, item in enumerate(checklist))}

Start by exploring the repository structure, then systematically check each item."""


def _extract_next_steps(response: str) -> List[str]:
    """Extract next steps from agent response."""
    # Simple extraction - look for lines that suggest action items
    next_steps = []

    # Look for common patterns
    for line in response.split('\n'):
        line = line.strip()
        if any(line.startswith(prefix) for prefix in ["- [ ]", "TODO:", "Next:", "Action:"]):
            next_steps.append(line)

    # If no next steps found, return empty list
    return next_steps[:5]  # Limit to 5 next steps


def _get_tool_definitions() -> List[Dict[str, Any]]:
    """Get tool definitions for Claude SDK."""
    return [
        {
            "name": "read_file",
            "description": "Read the contents of a file in the workspace. Returns the file contents as a string.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "file_path": {
                        "type": "string",
                        "description": "The path to the file relative to the workspace root"
                    }
                },
                "required": ["file_path"]
            }
        },
        {
            "name": "list_files",
            "description": "List files and directories in a directory. Returns a list of file/directory names.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "directory": {
                        "type": "string",
                        "description": "The directory path relative to the workspace root (use '.' for root)"
                    }
                },
                "required": ["directory"]
            }
        },
        {
            "name": "execute_command",
            "description": "Execute a bash command in the workspace directory. Use for testing, searching, or gathering information. Returns stdout and stderr.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "The bash command to execute"
                    }
                },
                "required": ["command"]
            }
        },
        {
            "name": "search_files",
            "description": "Search for a pattern in files using grep. Returns matching lines with file names.",
            "input_schema": {
                "type": "object",
                "properties": {
                    "pattern": {
                        "type": "string",
                        "description": "The pattern to search for"
                    },
                    "file_pattern": {
                        "type": "string",
                        "description": "File pattern to search in (e.g., '*.go', '*.yaml'). Optional, defaults to all files."
                    }
                },
                "required": ["pattern"]
            }
        }
    ]


class ToolExecutor:
    """Executes tools for the Claude agent."""

    def __init__(self, workspace: str):
        """Initialize tool executor with workspace path."""
        self.workspace = Path(workspace)
        if not self.workspace.exists():
            raise ValueError(f"Workspace does not exist: {workspace}")

    def execute_tool(self, tool_name: str, tool_input: Dict[str, Any]) -> str:
        """Execute a tool and return the result as a string."""
        logger.info(f"Executing tool: {tool_name} with input: {tool_input}")

        try:
            if tool_name == "read_file":
                return self._read_file(tool_input["file_path"])
            elif tool_name == "list_files":
                return self._list_files(tool_input["directory"])
            elif tool_name == "execute_command":
                return self._execute_command(tool_input["command"])
            elif tool_name == "search_files":
                return self._search_files(
                    tool_input["pattern"],
                    tool_input.get("file_pattern")
                )
            else:
                return f"Error: Unknown tool: {tool_name}"

        except Exception as e:
            error_msg = f"Tool execution failed: {str(e)}"
            logger.error(error_msg)
            return error_msg

    def _read_file(self, file_path: str) -> str:
        """Read file contents."""
        full_path = self.workspace / file_path

        # Security: ensure path is within workspace
        try:
            full_path = full_path.resolve()
            full_path.relative_to(self.workspace.resolve())
        except ValueError:
            return f"Error: Path {file_path} is outside workspace"

        if not full_path.exists():
            return f"Error: File not found: {file_path}"

        if not full_path.is_file():
            return f"Error: Not a file: {file_path}"

        try:
            with open(full_path, 'r', encoding='utf-8') as f:
                content = f.read()
            return content
        except UnicodeDecodeError:
            return f"Error: File {file_path} is not a text file (binary content)"
        except Exception as e:
            return f"Error reading file: {str(e)}"

    def _list_files(self, directory: str) -> str:
        """List files in directory."""
        dir_path = self.workspace / directory

        # Security: ensure path is within workspace
        try:
            dir_path = dir_path.resolve()
            dir_path.relative_to(self.workspace.resolve())
        except ValueError:
            return f"Error: Path {directory} is outside workspace"

        if not dir_path.exists():
            return f"Error: Directory not found: {directory}"

        if not dir_path.is_dir():
            return f"Error: Not a directory: {directory}"

        try:
            items = []
            for item in sorted(dir_path.iterdir()):
                if item.is_dir():
                    items.append(f"{item.name}/")
                else:
                    items.append(item.name)
            return "\n".join(items)
        except Exception as e:
            return f"Error listing directory: {str(e)}"

    def _execute_command(self, command: str) -> str:
        """Execute bash command in workspace."""
        logger.info(f"Executing command: {command}")

        try:
            result = subprocess.run(
                command,
                shell=True,
                cwd=str(self.workspace),
                capture_output=True,
                text=True,
                timeout=30  # 30 second timeout
            )

            output = []
            if result.stdout:
                output.append(f"STDOUT:\n{result.stdout}")
            if result.stderr:
                output.append(f"STDERR:\n{result.stderr}")
            if result.returncode != 0:
                output.append(f"Exit code: {result.returncode}")

            return "\n".join(output) if output else "Command completed with no output"

        except subprocess.TimeoutExpired:
            return "Error: Command timed out after 30 seconds"
        except Exception as e:
            return f"Error executing command: {str(e)}"

    def _search_files(self, pattern: str, file_pattern: Optional[str] = None) -> str:
        """Search for pattern in files."""
        try:
            if file_pattern:
                command = f"grep -r --include='{file_pattern}' '{pattern}' ."
            else:
                command = f"grep -r '{pattern}' ."

            result = subprocess.run(
                command,
                shell=True,
                cwd=str(self.workspace),
                capture_output=True,
                text=True,
                timeout=30
            )

            if result.returncode == 0:
                return result.stdout if result.stdout else "No matches found"
            elif result.returncode == 1:
                return "No matches found"
            else:
                return f"Search failed: {result.stderr}"

        except subprocess.TimeoutExpired:
            return "Error: Search timed out after 30 seconds"
        except Exception as e:
            return f"Error searching files: {str(e)}"
