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

	// ── Phase 9: Link graph (P1: wikilink extraction) ──
	task.AppendLog("[GBRAIN] ---- Phase 9: Link Graph ----\n")
	linkGraph := extractLinkGraph(articles)

	// ── Phase 10: Timeline extraction (P2) ──
	task.AppendLog("[GBRAIN] ---- Phase 10: Timeline ----\n")
	timeline := extractTimeline(articles)

	// ── Phase 11: Concept synthesis (P2) ──
	task.AppendLog("[GBRAIN] ---- Phase 11: Concept Synthesis ----\n")
	concepts := g.synthesizeConcepts(task, articles)

	// ── Phase 12: Calibration pipeline (P2) ──
	task.AppendLog("[GBRAIN] ---- Phase 12: Calibration ----\n")
	var profile *CalibrationProfile
	if len(articles) >= 3 {
		proposals := g.proposeTakes(task, articles)
		grades := g.gradeTakes(task, articles, proposals)
		profile = g.generateProfile(task, proposals, grades)
	}

	// ── Phase 13: Write enhanced output ──
	task.AppendLog("[GBRAIN] ---- Phase 13: Writing Enhanced Output ----\n")
	g.writeEnhancedOutput(task, input, articles, facts, takes, patterns, linkGraph, timeline, concepts, profile)

	// ── Complete ──
	task.SetStatus(CycleSuccess)
	task.FinishedAt = time.Now()
	g.setCooldown(input.WorkspaceID)
	task.AppendLog("[GBRAIN] ===== Gbrain Cycle Complete =====\n")
	task.AppendLog("Articles: %d | Facts: %d | Takes: %d | Patterns: %d | Links: %d | Timeline: %d | Concepts: %d\n",
		len(articles), len(facts), len(takes), len(patterns), len(linkGraph.Links), len(timeline), len(concepts))
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

	task.AppendLog("[GBRAIN] ---- Phase 9: Link Graph ----\n")
	linkGraph := extractLinkGraph(articles)

	task.AppendLog("[GBRAIN] ---- Phase 10: Timeline ----\n")
	timeline := extractTimeline(articles)

	task.AppendLog("[GBRAIN] ---- Phase 11: Concept Synthesis ----\n")
	concepts := g.synthesizeConcepts(task, articles)

	task.AppendLog("[GBRAIN] ---- Phase 12: Calibration ----\n")
	var profile *CalibrationProfile
	if len(articles) >= 3 {
		proposals := g.proposeTakes(task, articles)
		grades := g.gradeTakes(task, articles, proposals)
		profile = g.generateProfile(task, proposals, grades)
	}

	task.AppendLog("[GBRAIN] ---- Phase 13: Writing Report ----\n")
	g.writeEnhancedOutput(task, input, articles, facts, takes, patterns, linkGraph, timeline, concepts, profile)

	task.SetStatus(CycleSuccess)
	task.FinishedAt = time.Now()
	task.AppendLog("[GBRAIN] ===== Report Complete =====\n")
	task.AppendLog("Articles: %d | Facts: %d | Takes: %d | Patterns: %d | Links: %d | Timeline: %d | Concepts: %d\n",
		len(articles), len(facts), len(takes), len(patterns), len(linkGraph.Links), len(timeline), len(concepts))
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

