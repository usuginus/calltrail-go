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

func TestAnalyzeGRPCBasicExample(t *testing.T) {
	flows, err := Analyze([]string{"../../examples/grpc-basic"}, Options{Depth: 3})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if flow.Name != "GetBook" {
		t.Fatalf("flow.Name = %q, want GetBook", flow.Name)
	}
	if !hasCall(flow.Trail.LayerCalls("usecase"), "catalogUsecase.GetBook") {
		t.Fatalf("usecase layer = %#v, want catalogUsecase.GetBook", flow.Trail.LayerCalls("usecase"))
	}
	if !hasCall(flow.Trail.LayerCalls("repository"), "bookRepository.FindBook") {
		t.Fatalf("repository layer = %#v, want bookRepository.FindBook", flow.Trail.LayerCalls("repository"))
	}
	if !hasCall(flow.Trail.LayerCalls("converter"), "bookConverter.ToResponse") {
		t.Fatalf("converter layer = %#v, want bookConverter.ToResponse", flow.Trail.LayerCalls("converter"))
	}
}

func TestAnalyzeCustomLayersExample(t *testing.T) {
	ruleSet, err := rules.Load("../../examples/custom-layers/.calltrail.yaml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	flows, err := Analyze([]string{"../../examples/custom-layers"}, Options{
		Depth: 3,
		Rules: ruleSet,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if flow.Name != "PublishArticle" {
		t.Fatalf("flow.Name = %q, want PublishArticle", flow.Name)
	}
	for layer, symbol := range map[string]string{
		"application":     "articleApplication.PublishArticle",
		"domain":          "articlePolicy.Validate",
		"persistence":     "articleStore.Insert",
		"external_client": "searchIndexClient.Index",
	} {
		if !hasCall(flow.Trail.LayerCalls(layer), symbol) {
			t.Fatalf("%s layer = %#v, want %s", layer, flow.Trail.LayerCalls(layer), symbol)
		}
	}
}

func TestAnalyzeBranchDispatchExample(t *testing.T) {
	ruleSet, err := rules.Load("../../examples/branch-dispatch/.calltrail.yaml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	flows, err := Analyze([]string{"../../examples/branch-dispatch"}, Options{
		Depth: 3,
		Rules: ruleSet,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if flow.Name != "ProcessDocument" {
		t.Fatalf("flow.Name = %q, want ProcessDocument", flow.Name)
	}
	if len(flow.Trail.Branches) != 2 {
		t.Fatalf("branches = %#v, want 2 branches", flow.Trail.Branches)
	}

	typeSwitch := findBranch(flow.Trail.Branches, "type_switch", "asset := cmd.Asset.(type)")
	if typeSwitch == nil {
		t.Fatalf("type switch branch not found: %#v", flow.Trail.Branches)
	}
	if got := len(typeSwitch.Cases); got != 3 {
		t.Fatalf("type switch cases = %d, want 3", got)
	}
	markdownCase := findCase(typeSwitch.Cases, "MarkdownAsset")
	if markdownCase == nil || !hasCall(markdownCase.LayerCalls("domain"), "documentPolicy.ValidateMarkdown") {
		t.Fatalf("MarkdownAsset case = %#v, want documentPolicy.ValidateMarkdown", markdownCase)
	}
	if !hasCall(markdownCase.LayerCalls("domain"), "MarkdownAsset.Normalize") {
		t.Fatalf("MarkdownAsset case = %#v, want MarkdownAsset.Normalize", markdownCase)
	}
	imageCase := findCase(typeSwitch.Cases, "ImageAsset")
	if imageCase == nil || !hasCall(imageCase.LayerCalls("domain"), "ImageAsset.Normalize") {
		t.Fatalf("ImageAsset case = %#v, want ImageAsset.Normalize", imageCase)
	}
	defaultAssetCase := findDefaultCase(typeSwitch.Cases)
	if defaultAssetCase == nil || !hasCall(defaultAssetCase.LayerCalls("domain"), "documentPolicy.RejectUnsupportedAsset") {
		t.Fatalf("type switch default case = %#v, want documentPolicy.RejectUnsupportedAsset", defaultAssetCase)
	}

	valueSwitch := findBranch(flow.Trail.Branches, "switch", "cmd.Mode")
	if valueSwitch == nil {
		t.Fatalf("value switch branch not found: %#v", flow.Trail.Branches)
	}
	publishCase := findCase(valueSwitch.Cases, `"publish"`)
	if publishCase == nil {
		t.Fatalf("publish case not found: %#v", valueSwitch.Cases)
	}
	if !hasCall(publishCase.LayerCalls("persistence"), "documentStore.Publish") {
		t.Fatalf("publish case persistence = %#v, want documentStore.Publish", publishCase.LayerCalls("persistence"))
	}
	if !hasCall(publishCase.LayerCalls("external_client"), "previewClient.Index") {
		t.Fatalf("publish case external_client = %#v, want previewClient.Index", publishCase.LayerCalls("external_client"))
	}
	defaultModeCase := findDefaultCase(valueSwitch.Cases)
	if defaultModeCase == nil || !hasCall(defaultModeCase.LayerCalls("domain"), "documentPolicy.RejectUnsupportedMode") {
		t.Fatalf("mode switch default case = %#v, want documentPolicy.RejectUnsupportedMode", defaultModeCase)
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

func findBranch(branches []model.BranchTrace, kind string, expr string) *model.BranchTrace {
	for i := range branches {
		if branches[i].Kind == kind && branches[i].Expr == expr {
			return &branches[i]
		}
	}
	return nil
}

func findCase(cases []model.BranchCase, label string) *model.BranchCase {
	for i := range cases {
		for _, got := range cases[i].Labels {
			if got == label {
				return &cases[i]
			}
		}
	}
	return nil
}

func findDefaultCase(cases []model.BranchCase) *model.BranchCase {
	for i := range cases {
		if cases[i].Default {
			return &cases[i]
		}
	}
	return nil
}
