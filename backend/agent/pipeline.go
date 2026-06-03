package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"llmwiki/backend/entity"
	"llmwiki/backend/service"
)

// CompileInput is the input to the compilation pipeline.
type CompileInput struct {
	WorkspaceID  string
	TenantID     string
	Actor        string
	Instructions string
	SkillRefs    []string
	OutputDir    string
	CommitMsg    string
}

// CompileResult is the output of the compilation pipeline.
type CompileResult struct {
	TaskID       string
	CommitID     string
	FilesCreated int
	ErrorMessage string
}

// OutputFile represents a single output file to create.
type OutputFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// LLMResult contains the structured output from a single LLM call.
// Used by model.go for parsing multi-file JSON responses.
type LLMResult struct {
	Files []OutputFile `json:"files"`
}

// FileNode represents a file in the workspace tree.
type FileNode struct {
	ID      string
	Name    string
	Path    string
	Content string
}

// SkillDef represents a loaded skill definition.
type SkillDef struct {
	Name    string
	Content string
}

// TopicInfo from Phase 1 scan.
type TopicInfo struct {
	Name        string   `json:"name"`
	SourcePaths []string `json:"source_paths"`
	Description string   `json:"description"`
}

// TaskState tracks a running/tracked compilation task.
type TaskState struct {
	ID         string
	Status     string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Log        string
	Result     *CompileResult
	Error      string
	mu         sync.RWMutex
}

func (t *TaskState) AppendLog(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Log += fmt.Sprintf(format, args...)
}

func (t *TaskState) SetStatus(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = s
}

func (t *TaskState) GetStatus() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

func (t *TaskState) GetLog() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Log
}

// Compiler is the knowledge compilation agent with multi-phase execution.
type Compiler struct {
	fileSvc  *service.FileService
	llm      *LLMClient
	tasks    map[string]*TaskState
	tasksMu  sync.RWMutex
	entityID func() string
}

// NewCompiler creates a new Compiler instance.
func NewCompiler(fileSvc *service.FileService, llm *LLMClient) *Compiler {
	return &Compiler{
		fileSvc:  fileSvc,
		llm:      llm,
		tasks:    make(map[string]*TaskState),
		entityID: entity.NewID,
	}
}

