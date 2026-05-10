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
	if len(flow.Trail.InterfaceCalls) != 1 || len(flow.Trail.InterfaceCalls[0].Implementations) != 1 {
		t.Fatalf("interface calls = %#v, want one implementation", flow.Trail.InterfaceCalls)
	}
	if !flow.Trail.InterfaceCalls[0].Implementations[0].Expanded {
		t.Fatal("interface implementation expanded = false, want true")
	}
	if hasCall(flow.Trail.Unknown, "stdstrings.TrimSpace") {
		t.Fatal("standard library alias call was not ignored")
	}
}

func TestAnalyzeRecordsInterfaceCallCandidates(t *testing.T) {
	flows, err := Analyze([]string{"testdata/simple"}, Options{Depth: 1})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if len(flow.Trail.InterfaceCalls) != 1 {
		t.Fatalf("interface calls = %#v, want 1", flow.Trail.InterfaceCalls)
	}
	trace := flow.Trail.InterfaceCalls[0]
	if trace.Call.Symbol != "s.fooUsecase.GetFoo" {
		t.Fatalf("interface call symbol = %q, want s.fooUsecase.GetFoo", trace.Call.Symbol)
	}
	if trace.Interface != "FooUsecase" {
		t.Fatalf("interface = %q, want FooUsecase", trace.Interface)
	}
	if len(trace.Implementations) != 1 {
		t.Fatalf("implementations = %#v, want 1", trace.Implementations)
	}
	implementation := trace.Implementations[0]
	if implementation.Call.Symbol != "fooUsecase.GetFoo" {
		t.Fatalf("implementation = %q, want fooUsecase.GetFoo", implementation.Call.Symbol)
	}
	if implementation.Expanded {
		t.Fatal("implementation expanded at depth 1, want false")
	}
}

func TestAnalyzeMissingRPCReturnsNoFlows(t *testing.T) {
	flows, err := Analyze([]string{"testdata/simple"}, Options{RPC: "MissingRPC"})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 0 {
		t.Fatalf("len(flows) = %d, want 0", len(flows))
	}
}

func TestAnalyzeShortRPCFilterCanMatchMultipleHandlers(t *testing.T) {
	flows, err := Analyze([]string{"testdata/duplicate"}, Options{RPC: "CreateDocument"})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 2 {
		t.Fatalf("len(flows) = %d, want 2", len(flows))
	}
}

