package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"sort"
	"strings"
)

const iosPath = "gohome/ios"

// sdkImporter resolves the packages a gohome program is allowed to import. The
// gohome/ios SDK and every stdlib stub are parsed from embedded sources and
// type checked on demand, so no Go toolchain is involved.
type sdkImporter struct {
	fset     *token.FileSet
	packages map[string]*types.Package
	sdk      *types.Package
}

func (i *sdkImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := i.packages[path]; ok {
		return pkg, nil
	}
	if path == iosPath {
		i.packages[path] = i.sdk
		return i.sdk, nil
	}
	source, ok := stdlibSources[path]
	if !ok {
		return nil, fmt.Errorf("import %q is unsupported by the iOS 6 backend; available: %s", path, strings.Join(importable(), ", "))
	}
	file, err := parser.ParseFile(i.fset, path+".go", source, 0)
	if err != nil {
		return nil, err
	}
	placeholder := types.NewPackage(path, file.Name.Name)
	i.packages[path] = placeholder
	pkg, err := (&types.Config{Importer: i, Sizes: targetSizes}).Check(path, i.fset, []*ast.File{file}, nil)
	if err != nil {
		delete(i.packages, path)
		return nil, err
	}
	if pkg != placeholder {
		i.packages[path] = pkg
	}
	return i.packages[path], nil
}

