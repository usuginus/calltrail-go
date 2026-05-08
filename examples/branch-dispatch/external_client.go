package transport

import "context"

type previewClient struct{}

func (c *previewClient) Index(ctx context.Context, documentID string) error {
	return nil
}
