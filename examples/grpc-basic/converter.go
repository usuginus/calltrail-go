package grpc

type bookConverter struct{}

func (bookConverter) ToResponse(book *Book) *BookResponse {
	return &BookResponse{
		ID:    book.ID,
		Title: book.Title,
	}
}
