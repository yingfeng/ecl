package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// ========== Agent Loop: Claude Code-style ==========
// The LLM reads SKILL.md as system prompt, then autonomously decides
// which tools to call and in what order.

// AgentConfig configures the agent loop.
type AgentConfig struct {
	LLM          *LLMClient
	WorkspaceID  string
	SkillContent string // loaded SKILL.md content
	Instructions string // user-provided instructions
	OutputDir    string
	CP           *CheckpointManager // optional, for checkpoint resume
}

// AgentResult holds the final output of an agent run.
type AgentResult struct {
	Outputs []OutputFile
	Log     string
	Error   string
}

// RunAgentLoop executes the agent with the given config.
// Supports checkpoint resume: if cfg.CP is set and a checkpoint exists,
// restores messages and state from the last saved turn.
func RunAgentLoop(ctx context.Context, cfg *AgentConfig) (*AgentResult, error) {
	start := time.Now()

	// 1. Build system prompt from SKILL.md
	systemPrompt := buildSystemPrompt(cfg)

	// 2. Create tool instances and collect OpenAI Tool definitions
	compiler := &Compiler{}
	agentTools := createAgentTools(compiler)
	openaiTools := buildOpenAITools(agentTools)

	// 3. Try checkpoint resume
	maxTurns := 30
	turnCount := 0
	autoExecCount := 0

	userContent := cfg.Instructions
	if userContent == "" {
		userContent = "请按照上述工作流程，逐步执行知识编译任务。"
	}

	var messages []openai.ChatCompletionMessage

	if cfg.CP != nil {
		if restoredTurn, restoredMsgs, restoredState, err := cfg.CP.LoadAgentCheckpoint(); err == nil && restoredState != nil {
			turnCount = restoredTurn
			messages = restoredMsgs
			globalState = restoredState
			globalState.appendLog("[CP] Resumed from turn %d\n", restoredTurn)
		}
	}

	if messages == nil {
		// No checkpoint restored — start fresh
		globalState = &AgentState{
			Input: &CompileInput{WorkspaceID: cfg.WorkspaceID},
			Task:  &TaskState{ID: "agent", Status: "running", Log: ""},
		}
		messages = []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userContent},
		}
	}

	// 4. Save checkpoint function
	saveCP := func() {
		if cfg.CP != nil {
			if err := cfg.CP.SaveAgentCheckpoint(turnCount, messages, globalState); err != nil {
				globalState.appendLog("[CP] Save error: %v\n", err)
			}
		}
	}

	for turnCount < maxTurns {
		turnCount++
		globalState.appendLog("[TURN %d] LLM reasoning...\n", turnCount)

		// Call LLM with tool definitions
		resp, err := cfg.LLM.ChatWithToolCalls(ctx, messages, openaiTools)
		if err != nil {
			globalState.appendLog("[ERROR] LLM call failed: %v\n", err)
			break
		}

		if len(resp.Choices) == 0 {
			globalState.appendLog("[ERROR] No choices in response\n")
			break
		}

		choice := resp.Choices[0].Message

		if len(choice.ToolCalls) > 0 {
			// LLM returned structured tool calls via function calling
			assistantMsg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   choice.Content,
				ToolCalls: choice.ToolCalls,
			}
			messages = append(messages, assistantMsg)
			globalState.appendLog("[TURN %d] LLM called %d tool(s)\n", turnCount, len(choice.ToolCalls))

			for _, tc := range choice.ToolCalls {
				toolName := tc.Function.Name
				argsJSON := tc.Function.Arguments
				globalState.appendLog("  -> tool: %s\n", toolName)

				result := executeToolCall(agentTools, toolName, argsJSON)
				globalState.appendLog("  <- result: %s\n", truncate(result, 100))

				toolMsg := openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					Name:       toolName,
					ToolCallID: tc.ID,
				}
				messages = append(messages, toolMsg)
			}

			// Save checkpoint after each tool execution cycle
			saveCP()

			if isTaskComplete(globalState) {
				globalState.appendLog("[DONE] Task appears complete\n")
				break
			}
		} else {
			// No tool calls — LLM responded in text
			globalState.appendLog("[TURN %d] LLM response: %s\n", turnCount, truncate(choice.Content, 200))
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: choice.Content,
			})

			// Completion check
			if isCompletionText(choice.Content) {
				globalState.appendLog("[DONE] LLM indicated completion\n")
				break
			}

			// Auto-execute: detect tool call intent in text and skip next decision round
			if toolName, argsJSON, found := parseToolIntent(choice.Content); found && autoExecCount < 3 {
				autoExecCount++
				globalState.appendLog("[AUTO] Detected '%s' in text, executing directly\n", toolName)
				result := executeToolCall(agentTools, toolName, argsJSON)
				globalState.appendLog("  <- auto result: %s\n", truncate(result, 100))

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					Name:       toolName,
					ToolCallID: "auto_" + toolName,
				})

				// Save checkpoint after auto-execution
				saveCP()

				if isTaskComplete(globalState) {
					break
				}
				continue
			}

			// Guard: if no progress after several text-only turns, stop
			if turnCount > 4 && len(globalState.Outputs) == 0 {
				globalState.appendLog("[GUARD] No progress after %d turns, stopping\n", turnCount)
				break
			}
		}

		// 5. Context compression: prevent unbounded message growth
		messages = compressContext(messages, 24)

		// Save checkpoint after compression (reduced size)
		saveCP()
	}

	elapsed := time.Since(start)
	globalState.appendLog("[TASK] Completed in %v (%d turns)\n", elapsed, turnCount)

	// Delete checkpoint on successful completion
	if cfg.CP != nil {
		cfg.CP.Delete()
	}

	// Collect log from global state
	var logText string
	if globalState.Task != nil {
		logText = globalState.Task.GetLog()
	}

	return &AgentResult{
		Outputs: globalState.Outputs,
		Log:     logText,
	}, nil
}

