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
		Confidence: model.Confidence{Overall: "medium"},
	}

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			ref, ok := recordCall(fset, source.displayPath, &flow, n, scope, index, 1, "", opts.Rules, source.stdlibPackageAliases)
			if ok && opts.Depth > 1 {
				for _, candidate := range resolveCandidates(ref, scope, index, opts.Rules) {
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
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ref, added := recordCall(fset, info.file, flow, call, scope, index, currentDepth, via, ruleSet, info.stdlibPackageAliases)
		if !added || currentDepth >= maxDepth {
			return true
		}
		for _, candidate := range resolveCandidates(ref, scope, index, ruleSet) {
			candidateDepth := currentDepth + 1
			recordImplementation(fset, flow, candidate, ref.Symbol, candidateDepth, ruleSet)
			if candidateDepth < maxDepth {
				traceFunctionCalls(fset, flow, candidate, index, candidateDepth, implementationSymbol(candidate), maxDepth, ruleSet)
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
	if isNoiseCall(ref, ruleSet.IgnoreCalls, stdlibPackageAliases, scope) {
		return ref, false
	}
	ref.Depth = depth
	ref.Via = via
	appendCall(flow, ref, classify(ref, scope, ruleSet.Classifiers))
	return ref, true
}

func recordImplementation(fset *token.FileSet, flow *model.APIFlow, info functionInfo, via string, depth int, ruleSet rules.RuleSet) {
	pos := fset.Position(info.fn.Pos())
	ref := model.CallRef{
		Symbol:   implementationSymbol(info),
		Receiver: info.receiverType,
		Method:   info.fn.Name.Name,
		File:     info.file,
		Line:     pos.Line,
		Depth:    depth,
		Via:      via,
	}
	appendCall(flow, ref, classifyByFile(ref, info.file, ruleSet.Classifiers))
}

func appendCall(flow *model.APIFlow, ref model.CallRef, layer string) {
	switch layer {
	case "usecase":
		flow.Trail.Usecases = append(flow.Trail.Usecases, ref)
	case "service":
		flow.Trail.Services = append(flow.Trail.Services, ref)
	case "repository":
		flow.Trail.Repositories = append(flow.Trail.Repositories, ref)
	case "external_client":
		flow.Trail.ExternalClients = append(flow.Trail.ExternalClients, ref)
	case "converter":
		flow.Trail.Converters = append(flow.Trail.Converters, ref)
	case "model":
		flow.Trail.Models = append(flow.Trail.Models, model.TypeRef{Type: ref.Symbol})
	default:
		flow.Trail.Unknown = append(flow.Trail.Unknown, ref)
	}
}

func implementationSymbol(info functionInfo) string {
	if info.receiverType == "" {
		return info.fn.Name.Name
	}
	return info.receiverType + "." + info.fn.Name.Name
}
