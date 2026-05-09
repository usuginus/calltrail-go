package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

// Options controls how much of each handler's call trail is followed.
type Options struct {
	RPC   string
	Depth int
	Rules rules.RuleSet
}

// Analyze parses the provided Go paths and returns one flow per detected API
// handler. It intentionally stays syntax-driven so it can run on large repos
// without compiling or loading project dependencies.
func Analyze(paths []string, opts Options) ([]model.APIFlow, error) {
	opts, fset, sources, err := loadAnalysisInputs(paths, opts)
	if err != nil {
		return nil, err
	}

	index := buildProjectIndex(fset, sources)
	return collectHandlerFlows(sources, opts, func(source parsedSource, fn *ast.FuncDecl) model.APIFlow {
		return buildFlow(fset, source, fn, index, opts)
	}), nil
}

// DetectHandlers parses the provided Go paths and returns one flow header per
// detected API handler without building downstream call trails.
func DetectHandlers(paths []string, opts Options) ([]model.APIFlow, error) {
	opts, fset, sources, err := loadAnalysisInputs(paths, opts)
	if err != nil {
		return nil, err
	}

	return collectHandlerFlows(sources, opts, func(source parsedSource, fn *ast.FuncDecl) model.APIFlow {
		return buildFlowHeader(fset, source, fn)
	}), nil
}

type handlerFlowBuilder func(parsedSource, *ast.FuncDecl) model.APIFlow

func loadAnalysisInputs(paths []string, opts Options) (Options, *token.FileSet, []parsedSource, error) {
	opts, err := normalizeOptions(opts)
	if err != nil {
		return Options{}, nil, nil, err
	}

	fset, sources, err := loadSources(paths)
	if err != nil {
		return Options{}, nil, nil, err
	}
	return opts, fset, sources, nil
}

func collectHandlerFlows(sources []parsedSource, opts Options, build handlerFlowBuilder) []model.APIFlow {
	var flows []model.APIFlow
	for _, source := range sources {
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isHandler(fn, source.file.Name.Name, source.displayPath, opts.Rules.Handlers) {
				continue
			}
			if opts.RPC != "" && fn.Name.Name != opts.RPC {
				continue
			}
			flows = append(flows, build(source, fn))
		}
	}
	return flows
}

func normalizeOptions(opts Options) (Options, error) {
	if opts.Depth < 1 {
		opts.Depth = 1
	}
	if opts.Rules.IsZero() {
		ruleSet, err := rules.Load("")
		if err != nil {
			return Options{}, err
		}
		opts.Rules = ruleSet
	}
	return opts, nil
}
