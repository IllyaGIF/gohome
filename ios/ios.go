package ios

// Object is any Objective-C instance. gohome never dereferences it, it only
// forwards it to the runtime.
type Object struct{ handle uintptr }

func Text(value any) string { panic("use gohome build") }

func Log(text string)                                { panic("use gohome build") }
func Alert(title, text string)                       { panic("use gohome build") }
func Class(name string) Object                       { panic("use gohome build") }
func New(name string) Object                         { panic("use gohome build") }
func Hook(class, selector string, callback any)      { panic("use gohome build") }
func HookClass(class, selector string, callback any) { panic("use gohome build") }

// Label adds a text label at x, y. A negative width or height mirrors the
// opposite margin, so -1 keeps the same inset on both sides and stretches with
// the parent; pass -1 for both to fill the tab.
func Label(text string, x, y, width, height float64) Object { panic("use gohome build") }
func Button(text string, x, y, width, height float64, action func()) Object {
	panic("use gohome build")
}
func SetText(object Object, text string)                  { panic("use gohome build") }
func SetFrame(object Object, x, y, width, height float64) { panic("use gohome build") }
func AddSubview(parent, child Object)                     { panic("use gohome build") }
func RootView() Object                                    { panic("use gohome build") }

// Tab registers a tab bar entry. icon is "", "square", "triangle" or "circle";
// icons are drawn at the screen scale, which is 30 by 30 points and 60 by 60
// pixels on a retina screen. build runs once, when the tab is first shown, and
// receives the tab page, so Label and Button land on the visible tab.
func Tab(title, icon string, build func(Object))                      { panic("use gohome build") }
func (object Object) Send(selector string, args ...any) Object        { panic("use gohome build") }
func (object Object) SendVoid(selector string, args ...any)           { panic("use gohome build") }
func (object Object) SendString(selector string, args ...any) string  { panic("use gohome build") }
func (object Object) SendInt(selector string, args ...any) int        { panic("use gohome build") }
func (object Object) SendBool(selector string, args ...any) bool      { panic("use gohome build") }
func (object Object) SendFloat(selector string, args ...any) float32  { panic("use gohome build") }
func (object Object) SendDouble(selector string, args ...any) float64 { panic("use gohome build") }
