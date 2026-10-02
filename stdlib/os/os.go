// Package os is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// The backend has no error values and no byte slices, so the functions that
// return an error in the standard library report success as a bool here, and
// file contents are exchanged as strings instead of []byte. os.Args,
// os.Stdin, os.Stderr and os.Stdout are absent because they are slices and
// pointers.
package os

const DevNull = "/dev/null"

func Exit(code int) { panic("use gohome build") }

func Getenv(key string) string { panic("use gohome build") }

func Setenv(key, value string) bool { panic("use gohome build") }

func Unsetenv(key string) bool { panic("use gohome build") }

func Executable() string { panic("use gohome build") }

func Hostname() string { panic("use gohome build") }

func TempDir() string { panic("use gohome build") }

func Getpid() int { panic("use gohome build") }

func Getuid() int { panic("use gohome build") }

func Geteuid() int { panic("use gohome build") }

func ReadFile(name string) string { panic("use gohome build") }

func WriteFile(name, contents string) bool { panic("use gohome build") }

func Remove(name string) bool { panic("use gohome build") }

func RemoveAll(path string) bool { panic("use gohome build") }

func Rename(oldpath, newpath string) bool { panic("use gohome build") }

func Mkdir(name string) bool { panic("use gohome build") }

func MkdirAll(path string) bool { panic("use gohome build") }
