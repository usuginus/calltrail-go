package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
)

func WriteMarkdown(w io.Writer, flows []model.APIFlow) error {
	for _, flow := range flows {
		if _, err := fmt.Fprintf(w, "## %s\n\n", flow.Name); err != nil {
			return err
		}
		writeExecutionSummary(w, flow)
		writeLayerSummary(w, flow)
		writeDecisionPoints(w, flow)
	}
	return nil
}

func writeExecutionSummary(w io.Writer, flow model.APIFlow) {
	fmt.Fprintln(w, "### execution summary")
	fmt.Fprintf(w, "- kind: `%s`\n", flow.Kind)
	fmt.Fprintf(w, "- handler: %s\n", callReference(model.CallRef{
		Symbol: flow.Entrypoint.Symbol,
		File:   flow.Entrypoint.File,
		Line:   flow.Entrypoint.Line,
	}))
	fmt.Fprintf(w, "- request: `%s`\n", flow.Request.Type)
	fmt.Fprintf(w, "- response: `%s`\n", flow.Response.Type)

	if counts := layerOperationCounts(flow); len(counts) > 0 {
		fmt.Fprintln(w, "- layers:")
		for _, count := range counts {
			fmt.Fprintf(w, "  - %s: %d operations\n", count.Name, count.Count)
		}
	}

	interfaceCalls, branches, dispatches := decisionPointCounts(flow)
	fmt.Fprintln(w, "- decision points:")
	fmt.Fprintf(w, "  - interface calls: %d\n", interfaceCalls)
	fmt.Fprintf(w, "  - branches: %d\n", branches)
	fmt.Fprintf(w, "  - dispatches: %d\n\n", dispatches)
}

type layerOperationCount struct {
	Name  string
	Count int
}

func layerOperationCounts(flow model.APIFlow) []layerOperationCount {
	allCalls := collectCalls(flow)
	var counts []layerOperationCount
	for _, layer := range collectLayerCalls(flow) {
		count := len(sortedOperationSummaries(layer.Calls, allCalls))
		if count == 0 {
			continue
		}
		counts = append(counts, layerOperationCount{Name: layer.Name, Count: count})
	}
	return counts
}

func decisionPointCounts(flow model.APIFlow) (interfaceCalls int, branches int, dispatches int) {
	interfaceCalls = len(summarizeInterfaceCalls(flow.Trail.InterfaceCalls))
	branches = len(flow.Trail.Branches)
	dispatches = len(flow.Trail.Dispatches)
	return interfaceCalls, branches, dispatches
}

func writeLayerSummary(w io.Writer, flow model.APIFlow) {
	allCalls := collectCalls(flow)
	wrote := false
	for _, layer := range collectLayerCalls(flow) {
		if writeOperations(w, &wrote, layer.Name, layer.Calls, allCalls) {
			continue
		}
	}
	writeCalls(w, &wrote, "async", sortCalls(dedupeCalls(flow.Trail.Async)))
	writeCalls(w, &wrote, "other", sortCalls(summarizeUnknown(flow.Trail.Unknown, operationCallsiteSymbols(flow))))
}

func writeOperations(w io.Writer, wrote *bool, title string, calls []model.CallRef, allCalls []model.CallRef) bool {
	operations := sortedOperationSummaries(calls, allCalls)
	if len(operations) == 0 {
		return false
	}
	writeLayerSummaryHeading(w, wrote)
	fmt.Fprintf(w, "#### %s\n", title)
	for _, operation := range operations {
		fmt.Fprintf(w, "- `%s`\n", operation.Symbol)
		writeCalledFrom(w, operation.CalledFrom)
		if operation.HasImplementation {
			fmt.Fprintf(w, "  - implementation: %s:%d\n", operation.Implementation.File, operation.Implementation.Line)
		}
		writeRelatedCalls(w, operation.Related)
	}
	fmt.Fprintln(w)
	return true
}

func writeLayerSummaryHeading(w io.Writer, wrote *bool) {
	if *wrote {
		return
	}
	fmt.Fprintln(w, "### layer summary")
	*wrote = true
}

