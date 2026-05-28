package dao

import "llmwiki/backend/entity"

type KnowledgebaseDAO struct{}

func NewKnowledgebaseDAO() *KnowledgebaseDAO { return &KnowledgebaseDAO{} }

func (d *KnowledgebaseDAO) GetByID(id string) (*entity.Knowledgebase, error) {
	var kb entity.Knowledgebase
	err := DB.Where("id = ?", id).First(&kb).Error
	return &kb, err
}

func (d *KnowledgebaseDAO) Create(kb *entity.Knowledgebase) error {
	return DB.Create(kb).Error
}

func (d *KnowledgebaseDAO) UpdateByID(id string, updates map[string]interface{}) error {
	return DB.Model(&entity.Knowledgebase{}).Where("id = ?", id).Updates(updates).Error
}

func (d *KnowledgebaseDAO) DeleteByID(id string) error {
	return DB.Where("id = ?", id).Delete(&entity.Knowledgebase{}).Error
}

func (d *KnowledgebaseDAO) ListByTenantID(tenantID string, page, pageSize int, keywords string) ([]entity.Knowledgebase, int64, error) {
	var kbs []entity.Knowledgebase
	query := DB.Where("tenant_id = ?", tenantID)
	if keywords != "" {
		query = query.Where("name LIKE ?", "%"+keywords+"%")
	}
	var total int64
	query.Model(&entity.Knowledgebase{}).Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("create_time DESC").Find(&kbs).Error
	return kbs, total, err
}
