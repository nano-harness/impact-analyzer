package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// InterfaceMethodNames parses Go source and returns the explicitly declared
// method names of the named interface (embedded interfaces are not expanded).
// Returns nil when the interface is not found or declares no methods.
func InterfaceMethodNames(content []byte, ifaceName string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != ifaceName {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				return nil, nil
			}
			var names []string
			for _, method := range iface.Methods.List {
				for _, name := range method.Names {
					names = append(names, name.Name)
				}
			}
			return names, nil
		}
	}
	return nil, nil
}