// StartCompile creates a compilation task and starts it in background.
func (c *Compiler) StartCompile(ctx context.Context, input *CompileInput) (*TaskState, error) {
	taskID := c.entityID()
	task := &TaskState{
		ID:        taskID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	c.tasksMu.Lock()
	c.tasks[taskID] = task
	c.tasksMu.Unlock()

	go c.runCompile(task, input)
	return task, nil
}

// GetTask returns the task state by ID.
func (c *Compiler) GetTask(taskID string) *TaskState {
	c.tasksMu.RLock()
	defer c.tasksMu.RUnlock()
	return c.tasks[taskID]
}

// ListTasks returns all tracked tasks.
func (c *Compiler) ListTasks() []*TaskState {
	c.tasksMu.RLock()
	defer c.tasksMu.RUnlock()
	result := make([]*TaskState, 0, len(c.tasks))
	for _, t := range c.tasks {
		result = append(result, t)
	}
	return result
}

// ========== Multi-Phase Compilation Pipeline ==========

func (c *Compiler) runCompile(task *TaskState, input *CompileInput) {
	task.SetStatus("running")
	task.StartedAt = time.Now()
	task.AppendLog("[TASK] ===== Multi-Phase Compilation =====\n")
	task.AppendLog("[TASK] Workspace: %s\n", input.WorkspaceID)
	task.AppendLog("[TASK] Output: %s\n", input.OutputDir)
	task.AppendLog("[TASK] Skills: %v\n", input.SkillRefs)
	task.AppendLog("[TASK] Instructions: %s\n\n", input.Instructions)

	c.llm.SetLogCallback(func(format string, args ...interface{}) {
		task.AppendLog(format, args...)
	})

	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = "synthesis"
	}

	// Phase 1: Load files + load skills
	files, skills := c.loadPhase(task, input)
	if len(files) == 0 {
		return
	}

	// Phase 2: Scan source files to discover topics (small prompt, just summaries)
	topics := c.scanPhase(task, files, skills, input.Instructions)
	if len(topics) == 0 {
		return
	}

	task.AppendLog("\n[SCAN] Discovered %d topics:\n", len(topics))
	for _, t := range topics {
		task.AppendLog("  - %s (%d sources)\n", t.Name, len(t.SourcePaths))
	}

	// Phase 3: Compile each topic article (one LLM call per topic)
	var allOutputs []OutputFile
	for i, topic := range topics {
		article := c.compilePhase(task, topic, files, outputDir)
		if article != nil {
			allOutputs = append(allOutputs, *article)
		}
		task.AppendLog("[COMPILE] Topic %d/%d: '%s' → %s\n", i+1, len(topics), topic.Name, article.Path)
	}

	// Phase 4: Generate INDEX.md + log.md
	index := c.generateIndex(task, topics, len(files), outputDir)
	logFile := c.generateLog(task, topics, len(files), outputDir)
	allOutputs = append(allOutputs, index, logFile)

	task.AppendLog("\n[OUTPUT] Total files to create: %d\n", len(allOutputs))

	// Phase 5: Write output files
	created, outputWkspID, err := c.writeOutputFiles(task, input, outputDir, allOutputs)
	if err != nil {
		task.AppendLog("[ERROR] Write output: %v\n", err)
		task.Error = err.Error()
		task.SetStatus("failed")
		task.FinishedAt = time.Now()
		return
	}
	task.AppendLog("[OUTPUT] Created %d files\n", len(created))

	// Phase 6: Commit
	commitID, err := c.commit(input, outputWkspID)
	if err != nil {
		task.AppendLog("[COMMIT] Warning: %v\n", err)
	} else {
		task.AppendLog("[COMMIT] ID: %s\n", commitID)
	}

	task.AppendLog("[TASK] ===== Complete =====\n")
	task.Result = &CompileResult{
		FilesCreated: len(created),
		CommitID:     commitID,
	}
	task.SetStatus("success")
	task.FinishedAt = time.Now()
}

// loadPhase loads workspace files and skills.
func (c *Compiler) loadPhase(task *TaskState, input *CompileInput) ([]FileNode, []SkillDef) {
	task.AppendLog("[LOAD] Loading workspace files...\n")
	files, err := c.loadWorkspaceFiles(input.WorkspaceID)
	if err != nil {
		task.AppendLog("[ERROR] Load files: %v\n", err)
		task.Error = err.Error()
		task.SetStatus("failed")
		task.FinishedAt = time.Now()
		return nil, nil
	}
	task.AppendLog("[LOAD] %d files loaded\n", len(files))

	task.AppendLog("[LOAD] Loading skills...\n")
	skills, err := c.loadSkills(input.WorkspaceID, input.SkillRefs)
	if err != nil {
		task.AppendLog("[WARN] Load skills: %v\n", err)
	}
	task.AppendLog("[LOAD] %d skills loaded\n\n", len(skills))
	return files, skills
}

