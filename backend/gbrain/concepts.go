package gbrain

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ===== Phase: Concept Synthesis =====
// gbrain's synthesize_concepts: group articles by theme, tier by article count, Sonnet for T1/T2.

type ConceptEntry struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Tier        string   `json:"tier"` // T1/T2/T3/T4
	ArticleRefs []string `json:"article_refs"`
	Narrative   string   `json:"narrative,omitempty"` // T1/T2 get LLM-written narrative
}

func (g *GbrainCompiler) synthesizeConcepts(task *GbrainTask, articles []CompiledArticle) []ConceptEntry {
	task.AppendLog("[CONCEPTS] Synthesizing cross-article concepts...\n")
	if len(articles) < 3 {
		task.AppendLog("[CONCEPTS] Need at least 3 articles, got %d\n", len(articles))
		return nil
	}

	// Step 1: Cluster articles by LLM (like gbrain's concepts: frontmatter grouping)
	systemMsg := `分析以下知识文章的标题和摘要，将它们按主题分组。每个组代表一个"概念"。
输出JSON格式:
{
  "concepts": [
    {
      "name": "概念名称",
      "slug": "concept-slug",
      "description": "概念描述(一句话)",
      "article_refs": ["article1-slug.md", "article2-slug.md"]
    }
  ]
}
规则: 每个概念至少关联3篇文章。一篇文章可以属于多个概念。
输出纯JSON, 不要markdown fence。`

	var b strings.Builder
	for _, art := range articles {
		summary := art.Content
		if len(summary) > 200 {
			summary = summary[:200]
		}
		// Extract title from first heading
		title := art.Slug
		for _, line := range strings.Split(art.Content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "# ") {
				title = strings.TrimPrefix(strings.TrimSpace(line), "# ")
				break
			}
		}
		b.WriteString(fmt.Sprintf("### %s (slug: %s)\n%s\n\n", title, art.Slug, summary))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	content, err := g.llm.ChatRaw(ctx, systemMsg, b.String())
	cancel()
	if err != nil {
		task.AppendLog("[CONCEPTS] LLM error: %v\n", err)
		return nil
	}

	var result struct {
		Concepts []ConceptEntry `json:"concepts"`
	}
	content = stripJSONFence(content)
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		if err := unmarshalLenient(content, &result); err != nil {
			task.AppendLog("[CONCEPTS] Parse error: %v\n", err)
			return nil
		}
	}

	// Step 2: Assign tiers by article count
	for i := range result.Concepts {
		count := len(result.Concepts[i].ArticleRefs)
		switch {
		case count >= 8:
			result.Concepts[i].Tier = "T1"
		case count >= 5:
			result.Concepts[i].Tier = "T2"
		case count >= 3:
			result.Concepts[i].Tier = "T3"
		default:
			result.Concepts[i].Tier = "T4"
		}
	}

	// Step 3: For T1/T2, write narrative via LLM
	narrativePrompt := `根据以下概念描述和相关文章内容，写一段3-5句话的执行摘要。用中文, 现在时态, 综合描述这个概念的核心内涵。
概念: %s
描述: %s
相关文章主题: %s
输出纯文本, 不要markdown fence。`

	for i, c := range result.Concepts {
		if c.Tier == "T1" || c.Tier == "T2" {
			ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
			narrative, err := g.llm.ChatRaw(ctx2, "你是一个概念综合专家。",
				fmt.Sprintf(narrativePrompt, c.Name, c.Description, strings.Join(c.ArticleRefs, ", ")))
			cancel2()
			if err == nil && narrative != "" {
				result.Concepts[i].Narrative = strings.TrimSpace(narrative)
			}
		}
	}

	task.AppendLog("[CONCEPTS] Found %d concepts:\n", len(result.Concepts))
	for _, c := range result.Concepts {
		task.AppendLog("  [%s] %s (%s): %d articles\n", c.Tier, c.Name, c.Slug, len(c.ArticleRefs))
	}
	return result.Concepts
}

func renderConceptsSection(concepts []ConceptEntry) string {
	if len(concepts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n\n## Concepts (Tiered)\n\n")
	for _, c := range concepts {
		b.WriteString(fmt.Sprintf("### [%s] [[%s]] %s\n\n", c.Tier, c.Slug, c.Name))
		b.WriteString(c.Description + "\n\n")
		if c.Narrative != "" {
			b.WriteString(c.Narrative + "\n\n")
		}
		b.WriteString(fmt.Sprintf("_%d related articles: ", len(c.ArticleRefs)))
		for i, ref := range c.ArticleRefs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("[[%s]]", strings.TrimSuffix(ref, ".md")))
		}
		b.WriteString("_\n\n")
	}
	b.WriteString(fmt.Sprintf("**Tier legend**: T1 ≥8 articles, T2 ≥5, T3 ≥3, T4 ≥1 (_stub_)\n"))
	return b.String()
}

// ===== Phase: Calibration Pipeline (simplified) =====
// propose takes → grade takes → calibration profile

type TakeProposal struct {
	Claim     string `json:"claim"`
	Kind      string `json:"kind"`
	Article   string `json:"article"`
	Weight    float64 `json:"weight"`
}

type TakeGrade struct {
	Claim   string `json:"claim"`
	Quality string `json:"quality"` // correct | incorrect | partial | unresolvable
	Reason  string `json:"reason"`
}

type CalibrationProfile struct {
	NarrativeStatements []string `json:"narrative_statements"`
	BiasTags            []string `json:"bias_tags"`
}

