package router

import (
	"llmwiki/backend/handler"

	"github.com/gin-gonic/gin"
)

type Handlers struct {
	File    *handler.FileHandler
	Dataset *handler.DatasetHandler
	Doc     *handler.DocumentHandler
	Commit  *handler.CommitHandler
	Compile *handler.CompileHandler
	Gbrain  *handler.GbrainHandler
}

func Setup(r *gin.Engine, h *Handlers) {
	api := r.Group("/api/v1")

	// Inject tenant_id and user_id middleware
	api.Use(func(c *gin.Context) {
		// In production, extract from auth token. For now use defaults.
		if c.GetString("tenant_id") == "" {
			c.Set("tenant_id", c.Query("tenant_id"))
			if c.GetString("tenant_id") == "" {
				c.Set("tenant_id", "default")
			}
		}
		if c.GetString("user_id") == "" {
			c.Set("user_id", c.GetString("tenant_id"))
		}
		c.Next()
	})

	// ===== File APIs =====
	api.GET("/files", h.File.ListFiles)
	api.POST("/files", h.File.UploadFile)
	api.POST("/files/text", h.File.CreateTextFile)
	api.DELETE("/files", h.File.DeleteFiles)
	api.DELETE("/workspaces/:id", h.File.DeleteWorkspace)
	api.POST("/files/move", h.File.MoveFiles)
	api.POST("/files/folder", h.File.CreateFolder)
	api.GET("/files/:id", h.File.GetFile)
	api.GET("/files/:id/ancestors", h.File.GetFileAncestors)
	api.GET("/folders/:id/tree", h.File.GetFolderTree)
	api.GET("/workspaces/:id/changes", h.File.GetWorkspaceChanges)

	// ===== Dataset APIs =====
	api.GET("/datasets", h.Dataset.ListDatasets)
	api.POST("/datasets", h.Dataset.CreateDataset)
	api.GET("/datasets/:dataset_id", h.Dataset.GetDataset)
	api.DELETE("/datasets", h.Dataset.DeleteDatasets)
	api.GET("/datasets/:dataset_id/tree", h.File.GetFolderTree)

	// ===== Document APIs =====
	api.GET("/datasets/:dataset_id/documents", h.Doc.ListDocuments)
	api.POST("/documents", h.Doc.CreateDocument)
	api.GET("/documents/:id", h.Doc.GetDocument)
	api.DELETE("/documents/:id", h.Doc.DeleteDocument)

	// ===== Commit APIs (Version Control, scoped to workspace folder) =====
	api.POST("/workspaces/:id/commits", h.Commit.CreateCommit)
	api.GET("/workspaces/:id/commits", h.Commit.ListCommits)
	api.GET("/workspaces/:id/commits/diff", h.Commit.DiffCommits)
	api.GET("/workspaces/:id/commits/:commit_id", h.Commit.GetCommit)
	api.GET("/workspaces/:id/commits/:commit_id/files", h.Commit.ListCommitFiles)
	api.GET("/workspaces/:id/commits/:commit_id/files/:file_id/content", h.Commit.GetCommitFileContent)

	// New: Folder-level commit listing and standalone commit tree/file retrieval
	api.GET("/folders/:id/commits", h.Commit.ListFolderCommits)
	api.GET("/commits/:commit_id/tree", h.Commit.GetCommitTreeHandler)
	api.GET("/commits/:commit_id/files/:file_id/content", h.Commit.GetFileAtCommit)

	// ===== Agent Compile API =====
	compile := api.Group("/agent/compile")
	{
		compile.POST("", h.Compile.StartCompile)          // Start async compilation
		compile.GET("/list", h.Compile.ListTasks)         // List all compile tasks
		compile.GET("/:id", h.Compile.GetTask)            // Get task status + logs
		compile.GET("/:id/logs", h.Compile.StreamLogs)    // SSE log streaming
	}

	// ===== Gbrain Cycle API =====
	gbrain := api.Group("/agent/gbrain/cycle")
	{
		gbrain.POST("", h.Gbrain.StartCycle)              // Start gbrain cycle (?force=1 bypasses cooldown)
		gbrain.GET("/list", h.Gbrain.ListCycles)          // List all cycles
		gbrain.GET("/:id", h.Gbrain.GetCycle)             // Get cycle status
		gbrain.DELETE("/cooldown", h.Gbrain.ClearCooldown) // Clear cooldown (?workspace_id=xxx)
	}

	// ===== Gbrain Report API (skip compile, generate report only) =====
	api.POST("/agent/gbrain/report", h.Gbrain.GenerateReport)
}


