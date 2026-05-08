package transport

import "context"

type Service struct {
	app DocumentApplication
}

type ProcessDocumentRequest struct {
	Kind DocumentKind
	Body string
}

type ProcessDocumentResponse struct {
	Result string
}

func (s *Service) ProcessDocument(ctx context.Context, req *ProcessDocumentRequest) (*ProcessDocumentResponse, error) {
	result, err := s.app.ProcessDocument(ctx, ProcessDocumentCommand{
		Kind: req.Kind,
		Body: req.Body,
	})
	if err != nil {
		return nil, err
	}
	return &ProcessDocumentResponse{Result: result}, nil
}