func importable() []string {
	paths := make([]string, 0, len(stdlibSources)+1)
	paths = append(paths, iosPath)
	for path := range stdlibSources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// nativeFunc describes how a Go symbol of an embedded stub package is lowered.
// fixed is the number of leading parameters that are passed positionally, and
// variadic means the remaining ...any parameters are boxed into an NSArray.
type nativeFunc struct {
	c        string
	fixed    int
	variadic bool
}

// nativeFuncs maps "import/path.Function" and "import/path.Type.Method" keys
// onto their Objective-C implementation in runtime/runtime.m.
var nativeFuncs = map[string]nativeFunc{
	"fmt.Print":    {c: "gh_fmt_print", variadic: true},
	"fmt.Println":  {c: "gh_fmt_println", variadic: true},
	"fmt.Printf":   {c: "gh_fmt_printf", fixed: 1, variadic: true},
	"fmt.Sprint":   {c: "gh_fmt_sprint", variadic: true},
	"fmt.Sprintln": {c: "gh_fmt_sprintln", variadic: true},
	"fmt.Sprintf":  {c: "gh_fmt_sprintf", fixed: 1, variadic: true},

	"math.Abs":          {c: "gh_math_abs", fixed: 1},
	"math.Acos":         {c: "gh_math_acos", fixed: 1},
	"math.Acosh":        {c: "gh_math_acosh", fixed: 1},
	"math.Asin":         {c: "gh_math_asin", fixed: 1},
	"math.Asinh":        {c: "gh_math_asinh", fixed: 1},
	"math.Atan":         {c: "gh_math_atan", fixed: 1},
	"math.Atan2":        {c: "gh_math_atan2", fixed: 2},
	"math.Atanh":        {c: "gh_math_atanh", fixed: 1},
	"math.Cbrt":         {c: "gh_math_cbrt", fixed: 1},
	"math.Ceil":         {c: "gh_math_ceil", fixed: 1},
	"math.Copysign":     {c: "gh_math_copysign", fixed: 2},
	"math.Cos":          {c: "gh_math_cos", fixed: 1},
	"math.Cosh":         {c: "gh_math_cosh", fixed: 1},
	"math.Exp":          {c: "gh_math_exp", fixed: 1},
	"math.Floor":        {c: "gh_math_floor", fixed: 1},
	"math.Hypot":        {c: "gh_math_hypot", fixed: 2},
	"math.Inf":          {c: "gh_math_inf", fixed: 1},
	"math.IsInf":        {c: "gh_math_is_inf", fixed: 2},
	"math.IsNaN":        {c: "gh_math_is_nan", fixed: 1},
	"math.Log":          {c: "gh_math_log", fixed: 1},
	"math.Log10":        {c: "gh_math_log10", fixed: 1},
	"math.Log2":         {c: "gh_math_log2", fixed: 1},
	"math.Max":          {c: "gh_math_max", fixed: 2},
	"math.Min":          {c: "gh_math_min", fixed: 2},
	"math.Mod":          {c: "gh_math_mod", fixed: 2},
	"math.NaN":          {c: "gh_math_nan"},
	"math.Pow":          {c: "gh_math_pow", fixed: 2},
	"math.Round":        {c: "gh_math_round", fixed: 1},
	"math.Signbit":      {c: "gh_math_signbit", fixed: 1},
	"math.Sin":          {c: "gh_math_sin", fixed: 1},
	"math.Sinh":         {c: "gh_math_sinh", fixed: 1},
	"math.Sqrt":         {c: "gh_math_sqrt", fixed: 1},
	"math.Tan":          {c: "gh_math_tan", fixed: 1},
	"math.Tanh":         {c: "gh_math_tanh", fixed: 1},
	"math.Trunc":        {c: "gh_math_trunc", fixed: 1},
	"math/rand.Float32": {c: "gh_rand_float32"},
	"math/rand.Float64": {c: "gh_rand_float64"},
	"math/rand.Int":     {c: "gh_rand_int"},
	"math/rand.Int31":   {c: "gh_rand_int31"},
	"math/rand.Int31n":  {c: "gh_rand_int31n", fixed: 1},
	"math/rand.Int63":   {c: "gh_rand_int63"},
	"math/rand.Int63n":  {c: "gh_rand_int63n", fixed: 1},
	"math/rand.Intn":    {c: "gh_rand_intn", fixed: 1},
	"math/rand.Seed":    {c: "gh_rand_seed", fixed: 1},
	"math/rand.Uint32":  {c: "gh_rand_uint32"},
	"math/rand.Uint64":  {c: "gh_rand_uint64"},

	"os.Executable": {c: "gh_os_executable"},
	"os.Geteuid":    {c: "gh_os_geteuid"},
	"os.Getenv":     {c: "gh_os_getenv", fixed: 1},
	"os.Getpid":     {c: "gh_os_getpid"},
	"os.Getuid":     {c: "gh_os_getuid"},
	"os.Hostname":   {c: "gh_os_hostname"},
	"os.Mkdir":      {c: "gh_os_mkdir", fixed: 1},
	"os.MkdirAll":   {c: "gh_os_mkdir_all", fixed: 1},
	"os.ReadFile":   {c: "gh_os_read_file", fixed: 1},
	"os.Remove":     {c: "gh_os_remove", fixed: 1},
	"os.RemoveAll":  {c: "gh_os_remove_all", fixed: 1},
	"os.Rename":     {c: "gh_os_rename", fixed: 2},
	"os.Setenv":     {c: "gh_os_setenv", fixed: 2},
	"os.TempDir":    {c: "gh_os_temp_dir"},
	"os.Unsetenv":   {c: "gh_os_unsetenv", fixed: 1},
	"os.WriteFile":  {c: "gh_os_write_file", fixed: 2},

	"runtime.GC":           {c: "gh_runtime_gc"},
	"runtime.GOMAXPROCS":   {c: "gh_runtime_gomaxprocs", fixed: 1},
	"runtime.Gosched":      {c: "gh_runtime_gosched"},
	"runtime.NumCPU":       {c: "gh_runtime_num_cpu"},
	"runtime.NumGoroutine": {c: "gh_runtime_num_goroutine"},

	"strconv.Atoi":        {c: "gh_strconv_atoi", fixed: 1},
	"strconv.FormatBool":  {c: "gh_strconv_format_bool", fixed: 1},
	"strconv.FormatFloat": {c: "gh_strconv_format_float", fixed: 4},
	"strconv.FormatInt":   {c: "gh_strconv_format_int", fixed: 2},
	"strconv.Itoa":        {c: "gh_strconv_itoa", fixed: 1},
	"strconv.ParseBool":   {c: "gh_strconv_parse_bool", fixed: 1},
	"strconv.ParseFloat":  {c: "gh_strconv_parse_float", fixed: 2},
	"strconv.ParseInt":    {c: "gh_strconv_parse_int", fixed: 3},
	"strconv.Quote":       {c: "gh_strconv_quote", fixed: 1},
	"strconv.Unquote":     {c: "gh_strconv_unquote", fixed: 1},

	"strings.Compare":      {c: "gh_strings_compare", fixed: 2},
	"strings.Contains":     {c: "gh_contains", fixed: 2},
	"strings.ContainsAny":  {c: "gh_contains_any", fixed: 2},
	"strings.ContainsRune": {c: "gh_contains_rune", fixed: 2},
	"strings.Count":        {c: "gh_count", fixed: 2},
	"strings.EqualFold":    {c: "gh_equal_fold", fixed: 2},
	"strings.HasPrefix":    {c: "gh_has_prefix", fixed: 2},
	"strings.HasSuffix":    {c: "gh_has_suffix", fixed: 2},
	"strings.Index":        {c: "gh_index", fixed: 2},
	"strings.IndexAny":     {c: "gh_index_any", fixed: 2},
	"strings.IndexRune":    {c: "gh_index_rune", fixed: 2},
	"strings.LastIndex":    {c: "gh_last_index", fixed: 2},
	"strings.Repeat":       {c: "gh_repeat", fixed: 2},
	"strings.Replace":      {c: "gh_replace", fixed: 4},
	"strings.ToLower":      {c: "gh_to_lower", fixed: 1},
	"strings.ToTitle":      {c: "gh_to_title", fixed: 1},
	"strings.ToUpper":      {c: "gh_to_upper", fixed: 1},
	"strings.Trim":         {c: "gh_trim", fixed: 2},
	"strings.TrimLeft":     {c: "gh_trim_left", fixed: 2},
	"strings.TrimPrefix":   {c: "gh_trim_prefix", fixed: 2},
	"strings.TrimRight":    {c: "gh_trim_right", fixed: 2},
	"strings.TrimSpace":    {c: "gh_trim_space", fixed: 1},
	"strings.TrimSuffix":   {c: "gh_trim_suffix", fixed: 2},

	"time.Date":  {c: "gh_time_date", fixed: 7},
	"time.Now":   {c: "gh_time_now"},
	"time.Since": {c: "gh_time_since", fixed: 1},
	"time.Sleep": {c: "gh_time_sleep", fixed: 1},
	"time.Unix":  {c: "gh_time_unix", fixed: 2},
	"time.Until": {c: "gh_time_until", fixed: 1},

	"time.Duration.Hours":        {c: "gh_duration_hours"},
	"time.Duration.Microseconds": {c: "gh_duration_microseconds"},
	"time.Duration.Milliseconds": {c: "gh_duration_milliseconds"},
	"time.Duration.Minutes":      {c: "gh_duration_minutes"},
	"time.Duration.Nanoseconds":  {c: "gh_duration_nanoseconds"},
	"time.Duration.Seconds":      {c: "gh_duration_seconds"},
	"time.Duration.String":       {c: "gh_duration_string"},

	"time.Time.Add":        {c: "gh_time_add", fixed: 1},
	"time.Time.After":      {c: "gh_time_after", fixed: 1},
	"time.Time.Before":     {c: "gh_time_before", fixed: 1},
	"time.Time.Compare":    {c: "gh_time_compare", fixed: 1},
	"time.Time.Day":        {c: "gh_time_day"},
	"time.Time.Equal":      {c: "gh_time_equal", fixed: 1},
	"time.Time.Format":     {c: "gh_time_format", fixed: 1},
	"time.Time.Hour":       {c: "gh_time_hour"},
	"time.Time.IsZero":     {c: "gh_time_is_zero"},
	"time.Time.Minute":     {c: "gh_time_minute"},
	"time.Time.Month":      {c: "gh_time_month"},
	"time.Time.Nanosecond": {c: "gh_time_nanosecond"},
	"time.Time.Round":      {c: "gh_time_round", fixed: 1},
	"time.Time.Second":     {c: "gh_time_second"},
	"time.Time.String":     {c: "gh_time_string"},
	"time.Time.Sub":        {c: "gh_time_sub", fixed: 1},
	"time.Time.Truncate":   {c: "gh_time_truncate", fixed: 1},
	"time.Time.Unix":       {c: "gh_time_unix_nano"},
	"time.Time.UnixMilli":  {c: "gh_time_unix_milli"},
	"time.Time.UnixNano":   {c: "gh_time_unix_nano"},
	"time.Time.Weekday":    {c: "gh_time_weekday"},
	"time.Time.Year":       {c: "gh_time_year"},
	"time.Time.YearDay":    {c: "gh_time_year_day"},
}

// nativeValues maps "import/path.Variable" keys onto values the backend knows
// when it runs. runtime.Version reports the Go release that compiled gohome,
// which is the only toolchain version a program can be built against.
var nativeValues = map[string]string{
	"runtime.Version": runtime.Version(),
}

// nativeFunction reports whether a selector call target is an embedded stub
// function and returns the Object-C implementation to call instead.
func (f *frontend) nativeFunction(sel *ast.SelectorExpr) (nativeFunc, bool) {
	object := f.info.Uses[sel.Sel]
	if object == nil || object.Pkg() == nil {
		return nativeFunc{}, false
	}
	spec, ok := nativeFuncs[object.Pkg().Path()+"."+object.Name()]
	return spec, ok
}

// nativeMethod is nativeFunction for method calls, keyed by receiver type.
func nativeMethod(selection *types.Selection) (nativeFunc, bool) {
	function, ok := selection.Obj().(*types.Func)
	if !ok || function.Pkg() == nil {
		return nativeFunc{}, false
	}
	receiver, ok := types.Unalias(selection.Recv()).(*types.Named)
	if !ok {
		return nativeFunc{}, false
	}
	spec, ok := nativeFuncs[function.Pkg().Path()+"."+receiver.Obj().Name()+"."+function.Name()]
	return spec, ok
}

// nativeValue reports whether a selector names an embedded stub variable whose
// value the backend already knows, and returns that value.
func nativeValue(f *frontend, sel *ast.SelectorExpr) (string, bool) {
	object := f.info.Uses[sel.Sel]
	if object == nil || object.Pkg() == nil {
		return "", false
	}
	value, ok := nativeValues[object.Pkg().Path()+"."+object.Name()]
	return value, ok
}

func (f *frontend) nativeCall(e *ast.CallExpr, spec nativeFunc, receiver ast.Expr) string {
	args := e.Args
	fixed := spec.fixed
	if receiver != nil {
		args = append([]ast.Expr{receiver}, args...)
		fixed++
	}
	var prep, values []string
	for i := 0; i < fixed && i < len(args); i++ {
		name := f.fresh("arg")
		prep = append(prep, f.ctype(f.info.TypeOf(args[i]))+" "+name+" = "+f.expr(args[i])+";")
		values = append(values, name)
	}
	if spec.variadic {
		var items []string
		for _, arg := range args[fixed:] {
			name := f.fresh("boxed")
			prep = append(prep, "id "+name+" = "+f.boxed(arg)+";")
			items = append(items, name)
		}
		name := f.fresh("boxed")
		prep = append(prep, "NSArray *"+name+" = @["+strings.Join(items, ", ")+"];")
		values = append(values, name)
	}
	call := spec.c + "(" + strings.Join(values, ", ") + ")"
	if len(prep) == 0 {
		return call
	}
	return "({ " + strings.Join(prep, " ") + " " + call + "; })"
}

// boxed renders a value as an Objective-C object so that the C formatter can
// inspect it at runtime. Strings and ios.Object pass through, values with a
// String method are rendered by it, everything else becomes an NSNumber.
func (f *frontend) boxed(node ast.Expr) string {
	t := f.info.TypeOf(node)
	if isObject(t) {
		return "(" + f.expr(node) + ")"
	}
	value := f.expr(node)
	if spec, ok := nativeStringer(t); ok {
		return "(" + spec.c + "(" + value + "))"
	}
	b := basic(t)
	if b == nil {
		f.fail(node, "formatted values must be strings, numbers, bool, time values or ios.Object")
		return "nil"
	}
	switch {
	case b.Info()&types.IsString != 0:
		return "(" + value + " ?: @\"\")"
	case b.Info()&types.IsBoolean != 0:
		return "(@((BOOL)(" + value + ")))"
	case b.Info()&types.IsUnsigned != 0:
		if b.Kind() == types.Uint64 {
			return "(@((unsigned long long)(" + value + ")))"
		}
		return "(@((unsigned int)(" + value + ")))"
	case b.Info()&types.IsInteger != 0:
		return "(@((long long)(" + value + ")))"
	case b.Info()&types.IsFloat != 0:
		return "(@((double)(" + value + ")))"
	}
	f.fail(node, "formatted values must be strings, numbers, bool, time values or ios.Object")
	return "nil"
}

// nativeStringer finds the String method of an embedded stub type, so that
// fmt renders time.Time and time.Duration the way the standard library does.
func nativeStringer(t types.Type) (nativeFunc, bool) {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return nativeFunc{}, false
	}
	spec, ok := nativeFuncs[named.Obj().Pkg().Path()+"."+named.Obj().Name()+".String"]
	return spec, ok
}
