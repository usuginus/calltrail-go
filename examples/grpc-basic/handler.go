package grpc

import "context"

type Server struct {
	catalogUsecase CatalogUsecase
	bookConverter  bookConverter
}

type GetBookRequest struct {
	BookID string
}

type BookResponse struct {
	ID    string
	Title string
}

func (s *Server) GetBook(ctx context.Context, req *GetBookRequest) (*BookResponse, error) {
	book, err := s.catalogUsecase.GetBook(ctx, req.BookID)
	if err != nil {
		return nil, err
	}
	return s.bookConverter.ToResponse(book), nil
}