// Step 1: Propose takes from article prose
func (g *GbrainCompiler) proposeTakes(task *GbrainTask, articles []CompiledArticle) []TakeProposal {
	task.AppendLog("[CALIBRATION] Proposing takes from article prose...\n")
	if len(articles) == 0 {
		return nil
	}

	systemMsg := `从以下知识文章中提取隐含的主张/观点。这些是文章作者提出的、但未被明确标注为 takes 的隐式主张。
输出JSON:
{"proposals": [{"claim": "主张内容", "kind": "take|bet|hunch|rule", "article": "文章slug", "weight": 0.85}]}
每个文章提取1-2条。输出纯JSON。`

	var proposals []TakeProposal
	for _, art := range articles {
		body := truncate(art.Content, 4000)
		slug := strings.TrimSuffix(art.Slug, ".md")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		content, err := g.llm.ChatRaw(ctx, systemMsg, fmt.Sprintf("文章: %s\n\n%s", slug, body))
		cancel()
		if err != nil {
			continue
		}
		var r struct {
			Proposals []TakeProposal `json:"proposals"`
		}
		content = stripJSONFence(content)
		if json.Unmarshal([]byte(content), &r) != nil {
			unmarshalLenient(content, &r)
		}
		for _, p := range r.Proposals {
			p.Article = slug
			if p.Weight <= 0 {
				p.Weight = 0.5
			}
			proposals = append(proposals, p)
		}
	}
	task.AppendLog("[CALIBRATION] Proposed %d takes\n", len(proposals))
	return proposals
}

// Step 2: Grade takes against article content
func (g *GbrainCompiler) gradeTakes(task *GbrainTask, articles []CompiledArticle, proposals []TakeProposal) []TakeGrade {
	task.AppendLog("[CALIBRATION] Grading %d takes...\n", len(proposals))
	if len(proposals) == 0 {
		return nil
	}

	// Build article index for grade context
	articleMap := make(map[string]string)
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		s := art.Content
		if len(s) > 3000 {
			s = s[:3000]
		}
		articleMap[slug] = s
	}

	systemMsg := `你是主张裁判。判断以下主张是否被文章内容所支持。
quality: correct(正确)|incorrect(错误)|partial(部分正确)|unresolvable(无法判断)
reason: 一句话说明判断理由
输出JSON: {"grades": [{"claim": "主张", "quality": "correct", "reason": "..."}]}`

	var grades []TakeGrade
	for _, p := range proposals {
		body, ok := articleMap[p.Article]
		if !ok {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		content, err := g.llm.ChatRaw(ctx, systemMsg, fmt.Sprintf("文章: %s\n\n%s\n\n主张: %s", p.Article, body, p.Claim))
		cancel()
		if err != nil {
			continue
		}
		var r struct {
			Grades []TakeGrade `json:"grades"`
		}
		content = stripJSONFence(content)
		if json.Unmarshal([]byte(content), &r) != nil {
			if unmarshalLenient(content, &r) != nil {
				continue
			}
		}
		grades = append(grades, r.Grades...)
	}
	task.AppendLog("[CALIBRATION] Graded %d takes\n", len(grades))
	return grades
}

// Step 3: Aggregate into calibration profile
func (g *GbrainCompiler) generateProfile(task *GbrainTask, proposals []TakeProposal, grades []TakeGrade) *CalibrationProfile {
	task.AppendLog("[CALIBRATION] Generating calibration profile...\n")
	if len(grades) == 0 {
		return nil
	}

	// Count correct vs incorrect
	var correct, incorrect, partial int
	var claims []string
	for _, g := range grades {
		claims = append(claims, fmt.Sprintf("- %s → %s: %s", g.Claim, g.Quality, g.Reason))
		switch g.Quality {
		case "correct":
			correct++
		case "incorrect":
			incorrect++
		case "partial":
			partial++
		}
	}

	systemMsg := `分析以下主张的裁判结果, 输出2-4条叙事模式语句和0-3个偏见标签。
叙事模式: 从整体上看, 这个知识库中的知识主张呈现哪些模式?
偏见标签: 可能存在哪些认知偏见? (如: 近期偏见、确认偏见、过度自信等)
输出JSON:
{"narrative_statements": ["statement1", "statement2"], "bias_tags": ["tag1", "tag2"]}`

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	content, err := g.llm.ChatRaw(ctx, systemMsg, fmt.Sprintf(
		"裁判结果统计: %d correct, %d incorrect, %d partial\n\n%s",
		correct, incorrect, partial, strings.Join(claims, "\n")))
	cancel()
	if err != nil {
		return nil
	}

	var profile CalibrationProfile
	content = stripJSONFence(content)
	if json.Unmarshal([]byte(content), &profile) != nil {
		if unmarshalLenient(content, &profile) != nil {
			return nil
		}
	}

	task.AppendLog("[CALIBRATION] Profile: %d narratives, %d bias tags\n",
		len(profile.NarrativeStatements), len(profile.BiasTags))
	return &profile
}

func renderCalibrationSection(profile *CalibrationProfile) string {
	if profile == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n\n## Calibration Profile\n\n")
	if len(profile.NarrativeStatements) > 0 {
		b.WriteString("### Narrative Patterns\n\n")
		for _, s := range profile.NarrativeStatements {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
		b.WriteString("\n")
	}
	if len(profile.BiasTags) > 0 {
		b.WriteString("### Bias Tags\n\n")
		for _, t := range profile.BiasTags {
			b.WriteString(fmt.Sprintf("- `%s`\n", t))
		}
		b.WriteString("\n")
	}
	return b.String()
}
