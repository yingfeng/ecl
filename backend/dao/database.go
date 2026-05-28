package dao

import (
	"llmwiki/backend/entity"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dsn string) error {
	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return err
	}
	return DB.AutoMigrate(
		&entity.Knowledgebase{},
		&entity.Document{},
		&entity.File{},
		&entity.File2Document{},
		&entity.FileCommit{},
		&entity.FileCommitItem{},
	)
}
