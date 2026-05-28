package service

import (
	"fmt"
	"llmwiki/backend/entity"
	"path"
	"strings"

)

// GetFileByID returns a file by its ID.
func (s *FileService) GetFileByID(fileID string) (*entity.File, error) {
	return s.fileDAO.GetByID(fileID)
}

// GetAllParentFolders returns all ancestor folders for a file.
func (s *FileService) GetAllParentFolders(fileID, tenantID string) ([]entity.File, error) {
	return s.fileDAO.GetAllParentFolders(fileID, tenantID)
}

// GetStorageData retrieves raw data from MinIO for the given bucket/key.
func (s *FileService) GetStorageData(bucket, key string) ([]byte, error) {
	return s.storageImpl.Get(bucket, key)
}

// GetDatasetIDByFileID returns the KB ID associated with a file.
func (s *FileService) GetDatasetIDByFileID(fileID string) (string, error) {
	return s.fileDAO.GetDatasetIDByFileID(fileID)
}

// GetDatasetTree returns the root folder tree for a dataset.
func (s *FileService) GetDatasetTree(kbID string) (*entity.TreeNode, error) {
	kb, err := s.kbDAO.GetByID(kbID)
	if err != nil {
		return nil, err
	}
	root, err := s.fileDAO.GetRootFolder(kb.TenantID)
	if err != nil {
		// Auto-create root folder if it doesn't exist
		root, err = s.fileDAO.CreateRootFolder(kb.TenantID, kb.CreatedBy)
		if err != nil {
			return nil, err
		}
	}
	return s.GetCurrentTree(root.ID)
}

// CreateTextFile creates a new text file with the given content under a parent folder.
func (s *FileService) CreateTextFile(tenantID, parentID, name, content, createdBy string) (*entity.File, error) {
	if parentID == "" {
		root, err := s.GetOrCreateRootFolder(tenantID, createdBy)
		if err != nil {
			return nil, err
		}
		parentID = root.ID
	}

	data := []byte(content)
	contentHash := sha256Hex(data)
	ext := path.Ext(name)
	location := fmt.Sprintf("%s/%s%s", parentID, contentHash[:16], ext)

	if err := s.storageImpl.Put(parentID, location, data); err != nil {
		return nil, fmt.Errorf("storage put %s: %w", name, err)
	}

	fileType := strings.TrimPrefix(ext, ".")
	if fileType == "" {
		fileType = "other"
	}

	fileRec := &entity.File{
		ID:         entity.NewID(),
		ParentID:   parentID,
		TenantID:   tenantID,
		CreatedBy:  createdBy,
		Name:       name,
		Location:   &location,
		Size:       int64(len(data)),
		Type:       fileType,
		SourceType: "local",
	}
	if err := s.fileDAO.Create(fileRec); err != nil {
		return nil, fmt.Errorf("create file record: %w", err)
	}
	return fileRec, nil
}
