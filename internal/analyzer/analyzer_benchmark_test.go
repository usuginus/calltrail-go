package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
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

func BenchmarkAnalyzeMapDispatchRPCNoMatch(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/map-dispatch/.calltrail.yaml")
	opts := Options{RPC: "MissingRPC", Depth: 4, Rules: ruleSet}
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

func BenchmarkAnalyzePhasesMapDispatch(b *testing.B) {
	ruleSet := mustLoadRules(b, "../../examples/map-dispatch/.calltrail.yaml")
	opts := Options{RPC: "ProcessDocument", Depth: 4, Rules: ruleSet}

	b.Run("LoadSources", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, sources, err := loadSources([]string{"../../examples/map-dispatch"})
			if err != nil {
				b.Fatalf("loadSources returned error: %v", err)
			}
			benchmarkSources = sources
		}
	})

	b.Run("CollectGoFiles", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			files, err := collectGoFiles("../../examples/map-dispatch")
			if err != nil {
				b.Fatalf("collectGoFiles returned error: %v", err)
			}
			benchmarkSourceFiles = files
		}
	})

	files, err := collectGoFiles("../../examples/map-dispatch")
	if err != nil {
		b.Fatalf("collectGoFiles returned error: %v", err)
	}

	b.Run("ParseSources", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fset := token.NewFileSet()
			sources, err := parseSources(fset, files)
			if err != nil {
				b.Fatalf("parseSources returned error: %v", err)
			}
			benchmarkSources = sources
		}
	})

	b.Run("ParseFileOnly", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fset := token.NewFileSet()
			for _, file := range files {
				parsedFile, err := parser.ParseFile(fset, file.path, nil, 0)
				if err != nil {
					b.Fatalf("ParseFile returned error: %v", err)
				}
				benchmarkASTFile = parsedFile
			}
		}
	})

	parsedFiles := parseBenchmarkFiles(b, files)

	b.Run("CollectStructFieldTypes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, parsedFile := range parsedFiles {
				fields := collectStructFieldTypes(parsedFile)
				benchmarkFieldTypes = fields
			}
		}
	})

	b.Run("CollectStdlibPackageAliases", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			stdlibImports := make(map[string]bool)
			for _, parsedFile := range parsedFiles {
				aliases := collectStdlibPackageAliases(parsedFile, stdlibImports)
				benchmarkAliases = aliases
			}
		}
	})

	_, fset, sources, err := loadAnalysisInputs([]string{"../../examples/map-dispatch"}, opts)
	if err != nil {
		b.Fatalf("loadAnalysisInputs returned error: %v", err)
	}

	b.Run("CollectHandlers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			handlers := collectHandlers(sources, opts)
			benchmarkHandlers = handlers
		}
	})

	b.Run("BuildProjectIndex", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			index := buildProjectIndex(fset, sources)
			benchmarkIndex = index
		}
	})

	handlers := collectHandlers(sources, opts)
	index := buildProjectIndex(fset, sources)
	if len(handlers) == 0 {
		b.Fatal("no handlers found")
	}

	b.Run("BuildFlow", func(b *testing.B) {
		handler := handlers[0]
		for i := 0; i < b.N; i++ {
			flow := buildFlow(fset, handler.source, handler.fn, index, opts)
			benchmarkFlow = flow
		}
	})
}

func BenchmarkIsStdlibImportPath(b *testing.B) {
	b.Run("StdlibUncached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkBool = isStdlibImportPath("context", nil)
		}
	})

	b.Run("StdlibCached", func(b *testing.B) {
		cache := make(map[string]bool)
		for i := 0; i < b.N; i++ {
			benchmarkBool = isStdlibImportPath("context", cache)
		}
	})

	b.Run("DomainPackage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			benchmarkBool = isStdlibImportPath("go.temporal.io/server/common", nil)
		}
	})
}

func parseBenchmarkFiles(b *testing.B, files []sourceFile) []*ast.File {
	b.Helper()
	fset := token.NewFileSet()
	parsedFiles := make([]*ast.File, 0, len(files))
	for _, file := range files {
		parsedFile, err := parser.ParseFile(fset, file.path, nil, 0)
		if err != nil {
			b.Fatalf("ParseFile returned error: %v", err)
		}
		parsedFiles = append(parsedFiles, parsedFile)
	}
	return parsedFiles
}

var benchmarkSourceFiles []sourceFile
var benchmarkSources []parsedSource
var benchmarkHandlers []handlerInfo
var benchmarkIndex projectIndex
var benchmarkFlow model.APIFlow
var benchmarkASTFile *ast.File
var benchmarkFieldTypes map[string]map[string]string
var benchmarkAliases map[string]bool
var benchmarkBool bool

func mustLoadRules(b *testing.B, path string) rules.RuleSet {
	b.Helper()
	ruleSet, err := rules.Load(path)
	if err != nil {
		b.Fatalf("Load returned error: %v", err)
	}
	return ruleSet
}
