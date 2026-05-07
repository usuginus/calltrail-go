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
	if len(flow.Trail.Usecases) != 1 {
		t.Fatalf("usecases = %d, want 1", len(flow.Trail.Usecases))
	}
	if flow.Trail.Usecases[0].Symbol != "s.fooUsecase.GetFoo" {
		t.Fatalf("usecase symbol = %q", flow.Trail.Usecases[0].Symbol)
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
	if len(flow.Trail.Usecases) != 2 {
		t.Fatalf("usecases = %d, want 2", len(flow.Trail.Usecases))
	}
	if flow.Trail.Usecases[1].Symbol != "fooUsecase.GetFoo" {
		t.Fatalf("depth-2 usecase symbol = %q", flow.Trail.Usecases[1].Symbol)
	}
	if flow.Trail.Usecases[1].Depth != 2 {
		t.Fatalf("depth-2 usecase depth = %d", flow.Trail.Usecases[1].Depth)
	}
	if len(flow.Trail.Repositories) != 1 {
		t.Fatalf("repositories = %d, want 1", len(flow.Trail.Repositories))
	}
	if flow.Trail.Repositories[0].Symbol != "f.repos.Foo.FindFoo" {
		t.Fatalf("repository symbol = %q", flow.Trail.Repositories[0].Symbol)
	}
	if flow.Trail.Repositories[0].Via != "fooUsecase.GetFoo" {
		t.Fatalf("repository via = %q", flow.Trail.Repositories[0].Via)
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
	if len(flow.Trail.Repositories) != 2 {
		t.Fatalf("repositories = %d, want 2", len(flow.Trail.Repositories))
	}
	if flow.Trail.Repositories[1].Symbol != "fooRepository.FindFoo" {
		t.Fatalf("depth-3 repository symbol = %q", flow.Trail.Repositories[1].Symbol)
	}
	if flow.Trail.Repositories[1].Depth != 3 {
		t.Fatalf("depth-3 repository depth = %d", flow.Trail.Repositories[1].Depth)
	}
	if flow.Trail.Repositories[1].Via != "f.repos.Foo.FindFoo" {
		t.Fatalf("depth-3 repository via = %q", flow.Trail.Repositories[1].Via)
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
	if !hasCall(flow.Trail.Usecases, "fooUsecase.GetFoo") {
		t.Fatalf("usecases = %#v, want fooUsecase.GetFoo", flow.Trail.Usecases)
	}
	if !hasCall(flow.Trail.Services, "fooService.FetchFoo") {
		t.Fatalf("services = %#v, want fooService.FetchFoo", flow.Trail.Services)
	}
	if !hasCall(flow.Trail.Usecases, "NewFooUsecase().GetFoo") {
		t.Fatalf("usecases = %#v, want NewFooUsecase().GetFoo", flow.Trail.Usecases)
	}
}

func TestClassifyUsesReceiverTypeBeforeCurrentFilePath(t *testing.T) {
	ruleSet, err := rules.Load("generic", "")
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

	if got := classify(ref, scope, ruleSet.Classifiers); got != "repository" {
		t.Fatalf("classify = %q, want repository", got)
	}
}

func TestClassifyDoesNotUseCurrentFilePathForUtilityCalls(t *testing.T) {
	ruleSet, err := rules.Load("generic", "")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	ref := model.CallRef{
		Symbol:   "mspanner.ReadOnlyTransaction",
		Receiver: "mspanner",
		Method:   "ReadOnlyTransaction",
		File:     "internal/usecase/foo.go",
	}

	if got := classify(ref, scopeInfo{receiverVar: "c"}, ruleSet.Classifiers); got != "unknown" {
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
