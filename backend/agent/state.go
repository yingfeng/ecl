package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// ========== Agent State ==========

// AgentState holds the working state across multiple tool calls in an agent session.
type AgentState struct {
	Files   []FileNode
	Skills  []SkillDef
	Topics  []TopicInfo
	Outputs []OutputFile
	Input   *CompileInput
	Task    *TaskState
	mu      sync.Mutex
}

// globalState is the shared state for the current agent run.
var globalState = &AgentState{
	Input: &CompileInput{},
	Task:  &TaskState{ID: "agent", Status: "running", Log: ""},
}

func (s *AgentState) appendLog(format string, args ...interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Task != nil {
		s.Task.AppendLog(format, args...)
	}
}

// ========== Tool type ==========

// ToolFunc defines a tool available to the LLM agent.
// The Run function receives JSON arguments and returns a JSON result string.
type ToolFunc struct {
	Name        string
	Description string
	Run         func(ctx context.Context, argsJSON string) (string, error)
}

// ========== Tool name constants ==========

const (
	ToolLoadFiles     = "load_files"
	ToolScanTopics    = "scan_topics"
	ToolCompileTopic  = "compile_topic"
	ToolConsistency   = "consistency_review"
	ToolGenerateIndex = "generate_index"
	ToolGenerateLog   = "generate_log"
	ToolQualityReview = "quality_review"
	ToolReadSource    = "read_source_file"
	ToolWriteOutput   = "write_output"
	ToolAppendNote    = "append_mapping_note"
)

func usage(desc string) string {
	return fmt.Sprintf("工具说明：\n%s\n\n调用时传入JSON参数。", desc)
}

// ========== Tool call executor ==========

// executeToolCall finds and executes a tool by name.
func executeToolCall(tools []ToolFunc, name string, argsJSON string) string {
	for _, t := range tools {
		if t.Name == name {
			result, err := t.Run(context.Background(), argsJSON)
			if err != nil {
				return fmt.Sprintf("错误: %v", err)
			}
			return result
		}
	}
	return fmt.Sprintf("未知工具: %s", name)
}

// formatToolDescriptions creates a string listing all available tools for the prompt.
func formatToolDescriptions(tools []ToolFunc) string {
	var b strings.Builder
	for _, t := range tools {
		b.WriteString(fmt.Sprintf("- **%s**: %s", t.Name, t.Description))
		b.WriteString("\n")
	}
	return b.String()
}

// isTaskComplete checks if the agent should stop.
func isTaskComplete(state *AgentState) bool {
	if len(state.Outputs) > 0 {
		hasIndex := false
		hasLog := false
		for _, o := range state.Outputs {
			if o.Path == "INDEX.md" {
				hasIndex = true
			}
			if o.Path == "log.md" {
				hasLog = true
			}
		}
		return hasIndex && hasLog
	}
	return false
}
