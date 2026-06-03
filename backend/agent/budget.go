package agent

import (
	"fmt"
	"sync"
	"time"
)

// BranchBudgetLimits configures all budget thresholds.
type BranchBudgetLimits struct {
	// P0: Source File Budget — max files to fully read per topic
	SourceFileReadMax int
	// P0: Rewrite Retry Budget — max quality gate retries per topic
	RewriteRetryMax int
	// P2: Circuit Breaker — max consecutive failures before skipping a topic
	TopicConsecutiveFailMax int
	// P2: Per-topic timeout
	TopicCompileTimeout time.Duration
	// P0: Global LLM call budget per task
	MaxLLMCalls int
}

var defaultBudgetLimits = BranchBudgetLimits{
	SourceFileReadMax:       3,
	RewriteRetryMax:         2,
	TopicConsecutiveFailMax: 3,
	TopicCompileTimeout:     180 * time.Second,
	MaxLLMCalls:             200,
}

// BranchBudgetTracker implements P0 anti-loop protection and P2 circuit breaker.
// Modeled after iceCoder's BranchBudgetTracker (pure memory, zero LLM cost).
type BranchBudgetTracker struct {
	mu sync.Mutex

	// Per-topic budgets
	sourceFileReadCount map[string]int      // topicName -> files selected for full read
	rewriteRetryCount   map[string]int      // topicName -> quality gate rewrites
	consecutiveFails    map[string]int      // topicName -> consecutive compile failures
	topicStartTimes     map[string]time.Time // topicName -> compile start time

	// Global budgets
	totalLLMCalls int

	limits BranchBudgetLimits
}

// NewBranchBudgetTracker creates a new budget tracker with default limits.
func NewBranchBudgetTracker() *BranchBudgetTracker {
	return &BranchBudgetTracker{
		sourceFileReadCount: make(map[string]int),
		rewriteRetryCount:   make(map[string]int),
		consecutiveFails:    make(map[string]int),
		topicStartTimes:     make(map[string]time.Time),
		limits:              defaultBudgetLimits,
	}
}

// ========== P0: Source File Budget ==========

// CheckFileBudget returns true if the topic can still read more files.
func (b *BranchBudgetTracker) CheckFileBudget(topic string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sourceFileReadCount[topic] < b.limits.SourceFileReadMax
}

// RecordFileRead increments the file read count for a topic.
func (b *BranchBudgetTracker) RecordFileRead(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sourceFileReadCount[topic]++
}

// FileBudgetRemaining returns how many more files can be read for the topic.
func (b *BranchBudgetTracker) FileBudgetRemaining(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	rem := b.limits.SourceFileReadMax - b.sourceFileReadCount[topic]
	if rem < 0 {
		return 0
	}
	return rem
}

// ========== P0: Rewrite Retry Budget ==========

// CheckRewriteBudget returns true if the topic can still retry quality gate.
func (b *BranchBudgetTracker) CheckRewriteBudget(topic string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rewriteRetryCount[topic] < b.limits.RewriteRetryMax
}

// RecordRewrite increments the rewrite count for a topic.
func (b *BranchBudgetTracker) RecordRewrite(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rewriteRetryCount[topic]++
}

// RewriteBudgetRemaining returns how many rewrite retries remain.
func (b *BranchBudgetTracker) RewriteBudgetRemaining(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	rem := b.limits.RewriteRetryMax - b.rewriteRetryCount[topic]
	if rem < 0 {
		return 0
	}
	return rem
}

// ========== P2: Per-topic Circuit Breaker ==========

// CheckCircuitBreaker returns true if the topic should NOT be skipped.
// Returns false (skip) if consecutive failures exceed limit.
func (b *BranchBudgetTracker) CheckCircuitBreaker(topic string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	skipped := b.consecutiveFails[topic] >= b.limits.TopicConsecutiveFailMax
	return !skipped
}

// RecordFailure increments consecutive failures for a topic.
func (b *BranchBudgetTracker) RecordFailure(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails[topic]++
	return b.consecutiveFails[topic]
}

