package gbrain

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"llmwiki/backend/agent"
	"llmwiki/backend/entity"
	"llmwiki/backend/service"

	"github.com/go-redis/redis/v8"
)

// GbrainCompiler orchestrates the gbrain-style cycle compilation.
// It wraps the standard agent compiler and adds post-compilation phases:
//   - Fence-based structured knowledge (facts/takes)
//   - Cross-session pattern discovery
//   - Fact consolidation
type GbrainCompiler struct {
	agentComp  *agent.Compiler
	fileSvc    *service.FileService
	llm        *agent.LLMClient
	rdb        *redis.Client

	cycles   map[string]*GbrainTask
	cyclesMu sync.RWMutex
}

func NewGbrainCompiler(agentComp *agent.Compiler, fileSvc *service.FileService, llm *agent.LLMClient, rdb *redis.Client) *GbrainCompiler {
	return &GbrainCompiler{
		agentComp: agentComp,
		fileSvc:   fileSvc,
		llm:       llm,
		rdb:       rdb,
		cycles:    make(map[string]*GbrainTask),
	}
}

// StartCycle begins a gbrain-style compilation cycle.
// It runs the standard agent pipeline first, then adds gbrain-specific phases.
// force=true bypasses the cooldown check.
func (g *GbrainCompiler) StartCycle(ctx context.Context, input *agent.CompileInput, force bool) (*GbrainTask, error) {
	// Check cooldown
	if !force && !g.checkCooldown(input.WorkspaceID) {
		return nil, fmt.Errorf("gbrain cycle cooling down for workspace %s (use ?force=1 to bypass)", input.WorkspaceID)
	}

	taskID := entity.NewID()
	task := &GbrainTask{
		ID:          taskID,
		WorkspaceID: input.WorkspaceID,
		Status:      CyclePending,
		CreatedAt:   time.Now(),
	}
	g.cyclesMu.Lock()
	g.cycles[taskID] = task
	g.cyclesMu.Unlock()

	task.SetStatus(CycleRunning)
	task.StartedAt = time.Now()
	task.AppendLog("[GBRAIN] ===== Gbrain Cycle Started =====\n")
	task.AppendLog("[GBRAIN] Workspace: %s, Output: %s\n", input.WorkspaceID, input.OutputDir)

	// Run cycle in background
	go g.runCycle(task, input)

	return task, nil
}

// GenerateReport skips the agent compile phase and generates only the gbrain report
// from existing compiled output. Useful when the agent compile was already run.
func (g *GbrainCompiler) GenerateReport(ctx context.Context, input *agent.CompileInput) (*GbrainTask, error) {
	taskID := entity.NewID()
	task := &GbrainTask{
		ID:          taskID,
		WorkspaceID: input.WorkspaceID,
		Status:      CyclePending,
		CreatedAt:   time.Now(),
	}
	g.cyclesMu.Lock()
	g.cycles[taskID] = task
	g.cyclesMu.Unlock()

	task.SetStatus(CycleRunning)
	task.StartedAt = time.Now()
	task.AppendLog("[GBRAIN] ===== Gbrain Report-Only Mode =====\n")
	task.AppendLog("[GBRAIN] Skipping agent compile, reading existing output...\n")

	go g.runReportOnly(task, input)

	return task, nil
}

