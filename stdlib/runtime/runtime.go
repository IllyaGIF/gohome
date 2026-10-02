// Package runtime is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// Version is the Go release that compiled gohome, so a program can report the
// toolchain it was built with. Generated code is Objective-C, so there is no
// scheduler, no heap and nothing to collect: GC and Gosched are no-ops and
// NumGoroutine always reports the single thread that runs main.
package runtime

const (
	GOOS     = "ios"
	GOARCH   = "arm"
	Compiler = "gohome"
)

var Version string

func GC() { panic("use gohome build") }

func Gosched() { panic("use gohome build") }

func NumCPU() int { panic("use gohome build") }

func NumGoroutine() int { panic("use gohome build") }

func GOMAXPROCS(n int) int { panic("use gohome build") }
