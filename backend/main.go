package main

import (
	"fmt"
	"log"
	"llmwiki/backend/config"
	"llmwiki/backend/dao"
	"llmwiki/backend/handler"
	"llmwiki/backend/router"
	"llmwiki/backend/service"
	"llmwiki/backend/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// 1. Initialize MySQL database (auto-migrate schema)
	if err := dao.Init(cfg.MySQLDSN); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	log.Println("Database initialized and schema migrated.")

	// 2. Initialize MinIO storage
	minioSvc, err := storage.NewMinioStorage(cfg.MinIOHost, cfg.MinIOUser, cfg.MinIOPass, cfg.MinIOSecure)
	if err != nil {
		log.Fatalf("Failed to connect to MinIO: %v", err)
	}
	log.Println("MinIO storage connected.")

	// 3. Initialize services
	fileSvc := service.NewFileService(minioSvc)
	datasetSvc := service.NewDatasetService()
	docSvc := service.NewDocumentService()

	// 4. Initialize handlers
	h := &router.Handlers{
		File:    handler.NewFileHandler(fileSvc),
		Dataset: handler.NewDatasetHandler(datasetSvc),
		Doc:     handler.NewDocumentHandler(docSvc),
		Commit:  handler.NewCommitHandler(fileSvc),
	}

	// 5. Setup Gin router
	r := gin.Default()
	router.Setup(r, h)

	// 6. Start server
	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
