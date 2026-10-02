package compiler

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Result struct {
	Source string
	Hooks  int
}

type frontend struct {
	fset      *token.FileSet
	info      *types.Info
	pkg       *types.Package
	names     map[types.Object]string
	next      int
	hooks     int
	callbacks strings.Builder
	initCalls []string
	err       error
	app       bool
	originals map[types.Object]string
}

var targetSizes = &types.StdSizes{WordSize: 4, MaxAlign: 4}

func Compile(sources map[string]string, app bool) (Result, error) {
	fset := token.NewFileSet()
	sdk, err := parser.ParseFile(fset, "ios.go", SDKSource, 0)
	if err != nil {
		return Result{}, err
	}
	sdkPkg, err := new(types.Config).Check(iosPath, fset, []*ast.File{sdk}, nil)
	if err != nil {
		return Result{}, err
	}
	importer := &sdkImporter{fset: fset, packages: map[string]*types.Package{iosPath: sdkPkg}, sdk: sdkPkg}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	files := []*ast.File{}
	for _, name := range sortedNames(sources) {
		file, err := parser.ParseFile(fset, name, sources[name], 0)
		if err != nil {
			return Result{}, err
		}
		if file.Name.Name != "main" {
			return Result{}, fmt.Errorf("%s: package must be main", name)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("no Go source files found")
	}
	config := types.Config{Importer: importer, Sizes: targetSizes}
	pkg, err := config.Check("main", fset, files, info)
	if err != nil {
		return Result{}, explain(err)
	}
	f := &frontend{fset: fset, info: info, pkg: pkg, names: map[types.Object]string{}, originals: map[types.Object]string{}, app: app}
	var globals, prototypes, bodies strings.Builder
	mainFound := false
	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					f.fail(d, "methods on Go types are not supported yet")
					continue
				}
				obj := info.Defs[d.Name]
				sig := obj.Type().(*types.Signature)
				if sig.TypeParams() != nil && sig.TypeParams().Len() > 0 {
					f.fail(d, "generic functions are not supported")
					continue
				}
				if d.Body == nil {
					f.fail(d, "function requires a body")
					continue
				}
				if d.Name.Name == "main" {
					if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
						f.fail(d, "main must have signature func main()")
					}
					mainFound = true
					f.names[obj] = "gh_main"
				} else if d.Name.Name == "init" {
					f.names[obj] = f.fresh("init")
					f.initCalls = append(f.initCalls, f.names[obj]+"();")
				}
				fmt.Fprintf(&prototypes, "static %s;\n", f.signature(sig, f.name(obj), false))
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, ident := range s.Names {
							obj := info.Defs[ident]
							if v, ok := obj.(*types.Var); ok && ident.Name != "_" {
								fmt.Fprintf(&globals, "static %s %s;\n", f.ctype(v.Type()), f.name(v))
							}
						}
					case *ast.TypeSpec:
						f.ctype(info.Defs[s.Name].Type())
					case *ast.ImportSpec:
						if s.Name != nil && (s.Name.Name == "." || s.Name.Name == "_") {
							f.fail(s, "dot and side-effect imports are unsupported")
						}
					}
				}
			default:
				f.fail(d, "unsupported declaration")
			}
		}
	}
	if !mainFound {
		return Result{}, fmt.Errorf("project requires func main()")
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			if d, ok := decl.(*ast.FuncDecl); ok && d.Recv == nil && d.Body != nil {
				sig := info.Defs[d.Name].Type().(*types.Signature)
				fmt.Fprintf(&bodies, "static %s %s\n", f.signature(sig, f.name(info.Defs[d.Name]), false), f.block(d.Body))
			}
		}
	}
	var initializer strings.Builder
	initializer.WriteString("static void gh_init(void) {\n")
	for _, name := range pkg.Scope().Names() {
		if v, ok := pkg.Scope().Lookup(name).(*types.Var); ok {
			fmt.Fprintf(&initializer, "%s = %s;\n", f.name(v), f.zero(v.Type()))
		}
	}
	for _, init := range info.InitOrder {
		if len(init.Lhs) != 1 {
			f.fail(init.Rhs, "multiple-return initializers are unsupported")
			continue
		}
		if init.Lhs[0].Name() == "_" {
			fmt.Fprintf(&initializer, "%s;\n", f.expr(init.Rhs))
		} else {
			fmt.Fprintf(&initializer, "%s = %s;\n", f.name(init.Lhs[0]), f.expr(init.Rhs))
		}
	}
	for _, call := range f.initCalls {
		initializer.WriteString(call + "\n")
	}
	initializer.WriteString("}\n")
	if f.err != nil {
		return Result{}, f.err
	}
	flag := "0"
	if app {
		flag = "1"
	}
	return Result{Source: "#define GH_APP " + flag + "\n" + RuntimeSource + "\n" + globals.String() + prototypes.String() + f.callbacks.String() + bodies.String() + initializer.String(), Hooks: f.hooks}, nil
}

