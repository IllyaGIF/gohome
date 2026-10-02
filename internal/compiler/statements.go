package compiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

func sortedNames(sources map[string]string) []string {
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *frontend) block(block *ast.BlockStmt) string {
	var b strings.Builder
	b.WriteString("{\n")
	for _, statement := range block.List {
		b.WriteString(f.stmt(statement) + "\n")
	}
	b.WriteString("}")
	return b.String()
}

func (f *frontend) stmt(node ast.Stmt) string {
	switch s := node.(type) {
	case *ast.BlockStmt:
		return f.block(s)
	case *ast.EmptyStmt:
		return ";"
	case *ast.ExprStmt:
		return f.expr(s.X) + ";"
	case *ast.ReturnStmt:
		if len(s.Results) == 0 {
			return "return;"
		}
		if len(s.Results) > 1 {
			f.fail(s, "multiple return values are unsupported")
			return "return;"
		}
		return "return " + f.expr(s.Results[0]) + ";"
	case *ast.AssignStmt:
		return f.assignment(s)
	case *ast.IncDecStmt:
		if _, ok := s.X.(*ast.Ident); !ok {
			f.fail(s, "only variable increment/decrement is supported")
		}
		return f.expr(s.X) + s.Tok.String() + ";"
	case *ast.DeclStmt:
		decl, ok := s.Decl.(*ast.GenDecl)
		if !ok {
			f.fail(s, "unsupported local declaration")
			return ";"
		}
		var b strings.Builder
		for _, spec := range decl.Specs {
			switch v := spec.(type) {
			case *ast.ValueSpec:
				if decl.Tok == token.CONST {
					continue
				}
				if len(v.Values) > 0 && len(v.Values) != len(v.Names) {
					f.fail(s, "multiple-return declarations are unsupported")
					continue
				}
				var temps []string
				for i, value := range v.Values {
					temp := f.fresh("initial")
					t := f.info.Defs[v.Names[i]].Type()
					fmt.Fprintf(&b, "%s %s = %s;\n", f.ctype(t), temp, f.expr(value))
					temps = append(temps, temp)
				}
				for i, ident := range v.Names {
					if ident.Name == "_" {
						continue
					}
					obj := f.info.Defs[ident]
					initial := f.zero(obj.Type())
					if len(temps) > i {
						initial = temps[i]
					}
					fmt.Fprintf(&b, "%s %s = %s;\n", f.ctype(obj.Type()), f.name(obj), initial)
				}
			case *ast.TypeSpec:
				f.ctype(f.info.Defs[v.Name].Type())
			default:
				f.fail(s, "unsupported local declaration")
			}
		}
		return b.String()
	case *ast.IfStmt:
		init := ""
		if s.Init != nil {
			init = f.stmt(s.Init)
		}
		body := "{ " + init + " if (" + f.expr(s.Cond) + ") " + f.block(s.Body)
		if s.Else != nil {
			body += " else " + f.stmt(s.Else)
		}
		return body + " }"
	case *ast.ForStmt:
		init, cond, post := "", "true", ""
		if s.Init != nil {
			init = f.stmt(s.Init)
		}
		if s.Cond != nil {
			cond = f.expr(s.Cond)
		}
		if s.Post != nil {
			post = "({ " + f.stmt(s.Post) + " })"
		}
		return "{ " + init + " for (; " + cond + "; " + post + ") " + f.block(s.Body) + " }"
	case *ast.BranchStmt:
		if s.Label != nil || (s.Tok != token.BREAK && s.Tok != token.CONTINUE) {
			f.fail(s, "labels, goto and fallthrough are unsupported")
			return ";"
		}
		return s.Tok.String() + ";"
	case *ast.RangeStmt:
		return f.rangeStmt(s)
	case *ast.SwitchStmt:
		return f.switchStmt(s)
	default:
		f.fail(node, fmt.Sprintf("unsupported Go statement %T", node))
		return ";"
	}
}

