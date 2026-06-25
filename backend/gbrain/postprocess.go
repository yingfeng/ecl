package gbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ===== Phase 5: Extract Facts & Takes =====
// gbrain's extract phase: parse compiled articles and extract structured facts/takes.
// Uses a cheap LLM call (like gbrain's Haiku significance judge).

func (g *GbrainCompiler) extractFactsAndTakes(task *GbrainTask, articles []CompiledArticle) ([]FactEntry, []TakeEntry) {
	task.CurrentPhase = "extract_fk"
	task.AppendLog("[FACTS] Extracting structured knowledge from %d articles...\n", len(articles))

	if len(articles) == 0 {
		return nil, nil
	}

	systemMsg := `你是一个知识结构化专家。从以下知识文章中提取结构化知识。

输出JSON格式（所有字段都要填充）:

facts 字段说明:
- claim: 事实陈述
- kind: 类型 (fact|event|preference|commitment|belief)
- confidence: 置信度 0-1
- visibility: 可见性 (world|private)
- notability: 重要性 (high|medium|low)
- valid_from: 开始日期 (YYYY-MM-DD, 未知则填空串)
- valid_until: 结束日期 (YYYY-MM-DD, 无则填空串)
- source: 来源文件名
- context: 上下文/备注
- active: true

takes 字段说明:
- claim: 观点/主张
- kind: 类型 (take|bet|hunch|fact)
- weight: 权重 0-1
- holder: 持有者 (world|brain|compiler)
- since: 起始日期 (YYYY-MM-DD)
- source: 来源文件名
- active: true

规则:
- facts: 可验证的事实陈述, 每个文章提取2-5条
- takes: 基于事实的主观判断/观点, 每个文章提取1-2条
- 所有字段都输出, 未知的填空串
- 输出纯JSON, 不要markdown fence`

	var allFacts []FactEntry
	var allTakes []TakeEntry

	for _, art := range articles {
		body := truncate(art.Content, 6000)
		userMsg := fmt.Sprintf("文章: %s\n\n正文:\n%s", art.Slug, body)

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		content, err := g.llm.ChatRaw(ctx, systemMsg, userMsg)
		cancel()
		if err != nil {
			task.AppendLog("[FACTS] LLM error for '%s': %v\n", art.Slug, err)
			continue
		}

		var result struct {
			Facts []FactEntry `json:"facts"`
			Takes []TakeEntry `json:"takes"`
		}
		content = stripJSONFence(content)
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			task.AppendLog("[FACTS] JSON parse error for '%s', trying lenient...\n", art.Slug)
			if err := unmarshalLenient(content, &result); err != nil {
				task.AppendLog("[FACTS] Lenient parse also failed: %v\n", err)
				continue
			}
		}

		// Fill in source field if empty
		for i := range result.Facts {
			if result.Facts[i].Source == "" {
				result.Facts[i].Source = art.Slug
			}
		}
		for i := range result.Takes {
			if result.Takes[i].Source == "" {
				result.Takes[i].Source = art.Slug
			}
		}

		allFacts = append(allFacts, result.Facts...)
		allTakes = append(allTakes, result.Takes...)
		task.AppendLog("[FACTS] '%s': %d facts, %d takes\n", art.Slug, len(result.Facts), len(result.Takes))
	}

	task.FactsExtracted = len(allFacts)
	task.TakesExtracted = len(allTakes)
	task.AppendLog("[FACTS] Done: %d facts, %d takes extracted\n", len(allFacts), len(allTakes))
	return allFacts, allTakes
}

// ===== Phase: Pattern Discovery (enhanced with article search) =====
// Provides article search context to the LLM, like gbrain's brain_search tool.
// Requires at least 3 articles (like gbrain's minEvidence).