// explain points at gohome doc whenever the type checker reports a name that
// the backend does not provide, which is the usual reason a program written
// for a full Go toolchain is rejected here.
func explain(err error) error {
	text := err.Error()
	if strings.Contains(text, "undefined:") || strings.Contains(text, "is not a function") {
		return fmt.Errorf("%s; run gohome doc to see what this backend supports", text)
	}
	return err
}

func (f *frontend) fail(node ast.Node, message string) {
	if f.err == nil {
		f.err = fmt.Errorf("%s: %s", f.fset.Position(node.Pos()), message)
	}
}

func (f *frontend) fresh(kind string) string { f.next++; return fmt.Sprintf("gh_%s_%d", kind, f.next) }

func (f *frontend) name(obj types.Object) string {
	if obj == nil {
		return "gh_invalid"
	}
	if name, ok := f.names[obj]; ok {
		return name
	}
	name := f.fresh("v")
	if _, ok := obj.(*types.Func); ok {
		name = "gh_fn_" + obj.Name()
	}
	f.names[obj] = name
	return name
}

func isObject(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "gohome/ios" && n.Obj().Name() == "Object"
}

func basic(t types.Type) *types.Basic {
	if t == nil {
		return nil
	}
	b, _ := t.Underlying().(*types.Basic)
	return b
}

func (f *frontend) ctype(t types.Type) string {
	if t == nil {
		return "void"
	}
	if isObject(t) {
		return "id"
	}
	if b := basic(t); b != nil {
		switch b.Kind() {
		case types.Bool, types.UntypedBool:
			return "bool"
		case types.Int, types.Int32, types.UntypedInt, types.UntypedRune:
			return "int32_t"
		case types.Int8:
			return "int8_t"
		case types.Int16:
			return "int16_t"
		case types.Int64:
			return "int64_t"
		case types.Uint, types.Uint32:
			return "uint32_t"
		case types.Uint8:
			return "uint8_t"
		case types.Uint16:
			return "uint16_t"
		case types.Uint64:
			return "uint64_t"
		case types.Float32:
			return "float"
		case types.Float64, types.UntypedFloat:
			return "double"
		case types.String, types.UntypedString:
			return "NSString *"
		}
	}
	if tuple, ok := t.(*types.Tuple); ok && tuple.Len() == 0 {
		return "void"
	}
	if f.err == nil {
		f.err = fmt.Errorf("type %s is unsupported by the iOS 6 backend", t)
	}
	return "int32_t"
}

func (f *frontend) zero(t types.Type) string {
	if isObject(t) {
		return "nil"
	}
	if b := basic(t); b != nil && b.Info()&types.IsString != 0 {
		return "@\"\""
	}
	return "0"
}

func (f *frontend) signature(sig *types.Signature, name string, native bool) string {
	if sig.Variadic() {
		if f.err == nil {
			f.err = fmt.Errorf("variadic Go functions are unsupported")
		}
	}
	if sig.Results().Len() > 1 {
		if f.err == nil {
			f.err = fmt.Errorf("multiple return values are unsupported")
		}
	}
	result := "void"
	if sig.Results().Len() == 1 {
		if sig.Results().At(0).Name() != "" {
			if f.err == nil {
				f.err = fmt.Errorf("named results are unsupported")
			}
		}
		result = f.ctype(sig.Results().At(0).Type())
	}
	var params []string
	for i := 0; i < sig.Params().Len(); i++ {
		v := sig.Params().At(i)
		params = append(params, f.ctype(v.Type())+" "+f.name(v))
	}
	if len(params) == 0 {
		params = []string{"void"}
	}
	return result + " " + name + "(" + strings.Join(params, ", ") + ")"
}

