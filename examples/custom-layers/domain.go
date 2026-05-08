package transport

import "errors"

type articlePolicy struct{}

func (p *articlePolicy) Validate(cmd PublishArticleCommand) error {
	if cmd.Title == "" {
		return errors.New("title is required")
	}
	if cmd.Body == "" {
		return errors.New("body is required")
	}
	return nil
}
