package persistent

import (
	"context"

	"example.com/app/repo"
)

type TranslationRepo struct{}

var _ repo.TranslationRepo = (*TranslationRepo)(nil)

func (r *TranslationRepo) Store(context.Context, string) error {
	return nil
}

type UserRepo struct{}

var _ repo.UserRepo = (*UserRepo)(nil)

func (r *UserRepo) Store(context.Context, string) error {
	return nil
}
