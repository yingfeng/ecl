package dao

import "llmwiki/backend/entity"

type File2DocumentDAO struct{}

func NewFile2DocumentDAO() *File2DocumentDAO { return &File2DocumentDAO{} }

func (d *File2DocumentDAO) GetByFileID(fileID string) ([]entity.File2Document, error) {
	var items []entity.File2Document
	err := DB.Where("file_id = ?", fileID).Find(&items).Error
	return items, err
}

func (d *File2DocumentDAO) GetKBInfoByFileID(fileID string) (kbID, kbName, docID string, err error) {
	row := DB.Table("file2document").
		Select("document.kb_id, knowledgebase.name, file2document.document_id").
		Joins("JOIN document ON file2document.document_id = document.id").
		Joins("JOIN knowledgebase ON document.kb_id = knowledgebase.id").
		Where("file2document.file_id = ?", fileID).
		Limit(1).
		Row()
	err = row.Scan(&kbID, &kbName, &docID)
	return
}

func (d *File2DocumentDAO) Create(item *entity.File2Document) error {
	return DB.Create(item).Error
}

func (d *File2DocumentDAO) DeleteByFileID(fileID string) error {
	return DB.Where("file_id = ?", fileID).Delete(&entity.File2Document{}).Error
}
