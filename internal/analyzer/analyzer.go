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

	handlers := collectHandlers(sources, opts)
	if len(handlers) == 0 {
		return nil, nil
	}
	index := buildProjectIndex(fset, sources)
	return buildHandlerFlows(handlers, func(handler handlerInfo) model.APIFlow {
		return buildFlow(fset, handler.source, handler.fn, index, opts)
	}), nil
}

// DetectHandlers parses the provided Go paths and returns one flow header per
// detected API handler without building downstream call trails.
func DetectHandlers(paths []string, opts Options) ([]model.APIFlow, error) {
	opts, fset, sources, err := loadAnalysisInputs(paths, opts)
	if err != nil {
		return nil, err
	}

	handlers := collectHandlers(sources, opts)
	return buildHandlerFlows(handlers, func(handler handlerInfo) model.APIFlow {
		return buildFlowHeader(fset, handler.source, handler.fn)
	}), nil
}

type handlerInfo struct {
	source parsedSource
	fn     *ast.FuncDecl
}

type handlerFlowBuilder func(handlerInfo) model.APIFlow

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

func collectHandlers(sources []parsedSource, opts Options) []handlerInfo {
	var handlers []handlerInfo
	for _, source := range sources {
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !isHandler(fn, source.file.Name.Name, source.displayPath, opts.Rules.Handlers) {
				continue
			}
			if opts.RPC != "" && fn.Name.Name != opts.RPC {
				continue
			}
			handlers = append(handlers, handlerInfo{source: source, fn: fn})
		}
	}
	return handlers
}

func buildHandlerFlows(handlers []handlerInfo, build handlerFlowBuilder) []model.APIFlow {
	flows := make([]model.APIFlow, 0, len(handlers))
	for _, handler := range handlers {
		flows = append(flows, build(handler))
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
