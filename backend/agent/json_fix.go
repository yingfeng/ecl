package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ========== P3: Enhanced JSON Parsing with Progressive Fallback ==========
// Modeled after iceCoder's ToolArgumentsNormalizer.
// Provides multi-level JSON parsing with truncation recovery, alias support,
// and intelligent error diagnostics.

// parseJSONWithRecovery attempts to parse JSON with progressive fallback strategies.
// Returns the first successful parse, or the most informative error.
func parseJSONWithRecovery(data []byte, target interface{}) error {
	// Strategy 1: Direct unmarshal
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	s := string(data)

	// Strategy 2: Strip JSON fence ```json ... ```
	cleaned := stripJSONFence(s)
	if cleaned != s {
		if err := json.Unmarshal([]byte(cleaned), target); err == nil {
			return nil
		}
		s = cleaned
	}

	// Strategy 3: Try to salvage truncated JSON
	if salvaged := salvageTruncatedJSON(s); salvaged != s {
		if err := json.Unmarshal([]byte(salvaged), target); err == nil {
			return nil
		}
	}

	// Strategy 4: Extract JSON fragment from text
	if fragment := extractJSONFragment(s); fragment != "" {
		if err := json.Unmarshal([]byte(fragment), target); err == nil {
			return nil
		}
	}

	// Strategy 5: Fix common Go escaping issues
	fixed := fixCommonJSONIssues(s)
	if fixed != s {
		if err := json.Unmarshal([]byte(fixed), target); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to parse JSON after all recovery strategies")
}

// ========== Strategy 3: Truncation Recovery ==========

// salvageTruncatedJSON attempts to repair truncated JSON by completing it.
func salvageTruncatedJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Check if the string ends with a complete JSON structure
	if isCompleteJSON(s) {
		return s
	}

	// Try closing unclosed brackets
	stack := make([]rune, 0, 16)
	inString := false
	escaped := false

	for _, ch := range s {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, ch)
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}

	// Close any unclosed brackets
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i] {
		case '{':
			s += "}"
		case '[':
			s += "]"
		}
	}

	// If string was unterminated, close it
	if inString {
		s += "\""
	}

	return s
}

// isCompleteJSON checks if a string looks like complete JSON.
func isCompleteJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	first := s[0]
	last := s[len(s)-1]

	switch first {
	case '{':
		return last == '}'
	case '[':
		return last == ']'
	case '"':
		return last == '"'
	}
	return false
}

// ========== Strategy 4: JSON Fragment Extraction ==========

// jsonObjectRegex matches a JSON object or array in text.
var (
	jsonObjectRegex = regexp.MustCompile(`(?s)\{[^{}]*(\{[^{}]*\}[^{}]*)*\}`)
	jsonArrayRegex  = regexp.MustCompile(`(?s)\[[^\[\]]*(\[[^\[\]]*\][^\[\]]*)*\]`)
)

