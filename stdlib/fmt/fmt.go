// Package fmt is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// The backend has no interfaces, no io.Writer and no error values, so only
// the package level formatting functions exist. Every argument is boxed as
// NSString or NSNumber, which covers strings, integers, floats, bool,
// time.Duration and gohome/ios.Object. Verbs understood by the formatter:
// %v %s %q %d %b %o %x %X %c %e %g %t %%. Anything else prints the Go style
// %!verb(...) diagnostic.
package fmt

func Println(a ...any) { panic("use gohome build") }

func Print(a ...any) { panic("use gohome build") }

func Printf(format string, a ...any) { panic("use gohome build") }

func Sprint(a ...any) string { panic("use gohome build") }

func Sprintln(a ...any) string { panic("use gohome build") }

func Sprintf(format string, a ...any) string { panic("use gohome build") }
