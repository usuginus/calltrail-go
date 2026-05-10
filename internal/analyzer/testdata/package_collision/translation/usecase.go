package translation

import (
	"context"

	"example.com/app/repo"
)

type Translation interface {
	Translate(context.Context, string) error
}

type UseCase struct {
	repo repo.TranslationRepo
}

var _ Translation = (*UseCase)(nil)

func (uc *UseCase) Translate(ctx context.Context, text string) error {
	return uc.repo.Store(ctx, text)
}
