package gbrain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ===== Markers (mirror gbrain's FACTS_FENCE_BEGIN / FACTS_FENCE_END) =====

const (
	FactsFenceBegin = "<!--- llmwiki:facts:begin -->"
	FactsFenceEnd   = "<!--- llmwiki:facts:end -->"
	TakesFenceBegin = "<!--- llmwiki:takes:begin -->"
	TakesFenceEnd   = "<!--- llmwiki:takes:end -->"
)

// ===== Facts Fence: Render =====
// gbrain 10-column layout: claim | kind | confidence | visibility | notability | valid_from | valid_until | source | context

func RenderFactsFence(facts []FactEntry) string {
	var b strings.Builder
	b.WriteString("### Facts\n\n")
	b.WriteString(FactsFenceBegin + "\n")
	b.WriteString("| # | claim | kind | confidence | visibility | notability | valid_from | valid_until | source | context |\n")
	b.WriteString("|---|-------|------|------------|------------|------------|------------|-------------|--------|---------|\n")
	for i, f := range facts {
		claim := f.Claim
		if !f.Active {
			claim = "~~" + claim + "~~"
		}
		kind := f.Kind
		if kind == "" {
			kind = "fact"
		}
		vis := f.Visibility
		if vis == "" {
			vis = "world"
		}
		nota := f.Notability
		if nota == "" {
			nota = "medium"
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.2f | %s | %s | %s | %s | %s | %s |\n",
			i+1, escapeCell(claim), kind, f.Confidence, vis, nota,
			f.ValidFrom, f.ValidUntil, escapeCell(f.Source), escapeCell(f.Context)))
	}
	b.WriteString(FactsFenceEnd + "\n")
	return b.String()
}

// ===== Facts Fence: Parse =====

type FactsParseResult struct {
	Facts    []FactEntry
	Warnings []string
}

func ParseFactsFence(body string) FactsParseResult {
	beginIdx := strings.Index(body, FactsFenceBegin)
	endIdx := strings.LastIndex(body, FactsFenceEnd)
	var warnings []string

	if beginIdx == -1 && endIdx == -1 {
		return FactsParseResult{Warnings: warnings}
	}
	if beginIdx == -1 || endIdx == -1 || endIdx < beginIdx {
		warnings = append(warnings, "FACTS_FENCE_UNBALANCED")
		return FactsParseResult{Warnings: warnings}
	}

	inner := body[beginIdx+len(FactsFenceBegin) : endIdx]
	lines := strings.Split(inner, "\n")
	var facts []FactEntry
	sawHeader := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := splitPipe(line)
		if len(cells) < 2 {
			continue
		}
		if !sawHeader {
			lower := toLowerSlice(cells)
			if contains(lower, "claim") && contains(lower, "kind") {
				sawHeader = true
				continue
			}
			continue
		}
		// Separator row
		if isDashRow(cells) {
			continue
		}
		// Expect 9-10 data columns (cells[0]=#, cells[1..9/10]=data)
		if len(cells) < 9 {
			warnings = append(warnings, fmt.Sprintf("FACTS_TABLE_MALFORMED: only %d cells", len(cells)))
			continue
		}

		claim := stripStrikethrough(cells[1])
		active := !strings.HasPrefix(cells[1], "~~")

		kind := cells[2]
		confidence := parseFloat(cells[3], 0.5)
		visibility := cells[4]
		notability := cells[5]
		validFrom := cells[6]
		validUntil := cells[7]
		source := cells[8]
		context := ""
		if len(cells) >= 10 {
			context = cells[9]
		}

		facts = append(facts, FactEntry{
			Claim:      claim,
			Kind:       kind,
			Confidence: confidence,
			Visibility: visibility,
			Notability: notability,
			ValidFrom:  validFrom,
			ValidUntil: validUntil,
			Source:     source,
			Context:    context,
			Active:     active,
		})
	}

	return FactsParseResult{Facts: facts, Warnings: warnings}
}

// ===== Takes Fence: Render =====
// gbrain 7-column layout: claim | kind | who | weight | since | source

