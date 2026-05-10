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
		writeCallTree(w, flow)
		writeFunctionIndex(w, flow)
		writeDecisionPoints(w, flow)
	}
	return nil
}

func writeExecutionSummary(w io.Writer, flow model.APIFlow) {
	fmt.Fprintln(w, "### execution summary")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- kind: `%s`\n", flow.Kind)
	fmt.Fprintf(w, "- handler: %s\n", callReference(model.CallRef{
		Symbol: flow.Entrypoint.Symbol,
		File:   flow.Entrypoint.File,
		Line:   flow.Entrypoint.Line,
	}))
	fmt.Fprintf(w, "- request: `%s`\n", flow.Request.Type)
	fmt.Fprintf(w, "- response: `%s`\n", flow.Response.Type)

	if counts := layerCallCounts(flow); len(counts) > 0 {
		fmt.Fprintln(w, "- layers:")
		for _, count := range counts {
			fmt.Fprintf(w, "  - %s: %d %s\n", count.Name, count.Count, plural(count.Count, "call"))
		}
	}

	interfaceCalls, branches, dispatches := decisionPointCounts(flow)
	fmt.Fprintln(w, "- decision points:")
	fmt.Fprintf(w, "  - interface calls: %d\n", interfaceCalls)
	fmt.Fprintf(w, "  - branches: %d\n", branches)
	fmt.Fprintf(w, "  - dispatches: %d\n\n", dispatches)
}

type layerCallCount struct {
	Name  string
	Count int
}

func layerCallCounts(flow model.APIFlow) []layerCallCount {
	var counts []layerCallCount
	for _, layer := range collectLayerCalls(flow) {
		count := len(visibleLayerCalls(layer.Calls))
		if count == 0 {
			continue
		}
		counts = append(counts, layerCallCount{Name: layer.Name, Count: count})
	}
	return counts
}

func decisionPointCounts(flow model.APIFlow) (interfaceCalls int, branches int, dispatches int) {
	interfaceCalls = len(summarizeInterfaceCalls(flow.Trail.InterfaceCalls))
	branches = len(flow.Trail.Branches)
	dispatches = len(flow.Trail.Dispatches)
	return interfaceCalls, branches, dispatches
}

func writeDecisionPoints(w io.Writer, flow model.APIFlow) {
	hasInterfaceCalls := len(summarizeInterfaceCalls(flow.Trail.InterfaceCalls)) > 0
	hasBranches := len(flow.Trail.Branches) > 0
	hasDispatches := len(flow.Trail.Dispatches) > 0
	if !hasInterfaceCalls && !hasBranches && !hasDispatches {
		return
	}

	fmt.Fprintln(w, "### decision points")
	fmt.Fprintln(w)
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
	fmt.Fprintln(w)
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
	fmt.Fprintln(w)
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
	return callReference(model.CallRef{Symbol: branch.Function, File: branch.File, Line: branch.Line})
}

func branchCondition(branch model.BranchTrace) string {
	condition := branchKindLabel(branch.Kind)
	if branch.Expr == "" {
		return condition
	}
	return condition + " " + inlineCode(branch.Expr)
}

func branchCaseCallsCell(branch model.BranchTrace, branchCase model.BranchCase) string {
	layers, unknown := directBranchCaseLayerCalls(branch, branchCase)
	return layerCallsCell(layers, unknown)
}

func directBranchCaseLayerCalls(branch model.BranchTrace, branchCase model.BranchCase) ([]model.LayerCalls, []model.CallRef) {
	directSymbols := branchCaseDirectSymbols(branch, branchCase)
	layers := filterLayers(branchCase.Layers, func(call model.CallRef) bool {
		return isDirectBranchCaseCall(branch, directSymbols, call)
	})
	var unknown []model.CallRef
	for _, call := range branchCase.Unknown {
		if isDirectBranchCaseCall(branch, directSymbols, call) {
			unknown = append(unknown, call)
		}
	}
	return layers, dedupeCalls(unknown)
}

func branchCaseDirectSymbols(branch model.BranchTrace, branchCase model.BranchCase) map[string]bool {
	symbols := make(map[string]bool)
	for _, call := range allBranchCaseCalls(branchCase) {
		if call.Via == branch.Function || call.Via == "" {
			symbols[call.Symbol] = true
		}
	}
	return symbols
}

func isDirectBranchCaseCall(branch model.BranchTrace, directSymbols map[string]bool, call model.CallRef) bool {
	return call.Via == branch.Function || call.Via == "" || directSymbols[call.Via]
}

func allBranchCaseCalls(branchCase model.BranchCase) []model.CallRef {
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
	fmt.Fprintln(w)
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
	return layerCallsCell(dispatchCase.Layers, dispatchCase.Unknown)
}

func dispatchCaseTitle(dispatchCase model.DispatchCase) string {
	if len(dispatchCase.Labels) == 0 {
		return "case"
	}
	return "case " + inlineCodeList(dispatchCase.Labels)
}

func layerCallsCell(layers []model.LayerCalls, unknown []model.CallRef) string {
	var parts []string
	for _, layer := range layers {
		calls := visibleLayerCalls(layer.Calls)
		if len(calls) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", layer.Name, callNamesCell(calls)))
	}
	unknownCalls := visibleUnknownCalls(unknown)
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

func callNamesCell(calls []model.CallRef) string {
	var names []string
	for _, call := range calls {
		names = append(names, inlineCode(call.Symbol))
	}
	return strings.Join(names, ", ")
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

func plural(count int, singular string) string {
	if count == 1 {
		return singular
	}
	return singular + "s"
}
