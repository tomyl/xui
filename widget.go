package xui

import (
	"fmt"
	"strings"

	"github.com/tomyl/gocui"
)

// A TextWidget displays a string.
type TextWidget struct {
	BgColor, FgColor gocui.Attribute

	view *gocui.View
	text string
}

// SetText updates the string to display.
func (w *TextWidget) SetText(text string) {
	w.text = text
	w.render()
}

// View returns the gocui.View currently bound to this widget.
func (w *TextWidget) View() *gocui.View {
	return w.view
}

// SetView binds a gocui.View to this widget.
func (w *TextWidget) SetView(view *gocui.View) {
	if view != nil {
		view.Wrap = false
		view.FgColor = w.FgColor
		view.BgColor = w.BgColor
	}
	w.view = view
	w.render()
}

// SetPrompt focuses this widget and makes it editable.
func (w *TextWidget) SetPrompt(g *gocui.Gui, prefix, content string, callback func(bool, string)) error {
	if w.view == nil {
		return nil
	}

	gx := New(g)

	oldfocus := g.CurrentView()
	g.Cursor = true
	gx.Focus(w.view)

	w.SetText(prefix)
	fmt.Fprintf(w.view, content)

	editor := PromptEditor(g, len(prefix), func(success bool, response string) {
		w.setEditor(nil)
		gx.Focus(oldfocus)
		g.Cursor = false
		callback(success, response)
	})

	w.setEditor(editor)

	return w.view.SetCursor(len(prefix)+len(content), 0)
}

func (w *TextWidget) setEditor(e gocui.Editor) {
	if w.view != nil {
		if e == nil {
			w.view.Editable = false
		} else {
			w.view.Editable = true
		}
		w.view.Editor = e
	}
}

func (w *TextWidget) render() {
	if w.view != nil {
		w.view.Clear()
		fmt.Fprintf(w.view, w.text)
	}
}

// ScrollWidget provides vertical scrolling to other widgets.
type ScrollWidget struct {
	Highlight bool

	view    *gocui.View
	max     int
	current int
}

// View returns the gocui.View currently bound to this widget.
func (w *ScrollWidget) View() *gocui.View {
	return w.view
}

// SetView binds a gocui.View to this widget.
func (w *ScrollWidget) SetView(view *gocui.View) {
	if view != nil {
		view.Wrap = false
		view.Highlight = w.Highlight
		view.SelBgColor = gocui.ColorGreen
		view.SelFgColor = gocui.ColorBlack
	}
	w.view = view
}

// SetMax updates maximum number of lines for the widget.
func (w *ScrollWidget) SetMax(max int) {
	w.max = max
	w.clampCurrent()
}

// Current returns currently selected line.
func (w *ScrollWidget) Current() int {
	return w.current
}

// SetCurrent updates currently selected line.
func (w *ScrollWidget) SetCurrent(idx int) error {
	err := MoveLines(w.view, w.current, w.max, idx-w.current)
	w.current = GetLine(w.view)
	return err
}

func (w *ScrollWidget) clampCurrent() {
	if w.current < 0 {
		w.current = 0
	} else if w.current > 0 && w.current >= w.max {
		w.SetCurrent(w.max - 1)
	}
}

// HandleAction executes an action command.
func (w *ScrollWidget) HandleAction(action string) error {
	switch action {
	case ActionNextLine:
		return w.NextLine()
	case ActionNextPage:
		return w.NextPage()
	case ActionPreviousLine:
		return w.PreviousLine()
	case ActionPreviousPage:
		return w.PreviousPage()
	default:
		return UnknownAction()
	}
}

// PreviousLine selects the previous line.
func (w *ScrollWidget) PreviousLine() error {
	err := MoveLines(w.view, w.current, w.max, -1)
	w.current = GetLine(w.view)
	return err
}

// NextLine selects the next line.
func (w *ScrollWidget) NextLine() error {
	err := MoveLines(w.view, w.current, w.max, 1)
	w.current = GetLine(w.view)
	return err
}

// NextPage scrolls down one page.
func (w *ScrollWidget) NextPage() error {
	if w.view != nil {
		_, sy := w.view.Size()
		err := MoveLines(w.view, w.current, w.max, sy)
		w.current = GetLine(w.view)
		return err
	}
	return nil
}

// PreviousPage scrolls up one page.
func (w *ScrollWidget) PreviousPage() error {
	if w.view != nil {
		_, sy := w.view.Size()
		err := MoveLines(w.view, w.current, w.max, -sy)
		w.current = GetLine(w.view)
		return err
	}
	return nil
}