// extractJSONFragment attempts to extract a JSON object or array from surrounding text.
func extractJSONFragment(s string) string {
	s = strings.TrimSpace(s)

	// Try to find a JSON object
	if strings.Contains(s, "{") {
		start := strings.Index(s, "{")
		// Try to find a matching closing brace
		depth := 0
		inStr := false
		esc := false
		for i := start; i < len(s); i++ {
			ch := rune(s[i])
			if inStr {
				if esc {
					esc = false
					continue
				}
				if ch == '\\' {
					esc = true
					continue
				}
				if ch == '"' {
					inStr = false
				}
				continue
			}
			switch ch {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	// Try to find a JSON array
	if strings.Contains(s, "[") {
		start := strings.Index(s, "[")
		depth := 0
		inStr := false
		esc := false
		for i := start; i < len(s); i++ {
			ch := rune(s[i])
			if inStr {
				if esc {
					esc = false
					continue
				}
				if ch == '\\' {
					esc = true
					continue
				}
				if ch == '"' {
					inStr = false
				}
				continue
			}
			switch ch {
			case '"':
				inStr = true
			case '[':
				depth++
			case ']':
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return ""
}

// ========== Strategy 5: Common JSON Issue Fixes ==========

// fixCommonJSONIssues fixes common LLM JSON output issues.
func fixCommonJSONIssues(s string) string {
	// Fix single quotes instead of double quotes (but only for keys/string values)
	s = fixSingleQuotes(s)

	// Fix trailing commas (e.g. {"a":1,} -> {"a":1})
	s = removeTrailingCommas(s)

	// Fix unquoted keys (e.g. {name: "value"} -> {"name": "value"})
	s = fixUnquotedKeys(s)

	// Fix Python-style True/False/None
	s = fixPythonBooleans(s)

	// Fix comment lines (// ...)
	s = removeJSONComments(s)

	return s
}

// fixSingleQuotes replaces single quotes with double quotes in JSON values/keys.
// Only replaces when it makes structural sense (at the start/end of a token).
func fixSingleQuotes(s string) string {
	var buf bytes.Buffer
	inDouble := false
	escaped := false
	i := 0

	for i < len(s) {
		ch := s[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			i++
			continue
		}
		if inDouble {
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inDouble = false
			}
			buf.WriteByte(ch)
			i++
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteByte(ch)
			i++
			continue
		}
		if ch == '\'' {
			// Check if this is a JSON structural single quote (key or value)
			// Look at context: is it after : or , or [ or {?
			prev := byte(' ')
			if i > 0 {
				prev = s[i-1]
			}
			// Look at next char to see if this is the end of a key/value
			next := byte(' ')
			if i+1 < len(s) {
				next = s[i+1]
			}

			// Single quote can be replaced if used as string delimiter
			if isJSONStructuralChar(prev) || prev == ' ' || prev == '\t' || prev == '\n' {
				if next != '\'' { // Not empty string
					buf.WriteByte('"')
					i++
					// Find matching closing single quote
					for i < len(s) {
						if s[i] == '\\' {
							buf.WriteByte(s[i])
							i++
							if i < len(s) {
								buf.WriteByte(s[i])
								i++
							}
							continue
						}
						if s[i] == '\'' {
							// Check if next char is structural
							next := byte(' ')
							if i+1 < len(s) {
								next = s[i+1]
							}
							if isJSONStructuralChar(next) || next == ' ' || next == '\t' || next == '\n' || next == ',' || next == '}' || next == ']' || i+1 >= len(s) {
								buf.WriteByte('"')
								i++
								break
							}
						}
						buf.WriteByte(s[i])
						i++
					}
					continue
				}
			}
		}
		buf.WriteByte(ch)
		i++
	}
	return buf.String()
}

// isJSONStructuralChar returns true if c is a JSON structural character
// that could precede a string value.
func isJSONStructuralChar(c byte) bool {
	return c == ':' || c == ',' || c == '{' || c == '['
}

// removeTrailingCommas removes trailing commas before } or ].
func removeTrailingCommas(s string) string {
	// Remove commas before closing brackets
	result := strings.ReplaceAll(s, ",}", "}")
	result = strings.ReplaceAll(result, ",]", "]")
	result = strings.ReplaceAll(result, ",\n}", "}")
	result = strings.ReplaceAll(result, ",\n]", "]")
	result = strings.ReplaceAll(result, ",\r\n}", "}")
	result = strings.ReplaceAll(result, ",\r\n]", "]")
	return result
}

// fixUnquotedKeys quotes unquoted JSON keys.
func fixUnquotedKeys(s string) string {
	// This regex matches unquoted keys in JSON (word characters before ':')
	re := regexp.MustCompile(`([{,]\s*)(\w[\w_]*)(\s*:)`)
	result := re.ReplaceAllString(s, `$1"$2"$3`)

	// Also handle Python-style (single line)
	re2 := regexp.MustCompile(`([{,]\s*)'(\w[\w_]*)'(\s*:)`)
	result = re2.ReplaceAllString(result, `$1"$2"$3`)

	return result
}

// fixPythonBooleans replaces True/False/None with JSON equivalents.
func fixPythonBooleans(s string) string {
	re := regexp.MustCompile(`\bTrue\b`)
	s = re.ReplaceAllString(s, "true")
	re = regexp.MustCompile(`\bFalse\b`)
	s = re.ReplaceAllString(s, "false")
	re = regexp.MustCompile(`\bNone\b`)
	s = re.ReplaceAllString(s, "null")
	return s
}

// removeJSONComments removes // line comments from JSON.
func removeJSONComments(s string) string {
	lines := strings.Split(s, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Also handle inline comments (simple heuristic: remove // after content)
		if idx := strings.Index(line, "//"); idx >= 0 {
			// Check if // is inside a string
			before := line[:idx]
			inStr := false
			esc := false
			for _, ch := range before {
				if inStr {
					if ch == '\\' {
						esc = !esc
						continue
					}
					if ch == '"' && !esc {
						inStr = false
					}
					esc = false
					continue
				}
				if ch == '"' {
					inStr = true
				}
			}
			if !inStr {
				line = before
			}
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// ==========  Higher-level convenience parsers ==========

// parseJSONFenceAndRecover combines JSON fence stripping with recovery parsing.
// This is the recommended entry point for all LLM response parsing.
func parseJSONFenceAndRecover(s string, target interface{}) error {
	cleaned := stripJSONFence(s)
	return parseJSONWithRecovery([]byte(cleaned), target)
}

// parseLLMJSONResponse is the P3-enhanced version of the simple json.Unmarshal used throughout pipeline.go.
// Use this instead of direct json.Unmarshal for LLM responses.
func parseLLMJSONResponse(s string, target interface{}) error {
	return parseJSONFenceAndRecover(s, target)
}

// ==========  Field normalization helpers ==========

// normalizeFieldName tries common field name variations for JSON parsing.
type jsonFieldNormalizer struct {
	aliases map[string][]string
}

// newFieldNormalizer creates a normalizer with common Go-to-LLM field name mappings.
func newFieldNormalizer() *jsonFieldNormalizer {
	return &jsonFieldNormalizer{
		aliases: map[string][]string{
			"name":          {"name", "title", "slug", "topic_name", "concept_name"},
			"description":   {"description", "desc", "summary", "brief"},
			"content":       {"content", "body", "text", "markdown", "article"},
			"path":          {"path", "file", "file_path", "filename", "name"},
			"source_paths":  {"source_paths", "sources", "files", "source_files", "paths"},
			"topics":        {"topics", "topic_list", "related_topics", "categories"},
			"keywords":      {"keywords", "tags", "key_words", "kw"},
			"outputs":       {"outputs", "files", "articles", "results", "items"},
			"fixes":         {"fixes", "corrections", "changes", "updates"},
			"concepts":      {"concepts", "patterns", "cross_cutting_concepts"},
			"issues":        {"issues", "problems", "warnings", "flags"},
			"passed":        {"passed", "pass", "success", "ok", "valid"},
		},
	}
}

// tryAliases tries alternative field names when direct unmarshal fails.
func (n *jsonFieldNormalizer) tryAliases(data []byte, target interface{}) error {
	// First try direct parse
	if err := json.Unmarshal(data, target); err == nil {
		return nil
	}

	// Try with common replacements
	s := string(data)
	for _, aliases := range n.aliases {
		if len(aliases) < 2 {
			continue
		}
		for i := 1; i < len(aliases); i++ {
			replaced := strings.ReplaceAll(s, `"`+aliases[i]+`"`, `"`+aliases[0]+`"`)
			if replaced != s {
				if err := json.Unmarshal([]byte(replaced), target); err == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to parse JSON after alias normalization")
}

// ==========  LLM response structure detection ==========

// DetectJSONStructure returns the detected structure type of an LLM response.
type JSONStructure int

const (
	JSONStructureUnknown JSONStructure = iota
	JSONStructureObject
	JSONStructureArray
	JSONStructureMarkdown  // Markdown text, not JSON
	JSONStructureOther     // Plain text
)

// detectResponseType determines if a string is JSON, markdown, or plain text.
func detectResponseType(s string) JSONStructure {
	s = strings.TrimSpace(s)

	if s == "" {
		return JSONStructureOther
	}

	// Check for JSON fence
	if strings.HasPrefix(s, "```") && (strings.Contains(s, "```json") || strings.Contains(s, "```JSON")) {
		return JSONStructureObject
	}

	first := s[0]
	switch first {
	case '{':
		return JSONStructureObject
	case '[':
		return JSONStructureArray
	case '#':
		return JSONStructureMarkdown
	default:
		// Check if it starts with a Markdown heading or list
		if len(s) > 2 && (s[:2] == "# " || s[:2] == "##" || s[:2] == "- ") {
			return JSONStructureMarkdown
		}
		return JSONStructureOther
	}
}

// ==========  Safe numeric parsing ==========

// parseNumericField tries to parse a field as a number regardless of format.
// Handles: integer, float, string-encoded numbers ("42", "3.14").
func parseNumericField(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case string:
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

// ==========  Diagnostic helpers ==========

// diagnoseJSONError provides a human-readable explanation of JSON parse failure.
func diagnoseJSONError(s string, target interface{}) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "LLM returned empty response"
	}

	detected := detectResponseType(s)
	switch detected {
	case JSONStructureMarkdown:
		return fmt.Sprintf("LLM returned Markdown instead of JSON (first 100 chars: %s)", truncate(s, 100))
	case JSONStructureOther:
		return fmt.Sprintf("LLM returned plain text instead of JSON (first 100 chars: %s)", truncate(s, 100))
	case JSONStructureObject, JSONStructureArray:
		// Try salvaging
		salvaged := salvageTruncatedJSON(s)
		if salvaged != s {
			return fmt.Sprintf("JSON was truncated. Attempted recovery: %s", truncate(salvaged, 100))
		}
	}
	return fmt.Sprintf("JSON parse error. First 200 chars: %s", truncate(s, 200))
}

// ==========  Unmarshal helper using all strategies ==========

// unmarshalLenient is the all-in-one entry point for parsing LLM JSON responses.
// Combines all recovery strategies + alias normalization + diagnostic logging.
func unmarshalLenient(s string, target interface{}) error {
	// Fast path: direct parse after fence stripping
	cleaned := stripJSONFence(s)
	if err := json.Unmarshal([]byte(cleaned), target); err == nil {
		return nil
	}

	// Full recovery path
	if err := parseJSONWithRecovery([]byte(cleaned), target); err != nil {
		// Try alias normalization
		normalizer := newFieldNormalizer()
		if aliasErr := normalizer.tryAliases([]byte(cleaned), target); aliasErr != nil {
			return fmt.Errorf("JSON parsing failed: %s. Diagnostic: %s",
				err.Error(), diagnoseJSONError(cleaned, target))
		}
	}

	return nil
}

// isASCIILetter reports whether the byte is an ASCII letter.
func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// isJSONWhitespace reports whether the byte is JSON whitespace.
func isJSONWhitespace(b byte) bool {
	// Unicode whitespace check only for basic ASCII whitespace
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// skipJSONWhitespace advances index past whitespace.
func skipJSONWhitespace(s string, i int) int {
	for i < len(s) && isJSONWhitespace(s[i]) {
		i++
	}
	return i
}

// trimJSONString trims whitespace but only uses high-confidence unicode whitespace
func trimJSONWhitespace(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v' ||
			r == 0x00A0 || r == 0x1680 || unicode.IsSpace(r)
	})
}
