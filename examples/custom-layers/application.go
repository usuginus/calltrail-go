package transport

import "context"

type ArticleApplication interface {
	PublishArticle(context.Context, PublishArticleCommand) (string, error)
}

type PublishArticleCommand struct {
	Title string
	Body  string
}

type articleApplication struct {
	policy *articlePolicy
	store  *articleStore
	index  *searchIndexClient
}

var _ ArticleApplication = (*articleApplication)(nil)

func (a *articleApplication) PublishArticle(ctx context.Context, cmd PublishArticleCommand) (string, error) {
	if err := a.policy.Validate(cmd); err != nil {
		return "", err
	}
	articleID, err := a.store.Insert(ctx, cmd)
	if err != nil {
		return "", err
	}
	if err := a.index.Index(ctx, articleID); err != nil {
		return "", err
	}
	return articleID, nil
}
