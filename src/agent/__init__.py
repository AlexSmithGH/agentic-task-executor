"""
Agent module for autonomous task execution with Claude.

This module provides the core components for building agentic workflows
that can use tools to accomplish complex tasks autonomously.
"""

from .claude_client import ClaudeClient
from .tools import (
    ToolExecutor,
    get_all_tools,
    get_read_file_tool,
    get_list_files_tool,
    get_run_command_tool,
    get_search_code_tool,
)
from .prompts import (
    get_system_prompt,
    customize_prompt,
    AUDIT_REPOSITORY_PROMPT,
    CREATE_PR_PROMPT,
    ANALYZE_CI_FAILURE_PROMPT,
)

__all__ = [
    # Client
    "ClaudeClient",
    # Tools
    "ToolExecutor",
    "get_all_tools",
    "get_read_file_tool",
    "get_list_files_tool",
    "get_run_command_tool",
    "get_search_code_tool",
    # Prompts
    "get_system_prompt",
    "customize_prompt",
    "AUDIT_REPOSITORY_PROMPT",
    "CREATE_PR_PROMPT",
    "ANALYZE_CI_FAILURE_PROMPT",
]
