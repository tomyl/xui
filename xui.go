// Package xui is a console user interface toolkit based on gocui.
package xui

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/tomyl/gocui"
)

// Widget movements
const (
	ActionNextLine     = "next_line"
	ActionNextPage     = "next_page"
	ActionPreviousLine = "prev_line"
	ActionPreviousPage = "prev_page"
)

var reStripEscapeSeq *regexp.Regexp

func init() {
	reStripEscapeSeq = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
}

// ErrAction represents an error encountered during a widet action.
type ErrAction struct {
	viewname string
	action   string
	err      error
}

func (e ErrAction) Error() string {
	if e.action != "" {
		if e.err != nil {
			return fmt.Sprintf("widget \"%s\" failed to handle action \"%s\": %v", e.viewname, e.action, e.err)
		}
		return fmt.Sprintf("widget \"%s\" does not handle action \"%s\": %v", e.viewname, e.action)
	}

	return e.err.Error()
}

// Error builds an widget action error
func Error(msg string) ErrAction {
	return ErrAction{err: errors.New(msg)}
}

// UnknownAction builds an widget action error indicating that the widget
// didn't understand the action.
func UnknownAction() ErrAction {
	return ErrAction{err: nil}
}

// A Widget can be bound to gocui.View and handle action commands.
type Widget interface {
	View() *gocui.View
	SetView(*gocui.View)
	HandleAction(string) error
}

// Xui is a wrapper around gocui.Gui.
type Xui struct {
	g   *gocui.Gui
	err error

	preActionFunc  func()
	postActionFunc func(error) error
}

// New wraps a gocui.Gui instance.
func New(g *gocui.Gui) *Xui {
	return &Xui{g: g}
}

// SetPreActionHandler sets a hook that is called before a widget action is executed.
func (gx *Xui) SetPreActionHandler(f func()) {
	gx.preActionFunc = f
}

// SetPostActionHandler sets a hook that is called after a widget action is executed.
func (gx *Xui) SetPostActionHandler(f func(error) error) {
	gx.postActionFunc = f
	gx.err = nil
}

// Err returns the first encountered error. Always returns nil if a post-action
// handler has been set.
func (gx *Xui) Err() error {
	return gx.err
}

func (gx *Xui) callPreActionHandler() {
	if gx.preActionFunc != nil {
		gx.preActionFunc()
	}
}

func (gx *Xui) callPostActionHandler(err error) error {
	if gx.postActionFunc != nil {
		err = gx.postActionFunc(err)
	} else if gx.err == nil {
		gx.err = err
	}
	return err
}

// SetKeybinding is a wrapper around gocui.Gui.SetKeybinding.
func (gx *Xui) SetKeybinding(viewname string, key interface{}, mod gocui.Modifier, handler func(*gocui.Gui, *gocui.View) error) {
	if gx.err == nil {
		gx.err = gx.g.SetKeybinding(viewname, key, mod, func(g *gocui.Gui, view *gocui.View) error {
			gx.callPreActionHandler()
			return gx.callPostActionHandler(handler(g, view))
		})
	}
}

// SetWidgetKeybinding is a wrapper around gocui.Gui.SetKeybinding.
func (gx *Xui) SetWidgetKeybinding(widget Widget, key interface{}, mod gocui.Modifier, handler func() error) {
	if gx.err == nil {
		view := widget.View()
		if view == nil {
			gx.err = errors.New("widget has no view")
		} else {
			gx.err = gx.g.SetKeybinding(view.Name(), key, mod,
				func(*gocui.Gui, *gocui.View) error {
					gx.callPreActionHandler()
					err := handler()
					if err != nil {
						err = ErrAction{viewname: view.Name(), err: err}
					}
					return gx.callPostActionHandler(err)
				})
		}
	}
}

// SetWidgetAction is a wrapper around gocui.Gui.SetKeybinding for sending an action command to widget.
func (gx *Xui) SetWidgetAction(widget Widget, key interface{}, mod gocui.Modifier, action string) {
	if gx.err == nil {
		view := widget.View()
		if view == nil {
			gx.err = errors.New("widget has no view")
		} else {
			gx.err = gx.g.SetKeybinding(view.Name(), key, mod,
				func(*gocui.Gui, *gocui.View) error {
					gx.callPreActionHandler()
					err := widget.HandleAction(action)
					if e, ok := err.(ErrAction); ok {
						e.viewname = view.Name()
						e.action = action
						err = e
					}
					gx.callPostActionHandler(err)
					return nil
				})
		}
	}
}

