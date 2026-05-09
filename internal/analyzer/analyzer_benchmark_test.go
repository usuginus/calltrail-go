package analyzer

import (
	"testing"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

var benchmarkFlows []model.APIFlow

func BenchmarkAnalyzeGRPCBasicDepth3(b *testing.B) {
	ruleSet := mustLoadRules(b, "")
	opts := Options{Depth: 3, Rules: ruleSet}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flows, err := Analyze([]string{"../../examples/grpc-basic"}, opts)
		if err != nil {
			b.Fatalf("Analyze returned error: %v", err)
		}
		benchmarkFlows = flows
	}
}

func BenchmarkAnalyzeBranchDispatchDepth3(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/branch-dispatch/.calltrail.yaml")
	opts := Options{Depth: 3, Rules: ruleSet}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flows, err := Analyze([]string{"../../examples/branch-dispatch"}, opts)
		if err != nil {
			b.Fatalf("Analyze returned error: %v", err)
		}
		benchmarkFlows = flows
	}
}

func BenchmarkAnalyzeMapDispatchDepth4(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/map-dispatch/.calltrail.yaml")
	opts := Options{Depth: 4, Rules: ruleSet}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flows, err := Analyze([]string{"../../examples/map-dispatch"}, opts)
		if err != nil {
			b.Fatalf("Analyze returned error: %v", err)
		}
		benchmarkFlows = flows
	}
}

func BenchmarkAnalyzeMapDispatchRPCFilter(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/map-dispatch/.calltrail.yaml")
	opts := Options{RPC: "ProcessDocument", Depth: 4, Rules: ruleSet}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flows, err := Analyze([]string{"../../examples/map-dispatch"}, opts)
		if err != nil {
			b.Fatalf("Analyze returned error: %v", err)
		}
		benchmarkFlows = flows
	}
}

func BenchmarkDetectHandlersMapDispatch(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/map-dispatch/.calltrail.yaml")
	opts := Options{Rules: ruleSet}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flows, err := DetectHandlers([]string{"../../examples/map-dispatch"}, opts)
		if err != nil {
			b.Fatalf("DetectHandlers returned error: %v", err)
		}
		benchmarkFlows = flows
	}
}

func mustLoadRules(b *testing.B, path string) rules.RuleSet {
	b.Helper()
	ruleSet, err := rules.Load(path)
	if err != nil {
		b.Fatalf("Load returned error: %v", err)
	}
	return ruleSet
}
