package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

type scopeInfo struct {
	receiverType   string
	receiverVar    string
	receiverFields map[string]string
	localTypes     map[string]string
	structFields   map[string]map[string]string
}

func newScope(fn *ast.FuncDecl, index projectIndex, receiverType string, receiverVar string, receiverFields map[string]string) scopeInfo {
	scope := scopeInfo{
		receiverType:   receiverType,
		receiverVar:    receiverVar,
		receiverFields: receiverFields,
		structFields:   index.structFields,
	}
	scope.localTypes = collectLocalTypes(fn.Body, index, scope)
	return scope
}

func resolveCandidates(ref model.CallRef, scope scopeInfo, index projectIndex, ruleSet rules.RuleSet) []functionInfo {
	resolvedType := resolveReceiverType(ref.Receiver, scope)
	fieldType := baseType(resolvedType)
	fieldTypeIsInterface := fieldType != "" && len(index.interfaces[fieldType]) > 0
	if fieldTypeIsInterface && !strings.Contains(resolvedType, ".") && len(index.structFields[fieldType]) > 0 {
		fieldTypeIsInterface = false
	}
	if fieldTypeIsInterface {
		if methods := index.interfaces[fieldType]; len(methods) > 0 && !methods[ref.Method] {
			return nil
		}
	}

	var candidates []functionInfo
	for _, candidate := range index.methodsByName[ref.Method] {
		if candidate.fn == nil || candidate.receiverType == "" {
			continue
		}
		if fieldTypeIsInterface && candidate.receiverType == fieldType {
			continue
		}
		if isMockCandidate(candidate, ruleSet.Resolution) {
			continue
		}
		if fieldTypeIsInterface {
			if asserted := index.implementationAssertions[fieldType]; len(asserted) > 0 && !asserted[candidate.receiverType] {
				continue
			}
			if !implementsInterface(candidate.receiverType, fieldType, index) {
				continue
			}
		}
		if fieldType != "" && !fieldTypeIsInterface && candidate.receiverType != fieldType {
			continue
		}
		if fieldType == "" && candidate.receiverType != strings.TrimPrefix(ref.Receiver, "*") {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func implementsInterface(receiverType string, interfaceType string, index projectIndex) bool {
	interfaceMethods := index.interfaces[interfaceType]
	if len(interfaceMethods) == 0 {
		return true
	}
	receiverMethods := index.methodsByReceiver[receiverType]
	if len(receiverMethods) == 0 {
		return false
	}
	for method := range interfaceMethods {
		if !receiverMethods[method] {
			return false
		}
	}
	return true
}

func isMockCandidate(candidate functionInfo, resolution rules.ResolutionRules) bool {
	if matchesAnyPrefix(candidate.receiverType, resolution.SkipImplementations.ReceiverNamePrefixes) {
		return true
	}
	return matchesAnyContains(strings.ToLower(candidate.file), resolution.SkipImplementations.FilePathContains)
}

func callRef(fset *token.FileSet, file string, call *ast.CallExpr, index projectIndex, scope scopeInfo) model.CallRef {
	pos := fset.Position(call.Pos())
	ref := model.CallRef{File: file, Line: pos.Line}
	target := callTarget(call.Fun, index, scope)
	ref.Receiver = target.Receiver
	ref.Method = target.Method
	ref.Symbol = target.Symbol
	return ref
}

func callTarget(fun ast.Expr, index projectIndex, scope scopeInfo) model.CallRef {
	var ref model.CallRef
	switch fn := fun.(type) {
	case *ast.SelectorExpr:
		ref.Receiver = selectorReceiver(fn.X, index, scope)
		ref.Method = fn.Sel.Name
		if ref.Receiver == "" {
			return ref
		}
		if innerCall, ok := fn.X.(*ast.CallExpr); ok {
			if innerSymbol := callDisplaySymbol(innerCall, index, scope); innerSymbol != "" {
				ref.Symbol = innerSymbol + "." + ref.Method
				return ref
			}
		}
		ref.Symbol = ref.Receiver + "." + ref.Method
	case *ast.Ident:
		ref.Symbol = fn.Name
		ref.Method = fn.Name
	}
	return ref
}

func callDisplaySymbol(call *ast.CallExpr, index projectIndex, scope scopeInfo) string {
	ref := callTarget(call.Fun, index, scope)
	if ref.Symbol == "" {
		return ""
	}
	return ref.Symbol + "()"
}

func selectorReceiver(expr ast.Expr, index projectIndex, scope scopeInfo) string {
	switch fn := expr.(type) {
	case *ast.CallExpr:
		return baseType(callReturnType(fn, index, scope))
	default:
		return typeString(expr)
	}
}

func collectLocalTypes(body *ast.BlockStmt, index projectIndex, scope scopeInfo) map[string]string {
	out := make(map[string]string)
	if body == nil {
		return out
	}
	scope.localTypes = out
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				name, ok := lhs.(*ast.Ident)
				if !ok || name.Name == "_" || i >= len(n.Rhs) {
					continue
				}
				if typ := inferExprType(n.Rhs[i], index, scope); typ != "" {
					out[name.Name] = typ
				}
			}
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if name.Name == "_" {
					continue
				}
				if n.Type != nil {
					out[name.Name] = typeString(n.Type)
					continue
				}
				if i < len(n.Values) {
					if typ := inferExprType(n.Values[i], index, scope); typ != "" {
						out[name.Name] = typ
					}
				}
			}
		}
		return true
	})
	return out
}

