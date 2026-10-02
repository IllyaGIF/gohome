// Package rand is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// There are no goroutines and no sources of entropy, so rand is backed by a
// deterministic xorshift64* generator. Call rand.Seed before use if you need
// a different sequence. rand.New, rand.Perm and rand.Shuffle are absent
// because they return slices or pointers.
package rand

func Seed(seed int64) { panic("use gohome build") }

func Int() int { panic("use gohome build") }

func Int31() int32 { panic("use gohome build") }

func Int31n(n int32) int32 { panic("use gohome build") }

func Int63() int64 { panic("use gohome build") }

func Int63n(n int64) int64 { panic("use gohome build") }

func Intn(n int) int { panic("use gohome build") }

func Uint32() uint32 { panic("use gohome build") }

func Uint64() uint64 { panic("use gohome build") }

func Float32() float32 { panic("use gohome build") }

func Float64() float64 { panic("use gohome build") }
