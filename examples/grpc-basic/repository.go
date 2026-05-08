package grpc

import "context"

type bookRepository struct{}

type Book struct {
	ID    string
	Title string
}

func (r *bookRepository) FindBook(ctx context.Context, bookID string) (*Book, error) {
	return &Book{
		ID:    bookID,
		Title: "The Go Programming Language",
	}, nil
}
