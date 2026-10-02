// Package math is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// MaxInt, MinInt and MaxUint follow the ARMv32 target, so they are 32 bit.
// Math.Min and Math.Max are plain float64 functions here instead of the
// generic declarations of the host toolchain.
package math

const (
	Pi = 3.14159265358979323846264338327950288419716939937510582097494459
	E  = 2.71828182845904523536028747135266249775724709369995957496696763

	MaxInt8  = 1<<7 - 1
	MinInt8  = -1 << 7
	MaxInt16 = 1<<15 - 1
	MinInt16 = -1 << 15
	MaxInt32 = 1<<31 - 1
	MinInt32 = -1 << 31
	MaxInt64 = 1<<63 - 1
	MinInt64 = -1 << 63

	MaxUint8  = 1<<8 - 1
	MaxUint16 = 1<<16 - 1
	MaxUint32 = 1<<32 - 1
	MaxUint64 = 1<<64 - 1

	MaxInt  = 1<<31 - 1
	MinInt  = -1 << 31
	MaxUint = 1<<32 - 1

	MaxFloat32             = 3.40282346638528859811704183484516925440e+38
	SmallestNonzeroFloat32 = 1.401298464324817070923729583289916131280e-45
	MaxFloat64             = 1.79769313486231570814527423731704356798070e+308
	SmallestNonzeroFloat64 = 4.9406564584124654417656879286822137236505980e-324
)

func Abs(x float64) float64 { panic("use gohome build") }

func Acos(x float64) float64 { panic("use gohome build") }

func Acosh(x float64) float64 { panic("use gohome build") }

func Asin(x float64) float64 { panic("use gohome build") }

func Asinh(x float64) float64 { panic("use gohome build") }

func Atan(x float64) float64 { panic("use gohome build") }

func Atan2(y, x float64) float64 { panic("use gohome build") }

func Atanh(x float64) float64 { panic("use gohome build") }

func Cbrt(x float64) float64 { panic("use gohome build") }

func Ceil(x float64) float64 { panic("use gohome build") }

func Copysign(f, sign float64) float64 { panic("use gohome build") }

func Cos(x float64) float64 { panic("use gohome build") }

func Cosh(x float64) float64 { panic("use gohome build") }

func Exp(x float64) float64 { panic("use gohome build") }

func Floor(x float64) float64 { panic("use gohome build") }

func Hypot(p, q float64) float64 { panic("use gohome build") }

func Inf(sign int) float64 { panic("use gohome build") }

func IsInf(f float64, sign int) bool { panic("use gohome build") }

func IsNaN(f float64) bool { panic("use gohome build") }

func Log(x float64) float64 { panic("use gohome build") }

func Log10(x float64) float64 { panic("use gohome build") }

func Log2(x float64) float64 { panic("use gohome build") }

func Max(x, y float64) float64 { panic("use gohome build") }

func Min(x, y float64) float64 { panic("use gohome build") }

func Mod(x, y float64) float64 { panic("use gohome build") }

func NaN() float64 { panic("use gohome build") }

func Pow(x, y float64) float64 { panic("use gohome build") }

func Round(x float64) float64 { panic("use gohome build") }

func Sin(x float64) float64 { panic("use gohome build") }

func Sinh(x float64) float64 { panic("use gohome build") }

func Signbit(x float64) bool { panic("use gohome build") }

func Sqrt(x float64) float64 { panic("use gohome build") }

func Tan(x float64) float64 { panic("use gohome build") }

func Tanh(x float64) float64 { panic("use gohome build") }

func Trunc(x float64) float64 { panic("use gohome build") }
