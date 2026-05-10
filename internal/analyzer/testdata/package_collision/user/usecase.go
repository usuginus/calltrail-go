package user

import (
	"context"

	"example.com/app/repo"
)

type UseCase struct {
	repo repo.UserRepo
}

func (uc *UseCase) Register(ctx context.Context, id string) error {
	return uc.repo.Store(ctx, id)
}
