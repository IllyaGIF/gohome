package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestHooksAndUIKit(t *testing.T) {
	tweak := `package main
import i "gohome/ios"
var suffix = " from Go"
func init() { i.Log("loaded") }
func main() {
 i.Hook("NSObject", "description", func(self i.Object, original func(i.Object) string) string { return original(self) + suffix })
 i.Hook("UILabel", "setText:", func(self i.Object, original func(i.Object, string), text string) { original(self, text + suffix) })
 i.HookClass("NSObject", "new", func(self i.Object, original func(i.Object) i.Object) i.Object { return original(self) })
}`
	r, err := Compile(map[string]string{"main.go": tweak}, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Hooks != 3 {
		t.Fatalf("got %d hooks", r.Hooks)
	}
	app := `package main
import "gohome/ios"
var label ios.Object
func clicked() { ios.SetText(label,"clicked") }
func main() {
 label = ios.Label("hello",20,40,280,40)
 ios.Button("click",20,100,280,44,clicked)
 ios.Button("alert",20,160,280,44,func(){ ios.Alert("title","message") })
 if label != (ios.Object{}) { label.SendVoid("setHidden:",false) }
 ios.Log(label.SendString("text"))
}`
	if _, err := Compile(map[string]string{"app.go": app}, true); err != nil {
		t.Fatal(err)
	}
}

func TestStdlibPackages(t *testing.T) {
	source := `package main
import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"
)
var started = time.Now()
func main() {
	runtime.GC()
	runtime.Gosched()
	fmt.Println("hello", 42, 1.5, true, started)
	fmt.Printf("%s took %v (%d/%d)\n", "gohome", time.Since(started), runtime.NumCPU(), runtime.NumGoroutine())
	fmt.Printf("%s", fmt.Sprintf("%.1f %.3f %d %x %q %%", math.Sqrt(2), math.Max(math.Pi, 1), rand.Intn(10)+1, 255, "hi"))
	fmt.Print(os.Getenv("HOME"), os.Hostname(), os.Getpid(), rand.Float64())
	text := fmt.Sprint(1, "two", 3.0)
	fmt.Println(text, fmt.Sprintln("a", "b"))
	os.Setenv("GOHOME", "1")
	fmt.Println(os.Unsetenv("GOHOME"), os.Getenv("GOHOME"))
	moment := time.Date(2024, time.March, 7, 9, 5, 3, 250000000)
	fmt.Println(moment.Year(), moment.Month(), moment.Day(), moment.Weekday(), moment.YearDay())
	fmt.Println(moment.Hour(), moment.Minute(), moment.Second(), moment.Nanosecond())
	fmt.Println(moment.Format("2006-01-02 15:04:05.000 MST"), moment.String())
	fmt.Println(moment.Unix(), moment.UnixNano(), moment.UnixMilli())
	fmt.Println(moment.Add(time.Hour).Sub(moment), moment.Before(moment.Add(time.Second)), moment.After(moment))
	fmt.Println(moment.IsZero(), moment.Compare(moment), moment.Equal(moment), moment.Truncate(time.Minute))
	fmt.Println(time.Unix(1700000000, 0).Year(), time.Since(moment), time.Until(moment))
	fmt.Println((time.Second * 90).Seconds(), (time.Second * 90).String(), time.Nanosecond)
	fmt.Println(math.Abs(-2), math.Ceil(1.2), math.Floor(1.8), math.Pow(2, 10), math.MaxInt32)
	fmt.Println(math.IsNaN(math.NaN()), math.IsInf(math.Inf(1), 1), math.Signbit(-2.5), math.Mod(7, 3))
	fmt.Println(rand.Int(), rand.Int31(), rand.Int63(), rand.Int63n(9), rand.Uint32(), rand.Uint64())
	fmt.Println(os.DevNull, os.TempDir(), os.Executable(), os.Getuid(), os.Geteuid())
	fmt.Println(runtime.GOOS, runtime.GOARCH, runtime.Compiler, runtime.GOMAXPROCS(1))
	fmt.Println(runtime.Version, "built by", runtime.Compiler)
	if runtime.GOOS != "ios" { panic("backend mismatch") }
}
`
	r, err := Compile(map[string]string{"main.go": source}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gh_fmt_println(", "gh_fmt_printf(", "gh_fmt_sprint(", "gh_fmt_sprintln(", "gh_fmt_sprintf(", "gh_fmt_print(", "gh_math_sqrt(", "gh_math_max(", "gh_math_nan(", "gh_math_inf(", "gh_rand_intn(", "gh_rand_int63n(", "gh_rand_float64(", "gh_os_getenv(", "gh_os_unsetenv(", "gh_os_temp_dir(", "gh_runtime_gc(", "gh_time_now(", "gh_time_since(", "gh_time_date(", "gh_time_year(", "gh_time_format(", "gh_time_weekday(", "gh_time_is_zero(", "gh_duration_string(", "gh_duration_seconds("} {
		if !strings.Contains(r.Source, want) {
			t.Errorf("generated source is missing %s", want)
		}
	}
	if !strings.Contains(r.Source, "@[") {
		t.Error("variadic arguments are not boxed into an array literal")
	}
	if regexp.MustCompile(`gh_fmt_(print|println|printf|sprint|sprintln|sprintf)\(\)`).MatchString(r.Source) {
		t.Error("fmt call dropped its arguments")
	}
	if regexp.MustCompile(`gh_(time_format|time_year|time_month|duration_string|duration_seconds)\(\)`).MatchString(r.Source) {
		t.Error("method call dropped its receiver")
	}
	if !strings.Contains(r.Source, "gh_time_string(gh_") {
		t.Error("fmt does not render time values through their String method")
	}
	if !strings.Contains(r.Source, cbytes(runtime.Version())) {
		t.Errorf("runtime.Version does not report %q", runtime.Version())
	}
}

func TestStringsStrconvIndexingAndRange(t *testing.T) {
	source := `package main
import ("fmt"; "strconv"; "strings")
func main() {
	text := "  Привет, gohome  "
	trimmed := strings.TrimSpace(text)
	fmt.Println(trimmed, strings.ToUpper(trimmed), strings.ToTitle(trimmed))
	fmt.Println(strings.Contains(trimmed, "gohome"), strings.HasPrefix(trimmed, "Привет"), strings.HasSuffix(trimmed, "home"))
	fmt.Println(strings.Index(trimmed, ","), strings.LastIndex(trimmed, "o"), strings.Count(trimmed, "o"))
	fmt.Println(strings.Replace(trimmed, "gohome", "Go", 1), strings.Repeat("ab", 3), strings.TrimPrefix(trimmed, "Привет"))
	fmt.Println(strings.EqualFold(trimmed, strings.ToLower(trimmed)), strings.ContainsAny(trimmed, "!?"), strings.IndexAny(trimmed, ", "))
	fmt.Println(strings.ContainsRune(trimmed, 'П'), strings.IndexRune(trimmed, 'П'), strings.Compare(trimmed, trimmed))
	fmt.Println(strings.Trim(text, " "), strings.TrimLeft(trimmed, "Привет"), strings.TrimRight(trimmed, "gohome"))
	fmt.Println(strconv.Itoa(42), strconv.FormatInt(255, 16), strconv.FormatBool(true), strconv.Quote(trimmed))
	fmt.Println(strconv.Atoi("17"), strconv.ParseInt("ff", 16, 64), strconv.ParseFloat("2.5", 64), strconv.ParseBool("true"))
	fmt.Println(strconv.FormatFloat(1.0/3.0, 'f', 4, 64), strconv.Unquote("\"hi\""))
	if trimmed[0] != 0x20 {
		_ = trimmed[0]
	}
	total := 0
	for i, r := range trimmed {
		total += i + int(r)
	}
	for range trimmed {
		total++
	}
	fmt.Println(total, len(trimmed))
}
`
	r, err := Compile(map[string]string{"main.go": source}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gh_trim_space(", "gh_to_upper(", "gh_replace(", "gh_repeat(", "gh_equal_fold(", "gh_index_rune(", "gh_strconv_itoa(", "gh_strconv_format_float(", "gh_strconv_unquote(", "gh_byte(", "gh_rune(", "for (int32_t"} {
		if !strings.Contains(r.Source, want) {
			t.Errorf("generated source is missing %s", want)
		}
	}
}

func TestTabsAndViewFiles(t *testing.T) {
	source := `package main
import ("fmt"; "gohome/ios"; "time")
var label ios.Object
func viewHome(page ios.Object) {
	label = ios.Label("hello", 20, 20, -1, 30)
	ios.Button("tap", 20, 60, -1, 44, tapped)
}
func tapped() { ios.SetText(label, fmt.Sprint("tapped at ", time.Now().Format("15:04:05"))) }
func viewAbout(page ios.Object) { ios.Label(fmt.Sprint("pid ", 1), 20, 20, -1, 30) }
func main() {
	ios.Tab("Home", "square", viewHome)
	ios.Tab("About", "", viewAbout)
}
`
	r, err := Compile(map[string]string{"main.go": source}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(r.Source, "gh_tab(gh_arg_") != 2 {
		t.Errorf("got %d tab registrations", strings.Count(r.Source, "gh_tab(gh_arg_"))
	}
	if regexp.MustCompile(`gh_tab\([^,)]*\)`).MatchString(r.Source) {
		t.Error("tab callback is not passed to the runtime")
	}
	if !strings.Contains(r.Source, "gh_fn_viewHome") || !strings.Contains(r.Source, "gh_fn_viewAbout") {
		t.Error("view builders are missing from the generated source")
	}
	launch := launchSequence(r.Source)
	if launch == "" {
		t.Fatal("no application launch sequence in the generated source")
	}
	for _, step := range []string{"gh_init();", "gh_main();", "gh_build_tabs()", "gh_show("} {
		if !strings.Contains(launch, step) {
			t.Errorf("launch sequence is missing %s:\n%s", step, launch)
		}
	}
	if !strings.Contains(r.Source, "setViewControllers:controllers animated:NO") {
		t.Error("tabs are not installed with setViewControllers:")
	}
	for _, step := range []string{"gh_tab_count", "gh_build_tabs()", "gh_show("} {
		if strings.Index(launch, step) < strings.Index(launch, "gh_main();") {
			t.Errorf("%s runs before main, so the tabs are never built:\n%s", step, launch)
		}
	}
}

// launchSequence returns the body of application:didFinishLaunchingWithOptions
// so that the order of the startup steps can be asserted.
func launchSequence(source string) string {
	start := strings.Index(source, "- (BOOL)application:")
	if start < 0 {
		return ""
	}
	body := source[start:]
	if end := strings.Index(body, "\n@end"); end >= 0 {
		return body[:end]
	}
	return body
}

func TestRejectedFeatures(t *testing.T) {
	cases := []struct{ source, contains string }{
		{`package main; import "net/http"; func main(){http.Get("x")}`, `import "net/http"`},
		{`package main; import "runtime"; func main(){_ = runtime.Version()}`, `not a function`},
		{`package main; import "gohome/ios"; func main(){ios.Tab("x", "", func(p ios.Object){})}`, `requires an app project`},
		{`package main; func main(){for range 3 {}}`, `range only supports strings`},
		{`package main; func main(){s:="a"; _=s[0:1]}`, `*ast.SliceExpr`},
		{`package main; import "strings"; func main(){_ = strings.Split("a,b", ",")}`, `gohome doc`},
		{`package main; func main(){go main()}`, `unsupported Go statement`},
		{`package main; func main(){defer main()}`, `unsupported Go statement`},
		{`package main; func main(){a:=[]int{1}; _=a}`, `unsupported`},
		{`package main; func main(){var x map[string]int; _=x}`, `unsupported`},
		{`package main; import "gohome/ios"; func main(){x:=1; ios.Button("x",0,0,1,1,func(){x++})}`, `capture local`},
		{`package main; import "gohome/ios"; func main(){ios.Hook("NSObject","description",func(self ios.Object, original func(ios.Object) int) string{return "x"})}`, `return types`},
		{`package main; import "gohome/ios"; func main(){ios.Hook("NSObject","setX:",func(self ios.Object, original func(ios.Object)){})}`, `colon count`},
		{`package main; import "gohome/ios"; func main(){ios.New("UILabel").SendVoid("setText:")}`, `colon count`},
		{`package main; var x int = 2147483648; func main(){}`, `overflows`},
		{`package main; func main(){ _ = "\xff" }`, `valid UTF-8`},
		{`package main; func foo() (int,int){return 1,2}; func main(){}`, `multiple return`},
	}
	for _, tc := range cases {
		_, err := Compile(map[string]string{"main.go": tc.source}, true)
		if strings.Contains(tc.source, "ios.Hook") || strings.Contains(tc.source, "ios.Tab") {
			_, err = Compile(map[string]string{"main.go": tc.source}, false)
		}
		if err == nil || !strings.Contains(err.Error(), tc.contains) {
			t.Errorf("want %q, got %v for %s", tc.contains, err, tc.source)
		}
	}
}

func TestNativeEvaluationOrderAndControlFlow(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is unavailable")
	}
	source := `package main
var step int
func next() int { step++; return step }
func combine(a,b int) int { return a*10+b }
func result() int {
 a,b:=next(),next()
 a,b=b,a
 if a!=2 || b!=1 { return 1 }
 step=0
 if combine(next(),next()) != 12 { return 2 }
 step=0
 if next()*10+next() != 12 { return 3 }
 total:=0
 for i:=0; i<8; i++ {
  if i==2 {continue}
  switch i {case 4: break; case 6: total+=10; default: total+=i}
 }
 if total!=26 {return 4}
 if x:=7; x==7 {x:=9; if x!=9 {return 5}}
 switch {case total<0:return 6;case total==26:total++}
 var wrapped int8=127
 wrapped++
 if wrapped!=-128 {return 7}
 a=next()
 if false && next()>0 {return 8}
 if true || next()>0 {} else {return 9}
 if step!=3 {return 10}
 var huge uint64 = 18446744073709551615
 if a<<huge != 0 || a<<uint64(100000000000) != 0 {return 11}
 var negative int32 = -1
 if negative>>huge != -1 {return 12}
 negative <<= huge
 if negative != 0 {return 13}
 var minimum int64 = -9223372036854775808
 var minusOne int64 = -1
 if minimum/minusOne != minimum || minimum%minusOne != 0 {return 14}
 if huge/huge != 1 || huge%huge != 0 {return 15}
 var shifted uint8 = 255
 var count int = 4
 shifted >>= count
 if shifted!=15 {return 16}
 return 0
}
func main() {}`
	r, err := Compile(map[string]string{"main.go": source}, false)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimPrefix(r.Source, "#define GH_APP 0\n"+RuntimeSource+"\n")
	body = strings.ReplaceAll(body, `@"negative shift count"`, `"negative shift count"`)
	mathRuntime := RuntimeSource[strings.Index(RuntimeSource, "static int64_t gh_div"):strings.Index(RuntimeSource, "static void gh_log")]
	mathRuntime = strings.ReplaceAll(mathRuntime, `@"`, `"`)
	c := "#include <stdint.h>\n#include <stdbool.h>\n#include <limits.h>\n#include <stdlib.h>\nstatic void gh_panic(const char *message) { abort(); }\n" + mathRuntime + body + "\nint main(void) { return gh_fn_result(); }\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.c")
	if err := os.WriteFile(path, []byte(c), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "test")
	if out, err := exec.Command(clang, "-std=gnu11", "-fwrapv", path, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, out, c)
	}
	if out, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native semantics: %v %s", err, out)
	}
}

func TestMultipleFilesAndGlobalInitOrder(t *testing.T) {
	r, err := Compile(map[string]string{
		"a.go": `package main; var first = second+1; func main(){}`,
		"b.go": `package main; var second = 4`,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(r.Source, "static void gh_main") < 2 {
		t.Fatal("missing entrypoint")
	}
}
