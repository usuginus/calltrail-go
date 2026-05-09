package grpc

import "context"

type CreateDocumentRequest struct{}

type CreateDocumentResponse struct{}

type debugService struct{}

type userService struct{}

func (s *debugService) CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*CreateDocumentResponse, error) {
	return &CreateDocumentResponse{}, nil
}

func (s *userService) CreateDocument(ctx context.Context, req *CreateDocumentRequest) (*CreateDocumentResponse, error) {
	return &CreateDocumentResponse{}, nil
}
