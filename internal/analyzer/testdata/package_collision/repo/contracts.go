package repo

import "context"

type TranslationRepo interface {
	Store(context.Context, string) error
}

type UserRepo interface {
	Store(context.Context, string) error
}
