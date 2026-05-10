package analyzer

import (
	"go/ast"
	"go/token"
)

type functionInfo struct {
	fn                   *ast.FuncDecl
	file                 string
	packageName          string
	receiverType         string
	receiverTypeKey      string
	receiverVar          string
	returnType           string
	stdlibPackageAliases map[string]bool
}

type projectIndex struct {
	functionsByName          map[string][]functionInfo
	methodsByName            map[string][]functionInfo
	methodsByReceiver        map[string]map[string]bool
	interfaces               map[string]map[string]bool
	implementationAssertions map[string]map[string]bool
	structFields             map[string]map[string]string
	dispatchTables           map[string]dispatchTableInfo
}

func buildProjectIndex(fset *token.FileSet, sources []parsedSource) projectIndex {
	index := projectIndex{
		functionsByName:          make(map[string][]functionInfo),
		methodsByName:            make(map[string][]functionInfo),
		methodsByReceiver:        make(map[string]map[string]bool),
		interfaces:               make(map[string]map[string]bool),
		implementationAssertions: make(map[string]map[string]bool),
		structFields:             make(map[string]map[string]string),
		dispatchTables:           make(map[string]dispatchTableInfo),
	}
	for _, source := range sources {
		collectInterfaces(source.packageName, source.file, index.interfaces)
		collectImplementationAssertions(source.packageName, source.file, index.implementationAssertions)
		addStructFields(source.packageName, source.fieldTypes, index.structFields)
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			receiverType := receiverName(fn)
			info := functionInfo{
				fn:                   fn,
				file:                 source.displayPath,
				packageName:          source.packageName,
				receiverType:         receiverType,
				receiverTypeKey:      typeKey(source.packageName, receiverType),
				receiverVar:          receiverVarName(fn),
				returnType:           firstReturnType(fn),
				stdlibPackageAliases: source.stdlibPackageAliases,
			}
			if fn.Recv == nil {
				index.functionsByName[fn.Name.Name] = append(index.functionsByName[fn.Name.Name], info)
				continue
			}
			index.methodsByName[fn.Name.Name] = append(index.methodsByName[fn.Name.Name], info)
			addReceiverMethod(index.methodsByReceiver, info.receiverTypeKey, fn.Name.Name)
			addReceiverMethod(index.methodsByReceiver, info.receiverType, fn.Name.Name)
		}
	}
	for _, source := range sources {
		collectDispatchTables(fset, source.packageName, source.file, &index)
	}
	return index
}

func addStructFields(packageName string, fieldTypes map[string]map[string]string, out map[string]map[string]string) {
	for name, fields := range fieldTypes {
		out[typeKey(packageName, name)] = fields
		if _, exists := out[name]; !exists {
			out[name] = fields
		}
	}
}

func addReceiverMethod(methodsByReceiver map[string]map[string]bool, receiverType string, methodName string) {
	if receiverType == "" {
		return
	}
	if methodsByReceiver[receiverType] == nil {
		methodsByReceiver[receiverType] = make(map[string]bool)
	}
	methodsByReceiver[receiverType][methodName] = true
}

func collectImplementationAssertions(packageName string, file *ast.File, assertions map[string]map[string]bool) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok || len(valueSpec.Names) != 1 || valueSpec.Names[0].Name != "_" || valueSpec.Type == nil {
				continue
			}
			interfaceType := typeKey(packageName, typeString(valueSpec.Type))
			for _, value := range valueSpec.Values {
				concreteType := assertedConcreteType(value)
				if concreteType == "" {
					continue
				}
				addImplementationAssertion(assertions, interfaceType, typeKey(packageName, concreteType))
				addImplementationAssertion(assertions, baseType(interfaceType), concreteType)
			}
		}
	}
}

func addImplementationAssertion(assertions map[string]map[string]bool, interfaceType string, concreteType string) {
	if interfaceType == "" || concreteType == "" {
		return
	}
	if assertions[interfaceType] == nil {
		assertions[interfaceType] = make(map[string]bool)
	}
	assertions[interfaceType][concreteType] = true
}

func assertedConcreteType(expr ast.Expr) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return ""
	}
	fun := call.Fun
	if paren, ok := fun.(*ast.ParenExpr); ok {
		fun = paren.X
	}
	star, ok := fun.(*ast.StarExpr)
	if !ok {
		return ""
	}
	return baseType(typeString(star.X))
}

func collectInterfaces(packageName string, file *ast.File, interfaces map[string]map[string]bool) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok || iface.Methods == nil {
				continue
			}
			methods := make(map[string]bool)
			for _, method := range iface.Methods.List {
				for _, name := range method.Names {
					methods[name.Name] = true
				}
			}
			interfaces[typeKey(packageName, typeSpec.Name.Name)] = methods
			if _, exists := interfaces[typeSpec.Name.Name]; !exists {
				interfaces[typeSpec.Name.Name] = methods
			}
		}
	}
}

func collectStructFieldTypes(file *ast.File) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok || structType.Fields == nil {
				continue
			}
			fields := make(map[string]string)
			for _, field := range structType.Fields.List {
				fieldType := typeString(field.Type)
				for _, name := range field.Names {
					fields[name.Name] = fieldType
				}
			}
			out[typeSpec.Name.Name] = fields
		}
	}
	return out
}
