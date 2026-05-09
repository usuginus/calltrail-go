package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/usuginus/calltrail-go/internal/model"
)

func TestWriteMarkdownUsesConfiguredLayerNames(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "GetFoo",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.GetFoo",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*pb.GetFooRequest"},
			Response: model.TypeRef{Type: "*pb.GetFooResponse"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "domain",
						Calls: []model.CallRef{
							{Symbol: "entity.Foo.Validate", Receiver: "entity.Foo", Method: "Validate", File: "entity.go", Line: 12, Depth: 1},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "### call tree\n\n- [handler] `Server.GetFoo` (handler.go:10)\n  - [domain] `entity.Foo.Validate` (entity.go:12)") {
		t.Fatalf("markdown output does not include configured layer:\n%s", got)
	}
	if !strings.Contains(got, "### function index\n\n#### domain\n\n| function | location | occurrences |") {
		t.Fatalf("markdown output does not include function index:\n%s", got)
	}
}

func TestWriteMarkdownSummarizesRepositoryOperations(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "CreateFoo",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.CreateFoo",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*pb.CreateFooRequest"},
			Response: model.TypeRef{Type: "*pb.Foo"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "usecase",
						Calls: []model.CallRef{
							{Symbol: "s.fooUsecase.CreateFoo", Receiver: "s.fooUsecase", Method: "CreateFoo", File: "handler.go", Line: 12, Depth: 1},
							{Symbol: "fooUsecase.CreateFoo", Receiver: "fooUsecase", Method: "CreateFoo", File: "usecase.go", Line: 20, Depth: 2, Via: "s.fooUsecase.CreateFoo"},
						},
					},
					{
						Name: "repository",
						Calls: []model.CallRef{
							{Symbol: "u.repos.Foo.FindFoo", Receiver: "u.repos.Foo", Method: "FindFoo", File: "usecase.go", Line: 23, Depth: 2, Via: "s.fooUsecase.CreateFoo"},
							{Symbol: "FooRepository.FindFoo", Receiver: "FooRepository", Method: "FindFoo", File: "repository.go", Line: 30, Depth: 3, Via: "u.repos.Foo.FindFoo"},
							{Symbol: "repo.columns", Receiver: "repo", Method: "columns", File: "repository.go", Line: 31, Depth: 3, Via: "u.repos.Foo.FindFoo"},
							{Symbol: "u.repos.Foo.FindFoo", Receiver: "u.repos.Foo", Method: "FindFoo", File: "usecase.go", Line: 40, Depth: 2, Via: "s.fooUsecase.CreateFoo"},
							{Symbol: "FooRepository.FindFoo", Receiver: "FooRepository", Method: "FindFoo", File: "repository.go", Line: 30, Depth: 3, Via: "u.repos.Foo.FindFoo"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"- [usecase] `s.fooUsecase.CreateFoo` (handler.go:12)\n    - [usecase] `fooUsecase.CreateFoo` (usecase.go:20)",
		"- [repository] `u.repos.Foo.FindFoo` (usecase.go:23)\n      - [repository] `FooRepository.FindFoo` (repository.go:30)",
		"| `FooRepository.FindFoo` | `repository.go:30` | 1 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "repo.columns") {
		t.Fatalf("markdown output includes internal repository helper:\n%s", got)
	}
}

func TestWriteMarkdownDisambiguatesSameSymbolHelpersByFile(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "GetFoo",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.GetFoo",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*pb.GetFooRequest"},
			Response: model.TypeRef{Type: "*pb.GetFooResponse"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "repository",
						Calls: []model.CallRef{
							{Symbol: "s.fooRepo.Find", Receiver: "s.fooRepo", Method: "Find", File: "handler.go", Line: 12, Depth: 1},
							{Symbol: "repo.decode", Receiver: "repo", Method: "decode", File: "foo_repository.go", Line: 11, Depth: 2, Via: "s.fooRepo.Find"},
							{Symbol: "FooRepository.Decode", Receiver: "FooRepository", Method: "Decode", File: "foo_repository.go", Line: 20, Depth: 3, Via: "repo.decode"},
							{Symbol: "s.barRepo.Find", Receiver: "s.barRepo", Method: "Find", File: "handler.go", Line: 22, Depth: 1},
							{Symbol: "repo.decode", Receiver: "repo", Method: "decode", File: "bar_repository.go", Line: 11, Depth: 2, Via: "s.barRepo.Find"},
							{Symbol: "BarRepository.Decode", Receiver: "BarRepository", Method: "Decode", File: "bar_repository.go", Line: 20, Depth: 3, Via: "repo.decode"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"`repo.decode` (foo_repository.go:11)\n      - [repository] `FooRepository.Decode` (foo_repository.go:20)",
		"`repo.decode` (bar_repository.go:11)\n      - [repository] `BarRepository.Decode` (bar_repository.go:20)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{
		"`repo.decode` (foo_repository.go:11)\n      - [repository] `BarRepository.Decode`",
		"`repo.decode` (bar_repository.go:11)\n      - [repository] `FooRepository.Decode`",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("markdown output attaches helper child to wrong parent:\n%s", got)
		}
	}
}

func TestWriteMarkdownDoesNotTreatViaOnlyCallAsImplementation(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "CreateFoo",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.CreateFoo",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*pb.CreateFooRequest"},
			Response: model.TypeRef{Type: "*pb.Foo"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "usecase",
						Calls: []model.CallRef{
							{Symbol: "fooUsecase.CreateFoo", Receiver: "fooUsecase", Method: "CreateFoo", File: "usecase.go", Line: 18, Depth: 2, Via: "s.fooUsecase.CreateFoo"},
						},
					},
					{
						Name: "repository",
						Calls: []model.CallRef{
							{Symbol: "repository.IsNotFoundError", Receiver: "repository", Method: "IsNotFoundError", File: "usecase.go", Line: 40, Depth: 2, Via: "fooUsecase.CreateFoo"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"| `fooUsecase.CreateFoo` | `usecase.go:18` | 1 |",
		"| `repository.IsNotFoundError` | `usecase.go:40` | 1 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "implementation: usecase.go:40") {
		t.Fatalf("markdown output treats callsite as implementation:\n%s", got)
	}
}

func TestWriteMarkdownIncludesBranchSummary(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "ProcessDocument",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Service.ProcessDocument",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*ProcessDocumentRequest"},
			Response: model.TypeRef{Type: "*ProcessDocumentResponse"},
			Trail: model.Trail{
				Branches: []model.BranchTrace{
					{
						Kind:     "switch",
						Function: "documentApplication.ProcessDocument",
						Expr:     "cmd.Mode",
						File:     "application.go",
						Line:     24,
						Cases: []model.BranchCase{
							{
								Labels: []string{`"publish"`},
								Layers: []model.LayerCalls{
									{
										Name: "persistence",
										Calls: []model.CallRef{
											{Symbol: "a.store.Publish", Receiver: "a.store", Method: "Publish", File: "application.go", Line: 30, Depth: 2},
											{Symbol: "documentStore.Publish", Receiver: "documentStore", Method: "Publish", File: "persistence.go", Line: 12, Depth: 3, Via: "a.store.Publish"},
										},
									},
									{
										Name: "external_client",
										Calls: []model.CallRef{
											{Symbol: "a.index.Index", Receiver: "a.index", Method: "Index", File: "application.go", Line: 32, Depth: 2, Via: "documentApplication.ProcessDocument"},
											{Symbol: "previewClient.Index", Receiver: "previewClient", Method: "Index", File: "external_client.go", Line: 7, Depth: 3, Via: "a.index.Index"},
										},
									},
								},
							},
							{
								Default: true,
								Layers: []model.LayerCalls{
									{
										Name: "domain",
										Calls: []model.CallRef{
											{Symbol: "a.policy.RejectUnsupportedMode", Receiver: "a.policy", Method: "RejectUnsupportedMode", File: "application.go", Line: 42, Depth: 2, Via: "documentApplication.ProcessDocument"},
											{Symbol: "documentPolicy.RejectUnsupportedMode", Receiver: "documentPolicy", Method: "RejectUnsupportedMode", File: "domain.go", Line: 24, Depth: 3, Via: "a.policy.RejectUnsupportedMode"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"### decision points\n\n#### branches\n\n| function | condition | case | calls |",
		"| `documentApplication.ProcessDocument` (application.go:24) | switch `cmd.Mode` | case `\"publish\"` | persistence: `documentStore.Publish`<br>external_client: `previewClient.Index` |",
		"| `documentApplication.ProcessDocument` (application.go:24) | switch `cmd.Mode` | default | domain: `documentPolicy.RejectUnsupportedMode` |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownBranchSummaryKeepsDirectDecisionCalls(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetReport",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.GetReport", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*GetReportRequest"},
			Response:   model.TypeRef{Type: "*Report"},
			Trail: model.Trail{
				Branches: []model.BranchTrace{
					{
						Kind:     "type_switch",
						Function: "Service.GetReport",
						Expr:     "req := in.GetRequest().(type)",
						File:     "handler.go",
						Line:     11,
						Cases: []model.BranchCase{
							{
								Labels: []string{"*GetReportRequest_V1"},
								Layers: []model.LayerCalls{
									{
										Name: "usecase",
										Calls: []model.CallRef{
											{Symbol: "s.report.Get", Receiver: "s.report", Method: "Get", File: "handler.go", Line: 14, Depth: 1, Via: "Service.GetReport"},
											{Symbol: "Report.Get", Receiver: "Report", Method: "Get", File: "report.go", Line: 20, Depth: 2, Via: "s.report.Get"},
										},
									},
									{
										Name: "repository",
										Calls: []model.CallRef{
											{Symbol: "ReportRepository.Find", Receiver: "ReportRepository", Method: "Find", File: "repository.go", Line: 12, Depth: 3, Via: "Report.Get"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	want := "| `Service.GetReport` (handler.go:11) | type switch `req := in.GetRequest().(type)` | case `*GetReportRequest_V1` | usecase: `Report.Get` |"
	if !strings.Contains(got, want) {
		t.Fatalf("markdown output does not contain direct branch summary %q:\n%s", want, got)
	}
	if !strings.Contains(got, "| `ReportRepository.Find` | `repository.go:12` | 1 |") {
		t.Fatalf("markdown function index does not include transitive branch details:\n%s", got)
	}
	if strings.Contains(got, "| `Service.GetReport` (handler.go:11) | type switch `req := in.GetRequest().(type)` | case `*GetReportRequest_V1` | usecase: `Report.Get`<br>repository: `ReportRepository.Find` |") {
		t.Fatalf("markdown branch summary includes transitive implementation details:\n%s", got)
	}
}

func TestWriteMarkdownIncludesDispatchSummary(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "ProcessDocument",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Service.ProcessDocument",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*ProcessDocumentRequest"},
			Response: model.TypeRef{Type: "*ProcessDocumentResponse"},
			Trail: model.Trail{
				Dispatches: []model.DispatchTrace{
					{
						Table:     "a.processors",
						Key:       "cmd.Kind",
						Call:      model.CallRef{Symbol: "processor.Process", Receiver: "processor", Method: "Process", File: "application.go", Line: 44, Depth: 2},
						Interface: "DocumentProcessor",
						Cases: []model.DispatchCase{
							{
								Labels: []string{"KindMarkdown"},
								Layers: []model.LayerCalls{
									{
										Name: "application",
										Calls: []model.CallRef{
											{Symbol: "markdownProcessor.Process", Receiver: "markdownProcessor", Method: "Process", File: "application.go", Line: 56, Depth: 3, Via: "processor.Process"},
										},
									},
									{
										Name: "persistence",
										Calls: []model.CallRef{
											{Symbol: "documentStore.SaveMarkdown", Receiver: "documentStore", Method: "SaveMarkdown", File: "persistence.go", Line: 7, Depth: 4, Via: "markdownProcessor.Process"},
										},
									},
								},
							},
							{
								Labels: []string{"KindImage"},
								Layers: []model.LayerCalls{
									{
										Name: "application",
										Calls: []model.CallRef{
											{Symbol: "imageProcessor.Process", Receiver: "imageProcessor", Method: "Process", File: "application.go", Line: 75, Depth: 3, Via: "processor.Process"},
										},
									},
									{
										Name: "external_client",
										Calls: []model.CallRef{
											{Symbol: "previewClient.RenderImage", Receiver: "previewClient", Method: "RenderImage", File: "external_client.go", Line: 7, Depth: 4, Via: "imageProcessor.Process"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"### decision points\n\n#### dispatches\n\n| dispatch | case | calls |",
		"| `processor.Process` (application.go:44)<br>from `a.processors[cmd.Kind]`<br>interface: `DocumentProcessor` | case `KindMarkdown` | application: `markdownProcessor.Process`<br>persistence: `documentStore.SaveMarkdown` |",
		"| `processor.Process` (application.go:44)<br>from `a.processors[cmd.Kind]`<br>interface: `DocumentProcessor` | case `KindImage` | application: `imageProcessor.Process`<br>external_client: `previewClient.RenderImage` |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownOrdersDecisionPointSectionsAndOmitsErrorCodes(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "ProcessDocument",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.ProcessDocument", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*ProcessDocumentRequest"},
			Response:   model.TypeRef{Type: "*ProcessDocumentResponse"},
			Trail: model.Trail{
				InterfaceCalls: []model.InterfaceCallTrace{
					{Call: model.CallRef{Symbol: "s.processor.Process", File: "handler.go", Line: 12}, Interface: "Processor"},
				},
				Branches: []model.BranchTrace{
					{Kind: "switch", Function: "Service.ProcessDocument", Expr: "req.Kind", File: "handler.go", Line: 14},
				},
				Dispatches: []model.DispatchTrace{
					{Table: "processors", Key: "req.Kind", Call: model.CallRef{Symbol: "processor.Process", File: "handler.go", Line: 18}},
				},
			},
			Errors: model.ErrorSummary{GRPCCodes: []string{"InvalidArgument"}},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	interfaceIndex := strings.Index(got, "#### interface calls")
	branchesIndex := strings.Index(got, "#### branches")
	dispatchesIndex := strings.Index(got, "#### dispatches")
	if interfaceIndex < 0 || branchesIndex < 0 || dispatchesIndex < 0 {
		t.Fatalf("markdown output is missing decision point sections:\n%s", got)
	}
	if !(interfaceIndex < branchesIndex && branchesIndex < dispatchesIndex) {
		t.Fatalf("markdown decision point sections are not in stable order:\n%s", got)
	}
	if strings.Contains(got, "Error Codes") || strings.Contains(got, "InvalidArgument") {
		t.Fatalf("markdown output includes project-specific error summary:\n%s", got)
	}
}

func TestWriteMarkdownUsesConfiguredLayerForExternalCalls(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetWorkspace",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.GetWorkspace", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*GetWorkspaceRequest"},
			Response:   model.TypeRef{Type: "*Workspace"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "external_client",
						Calls: []model.CallRef{
							{Symbol: "s.clients.Directory.GetProfile", Receiver: "s.clients.Directory", Method: "GetProfile", File: "usecase.go", Line: 20, Depth: 2},
							{Symbol: "s.clients.Preference.GetSettings", Receiver: "s.clients.Preference", Method: "GetSettings", File: "usecase.go", Line: 22, Depth: 2},
							{Symbol: "archive.GetDocument", Receiver: "archive", Method: "GetDocument", File: "archive.go", Line: 30, Depth: 3, Via: "clients.Archive.GetDocument"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"- [external_client] `s.clients.Directory.GetProfile` (usecase.go:20)",
		"- [external_client] `s.clients.Preference.GetSettings` (usecase.go:22)",
		"| `archive.GetDocument` | `archive.go:30` | 1 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownIncludesEntrypointTypeSwitchAsBranch(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetReport",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.GetReport", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*GetReportRequest"},
			Response:   model.TypeRef{Type: "*Report"},
			Trail: model.Trail{
				Branches: []model.BranchTrace{
					{
						Kind:     "type_switch",
						Function: "Service.GetReport",
						Expr:     "payload := req.Payload.(type)",
						File:     "handler.go",
						Line:     12,
						Cases: []model.BranchCase{
							{
								Labels: []string{"*GetReportRequest_V1"},
								Layers: []model.LayerCalls{
									{
										Name: "usecase",
										Calls: []model.CallRef{
											{Symbol: "report.Get", Receiver: "report", Method: "Get", File: "usecase.go", Line: 20, Depth: 2},
										},
									},
								},
							},
							{
								Default: true,
								Unknown: []model.CallRef{
									{Symbol: "errors.NewInvalidArgumentErr", Receiver: "errors", Method: "NewInvalidArgumentErr", File: "handler.go", Line: 28, Depth: 1},
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"### decision points\n\n#### branches\n\n| function | condition | case | calls |",
		"| `Service.GetReport` (handler.go:12) | type switch `payload := req.Payload.(type)` | case `*GetReportRequest_V1` | usecase: `report.Get` |",
		"| `Service.GetReport` (handler.go:12) | type switch `payload := req.Payload.(type)` | default | other: `errors.NewInvalidArgumentErr` |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownIncludesInterfaceCallSummary(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "GetFoo",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Server.GetFoo",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*pb.GetFooRequest"},
			Response: model.TypeRef{Type: "*pb.GetFooResponse"},
			Trail: model.Trail{
				InterfaceCalls: []model.InterfaceCallTrace{
					{
						Call:      model.CallRef{Symbol: "s.fooUsecase.GetFoo", File: "handler.go", Line: 12},
						Interface: "FooUsecase",
						Implementations: []model.ImplementationCandidate{
							{
								Call:     model.CallRef{Symbol: "fooUsecase.GetFoo", File: "usecase.go", Line: 20},
								Expanded: true,
							},
							{
								Call: model.CallRef{Symbol: "otherFooUsecase.GetFoo", File: "other_usecase.go", Line: 18},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"### decision points\n\n#### interface calls\n\n| call | interface | candidates | resolution |",
		"| `s.fooUsecase.GetFoo` (handler.go:12) | `FooUsecase` | `fooUsecase.GetFoo` (usecase.go:20) expanded<br>`otherFooUsecase.GetFoo` (other_usecase.go:18) candidate | partial |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownSplitsUnresolvedInterfaceCalls(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetFoo",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Server.GetFoo", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*pb.GetFooRequest"},
			Response:   model.TypeRef{Type: "*pb.GetFooResponse"},
			Trail: model.Trail{
				InterfaceCalls: []model.InterfaceCallTrace{
					{
						Call:      model.CallRef{Symbol: "s.fooUsecase.GetFoo", File: "handler.go", Line: 12},
						Interface: "FooUsecase",
						Implementations: []model.ImplementationCandidate{
							{Call: model.CallRef{Symbol: "fooUsecase.GetFoo", File: "usecase.go", Line: 20}, Expanded: true},
						},
					},
					{
						Call:      model.CallRef{Symbol: "s.inventory.List", File: "usecase.go", Line: 24},
						Interface: "InventoryClient",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"| `s.fooUsecase.GetFoo` (handler.go:12) | `FooUsecase` | `fooUsecase.GetFoo` (usecase.go:20) expanded | single expanded |",
		"| `s.inventory.List` (usecase.go:24) | `InventoryClient` | - | unresolved |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownOmitsInterfaceCallsWithOnlyInternalHelperImplementations(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetFoo",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Server.GetFoo", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*pb.GetFooRequest"},
			Response:   model.TypeRef{Type: "*pb.GetFooResponse"},
			Trail: model.Trail{
				InterfaceCalls: []model.InterfaceCallTrace{
					{
						Call:      model.CallRef{Symbol: "u.helper.normalize", Method: "normalize", File: "usecase.go", Line: 12},
						Interface: "DocumentNormalizer",
						Implementations: []model.ImplementationCandidate{
							{Call: model.CallRef{Symbol: "documentNormalizer.normalize", Method: "normalize", File: "normalizer.go", Line: 8, Depth: 2, Via: "u.helper.normalize"}, Expanded: true},
						},
					},
					{
						Call:      model.CallRef{Symbol: "s.fooUsecase.GetFoo", Method: "GetFoo", File: "handler.go", Line: 14},
						Interface: "FooUsecase",
						Implementations: []model.ImplementationCandidate{
							{Call: model.CallRef{Symbol: "fooUsecase.GetFoo", Method: "GetFoo", File: "usecase.go", Line: 20, Depth: 2, Via: "s.fooUsecase.GetFoo"}, Expanded: true},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "u.helper.normalize") || strings.Contains(got, "documentNormalizer.normalize") {
		t.Fatalf("markdown output includes internal helper interface call:\n%s", got)
	}
	if !strings.Contains(got, "s.fooUsecase.GetFoo") {
		t.Fatalf("markdown output omitted useful interface call:\n%s", got)
	}
}

func TestWriteMarkdownDoesNotCollapseAmbiguousImplementations(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name: "ProcessDocument",
			Kind: "grpc",
			Entrypoint: model.Entrypoint{
				Symbol: "Service.ProcessDocument",
				File:   "handler.go",
				Line:   10,
			},
			Request:  model.TypeRef{Type: "*ProcessDocumentRequest"},
			Response: model.TypeRef{Type: "*ProcessDocumentResponse"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "domain",
						Calls: []model.CallRef{
							{Symbol: "asset.Normalize", Receiver: "asset", Method: "Normalize", File: "application.go", Line: 20, Depth: 2, Via: "documentApplication.ProcessDocument"},
							{Symbol: "MarkdownAsset.Normalize", Receiver: "MarkdownAsset", Method: "Normalize", File: "application.go", Line: 8, Depth: 3, Via: "asset.Normalize"},
							{Symbol: "asset.Normalize", Receiver: "asset", Method: "Normalize", File: "application.go", Line: 24, Depth: 2, Via: "documentApplication.ProcessDocument"},
							{Symbol: "ImageAsset.Normalize", Receiver: "ImageAsset", Method: "Normalize", File: "application.go", Line: 14, Depth: 3, Via: "asset.Normalize"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteMarkdown returned error: %v", err)
	}

	got := buf.String()
	for _, want := range []string{
		"| `asset.Normalize` | `application.go:20` | 1 |",
		"| `asset.Normalize` | `application.go:24` | 1 |",
		"| `MarkdownAsset.Normalize` | `application.go:8` | 1 |",
		"| `ImageAsset.Normalize` | `application.go:14` | 1 |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "implementation: application.go:8") || strings.Contains(got, "related internal call") {
		t.Fatalf("markdown output collapsed ambiguous implementations:\n%s", got)
	}
}
