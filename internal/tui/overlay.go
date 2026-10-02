package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var panelColor = tcell.NewHexColor(0x111827)
var mutedColor = tcell.NewHexColor(0x94a3b8)
var accentColor = tcell.NewHexColor(0x67e8f9)

// Draw only the centered panel, leaving the dashboard visible around it.
type overlay struct {
	tview.Primitive
	width, height int
	x, y, w, h    int
}

func (o *overlay) SetRect(x, y, w, h int)        { o.x, o.y, o.w, o.h = x, y, w, h }
func (o *overlay) GetRect() (int, int, int, int) { return o.x, o.y, o.w, o.h }
func (o *overlay) Draw(screen tcell.Screen) {
	w := min(o.width, max(1, o.w-4))
	h := min(o.height, max(1, o.h-2))
	o.Primitive.SetRect(o.x+(o.w-w)/2, o.y+(o.h-h)/2, w, h)
	o.Primitive.Draw(screen)
}
func (d *dashboard) showOverlay(content tview.Primitive, focus tview.Primitive, width, height int) {
	pages := tview.NewPages().AddPage("dashboard", d.layout, true, true).
		AddPage("dialog", &overlay{Primitive: content, width: width, height: height}, true, true)
	d.app.SetRoot(pages, true).SetFocus(focus)
}