func (g *GbrainCompiler) runCycle(task *GbrainTask, input *agent.CompileInput) {
	defer func() {
		if r := recover(); r != nil {
			task.AppendLog("[PANIC] %v\n", r)
			task.Error = fmt.Sprintf("panic: %v", r)
			task.SetStatus(CycleFailed)
			task.FinishedAt = time.Now()
		}
	}()

	task.AppendLog("[GBRAIN] Phase 1-5: Running standard agent compilation...\n")

	// ── Phase 1-5: Standard agent compilation ──
	agentTask, err := g.agentComp.StartCompile(context.Background(), input)
	if err != nil {
		task.AppendLog("[AGENT] Failed to start compile: %v\n", err)
		task.Error = err.Error()
		task.SetStatus(CycleFailed)
		task.FinishedAt = time.Now()
		return
	}
	task.AgentTaskID = agentTask.ID
	task.AppendLog("[AGENT] Sub-task: %s\n", agentTask.ID)

	// Wait for agent compile to complete (polling)
	task.AppendLog("[GBRAIN] Waiting for agent compilation...\n")
	for {
		time.Sleep(2 * time.Second)
		agentTask = g.agentComp.GetTask(agentTask.ID)
		if agentTask == nil {
			task.AppendLog("[AGENT] Task not found\n")
			task.Error = "agent sub-task lost"
			task.SetStatus(CycleFailed)
			task.FinishedAt = time.Now()
			return
		}
		if agentTask.GetStatus() == "success" {
			break
		}
		if agentTask.GetStatus() == "failed" {
			task.AppendLog("[AGENT] Agent compilation failed: %s\n", agentTask.Error)
			task.Error = "agent compilation: " + agentTask.Error
			task.SetStatus(CycleFailed)
			task.FinishedAt = time.Now()
			return
		}
	}
	task.AppendLog("[AGENT] Agent compilation completed successfully\n")

	// ── Read compiled articles from output workspace ──
	task.AppendLog("[GBRAIN] Reading compiled articles...\n")
	articles, err := g.readOutputArticles(input)
	if err != nil {
		task.AppendLog("[GBRAIN] Failed to read output: %v\n", err)
		task.Error = err.Error()
		task.SetStatus(CycleFailed)
		task.FinishedAt = time.Now()
		return
	}

	if len(articles) == 0 {
		task.AppendLog("[GBRAIN] No articles to process, cycle completed\n")
		task.SetStatus(CycleSuccess)
		task.FinishedAt = time.Now()
		g.setCooldown(input.WorkspaceID)
		return
	}
	task.AppendLog("[GBRAIN] %d articles loaded\n", len(articles))

	// ── Phase 6: Structured knowledge extraction ──
	task.AppendLog("[GBRAIN] ---- Phase 6: Extract Facts & Takes ----\n")
	facts, takes := g.extractFactsAndTakes(task, articles)

	// ── Phase 7: Cross-session pattern discovery ──
	task.AppendLog("[GBRAIN] ---- Phase 7: Pattern Discovery ----\n")
	patterns := g.discoverPatterns(task, articles)

	// ── Phase 8: Fact consolidation ──
	task.AppendLog("[GBRAIN] ---- Phase 8: Consolidate Facts → Takes ----\n")
	consolidatedTakes := g.consolidateFacts(task, facts, takes)
	takes = append(takes, consolidatedTakes...)

	// ── Phase 9: Write enhanced output ──
	task.AppendLog("[GBRAIN] ---- Phase 9: Writing Enhanced Output ----\n")
	g.writeEnhancedOutput(task, input, articles, facts, takes, patterns)

	// ── Complete ──
	task.SetStatus(CycleSuccess)
	task.FinishedAt = time.Now()
	g.setCooldown(input.WorkspaceID)
	task.AppendLog("[GBRAIN] ===== Gbrain Cycle Complete =====\n")
	task.AppendLog("Articles: %d new + %d updated | Facts: %d | Takes: %d | Patterns: %d | Consolidated: %d\n",
		task.NewArticles, task.UpdatedArticles, task.FactsExtracted, task.TakesExtracted, task.PatternsFound, task.TakesConsolidated)
}

