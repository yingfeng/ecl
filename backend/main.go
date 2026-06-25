package main

import (
	"context"
	"fmt"
	"log"
	"llmwiki/backend/agent"
	"llmwiki/backend/config"
	"llmwiki/backend/dao"
	"llmwiki/backend/gbrain"
	"llmwiki/backend/handler"
	"llmwiki/backend/router"
	"llmwiki/backend/service"
	"llmwiki/backend/storage"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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

	// 4. Initialize Redis for checkpointing
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		DB:       cfg.RedisDB,
		Username: cfg.RedisUser,
		Password: cfg.RedisPass,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Printf("Redis connected (%s db=%d)", cfg.RedisAddr, cfg.RedisDB)

	// 5. Initialize Agent Compiler
	llmClient := agent.NewLLMClient(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.LLMModel)
	compiler := agent.NewCompiler(fileSvc, llmClient, rdb)
	log.Printf("Agent compiler initialized (model=%s, base=%s)", cfg.LLMModel, cfg.LLMBaseURL)

	// 6. Initialize Gbrain Compiler
	gbrainCompiler := gbrain.NewGbrainCompiler(compiler, fileSvc, llmClient, rdb)
	log.Println("Gbrain cycle compiler initialized")

	// 7. Initialize handlers
	h := &router.Handlers{
		File:    handler.NewFileHandler(fileSvc),
		Dataset: handler.NewDatasetHandler(datasetSvc),
		Doc:     handler.NewDocumentHandler(docSvc),
		Commit:  handler.NewCommitHandler(fileSvc),
		Compile: handler.NewCompileHandler(compiler),
		Gbrain:  handler.NewGbrainHandler(gbrainCompiler),
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
