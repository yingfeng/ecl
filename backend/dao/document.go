package dao

import "llmwiki/backend/entity"

type DocumentDAO struct{}

func NewDocumentDAO() *DocumentDAO { return &DocumentDAO{} }

func (d *DocumentDAO) GetByID(id string) (*entity.Document, error) {
	var doc entity.Document
	err := DB.Where("id = ?", id).First(&doc).Error
	return &doc, err
}

func (d *DocumentDAO) Create(doc *entity.Document) error {
	return DB.Create(doc).Error
}

func (d *DocumentDAO) UpdateByID(id string, updates map[string]interface{}) error {
	return DB.Model(&entity.Document{}).Where("id = ?", id).Updates(updates).Error
}

func (d *DocumentDAO) DeleteByID(id string) error {
	return DB.Where("id = ?", id).Delete(&entity.Document{}).Error
}

func (d *DocumentDAO) ListByKBID(kbID string, page, pageSize int) ([]entity.Document, int64, error) {
	var docs []entity.Document
	var total int64
	DB.Model(&entity.Document{}).Where("kb_id = ?", kbID).Count(&total)
	err := DB.Where("kb_id = ?", kbID).Offset((page - 1) * pageSize).Limit(pageSize).Order("create_time DESC").Find(&docs).Error
	return docs, total, err
}

func (d *DocumentDAO) SumSizeByDatasetID(kbID string) (int64, error) {
	var sum int64
	err := DB.Model(&entity.Document{}).Where("kb_id = ?", kbID).Select("COALESCE(SUM(size), 0)").Scan(&sum).Error
	return sum, err
}