func (g *GbrainCompiler) discoverPatterns(task *GbrainTask, articles []CompiledArticle) []PatternEntry {
	task.CurrentPhase = "patterns"
	task.AppendLog("[PATTERNS] Discovering cross-article patterns...\n")

	if len(articles) < 3 {
		task.AppendLog("[PATTERNS] Skipped: need at least 3 articles (got %d)\n", len(articles))
		return nil
	}

	// Build article index (like gbrain's brain_search)
	articleIndex := g.buildArticleSearchIndex(articles)
	articleMap := make(map[string]CompiledArticle)
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		articleMap[slug] = art
	}

	systemMsg := `你是一个模式发现专家。分析以下知识文章，发现跨文章重复出现的主题、模式或原则。
你拥有文章搜索工具: 调用 "SEARCH: <关键词>" 来搜索相关文章。
调用 "READ: <文章slug>" 来读取完整文章内容。

输出JSON格式:
{
  "patterns": [
    {
      "title": "模式名称(中文)",
      "slug": "pattern-slug-in-english",
      "description": "简要描述这个模式",
      "article_refs": ["article-slug", "article-slug"]
    }
  ]
}

规则:
- 只在确实有3篇或以上文章共享时创建模式
- 每篇文章可以出现在多个模式中
- 输出纯JSON, 不要markdown fence`

	// Build initial summaries
	var b strings.Builder
	b.WriteString("## Article Index\n\n")
	for slug, entries := range articleIndex {
		b.WriteString(fmt.Sprintf("- [[%s]]: %s\n", slug, strings.Join(entries, "; ")))
	}
	b.WriteString("\n使用 SEARCH: <关键词> 搜索, 使用 READ: <slug> 读取全文。\n\n")

	// Simulate search: build a comprehensive prompt with keyword-grouped articles
	b.WriteString("## Articles by Keywords\n\n")
	kwArticles := g.groupArticlesByKeyword(articles)
	for kw, group := range kwArticles {
		b.WriteString(fmt.Sprintf("### %s\n", kw))
		for _, slug := range group {
			b.WriteString(fmt.Sprintf("- [[%s]]\n", slug))
		}
		b.WriteString("\n")
	}

	// Full summaries for reference
	b.WriteString("## Full Article Summaries\n\n")
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		summary := art.Content
		if len(summary) > 400 {
			summary = summary[:400]
		}
		b.WriteString(fmt.Sprintf("### [[%s]]\n%s\n\n", slug, summary))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	content, err := g.llm.ChatRaw(ctx, systemMsg, b.String())
	cancel()
	if err != nil {
		task.AppendLog("[PATTERNS] LLM error: %v\n", err)
		return nil
	}

	// Handle SEARCH: and READ: commands in output (in case LLM uses them)
	_ = articleMap // available for future tool expansion

	var result struct {
		Patterns []PatternEntry `json:"patterns"`
	}
	content = stripJSONFence(content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		task.AppendLog("[PATTERNS] JSON parse error, skipping\n")
		return nil
	}

	task.PatternsFound = len(result.Patterns)
	task.AppendLog("[PATTERNS] Found %d patterns:\n", len(result.Patterns))
	for _, p := range result.Patterns {
		task.AppendLog("  - %s (%s): %d articles\n", p.Title, p.Slug, len(p.ArticleRefs))
	}
	return result.Patterns
}

// buildArticleSearchIndex creates keyword→articles index for search.
func (g *GbrainCompiler) buildArticleSearchIndex(articles []CompiledArticle) map[string][]string {
	idx := make(map[string][]string)
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		// Extract keywords from content (simple heuristic: top-N noun phrases)
		words := strings.Fields(art.Content)
		seen := make(map[string]bool)
		for _, w := range words {
			w = strings.Trim(w, ".,;:!?()[]{}'\"")
			if len(w) < 3 || isStopWord(w) {
				continue
			}
			if !seen[w] {
				idx[w] = append(idx[w], slug)
				seen[w] = true
			}
		}
	}
	return idx
}

