package handler

import (
	"llmwiki/backend/agent"
	"llmwiki/backend/gbrain"

	"github.com/gin-gonic/gin"
)

// GbrainHandler provides the gbrain cycle compilation API endpoints.
type GbrainHandler struct {
	compiler *gbrain.GbrainCompiler
}

func NewGbrainHandler(compiler *gbrain.GbrainCompiler) *GbrainHandler {
	return &GbrainHandler{compiler: compiler}
}

// StartCycle POST /api/v1/agent/gbrain/cycle?force=1
func (h *GbrainHandler) StartCycle(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	actor := c.GetString("user_id")
	force := c.Query("force") == "1"

	var req struct {
		WorkspaceID  string   `json:"workspace_id"`
		Instructions string   `json:"instructions"`
		SkillRefs    []string `json:"skill_refs"`
		OutputDir    string   `json:"output_dir"`
		CommitMsg    string   `json:"commit_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginAbort(c, 400, "bad request: "+err.Error())
		return
	}
	if req.WorkspaceID == "" {
		ginAbort(c, 400, "workspace_id required")
		return
	}

	input := &agent.CompileInput{
		WorkspaceID:  req.WorkspaceID,
		TenantID:     tenantID,
		Actor:        actor,
		Instructions: req.Instructions,
		SkillRefs:    req.SkillRefs,
		OutputDir:    req.OutputDir,
		CommitMsg:    req.CommitMsg,
	}

	task, err := h.compiler.StartCycle(c.Request.Context(), input, force)
	if err != nil {
		ginAbort(c, 500, "start gbrain cycle: "+err.Error())
		return
	}

	ginJSON(c, gin.H{"data": gin.H{
		"cycle_id": task.ID,
		"status":   task.GetStatus(),
	}})
}

// GetCycle GET /api/v1/agent/gbrain/cycle/:id
func (h *GbrainHandler) GetCycle(c *gin.Context) {
	taskID := c.Param("id")
	task := h.compiler.GetCycle(taskID)
	if task == nil {
		ginAbort(c, 404, "cycle not found")
		return
	}

	ginJSON(c, gin.H{"data": gin.H{
		"id":                task.ID,
		"status":            task.GetStatus(),
		"log":               task.GetLog(),
		"agent_task_id":     task.AgentTaskID,
		"new_articles":      task.NewArticles,
		"updated_articles":  task.UpdatedArticles,
		"facts_extracted":   task.FactsExtracted,
		"takes_extracted":   task.TakesExtracted,
		"patterns_found":    task.PatternsFound,
		"takes_consolidated": task.TakesConsolidated,
		"created_at":        task.CreatedAt,
		"started_at":        task.StartedAt,
		"finished_at":       task.FinishedAt,
		"error":             task.Error,
	}})
}

// ClearCooldown DELETE /api/v1/agent/gbrain/cycle/cooldown?workspace_id=xxx
func (h *GbrainHandler) ClearCooldown(c *gin.Context) {
	workspaceID := c.Query("workspace_id")
	if workspaceID == "" {
		ginAbort(c, 400, "workspace_id query parameter required")
		return
	}
	if err := h.compiler.ClearCooldown(workspaceID); err != nil {
		ginAbort(c, 500, "clear cooldown: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"data": gin.H{"status": "cooldown_cleared", "workspace_id": workspaceID}})
}

// ListCycles GET /api/v1/agent/gbrain/cycle/list
func (h *GbrainHandler) ListCycles(c *gin.Context) {
	tasks := h.compiler.ListCycles()
	type cycleSummary struct {
		ID              string `json:"id"`
		Status          string `json:"status"`
		NewArticles     int    `json:"new_articles"`
		FactsExtracted  int    `json:"facts_extracted"`
		PatternsFound   int    `json:"patterns_found"`
		CreatedAt       string `json:"created_at"`
		FinishedAt      string `json:"finished_at,omitempty"`
	}
	var result []cycleSummary
	for _, t := range tasks {
		summary := cycleSummary{
			ID:             t.ID,
			Status:         string(t.GetStatus()),
			NewArticles:    t.NewArticles,
			FactsExtracted: t.FactsExtracted,
			PatternsFound:  t.PatternsFound,
			CreatedAt:      t.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if !t.FinishedAt.IsZero() {
			summary.FinishedAt = t.FinishedAt.Format("2006-01-02 15:04:05")
		}
		result = append(result, summary)
	}
	ginJSON(c, gin.H{"data": result})
}

// GenerateReport POST /api/v1/agent/gbrain/report
// Skips agent compile, runs gbrain phases 6-9 on existing output.
func (h *GbrainHandler) GenerateReport(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	actor := c.GetString("user_id")

	var req struct {
		WorkspaceID string `json:"workspace_id"`
		OutputDir   string `json:"output_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginAbort(c, 400, "bad request: "+err.Error())
		return
	}
	if req.WorkspaceID == "" {
		ginAbort(c, 400, "workspace_id required")
		return
	}

	input := &agent.CompileInput{
		WorkspaceID: req.WorkspaceID,
		TenantID:    tenantID,
		Actor:       actor,
		OutputDir:   req.OutputDir,
	}

	task, err := h.compiler.GenerateReport(c.Request.Context(), input)
	if err != nil {
		ginAbort(c, 500, "generate report: "+err.Error())
		return
	}

	ginJSON(c, gin.H{"data": gin.H{
		"cycle_id": task.ID,
		"status":   task.GetStatus(),
	}})
}
