package transport

import "context"

type documentStore struct{}

func (s *documentStore) SaveDraft(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	return "draft_123", nil
}

func (s *documentStore) Publish(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	return "document_123", nil
}
