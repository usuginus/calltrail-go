package grpc

import "context"

type CatalogUsecase interface {
	GetBook(context.Context, string) (*Book, error)
}

type catalogUsecase struct {
	repositories *Repositories
}

type Repositories struct {
	Books *bookRepository
}

var _ CatalogUsecase = (*catalogUsecase)(nil)

func (u *catalogUsecase) GetBook(ctx context.Context, bookID string) (*Book, error) {
	book, err := u.repositories.Books.FindBook(ctx, bookID)
	if err != nil {
		return nil, err
	}
	return book, nil
}
