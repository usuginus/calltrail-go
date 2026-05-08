package transport

import "context"

type previewClient struct{}

func (c *previewClient) RenderImage(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	return "image-preview", nil
}
