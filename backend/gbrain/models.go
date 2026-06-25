package gbrain

import (
	"fmt"
	"sync"
	"time"
)

// GbrainCycleStatus tracks the lifecycle of a gbrain cycle.
type GbrainCycleStatus string

const (
	CyclePending  GbrainCycleStatus = "pending"
	CycleRunning  GbrainCycleStatus = "running"
	CycleSuccess  GbrainCycleStatus = "success"
	CycleFailed   GbrainCycleStatus = "failed"
	CycleSkipped  GbrainCycleStatus = "skipped"
)

// GbrainTask tracks a single gbrain cycle execution.
type GbrainTask struct {
	ID          string
	WorkspaceID string
	Status      GbrainCycleStatus

	// Agent sub-task
	AgentTaskID string

	// Phase tracking
	CurrentPhase string
	PhaseLog     string

	// Cycle-level results
	NewArticles      int
	UpdatedArticles  int
	FactsExtracted   int
	TakesExtracted   int
	PatternsFound    int
	TakesConsolidated int

	// Timestamps
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string

	mu sync.RWMutex
}

func (t *GbrainTask) AppendLog(format string, args ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.PhaseLog += time.Now().Format("15:04:05") + " " + fmt.Sprintf(format, args...)
}

func (t *GbrainTask) SetStatus(s GbrainCycleStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = s
}

func (t *GbrainTask) GetStatus() GbrainCycleStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Status
}

func (t *GbrainTask) GetLog() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.PhaseLog
}

// FactEntry is a structured fact matching gbrain's ParsedFact schema.
// Columns: claim | kind | confidence | visibility | notability | valid_from | valid_until | source | context
type FactEntry struct {
	Claim      string  `json:"claim"`
	Kind       string  `json:"kind"`  // fact | event | preference | commitment | belief
	Confidence float64 `json:"confidence"`
	Visibility string  `json:"visibility"`  // private | world
	Notability string  `json:"notability"`  // high | medium | low
	ValidFrom  string  `json:"valid_from,omitempty"`   // ISO date "YYYY-MM-DD"
	ValidUntil string  `json:"valid_until,omitempty"`  // ISO date "YYYY-MM-DD"
	Source     string  `json:"source"`
	Context    string  `json:"context,omitempty"`      // e.g. "superseded by #3" / "forgotten: reason"
	Active     bool    `json:"active"`                 // false when claim is strikethrough
}

// TakeEntry is a structured take matching gbrain's ParsedTake schema.
// Columns: claim | kind | who | weight | since | source
type TakeEntry struct {
	Claim     string  `json:"claim"`
	Kind      string  `json:"kind"`       // take | bet | hunch | fact (open string since v0.38)
	Holder    string  `json:"holder"`     // world | brain | people/<slug> | companies/<slug>
	Weight    float64 `json:"weight"`     // 0..1
	SinceDate string  `json:"since,omitempty"`   // ISO date "YYYY-MM-DD" or "YYYY-MM"
	UntilDate string  `json:"until,omitempty"`   // ISO date
	Source    string  `json:"source"`
	Active    bool    `json:"active"`     // false when claim is strikethrough
}

// PatternEntry is a cross-session pattern discovered across articles.
type PatternEntry struct {
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	ArticleRefs []string `json:"article_refs"` // slugs of articles that exhibit this pattern
}

// CompiledArticle is the in-memory representation of a compiled article.
type CompiledArticle struct {
	Slug    string // e.g. "transformer-architecture.md"
	Content string // Full markdown content
	Score   float64
}