// RecordSuccess resets consecutive failures for a topic.
func (b *BranchBudgetTracker) RecordSuccess(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutiveFails[topic] = 0
}

// ConsecutiveFailCount returns the consecutive failure count for a topic.
func (b *BranchBudgetTracker) ConsecutiveFailCount(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.consecutiveFails[topic]
}

// ========== P2: Per-topic Timeout ==========

// RecordTopicStart records the start time for a topic compile.
func (b *BranchBudgetTracker) RecordTopicStart(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.topicStartTimes[topic] = time.Now()
}

// CheckTopicTimeout returns true if the topic's compile time has exceeded the limit.
func (b *BranchBudgetTracker) CheckTopicTimeout(topic string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	start, ok := b.topicStartTimes[topic]
	if !ok {
		return false
	}
	return time.Since(start) > b.limits.TopicCompileTimeout
}

// ========== Global Budget ==========

// RecordLLMCall increments the global LLM call counter.
func (b *BranchBudgetTracker) RecordLLMCall() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.totalLLMCalls++
	return b.totalLLMCalls
}

// CheckLLMBudget returns true if the task hasn't exceeded max LLM calls.
func (b *BranchBudgetTracker) CheckLLMBudget() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalLLMCalls < b.limits.MaxLLMCalls
}

// LLMCallCount returns the total LLM calls so far.
func (b *BranchBudgetTracker) LLMCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalLLMCalls
}

// ========== Reset ==========

// ResetForNewTopic resets per-topic tracking for clean start (called before each topic compile).
func (b *BranchBudgetTracker) ResetForNewTopic(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sourceFileReadCount[topic] = 0
	b.rewriteRetryCount[topic] = 0
	// Don't reset consecutiveFails — that tracks across topics
}

// Snapshot returns a string representation of current budget state for logging.
func (b *BranchBudgetTracker) Snapshot() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return fmt.Sprintf("LLMCalls=%d, Topics=%d, Rewrites=%d, ConsecutiveFails=%d",
		b.totalLLMCalls, len(b.sourceFileReadCount),
		len(b.rewriteRetryCount), len(b.consecutiveFails))
}

// ========== P2: Execution Mode ==========

// ExecutionMode controls how strictly the pipeline enforces output format.
// Modeled after iceCoder's L1 ModeDecisionEngine.
type ExecutionMode int

const (
	// ModeExplore: Loose mode — LLM can freely discover topics, lenient quality gate
	ModeExplore ExecutionMode = iota
	// ModeStrict: Enforce output format, strict quality gate, structured context
	ModeStrict
)

func (m ExecutionMode) String() string {
	switch m {
	case ModeExplore:
		return "explore"
	case ModeStrict:
		return "strict"
	default:
		return "unknown"
	}
}

// ModeDecider determines the execution mode based on pipeline signals.
type ModeDecider struct {
	mu               sync.Mutex
	currentMode      ExecutionMode
	hadFailures      bool
	lastModeSwitch   time.Time
	cooldownDuration time.Duration
}

// NewModeDecider creates a ModeDecider starting in explore mode.
func NewModeDecider() *ModeDecider {
	return &ModeDecider{
		currentMode:      ModeExplore,
		cooldownDuration: 60 * time.Second, // wait 60s before switching again
	}
}

// CurrentMode returns the current execution mode.
func (d *ModeDecider) CurrentMode() ExecutionMode {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.currentMode
}

// ModeString returns the string representation of current mode.
func (d *ModeDecider) ModeString() string {
	return d.CurrentMode().String()
}

// RecordFailure signals to the decider that a failure occurred.
func (d *ModeDecider) RecordFailure() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.hadFailures = true
}

// Decide evaluates signals and may switch mode (respects cooldown).
func (d *ModeDecider) Decide() ExecutionMode {
	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastModeSwitch) < d.cooldownDuration {
		return d.currentMode
	}

	if d.hadFailures && d.currentMode == ModeExplore {
		// Switch to strict mode after cooldown
		d.currentMode = ModeStrict
		d.lastModeSwitch = time.Now()
		d.hadFailures = false
	}

	return d.currentMode
}
