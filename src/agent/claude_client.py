"""
Claude Client wrapper for multi-turn reasoning with tool use via Vertex AI.

This module provides a high-level interface to the Anthropic SDK's Vertex AI
integration for building autonomous agents that can use tools and maintain
conversation state.
"""

import logging
from typing import Any, Dict, List, Optional

from anthropic import AnthropicVertex

logger = logging.getLogger(__name__)


class ClaudeClient:
    """
    Wrapper around Anthropic Vertex AI SDK for multi-turn agent interactions.

    Manages conversation state and handles tool execution loops.
    """

    def __init__(
        self,
        project_id: str,
        region: str = "us-east5",
        model: str = "claude-sonnet-4-5@20250929",
        max_tokens: int = 4096
    ):
        """
        Initialize the Claude client with Vertex AI.

        Args:
            project_id: GCP project ID
            region: GCP region for Vertex AI (default: us-east5)
            model: Model ID to use (default: Sonnet 4.5)
            max_tokens: Maximum tokens for responses
        """
        self.project_id = project_id
        self.region = region
        self.model = model
        self.max_tokens = max_tokens

        # Initialize Anthropic Vertex AI client
        # This will use Application Default Credentials from gcloud
        self.client = AnthropicVertex(
            project_id=project_id,
            region=region
        )

        logger.info(
            f"Initialized Claude client with Vertex AI: "
            f"project={project_id}, region={region}, model={model}"
        )

    def create_message(
        self,
        messages: List[Dict[str, Any]],
        system: Optional[str] = None,
        tools: Optional[List[Dict[str, Any]]] = None,
        temperature: float = 1.0
    ) -> Dict[str, Any]:
        """
        Create a single message with Claude via Vertex AI.

        Args:
            messages: List of message dicts with role and content
            system: Optional system prompt
            tools: Optional list of tool definitions
            temperature: Sampling temperature (0-1)

        Returns:
            Response dict from Claude API
        """
        kwargs = {
            "model": self.model,
            "max_tokens": self.max_tokens,
            "messages": messages,
            "temperature": temperature,
        }

        if system:
            kwargs["system"] = system

        if tools:
            kwargs["tools"] = tools

        try:
            response = self.client.messages.create(**kwargs)
            return response
        except Exception as e:
            logger.error(f"Error creating message: {e}")
            raise

    def run_agent_loop(
        self,
        initial_prompt: str,
        system_prompt: str,
        tools: List[Dict[str, Any]],
        tool_executor: Any,
        max_iterations: int = 10
    ) -> Dict[str, Any]:
        """
        Run a multi-turn agent loop with tool use.

        Continues until Claude stops requesting tools or max_iterations reached.

        Args:
            initial_prompt: Initial user message to start the conversation
            system_prompt: System prompt defining agent behavior
            tools: List of tool definitions available to the agent
            tool_executor: Object that can execute tools (must have execute_tool method)
            max_iterations: Maximum number of turns to prevent infinite loops

        Returns:
            Dict containing:
                - final_response: Claude's final text response
                - conversation_history: Full message history
                - iterations: Number of turns taken
                - tool_calls: List of all tool calls made
        """
        conversation_history = [
            {"role": "user", "content": initial_prompt}
        ]

        all_tool_calls = []
        iteration = 0

        logger.info(f"Starting agent loop with max_iterations={max_iterations}")

        while iteration < max_iterations:
            iteration += 1
            logger.debug(f"Agent iteration {iteration}/{max_iterations}")

            # Get response from Claude
            response = self.create_message(
                messages=conversation_history,
                system=system_prompt,
                tools=tools
            )

            # Add assistant response to history
            assistant_message = {
                "role": "assistant",
                "content": response.content
            }
            conversation_history.append(assistant_message)

            # Check if there are any tool uses
            tool_uses = [
                block for block in response.content
                if block.type == "tool_use"
            ]

            if not tool_uses:
                # No more tool uses, extract final text response
                text_blocks = [
                    block.text for block in response.content
                    if block.type == "text"
                ]
                final_response = "\n".join(text_blocks)

                logger.info(f"Agent loop completed after {iteration} iterations")

                return {
                    "final_response": final_response,
                    "conversation_history": conversation_history,
                    "iterations": iteration,
                    "tool_calls": all_tool_calls
                }

            # Execute all tool uses
            tool_results = []
            for tool_use in tool_uses:
                logger.debug(f"Executing tool: {tool_use.name}")
                all_tool_calls.append({
                    "name": tool_use.name,
                    "input": tool_use.input
                })

                result = self._handle_tool_use(tool_use, tool_executor)
                tool_results.append(result)

            # Add tool results to conversation
            conversation_history.append({
                "role": "user",
                "content": tool_results
            })

        # Max iterations reached
        logger.warning(f"Agent loop reached max_iterations={max_iterations}")

        return {
            "final_response": "Max iterations reached",
            "conversation_history": conversation_history,
            "iterations": iteration,
            "tool_calls": all_tool_calls
        }

    def continue_conversation(
        self,
        conversation_history: List[Dict[str, Any]],
        new_message: str,
        tools: Optional[List[Dict[str, Any]]] = None,
        tool_executor: Optional[Any] = None
    ) -> Dict[str, Any]:
        """
        Continue an existing conversation with a new user message.

        Args:
            conversation_history: Existing message history
            new_message: New user message to add
            tools: Optional tools for this turn
            tool_executor: Optional tool executor if tools provided

        Returns:
            Updated conversation state
        """
        # Add new user message
        conversation_history.append({
            "role": "user",
            "content": new_message
        })

        # Get response
        response = self.create_message(
            messages=conversation_history,
            tools=tools
        )

        # Add response to history
        conversation_history.append({
            "role": "assistant",
            "content": response.content
        })

        # If tools and executor provided, handle tool uses
        if tools and tool_executor:
            tool_uses = [
                block for block in response.content
                if block.type == "tool_use"
            ]

            if tool_uses:
                tool_results = []
                for tool_use in tool_uses:
                    result = self._handle_tool_use(tool_use, tool_executor)
                    tool_results.append(result)

                conversation_history.append({
                    "role": "user",
                    "content": tool_results
                })

        return {
            "conversation_history": conversation_history,
            "response": response
        }

    def _handle_tool_use(
        self,
        tool_use_block: Any,
        tool_executor: Any
    ) -> Dict[str, Any]:
        """
        Execute a single tool use request.

        Args:
            tool_use_block: Tool use content block from Claude
            tool_executor: Object that can execute the tool

        Returns:
            Tool result dict formatted for Claude API
        """
        tool_name = tool_use_block.name
        tool_input = tool_use_block.input
        tool_use_id = tool_use_block.id

        try:
            # Execute the tool
            result = tool_executor.execute_tool(tool_name, tool_input)

            # Format success result
            return {
                "type": "tool_result",
                "tool_use_id": tool_use_id,
                "content": str(result)
            }
        except Exception as e:
            logger.error(f"Error executing tool {tool_name}: {e}")

            # Format error result
            return {
                "type": "tool_result",
                "tool_use_id": tool_use_id,
                "content": f"Error executing tool: {str(e)}",
                "is_error": True
            }
