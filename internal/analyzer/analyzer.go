package analyzer

import (
	"go/ast"

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
	opts, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}

	fset, sources, err := loadSources(paths)
	if err != nil {
		return nil, err
	}

	index := buildProjectIndex(sources)
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
			flows = append(flows, buildFlow(fset, source, fn, index, opts))
		}
	}
	return flows, nil
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
