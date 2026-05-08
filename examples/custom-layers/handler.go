package transport

import "context"

type Service struct {
	articleApplication ArticleApplication
}

type PublishArticleRequest struct {
	Title string
	Body  string
}

type PublishArticleResponse struct {
	ArticleID string
}

func (s *Service) PublishArticle(ctx context.Context, req *PublishArticleRequest) (*PublishArticleResponse, error) {
	articleID, err := s.articleApplication.PublishArticle(ctx, PublishArticleCommand{
		Title: req.Title,
		Body:  req.Body,
	})
	if err != nil {
		return nil, err
	}
	return &PublishArticleResponse{ArticleID: articleID}, nil
}
