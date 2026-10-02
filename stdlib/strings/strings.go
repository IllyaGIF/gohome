// Package strings is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// Indices are byte offsets like in the standard library, so string functions
// work with non-ASCII text, but there are no slices here: Split, Fields,
// Join and every other function that returns []string is absent, as are the
// Cut family and Builder, which would need multiple return values.
package strings

func Compare(a, b string) int { panic("use gohome build") }

func Contains(s, substr string) bool { panic("use gohome build") }

func ContainsAny(s, chars string) bool { panic("use gohome build") }

func ContainsRune(s string, r rune) bool { panic("use gohome build") }

func Count(s, substr string) int { panic("use gohome build") }

func EqualFold(s, t string) bool { panic("use gohome build") }

func HasPrefix(s, prefix string) bool { panic("use gohome build") }

func HasSuffix(s, suffix string) bool { panic("use gohome build") }

func Index(s, substr string) int { panic("use gohome build") }

func IndexAny(s, chars string) int { panic("use gohome build") }

func IndexRune(s string, r rune) int { panic("use gohome build") }

func LastIndex(s, substr string) int { panic("use gohome build") }

func Repeat(s string, count int) string { panic("use gohome build") }

func Replace(s, old, new string, n int) string { panic("use gohome build") }

func ToLower(s string) string { panic("use gohome build") }

func ToTitle(s string) string { panic("use gohome build") }

func ToUpper(s string) string { panic("use gohome build") }

func Trim(s, cutset string) string { panic("use gohome build") }

func TrimLeft(s, cutset string) string { panic("use gohome build") }

func TrimPrefix(s, prefix string) string { panic("use gohome build") }

func TrimRight(s, cutset string) string { panic("use gohome build") }

func TrimSpace(s string) string { panic("use gohome build") }

func TrimSuffix(s, suffix string) string { panic("use gohome build") }
