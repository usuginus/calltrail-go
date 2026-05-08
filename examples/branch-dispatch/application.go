package transport

import "context"

type DocumentApplication interface {
	ProcessDocument(context.Context, ProcessDocumentCommand) (string, error)
}

type ProcessDocumentCommand struct {
	Mode  string
	Asset DocumentAsset
}

type DocumentAsset interface {
	documentAsset()
}

type MarkdownAsset struct {
	Body string
}

func (MarkdownAsset) documentAsset() {}

func (a MarkdownAsset) Normalize() string {
	return a.Body
}

type ImageAsset struct {
	URL string
}

func (ImageAsset) documentAsset() {}

func (a ImageAsset) Normalize() string {
	return a.URL
}

type documentApplication struct {
	policy *documentPolicy
	store  *documentStore
	index  *previewClient
}

var _ DocumentApplication = (*documentApplication)(nil)

func (a *documentApplication) ProcessDocument(ctx context.Context, cmd ProcessDocumentCommand) (string, error) {
	switch asset := cmd.Asset.(type) {
	case MarkdownAsset:
		asset.Normalize()
		if err := a.policy.ValidateMarkdown(asset); err != nil {
			return "", err
		}
	case ImageAsset:
		asset.Normalize()
		if err := a.policy.ValidateImage(asset); err != nil {
			return "", err
		}
	default:
		return "", a.policy.RejectUnsupportedAsset()
	}

	switch cmd.Mode {
	case "draft":
		return a.store.SaveDraft(ctx, cmd)
	case "publish":
		documentID, err := a.store.Publish(ctx, cmd)
		if err != nil {
			return "", err
		}
		if err := a.index.Index(ctx, documentID); err != nil {
			return "", err
		}
		return documentID, nil
	default:
		return "", a.policy.RejectUnsupportedMode(cmd.Mode)
	}
}
