package transport

import "context"

type DocumentKind string

const (
	KindMarkdown DocumentKind = "markdown"
	KindImage    DocumentKind = "image"
)

type ProcessDocumentCommand struct {
	Kind DocumentKind
	Body string
}

type DocumentApplication interface {
	ProcessDocument(context.Context, ProcessDocumentCommand) (string, error)
}

type DocumentProcessor interface {
	Process(context.Context, ProcessDocumentCommand) (string, error)
}

type documentApplication struct {
	policy     *documentPolicy
	processors map[DocumentKind]DocumentProcessor
}

var _ DocumentApplication = (*documentApplication)(nil)

func NewDocumentApplication(store *documentStore, preview *previewClient) DocumentApplication {
	return &documentApplication{
		policy: &documentPolicy{},
		processors: map[DocumentKind]DocumentProcessor{
			KindMarkdown: newMarkdownProcessor(store),
			KindImage:    newImageProcessor(preview),
		},
	}
}

func (a *documentApplication) ProcessDocument(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	processor, ok := a.processors[cmd.Kind]
	if !ok {
		return "", a.policy.RejectUnsupportedKind(cmd.Kind)
	}
	return processor.Process(ctx, cmd)
}

type markdownProcessor struct {
	policy *documentPolicy
	store  *documentStore
}

func newMarkdownProcessor(store *documentStore) DocumentProcessor {
	return &markdownProcessor{
		policy: &documentPolicy{},
		store:  store,
	}
}

func (p *markdownProcessor) Process(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	if err := p.policy.ValidateMarkdown(cmd); err != nil {
		return "", err
	}
	return p.store.SaveMarkdown(ctx, cmd)
}

type imageProcessor struct {
	policy  *documentPolicy
	preview *previewClient
}

func newImageProcessor(preview *previewClient) DocumentProcessor {
	return &imageProcessor{
		policy:  &documentPolicy{},
		preview: preview,
	}
}

func (p *imageProcessor) Process(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	if err := p.policy.ValidateImage(cmd); err != nil {
		return "", err
	}
	return p.preview.RenderImage(ctx, cmd)
}
