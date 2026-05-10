package output

import (
	"fmt"
	"unicode"

	"github.com/usuginus/calltrail-go/internal/model"
)

func collectLayerCalls(flow model.APIFlow) []model.LayerCalls {
	var layers []model.LayerCalls
	walkLayerCalls(flow, func(layer model.LayerCalls) {
		layers = appendLayerCalls(layers, layer)
	})
	return layers
}

func walkLayerCalls(flow model.APIFlow, visit func(model.LayerCalls)) {
	for _, layer := range flow.Trail.Layers {
		visit(layer)
	}
	for _, branch := range flow.Trail.Branches {
		for _, branchCase := range branch.Cases {
			for _, layer := range branchCase.Layers {
				visit(layer)
			}
		}
	}
	for _, dispatch := range flow.Trail.Dispatches {
		for _, dispatchCase := range dispatch.Cases {
			for _, layer := range dispatchCase.Layers {
				visit(layer)
			}
		}
	}
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

func visibleLayerCalls(calls []model.CallRef) []model.CallRef {
	calls = dedupeCalls(calls)
	hidden := implementationCallsiteKeys(calls)

	var out []model.CallRef
	for _, call := range calls {
		if hidden[callKey(call)] || isInternalHelperCall(call) {
			continue
		}
		out = append(out, call)
	}
	return sortCalls(out)
}

func implementationCallsiteKeys(calls []model.CallRef) map[string]bool {
	hidden := make(map[string]bool)
	for _, parent := range calls {
		if parent.Method == "" {
			continue
		}
		for _, child := range calls {
			if child.Via == parent.Symbol && child.Symbol != parent.Symbol && child.Method == parent.Method {
				hidden[callKey(parent)] = true
				break
			}
		}
	}
	return hidden
}

func visibleUnknownCalls(calls []model.CallRef) []model.CallRef {
	var out []model.CallRef
	for _, call := range calls {
		if call.Depth > 2 || isInternalHelperCall(call) {
			continue
		}
		out = append(out, call)
	}
	return sortCalls(dedupeCalls(out))
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
