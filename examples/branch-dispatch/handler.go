package transport

import "context"

type Service struct {
	documentApplication DocumentApplication
}

type ProcessDocumentRequest struct {
	Mode  string
	Asset DocumentAsset
}

type ProcessDocumentResponse struct {
	DocumentID string
}

func (s *Service) ProcessDocument(ctx context.Context, req *ProcessDocumentRequest) (*ProcessDocumentResponse, error) {
	documentID, err := s.documentApplication.ProcessDocument(ctx, ProcessDocumentCommand{
		Mode:  req.Mode,
		Asset: req.Asset,
	})
	if err != nil {
		return nil, err
	}
	return &ProcessDocumentResponse{DocumentID: documentID}, nil
}
