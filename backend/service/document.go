package service

import (
	"llmwiki/backend/dao"
	"llmwiki/backend/entity"

)

type DocumentService struct {
	docDAO    *dao.DocumentDAO
	kbDAO     *dao.KnowledgebaseDAO
	fileDAO   *dao.FileDAO
	f2dDAO    *dao.File2DocumentDAO
}

func NewDocumentService() *DocumentService {
	return &DocumentService{
		docDAO:  dao.NewDocumentDAO(),
		kbDAO:   dao.NewKnowledgebaseDAO(),
		fileDAO: dao.NewFileDAO(),
		f2dDAO:  dao.NewFile2DocumentDAO(),
	}
}

func (s *DocumentService) CreateDocument(kbID, parserID, createdBy, name, sourceType string) (*entity.Document, error) {
	suffix := ""
	if len(name) > 0 {
		for i := len(name) - 1; i >= 0; i-- {
			if name[i] == '.' {
				suffix = name[i+1:]
				break
			}
		}
	}

	doc := &entity.Document{
		ID:         entity.NewID(),
		KbID:       kbID,
		ParserID:   parserID,
		CreatedBy:  createdBy,
		Name:       &name,
		SourceType: sourceType,
		Suffix:     suffix,
	}
	if err := s.docDAO.Create(doc); err != nil {
		return nil, err
	}

	// Update doc count on KB
	_ = s.kbDAO.UpdateByID(kbID, map[string]interface{}{
		"doc_num": gormExpr("doc_num + 1"),
	})

	return doc, nil
}

func (s *DocumentService) GetDocument(id string) (*entity.Document, error) {
	return s.docDAO.GetByID(id)
}

func (s *DocumentService) ListByKBID(kbID string, page, pageSize int) ([]entity.Document, int64, error) {
	return s.docDAO.ListByKBID(kbID, page, pageSize)
}

func (s *DocumentService) DeleteDocument(id string) error {
	doc, err := s.docDAO.GetByID(id)
	if err != nil {
		return err
	}
	if err := s.docDAO.DeleteByID(id); err != nil {
		return err
	}
	// Update doc count
	_ = s.kbDAO.UpdateByID(doc.KbID, map[string]interface{}{
		"doc_num": gormExpr("GREATEST(doc_num - 1, 0)"),
	})
	return nil
}