// buildOpenAITools converts ToolFunc slice to OpenAI Tool slice.
func buildOpenAITools(tools []ToolFunc) []openai.Tool {
	result := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		result = append(result, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
			},
		})
	}
	return result
}

// createAgentTools builds the list of tools available to the LLM.
func createAgentTools(compiler *Compiler) []ToolFunc {
	return []ToolFunc{
		{
			Name: "load_files",
			Description: usage("加载工作区文件。需要 workspace_id。返回文件摘要。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				var args struct {
					WorkspaceID string `json:"workspace_id"`
				}
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return "", fmt.Errorf("parse args: %w", err)
				}
				globalState.Input.WorkspaceID = args.WorkspaceID

				input := &CompileInput{WorkspaceID: args.WorkspaceID}
				files, skills := compiler.loadPhase(globalState.Task, input)
				globalState.Files = files
				globalState.Skills = skills

				result := map[string]any{
					"file_count":  len(files),
					"skill_count": len(skills),
				}
				data, _ := json.Marshal(result)
				return string(data), nil
			},
		},
		{
			Name: "read_file",
			Description: usage("读取指定源文件的完整内容。需要 file_path。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				var args struct {
					FilePath string `json:"file_path"`
				}
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return "", fmt.Errorf("parse args: %w", err)
				}
				for _, f := range globalState.Files {
					if strings.Contains(f.Path, args.FilePath) || f.Path == args.FilePath {
						return fmt.Sprintf("---\npath: %s\ntitle: %s\ncontent:\n%s\n---",
							f.Path, f.Title, f.Content), nil
					}
				}
				return fmt.Sprintf("未找到: %s", args.FilePath), nil
			},
		},
		{
			Name: "extract_keywords",
			Description: usage("从已加载文件中提取关键词。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				compiler.extractKeywords(globalState.Task, globalState.Files)
				return fmt.Sprintf("已提取 %d 个文件的关键词", len(globalState.Files)), nil
			},
		},
		{
			Name: "scan_topics",
			Description: usage("扫描文件发现知识主题。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				topics := compiler.scanPhase(globalState.Task, globalState.Files, globalState.Skills, globalState.Input.Instructions)
				globalState.Topics = topics
				data, _ := json.MarshalIndent(topics, "", "  ")
				return fmt.Sprintf("发现 %d 个主题:\n%s", len(topics), string(data)), nil
			},
		},
		{
			Name: "compile_topic",
			Description: usage("编译单个主题的知识文章。需要 topic_name。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				var args struct {
					TopicName string `json:"topic_name"`
				}
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return "", fmt.Errorf("parse args: %w", err)
				}

				for i := range globalState.Topics {
					if globalState.Topics[i].Name == args.TopicName {
						article := compiler.compilePhase(globalState.Task, globalState.Topics[i],
							globalState.Files, globalState.Topics, globalState.Outputs, "synthesis", globalState.Skills, "wiki", "")
						if article != nil {
							globalState.Outputs = append(globalState.Outputs, *article)
							return fmt.Sprintf("已编译: %s (%d chars)", article.Path, len(article.Content)), nil
						}
						return fmt.Sprintf("编译失败: %s", args.TopicName), nil
					}
				}
				return fmt.Sprintf("未找到主题: %s", args.TopicName), nil
			},
		},
		{
			Name: "consistency_review",
			Description: usage("对所有已编译文章进行一致性审查。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				if len(globalState.Outputs) == 0 {
					return "没有已编译的文章", nil
				}
				globalState.Outputs = compiler.consistencyReview(globalState.Task, globalState.Outputs)
				return fmt.Sprintf("已审查 %d 篇文章", len(globalState.Outputs)), nil
			},
		},
		{
			Name: "generate_index",
			Description: usage("生成 INDEX.md 索引文件。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				index := generateDomainIndex(nil, globalState.Outputs)
				globalState.Outputs = append(globalState.Outputs, index)
				return fmt.Sprintf("已生成 INDEX.md (%d chars)", len(index.Content)), nil
			},
		},
		{
			Name: "generate_log",
			Description: usage("生成 log.md 编译日志。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				log := compiler.generateLog(globalState.Task, globalState.Topics, len(globalState.Files), "synthesis")
				globalState.Outputs = append(globalState.Outputs, log)
				return "已生成 log.md", nil
			},
		},
		{
			Name: "quality_review",
			Description: usage("质量审查：统计链接、孤立文章、覆盖度。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				globalState.Outputs = compiler.qualityReview(globalState.Task, globalState.Topics, globalState.Outputs)
				return fmt.Sprintf("已审查 %d 篇文章", len(globalState.Outputs)), nil
			},
		},
		{
			Name: "write_output",
			Description: usage("保存编译结果。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				return fmt.Sprintf("输出 %d 个文件:\n%s", len(globalState.Outputs), outputPaths(globalState.Outputs)), nil
			},
		},
		{
			Name: "search_files",
			Description: usage("在已加载文件中搜索关键词。需要 query。"),
			Run: func(ctx context.Context, argsJSON string) (string, error) {
				var args struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					return "", fmt.Errorf("parse args: %w", err)
				}
				var matches []string
				for _, f := range globalState.Files {
					if strings.Contains(f.Content, args.Query) {
						matches = append(matches, f.Path)
					}
				}
				if len(matches) == 0 {
					return "未找到匹配", nil
				}
				return fmt.Sprintf("找到 %d 个:\n%s", len(matches), strings.Join(matches, "\n")), nil
			},
		},
	}
}

