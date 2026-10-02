// Package time is a gohome backend stub: declarations only, bodies are
// replaced by Objective-C helpers at build time.
//
// Time is an int64 count of nanoseconds since the Unix epoch in UTC, so the
// zero Time is the epoch rather than year 1. Everything is UTC: Local,
// FixedZone, Location, timers and channels are absent. Time.Format
// understands the reference layout tokens 2006 06 01 1 Jan January 02 _2 2
// Monday Mon 15 03 04 4 05 5 PM pm, fractional seconds, MST, -0700, -07:00
// and Z07:00.
package time

type Duration int64

type Month int

type Weekday int

type Time int64

const (
	Nanosecond  Duration = 1
	Microsecond          = 1000 * Nanosecond
	Millisecond          = 1000 * Microsecond
	Second               = 1000 * Millisecond
	Minute               = 60 * Second
	Hour                 = 60 * Minute
)

const (
	Sunday Weekday = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

const (
	January Month = iota + 1
	February
	March
	April
	May
	June
	July
	August
	September
	October
	November
	December
)

func Now() Time { panic("use gohome build") }

func Since(t Time) Duration { panic("use gohome build") }

func Until(t Time) Duration { panic("use gohome build") }

func Unix(sec int64, nsec int64) Time { panic("use gohome build") }

func Date(year int, month Month, day int, hour int, min int, sec int, nsec int) Time {
	panic("use gohome build")
}

func Sleep(d Duration) { panic("use gohome build") }

func (t Time) Unix() int64 { panic("use gohome build") }

func (t Time) UnixNano() int64 { panic("use gohome build") }

func (t Time) UnixMilli() int64 { panic("use gohome build") }

func (t Time) Add(d Duration) Time { panic("use gohome build") }

func (t Time) Sub(u Time) Duration { panic("use gohome build") }

func (t Time) Before(u Time) bool { panic("use gohome build") }

func (t Time) After(u Time) bool { panic("use gohome build") }

func (t Time) Equal(u Time) bool { panic("use gohome build") }

func (t Time) Compare(u Time) int { panic("use gohome build") }

func (t Time) IsZero() bool { panic("use gohome build") }

func (t Time) Truncate(d Duration) Time { panic("use gohome build") }

func (t Time) Round(d Duration) Time { panic("use gohome build") }

func (t Time) Year() int { panic("use gohome build") }

func (t Time) YearDay() int { panic("use gohome build") }

func (t Time) Month() Month { panic("use gohome build") }

func (t Time) Day() int { panic("use gohome build") }

func (t Time) Weekday() Weekday { panic("use gohome build") }

func (t Time) Hour() int { panic("use gohome build") }

func (t Time) Minute() int { panic("use gohome build") }

func (t Time) Second() int { panic("use gohome build") }

func (t Time) Nanosecond() int { panic("use gohome build") }

func (t Time) String() string { panic("use gohome build") }

func (t Time) Format(layout string) string { panic("use gohome build") }

func (d Duration) Nanoseconds() int64 { panic("use gohome build") }

func (d Duration) Microseconds() int64 { panic("use gohome build") }

func (d Duration) Milliseconds() int64 { panic("use gohome build") }

func (d Duration) Seconds() float64 { panic("use gohome build") }

func (d Duration) Minutes() float64 { panic("use gohome build") }

func (d Duration) Hours() float64 { panic("use gohome build") }

func (d Duration) String() string { panic("use gohome build") }
