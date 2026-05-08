package analyzer

import (
	"fmt"
	"go/token"

	"github.com/usuginus/calltrail-go/internal/model"
)

func recordInterfaceCall(
	fset *token.FileSet,
	flow *model.APIFlow,
	call model.CallRef,
	resolved resolvedCall,
	candidateDepth int,
	maxDepth int,
) {
	if resolved.interfaceType == "" {
		return
	}
	trace := model.InterfaceCallTrace{
		Call:      call,
		Interface: resolved.interfaceType,
	}
	for _, candidate := range resolved.candidates {
		trace.Implementations = append(trace.Implementations, model.ImplementationCandidate{
			Call:     implementationRef(fset, candidate, call.Symbol, candidateDepth),
			Expanded: candidateDepth <= maxDepth,
		})
	}
	appendInterfaceCallTrace(flow, trace)
}

func appendInterfaceCallTrace(flow *model.APIFlow, trace model.InterfaceCallTrace) {
	for i := range flow.Trail.InterfaceCalls {
		if interfaceCallTraceKey(flow.Trail.InterfaceCalls[i]) != interfaceCallTraceKey(trace) {
			continue
		}
		flow.Trail.InterfaceCalls[i].Implementations = appendImplementationCandidates(
			flow.Trail.InterfaceCalls[i].Implementations,
			trace.Implementations...,
		)
		return
	}
	trace.Implementations = appendImplementationCandidates(nil, trace.Implementations...)
	flow.Trail.InterfaceCalls = append(flow.Trail.InterfaceCalls, trace)
}

func appendImplementationCandidates(
	candidates []model.ImplementationCandidate,
	more ...model.ImplementationCandidate,
) []model.ImplementationCandidate {
	seen := make(map[string]int, len(candidates)+len(more))
	for i, candidate := range candidates {
		seen[implementationCandidateKey(candidate)] = i
	}
	for _, candidate := range more {
		key := implementationCandidateKey(candidate)
		if existingIndex, ok := seen[key]; ok {
			candidates[existingIndex].Expanded = candidates[existingIndex].Expanded || candidate.Expanded
			continue
		}
		seen[key] = len(candidates)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func interfaceCallTraceKey(trace model.InterfaceCallTrace) string {
	return fmt.Sprintf("%s\x00%s\x00%d\x00%s", trace.Call.Symbol, trace.Call.File, trace.Call.Line, trace.Interface)
}

func implementationCandidateKey(candidate model.ImplementationCandidate) string {
	call := candidate.Call
	return fmt.Sprintf("%s\x00%s\x00%d", call.Symbol, call.File, call.Line)
}
