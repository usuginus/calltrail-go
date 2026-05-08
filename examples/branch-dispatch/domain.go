package transport

import "errors"

type documentPolicy struct{}

func (p *documentPolicy) ValidateMarkdown(asset MarkdownAsset) error {
	if asset.Body == "" {
		return errors.New("body is required")
	}
	return nil
}

func (p *documentPolicy) ValidateImage(asset ImageAsset) error {
	if asset.URL == "" {
		return errors.New("URL is required")
	}
	return nil
}

func (p *documentPolicy) RejectUnsupportedAsset() error {
	return errors.New("unsupported asset")
}

func (p *documentPolicy) RejectUnsupportedMode(mode string) error {
	return errors.New("unsupported mode")
}
