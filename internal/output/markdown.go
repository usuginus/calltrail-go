package output

import (
	"fmt"
	"io"

	"github.com/usuginus/calltrail-go/internal/model"
)

func WriteMarkdown(w io.Writer, flows []model.APIFlow) error {
	for _, flow := range flows {
		if err := writeFlowHeader(w, flow); err != nil {
			return err
		}

		allCalls := collectCalls(flow)
		writeOperations(w, "Usecases", flow.Trail.Usecases, allCalls, false)
		writeOperations(w, "Services", flow.Trail.Services, allCalls, false)
		writeOperations(w, "Repositories", flow.Trail.Repositories, allCalls, true)
		writeTypes(w, "Models", flow.Trail.Models)
		writeOperations(w, "External Clients", flow.Trail.ExternalClients, allCalls, false)
		writeCalls(w, "Converters", dedupeCalls(flow.Trail.Converters))
		writeCalls(w, "Async", dedupeCalls(flow.Trail.Async))
		writeCalls(w, "Other Notable Calls", summarizeUnknown(flow.Trail.Unknown, operationCallsiteSymbols(flow)))
		writeErrorCodes(w, flow.Errors.GRPCCodes)
	}
	return nil
}

func writeFlowHeader(w io.Writer, flow model.APIFlow) error {
	if _, err := fmt.Fprintf(w, "## %s\n\n", flow.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- kind: `%s`\n", flow.Kind); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- handler: `%s` (%s:%d)\n", flow.Entrypoint.Symbol, flow.Entrypoint.File, flow.Entrypoint.Line); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- request: `%s`\n", flow.Request.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- response: `%s`\n\n", flow.Response.Type); err != nil {
		return err
	}
	return nil
}

func writeOperations(w io.Writer, title string, calls []model.CallRef, allCalls []model.CallRef, repositoryOnly bool) {
	operations := summarizeOperations(calls, allCalls, repositoryOnly)
	if len(operations) == 0 {
		return
	}
	fmt.Fprintf(w, "### %s\n", title)
	for _, operation := range operations {
		fmt.Fprintf(w, "- `%s`\n", operation.Symbol)
		writeCalledFrom(w, operation.CalledFrom)
		if operation.HasImplementation {
			fmt.Fprintf(w, "  - implementation: %s:%d\n", operation.Implementation.File, operation.Implementation.Line)
		}
		writeRelatedCalls(w, operation.Related)
	}
	fmt.Fprintln(w)
}

func writeCalledFrom(w io.Writer, calls []model.CallRef) {
	calls = dedupeCalls(calls)
	if len(calls) == 0 {
		return
	}
	if len(calls) == 1 {
		call := calls[0]
		if call.File == "" {
			fmt.Fprintf(w, "  - called from: `%s`\n", call.Symbol)
			return
		}
		fmt.Fprintf(w, "  - called from: `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
		return
	}
	fmt.Fprintln(w, "  - called from:")
	for _, call := range calls {
		if call.File == "" {
			fmt.Fprintf(w, "    - `%s`\n", call.Symbol)
			continue
		}
		fmt.Fprintf(w, "    - `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
	}
}

func writeRelatedCalls(w io.Writer, calls []model.CallRef) {
	if len(calls) == 0 {
		return
	}
	if len(calls) == 1 {
		call := calls[0]
		fmt.Fprintf(w, "  - related internal call: `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
		return
	}
	fmt.Fprintln(w, "  - related internal calls:")
	for _, call := range calls {
		fmt.Fprintf(w, "    - `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
	}
}

func writeTypes(w io.Writer, title string, types []model.TypeRef) {
	types = dedupeTypes(types)
	if len(types) == 0 {
		return
	}
	fmt.Fprintf(w, "### %s\n", title)
	for _, typ := range types {
		fmt.Fprintf(w, "- `%s`\n", typ.Type)
	}
	fmt.Fprintln(w)
}

func writeCalls(w io.Writer, title string, calls []model.CallRef) {
	if len(calls) == 0 {
		return
	}
	fmt.Fprintf(w, "### %s\n", title)
	for _, call := range calls {
		if call.File == "" {
			fmt.Fprintf(w, "- `%s`\n", call.Symbol)
			continue
		}
		fmt.Fprintf(w, "- `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
	}
	fmt.Fprintln(w)
}

func writeErrorCodes(w io.Writer, codes []string) {
	if len(codes) == 0 {
		return
	}
	fmt.Fprintln(w, "### Error Codes")
	for _, code := range codes {
		fmt.Fprintf(w, "- `%s`\n", code)
	}
	fmt.Fprintln(w)
}
