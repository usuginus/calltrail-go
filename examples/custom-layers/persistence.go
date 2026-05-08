package transport

import "context"

type articleStore struct{}

func (s *articleStore) Insert(ctx context.Context, cmd PublishArticleCommand) (string, error) {
	return "article_123", nil
}
