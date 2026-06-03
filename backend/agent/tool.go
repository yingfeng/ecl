package agent

import (
	"context"
	"fmt"
	"strings"
)

// ========== P0: Tool Abstraction Interface ==========
// Modeled after iceCoder's ToolRegistry + ToolExecutor architecture.
// Each pipeline stage is a compilable, independently-testable Tool.

// ToolDeps carries shared pipeline state into a Tool execution.
type ToolDeps struct {
	Files   []FileNode
	Skills  []SkillDef
	Topics  []TopicInfo
	Outputs []OutputFile

	// Runtime context
	Budget   *BranchBudgetTracker
	ModeDec  *ModeDecider
	Task     *TaskState
	Input    *CompileInput
	Output   []OutputFile
	FileSvc  *serviceProvider
}

// serviceProvider wraps the minimum services a tool needs.
type serviceProvider struct {
	*Compiler
}

// Tool defines a single stage/operation in the compilation pipeline.
// Each Tool has a name, description, and execute method.
type Tool interface {
	// Name returns the unique tool identifier.
	Name() string

	// Description returns a human-readable description of what this tool does.
	Description() string

	// Execute runs the tool with the given dependencies and returns outputs.
	Execute(ctx context.Context, deps *ToolDeps) (any, error)
}

// ToolRegistry manages tool registration and lookup.
// Modeled after iceCoder's ToolRegistry.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds or replaces a tool in the registry.
// Modeled after iceCoder's register (overwrite on duplicate).
func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get retrieves a registered tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool names.
func (r *ToolRegistry) List() []string {
	var names []string
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// ==========  StageTool: wraps a pipeline stage as a Tool ==========

// StageTool is a convenience wrapper for converting a pipeline stage function into a Tool.
type StageTool struct {
	name        string
	description string
	runFn       func(ctx context.Context, deps *ToolDeps) (any, error)
}

// NewStageTool creates a Tool from a function.
func NewStageTool(name, description string, fn func(ctx context.Context, deps *ToolDeps) (any, error)) Tool {
	return &StageTool{
		name:        name,
		description: description,
		runFn:       fn,
	}
}

func (t *StageTool) Name() string             { return t.name }
func (t *StageTool) Description() string        { return t.description }
func (t *StageTool) Execute(ctx context.Context, deps *ToolDeps) (any, error) {
	return t.runFn(ctx, deps)
}

// ==========  Tool Names (constants) ==========

const (
	ToolLoad        = "load"
	ToolKeywords    = "extract_keywords"
	ToolScan        = "scan"
	ToolMerge       = "merge_topics"
	ToolCompile     = "compile"
	ToolVerify      = "verify"
	ToolConsistency = "consistency_review"
	ToolConcept     = "concept_discovery"
	ToolIndex       = "generate_index"
	ToolLog         = "generate_log"
	ToolQuality     = "quality_review"
)

// ==========  Tool Command Parsing ==========
// Extends pipeline.go's parseJSONWithRecovery for LLM tool call parsing.
// P3: Enhanced JSON parsing with truncation recovery.

// ToolCall represents an LLM's request to execute a tool.
type ToolCall struct {
	Name      string            `json:"name"`
	Arguments map[string]any    `json:"arguments"`
}

// ParseToolCall attempts to parse a tool call from an LLM response string.
// Supports multiple formats:
//   - JSON: {"name": "...", "arguments": {...}}
//   - Function call format: Function(name="...", arguments={...})
//   - Text format: TOOL: name -> arguments
func ParseToolCall(content string) (*ToolCall, error) {
	content = strings.TrimSpace(content)

	// Try direct JSON parse first
	var tc ToolCall
	if err := parseJSONWithRecovery([]byte(content), &tc); err == nil && tc.Name != "" {
		return &tc, nil
	}

	// Try JSON with ```json fence
	cleaned := stripJSONFence(content)
	if cleaned != content {
		if err := parseJSONWithRecovery([]byte(cleaned), &tc); err == nil && tc.Name != "" {
			return &tc, nil
		}
	}

	// Try TOOL: prefix format
	if strings.HasPrefix(content, "TOOL:") {
		rest := strings.TrimSpace(content[5:])
		parts := strings.SplitN(rest, "->", 2)
		if len(parts) == 2 {
			tc.Name = strings.TrimSpace(parts[0])
			// arguments part is unstructured text
			tc.Arguments = map[string]any{"text": strings.TrimSpace(parts[1])}
			return &tc, nil
		}
	}

	return nil, fmt.Errorf("cannot parse tool call from: %s", truncate(content, 100))
}