func cbytes(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range []byte(text) {
		fmt.Fprintf(&b, "\\%03o", c)
	}
	b.WriteByte('"')
	return b.String()
}

func (f *frontend) value(v constant.Value, t types.Type) string {
	switch v.Kind() {
	case constant.String:
		s := constant.StringVal(v)
		if !utf8.ValidString(s) {
			if f.err == nil {
				f.err = fmt.Errorf("strings must contain valid UTF-8 in this backend")
			}
		}
		return fmt.Sprintf("gh_string(%s, %d)", cbytes(s), len(s))
	case constant.Bool:
		return strconv.FormatBool(constant.BoolVal(v))
	case constant.Int:
		text := v.ExactString()
		if text == "-9223372036854775808" {
			text = "(-9223372036854775807LL - 1LL)"
		} else if b := basic(t); b != nil && b.Info()&types.IsUnsigned != 0 {
			text += "ULL"
		} else {
			text += "LL"
		}
		return "((" + f.ctype(t) + ")(" + text + "))"
	case constant.Float:
		v, _ := constant.Float64Val(v)
		return "((" + f.ctype(t) + ")(" + strconv.FormatFloat(v, 'g', -1, 64) + "))"
	default:
		if f.err == nil {
			f.err = fmt.Errorf("unsupported constant %s", v)
		}
		return "0"
	}
}

func (f *frontend) expr(node ast.Expr) string {
	if node == nil {
		return ""
	}
	if tv, ok := f.info.Types[node]; ok && tv.Value != nil {
		return f.value(tv.Value, tv.Type)
	}
	switch e := node.(type) {
	case *ast.Ident:
		if e.Name == "nil" {
			return "nil"
		}
		obj := f.info.Uses[e]
		if c, ok := obj.(*types.Const); ok {
			return f.value(c.Val(), c.Type())
		}
		return f.name(obj)
	case *ast.ParenExpr:
		return "(" + f.expr(e.X) + ")"
	case *ast.UnaryExpr:
		if e.Op != token.NOT && e.Op != token.ADD && e.Op != token.SUB && e.Op != token.XOR {
			f.fail(e, "pointers and channels are unsupported")
			return "0"
		}
		op := e.Op.String()
		if e.Op == token.XOR {
			op = "~"
		}
		return "((" + f.ctype(f.info.TypeOf(e)) + ")(" + op + "(" + f.expr(e.X) + ")))"
	case *ast.BinaryExpr:
		return f.binary(e)
	case *ast.CallExpr:
		return f.call(e)
	case *ast.IndexExpr:
		if b := basic(f.info.TypeOf(e.X)); b != nil && b.Info()&types.IsString != 0 {
			return f.invoke("gh_byte", []ast.Expr{e.X, e.Index}, nil)
		}
		f.fail(e, "only strings can be indexed")
		return "0"
	case *ast.SelectorExpr:
		if value, ok := nativeValue(f, e); ok {
			return f.value(constant.MakeString(value), types.Typ[types.String])
		}
		f.fail(e, "package variables are unsupported")
		return "@\"\""
	case *ast.CompositeLit:
		if isObject(f.info.TypeOf(e)) && len(e.Elts) == 0 {
			return "nil"
		}
		f.fail(e, "unsupported composite literal; only empty ios.Object{} is supported")
		return "0"
	default:
		f.fail(node, fmt.Sprintf("unsupported Go expression %T", node))
		return "0"
	}
}

