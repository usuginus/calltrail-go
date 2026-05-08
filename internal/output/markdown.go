package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
)

func WriteMarkdown(w io.Writer, flows []model.APIFlow) error {
	for _, flow := range flows {
		if err := writeFlowHeader(w, flow); err != nil {
			return err
		}

		allCalls := collectCalls(flow)
		for _, layer := range flow.Trail.Layers {
			writeOperations(w, layer.Name, layer.Calls, allCalls)
		}
		writeInterfaceCalls(w, flow.Trail.InterfaceCalls)
		writeBranches(w, flow.Trail.Branches)
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

func writeOperations(w io.Writer, title string, calls []model.CallRef, allCalls []model.CallRef) {
	operations := summarizeOperations(calls, allCalls)
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

func writeInterfaceCalls(w io.Writer, calls []model.InterfaceCallTrace) {
	calls = summarizeInterfaceCalls(calls)
	if len(calls) == 0 {
		return
	}
	fmt.Fprintln(w, "### Interface Calls")
	for _, call := range calls {
		writeInterfaceCallHeader(w, call)
		if call.Interface != "" {
			fmt.Fprintf(w, "  - interface: `%s`\n", call.Interface)
		}
		writeInterfaceImplementations(w, call.Implementations)
	}
	fmt.Fprintln(w)
}

func writeInterfaceCallHeader(w io.Writer, trace model.InterfaceCallTrace) {
	call := trace.Call
	if call.File == "" {
		fmt.Fprintf(w, "- `%s`\n", call.Symbol)
		return
	}
	fmt.Fprintf(w, "- `%s` (%s:%d)\n", call.Symbol, call.File, call.Line)
}

func writeInterfaceImplementations(w io.Writer, implementations []model.ImplementationCandidate) {
	if len(implementations) == 0 {
		return
	}
	fmt.Fprintln(w, "  - candidates:")
	for _, implementation := range implementations {
		call := implementation.Call
		status := "candidate"
		if implementation.Expanded {
			status = "expanded"
		}
		if call.File == "" {
			fmt.Fprintf(w, "    - `%s` %s\n", call.Symbol, status)
			continue
		}
		fmt.Fprintf(w, "    - `%s` (%s:%d) %s\n", call.Symbol, call.File, call.Line, status)
	}
}

func summarizeInterfaceCalls(calls []model.InterfaceCallTrace) []model.InterfaceCallTrace {
	var out []model.InterfaceCallTrace
	for _, call := range calls {
		if isLowSignalInterfaceCall(call) {
			continue
		}
		out = append(out, call)
	}
	return out
}

func isLowSignalInterfaceCall(trace model.InterfaceCallTrace) bool {
	method := strings.ToLower(trace.Call.Method)
	if method == "now" || strings.Contains(method, "timestamp") {
		return true
	}
	if strings.Contains(strings.ToLower(trace.Interface), "clock") {
		return true
	}
	if len(trace.Implementations) == 0 {
		return false
	}
	for _, implementation := range trace.Implementations {
		if !isInternalHelperCall(implementation.Call) {
			return false
		}
	}
	return true
}

func writeBranches(w io.Writer, branches []model.BranchTrace) {
	if len(branches) == 0 {
		return
	}
	fmt.Fprintln(w, "### Branches")
	for _, branch := range branches {
		writeBranchHeader(w, branch)
		for _, branchCase := range branch.Cases {
			fmt.Fprintf(w, "  - %s\n", branchCaseTitle(branchCase))
			writeBranchCaseLayers(w, branchCase)
			writeBranchCaseUnknown(w, branchCase)
		}
	}
	fmt.Fprintln(w)
}

func writeBranchHeader(w io.Writer, branch model.BranchTrace) {
	fmt.Fprintf(w, "- `%s` %s", branch.Function, branchKindLabel(branch.Kind))
	if branch.Expr != "" {
		fmt.Fprintf(w, " `%s`", branch.Expr)
	}
	if branch.File != "" {
		fmt.Fprintf(w, " (%s:%d)", branch.File, branch.Line)
	}
	fmt.Fprintln(w)
}

func writeBranchCaseLayers(w io.Writer, branchCase model.BranchCase) {
	allCalls := branchCaseCalls(branchCase)
	for _, layer := range branchCase.Layers {
		operations := summarizeOperations(layer.Calls, allCalls)
		if len(operations) == 0 {
			continue
		}
		if len(operations) == 1 {
			fmt.Fprintf(w, "    - %s: `%s`\n", layer.Name, operations[0].Symbol)
			continue
		}
		fmt.Fprintf(w, "    - %s:\n", layer.Name)
		for _, operation := range operations {
			fmt.Fprintf(w, "      - `%s`\n", operation.Symbol)
		}
	}
}

func writeBranchCaseUnknown(w io.Writer, branchCase model.BranchCase) {
	unknown := summarizeUnknown(branchCase.Unknown, map[string]bool{})
	if len(unknown) == 0 {
		return
	}
	if len(unknown) == 1 {
		fmt.Fprintf(w, "    - other: `%s`\n", unknown[0].Symbol)
		return
	}
	fmt.Fprintln(w, "    - other:")
	for _, call := range unknown {
		fmt.Fprintf(w, "      - `%s`\n", call.Symbol)
	}
}

func branchCaseCalls(branchCase model.BranchCase) []model.CallRef {
	var calls []model.CallRef
	for _, layer := range branchCase.Layers {
		calls = append(calls, layer.Calls...)
	}
	calls = append(calls, branchCase.Unknown...)
	return calls
}

func branchKindLabel(kind string) string {
	switch kind {
	case "type_switch":
		return "type switch"
	default:
		return "switch"
	}
}

func branchCaseTitle(branchCase model.BranchCase) string {
	if branchCase.Default {
		return "default"
	}
	if len(branchCase.Labels) == 0 {
		return "case"
	}
	return "case `" + strings.Join(branchCase.Labels, "`, `") + "`"
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