// A ListWidget displays a list of lines.
type ListWidget struct {
	Highlight bool

	base  ScrollWidget
	model []string
}

// View returns the gocui.View currently bound to this widget.
func (w *ListWidget) View() *gocui.View {
	return w.base.View()
}

// SetView binds a gocui.View to this widget.
func (w *ListWidget) SetView(view *gocui.View) {
	w.base.SetView(view)
	w.base.Highlight = w.Highlight
	w.render()
}

// SetModel updates the list of lines to display.
func (w *ListWidget) SetModel(model []string) {
	w.base.SetMax(len(model))
	w.model = model
	w.render()
}

// Current returns currently selected line.
func (w *ListWidget) Current() int {
	return w.base.Current()
}

// SetCurrent updates currently selected line.
func (w *ListWidget) SetCurrent(idx int) error {
	return w.base.SetCurrent(idx)
}

func (w *ListWidget) render() {
	view := w.base.View()

	if view != nil {
		view.Clear()
		sx, _ := view.Size()
		for i, line := range w.model {
			if i > 0 {
				fmt.Fprintf(view, "\n")
			}
			fmt.Fprintf(view, Pad(line, sx))
		}
	}
}

// HandleAction executes an action command.
func (w *ListWidget) HandleAction(action string) error {
	return w.base.HandleAction(action)
}

// GetLine returns currently selected line for a view (relative to origin).
func GetLine(view *gocui.View) int {
	if view != nil {
		_, oy := view.Origin()
		_, cy := view.Cursor()
		return oy + cy
	}
	return 0
}

// MoveLines changes selected line of a view.
func MoveLines(view *gocui.View, current, max, delta int) error {
	if view == nil {
		return nil
	}

	if delta < 0 {
		if current <= 0 {
			return Error("at top")
		}
		if current+delta < 0 {
			delta = -current
		}
	} else {
		fromBottom := max - current
		if fromBottom > 0 && delta > fromBottom {
			delta = fromBottom
		}
		if current+1 >= max {
			return Error("at bottom")
		}
		if current+delta >= max {
			delta = max - current - 1
		}
	}

	if view != nil && delta != 0 {
		_, sy := view.Size()
		ox, oy := view.Origin()
		cx, cy := view.Cursor()

		if delta < 0 {
			if cy+delta < 0 {
				odelta := maxInt(delta, -oy)
				if err := view.SetOrigin(ox, oy+odelta); err != nil {
					return err
				}
				delta -= odelta
			}
		} else {
			if cy+delta >= sy {
				my := max - sy
				if oy < my {
					odelta := minInt(delta, my-oy)
					if err := view.SetOrigin(ox, oy+odelta); err != nil {
						return err
					}
					delta -= odelta
				}
			}
		}

		if delta != 0 {
			if err := view.SetCursor(cx, cy+delta); err != nil {
				return err
			}
		}
	}

	return nil
}

// PromptEditor builds a gocui.Editor function letting user enter a line of text.
func PromptEditor(g *gocui.Gui, offset int, callback func(bool, string)) gocui.Editor {
	promptEditor := func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
		cx, _ := v.Cursor()
		cancel := false
		done := false
		consumed := true

		switch {
		case ch != 0 && mod == 0:
			v.EditWrite(ch)
		case key == gocui.KeySpace:
			v.EditWrite(' ')
		case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
			if cx > offset {
				v.EditDelete(true)
			} else {
				cancel = true
			}
		case key == gocui.KeyDelete:
			v.EditDelete(false)
		case key == gocui.KeyInsert:
			// v.Overwrite = !v.Overwrite
		case key == gocui.KeyEnter:
			// v.EditNewLine()
			done = true
		case key == gocui.KeyEsc || key == gocui.KeyCtrlG:
			cancel = true
		case key == gocui.KeyArrowDown:
			// v.MoveCursor(0, 1, false)
		case key == gocui.KeyArrowUp:
			// v.MoveCursor(0, -1, false)
		case key == gocui.KeyArrowLeft:
			if cx > offset {
				v.MoveCursor(-1, 0, false)
			}
		case key == gocui.KeyArrowRight:
			v.MoveCursor(1, 0, false)
		default:
			consumed = false
		}

		if done || cancel {
			content := strings.TrimSpace(getFirstLine(v.Buffer()))
			if offset > len(content) {
				offset = len(content)
			}
			callback(done, content[offset:])
		}

		return consumed
	}

	return gocui.EditorFunc(promptEditor)
}

func getFirstLine(s string) string {
	idx := strings.Index(s, "\n")

	if idx >= 0 {
		return s[:idx]
	}

	return s
}
