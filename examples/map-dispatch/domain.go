package transport

import "fmt"

type documentPolicy struct{}

func (p *documentPolicy) RejectUnsupportedKind(kind DocumentKind) error {
	return fmt.Errorf("unsupported document kind: %s", kind)
}

func (p *documentPolicy) ValidateMarkdown(cmd ProcessDocumentCommand) error {
	if cmd.Body == "" {
		return fmt.Errorf("markdown body is required")
	}
	return nil
}

func (p *documentPolicy) ValidateImage(cmd ProcessDocumentCommand) error {
	if cmd.Body == "" {
		return fmt.Errorf("image body is required")
	}
	return nil
}