func (f *frontend) assignment(s *ast.AssignStmt) string {
	if len(s.Lhs) != len(s.Rhs) {
		f.fail(s, "multiple-return assignments are unsupported")
		return ";"
	}
	var b strings.Builder
	var temps []string
	for _, rhs := range s.Rhs {
		t := f.info.TypeOf(rhs)
		value := f.expr(rhs)
		if s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN {
			value, t = f.shiftCount(rhs)
		}
		n := f.fresh("assign")
		fmt.Fprintf(&b, "%s %s = %s;\n", f.ctype(t), n, value)
		if s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN {
			if base := basic(t); base != nil && base.Info()&types.IsUnsigned == 0 {
				fmt.Fprintf(&b, "if (%s < 0) gh_panic(@\"negative shift count\");\n", n)
			}
		}
		temps = append(temps, n)
	}
	for i, lhs := range s.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			f.fail(lhs, "assignment target must be a variable")
			continue
		}
		if id.Name == "_" {
			continue
		}
		obj := f.info.Uses[id]
		decl := ""
		if defined := f.info.Defs[id]; defined != nil {
			obj = defined
			decl = f.ctype(obj.Type()) + " "
		}
		if _, ok := obj.(*types.Var); !ok {
			f.fail(lhs, "assignment target must be a variable")
			continue
		}
		name := f.name(obj)
		if s.Tok == token.DEFINE || s.Tok == token.ASSIGN {
			fmt.Fprintf(&b, "%s%s = %s;\n", decl, name, temps[i])
			continue
		}
		value := temps[i]
		op := strings.TrimSuffix(s.Tok.String(), "=")
		if base := basic(obj.Type()); base != nil && base.Info()&types.IsString != 0 {
			value = "gh_concat(" + name + ", " + value + ")"
		} else if s.Tok == token.AND_NOT_ASSIGN {
			value = name + " & ~" + value
		} else if s.Tok == token.QUO_ASSIGN || s.Tok == token.REM_ASSIGN || s.Tok == token.SHL_ASSIGN || s.Tok == token.SHR_ASSIGN {
			operator := map[token.Token]token.Token{token.QUO_ASSIGN: token.QUO, token.REM_ASSIGN: token.REM, token.SHL_ASSIGN: token.SHL, token.SHR_ASSIGN: token.SHR}[s.Tok]
			if base := basic(obj.Type()); base != nil && base.Info()&types.IsInteger != 0 {
				if operator == token.QUO || operator == token.REM {
					fun := "gh_div"
					if operator == token.REM {
						fun = "gh_mod"
					}
					if base.Info()&types.IsUnsigned != 0 {
						fun = strings.Replace(fun, "gh_", "gh_u", 1)
					}
					value = fun + "(" + name + ", " + value + ")"
				} else {
					width := (&types.StdSizes{WordSize: 4, MaxAlign: 4}).Sizeof(obj.Type()) * 8
					right, sign := 0, 1
					if operator == token.SHR {
						right = 1
					}
					if base.Info()&types.IsUnsigned != 0 {
						sign = 0
					}
					value = fmt.Sprintf("gh_shift(%s, %s, %d, %d, %d)", name, value, width, right, sign)
				}
			} else {
				value = name + " " + op + " " + value
			}
		} else {
			value = name + " " + op + " " + value
		}
		fmt.Fprintf(&b, "%s = (%s)(%s);\n", name, f.ctype(obj.Type()), value)
	}
	return b.String()
}

