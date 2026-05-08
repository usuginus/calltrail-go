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
		writeDispatches(w, flow.Trail.Dispatches)
		writeBranches(w, flow)
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
	resolved, unresolved := splitInterfaceCalls(calls)
	if len(resolved) == 0 && len(unresolved) == 0 {
		return
	}
	if len(resolved) > 0 {
		fmt.Fprintln(w, "### Interface Calls")
	}
	for _, call := range resolved {
		writeInterfaceCallHeader(w, call)
		if call.Interface != "" {
			fmt.Fprintf(w, "  - interface: `%s`\n", call.Interface)
		}
		writeInterfaceImplementations(w, call.Implementations)
	}
	if len(resolved) > 0 {
		fmt.Fprintln(w)
	}
	if len(unresolved) > 0 {
		fmt.Fprintln(w, "### Unresolved Interface Calls")
		for _, call := range unresolved {
			writeInterfaceCallHeader(w, call)
			if call.Interface != "" {
				fmt.Fprintf(w, "  - interface: `%s`\n", call.Interface)
			}
		}
		fmt.Fprintln(w)
	}
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
		if hasOnlyInternalHelperImplementations(call) {
			continue
		}
		out = append(out, call)
	}
	return out
}

func splitInterfaceCalls(calls []model.InterfaceCallTrace) ([]model.InterfaceCallTrace, []model.InterfaceCallTrace) {
	var resolved []model.InterfaceCallTrace
	var unresolved []model.InterfaceCallTrace
	for _, call := range calls {
		if len(call.Implementations) == 0 {
			unresolved = append(unresolved, call)
			continue
		}
		resolved = append(resolved, call)
	}
	return resolved, unresolved
}

func hasOnlyInternalHelperImplementations(trace model.InterfaceCallTrace) bool {
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

func writeDispatches(w io.Writer, dispatches []model.DispatchTrace) {
	if len(dispatches) == 0 {
		return
	}
	fmt.Fprintln(w, "### Dispatches")
	for _, dispatch := range dispatches {
		writeDispatchHeader(w, dispatch)
		if dispatch.Interface != "" {
			fmt.Fprintf(w, "  - interface: `%s`\n", dispatch.Interface)
		}
		for _, dispatchCase := range dispatch.Cases {
			fmt.Fprintf(w, "  - %s\n", dispatchCaseTitle(dispatchCase))
			writeDispatchCaseLayers(w, dispatchCase)
			writeDispatchCaseUnknown(w, dispatchCase)
		}
	}
	fmt.Fprintln(w)
}

func writeDispatchHeader(w io.Writer, dispatch model.DispatchTrace) {
	call := dispatch.Call
	if call.File == "" {
		fmt.Fprintf(w, "- `%s` dispatched from `%s`\n", call.Symbol, dispatchLookupDisplay(dispatch))
		return
	}
	fmt.Fprintf(w, "- `%s` dispatched from `%s` (%s:%d)\n", call.Symbol, dispatchLookupDisplay(dispatch), call.File, call.Line)
}

func dispatchLookupDisplay(dispatch model.DispatchTrace) string {
	if dispatch.Key == "" {
		return dispatch.Table
	}
	return dispatch.Table + "[" + dispatch.Key + "]"
}

func writeDispatchCaseLayers(w io.Writer, dispatchCase model.DispatchCase) {
	allCalls := dispatchCaseCalls(dispatchCase)
	for _, layer := range dispatchCase.Layers {
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

func writeDispatchCaseUnknown(w io.Writer, dispatchCase model.DispatchCase) {
	unknown := summarizeUnknown(dispatchCase.Unknown, map[string]bool{})
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

func dispatchCaseCalls(dispatchCase model.DispatchCase) []model.CallRef {
	var calls []model.CallRef
	for _, layer := range dispatchCase.Layers {
		calls = append(calls, layer.Calls...)
	}
	calls = append(calls, dispatchCase.Unknown...)
	return calls
}

func dispatchCaseTitle(dispatchCase model.DispatchCase) string {
	if len(dispatchCase.Labels) == 0 {
		return "case"
	}
	return "case `" + strings.Join(dispatchCase.Labels, "`, `") + "`"
}

func writeBranches(w io.Writer, flow model.APIFlow) {
	if len(flow.Trail.Branches) == 0 {
		return
	}
	fmt.Fprintln(w, "### Branches")
	for _, branch := range flow.Trail.Branches {
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
