package gbrain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// TimelineEntry represents a dated event extracted from an article.
type TimelineEntry struct {
	Date    string // ISO date "YYYY-MM-DD"
	Summary string
	Source  string // article slug
}

// dateRe matches ISO dates like 2024-01-15 or 2024-01
var dateRe = regexp.MustCompile(`(\d{4})-(\d{2})(?:-(\d{2}))?`)

// frontmatterDateRe matches "date: 2024-01-15" in frontmatter
var frontmatterDateRe = regexp.MustCompile(`(?m)^date:\s*['"]?(\d{4}-\d{2}(?:-\d{2})?)['"]?\s*$`)

// extractTimeline extracts dated events from compiled articles.
func extractTimeline(articles []CompiledArticle) []TimelineEntry {
	var entries []TimelineEntry

	for _, art := range articles {
		slug := strings.TrimSuffix(art.Slug, ".md")
		content := art.Content

		// 1. Check frontmatter for date field
		m := frontmatterDateRe.FindStringSubmatch(content)
		if m != nil {
			entries = append(entries, TimelineEntry{
				Date:    m[1],
				Summary: fmt.Sprintf("Article published: [[%s]]", slug),
				Source:  slug,
			})
		}

		// 2. Scan first 2000 chars for dates with context
		preview := content
		if len(preview) > 2000 {
			preview = preview[:2000]
		}

		lines := strings.Split(preview, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "<!---") {
				continue
			}
			dates := dateRe.FindAllString(line, -1)
			if len(dates) > 0 && len(line) < 200 {
				summary := strings.TrimSpace(line)
				if len(summary) > 100 {
					summary = summary[:100] + "..."
				}
				for _, d := range dates {
					entries = append(entries, TimelineEntry{
						Date:    normalizeDate(d),
						Summary: summary,
						Source:  slug,
					})
				}
			}
		}
	}

	// Sort by date
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})

	return entries
}

func normalizeDate(s string) string {
	parts := strings.Split(s, "-")
	if len(parts) == 2 {
		return s + "-01" // YYYY-MM → YYYY-MM-01
	}
	return s
}

func renderTimelineSection(entries []TimelineEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("---\n\n## Timeline\n\n")
	b.WriteString(fmt.Sprintf("**%d dated events** extracted from %d sources\n\n", len(entries), countSources(entries)))
	b.WriteString("| Date | Event | Source |\n")
	b.WriteString("|------|-------|--------|\n")

	for _, e := range entries {
		dateStr := e.Date
		if t, err := time.Parse("2006-01-02", e.Date); err == nil {
			dateStr = t.Format("2006-01-02")
		}
		b.WriteString(fmt.Sprintf("| %s | %s | [[%s]] |\n", dateStr, escapeCell(e.Summary), e.Source))
	}
	return b.String()
}

func countSources(entries []TimelineEntry) int {
	seen := make(map[string]bool)
	for _, e := range entries {
		seen[e.Source] = true
	}
	return len(seen)
}