// runReportOnly skips agent compile, reads existing output, runs gbrain phases 6-9.
func (g *GbrainCompiler) runReportOnly(task *GbrainTask, input *agent.CompileInput) {
	defer func() {
		if r := recover(); r != nil {
			task.AppendLog("[PANIC] %v\n", r)
			task.Error = fmt.Sprintf("panic: %v", r)
			task.SetStatus(CycleFailed)
			task.FinishedAt = time.Now()
		}
	}()

	task.AppendLog("[GBRAIN] Reading existing compiled output...\n")
	articles, err := g.readOutputArticles(input)
	if err != nil {
		task.AppendLog("[GBRAIN] Failed to read output: %v\n", err)
		task.Error = err.Error()
		task.SetStatus(CycleFailed)
		task.FinishedAt = time.Now()
		return
	}

	task.AppendLog("[GBRAIN] %d articles loaded\n", len(articles))
	if len(articles) == 0 {
		task.AppendLog("[GBRAIN] No articles found, nothing to do\n")
		task.SetStatus(CycleSuccess)
		task.FinishedAt = time.Now()
		return
	}

	task.AppendLog("[GBRAIN] ---- Phase 6: Extract Facts & Takes ----\n")
	facts, takes := g.extractFactsAndTakes(task, articles)

	task.AppendLog("[GBRAIN] ---- Phase 7: Pattern Discovery ----\n")
	patterns := g.discoverPatterns(task, articles)

	task.AppendLog("[GBRAIN] ---- Phase 8: Consolidate Facts → Takes ----\n")
	consolidatedTakes := g.consolidateFacts(task, facts, takes)
	takes = append(takes, consolidatedTakes...)

	task.AppendLog("[GBRAIN] ---- Phase 9: Writing Report ----\n")
	g.writeEnhancedOutput(task, input, articles, facts, takes, patterns)

	task.SetStatus(CycleSuccess)
	task.FinishedAt = time.Now()
	task.AppendLog("[GBRAIN] ===== Report Complete =====\n")
	task.AppendLog("Articles: %d | Facts: %d | Takes: %d | Patterns: %d | Consolidated: %d\n",
		len(articles), len(facts), len(takes), len(patterns), len(consolidatedTakes))
}

// readOutputArticles reads compiled articles only from the agent's output directory.
func (g *GbrainCompiler) readOutputArticles(input *agent.CompileInput) ([]CompiledArticle, error) {
	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = "synthesis"
	}

	root, err := g.fileSvc.GetOrCreateRootFolder(input.TenantID, input.Actor)
	if err != nil {
		return nil, fmt.Errorf("get root folder: %w", err)
	}

	// Agent writes to a folder named "{workspaceName}-{outputDir}"
	srcWorkspace, err := g.fileSvc.GetFileByID(input.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	targetName := srcWorkspace.Name + "-" + outputDir

	// Search for the output folder recursively
	var outputFolder *entity.TreeNode
	tree, err := g.fileSvc.GetCurrentTree(root.ID)
	if err == nil {
		var search func(nodes []entity.TreeNode) *entity.TreeNode
		search = func(nodes []entity.TreeNode) *entity.TreeNode {
			for i := range nodes {
				if nodes[i].Type == "folder" && nodes[i].Name == targetName {
					return &nodes[i]
				}
				if len(nodes[i].Children) > 0 {
					if found := search(nodes[i].Children); found != nil {
						return found
					}
				}
			}
			return nil
		}
		outputFolder = search(tree.Children)
	}
	if outputFolder == nil {
		// Collect all folders recursively for error message
		var allFolders []string
		var collect func(nodes []entity.TreeNode)
		collect = func(nodes []entity.TreeNode) {
			for _, c := range nodes {
				if c.Type == "folder" {
					allFolders = append(allFolders, c.Name)
					if len(c.Children) > 0 {
						collect(c.Children)
					}
				}
			}
		}
		if tree != nil {
			collect(tree.Children)
		}
		hint := fmt.Sprintf(" (expected name: '%s')", targetName)
		if len(allFolders) > 0 {
			hint += fmt.Sprintf(" available folders: %v", allFolders)
		}
		return nil, fmt.Errorf("output directory not found%s", hint)
	}

	var articles []CompiledArticle
	var walk func(nodes []entity.TreeNode)
	walk = func(nodes []entity.TreeNode) {
		for _, n := range nodes {
			if n.Type == "file" && strings.HasSuffix(n.Name, ".md") &&
				n.Name != "INDEX.md" && n.Name != "log.md" && n.Name != "gbrain-report.md" {
				f, data, err := g.fileSvc.GetFileContent(n.ID)
				if err != nil || f == nil || len(data) == 0 {
					continue
				}
				articles = append(articles, CompiledArticle{
					Slug:    n.Name,
					Content: string(data),
					Score:   0,
				})
			}
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}

	tree, err = g.fileSvc.GetCurrentTree(outputFolder.ID)
	if err != nil {
		return nil, fmt.Errorf("get output tree: %w", err)
	}
	walk(tree.Children)

	return articles, nil
}

// writeEnhancedOutput creates a single gbrain-report.md with all structured knowledge.
// Does NOT modify the original agent output files.
func (g *GbrainCompiler) writeEnhancedOutput(task *GbrainTask, input *agent.CompileInput, articles []CompiledArticle, facts []FactEntry, takes []TakeEntry, patterns []PatternEntry) {
	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = "synthesis"
	}

	// Find the output folder — agent names it "{workspaceName}-{outputDir}"
	root, _ := g.fileSvc.GetOrCreateRootFolder(input.TenantID, input.Actor)
	srcWorkspace, err := g.fileSvc.GetFileByID(input.WorkspaceID)
	targetName := outputDir
	if err == nil && srcWorkspace != nil {
		targetName = srcWorkspace.Name + "-" + outputDir
	}
	outputFolderID := root.ID
	tree, _ := g.fileSvc.GetCurrentTree(root.ID)
	if tree != nil {
		var search func(nodes []entity.TreeNode) string
		search = func(nodes []entity.TreeNode) string {
			for _, c := range nodes {
				if c.Type == "folder" && c.Name == targetName {
					return c.ID
				}
				if len(c.Children) > 0 {
					if id := search(c.Children); id != "" {
						return id
					}
				}
			}
			return ""
		}
		if id := search(tree.Children); id != "" {
			outputFolderID = id
		}
	}

	// Build the report
	report := renderGbrainReport(articles, facts, takes, patterns)
	var writeErr error
	_, writeErr = g.fileSvc.CreateTextFile(input.TenantID, outputFolderID, "gbrain-report.md", report, input.Actor)
	if writeErr != nil {
		task.AppendLog("[WRITE] Failed to write gbrain-report.md: %v\n", writeErr)
		return
	}
	task.NewArticles = 1
	task.AppendLog("[WRITE] Created gbrain-report.md (%d facts, %d takes, %d patterns)\n", len(facts), len(takes), len(patterns))
}