func writeCalledFrom(w io.Writer, calls []model.CallRef) {
	calls = sortCalls(dedupeCalls(calls))
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
	calls = sortCalls(dedupeCalls(calls))
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

func writeCalls(w io.Writer, wrote *bool, title string, calls []model.CallRef) {
	if len(calls) == 0 {
		return
	}
	writeLayerSummaryHeading(w, wrote)
	fmt.Fprintf(w, "#### %s\n", title)
	for _, call := range calls {
		fmt.Fprintf(w, "- %s\n", callReference(call))
	}
	fmt.Fprintln(w)
}

func writeDecisionPoints(w io.Writer, flow model.APIFlow) {
	hasInterfaceCalls := len(summarizeInterfaceCalls(flow.Trail.InterfaceCalls)) > 0
	hasBranches := len(flow.Trail.Branches) > 0
	hasDispatches := len(flow.Trail.Dispatches) > 0
	if !hasInterfaceCalls && !hasBranches && !hasDispatches {
		return
	}

	fmt.Fprintln(w, "### decision points")
	writeInterfaceCallsTable(w, flow.Trail.InterfaceCalls)
	writeBranchesTable(w, flow.Trail.Branches)
	writeDispatchesTable(w, flow.Trail.Dispatches)
}

func writeInterfaceCallsTable(w io.Writer, calls []model.InterfaceCallTrace) {
	calls = sortedInterfaceCalls(summarizeInterfaceCalls(calls))
	if len(calls) == 0 {
		return
	}

	fmt.Fprintln(w, "#### interface calls")
	fmt.Fprintln(w, "| call | interface | candidates | resolution |")
	fmt.Fprintln(w, "| --- | --- | --- | --- |")
	for _, call := range calls {
		fmt.Fprintf(
			w,
			"| %s | %s | %s | %s |\n",
			tableCell(callReference(call.Call)),
			tableCell(inlineCode(call.Interface)),
			tableCell(interfaceCandidatesCell(call.Implementations)),
			tableCell(interfaceResolution(call.Implementations)),
		)
	}
	fmt.Fprintln(w)
}

func sortedInterfaceCalls(calls []model.InterfaceCallTrace) []model.InterfaceCallTrace {
	out := append([]model.InterfaceCallTrace(nil), calls...)
	for i := range out {
		out[i].Implementations = sortImplementationCandidates(out[i].Implementations)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Interface != out[j].Interface {
			return out[i].Interface < out[j].Interface
		}
		return callLess(out[i].Call, out[j].Call)
	})
	return out
}

func interfaceCandidatesCell(candidates []model.ImplementationCandidate) string {
	if len(candidates) == 0 {
		return "-"
	}
	var parts []string
	for _, candidate := range candidates {
		status := "candidate"
		if candidate.Expanded {
			status = "expanded"
		}
		parts = append(parts, fmt.Sprintf("%s %s", callReference(candidate.Call), status))
	}
	return strings.Join(parts, "<br>")
}

