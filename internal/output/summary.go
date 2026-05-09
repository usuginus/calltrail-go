package output

import (
	"fmt"
	"unicode"

	"github.com/usuginus/calltrail-go/internal/model"
)

type operationSummary struct {
	Symbol            string
	Implementation    model.CallRef
	HasImplementation bool
	CalledFrom        []model.CallRef
	Related           []model.CallRef
}

func collectCalls(flow model.APIFlow) []model.CallRef {
	var calls []model.CallRef
	for _, layer := range collectLayerCalls(flow) {
		calls = append(calls, layer.Calls...)
	}
	calls = append(calls, flow.Trail.Async...)
	calls = append(calls, flow.Trail.Unknown...)
	for _, branch := range flow.Trail.Branches {
		for _, branchCase := range branch.Cases {
			calls = append(calls, branchCase.Unknown...)
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		for _, dispatchCase := range dispatch.Cases {
			calls = append(calls, dispatchCase.Unknown...)
		}
	}
	return calls
}

func collectLayerCalls(flow model.APIFlow) []model.LayerCalls {
	var layers []model.LayerCalls
	for _, layer := range flow.Trail.Layers {
		layers = appendLayerCalls(layers, layer)
	}
	for _, branch := range flow.Trail.Branches {
		for _, branchCase := range branch.Cases {
			for _, layer := range branchCase.Layers {
				layers = appendLayerCalls(layers, layer)
			}
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		for _, dispatchCase := range dispatch.Cases {
			for _, layer := range dispatchCase.Layers {
				layers = appendLayerCalls(layers, layer)
			}
		}
	}
	return layers
}

func appendLayerCalls(layers []model.LayerCalls, next model.LayerCalls) []model.LayerCalls {
	if next.Name == "" || len(next.Calls) == 0 {
		return layers
	}
	for i := range layers {
		if layers[i].Name == next.Name {
			layers[i].Calls = appendUniqueCalls(layers[i].Calls, next.Calls...)
			return layers
		}
	}
	next.Calls = dedupeCalls(next.Calls)
	return append(layers, next)
}

func summarizeOperations(calls []model.CallRef, allCalls []model.CallRef) []operationSummary {
	childrenByVia := make(map[string][]model.CallRef)
	callSymbols := make(map[string]bool)
	for _, call := range calls {
		callSymbols[call.Symbol] = true
		if call.Via != "" {
			childrenByVia[call.Via] = append(childrenByVia[call.Via], call)
		}
	}
	firstCallBySymbol := firstCallsBySymbol(allCalls)

	var operations []operationSummary
	operationBySymbol := make(map[string]int)
	for _, call := range calls {
		if call.Via != "" && callSymbols[call.Via] {
			continue
		}
		operation, ok := buildOperationSummary(call, childrenByVia[call.Symbol], firstCallBySymbol)
		if !ok {
			continue
		}
		operations = appendOperation(operations, operationBySymbol, operation)
	}
	return operations
}

func firstCallsBySymbol(calls []model.CallRef) map[string]model.CallRef {
	out := make(map[string]model.CallRef)
	for _, call := range calls {
		if _, ok := out[call.Symbol]; !ok {
			out[call.Symbol] = call
		}
	}
	return out
}

func buildOperationSummary(
	call model.CallRef,
	children []model.CallRef,
	firstCallBySymbol map[string]model.CallRef,
) (operationSummary, bool) {
	implementation, hasImplementation := sameOperationChild(call, children)
	switch {
	case hasImplementation:
		operation := operationSummary{
			Symbol:            implementation.Symbol,
			Implementation:    implementation,
			HasImplementation: true,
			CalledFrom:        []model.CallRef{call},
			Related:           relatedInternalCalls(children, implementation),
		}
		return operation, true
	case call.Via != "":
		operation := operationSummary{
			Symbol:     call.Symbol,
			CalledFrom: viaCallsite(call.Via, firstCallBySymbol),
		}
		return operation, true
	default:
		operation := operationSummary{Symbol: call.Symbol, CalledFrom: []model.CallRef{call}}
		return operation, true
	}
}

func viaCallsite(via string, firstCallBySymbol map[string]model.CallRef) []model.CallRef {
	if call, ok := firstCallBySymbol[via]; ok {
		return []model.CallRef{call}
	}
	return []model.CallRef{{Symbol: via}}
}

func sameOperationChild(parent model.CallRef, children []model.CallRef) (model.CallRef, bool) {
	var matches []model.CallRef
	for _, child := range children {
		if child.Symbol != parent.Symbol && child.Method == parent.Method {
			matches = append(matches, child)
		}
	}
	matches = dedupeCalls(matches)
	if len(matches) != 1 {
		return model.CallRef{}, false
	}
	return matches[0], true
}

func appendOperation(operations []operationSummary, index map[string]int, next operationSummary) []operationSummary {
	if existingIndex, ok := index[next.Symbol]; ok {
		existing := &operations[existingIndex]
		if !existing.HasImplementation && next.HasImplementation {
			existing.Implementation = next.Implementation
			existing.HasImplementation = true
		}
		existing.CalledFrom = appendUniqueCalls(existing.CalledFrom, next.CalledFrom...)
		existing.Related = appendUniqueCalls(existing.Related, next.Related...)
		return operations
	}
	index[next.Symbol] = len(operations)
	next.CalledFrom = dedupeCalls(next.CalledFrom)
	next.Related = dedupeCalls(next.Related)
	return append(operations, next)
}

func relatedInternalCalls(children []model.CallRef, implementation model.CallRef) []model.CallRef {
	var related []model.CallRef
	for _, child := range children {
		if sameCall(child, implementation) || isInternalHelperCall(child) {
			continue
		}
		related = append(related, child)
	}
	return dedupeCalls(related)
}

func operationCallsiteSymbols(flow model.APIFlow) map[string]bool {
	symbols := make(map[string]bool)
	for _, layer := range collectLayerCalls(flow) {
		for _, call := range layer.Calls {
			if call.Via != "" {
				symbols[call.Via] = true
			}
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		symbols[dispatch.Call.Symbol] = true
	}
	return symbols
}

func summarizeUnknown(calls []model.CallRef, operationCallsites map[string]bool) []model.CallRef {
	var out []model.CallRef
	for _, call := range calls {
		if operationCallsites[call.Symbol] || call.Depth > 2 || isInternalHelperCall(call) {
			continue
		}
		out = append(out, call)
	}
	return dedupeCalls(out)
}

func isInternalHelperCall(call model.CallRef) bool {
	if call.Method == "" {
		return false
	}
	return startsLower(call.Method)
}

func startsLower(value string) bool {
	for _, r := range value {
		return unicode.IsLower(r)
	}
	return false
}

func appendUniqueCalls(calls []model.CallRef, more ...model.CallRef) []model.CallRef {
	seen := make(map[string]bool, len(calls)+len(more))
	for _, call := range calls {
		seen[callKey(call)] = true
	}
	for _, call := range more {
		key := callKey(call)
		if seen[key] {
			continue
		}
		seen[key] = true
		calls = append(calls, call)
	}
	return calls
}

func dedupeCalls(calls []model.CallRef) []model.CallRef {
	return appendUniqueCalls(nil, calls...)
}

func sameCall(a model.CallRef, b model.CallRef) bool {
	return callKey(a) == callKey(b)
}

func callKey(call model.CallRef) string {
	return fmt.Sprintf("%s\x00%s\x00%d", call.Symbol, call.File, call.Line)
}
