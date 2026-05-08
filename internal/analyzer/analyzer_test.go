package analyzer

import (
	"testing"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

func TestAnalyzeDetectsGRPCHandlerTrail(t *testing.T) {
	flows, err := Analyze([]string{"testdata/simple"}, Options{})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if flow.Name != "GetFoo" {
		t.Fatalf("flow.Name = %q, want GetFoo", flow.Name)
	}
	if flow.Entrypoint.File != "internal/analyzer/testdata/simple/handler.go" {
		t.Fatalf("entrypoint file = %q", flow.Entrypoint.File)
	}
	if flow.Request.Type != "*pb.GetFooRequest" {
		t.Fatalf("request type = %q", flow.Request.Type)
	}
	usecases := flow.Trail.LayerCalls("usecase")
	if len(usecases) != 1 {
		t.Fatalf("usecases = %d, want 1", len(usecases))
	}
	if usecases[0].Symbol != "s.fooUsecase.GetFoo" {
		t.Fatalf("usecase symbol = %q", usecases[0].Symbol)
	}
	if len(flow.Errors.GRPCCodes) != 1 || flow.Errors.GRPCCodes[0] != "Internal" {
		t.Fatalf("grpc codes = %#v, want [Internal]", flow.Errors.GRPCCodes)
	}
}

func TestAnalyzeDepthTwoFollowsInterfaceImplementationCandidate(t *testing.T) {
	flows, err := Analyze([]string{"testdata/simple"}, Options{Depth: 2})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	usecases := flow.Trail.LayerCalls("usecase")
	if len(usecases) != 2 {
		t.Fatalf("usecases = %d, want 2", len(usecases))
	}
	if usecases[1].Symbol != "fooUsecase.GetFoo" {
		t.Fatalf("depth-2 usecase symbol = %q", usecases[1].Symbol)
	}
	if usecases[1].Depth != 2 {
		t.Fatalf("depth-2 usecase depth = %d", usecases[1].Depth)
	}
	repositories := flow.Trail.LayerCalls("repository")
	if len(repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(repositories))
	}
	if repositories[0].Symbol != "f.repos.Foo.FindFoo" {
		t.Fatalf("repository symbol = %q", repositories[0].Symbol)
	}
	if repositories[0].Via != "fooUsecase.GetFoo" {
		t.Fatalf("repository via = %q", repositories[0].Via)
	}
	if hasCall(flow.Trail.Unknown, "stdstrings.TrimSpace") {
		t.Fatal("standard library alias call was not ignored")
	}
}

func TestAnalyzeDepthThreeFollowsNestedStructFieldCandidate(t *testing.T) {
	flows, err := Analyze([]string{"testdata/simple"}, Options{Depth: 3})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	repositories := flow.Trail.LayerCalls("repository")
	if len(repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(repositories))
	}
	if repositories[1].Symbol != "fooRepository.FindFoo" {
		t.Fatalf("depth-3 repository symbol = %q", repositories[1].Symbol)
	}
	if repositories[1].Depth != 3 {
		t.Fatalf("depth-3 repository depth = %d", repositories[1].Depth)
	}
	if repositories[1].Via != "f.repos.Foo.FindFoo" {
		t.Fatalf("depth-3 repository via = %q", repositories[1].Via)
	}
}

func TestAnalyzeFollowsConstructorChainedAndLocalVariableCalls(t *testing.T) {
	flows, err := Analyze([]string{"testdata/chained"}, Options{Depth: 4})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	usecases := flow.Trail.LayerCalls("usecase")
	if !hasCall(usecases, "fooUsecase.GetFoo") {
		t.Fatalf("usecases = %#v, want fooUsecase.GetFoo", usecases)
	}
	services := flow.Trail.LayerCalls("service")
	if !hasCall(services, "fooService.FetchFoo") {
		t.Fatalf("services = %#v, want fooService.FetchFoo", services)
	}
	if !hasCall(usecases, "NewFooUsecase().GetFoo") {
		t.Fatalf("usecases = %#v, want NewFooUsecase().GetFoo", usecases)
	}
}

func TestAnalyzeUsesConfiguredLayerNameInTrail(t *testing.T) {
	ruleSet, err := rules.Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	for i := range ruleSet.Layers {
		if ruleSet.Layers[i].Name == "usecase" {
			ruleSet.Layers[i].Name = "application"
		}
	}

	flows, err := Analyze([]string{"testdata/simple"}, Options{Rules: ruleSet})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if len(flow.Trail.LayerCalls("usecase")) != 0 {
		t.Fatalf("usecase layer = %#v, want empty", flow.Trail.LayerCalls("usecase"))
	}
	if got := flow.Trail.LayerCalls("application"); len(got) != 1 || got[0].Symbol != "s.fooUsecase.GetFoo" {
		t.Fatalf("application layer = %#v, want s.fooUsecase.GetFoo", got)
	}
}

func TestClassifyUsesReceiverTypeBeforeCurrentFilePath(t *testing.T) {
	ruleSet, err := rules.Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	ref := model.CallRef{
		Symbol:   "c.repos.Foo.FindFoo",
		Receiver: "c.repos.Foo",
		Method:   "FindFoo",
		File:     "internal/usecase/foo.go",
	}
	scope := scopeInfo{
		receiverVar:    "c",
		receiverFields: map[string]string{"repos": "Repositories"},
		structFields: map[string]map[string]string{
			"Repositories": {"Foo": "FooRepository"},
		},
	}

	if got := classify(ref, scope, ruleSet.Layers); got != "repository" {
		t.Fatalf("classify = %q, want repository", got)
	}
}

func TestClassifyDoesNotUseCurrentFilePathForUtilityCalls(t *testing.T) {
	ruleSet, err := rules.Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	ref := model.CallRef{
		Symbol:   "mspanner.ReadOnlyTransaction",
		Receiver: "mspanner",
		Method:   "ReadOnlyTransaction",
		File:     "internal/usecase/foo.go",
	}

	if got := classify(ref, scopeInfo{receiverVar: "c"}, ruleSet.Layers); got != "unknown" {
		t.Fatalf("classify = %q, want unknown", got)
	}
}

func hasCall(calls []model.CallRef, symbol string) bool {
	for _, call := range calls {
		if call.Symbol == symbol {
			return true
		}
	}
	return false
}
