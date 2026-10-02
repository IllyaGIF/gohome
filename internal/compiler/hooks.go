package compiler

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
)

func methodFamily(selector string) string {
	selector = strings.TrimLeft(selector, "_")
	for _, family := range []string{"alloc", "new", "init", "copy", "mutableCopy"} {
		if strings.HasPrefix(selector, family) && (len(selector) == len(family) || selector[len(family)] < 'a' || selector[len(family)] > 'z') {
			return family
		}
	}
	return ""
}

func (f *frontend) abi(t types.Type) (string, string) {
	if isObject(t) {
		return "id", "@"
	}
	if t == nil {
		return "void", "v"
	}
	if b := basic(t); b != nil {
		switch b.Kind() {
		case types.String:
			return "NSString *", "@"
		case types.Bool:
			return "BOOL", "c"
		case types.Int, types.Int32:
			return "int32_t", "i"
		case types.Int8:
			return "int8_t", "c"
		case types.Int16:
			return "int16_t", "s"
		case types.Int64:
			return "int64_t", "q"
		case types.Uint, types.Uint32:
			return "uint32_t", "I"
		case types.Uint8:
			return "uint8_t", "C"
		case types.Uint16:
			return "uint16_t", "S"
		case types.Uint64:
			return "uint64_t", "Q"
		case types.Float32:
			return "float", "f"
		case types.Float64:
			return "double", "d"
		}
	}
	f.ctype(t)
	if f.err == nil {
		f.err = fmt.Errorf("unsupported hook ABI type %s", t)
	}
	return "int32_t", "i"
}

func (f *frontend) hook(e *ast.CallExpr, meta bool) string {
	if f.app {
		f.fail(e, "hooks require a tweak project")
		return "0"
	}
	for _, arg := range e.Args[:2] {
		v := f.info.Types[arg].Value
		if v == nil || v.Kind() != constant.String || constant.StringVal(v) == "" {
			f.fail(arg, "hook class and selector must be nonempty constant strings")
			return "0"
		}
	}
	lit, ok := e.Args[2].(*ast.FuncLit)
	if !ok {
		f.fail(e.Args[2], "hook callback must be a function literal")
		return "0"
	}
	f.checkCapture(lit)
	sig := f.info.TypeOf(lit).(*types.Signature)
	if sig.Params().Len() < 2 || !isObject(sig.Params().At(0).Type()) {
		f.fail(lit, "hook callback starts with (self ios.Object, original func(ios.Object, ...))")
		return "0"
	}
	orig, ok := sig.Params().At(1).Type().(*types.Signature)
	if !ok {
		f.fail(lit, "second callback parameter must be the original function")
		return "0"
	}
	if sig.Variadic() || orig.Variadic() || sig.Results().Len() > 1 || orig.Results().Len() != sig.Results().Len() || orig.Params().Len() != sig.Params().Len()-1 {
		f.fail(lit, "hook and original signatures do not match")
		return "0"
	}
	for i := 0; i < orig.Params().Len(); i++ {
		j := i
		if i > 0 {
			j++
		}
		if !types.Identical(orig.Params().At(i).Type(), sig.Params().At(j).Type()) {
			f.fail(lit, "original argument types must match the hook callback")
			return "0"
		}
	}
	var resultType types.Type
	if sig.Results().Len() == 1 {
		resultType = sig.Results().At(0).Type()
		if sig.Results().At(0).Name() != "" || !types.Identical(resultType, orig.Results().At(0).Type()) {
			f.fail(lit, "hook return types must match and be unnamed")
			return "0"
		}
	}
	selector := constant.StringVal(f.info.Types[e.Args[1]].Value)
	if strings.Count(selector, ":") != sig.Params().Len()-2 {
		f.fail(lit, "selector colon count must match hook arguments")
		return "0"
	}
	f.hooks++
	name, old, sel, wrapper := f.fresh("hook"), f.fresh("old"), f.fresh("sel"), f.fresh("original")
	f.originals[sig.Params().At(1)] = wrapper
	ret, encoding := f.abi(resultType)
	goRet := f.ctype(resultType)
	params := []string{"id " + f.name(sig.Params().At(0)), "SEL gh_command"}
	oldTypes := []string{"id", "SEL"}
	attribute := ""
	if family := methodFamily(selector); family != "" && ret != "void" && (ret == "id" || ret == "NSString *") {
		attribute = " __attribute__((ns_returns_retained))"
		if family == "init" {
			params[0] = "__attribute__((ns_consumed)) " + params[0]
			oldTypes[0] = "__attribute__((ns_consumed)) id"
		}
	}
	var encodings strings.Builder
	for i := 2; i < sig.Params().Len(); i++ {
		v := sig.Params().At(i)
		t, enc := f.abi(v.Type())
		params = append(params, t+" "+f.name(v))
		oldTypes = append(oldTypes, t)
		encodings.WriteString(enc)
	}
	var origParams, origArgs []string
	for i := 0; i < orig.Params().Len(); i++ {
		v := orig.Params().At(i)
		origParams = append(origParams, f.ctype(v.Type())+" "+f.name(v))
		origArgs = append(origArgs, f.name(v))
		if i == 0 {
			origArgs = append(origArgs, sel)
		}
	}
	oldCall := old + "(" + strings.Join(origArgs, ", ") + ")"
	if resultType != nil {
		if b := basic(resultType); b != nil && b.Info()&types.IsString != 0 {
			oldCall = "(" + oldCall + " ?: @\"\")"
		}
		oldCall = "return " + oldCall
	}
	body := f.block(lit.Body)
	fmt.Fprintf(&f.callbacks, "static %s (*%s)(%s)%s;\nstatic SEL %s;\nstatic %s %s(%s) { %s; }\nstatic %s %s(%s)%s %s\n", ret, old, strings.Join(oldTypes, ", "), attribute, sel, goRet, wrapper, strings.Join(origParams, ", "), oldCall, ret, name, strings.Join(params, ", "), attribute, body)
	metaFlag := "false"
	if meta {
		metaFlag = "true"
	}
	return fmt.Sprintf("({ if (!%s) { %s = NSSelectorFromString(%s); gh_hook(%s, %s, (IMP)%s, (IMP *)&%s, %s, %s, %s); } (void)0; })", old, sel, f.expr(e.Args[1]), f.expr(e.Args[0]), f.expr(e.Args[1]), name, old, cbytes(encoding), cbytes(encodings.String()), metaFlag)
}