// writeEnhancedOutput writes gbrain phase outputs to per-subfolder files.
// Patterns/concepts/calibration → standalone pages (user-readable).
// Facts/takes → fence-embedded (intermediate data, not standalone).
func (g *GbrainCompiler) writeEnhancedOutput(task *GbrainTask, input *agent.CompileInput, articles []CompiledArticle, facts []FactEntry, takes []TakeEntry, patterns []PatternEntry, linkGraph LinkGraph, timeline []TimelineEntry, concepts []ConceptEntry, profile *CalibrationProfile) {
	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = "synthesis"
	}

	// Find the output folder
	root, _ := g.fileSvc.GetOrCreateRootFolder(input.TenantID, input.Actor)
	srcWorkspace, _ := g.fileSvc.GetFileByID(input.WorkspaceID)
	targetName := outputDir
	if srcWorkspace != nil {
		targetName = srcWorkspace.Name + "-" + outputDir
	}
	outputFolderID := g.findFolderByName(root.ID, targetName)
	if outputFolderID == "" {
		outputFolderID = root.ID
	}

	// Create gbrain subfolder for supplementary knowledge
	gbrainFolderID := g.findOrCreateFolder(input, outputFolderID, "gbrain")

	var totalWritten int

	// ── Overview (lightweight index, NOT a data dump) ──
	overview := renderGbrainOverview(articles, facts, takes, patterns, linkGraph, timeline, concepts)
	if _, err := g.fileSvc.CreateTextFile(input.TenantID, gbrainFolderID, "overview.md", overview, input.Actor); err == nil {
		totalWritten++
	}

	// ── Knowledge Graph (Link Graph + Timeline) ──
	if len(linkGraph.Links) > 0 || len(timeline) > 0 {
		kg := renderKnowledgeGraph(linkGraph, timeline)
		if _, err := g.fileSvc.CreateTextFile(input.TenantID, gbrainFolderID, "knowledge-graph.md", kg, input.Actor); err == nil {
			totalWritten++
		}
	}

	// ── Patterns: one file per pattern in gbrain/patterns/ ──
	if len(patterns) > 0 {
		patternsFolderID := g.findOrCreateFolder(input, gbrainFolderID, "patterns")
		// Index
		patternsIndex := renderPatternsIndex(patterns)
		if _, err := g.fileSvc.CreateTextFile(input.TenantID, patternsFolderID, "index.md", patternsIndex, input.Actor); err == nil {
			totalWritten++
		}
		// Per-pattern pages
		for _, p := range patterns {
			page := renderPatternPage(p)
			if _, err := g.fileSvc.CreateTextFile(input.TenantID, patternsFolderID, p.Slug+".md", page, input.Actor); err == nil {
				totalWritten++
			}
		}
	}

	// ── Concepts: one file per tier group in gbrain/concepts/ ──
	if len(concepts) > 0 {
		conceptsFolderID := g.findOrCreateFolder(input, gbrainFolderID, "concepts")
		conceptsIndex := renderConceptsIndex(concepts)
		if _, err := g.fileSvc.CreateTextFile(input.TenantID, conceptsFolderID, "index.md", conceptsIndex, input.Actor); err == nil {
			totalWritten++
		}
		for _, c := range concepts {
			page := renderConceptPage(c)
			if _, err := g.fileSvc.CreateTextFile(input.TenantID, conceptsFolderID, c.Slug+".md", page, input.Actor); err == nil {
				totalWritten++
			}
		}
	}

	// ── Calibration Profile ──
	if profile != nil {
		prof := renderCalibrationPage(profile)
		if _, err := g.fileSvc.CreateTextFile(input.TenantID, gbrainFolderID, "calibration-profile.md", prof, input.Actor); err == nil {
			totalWritten++
		}
	}

	task.NewArticles = totalWritten
	task.AppendLog("[WRITE] Wrote %d gbrain files (%d patterns, %d concepts, links+timeline, profile)\n",
		totalWritten, len(patterns), len(concepts))
}

// ===== Helper: find folder by name anywhere in tree (recursive) =====