func renderGbrainReport(articles []CompiledArticle, facts []FactEntry, takes []TakeEntry, patterns []PatternEntry) string {
	var b strings.Builder
	b.WriteString("# Gbrain Compilation Report\n\n")
	b.WriteString("> 此报告由 gbrain 循环编译器自动生成，包含结构化知识提取和跨 session 模式发现结果。\n\n")
	b.WriteString("## Overview\n\n")
	b.WriteString(fmt.Sprintf("- **Articles analyzed**: %d\n", len(articles)))
	b.WriteString(fmt.Sprintf("- **Facts extracted**: %d\n", len(facts)))
	b.WriteString(fmt.Sprintf("- **Takes extracted**: %d\n", len(takes)))
	b.WriteString(fmt.Sprintf("- **Patterns discovered**: %d\n", len(patterns)))
	b.WriteString("\n---\n\n")

	// Per-article knowledge summary
	b.WriteString("## Per-Article Knowledge\n\n")
	for _, art := range articles {
		b.WriteString(fmt.Sprintf("### [[%s]]\n\n", strings.TrimSuffix(art.Slug, ".md")))
		// Find facts/takes for this article
		var artFacts []FactEntry
		var artTakes []TakeEntry
		for _, f := range facts {
			if strings.Contains(art.Slug, f.Source) || strings.Contains(f.Source, art.Slug) {
				artFacts = append(artFacts, f)
			}
		}
		for _, t := range takes {
			if t.Holder != "consolidated" && (strings.Contains(art.Slug, t.Source) || strings.Contains(t.Source, art.Slug)) {
				artTakes = append(artTakes, t)
			}
		}
		if len(artFacts) > 0 {
			b.WriteString("**Facts**:\n")
			for _, f := range artFacts {
				b.WriteString(fmt.Sprintf("- %s [confidence: %.2f, kind: %s]\n", f.Claim, f.Confidence, f.Kind))
			}
			b.WriteString("\n")
		}
		if len(artTakes) > 0 {
			b.WriteString("**Takes**:\n")
			for _, t2 := range artTakes {
				b.WriteString(fmt.Sprintf("- %s [weight: %.2f, kind: %s]\n", t2.Claim, t2.Weight, t2.Kind))
			}
			b.WriteString("\n")
		}
	}

	// Full knowledge snapshot (fence)
	b.WriteString("---\n\n## Full Knowledge Snapshot\n\n")

	// Deduplicate facts/takes for the global snapshot
	seenFacts := make(map[string]bool)
	var uniqueFacts []FactEntry
	for _, f := range facts {
		key := f.Claim
		if !seenFacts[key] {
			seenFacts[key] = true
			uniqueFacts = append(uniqueFacts, f)
		}
	}
	seenTakes := make(map[string]bool)
	var uniqueTakes []TakeEntry
	for _, t := range takes {
		key := t.Claim
		if !seenTakes[key] {
			seenTakes[key] = true
			uniqueTakes = append(uniqueTakes, t)
		}
	}

	b.WriteString(RenderFactsFence(uniqueFacts))
	b.WriteString("\n")
	b.WriteString(RenderTakesFence(uniqueTakes))

	// Patterns
	if len(patterns) > 0 {
		b.WriteString("---\n\n## Patterns (Cross-Article Themes)\n\n")
		for _, p := range patterns {
			b.WriteString(fmt.Sprintf("### %s\n\n", p.Title))
			b.WriteString(p.Description + "\n\n")
			b.WriteString("**Related articles**:\n")
			for _, ref := range p.ArticleRefs {
				b.WriteString(fmt.Sprintf("- [[%s]]\n", strings.TrimSuffix(ref, ".md")))
			}
			b.WriteString("\n")
		}
	}

	// Consolidated takes
	var consolidated []TakeEntry
	for _, t := range takes {
		if t.Holder == "consolidated" || t.Kind == "consolidated" {
			consolidated = append(consolidated, t)
		}
	}
	if len(consolidated) > 0 {
		b.WriteString("---\n\n## Consolidated Takes (Facts → Takes)\n\n")
		for _, t := range consolidated {
			b.WriteString(fmt.Sprintf("- %s [weight: %.2f]\n  _Source: %s_\n\n", t.Claim, t.Weight, t.Source))
		}
	}

	return b.String()
}