func (f *frontend) binary(e *ast.BinaryExpr) string {
	left, right := f.expr(e.X), f.expr(e.Y)
	op := e.Op.String()
	if e.Op == token.LAND || e.Op == token.LOR {
		return "(" + left + " " + op + " " + right + ")"
	}
	lt, rt := f.info.TypeOf(e.X), f.info.TypeOf(e.Y)
	a, b := f.fresh("lhs"), f.fresh("rhs")
	value := a + " " + op + " " + b
	if base := basic(lt); base != nil && base.Info()&types.IsString != 0 {
		if e.Op == token.ADD {
			value = "gh_concat(" + a + ", " + b + ")"
		} else {
			value = "gh_compare(" + a + ", " + b + ") " + op + " 0"
		}
	} else if base != nil && base.Info()&types.IsInteger != 0 {
		if e.Op == token.QUO || e.Op == token.REM {
			fun := "gh_div"
			if e.Op == token.REM {
				fun = "gh_mod"
			}
			if base.Info()&types.IsUnsigned != 0 {
				fun = strings.Replace(fun, "gh_", "gh_u", 1)
			}
			value = "(" + f.ctype(lt) + ")" + fun + "(" + a + ", " + b + ")"
		} else if e.Op == token.SHL || e.Op == token.SHR {
			right, rt = f.shiftCount(e.Y)
			negative := ""
			if rhs := basic(rt); rhs != nil && rhs.Info()&types.IsUnsigned == 0 {
				negative = "if (" + b + " < 0) gh_panic(@\"negative shift count\"); "
			}
			width := (&types.StdSizes{WordSize: 4, MaxAlign: 4}).Sizeof(lt) * 8
			direction, signed := 0, 1
			if e.Op == token.SHR {
				direction = 1
			}
			if base.Info()&types.IsUnsigned != 0 {
				signed = 0
			}
			value = fmt.Sprintf("(%s)gh_shift(%s, %s, %d, %d, %d)", f.ctype(lt), a, b, width, direction, signed)
			return "({ " + f.ctype(lt) + " " + a + " = " + left + "; " + f.ctype(rt) + " " + b + " = " + right + "; " + negative + value + "; })"
		} else if e.Op == token.AND_NOT {
			value = a + " & ~" + b
		}
	}
	return "({ " + f.ctype(lt) + " " + a + " = " + left + "; " + f.ctype(rt) + " " + b + " = " + right + "; (" + f.ctype(f.info.TypeOf(e)) + ")(" + value + "); })"
}

func (f *frontend) shiftCount(node ast.Expr) (string, types.Type) {
	if value := f.info.Types[node].Value; value != nil {
		if constant.Compare(value, token.GEQ, constant.MakeInt64(64)) {
			return "64ULL", types.Typ[types.Uint64]
		}
		return value.ExactString() + "ULL", types.Typ[types.Uint64]
	}
	return f.expr(node), f.info.TypeOf(node)
}

func (f *frontend) invoke(name string, args []ast.Expr, result types.Type) string {
	var prep, values []string
	for _, arg := range args {
		n := f.fresh("arg")
		prep = append(prep, f.ctype(f.info.TypeOf(arg))+" "+n+" = "+f.expr(arg)+";")
		values = append(values, n)
	}
	call := name + "(" + strings.Join(values, ", ") + ")"
	if len(prep) == 0 {
		return call
	}
	return "({ " + strings.Join(prep, " ") + " " + call + "; })"
}