// SetView is a wrapper around gocui.Gui.SetView.
func (gx *Xui) SetView(name string, x0, y0, x1, y1 int) *gocui.View {
	if gx.err != nil {
		return nil
	}

	view, err := gx.g.SetView(name, x0, y0, x1, y1)

	if err != nil {
		if err != gocui.ErrUnknownView {
			gx.err = err
			return nil
		}
	}

	return view
}

// SetRegionView is a wrapper around gocui.Gui.SetRegionView for changing view
// size to size of provided region.
func (gx *Xui) SetRegionView(name string, r Region) *gocui.View {
	x0, y0, x1, y1 := r.Rect(gx.g)
	view := gx.SetView(name, x0, y0, x1, y1)
	if view != nil {
		view.Frame = false
	}
	return view
}

// SetCurrentView is a wrapper around gocui.Gui.SetCurrentView.
func (gx *Xui) SetCurrentView(name string) {
	if gx.err == nil {
		_, gx.err = gx.g.SetCurrentView(name)
	}
}

// SetViewOnTop is a wrapper around gocui.Gui.SetViewOnTop.
func (gx *Xui) SetViewOnTop(name string) {
	if gx.err == nil {
		_, gx.err = gx.g.SetViewOnTop(name)
	}
}

// Focus changes focus to provided view.
func (gx *Xui) Focus(view *gocui.View) {
	if view != nil {
		gx.FocusName(view.Name())
	}
}

// FocusName changes focus to view with provided name.
func (gx *Xui) FocusName(name string) {
	if name != "" {
		gx.SetViewOnTop(name)
		gx.SetCurrentView(name)
	}
}

// A Region represents the area occupied by a gocui.View without the outer frame.
type Region struct {
	Left   int
	Top    int
	Right  int
	Bottom int
}

// Rect returns the area occupied by a gocui.View including the outer frame.
func (r Region) Rect(g *gocui.Gui) (int, int, int, int) {
	maxX, maxY := g.Size()

	left := r.Left
	top := r.Top
	right := r.Right
	bottom := r.Bottom

	if left < 0 {
		left = maxX + left
	}

	if top < 0 {
		top = maxY + top
	}

	if right < 0 {
		right = maxX + right
	}

	if bottom < 0 {
		bottom = maxY + bottom
	}

	// Adjust for gocui frame
	x0 := left - 1
	y0 := top - 1
	x1 := right + 1
	y1 := bottom + 1

	return x0, y0, x1, y1
}

// Handler returns a gocui keybinding handler that executes provided function
// and returns a nil error.
func Handler(f func()) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		f()
		return nil
	}
}

// ErrorHandler returns a gocui keybinding handler that returns provided error.
func ErrorHandler(err error) func(*gocui.Gui, *gocui.View) error {
	return func(g *gocui.Gui, v *gocui.View) error {
		return err
	}
}

// ResizeLayout calls provided layout function if view was resized.
func ResizeLayout(layout func(g *gocui.Gui) error) func(g *gocui.Gui) error {
	var ox int
	var oy int
	return func(g *gocui.Gui) error {
		x, y := g.Size()
		if x != ox || y != oy {
			ox = x
			oy = y
			return layout(g)
		}
		return nil
	}
}

// StringWidth returns the width of string in single-width unicode character units.
func StringWidth(s string) int {
	w := 0
	for _, ch := range s {
		rw := runewidth.RuneWidth(ch)
		if rw == 0 || rw == 2 && runewidth.IsAmbiguousWidth(ch) {
			rw = 1
		}
		w += rw
	}
	return w
}

// Pad appends spaces to provided string so that its length matches the width
// of the widget, while taking double-width unicode characters and ANSI escape
// sequences into account.
func Pad(s string, n int) string {
	ss := reStripEscapeSeq.ReplaceAllString(s, "")
	w := StringWidth(ss)
	if w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