// groupArticlesByKeyword groups articles by shared keywords for search simulation.
func (g *GbrainCompiler) groupArticlesByKeyword(articles []CompiledArticle) map[string][]string {
	groups := make(map[string][]string)
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		// Use title headings as keywords
		for _, line := range strings.Split(art.Content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				title := strings.TrimPrefix(line, "# ")
				groups[title] = append(groups[title], slug)
			}
			if strings.HasPrefix(line, "## ") {
				section := strings.TrimPrefix(line, "## ")
				groups[section] = append(groups[section], slug)
				break // only first section heading
			}
		}
	}
	return groups
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
	"you": true, "all": true, "can": true, "had": true, "her": true, "was": true,
	"one": true, "our": true, "out": true, "has": true, "have": true, "this": true,
	"that": true, "with": true, "from": true, "they": true, "been": true, "said": true,
	"will": true, "each": true, "than": true, "them": true, "when": true, "what": true,
	"which": true, "their": true, "there": true, "about": true, "would": true,
	"into": true, "over": true, "such": true, "also": true, "other": true, "more": true,
	"some": true, "these": true, "very": true, "just": true, "like": true, "make": true,
	"made": true, "well": true, "even": true, "most": true, "much": true, "same": true,
	"both": true, "here": true, "then": true, "only": true, "way": true,
	"were": true, "its": true, "does": true, "down": true, "how": true,
	"now": true, "too": true, "two": true, "use": true, "used": true, "using": true,
	"基于": true, "通过": true, "进行": true, "用于": true, "可以": true, "需要": true,
	"一个": true, "这个": true, "这些": true, "那些": true, "没有": true, "不是": true,
	"因为": true, "所以": true, "如果": true, "但是": true, "而且": true, "或者": true,
	"什么": true, "怎么": true, "如何": true, "就是": true, "还是": true, "虽然": true,
}

func isStopWord(w string) bool {
	w = strings.ToLower(w)
	_, ok := stopWords[w]
	return ok
}

// ===== Phase 7: Consolidate Facts → Takes =====
// gbrain's consolidate phase: group related facts and promote to takes.
// Non-LLM in v1 (like gbrain's v0.31 which uses highest-confidence pick).

func (g *GbrainCompiler) consolidateFacts(task *GbrainTask, facts []FactEntry, existingTakes []TakeEntry) []TakeEntry {
	task.CurrentPhase = "consolidate"
	task.AppendLog("[CONSOLIDATE] Consolidating %d facts into takes...\n", len(facts))

	if len(facts) < 3 {
		task.AppendLog("[CONSOLIDATE] Skipped: need at least 3 facts (got %d)\n", len(facts))
		return nil
	}

	// Group facts by source (simple heuristic, not full embedding clustering)
	bySource := make(map[string][]FactEntry)
	for _, f := range facts {
		src := f.Source
		if src == "" {
			src = "unknown"
		}
		bySource[src] = append(bySource[src], f)
	}

	var newTakes []TakeEntry
	for src, srcFacts := range bySource {
		if len(srcFacts) < 2 {
			continue
		}
		// Pick highest confidence fact as the take claim (like gbrain v0.31)
		best := srcFacts[0]
		for _, f := range srcFacts[1:] {
			if f.Confidence > best.Confidence {
				best = f
			}
		}
		newTakes = append(newTakes, TakeEntry{
			Claim:  best.Claim,
			Kind:   "consolidated",
			Weight: best.Confidence,
			Holder: "compiler",
			Source: fmt.Sprintf("consolidated from %d facts in %s", len(srcFacts), src),
		})
	}

	task.TakesConsolidated = len(newTakes)
	task.AppendLog("[CONSOLIDATE] %d facts → %d takes\n", len(facts), len(newTakes))
	return newTakes
}

// ===== Helper: add knowledge snapshot to article content =====

func addKnowledgeSnapshot(content string, facts []FactEntry, takes []TakeEntry) string {
	// Find the article slug from the content (first `# ` heading or front-matter)
	snapshot := GenerateKnowledgeSnapshot(facts, takes)
	if snapshot == "" {
		return content
	}

	// Append knowledge snapshot before end of file (or at end)
	content = strings.TrimRight(content, "\n \t")
	return content + snapshot + "\n"
}

// ===== Utility functions (ported from agent package) =====

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
	}
	if strings.HasSuffix(s, "```") {
		idx := strings.LastIndex(s, "```")
		if idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

func unmarshalLenient(s string, v interface{}) error {
	// Try direct first
	if err := json.Unmarshal([]byte(s), v); err == nil {
		return nil
	}
	// Try extracting JSON object from text
	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			if err := json.Unmarshal([]byte(s[start:end+1]), v); err == nil {
				return nil
			}
		}
	}
	// Try extracting JSON array from text
	if start := strings.Index(s, "["); start >= 0 {
		if end := strings.LastIndex(s, "]"); end > start {
			return json.Unmarshal([]byte(s[start:end+1]), v)
		}
	}
	return fmt.Errorf("cannot extract JSON from: %s", truncate(s, 100))
}