func (f *frontend) call(e *ast.CallExpr) string {
	if e.Ellipsis.IsValid() {
		f.fail(e, "variadic expansion is unsupported")
		return "0"
	}
	if tv := f.info.Types[e.Fun]; tv.IsType() {
		t := f.info.TypeOf(e)
		if len(e.Args) != 1 {
			f.fail(e, "invalid conversion")
			return "0"
		}
		if isObject(t) || isObject(f.info.TypeOf(e.Args[0])) {
			f.fail(e, "Object pointer conversions are unsupported")
			return "0"
		}
		if b := basic(t); b != nil && b.Info()&types.IsString != 0 {
			if source := basic(f.info.TypeOf(e.Args[0])); source == nil || source.Info()&types.IsString == 0 {
				f.fail(e, "integer-to-string conversion is unsupported")
			}
			return f.expr(e.Args[0])
		}
		return "((" + f.ctype(t) + ")(" + f.expr(e.Args[0]) + "))"
	}
	if id, ok := e.Fun.(*ast.Ident); ok {
		if builtin, ok := f.info.Uses[id].(*types.Builtin); ok {
			switch builtin.Name() {
			case "len":
				if b := basic(f.info.TypeOf(e.Args[0])); b == nil || b.Info()&types.IsString == 0 {
					f.fail(e, "len only supports strings")
				}
				return f.invoke("gh_len", e.Args, f.info.TypeOf(e))
			case "panic":
				if b := basic(f.info.TypeOf(e.Args[0])); b == nil || b.Info()&types.IsString == 0 {
					f.fail(e, "panic requires a string")
				}
				return f.invoke("gh_panic", e.Args, nil)
			default:
				f.fail(e, "unsupported builtin "+builtin.Name())
				return "0"
			}
		}
		if name, ok := f.originals[f.info.Uses[id]]; ok {
			return f.invoke(name, e.Args, f.info.TypeOf(e))
		}
		if _, ok := f.info.Uses[id].(*types.Func); !ok {
			f.fail(e, "function values are only supported as hook original callbacks")
			return "0"
		}
		return f.invoke(f.name(f.info.Uses[id]), e.Args, f.info.TypeOf(e))
	}
	if selector, ok := e.Fun.(*ast.SelectorExpr); ok {
		if selection := f.info.Selections[selector]; selection != nil {
			if spec, ok := nativeMethod(selection); ok {
				return f.nativeCall(e, spec, selector.X)
			}
			return f.send(e, selector)
		}
		if spec, ok := f.nativeFunction(selector); ok {
			return f.nativeCall(e, spec, nil)
		}
		obj := f.info.Uses[selector.Sel]
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != iosPath {
			f.fail(e, "unsupported external function")
			return "0"
		}
		switch selector.Sel.Name {
		case "Text":
			t := f.info.TypeOf(e.Args[0])
			name := ""
			if isObject(t) {
				name = "gh_text_object"
			} else if base := basic(t); base != nil {
				switch {
				case base.Info()&types.IsString != 0:
					return f.expr(e.Args[0])
				case base.Info()&types.IsBoolean != 0:
					name = "gh_text_bool"
				case base.Info()&types.IsUnsigned != 0:
					name = "gh_text_uint"
				case base.Info()&types.IsInteger != 0:
					name = "gh_text_int"
				case base.Info()&types.IsFloat != 0:
					name = "gh_text_float"
				}
			}
			if name == "" {
				f.fail(e, "ios.Text supports strings, numbers, bool and Object")
				return "0"
			}
			return f.invoke(name, e.Args, f.info.TypeOf(e))
		case "Hook", "HookClass":
			return f.hook(e, selector.Sel.Name == "HookClass")
		case "Button":
			if !f.app {
				f.fail(e, "ios.Button requires an app project")
			}
			callback := f.callback(e.Args[5])
			return f.invokeExtra("gh_button", e.Args[:5], callback)
		case "Tab":
			if !f.app {
				f.fail(e, "ios.Tab requires an app project")
			}
			return f.invokeExtra("gh_tab", e.Args[:2], f.callback(e.Args[2]))
		default:
			mapping := map[string]string{"Log": "gh_log", "Alert": "gh_alert", "Class": "gh_class", "New": "gh_new", "Label": "gh_label", "SetText": "gh_set_text", "SetFrame": "gh_set_frame", "AddSubview": "gh_add_subview", "RootView": "gh_root_view"}
			name, ok := mapping[selector.Sel.Name]
			if !ok {
				f.fail(e, "unsupported ios API "+selector.Sel.Name)
				return "0"
			}
			if !f.app && (selector.Sel.Name == "Label" || selector.Sel.Name == "RootView") {
				f.fail(e, selector.Sel.Name+" requires an app project")
			}
			return f.invoke(name, e.Args, f.info.TypeOf(e))
		}
	}
	f.fail(e, "unsupported function call")
	return "0"
}

func (f *frontend) invokeExtra(name string, args []ast.Expr, extra string) string {
	var prep, values []string
	for _, arg := range args {
		n := f.fresh("arg")
		prep = append(prep, f.ctype(f.info.TypeOf(arg))+" "+n+" = "+f.expr(arg)+";")
		values = append(values, n)
	}
	values = append(values, extra)
	return "({ " + strings.Join(prep, " ") + " " + name + "(" + strings.Join(values, ", ") + "); })"
}

