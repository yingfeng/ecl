package handler

import (
	"llmwiki/backend/service"

	"github.com/gin-gonic/gin"
)

type DocumentHandler struct {
	svc *service.DocumentService
}

func NewDocumentHandler(svc *service.DocumentService) *DocumentHandler {
	return &DocumentHandler{svc: svc}
}

func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	kbID := c.Param("dataset_id")
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "20"), 20)

	docs, total, err := h.svc.ListByKBID(kbID, page, pageSize)
	if err != nil {
		ginAbort(c, 500, "list documents: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"data": docs, "total": total})
}

func (h *DocumentHandler) CreateDocument(c *gin.Context) {
	var req struct {
		KbID       string `json:"kb_id"`
		ParserID   string `json:"parser_id"`
		Name       string `json:"name"`
		SourceType string `json:"source_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.KbID == "" {
		ginAbort(c, 400, "kb_id and name required")
		return
	}
	createdBy := c.GetString("user_id")
	if req.ParserID == "" {
		req.ParserID = "naive"
	}

	doc, err := h.svc.CreateDocument(req.KbID, req.ParserID, createdBy, req.Name, req.SourceType)
	if err != nil {
		ginAbort(c, 500, "create document: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"data": doc})
}

func (h *DocumentHandler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	doc, err := h.svc.GetDocument(id)
	if err != nil {
		ginAbort(c, 404, "document not found")
		return
	}
	ginJSON(c, gin.H{"data": doc})
}

func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.DeleteDocument(id); err != nil {
		ginAbort(c, 500, "delete document: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"ok": true})
}
