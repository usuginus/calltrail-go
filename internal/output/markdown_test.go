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
