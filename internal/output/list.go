package output

import (
	"fmt"
	"io"

	"github.com/usuginus/calltrail-go/internal/model"
)

func WriteList(w io.Writer, flows []model.APIFlow) error {
	for _, flow := range flows {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s:%d\n", flow.Name, flow.Entrypoint.Symbol, flow.Entrypoint.File, flow.Entrypoint.Line); err != nil {
			return err
		}
	}
	return nil
}
