package analyzer

import (
	"go/ast"
	"strings"

	"github.com/usuginus/calltrail-go/internal/model"
	"github.com/usuginus/calltrail-go/internal/rules"
)

func isHandler(fn *ast.FuncDecl, packageName string, file string, handlerRules rules.HandlerRules) bool {
	if !matchesAnyEqual(packageName, handlerRules.PackageNames) && !matchesAnyContains(file, handlerRules.PathContains) {
		return false
	}
	if fn.Recv == nil || fn.Type.Params == nil || fn.Type.Results == nil {
		return false
	}
	if len(fn.Type.Params.List) < 2 || len(fn.Type.Results.List) != 2 {
		return false
	}
	if handlerRules.RequireContextFirstArg && !isContextType(fn.Type.Params.List[0].Type) {
		return false
	}
	if handlerRules.RequirePointerRequest && !isPointerType(fn.Type.Params.List[1].Type) {
		return false
	}
	if handlerRules.RequirePointerResponse && !isPointerType(fn.Type.Results.List[0].Type) {
		return false
	}
	return !handlerRules.RequireErrorReturn || typeString(fn.Type.Results.List[1].Type) == "error"
}

func isContextType(expr ast.Expr) bool {
	return typeString(expr) == "context.Context"
}

func isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

func classify(ref model.CallRef, scope scopeInfo, classifiers []rules.ClassifierRule) string {
	fieldType := strings.ToLower(resolveReceiverType(ref.Receiver, scope))
	for _, classifier := range classifiers {
		if matchesClassifier(ref, fieldType, classifier) {
			return classifier.Layer
		}
	}
	return "unknown"
}

func classifyByFile(ref model.CallRef, file string, classifiers []rules.ClassifierRule) string {
	file = strings.ToLower(file)
	ref.File = file
	for _, classifier := range classifiers {
		if matchesAnyContains(file, classifier.PathContains) {
			return classifier.Layer
		}
	}
	return classify(ref, scopeInfo{}, classifiers)
}

func isNoiseCall(ref model.CallRef, ignore rules.IgnoreCallRules, stdlibPackageAliases map[string]bool, scope scopeInfo) bool {
	if ref.Receiver == "" {
		return true
	}
	if matchesAnyEqual(ref.Symbol, ignore.Symbols) || matchesAnyEqual(ref.Method, ignore.Methods) {
		return true
	}
	if matchesAnyPrefix(ref.Symbol, ignore.SymbolPrefixes) || matchesAnyPrefix(ref.Method, ignore.MethodPrefixes) {
		return true
	}
	for _, pkg := range ignore.Packages {
		if isPackageCall(ref, pkg) {
			return true
		}
	}
	if ignore.AutoStdlib && isStdlibPackageCall(ref, stdlibPackageAliases) {
		return true
	}
	if strings.HasPrefix(ref.Method, "Get") && matchesAnyEqual(ref.Receiver, ignore.ProtoGetterReceivers) {
		return true
	}
	return ignore.LocalGetters &&
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
	if _, ok := scope.structFields[baseType(parts[0])]; ok {
		return true
	}
	if parts[0] == scope.receiverVar && len(parts) > 1 && scope.receiverFields[parts[1]] != "" {
		return true
	}
	return false
}

func matchesClassifier(ref model.CallRef, fieldType string, classifier rules.ClassifierRule) bool {
	symbol := strings.ToLower(ref.Symbol)
	method := strings.ToLower(ref.Method)
	fieldType = strings.ToLower(fieldType)
	return matchesAnyContains(symbol, classifier.SymbolContains) ||
		matchesAnyContains(fieldType, classifier.TypeContains) ||
		matchesAnyPrefix(method, classifier.MethodPrefixes) ||
		matchesAnyContains(method, classifier.MethodContains)
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
