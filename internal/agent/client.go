package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/vertex"

	"github.com/alexasmi/agentic-task-executor/internal/models"
)

type ToolExecutor interface {
	ExecuteTool(name string, input map[string]any) string
}

type ClaudeClient struct {
	client    anthropic.Client
	model     string
	maxTokens int64
}

func NewClaudeClient(ctx context.Context, projectID, region, model string, maxTokens int64) *ClaudeClient {
	client := anthropic.NewClient(
		vertex.WithGoogleAuth(ctx, region, projectID),
	)
	slog.Info("Initialized Claude client with Vertex AI",
		"project", projectID, "region", region, "model", model)

	return &ClaudeClient{
		client:    client,
		model:     model,
		maxTokens: maxTokens,
	}
}

func (c *ClaudeClient) CreateMessage(ctx context.Context, messages []anthropic.MessageParam, system string, tools []anthropic.ToolUnionParam) (*anthropic.Message, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Messages:  messages,
	}

	if system != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: system},
		}
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	return c.client.Messages.New(ctx, params)
}

func (c *ClaudeClient) RunAgentLoop(ctx context.Context, initialPrompt, systemPrompt string, tools []anthropic.ToolUnionParam, executor ToolExecutor, maxIterations int) (*models.AgentLoopResult, error) {
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(initialPrompt)),
	}

	var allToolCalls []models.ToolCall

	slog.Info("Starting agent loop", "max_iterations", maxIterations)

	for i := range maxIterations {
		slog.Debug("Agent iteration", "iteration", i+1, "max", maxIterations)

		response, err := c.CreateMessage(ctx, messages, systemPrompt, tools)
		if err != nil {
			return nil, fmt.Errorf("create message at iteration %d: %w", i+1, err)
		}

		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(response.Content))
		var toolUses []anthropic.ToolUseBlock
		var textParts []string

		for _, block := range response.Content {
			switch block.Type {
			case "tool_use":
				tu := block.AsToolUse()
				toolUses = append(toolUses, tu)
				assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(tu.ID, json.RawMessage(tu.Input), tu.Name))
			case "text":
				t := block.AsText()
				textParts = append(textParts, t.Text)
				assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(t.Text))
			}
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		if len(toolUses) == 0 {
			slog.Info("Agent loop completed", "iterations", i+1)
			return &models.AgentLoopResult{
				FinalResponse: strings.Join(textParts, "\n"),
				Iterations:    i + 1,
				ToolCalls:     allToolCalls,
			}, nil
		}

		toolResults := make([]anthropic.ContentBlockParamUnion, 0, len(toolUses))
		for _, tu := range toolUses {
			slog.Debug("Executing tool", "name", tu.Name)

			inputMap := make(map[string]any)
			if err := json.Unmarshal(tu.Input, &inputMap); err != nil {
				inputMap = map[string]any{"raw": string(tu.Input)}
			}

			allToolCalls = append(allToolCalls, models.ToolCall{Name: tu.Name, Input: inputMap})

			result := executor.ExecuteTool(tu.Name, inputMap)
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, result, false))
		}
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	slog.Warn("Agent loop reached max iterations", "max", maxIterations)
	return &models.AgentLoopResult{
		FinalResponse: "Max iterations reached",
		Iterations:    maxIterations,
		ToolCalls:     allToolCalls,
	}, nil
}