func (g *GbrainCompiler) findFolderByName(rootID, name string) string {
	tree, err := g.fileSvc.GetCurrentTree(rootID)
	if err != nil {
		return ""
	}
	var search func(nodes []entity.TreeNode) string
	search = func(nodes []entity.TreeNode) string {
		for _, c := range nodes {
			if c.Type == "folder" && c.Name == name {
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
	return search(tree.Children)
}

// ===== Helper: find or create subfolder =====

func (g *GbrainCompiler) findOrCreateFolder(input *agent.CompileInput, parentID, name string) string {
	// Try to find existing
	if id := g.findFolderByName(parentID, name); id != "" {
		return id
	}
	// Create new
	f, err := g.fileSvc.CreateFolder(input.TenantID, parentID, name, input.Actor)
	if err != nil {
		return parentID // fallback to parent
	}
	return f.ID
}

// ===== Per-file renderers =====

// renderGbrainOverview: lightweight index, NOT data dump.
// Users and agents read this to navigate gbrain outputs.
func renderGbrainOverview(articles []CompiledArticle, facts []FactEntry, takes []TakeEntry, patterns []PatternEntry, linkGraph LinkGraph, timeline []TimelineEntry, concepts []ConceptEntry) string {
	var b strings.Builder
	b.WriteString("# Gbrain Knowledge Overview\n\n")
	b.WriteString("> 此概览由 gbrain 循环编译器生成。详细内容见各子目录。\n\n")

	b.WriteString("## Compilation Stats\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Count |\n|---|---|\n"))
	b.WriteString(fmt.Sprintf("| Source articles | %d |\n", len(articles)))
	b.WriteString(fmt.Sprintf("| Facts extracted | %d |\n", len(facts)))
	b.WriteString(fmt.Sprintf("| Takes extracted | %d |\n", len(takes)))
	b.WriteString(fmt.Sprintf("| Patterns discovered | %d |\n", len(patterns)))
	b.WriteString(fmt.Sprintf("| Links extracted | %d |\n", len(linkGraph.Links)))
	b.WriteString(fmt.Sprintf("| Timeline entries | %d |\n", len(timeline)))
	b.WriteString(fmt.Sprintf("| Concepts synthesized | %d |\n", len(concepts)))

	b.WriteString("\n## Navigation\n\n")
	b.WriteString("- **[[knowledge-graph]]** → Article link graph and timeline\n")
	if len(patterns) > 0 {
		b.WriteString("- **[[patterns/index]]** → Cross-article patterns\n")
	}
	if len(concepts) > 0 {
		b.WriteString("- **[[concepts/index]]** → Synthesized concepts\n")
	}
	b.WriteString("- **[[calibration-profile]]** → Calibration profile\n")

	b.WriteString("\n---\n\n## Primary Articles\n\n")
	b.WriteString("以下为主要知识文章（由 agent 编译生成）：\n\n")
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		b.WriteString(fmt.Sprintf("- [[%s]]\n", slug))
	}
	return b.String()
}

// renderKnowledgeGraph: link graph + timeline (intermediate data)
func renderKnowledgeGraph(linkGraph LinkGraph, timeline []TimelineEntry) string {
	var b strings.Builder
	b.WriteString("# Knowledge Graph\n\n")
	b.WriteString("> 文章之间的链接关系和时序信息。由 gbrain 自动提取。\n\n")

	if len(linkGraph.Links) > 0 {
		b.WriteString(renderLinkSection(linkGraph))
	}
	if len(timeline) > 0 {
		b.WriteString(renderTimelineSection(timeline))
	}
	return b.String()
}

// renderPatternsIndex: list of all patterns
func renderPatternsIndex(patterns []PatternEntry) string {
	var b strings.Builder
	b.WriteString("# Patterns Index\n\n")
	b.WriteString("> 跨文章重复出现的主题和模式。每个模式是一个独立的知识页面。\n\n")
	b.WriteString("| Pattern | Articles | Description |\n")
	b.WriteString("|---------|----------|-------------|\n")
	for _, p := range patterns {
		b.WriteString(fmt.Sprintf("| [[%s]] | %d | %s |\n", p.Slug, len(p.ArticleRefs), p.Description))
	}
	return b.String()
}

// renderPatternPage: standalone pattern page (user-readable)
func renderPatternPage(p PatternEntry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", p.Title))
	b.WriteString(fmt.Sprintf("> %s\n\n", p.Description))
	b.WriteString("## Related Articles\n\n")
	for _, ref := range p.ArticleRefs {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", strings.TrimSuffix(ref, ".md")))
	}
	b.WriteString("\n---\n")
	b.WriteString(fmt.Sprintf("_Discovered by gbrain pattern detection_\n"))
	return b.String()
}

// renderConceptsIndex: list of all concepts
func renderConceptsIndex(concepts []ConceptEntry) string {
	var b strings.Builder
	b.WriteString("# Concepts Index\n\n")
	b.WriteString("> 跨文章综合而成的知识概念。按文章数量分 T1/T2/T3/T4 四个层级。\n\n")
	b.WriteString("| Tier | Concept | Articles | Description |\n")
	b.WriteString("|------|---------|----------|-------------|\n")
	for _, c := range concepts {
		b.WriteString(fmt.Sprintf("| %s | [[%s]] | %d | %s |\n", c.Tier, c.Slug, len(c.ArticleRefs), c.Description))
	}
	return b.String()
}

// renderConceptPage: standalone concept page (user-readable)
func renderConceptPage(c ConceptEntry) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", c.Name))
	b.WriteString(fmt.Sprintf("**Tier**: %s  \n", c.Tier))
	b.WriteString(fmt.Sprintf("**Description**: %s\n\n", c.Description))
	if c.Narrative != "" {
		b.WriteString("## Executive Summary\n\n")
		b.WriteString(c.Narrative + "\n\n")
	}
	b.WriteString("## Related Articles\n\n")
	for _, ref := range c.ArticleRefs {
		b.WriteString(fmt.Sprintf("- [[%s]]\n", strings.TrimSuffix(ref, ".md")))
	}
	b.WriteString(fmt.Sprintf("\n_Synthesized from %d articles by gbrain concept synthesis_\n", len(c.ArticleRefs)))
	return b.String()
}

// renderCalibrationPage: standalone calibration profile page
func renderCalibrationPage(profile *CalibrationProfile) string {
	return fmt.Sprintf(`# Calibration Profile

> 此画像由 gbrain 校准管道生成，基于知识主张的自动裁决结果。

## Narrative Patterns

%s

## Bias Tags

%s

---

_Generated by gbrain calibration pipeline (propose → grade → aggregate)_
`,
		formatBullets(profile.NarrativeStatements),
		formatBullets(profile.BiasTags),
	)
}

func formatBullets(items []string) string {
	if len(items) == 0 {
		return "_None_\n"
	}
	var b strings.Builder
	for _, item := range items {
		b.WriteString(fmt.Sprintf("- %s\n", item))
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