// buildSystemPrompt constructs the agent's system prompt from SKILL.md.
func buildSystemPrompt(cfg *AgentConfig) string {
	var b strings.Builder
	b.WriteString("你是一个知识编译 Agent。你的工作流程由以下 SKILL.md 定义：\n\n")

	if cfg.SkillContent != "" {
		b.WriteString(cfg.SkillContent)
	} else {
		b.WriteString("[无已加载技能]\n")
	}

	b.WriteString("\n\n## 工作方式\n\n")
	b.WriteString("1. 调用 load_files 加载工作区\n")
	b.WriteString("2. 调用 scan_topics 发现知识主题\n")
	b.WriteString("3. 对每个主题调用 compile_topic 编译文章\n")
	b.WriteString("4. 调用 generate_index 和 generate_log\n")
	b.WriteString("5. 调用 write_output 保存结果\n")

	return b.String()
}

// StartAgentCompile starts a skill-driven agent compilation.
func (c *Compiler) StartAgentCompile(ctx context.Context, input *CompileInput) (*TaskState, error) {
	taskID := c.entityID()
	task := &TaskState{
		ID:        taskID,
		Status:    "running",
		CreatedAt: time.Now(),
	}
	c.tasksMu.Lock()
	c.tasks[taskID] = task
	c.tasksMu.Unlock()

	go func() {
		_, skills := c.loadPhase(task, input)
		var skillContent string
		for _, s := range skills {
			skillContent += fmt.Sprintf("### Skill: %s\n\n%s\n\n", s.Name, s.Content)
		}

		cp := NewCheckpointManager(c.rdb, taskID)

		cfg := &AgentConfig{
			LLM:          c.llm,
			WorkspaceID:  input.WorkspaceID,
			SkillContent: skillContent,
			Instructions: input.Instructions,
			OutputDir:    input.OutputDir,
			CP:           cp,
		}

		result, err := RunAgentLoop(ctx, cfg)
		if err != nil {
			task.SetStatus("failed")
			task.Error = err.Error()
			task.FinishedAt = time.Now()
			return
		}

		task.AppendLog("%s", result.Log)

		for _, o := range result.Outputs {
			task.AppendLog("[OUTPUT] %s\n", o.Path)
		}
		task.Result = &CompileResult{
			TaskID:       taskID,
			FilesCreated: len(result.Outputs),
		}
		task.SetStatus("success")
		task.FinishedAt = time.Now()
	}()

	return task, nil
}

