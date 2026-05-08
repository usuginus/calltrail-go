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
	receiverVar          string
	returnType           string
	fieldTypes           map[string]map[string]string
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
		collectInterfaces(source.file, index.interfaces)
		collectImplementationAssertions(source.file, index.implementationAssertions)
		for name, fields := range source.fieldTypes {
			index.structFields[name] = fields
		}
		for _, decl := range source.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			info := functionInfo{
				fn:                   fn,
				file:                 source.displayPath,
				packageName:          source.packageName,
				receiverType:         receiverName(fn),
				receiverVar:          receiverVarName(fn),
				returnType:           firstReturnType(fn),
				fieldTypes:           index.structFields,
				stdlibPackageAliases: source.stdlibPackageAliases,
			}
			if fn.Recv == nil {
				index.functionsByName[fn.Name.Name] = append(index.functionsByName[fn.Name.Name], info)
				continue
			}
			index.methodsByName[fn.Name.Name] = append(index.methodsByName[fn.Name.Name], info)
			if index.methodsByReceiver[info.receiverType] == nil {
				index.methodsByReceiver[info.receiverType] = make(map[string]bool)
			}
			index.methodsByReceiver[info.receiverType][fn.Name.Name] = true
		}
	}
	for _, source := range sources {
		collectDispatchTables(fset, source.file, &index)
	}
	return index
}

func collectImplementationAssertions(file *ast.File, assertions map[string]map[string]bool) {
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
			interfaceType := baseType(typeString(valueSpec.Type))
			for _, value := range valueSpec.Values {
				concreteType := assertedConcreteType(value)
				if concreteType == "" {
					continue
				}
				if assertions[interfaceType] == nil {
					assertions[interfaceType] = make(map[string]bool)
				}
				assertions[interfaceType][concreteType] = true
			}
		}
	}
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

func collectInterfaces(file *ast.File, interfaces map[string]map[string]bool) {
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
			interfaces[typeSpec.Name.Name] = methods
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