func inferExprType(expr ast.Expr, index projectIndex, scope scopeInfo) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		return callReturnType(e, index, scope)
	case *ast.CompositeLit:
		return typeString(e.Type)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return inferExprType(e.X, index, scope)
		}
	case *ast.Ident:
		return scope.localTypes[e.Name]
	}
	return ""
}

func callReturnType(call *ast.CallExpr, index projectIndex, scope scopeInfo) string {
	ref := callTarget(call.Fun, index, scope)
	if ref.Method == "" {
		return ""
	}
	if ref.Receiver != "" {
		if typ := lookupFunctionReturnType(ref, index); typ != "" {
			return typ
		}
		return commonReturnType(resolveCandidates(ref, scope, index, rules.RuleSet{}))
	}
	return commonReturnType(index.functionsByName[ref.Method])
}

func lookupFunctionReturnType(ref model.CallRef, index projectIndex) string {
	var matches []functionInfo
	for _, candidate := range index.functionsByName[ref.Method] {
		if candidate.packageName == ref.Receiver {
			matches = append(matches, candidate)
		}
	}
	return commonReturnType(matches)
}

func commonReturnType(candidates []functionInfo) string {
	var typ string
	for _, candidate := range candidates {
		if candidate.returnType == "" {
			continue
		}
		if typ == "" {
			typ = candidate.returnType
			continue
		}
		if typ != candidate.returnType {
			return ""
		}
	}
	return typ
}

func resolveReceiverType(receiver string, scope scopeInfo) string {
	if receiver == "" {
		return ""
	}
	if receiver == scope.receiverVar {
		return scope.receiverType
	}
	if typ := scope.localTypes[receiver]; typ != "" {
		return typ
	}
	parts := strings.Split(receiver, ".")
	if len(parts) == 0 {
		return ""
	}
	if parts[0] == scope.receiverVar {
		return resolveFieldChain(scope.receiverFields[parts[1]], parts[2:], scope.structFields)
	}
	if typ := scope.localTypes[parts[0]]; typ != "" {
		return resolveFieldChain(typ, parts[1:], scope.structFields)
	}
	return receiver
}

func resolveFieldChain(currentType string, fields []string, structFields map[string]map[string]string) string {
	for _, field := range fields {
		typeFields := structFields[baseType(currentType)]
		if typeFields == nil {
			return currentType
		}
		nextType := typeFields[field]
		if nextType == "" {
			return currentType
		}
		currentType = nextType
	}
	return currentType
}
