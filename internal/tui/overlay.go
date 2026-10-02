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
	background    tview.Primitive
	width, height int
	x, y, w, h    int
}

func (o *overlay) SetRect(x, y, w, h int)        { o.x, o.y, o.w, o.h = x, y, w, h }
func (o *overlay) GetRect() (int, int, int, int) { return o.x, o.y, o.w, o.h }
func (o *overlay) Draw(screen tcell.Screen) {
	w := min(o.width, max(1, o.w-4))
	h := min(o.height, max(1, o.h-2))
	if o.background != nil {
		o.background.SetRect(o.x, o.y, o.w, o.h)
		o.background.Draw(screen)
	}
	x, y := o.x+(o.w-w)/2, o.y+(o.h-h)/2
	style := tcell.StyleDefault.Background(panelColor).Foreground(tcell.ColorWhite)
	for row := y; row < y+h; row++ {
		for col := x; col < x+w; col++ {
			screen.SetContent(col, row, ' ', nil, style)
		}
	}
	o.Primitive.SetRect(o.x+(o.w-w)/2, o.y+(o.h-h)/2, w, h)
	o.Primitive.Draw(screen)
}
func (d *dashboard) showOverlay(content tview.Primitive, focus tview.Primitive, width, height int) {
	d.app.SetRoot(&overlay{Primitive: content, background: d.layout, width: width, height: height}, true).SetFocus(focus)
}