func TestAnalyzeAllowsReceiverQualifiedRPCFilter(t *testing.T) {
	flows, err := Analyze([]string{"testdata/duplicate"}, Options{RPC: "userService.CreateDocument"})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}
	if flows[0].Entrypoint.Symbol != "userService.CreateDocument" {
		t.Fatalf("entrypoint symbol = %q, want userService.CreateDocument", flows[0].Entrypoint.Symbol)
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

func TestAnalyzeUsesPackageQualifiedStructFields(t *testing.T) {
	flows, err := Analyze([]string{"testdata/package_collision"}, Options{
		RPC:   "Translate",
		Depth: 3,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	repoCall := findInterfaceCall(flow.Trail.InterfaceCalls, "uc.repo.Store")
	if repoCall == nil {
		t.Fatalf("interface calls = %#v, want uc.repo.Store", flow.Trail.InterfaceCalls)
	}
	if repoCall.Interface != "TranslationRepo" {
		t.Fatalf("repository interface = %q, want TranslationRepo", repoCall.Interface)
	}
	if len(repoCall.Implementations) != 1 {
		t.Fatalf("repository implementations = %#v, want one candidate", repoCall.Implementations)
	}
	if got := repoCall.Implementations[0].Call.Symbol; got != "TranslationRepo.Store" {
		t.Fatalf("repository implementation = %q, want TranslationRepo.Store", got)
	}
	if hasCallImplementation(repoCall.Implementations, "UserRepo.Store") {
		t.Fatalf("repository implementations = %#v, must not include UserRepo.Store", repoCall.Implementations)
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

func TestDetectHandlersReturnsHandlerHeadersOnly(t *testing.T) {
	flows, err := DetectHandlers([]string{"testdata/simple"}, Options{})
	if err != nil {
		t.Fatalf("DetectHandlers returned error: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("len(flows) = %d, want 1", len(flows))
	}

	flow := flows[0]
	if flow.Name != "GetFoo" {
		t.Fatalf("flow.Name = %q, want GetFoo", flow.Name)
	}
	if flow.Entrypoint.Symbol != "Server.GetFoo" {
		t.Fatalf("entrypoint symbol = %q, want Server.GetFoo", flow.Entrypoint.Symbol)
	}
	if flow.Request.Type != "*pb.GetFooRequest" {
		t.Fatalf("request type = %q, want *pb.GetFooRequest", flow.Request.Type)
	}
	if len(flow.Trail.Layers) != 0 || len(flow.Trail.InterfaceCalls) != 0 || len(flow.Trail.Branches) != 0 {
		t.Fatalf("flow trail = %#v, want empty", flow.Trail)
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

func TestAnalyzeMapDispatchExample(t *testing.T) {
	ruleSet, err := rules.Load("../../examples/map-dispatch/.calltrail.yaml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	flows, err := Analyze([]string{"../../examples/map-dispatch"}, Options{
		Depth: 4,
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
	dispatch := findDispatch(flow.Trail.Dispatches, "processor.Process", "a.processors", "cmd.Kind")
	if dispatch == nil {
		t.Fatalf("dispatches = %#v, want processor.Process from a.processors[cmd.Kind]", flow.Trail.Dispatches)
	}
	if dispatch.Interface != "DocumentProcessor" {
		t.Fatalf("dispatch interface = %q, want DocumentProcessor", dispatch.Interface)
	}

	markdownCase := findDispatchCase(dispatch.Cases, "KindMarkdown")
	if markdownCase == nil {
		t.Fatalf("KindMarkdown case not found: %#v", dispatch.Cases)
	}
	for layer, symbol := range map[string]string{
		"application": "markdownProcessor.Process",
		"domain":      "documentPolicy.ValidateMarkdown",
		"persistence": "documentStore.SaveMarkdown",
	} {
		if !hasCall(markdownCase.LayerCalls(layer), symbol) {
			t.Fatalf("KindMarkdown %s layer = %#v, want %s", layer, markdownCase.LayerCalls(layer), symbol)
		}
	}

	imageCase := findDispatchCase(dispatch.Cases, "KindImage")
	if imageCase == nil {
		t.Fatalf("KindImage case not found: %#v", dispatch.Cases)
	}
	for layer, symbol := range map[string]string{
		"application":     "imageProcessor.Process",
		"domain":          "documentPolicy.ValidateImage",
		"external_client": "previewClient.RenderImage",
	} {
		if !hasCall(imageCase.LayerCalls(layer), symbol) {
			t.Fatalf("KindImage %s layer = %#v, want %s", layer, imageCase.LayerCalls(layer), symbol)
		}
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
		Symbol:   "tx.ReadOnlyTransaction",
		Receiver: "tx",
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

func hasCallImplementation(candidates []model.ImplementationCandidate, symbol string) bool {
	for _, candidate := range candidates {
		if candidate.Call.Symbol == symbol {
			return true
		}
	}
	return false
}

func findInterfaceCall(calls []model.InterfaceCallTrace, symbol string) *model.InterfaceCallTrace {
	for i := range calls {
		if calls[i].Call.Symbol == symbol {
			return &calls[i]
		}
	}
	return nil
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

func findDispatch(dispatches []model.DispatchTrace, symbol string, table string, key string) *model.DispatchTrace {
	for i := range dispatches {
		if dispatches[i].Call.Symbol == symbol && dispatches[i].Table == table && dispatches[i].Key == key {
			return &dispatches[i]
		}
	}
	return nil
}

func findDispatchCase(cases []model.DispatchCase, label string) *model.DispatchCase {
	for i := range cases {
		for _, got := range cases[i].Labels {
			if got == label {
				return &cases[i]
			}
		}
	}
	return nil
}
