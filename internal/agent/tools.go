package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
)

func toolDef(name, description string, properties map[string]any, required []string) anthropic.ToolUnionParam {
	t := anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: properties,
			Required:   required,
		},
		name,
	)
	t.OfTool.Description = anthropic.String(description)
	return t
}

func GetToolDefinitions() []anthropic.ToolUnionParam {
	return []anthropic.ToolUnionParam{
		toolDef("read_file",
			"Read the contents of a file in the workspace. Returns the file contents as a string.",
			map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The path to the file relative to the workspace root",
				},
			},
			[]string{"file_path"},
		),
		toolDef("list_files",
			"List files and directories in a directory. Returns a list of file/directory names.",
			map[string]any{
				"directory": map[string]any{
					"type":        "string",
					"description": "The directory path relative to the workspace root (use '.' for root)",
				},
			},
			[]string{"directory"},
		),
		toolDef("execute_command",
			"Execute a bash command in the workspace directory. Use for testing, searching, or gathering information. Returns stdout and stderr.",
			map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The bash command to execute",
				},
			},
			[]string{"command"},
		),
		toolDef("write_file",
			"Write content to a file in the workspace. Creates the file if it doesn't exist, overwrites if it does.",
			map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "The path to the file relative to the workspace root",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The content to write to the file",
				},
			},
			[]string{"file_path", "content"},
		),
		toolDef("search_files",
			"Search for a pattern in files using grep. Returns matching lines with file names.",
			map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "The pattern to search for",
				},
				"file_pattern": map[string]any{
					"type":        "string",
					"description": "File pattern to search in (e.g., '*.go', '*.yaml'). Optional, defaults to all files.",
				},
			},
			[]string{"pattern"},
		),
	}
}