// ===== Cooldown (Redis-based, like gbrain's cooldown) =====

const defaultCooldownHours = 2

func (g *GbrainCompiler) checkCooldown(workspaceID string) bool {
	if g.rdb == nil {
		return true
	}
	key := "gbrain:cooldown:" + workspaceID
	exists, err := g.rdb.Exists(context.Background(), key).Result()
	return err == nil && exists == 0
}

func (g *GbrainCompiler) setCooldown(workspaceID string) {
	if g.rdb == nil {
		return
	}
	key := "gbrain:cooldown:" + workspaceID
	g.rdb.Set(context.Background(), key, "1", defaultCooldownHours*time.Hour)
}

// ===== Public query methods =====

func (g *GbrainCompiler) GetCycle(id string) *GbrainTask {
	g.cyclesMu.RLock()
	defer g.cyclesMu.RUnlock()
	return g.cycles[id]
}

func (g *GbrainCompiler) ListCycles() []*GbrainTask {
	g.cyclesMu.RLock()
	defer g.cyclesMu.RUnlock()
	result := make([]*GbrainTask, 0, len(g.cycles))
	for _, t := range g.cycles {
		result = append(result, t)
	}
	return result
}

// ClearCooldown removes the cooldown for a workspace, allowing a new cycle immediately.
func (g *GbrainCompiler) ClearCooldown(workspaceID string) error {
	if g.rdb == nil {
		return nil
	}
	key := "gbrain:cooldown:" + workspaceID
	return g.rdb.Del(context.Background(), key).Err()
}

// ===== Helpers =====

func findOrCreateOutputFolder(fileSvc *service.FileService, input *agent.CompileInput, root *entity.File) (*entity.File, error) {
	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = "synthesis"
	}
	// Try to find existing output folder
	tree, err := fileSvc.GetCurrentTree(root.ID)
	if err != nil {
		return nil, err
	}
	var found *entity.TreeNode
	var walk func(nodes []entity.TreeNode)
	walk = func(nodes []entity.TreeNode) {
		for _, n := range nodes {
			if n.Name == outputDir && n.Type == "folder" {
				found = &n
				return
			}
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(tree.Children)

	if found != nil {
		return &entity.File{ID: found.ID, Name: found.Name}, nil
	}
	return nil, nil
}
