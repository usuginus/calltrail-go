package analyzer

import (
	"go/ast"
	"go/token"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

func buildFlow(fset *token.FileSet, source parsedSource, fn *ast.FuncDecl, index projectIndex, opts Options) model.APIFlow {
	pos := fset.Position(fn.Pos())
	receiverType := receiverName(fn)
	scope := newScope(fn, index, receiverType, receiverVarName(fn), index.structFields[receiverType])
	flow := model.APIFlow{
		Name: fn.Name.Name,
		Kind: "grpc",
		Entrypoint: model.Entrypoint{
			Symbol: receiverType + "." + fn.Name.Name,
			File:   source.displayPath,
			Line:   pos.Line,
		},
		Request: model.TypeRef{
			Type: typeString(fn.Type.Params.List[1].Type),
		},
		Response: model.TypeRef{
			Type: typeString(fn.Type.Results.List[0].Type),
		},
		Trail:      model.NewTrail(layerNames(opts.Rules.Layers)),
		Confidence: model.Confidence{Overall: "medium"},
	}

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SwitchStmt:
			recordSwitchBranchTrace(fset, source.displayPath, &flow, n, scope, index, 1, flow.Entrypoint.Symbol, opts.Depth, opts.Rules, source.stdlibPackageAliases)
		case *ast.TypeSwitchStmt:
			recordTypeSwitchBranchTrace(fset, source.displayPath, &flow, n, scope, index, 1, flow.Entrypoint.Symbol, opts.Depth, opts.Rules, source.stdlibPackageAliases)
			traceTypeSwitchCasesForFlow(fset, source.displayPath, &flow, n, scope, index, 1, "", opts.Depth, opts.Rules, source.stdlibPackageAliases)
			return false
		case *ast.CallExpr:
			ref, ok := recordCall(fset, source.displayPath, &flow, n, scope, index, 1, "", opts.Rules, source.stdlibPackageAliases)
			if ok {
				candidateDepth := 2
				resolved := resolveCall(ref, scope, index, opts.Rules)
				recordInterfaceCall(fset, &flow, ref, resolved, candidateDepth, opts.Depth)
				if opts.Depth <= 1 {
					return true
				}
				for _, candidate := range resolved.candidates {
					recordImplementation(fset, &flow, candidate, ref.Symbol, 2, opts.Rules)
					traceFunctionCalls(fset, &flow, candidate, index, 2, implementationSymbol(candidate), opts.Depth, opts.Rules)
				}
			}
		case *ast.GoStmt:
			if call := n.Call; call != nil {
				ref := callRef(fset, source.displayPath, call, index, scope)
				if ref.Symbol != "" {
					ref.Depth = 1
					flow.Trail.Async = append(flow.Trail.Async, ref)
				}
			}
		}
		return true
	})
	flow.Errors.GRPCCodes = unique(flow.Errors.GRPCCodes)
	return flow
}

func traceFunctionCalls(
	fset *token.FileSet,
	flow *model.APIFlow,
	info functionInfo,
	index projectIndex,
	currentDepth int,
	via string,
	maxDepth int,
	ruleSet rules.RuleSet,
) {
	if currentDepth > maxDepth {
		return
	}
	scope := newScope(info.fn, index, info.receiverType, info.receiverVar, info.fieldTypes[info.receiverType])
	ast.Inspect(info.fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SwitchStmt:
			recordSwitchBranchTrace(fset, info.file, flow, n, scope, index, currentDepth, implementationSymbol(info), maxDepth, ruleSet, info.stdlibPackageAliases)
		case *ast.TypeSwitchStmt:
			recordTypeSwitchBranchTrace(fset, info.file, flow, n, scope, index, currentDepth, implementationSymbol(info), maxDepth, ruleSet, info.stdlibPackageAliases)
			traceTypeSwitchCasesForFlow(fset, info.file, flow, n, scope, index, currentDepth, via, maxDepth, ruleSet, info.stdlibPackageAliases)
			return false
		case *ast.CallExpr:
			ref, added := recordCall(fset, info.file, flow, n, scope, index, currentDepth, via, ruleSet, info.stdlibPackageAliases)
			if !added {
				return true
			}
			candidateDepth := currentDepth + 1
			resolved := resolveCall(ref, scope, index, ruleSet)
			recordInterfaceCall(fset, flow, ref, resolved, candidateDepth, maxDepth)
			if currentDepth >= maxDepth {
				return true
			}
			for _, candidate := range resolved.candidates {
				recordImplementation(fset, flow, candidate, ref.Symbol, candidateDepth, ruleSet)
				if candidateDepth < maxDepth {
					traceFunctionCalls(fset, flow, candidate, index, candidateDepth, implementationSymbol(candidate), maxDepth, ruleSet)
				}
			}
		}
		return true
	})
}

func recordCall(
	fset *token.FileSet,
	file string,
	flow *model.APIFlow,
	call *ast.CallExpr,
	scope scopeInfo,
	index projectIndex,
	depth int,
	via string,
	ruleSet rules.RuleSet,
	stdlibPackageAliases map[string]bool,
) (model.CallRef, bool) {
	ref := callRef(fset, file, call, index, scope)
	if ref.Symbol == "" {
		return ref, false
	}
	if code, ok := grpcCode(call); ok {
		flow.Errors.GRPCCodes = append(flow.Errors.GRPCCodes, code)
		return ref, false
	}
	if isNoiseCall(ref, ruleSet.Ignore, stdlibPackageAliases, scope) {
		return ref, false
	}
	ref.Depth = depth
	ref.Via = via
	appendCall(flow, ref, classify(ref, scope, ruleSet.Layers))
	return ref, true
}

func recordImplementation(fset *token.FileSet, flow *model.APIFlow, info functionInfo, via string, depth int, ruleSet rules.RuleSet) {
	ref := implementationRef(fset, info, via, depth)
	appendCall(flow, ref, classifyByFile(ref, info.file, ruleSet.Layers))
}

func implementationRef(fset *token.FileSet, info functionInfo, via string, depth int) model.CallRef {
	pos := fset.Position(info.fn.Pos())
	return model.CallRef{
		Symbol:   implementationSymbol(info),
		Receiver: info.receiverType,
		Method:   info.fn.Name.Name,
		File:     info.file,
		Line:     pos.Line,
		Depth:    depth,
		Via:      via,
	}
}

func appendCall(flow *model.APIFlow, ref model.CallRef, layer string) {
	if layer == "unknown" {
		flow.Trail.Unknown = append(flow.Trail.Unknown, ref)
		return
	}
	flow.Trail.AppendLayerCall(layer, ref)
}

func implementationSymbol(info functionInfo) string {
	if info.receiverType == "" {
		return info.fn.Name.Name
	}
	return info.receiverType + "." + info.fn.Name.Name
}

func layerNames(layers []rules.LayerRule) []string {
	names := make([]string, 0, len(layers))
	for _, layer := range layers {
		names = append(names, layer.Name)
	}
	return names
}