// outputPaths returns the paths of all output files.
func outputPaths(outputs []OutputFile) string {
	var paths []string
	for _, o := range outputs {
		paths = append(paths, o.Path)
	}
	return strings.Join(paths, "\n")
}

// ========== Context Compression ==========

// compressContext trims message history to prevent unbounded prompt growth.
// Keeps the system message + the most recent N message pairs.
func compressContext(msgs []openai.ChatCompletionMessage, maxLen int) []openai.ChatCompletionMessage {
	if len(msgs) <= maxLen {
		return msgs
	}
	// Always keep system message (index 0)
	compressed := []openai.ChatCompletionMessage{msgs[0]}
	// Keep recent messages
	start := len(msgs) - (maxLen - 1)
	if start < 1 {
		start = 1
	}
	compressed = append(compressed, msgs[start:]...)
	return compressed
}

// ========== Auto-Execution ==========

var toolIntentPatterns = []struct {
	name  string
	check func(string) (string, string, bool)
}{
	{name: "load_files", check: func(s string) (string, string, bool) {
		if containsAny(s, "load_files", "先加载工作区", "首先加载") {
			return "load_files", `{"workspace_id":""}`, true
		}
		return "", "", false
	}},
	{name: "scan_topics", check: func(s string) (string, string, bool) {
		if containsAny(s, "scan_topics", "扫描主题", "发现主题", "开始扫描") {
			return "scan_topics", "{}", true
		}
		return "", "", false
	}},
	{name: "generate_index", check: func(s string) (string, string, bool) {
		if containsAny(s, "generate_index", "生成 INDEX", "创建索引", "INDEX.md") {
			return "generate_index", "{}", true
		}
		return "", "", false
	}},
	{name: "generate_log", check: func(s string) (string, string, bool) {
		if containsAny(s, "generate_log", "生成日志", "log.md") {
			return "generate_log", "{}", true
		}
		return "", "", false
	}},
	{name: "consistency_review", check: func(s string) (string, string, bool) {
		if containsAny(s, "consistency_review", "一致性审查", "交叉引用检查") {
			return "consistency_review", "{}", true
		}
		return "", "", false
	}},
	{name: "quality_review", check: func(s string) (string, string, bool) {
		if containsAny(s, "quality_review", "质量审查", "质量检查") {
			return "quality_review", "{}", true
		}
		return "", "", false
	}},
}

// parseToolIntent detects an LLM's intent to call a tool from text.
// Returns (toolName, argsJSON, found).
func parseToolIntent(content string) (string, string, bool) {
	for _, p := range toolIntentPatterns {
		if name, args, ok := p.check(content); ok {
			return name, args, true
		}
	}
	// Generic pattern: "调用了 X" / "调用 X" / "执行 X"
	return "", "", false
}

// isCompletionText checks if the LLM is indicating task completion.
func isCompletionText(content string) bool {
	return containsAny(content,
		"完成", "DONE", "complete",
		"任务完成", "编译完成", "全部完成",
		"以上就是所有", "已全部",
	)
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