func interfaceResolution(candidates []model.ImplementationCandidate) string {
	if len(candidates) == 0 {
		return "unresolved"
	}
	expanded := 0
	for _, candidate := range candidates {
		if candidate.Expanded {
			expanded++
		}
	}
	switch {
	case len(candidates) == 1 && expanded == 1:
		return "single expanded"
	case len(candidates) == 1:
		return "single candidate"
	case expanded == len(candidates):
		return "multiple expanded"
	case expanded == 0:
		return "multiple candidates"
	default:
		return "partial"
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

func writeBranchesTable(w io.Writer, branches []model.BranchTrace) {
	if len(branches) == 0 {
		return
	}

	fmt.Fprintln(w, "#### branches")
	fmt.Fprintln(w, "| function | condition | case | calls |")
	fmt.Fprintln(w, "| --- | --- | --- | --- |")
	for _, branch := range branches {
		for _, branchCase := range branch.Cases {
			fmt.Fprintf(
				w,
				"| %s | %s | %s | %s |\n",
				tableCell(branchFunctionCell(branch)),
				tableCell(branchCondition(branch)),
				tableCell(branchCaseTitle(branchCase)),
				tableCell(branchCaseCallsCell(branch, branchCase)),
			)
		}
	}
	fmt.Fprintln(w)
}

func branchFunctionCell(branch model.BranchTrace) string {
	call := callReference(model.CallRef{Symbol: branch.Function, File: branch.File, Line: branch.Line})
	if branch.Kind == "" {
		return call
	}
	return call
}

func branchCondition(branch model.BranchTrace) string {
	condition := branchKindLabel(branch.Kind)
	if branch.Expr == "" {
		return condition
	}
	return condition + " " + inlineCode(branch.Expr)
}

func branchCaseCallsCell(branch model.BranchTrace, branchCase model.BranchCase) string {
	layers, unknown := directBranchCaseCalls(branch, branchCase)
	return layerCallsCell(layers, unknown, layerCalls(layers, unknown))
}

func directBranchCaseCalls(branch model.BranchTrace, branchCase model.BranchCase) ([]model.LayerCalls, []model.CallRef) {
	directSymbols := branchDirectSymbols(branch, branchCase)
	layers := filterLayers(branchCase.Layers, func(call model.CallRef) bool {
		return isBranchDirectCall(branch, directSymbols, call)
	})
	var unknown []model.CallRef
	for _, call := range branchCase.Unknown {
		if isBranchDirectCall(branch, directSymbols, call) {
			unknown = append(unknown, call)
		}
	}
	return layers, dedupeCalls(unknown)
}

func branchDirectSymbols(branch model.BranchTrace, branchCase model.BranchCase) map[string]bool {
	symbols := make(map[string]bool)
	for _, call := range branchCaseCalls(branchCase) {
		if call.Via == branch.Function || call.Via == "" {
			symbols[call.Symbol] = true
		}
	}
	return symbols
}

func isBranchDirectCall(branch model.BranchTrace, directSymbols map[string]bool, call model.CallRef) bool {
	return call.Via == branch.Function || call.Via == "" || directSymbols[call.Via]
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
	return "case " + inlineCodeList(branchCase.Labels)
}

func writeDispatchesTable(w io.Writer, dispatches []model.DispatchTrace) {
	if len(dispatches) == 0 {
		return
	}

	fmt.Fprintln(w, "#### dispatches")
	fmt.Fprintln(w, "| dispatch | case | calls |")
	fmt.Fprintln(w, "| --- | --- | --- |")
	for _, dispatch := range dispatches {
		for _, dispatchCase := range dispatch.Cases {
			fmt.Fprintf(
				w,
				"| %s | %s | %s |\n",
				tableCell(dispatchCell(dispatch)),
				tableCell(dispatchCaseTitle(dispatchCase)),
				tableCell(dispatchCaseCallsCell(dispatchCase)),
			)
		}
	}
	fmt.Fprintln(w)
}

func dispatchCell(dispatch model.DispatchTrace) string {
	parts := []string{
		callReference(dispatch.Call),
		"from " + inlineCode(dispatchLookupDisplay(dispatch)),
	}
	if dispatch.Interface != "" {
		parts = append(parts, "interface: "+inlineCode(dispatch.Interface))
	}
	return strings.Join(parts, "<br>")
}

func dispatchLookupDisplay(dispatch model.DispatchTrace) string {
	if dispatch.Key == "" {
		return dispatch.Table
	}
	return dispatch.Table + "[" + dispatch.Key + "]"
}

func dispatchCaseCallsCell(dispatchCase model.DispatchCase) string {
	return layerCallsCell(dispatchCase.Layers, dispatchCase.Unknown, dispatchCaseCalls(dispatchCase))
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
	return "case " + inlineCodeList(dispatchCase.Labels)
}

func layerCallsCell(layers []model.LayerCalls, unknown []model.CallRef, allCalls []model.CallRef) string {
	var parts []string
	for _, layer := range layers {
		operations := sortedOperationSummaries(layer.Calls, allCalls)
		if len(operations) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", layer.Name, operationNamesCell(operations)))
	}
	unknownCalls := sortCalls(summarizeUnknown(unknown, map[string]bool{}))
	if len(unknownCalls) > 0 {
		parts = append(parts, fmt.Sprintf("other: %s", callNamesCell(unknownCalls)))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "<br>")
}

func filterLayers(layers []model.LayerCalls, keep func(model.CallRef) bool) []model.LayerCalls {
	var out []model.LayerCalls
	for _, layer := range layers {
		var calls []model.CallRef
		for _, call := range layer.Calls {
			if keep(call) {
				calls = append(calls, call)
			}
		}
		if len(calls) == 0 {
			continue
		}
		out = append(out, model.LayerCalls{Name: layer.Name, Calls: dedupeCalls(calls)})
	}
	return out
}

func layerCalls(layers []model.LayerCalls, unknown []model.CallRef) []model.CallRef {
	var calls []model.CallRef
	for _, layer := range layers {
		calls = append(calls, layer.Calls...)
	}
	calls = append(calls, unknown...)
	return calls
}

func operationNamesCell(operations []operationSummary) string {
	var names []string
	for _, operation := range operations {
		names = append(names, inlineCode(operation.Symbol))
	}
	return strings.Join(names, ", ")
}

func callNamesCell(calls []model.CallRef) string {
	var names []string
	for _, call := range calls {
		names = append(names, inlineCode(call.Symbol))
	}
	return strings.Join(names, ", ")
}

func sortedOperationSummaries(calls []model.CallRef, allCalls []model.CallRef) []operationSummary {
	operations := summarizeOperations(calls, allCalls)
	sort.SliceStable(operations, func(i, j int) bool {
		left := operationAnchor(operations[i])
		right := operationAnchor(operations[j])
		if !sameCall(left, right) {
			return callLess(left, right)
		}
		return operations[i].Symbol < operations[j].Symbol
	})
	return operations
}

func operationAnchor(operation operationSummary) model.CallRef {
	calls := sortCalls(operation.CalledFrom)
	if len(calls) > 0 {
		return calls[0]
	}
	if operation.HasImplementation {
		return operation.Implementation
	}
	return model.CallRef{Symbol: operation.Symbol}
}

func sortImplementationCandidates(candidates []model.ImplementationCandidate) []model.ImplementationCandidate {
	out := append([]model.ImplementationCandidate(nil), candidates...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Expanded != out[j].Expanded {
			return out[i].Expanded
		}
		return callLess(out[i].Call, out[j].Call)
	})
	return out
}

func sortCalls(calls []model.CallRef) []model.CallRef {
	out := append([]model.CallRef(nil), calls...)
	sort.SliceStable(out, func(i, j int) bool {
		return callLess(out[i], out[j])
	})
	return out
}

func callLess(left model.CallRef, right model.CallRef) bool {
	if (left.File == "") != (right.File == "") {
		return left.File != ""
	}
	if left.File != right.File {
		return left.File < right.File
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	if left.Symbol != right.Symbol {
		return left.Symbol < right.Symbol
	}
	if left.Method != right.Method {
		return left.Method < right.Method
	}
	if left.Receiver != right.Receiver {
		return left.Receiver < right.Receiver
	}
	return left.Depth < right.Depth
}

func callReference(call model.CallRef) string {
	if call.Symbol == "" {
		return "-"
	}
	reference := inlineCode(call.Symbol)
	if call.File == "" {
		return reference
	}
	return fmt.Sprintf("%s (%s:%d)", reference, call.File, call.Line)
}

func inlineCodeList(values []string) string {
	var out []string
	for _, value := range values {
		out = append(out, inlineCode(value))
	}
	return strings.Join(out, ", ")
}

func inlineCode(value string) string {
	if value == "" {
		return "-"
	}
	return "`" + strings.ReplaceAll(value, "`", "\\`") + "`"
}

func tableCell(value string) string {
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "|", `\|`)
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}