// rangeStmt lowers "for k, v := range text" over a string. The key is the byte
// index of the rune and the value is the rune itself, like the standard
// library, so multibyte text advances by whole runes.
func (f *frontend) rangeStmt(s *ast.RangeStmt) string {
	if s.Tok != token.DEFINE && s.Tok != token.ASSIGN && s.Tok != token.ILLEGAL {
		f.fail(s, "range only supports key and value assignment")
		return ";"
	}
	if s.X == nil {
		f.fail(s, "range requires a value")
		return ";"
	}
	if b := basic(f.info.TypeOf(s.X)); b == nil || b.Info()&types.IsString == 0 {
		f.fail(s, "range only supports strings")
		return ";"
	}
	text, index, next, decoded := f.fresh("text"), f.fresh("at"), f.fresh("next"), f.fresh("char")
	key, keyDeclares := f.rangeTarget(s.Key)
	value, valueDeclares := f.rangeTarget(s.Value)
	var b strings.Builder
	fmt.Fprintf(&b, "{ NSString * %s = %s;\n", text, f.expr(s.X))
	if value == "" {
		fmt.Fprintf(&b, "for (int32_t %s = 0, %s = 0; %s < gh_len(%s); %s = %s) {\n", index, next, index, text, index, next)
		fmt.Fprintf(&b, "%s = %s; gh_rune(%s, %s, &%s);\n", next, index, text, index, next)
	} else {
		fmt.Fprintf(&b, "for (int32_t %s = 0, %s = 0, %s = 0; %s < gh_len(%s); %s = %s) {\n", index, next, decoded, index, text, index, next)
		fmt.Fprintf(&b, "%s = %s; %s = gh_rune(%s, %s, &%s);\n", next, index, decoded, text, index, next)
	}
	if key != "" {
		fmt.Fprintf(&b, "%s%s = %s;\n", declare(keyDeclares), key, index)
	}
	if value != "" {
		fmt.Fprintf(&b, "%s%s = %s;\n", declare(valueDeclares), value, decoded)
	}
	for _, statement := range s.Body.List {
		b.WriteString(f.stmt(statement) + "\n")
	}
	b.WriteString("} }")
	return b.String()
}

func declare(needed bool) string {
	if needed {
		return "int32_t "
	}
	return ""
}

// rangeTarget returns the C name for a range key or value, and whether the
// name still has to be declared.
func (f *frontend) rangeTarget(expr ast.Expr) (string, bool) {
	id, ok := expr.(*ast.Ident)
	if !ok || id.Name == "_" {
		return "", false
	}
	if obj := f.info.Defs[id]; obj != nil {
		return f.name(obj), true
	}
	return f.name(f.info.Uses[id]), false
}

func (f *frontend) switchStmt(s *ast.SwitchStmt) string {
	var b strings.Builder
	b.WriteString("{\n")
	if s.Init != nil {
		b.WriteString(f.stmt(s.Init) + "\n")
	}
	name, tagType := "true", types.Type(types.Typ[types.Bool])
	if s.Tag != nil {
		tagType = f.info.TypeOf(s.Tag)
		name = f.fresh("switch")
		fmt.Fprintf(&b, "%s %s = %s;\n", f.ctype(tagType), name, f.expr(s.Tag))
	}
	b.WriteString("switch (0) { default:\n")
	var defaultBody string
	count := 0
	for _, statement := range s.Body.List {
		clause := statement.(*ast.CaseClause)
		var body strings.Builder
		for _, stmt := range clause.Body {
			body.WriteString(f.stmt(stmt) + "\n")
		}
		if clause.List == nil {
			defaultBody = body.String()
			continue
		}
		var conditions []string
		for _, expr := range clause.List {
			value := f.expr(expr)
			if base := basic(tagType); base != nil && base.Info()&types.IsString != 0 {
				conditions = append(conditions, "gh_compare("+name+", "+value+") == 0")
			} else {
				conditions = append(conditions, name+" == ("+value+")")
			}
		}
		if count > 0 {
			b.WriteString("else ")
		}
		b.WriteString("if (" + strings.Join(conditions, " || ") + ") {\n" + body.String() + "}\n")
		count++
	}
	if defaultBody != "" {
		if count > 0 {
			b.WriteString("else ")
		}
		b.WriteString("{\n" + defaultBody + "}\n")
	}
	b.WriteString("}\n}")
	return b.String()
}
