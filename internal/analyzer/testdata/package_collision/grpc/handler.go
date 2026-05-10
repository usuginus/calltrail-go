package grpc

import (
	"context"

	"example.com/app/translation"
)

type Server struct {
	translation translation.Translation
}

type TranslateRequest struct {
	Text string
}

type TranslateResponse struct{}

func (s *Server) Translate(ctx context.Context, req *TranslateRequest) (*TranslateResponse, error) {
	if err := s.translation.Translate(ctx, req.Text); err != nil {
		return nil, err
	}
	return &TranslateResponse{}, nil
}
