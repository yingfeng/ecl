package dao

import (
	"llmwiki/backend/entity"

	"gorm.io/gorm"
)

type FileDAO struct{}

func NewFileDAO() *FileDAO { return &FileDAO{} }

func (d *FileDAO) GetByID(id string) (*entity.File, error) {
	var f entity.File
	err := DB.Where("id = ?", id).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (d *FileDAO) GetByParentID(parentID string, page, pageSize int, keywords string) ([]entity.File, int64, error) {
	var files []entity.File
	query := DB.Where("parent_id = ?", parentID)
	if keywords != "" {
		query = query.Where("name LIKE ?", "%"+keywords+"%")
	}
	var total int64
	query.Model(&entity.File{}).Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("create_time DESC").Find(&files).Error
	return files, total, err
}

func (d *FileDAO) GetByIDs(ids []string) ([]entity.File, error) {
	var files []entity.File
	err := DB.Where("id IN ?", ids).Find(&files).Error
	return files, err
}

func (d *FileDAO) GetRootFolder(tenantID string) (*entity.File, error) {
	var f entity.File
	err := DB.Where("tenant_id = ? AND parent_id = id AND type = 'folder'", tenantID).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (d *FileDAO) CreateRootFolder(tenantID, createdBy string) (*entity.File, error) {
	id := entity.NewID()
	f := &entity.File{
		ID:        id,
		ParentID:  id,
		TenantID:  tenantID,
		CreatedBy: createdBy,
		Name:      "root",
		Type:      "folder",
	}
	err := DB.Create(f).Error
	return f, err
}

func (d *FileDAO) Create(f *entity.File) error {
	if f.ID == "" {
		f.ID = entity.NewID()
	}
	return DB.Create(f).Error
}

func (d *FileDAO) UpdateByID(id string, updates map[string]interface{}) error {
	return DB.Model(&entity.File{}).Where("id = ?", id).Updates(updates).Error
}

func (d *FileDAO) DeleteByID(tx *gorm.DB, id string) error {
	db := tx
	if db == nil {
		db = DB
	}
	return db.Where("id = ?", id).Delete(&entity.File{}).Error
}

func (d *FileDAO) DeleteByIDs(ids []string) error {
	return DB.Where("id IN ?", ids).Delete(&entity.File{}).Error
}

func (d *FileDAO) GetByParentIDAndName(parentID, name string) (*entity.File, error) {
	var f entity.File
	err := DB.Where("parent_id = ? AND name = ?", parentID, name).First(&f).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (d *FileDAO) ListChildIDs(parentID string) ([]string, error) {
	var ids []string
	err := DB.Model(&entity.File{}).Where("parent_id = ?", parentID).Pluck("id", &ids).Error
	return ids, err
}

func (d *FileDAO) GetDatasetIDByFileID(fileID string) (string, error) {
	var kbID string
	err := DB.Table("file2document").
		Select("document.kb_id").
		Joins("JOIN document ON file2document.document_id = document.id").
		Where("file2document.file_id = ?", fileID).
		Limit(1).
		Pluck("document.kb_id", &kbID).Error
	return kbID, err
}

func (d *FileDAO) GetParentFolder(fileID, tenantID string) (*entity.File, error) {
	f, err := d.GetByID(fileID)
	if err != nil {
		return nil, err
	}
	var parent entity.File
	err = DB.Where("id = ? AND tenant_id = ?", f.ParentID, tenantID).First(&parent).Error
	return &parent, err
}

func (d *FileDAO) GetAllParentFolders(fileID, tenantID string) ([]entity.File, error) {
	var chain []entity.File
	current, err := d.GetByID(fileID)
	if err != nil {
		return nil, err
	}
	for {
		if current.ParentID == current.ID {
			chain = append([]entity.File{*current}, chain...)
			break
		}
		chain = append([]entity.File{*current}, chain...)
		current, err = d.GetByID(current.ParentID)
		if err != nil {
			return nil, err
		}
	}
	return chain, nil
}
