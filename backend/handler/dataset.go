package handler

import (
	"llmwiki/backend/service"

	"github.com/gin-gonic/gin"
)

type DatasetHandler struct {
	svc *service.DatasetService
}

func NewDatasetHandler(svc *service.DatasetService) *DatasetHandler {
	return &DatasetHandler{svc: svc}
}

func (h *DatasetHandler) ListDatasets(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "20"), 20)
	keywords := c.Query("keywords")

	kbs, total, err := h.svc.ListDatasets(tenantID, page, pageSize, keywords)
	if err != nil {
		ginAbort(c, 500, "list datasets: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"data": kbs, "total": total})
}

func (h *DatasetHandler) GetDataset(c *gin.Context) {
	id := c.Param("dataset_id")
	kb, err := h.svc.GetDataset(id)
	if err != nil {
		ginAbort(c, 404, "dataset not found")
		return
	}
	size, _ := h.svc.GetDatasetSize(id)
	ginJSON(c, gin.H{"data": kb, "size": size})
}

func (h *DatasetHandler) CreateDataset(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	createdBy := c.GetString("user_id")

	var req struct {
		Name   string `json:"name"`
		EmbdID string `json:"embd_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		ginAbort(c, 400, "name required")
		return
	}
	if req.EmbdID == "" {
		req.EmbdID = "default"
	}

	kb, err := h.svc.CreateDataset(tenantID, req.Name, req.EmbdID, createdBy)
	if err != nil {
		ginAbort(c, 500, "create dataset: "+err.Error())
		return
	}
	ginJSON(c, gin.H{"data": kb})
}

func (h *DatasetHandler) DeleteDatasets(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		ginAbort(c, 400, "ids required")
		return
	}
	for _, id := range req.IDs {
		if err := h.svc.DeleteDataset(id); err != nil {
			ginAbort(c, 500, "delete dataset: "+err.Error())
			return
		}
	}
	ginJSON(c, gin.H{"ok": true})
}
