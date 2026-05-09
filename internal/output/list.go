package output

import (
	"fmt"
	"io"

	"github.com/usuginus/calltrail-go/internal/model"
)

func WriteList(w io.Writer, flows []model.APIFlow) error {
	if len(flows) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "| rpc | handler | location |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- |"); err != nil {
		return err
	}
	for _, flow := range flows {
		if _, err := fmt.Fprintf(
			w,
			"| `%s` | `%s` | `%s:%d` |\n",
			flow.Name,
			flow.Entrypoint.Symbol,
			flow.Entrypoint.File,
			flow.Entrypoint.Line,
		); err != nil {
			return err
		}
	}
	return nil
}
