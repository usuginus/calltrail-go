package analyzer

import (
	"go/ast"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

func isHandler(fn *ast.FuncDecl, packageName string, file string, handlerRules rules.HandlerRules) bool {
	if !matchesAnyEqual(packageName, handlerRules.Match.PackageNames) &&
		!matchesAnyContains(file, handlerRules.Match.FilePathContains) {
		return false
	}
	if fn.Recv == nil || fn.Type.Params == nil || fn.Type.Results == nil {
		return false
	}
	if len(fn.Type.Params.List) < 2 || len(fn.Type.Results.List) != 2 {
		return false
	}
	if handlerRules.Signature.RequireContextFirstArg && !isContextType(fn.Type.Params.List[0].Type) {
		return false
	}
	if handlerRules.Signature.RequirePointerRequest && !isPointerType(fn.Type.Params.List[1].Type) {
		return false
	}
	if handlerRules.Signature.RequirePointerResponse && !isPointerType(fn.Type.Results.List[0].Type) {
		return false
	}
	return !handlerRules.Signature.RequireErrorReturn || typeString(fn.Type.Results.List[1].Type) == "error"
}

func isContextType(expr ast.Expr) bool {
	return typeString(expr) == "context.Context"
}

func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

func classify(ref model.CallRef, scope scopeInfo, layers []rules.LayerRule) string {
	fieldType := strings.ToLower(resolveReceiverType(ref.Receiver, scope))
	for _, layer := range layers {
		if matchesLayer(ref, fieldType, layer) {
			return layer.Name
		}
	}
	return "unknown"
}

func classifyByFile(ref model.CallRef, file string, layers []rules.LayerRule) string {
	file = strings.ToLower(file)
	ref.File = file
	for _, layer := range layers {
		if matchesAnyContains(file, layer.Match.FilePathContains) {
			return layer.Name
		}
	}
	return classify(ref, scopeInfo{}, layers)
}

func isNoiseCall(ref model.CallRef, ignore rules.IgnoreRules, stdlibPackageAliases map[string]bool, scope scopeInfo) bool {
	if ref.Receiver == "" {
		return true
	}
	if matchesAnyEqual(ref.Symbol, ignore.Calls.FullNames) ||
		matchesAnyEqual(ref.Method, ignore.Calls.MethodNames) {
		return true
	}
	if matchesAnyPrefix(ref.Symbol, ignore.Calls.FullNamePrefixes) ||
		matchesAnyPrefix(ref.Method, ignore.Calls.MethodNamePrefixes) {
		return true
	}
	for _, pkg := range ignore.Calls.PackageNames {
		if isPackageCall(ref, pkg) {
			return true
		}
	}
	if ignore.StandardLibrary && isStdlibPackageCall(ref, stdlibPackageAliases) {
		return true
	}
	if strings.HasPrefix(ref.Method, "Get") && matchesAnyEqual(ref.Receiver, ignore.Getters.ReceiverNames) {
		return true
	}
	return ignore.Getters.LocalValues &&
		strings.HasPrefix(ref.Method, "Get") &&
		isLocalValueReceiver(ref.Receiver) &&
		!hasKnownReceiverType(ref.Receiver, scope)
}

func isStdlibPackageCall(ref model.CallRef, stdlibPackageAliases map[string]bool) bool {
	for alias := range stdlibPackageAliases {
		if isPackageCall(ref, alias) {
			return true
		}
	}
	return false
}

func isPackageCall(ref model.CallRef, pkg string) bool {
	return ref.Receiver == pkg || strings.HasPrefix(ref.Receiver, pkg+".")
}

func isLocalValueReceiver(receiver string) bool {
	return receiver != "" && !strings.Contains(receiver, ".") && strings.ToLower(receiver[:1]) == receiver[:1]
}

func hasKnownReceiverType(receiver string, scope scopeInfo) bool {
	if receiver == "" {
		return false
	}
	if receiver == scope.receiverVar && scope.receiverType != "" {
		return true
	}
	parts := strings.Split(receiver, ".")
	if len(parts) == 0 {
		return false
	}
	if _, ok := scope.localTypes[parts[0]]; ok {
		return true
	}
	if fields := lookupTypeMembers(scope.structFields, typeKey(scope.packageName, parts[0]), baseType(parts[0])); fields != nil {
		return true
	}
	if parts[0] == scope.receiverVar && len(parts) > 1 && scope.receiverFields[parts[1]] != "" {
		return true
	}
	return false
}

func matchesLayer(ref model.CallRef, fieldType string, layer rules.LayerRule) bool {
	symbol := strings.ToLower(ref.Symbol)
	method := strings.ToLower(ref.Method)
	fieldType = strings.ToLower(fieldType)
	return matchesAnyContains(symbol, layer.Match.CallNameContains) ||
		matchesAnyContains(fieldType, layer.Match.ReceiverTypeContains) ||
		matchesAnyPrefix(method, layer.Match.MethodNamePrefixes) ||
		matchesAnyContains(method, layer.Match.MethodNameContains)
}

func matchesAnyEqual(value string, patterns []string) bool {
	for _, pattern := range patterns {
		if value == pattern {
			return true
		}
	}
	return false
}

func matchesAnyContains(value string, patterns []string) bool {
	value = strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.Contains(value, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func matchesAnyPrefix(value string, patterns []string) bool {
	value = strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.HasPrefix(value, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func grpcCode(call *ast.CallExpr) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if typeString(selector.X) != "status" || (selector.Sel.Name != "Error" && selector.Sel.Name != "Errorf") {
		return "", false
	}
	if len(call.Args) == 0 {
		return "", false
	}
	arg, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok || typeString(arg.X) != "codes" {
		return "", false
	}
	return arg.Sel.Name, true
}
