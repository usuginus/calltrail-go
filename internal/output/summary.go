package output

import (
	"fmt"
	"strings"
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
	calls = append(calls, flow.Trail.Usecases...)
	calls = append(calls, flow.Trail.Services...)
	calls = append(calls, flow.Trail.Repositories...)
	calls = append(calls, flow.Trail.ExternalClients...)
	calls = append(calls, flow.Trail.Converters...)
	calls = append(calls, flow.Trail.Async...)
	calls = append(calls, flow.Trail.Unknown...)
	return calls
}

func summarizeOperations(calls []model.CallRef, allCalls []model.CallRef, repositoryOnly bool) []operationSummary {
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
		operation, ok := buildOperationSummary(call, childrenByVia[call.Symbol], firstCallBySymbol, repositoryOnly)
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
	repositoryOnly bool,
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
		return operation, keepOperation(operation, repositoryOnly)
	case call.Via != "":
		operation := operationSummary{
			Symbol:            call.Symbol,
			Implementation:    call,
			HasImplementation: true,
			CalledFrom:        viaCallsite(call.Via, firstCallBySymbol),
		}
		return operation, keepOperation(operation, repositoryOnly)
	default:
		operation := operationSummary{Symbol: call.Symbol, CalledFrom: []model.CallRef{call}}
		return operation, keepOperation(operation, repositoryOnly)
	}
}

func keepOperation(operation operationSummary, repositoryOnly bool) bool {
	if !repositoryOnly {
		return true
	}
	return looksLikeRepositoryOperation(model.CallRef{Symbol: operation.Symbol})
}

func viaCallsite(via string, firstCallBySymbol map[string]model.CallRef) []model.CallRef {
	if call, ok := firstCallBySymbol[via]; ok {
		return []model.CallRef{call}
	}
	return []model.CallRef{{Symbol: via}}
}

func sameOperationChild(parent model.CallRef, children []model.CallRef) (model.CallRef, bool) {
	for _, child := range children {
		if child.Symbol != parent.Symbol && child.Method == parent.Method {
			return child, true
		}
	}
	return model.CallRef{}, false
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
	for _, call := range collectCalls(model.APIFlow{Trail: model.Trail{
		Usecases:        flow.Trail.Usecases,
		Services:        flow.Trail.Services,
		Repositories:    flow.Trail.Repositories,
		ExternalClients: flow.Trail.ExternalClients,
	}}) {
		if call.Via != "" {
			symbols[call.Via] = true
		}
	}
	return symbols
}

func summarizeUnknown(calls []model.CallRef, operationCallsites map[string]bool) []model.CallRef {
	var out []model.CallRef
	for _, call := range calls {
		if operationCallsites[call.Symbol] || call.Depth > 2 || isInternalHelperCall(call) || isLowSignalUnknown(call) {
			continue
		}
		out = append(out, call)
	}
	return dedupeCalls(out)
}

func isLowSignalUnknown(call model.CallRef) bool {
	symbol := strings.ToLower(call.Symbol)
	receiver := strings.ToLower(call.Receiver)
	return strings.Contains(symbol, "log") ||
		strings.Contains(symbol, "zap") ||
		receiver == "tok" ||
		call.Method == "String"
}

func looksLikeRepositoryOperation(call model.CallRef) bool {
	return strings.Contains(call.Symbol, "Repository.") ||
		strings.Contains(call.Receiver, ".repos.") ||
		strings.Contains(call.Symbol, ".repos.")
}

func isInternalHelperCall(call model.CallRef) bool {
	if call.Method == "" {
		return false
	}
	method := strings.ToLower(call.Method)
	return startsLower(call.Method) ||
		strings.Contains(method, "column") ||
		strings.Contains(method, "decoder") ||
		strings.Contains(method, "shard") ||
		strings.Contains(method, "spannercommittimestamp")
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

func dedupeTypes(types []model.TypeRef) []model.TypeRef {
	seen := make(map[string]bool, len(types))
	var out []model.TypeRef
	for _, typ := range types {
		if seen[typ.Type] {
			continue
		}
		seen[typ.Type] = true
		out = append(out, typ)
	}
	return out
}