func RenderTakesFence(takes []TakeEntry) string {
	var b strings.Builder
	b.WriteString("### Takes\n\n")
	b.WriteString(TakesFenceBegin + "\n")
	b.WriteString("| # | claim | kind | who | weight | since | source |\n")
	b.WriteString("|---|-------|------|-----|--------|-------|--------|\n")
	for i, t := range takes {
		claim := t.Claim
		if !t.Active {
			claim = "~~" + claim + "~~"
		}
		kind := t.Kind
		if kind == "" {
			kind = "take"
		}
		holder := t.Holder
		if holder == "" {
			holder = "compiler"
		}
		since := t.SinceDate
		if since != "" && t.UntilDate != "" {
			since = since + " → " + t.UntilDate
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %.2f | %s | %s |\n",
			i+1, escapeCell(claim), kind, escapeCell(holder), t.Weight, since, escapeCell(t.Source)))
	}
	b.WriteString(TakesFenceEnd + "\n")
	return b.String()
}

// ===== Takes Fence: Parse =====

type TakesParseResult struct {
	Takes    []TakeEntry
	Warnings []string
}

func ParseTakesFence(body string) TakesParseResult {
	beginIdx := strings.Index(body, TakesFenceBegin)
	endIdx := strings.LastIndex(body, TakesFenceEnd)
	var warnings []string

	if beginIdx == -1 && endIdx == -1 {
		return TakesParseResult{Warnings: warnings}
	}
	if beginIdx == -1 || endIdx == -1 || endIdx < beginIdx {
		warnings = append(warnings, "TAKES_FENCE_UNBALANCED")
		return TakesParseResult{Warnings: warnings}
	}

	inner := body[beginIdx+len(TakesFenceBegin) : endIdx]
	lines := strings.Split(inner, "\n")
	var takes []TakeEntry
	sawHeader := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := splitPipe(line)
		if len(cells) < 2 {
			continue
		}
		if !sawHeader {
			lower := toLowerSlice(cells)
			if contains(lower, "claim") && contains(lower, "kind") {
				sawHeader = true
				continue
			}
			continue
		}
		if isDashRow(cells) {
			continue
		}
		if len(cells) < 6 {
			warnings = append(warnings, fmt.Sprintf("TAKES_TABLE_MALFORMED: only %d cells", len(cells)))
			continue
		}

		claim := stripStrikethrough(cells[1])
		active := !strings.HasPrefix(cells[1], "~~")

		kind := cells[2]
		holder := cells[3]
		weight := parseFloat(cells[4], 0.5)

		since := cells[5]
		sinceDate := since
		untilDate := ""
		if idx := strings.Index(since, "→"); idx >= 0 {
			sinceDate = strings.TrimSpace(since[:idx])
			untilDate = strings.TrimSpace(since[idx+len("→"):])
		} else if idx := strings.Index(since, "->"); idx >= 0 {
			sinceDate = strings.TrimSpace(since[:idx])
			untilDate = strings.TrimSpace(since[idx+len("->"):])
		}

		source := ""
		if len(cells) >= 7 {
			source = cells[6]
		}

		takes = append(takes, TakeEntry{
			Claim:     claim,
			Kind:      kind,
			Holder:    holder,
			Weight:    weight,
			SinceDate: sinceDate,
			UntilDate: untilDate,
			Source:    source,
			Active:    active,
		})
	}

	return TakesParseResult{Takes: takes, Warnings: warnings}
}

// ===== Helpers (gbrain fence-shared.ts equivalents) =====

func splitPipe(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return nil
	}
	line = strings.TrimPrefix(line, "|")
	if strings.HasSuffix(line, "|") {
		line = strings.TrimSuffix(line, "|")
	}
	raw := strings.Split(line, "|")
	cells := make([]string, 0, len(raw))
	for _, c := range raw {
		cells = append(cells, strings.TrimSpace(c))
	}
	return cells
}

func isDashRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	// All cells contain only dashes, colons, and optional spaces
	dashOrColon := regexp.MustCompile(`^[-:\s]+$`)
	for _, c := range cells {
		if !dashOrColon.MatchString(c) {
			return false
		}
	}
	return true
}

func stripStrikethrough(s string) string {
	s = strings.TrimPrefix(s, "~~")
	s = strings.TrimSuffix(s, "~~")
	return strings.TrimSpace(s)
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func parseFloat(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func toLowerSlice(slice []string) []string {
	out := make([]string, len(slice))
	for i, s := range slice {
		out[i] = strings.ToLower(s)
	}
	return out
}

// ===== Generate knowledge snapshot (full fence section) =====

func GenerateKnowledgeSnapshot(facts []FactEntry, takes []TakeEntry) string {
	if len(facts) == 0 && len(takes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 知识快照\n\n")

	if len(facts) > 0 {
		b.WriteString(RenderFactsFence(facts))
		b.WriteString("\n")
	}
	if len(takes) > 0 {
		b.WriteString(RenderTakesFence(takes))
	}
	return b.String()
}
