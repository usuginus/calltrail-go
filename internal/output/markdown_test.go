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
	if !strings.Contains(got, "### domain\n- `entity.Foo.Validate`") {
		t.Fatalf("markdown output does not include configured layer:\n%s", got)
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
		"- `fooUsecase.CreateFoo`\n  - called from: `s.fooUsecase.CreateFoo` (handler.go:12)\n  - implementation: usecase.go:20",
		"- `FooRepository.FindFoo`\n  - called from:\n    - `u.repos.Foo.FindFoo` (usecase.go:23)\n    - `u.repos.Foo.FindFoo` (usecase.go:40)\n  - implementation: repository.go:30",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "repo.columns") {
		t.Fatalf("markdown output includes internal repository helper:\n%s", got)
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
	want := "- `repository.IsNotFoundError`\n  - called from: `fooUsecase.CreateFoo` (usecase.go:18)"
	if !strings.Contains(got, want) {
		t.Fatalf("markdown output does not contain %q:\n%s", want, got)
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
		"### Branches\n- `documentApplication.ProcessDocument` switch `cmd.Mode` (application.go:24)",
		"  - case `\"publish\"`\n    - persistence: `documentStore.Publish`\n    - external_client: `previewClient.Index`",
		"  - default\n    - domain: `documentPolicy.RejectUnsupportedMode`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
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
		"### Dispatches\n- `processor.Process` dispatched from `a.processors[cmd.Kind]` (application.go:44)",
		"  - interface: `DocumentProcessor`",
		"  - case `KindMarkdown`\n    - application: `markdownProcessor.Process`\n    - persistence: `documentStore.SaveMarkdown`",
		"  - case `KindImage`\n    - application: `imageProcessor.Process`\n    - external_client: `previewClient.RenderImage`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown output does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMarkdownUsesConfiguredLayerForExternalCalls(t *testing.T) {
	var buf bytes.Buffer
	err := WriteMarkdown(&buf, []model.APIFlow{
		{
			Name:       "GetDashboard",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.GetDashboard", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*GetDashboardRequest"},
			Response:   model.TypeRef{Type: "*Dashboard"},
			Trail: model.Trail{
				Layers: []model.LayerCalls{
					{
						Name: "external_client",
						Calls: []model.CallRef{
							{Symbol: "s.clients.UserAccount.GetContract", Receiver: "s.clients.UserAccount", Method: "GetContract", File: "usecase.go", Line: 20, Depth: 2},
							{Symbol: "s.clients.UserAccount.GetPaymentConfig", Receiver: "s.clients.UserAccount", Method: "GetPaymentConfig", File: "usecase.go", Line: 22, Depth: 2},
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
		"### external_client\n- `s.clients.UserAccount.GetContract`",
		"- `s.clients.UserAccount.GetPaymentConfig`",
		"- `archive.GetDocument`\n  - called from: `clients.Archive.GetDocument`",
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
			Name:       "GetDashboard",
			Kind:       "grpc",
			Entrypoint: model.Entrypoint{Symbol: "Service.GetDashboard", File: "handler.go", Line: 10},
			Request:    model.TypeRef{Type: "*GetDashboardRequest"},
			Response:   model.TypeRef{Type: "*Dashboard"},
			Trail: model.Trail{
				Branches: []model.BranchTrace{
					{
						Kind:     "type_switch",
						Function: "Service.GetDashboard",
						Expr:     "payload := req.Payload.(type)",
						File:     "handler.go",
						Line:     12,
						Cases: []model.BranchCase{
							{
								Labels: []string{"*GetDashboardRequest_V1"},
								Layers: []model.LayerCalls{
									{
										Name: "usecase",
										Calls: []model.CallRef{
											{Symbol: "dashboard.Get", Receiver: "dashboard", Method: "Get", File: "usecase.go", Line: 20, Depth: 2},
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
		"### Branches\n- `Service.GetDashboard` type switch `payload := req.Payload.(type)` (handler.go:12)",
		"  - case `*GetDashboardRequest_V1`\n    - usecase: `dashboard.Get`",
		"  - default\n    - other: `errors.NewInvalidArgumentErr`",
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
		"### Interface Calls\n- `s.fooUsecase.GetFoo` (handler.go:12)",
		"  - interface: `FooUsecase`",
		"    - `fooUsecase.GetFoo` (usecase.go:20) expanded",
		"    - `otherFooUsecase.GetFoo` (other_usecase.go:18) candidate",
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
		"### Interface Calls\n- `s.fooUsecase.GetFoo` (handler.go:12)",
		"### Unresolved Interface Calls\n- `s.inventory.List` (usecase.go:24)\n  - interface: `InventoryClient`",
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
	if !strings.Contains(got, "- `asset.Normalize`\n  - called from:") {
		t.Fatalf("markdown output does not keep ambiguous call as direct operation:\n%s", got)
	}
	if strings.Contains(got, "implementation: application.go:8") || strings.Contains(got, "related internal call") {
		t.Fatalf("markdown output collapsed ambiguous implementations:\n%s", got)
	}
}
