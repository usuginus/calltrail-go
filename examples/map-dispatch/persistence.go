package transport

import "context"

type documentStore struct{}

func (s *documentStore) SaveMarkdown(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	return "markdown-document", nil
}