func (f *frontend) callback(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		if fn, ok := f.info.Uses[id].(*types.Func); ok {
			return f.name(fn)
		}
	}
	lit, ok := expr.(*ast.FuncLit)
	if !ok {
		f.fail(expr, "callback must be a function declaration or literal")
		return "NULL"
	}
	f.checkCapture(lit)
	name := f.fresh("callback")
	sig := f.info.TypeOf(lit).(*types.Signature)
	body := f.block(lit.Body)
	f.callbacks.WriteString("static " + f.signature(sig, name, false) + " " + body + "\n")
	return name
}

func (f *frontend) checkCapture(lit *ast.FuncLit) {
	ast.Inspect(lit, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := f.info.Uses[id].(*types.Var); ok && v.Pkg() == f.pkg && v.Parent() != f.pkg.Scope() && (v.Pos() < lit.Pos() || v.Pos() > lit.End()) {
				f.fail(id, "callbacks cannot capture local variables; use package-level state")
			}
		}
		return true
	})
}

func (f *frontend) send(e *ast.CallExpr, selector *ast.SelectorExpr) string {
	method := selector.Sel.Name
	if method == "Retain" || method == "Release" {
		f.fail(e, "objects use automatic ARC ownership; Retain/Release are unsupported")
		return "0"
	}
	if len(e.Args) < 1 {
		f.fail(e, "message needs a selector")
		return "0"
	}
	if value := f.info.Types[e.Args[0]].Value; value == nil || value.Kind() != constant.String {
		f.fail(e.Args[0], "message selector must be a constant string")
		return "0"
	} else if strings.Count(constant.StringVal(value), ":") != len(e.Args)-1 {
		f.fail(e, "selector colon count must match argument count")
	} else {
		name := constant.StringVal(value)
		if methodFamily(name) != "" {
			f.fail(e, "ownership-family messages require ios.New instead of Object.Send")
			return "0"
		}
	}
	result := map[string]string{"Send": "id", "SendVoid": "void", "SendString": "NSString *", "SendInt": "int32_t", "SendBool": "BOOL", "SendFloat": "float", "SendDouble": "double"}[method]
	if result == "" {
		f.fail(e, "unsupported Object method "+method)
		return "0"
	}
	receiver, sel := f.fresh("receiver"), f.fresh("selector")
	prep := []string{"id " + receiver + " = " + f.expr(selector.X) + ";", "SEL " + sel + " = NSSelectorFromString(" + f.expr(e.Args[0]) + ");"}
	typesList, values := []string{"id", "SEL"}, []string{receiver, sel}
	var argumentEncoding strings.Builder
	for _, arg := range e.Args[1:] {
		n := f.fresh("arg")
		t := f.ctype(f.info.TypeOf(arg))
		if b := basic(f.info.TypeOf(arg)); !isObject(f.info.TypeOf(arg)) && b != nil && b.Info()&types.IsBoolean != 0 {
			t = "BOOL"
		}
		prep = append(prep, t+" "+n+" = "+f.expr(arg)+";")
		typesList = append(typesList, t)
		values = append(values, n)
		argType := f.info.TypeOf(arg)
		if base := basic(argType); base != nil && base.Info()&types.IsUntyped != 0 {
			argType = types.Default(argType)
		}
		_, encoding := f.abi(argType)
		argumentEncoding.WriteString(encoding)
	}
	call := "((" + result + " (*)(" + strings.Join(typesList, ", ") + "))objc_msgSend)(" + strings.Join(values, ", ") + ")"
	if method == "SendString" {
		call = "(" + call + " ?: @\"\")"
	}
	var resultType types.Type
	if method != "SendVoid" {
		resultType = f.info.TypeOf(e)
	}
	_, encoding := f.abi(resultType)
	valid := "gh_message(" + receiver + ", " + sel + ", " + cbytes(encoding) + ", " + cbytes(argumentEncoding.String()) + ")"
	if method == "SendVoid" {
		call = "if (" + valid + ") " + call + "; (void)0"
	} else {
		call = "(" + valid + " ? " + call + " : " + f.zero(resultType) + ")"
	}
	return "({ " + strings.Join(prep, " ") + " " + call + "; })"
}
