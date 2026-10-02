package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// Packages lists every import path a gohome program may use.
func Packages() []string { return importable() }

// Doc renders the exported declarations of an embedded stub package the way
// godoc would, so that the supported subset can be read without the source
// tree. Constants are printed with the value they resolve to.
func Doc(path string) (string, error) {
	source, ok := stdlibSources[path]
	if !ok {
		if path != iosPath {
			return "", fmt.Errorf("unknown package %q; run gohome doc to list packages", path)
		}
		source = SDKSource
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path+".go", source, parser.ParseComments)
	if err != nil {
		return "", err
	}
	scope := types.NewScope(nil, token.NoPos, token.NoPos, "")
	if pkg, err := new(types.Config).Check(path, fset, []*ast.File{file}, nil); err == nil {
		scope = pkg.Scope()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "package %s // gohome backend, bodies live in runtime/runtime.m\n", file.Name.Name)
	if file.Doc != nil {
		fmt.Fprintf(&b, "\n%s\n", comment(file.Doc.Text()))
	}
	var funcs, values []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() || (d.Body != nil && !stubBody(d.Body)) {
				continue
			}
			key := path + "." + d.Name.Name
			if d.Recv != nil {
				if len(d.Recv.List) != 1 {
					continue
				}
				key = path + "." + strings.TrimPrefix(render(d.Recv.List[0].Type), "*") + "." + d.Name.Name
				funcs = append(funcs, "func ("+render(d.Recv.List[0].Type)+") "+d.Name.Name+signature(d.Type)+" // "+nativeFor(key))
				continue
			}
			funcs = append(funcs, "func "+d.Name.Name+signature(d.Type)+" // "+nativeFor(key))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch v := spec.(type) {
				case *ast.TypeSpec:
					if v.Name.IsExported() {
						values = append(values, "type "+v.Name.Name+" "+render(v.Type))
					}
				case *ast.ValueSpec:
					keyword := "const "
					if d.Tok != token.CONST {
						keyword = "var "
					}
					for _, name := range v.Names {
						if !name.IsExported() {
							continue
						}
						values = append(values, keyword+declName(name.Name, scope, nativeValues[path+"."+name.Name]))
					}
				}
			}
		}
	}
	b.WriteString("\n")
	writeAll(&b, "const", func() []string { return filter(values, "const ") })
	writeAll(&b, "var", func() []string { return filter(values, "var ") })
	writeAll(&b, "type", func() []string { return filter(values, "type ") })
	b.WriteString("\n")
	sort.Strings(funcs)
	for _, text := range funcs {
		b.WriteString(text + "\n")
	}
	return b.String(), nil
}

func writeAll(b *strings.Builder, keyword string, all func() []string) {
	lines := all()
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "%s %d\n", keyword, len(lines))
	for _, line := range lines {
		fmt.Fprintf(b, "\t%s\n", line)
	}
	fmt.Fprint(b, "\n")
}

func filter(values []string, prefix string) []string {
	var out []string
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			out = append(out, strings.TrimSpace(value[len(prefix):]))
		}
	}
	return out
}

func declName(name string, scope *types.Scope, value string) string {
	switch resolved := scope.Lookup(name).(type) {
	case *types.Const:
		text := resolved.Val().ExactString()
		if b := basic(resolved.Type()); b != nil && b.Info()&types.IsString != 0 {
			text = resolved.Val().String()
		}
		return name + " " + text
	case *types.Var:
		return name + " " + types.TypeString(resolved.Type(), nil)
	}
	if value != "" {
		return name + fmt.Sprintf(" // resolved at build time, for example %q", value)
	}
	return name
}

func comment(text string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		out = append(out, strings.TrimPrefix(strings.TrimPrefix(line, "//"), " "))
	}
	return strings.Join(out, "\n")
}

// stubBody reports whether a declaration is one of the panic placeholders the
// embedded stubs use, which the backend replaces with native code.
func stubBody(body *ast.BlockStmt) bool {
	if len(body.List) != 1 {
		return false
	}
	statement, ok := body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "panic"
}

func render(node ast.Node) string {
	var b strings.Builder
	printer.Fprint(&b, token.NewFileSet(), node)
	return b.String()
}

func signature(node *ast.FuncType) string {
	return strings.TrimPrefix(render(node), "func")
}

func nativeFor(key string) string {
	if spec, ok := nativeFuncs[key]; ok {
		return "backed by " + spec.c
	}
	return "implemented by the ios SDK"
}