// scanPhase sends file summaries to discover topics.
func (c *Compiler) scanPhase(task *TaskState, files []FileNode, skills []SkillDef, instructions string) []TopicInfo {
	task.AppendLog("[SCAN] Analyzing %d source files to discover topics...\n", len(files))

	// Build a compact summary prompt (just paths + first 300 chars)
	var b strings.Builder
	b.WriteString("以下是要编译的源文件列表。请分析它们并发现主题。\n\n")
	for _, f := range files {
		summary := f.Content
		if len(summary) > 300 {
			summary = summary[:300] + "..."
		}
		b.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", f.Path, summary))
	}

	if instructions != "" {
		b.WriteString(fmt.Sprintf("## 用户指令\n\n%s\n", instructions))
	}

	b.WriteString("\n请以 JSON 格式输出你发现的主题列表：\n")
	b.WriteString(`{"topics": [{"name": "topic-slug", "source_paths": ["file1.md"], "description": "简要描述"}]}`)

	systemMsg := "你是一个知识编译专家。分析下面的源文件摘要，发现其中的独立主题。\n每个主题应该是一组相关的文件集合。主题名用 kebab-case。"

	ctx := context.Background()
	task.AppendLog("[SCAN] Calling LLM with %d file summaries...\n", len(files))
	content, err := c.llm.ChatRaw(ctx, systemMsg, b.String())
	if err != nil {
		task.AppendLog("[SCAN] LLM error: %v\n", err)
		task.AppendLog("[SCAN] Falling back: treating entire workspace as one topic\n")
		return c.fallbackTopics(files)
	}

	var result struct {
		Topics []TopicInfo `json:"topics"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil || len(result.Topics) == 0 {
		task.AppendLog("[SCAN] Could not parse topics from LLM response, using fallback\n")
		return c.fallbackTopics(files)
	}

	return result.Topics
}

func (c *Compiler) fallbackTopics(files []FileNode) []TopicInfo {
	// Group by directory prefix
	groups := make(map[string][]string)
	for _, f := range files {
		dir := ""
		if idx := strings.LastIndex(f.Path, "/"); idx > 0 {
			dir = f.Path[:idx]
		} else {
			dir = "root"
		}
		groups[dir] = append(groups[dir], f.Path)
	}
	var topics []TopicInfo
	for dir, paths := range groups {
		name := strings.ReplaceAll(dir, "/", "-")
		if name == "" {
			name = "topic"
		}
		topics = append(topics, TopicInfo{
			Name:        name,
			SourcePaths: paths,
			Description: fmt.Sprintf("Files in %s", dir),
		})
	}
	return topics
}

// compilePhase compiles a single topic article.
func (c *Compiler) compilePhase(task *TaskState, topic TopicInfo, allFiles []FileNode, outputDir string) *OutputFile {
	task.AppendLog("[COMPILE] Compiling topic '%s' (%d sources)...\n", topic.Name, len(topic.SourcePaths))

	// Collect relevant files for this topic
	var relevantFiles []FileNode
	topicPaths := make(map[string]bool)
	for _, p := range topic.SourcePaths {
		topicPaths[p] = true
	}
	for _, f := range allFiles {
		if topicPaths[f.Path] {
			relevantFiles = append(relevantFiles, f)
		}
	}

	// Build compact prompt with full content of relevant files
	var b strings.Builder
	b.WriteString(fmt.Sprintf("编译主题「%s」的知识文章。\n\n", topic.Name))
	for _, f := range relevantFiles {
		b.WriteString(fmt.Sprintf("### %s\n```\n%s\n```\n\n", f.Path, f.Content))
	}

	b.WriteString(fmt.Sprintf("输出到文件: %s.md\n", topic.Name))
	b.WriteString("\n输出 JSON 格式: " + `{"path": "topic-name.md", "content": "# Title\n\n..."}`)

	systemMsg := `你正在编译一篇知识文章。严格遵循以下要求：

1. 合成多个源文件的信息，不要只照搬一个源
2. 用自己的语言重新组织内容
3. 正文中用 [[OtherArticle]] 格式交叉引用相关主题
4. 标记覆盖度: [coverage: high/medium/low]
5. 输出纯 JSON`

	ctx := context.Background()
	content, err := c.llm.ChatRaw(ctx, systemMsg, b.String())
	if err != nil {
		task.AppendLog("[COMPILE] LLM error: %v\n", err)
		return nil
	}

	var of OutputFile
	if err := json.Unmarshal([]byte(content), &of); err != nil || of.Path == "" || of.Content == "" {
		// Fallback: wrap raw content
		return &OutputFile{
			Path:    topic.Name + ".md",
			Content: content,
		}
	}
	return &of
}

// generateIndex creates INDEX.md.
func (c *Compiler) generateIndex(task *TaskState, topics []TopicInfo, fileCount int, outputDir string) OutputFile {
	today := time.Now().Format("2006-01-02")

	var b strings.Builder
	b.WriteString("# Knowledge Base\n\n")
	b.WriteString(fmt.Sprintf("最后编译: %s\n", today))
	b.WriteString(fmt.Sprintf("主题数: %d | 源文件: %d\n\n", len(topics), fileCount))
	b.WriteString("## 主题\n\n")
	b.WriteString("| 主题 | 来源 |\n")
	b.WriteString("|------|------|\n")
	for _, t := range topics {
		b.WriteString(fmt.Sprintf("| [[%s]] | %d |\n", t.Name, len(t.SourcePaths)))
	}
	b.WriteString("\n## 最近变更\n")
	b.WriteString(fmt.Sprintf("- %s: 知识编译\n", today))

	return OutputFile{
		Path:    "INDEX.md",
		Content: b.String(),
	}
}

// generateLog creates log.md.
func (c *Compiler) generateLog(task *TaskState, topics []TopicInfo, fileCount int, outputDir string) OutputFile {
	today := time.Now().Format("2006-01-02")
	var names []string
	for _, t := range topics {
		names = append(names, t.Name)
	}

	content := fmt.Sprintf("## %s\n\n**创建的主题:** %s\n**处理的源文件:** %d\n",
		today, strings.Join(names, ", "), fileCount)

	return OutputFile{
		Path:    "log.md",
		Content: content,
	}
}

// ========== File & Skills Loading (unchanged) ==========

func (c *Compiler) loadWorkspaceFiles(workspaceID string) ([]FileNode, error) {
	tree, err := c.fileSvc.GetCurrentTree(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("get tree: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("workspace not found")
	}
	var nodes []FileNode
	walkTree(tree, "", &nodes, c.fileSvc, workspaceID)
	return nodes, nil
}

func walkTree(node *entity.TreeNode, parentPath string, nodes *[]FileNode, svc *service.FileService, workspaceID string) {
	for i := range node.Children {
		child := node.Children[i]
		relPath := child.Name
		if parentPath != "" {
			relPath = parentPath + "/" + child.Name
		}
		if child.Type == "folder" {
			walkTree(&child, relPath, nodes, svc, workspaceID)
		} else {
			var content string
			if child.Location != nil && *child.Location != "" {
				data, err := svc.GetStorageData(workspaceID, *child.Location)
				if err == nil {
					content = string(data)
				}
			}
			if content == "" {
				content = "[empty]"
			}
			*nodes = append(*nodes, FileNode{
				ID:      child.ID,
				Name:    child.Name,
				Path:    relPath,
				Content: content,
			})
		}
	}
}

func (c *Compiler) loadSkills(workspaceID string, skillRefs []string) ([]SkillDef, error) {
	// Try workspace skills folder first
	tree, err := c.fileSvc.GetCurrentTree(workspaceID)
	if err == nil && tree != nil {
		skillsFolderID := findFolderNamed(tree, ".knowledgebase")
		if skillsFolderID == "" {
			skillsFolderID = findFolderNamed(tree, "skills")
		}
		if skillsFolderID != "" {
			skillsTree, _ := c.fileSvc.GetCurrentTree(skillsFolderID)
			if skillsTree != nil {
				skills := collectSkills(skillsTree, workspaceID, skillRefs, c.fileSvc)
				if len(skills) > 0 {
					return skills, nil
				}
			}
		}
	}
	// Fallback to filesystem
	return loadLocalSkills()
}

func collectSkills(skillsTree *entity.TreeNode, workspaceID string, skillRefs []string, svc *service.FileService) []SkillDef {
	var skills []SkillDef
	match := func(name string) bool {
		if len(skillRefs) == 0 {
			return true
		}
		for _, ref := range skillRefs {
			if name == ref {
				return true
			}
		}
		return false
	}
	for _, child := range skillsTree.Children {
		if child.Type == "file" && match(child.Name) && child.Location != nil && *child.Location != "" {
			data, err := svc.GetStorageData(workspaceID, *child.Location)
			if err == nil {
				skills = append(skills, SkillDef{Name: child.Name, Content: string(data)})
			}
		}
	}
	return skills
}

func findFolderNamed(node *entity.TreeNode, name string) string {
	for _, child := range node.Children {
		if child.Type == "folder" && child.Name == name {
			return child.ID
		}
		if child.Type == "folder" {
			if id := findFolderNamed(&child, name); id != "" {
				return id
			}
		}
	}
	return ""
}

func loadLocalSkills() ([]SkillDef, error) {
	candidates := []string{"skills", "../skills", "skills/wiki-compiler", "../skills/wiki-compiler"}
	for _, dir := range candidates {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var skills []SkillDef
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".txt")) {
				data, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				skills = append(skills, SkillDef{Name: e.Name(), Content: string(data)})
			}
		}
		if len(skills) > 0 {
			return skills, nil
		}
	}
	return nil, nil
}

// ========== Output Writing & Commit (unchanged) ==========

func (c *Compiler) writeOutputFiles(task *TaskState, input *CompileInput, outputDir string, files []OutputFile) ([]string, string, error) {
	srcWorkspace, err := c.fileSvc.GetFileByID(input.WorkspaceID)
	if err != nil {
		return nil, "", fmt.Errorf("get source workspace: %w", err)
	}
	rootFolder, err := c.fileSvc.GetOrCreateRootFolder(input.TenantID, input.Actor)
	if err != nil {
		return nil, "", fmt.Errorf("get root folder: %w", err)
	}

	outputName := srcWorkspace.Name + "-" + outputDir
	outputWS, err := c.fileSvc.CreateFolder(input.TenantID, rootFolder.ID, outputName, input.Actor)
	if err != nil {
		task.AppendLog("[OUTPUT] Folder '%s' may already exist: %v\n", outputName, err)
		rootTree, _ := c.fileSvc.GetCurrentTree(rootFolder.ID)
		outputWS = findChildFolderByName(rootTree, outputName)
	}
	if outputWS == nil {
		return nil, "", fmt.Errorf("cannot create output workspace '%s'", outputName)
	}
	task.AppendLog("[OUTPUT] Workspace: '%s' (id=%s)\n", outputName, outputWS.ID)

	var created []string
	for _, f := range files {
		name := strings.TrimPrefix(f.Path, outputDir+"/")
		name = strings.TrimPrefix(name, outputDir+"\\")
		if name == "" {
			name = f.Path
		}
		task.AppendLog("[OUTPUT] Creating: %s\n", name)
		fileRec, err := c.fileSvc.CreateTextFile(input.TenantID, outputWS.ID, name, f.Content, input.Actor)
		if err != nil {
			task.AppendLog("[OUTPUT] Warning: %v\n", err)
			continue
		}
		created = append(created, fileRec.ID)
	}
	return created, outputWS.ID, nil
}

func (c *Compiler) commit(input *CompileInput, outputWorkspaceID string) (string, error) {
	msg := input.CommitMsg
	if msg == "" {
		msg = "Agent multi-phase knowledge compilation"
	}
	commit, err := c.fileSvc.CreateCommit(outputWorkspaceID, input.Actor, msg, nil)
	if err != nil {
		return "", err
	}
	return commit.ID, nil
}

func findChildFolderByName(node *entity.TreeNode, name string) *entity.File {
	for _, child := range node.Children {
		if child.Type == "folder" && child.Name == name {
			return &entity.File{ID: child.ID}
		}
	}
	return nil
}



