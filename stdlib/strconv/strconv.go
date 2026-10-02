// Package strconv is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// The backend has no error values, so the parsing functions report failure the
// way the standard library does in the value they return: Atoi and ParseInt
// give 0, ParseFloat gives 0 and ParseBool gives false. Quote and FormatFloat
// understand 'b', 'e', 'E', 'f', 'g' and 'G', with bitSize 32 or 64.
package strconv

func Itoa(i int) string { panic("use gohome build") }

func Atoi(s string) int { panic("use gohome build") }

func FormatInt(i int64, base int) string { panic("use gohome build") }

func FormatFloat(f float64, format byte, precision int, bitSize int) string {
	panic("use gohome build")
}

func ParseFloat(s string, bitSize int) float64 { panic("use gohome build") }

func ParseInt(s string, base int, bitSize int) int64 { panic("use gohome build") }

func ParseBool(str string) bool { panic("use gohome build") }

func FormatBool(b bool) string { panic("use gohome build") }

func Quote(s string) string { panic("use gohome build") }

func Unquote(s string) string { panic("use gohome build") }
