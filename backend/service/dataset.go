package service

import (
	"llmwiki/backend/dao"
	"llmwiki/backend/entity"

)

type DatasetService struct {
	kbDAO     *dao.KnowledgebaseDAO
	docDAO    *dao.DocumentDAO
	fileDAO   *dao.FileDAO
}

func NewDatasetService() *DatasetService {
	return &DatasetService{
		kbDAO:   dao.NewKnowledgebaseDAO(),
		docDAO:  dao.NewDocumentDAO(),
		fileDAO: dao.NewFileDAO(),
	}
}

func (s *DatasetService) ListDatasets(tenantID string, page, pageSize int, keywords string) ([]entity.Knowledgebase, int64, error) {
	return s.kbDAO.ListByTenantID(tenantID, page, pageSize, keywords)
}

func (s *DatasetService) GetDataset(id string) (*entity.Knowledgebase, error) {
	return s.kbDAO.GetByID(id)
}

func (s *DatasetService) CreateDataset(tenantID, name, embdID, createdBy string) (*entity.Knowledgebase, error) {
	kb := &entity.Knowledgebase{
		ID:        entity.NewID(),
		TenantID:  tenantID,
		Name:      name,
		EmbdID:    embdID,
		CreatedBy: createdBy,
		ParserID:  string(entity.ParserTypeNaive),
	}
	if err := s.kbDAO.Create(kb); err != nil {
		return nil, err
	}
	return kb, nil
}

func (s *DatasetService) DeleteDataset(id string) error {
	// Delete associated documents
	docs, _, _ := s.docDAO.ListByKBID(id, 1, 10000)
	for _, doc := range docs {
		_ = s.docDAO.DeleteByID(doc.ID)
	}
	return s.kbDAO.DeleteByID(id)
}

func (s *DatasetService) GetDatasetSize(kbID string) (int64, error) {
	return s.docDAO.SumSizeByDatasetID(kbID)
}
