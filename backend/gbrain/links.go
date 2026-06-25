package gbrain

import (
	"fmt"
	"regexp"
	"strings"
)

// LinkEntry represents a single wikilink extracted from an article.
type LinkEntry struct {
	Source string // article slug
	Target string // target slug (without .md)
}

// LinkGraph holds the complete link graph for all articles.
type LinkGraph struct {
	Links     []LinkEntry
	BySource  map[string][]string // source slug → target slugs
	ByTarget  map[string][]string // target slug → source slugs
	Orphans   []string            // articles with zero outgoing links
	Isolated  []string            // articles with zero incoming links
}

// wikilinkRe matches [[target]] and [[target|display text]]
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// extractLinkGraph parses all wikilinks from compiled articles.
func extractLinkGraph(articles []CompiledArticle) LinkGraph {
	lg := LinkGraph{
		BySource: make(map[string][]string),
		ByTarget: make(map[string][]string),
	}

	slugSet := make(map[string]bool)
	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		slugSet[slug] = true

		matches := wikilinkRe.FindAllStringSubmatch(art.Content, -1)
		var targets []string
		for _, m := range matches {
			target := strings.TrimSpace(m[1])
			if target == "" || target == slug {
				continue
			}
			// Normalize: remove .md if present
			target = strings.TrimSuffix(target, ".md")
			targets = append(targets, target)
			lg.Links = append(lg.Links, LinkEntry{Source: slug, Target: target})
		}
		if len(targets) > 0 {
			lg.BySource[slug] = targets
			for _, t := range targets {
				lg.ByTarget[t] = append(lg.ByTarget[t], slug)
			}
		}
	}

	// Compute orphans and isolated
	for slug := range slugSet {
		if _, ok := lg.BySource[slug]; !ok {
			lg.Orphans = append(lg.Orphans, slug)
		}
		if _, ok := lg.ByTarget[slug]; !ok {
			lg.Isolated = append(lg.Isolated, slug)
		}
	}

	return lg
}

func renderLinkSection(lg LinkGraph) string {
	var b strings.Builder
	b.WriteString("---\n\n## Link Graph\n\n")
	b.WriteString(fmt.Sprintf("- **Total links**: %d\n", len(lg.Links)))

	if len(lg.Orphans) > 0 {
		b.WriteString(fmt.Sprintf("- **Orphans** (no outgoing links): %d\n", len(lg.Orphans)))
		for _, o := range lg.Orphans {
			b.WriteString(fmt.Sprintf("  - [[%s]]\n", o))
		}
	}
	if len(lg.Isolated) > 0 {
		b.WriteString(fmt.Sprintf("- **Isolated** (no incoming links): %d\n", len(lg.Isolated)))
		for _, iso := range lg.Isolated {
			b.WriteString(fmt.Sprintf("  - [[%s]]\n", iso))
		}
	}
	b.WriteString("\n### Link Map\n\n")
	b.WriteString("| Source | → Targets |\n")
	b.WriteString("|--------|----------|\n")
	for _, art := range sortedKeys(lg.BySource) {
		targets := lg.BySource[art]
		b.WriteString(fmt.Sprintf("| [[%s]] | %s |\n", art, strings.Join(targets, ", ")))
	}
	return b.String()
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	// Sort for deterministic output
	for k := range m {
		if len(m[k]) > 0 {
			keys = append(keys, k)
		}
	}
	// Simple stable sort by length then content
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
