package transport

import "context"

type searchIndexClient struct{}

func (c *searchIndexClient) Index(ctx context.Context, articleID string) error {
	return nil
}
