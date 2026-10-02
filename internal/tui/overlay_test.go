package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestPaletteCenteredWithDashboardVisible(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDashboard(ctx, nil, nil)
	d.execution = &Execution{Palette: func(*Row) ([]Command, error) {
		return []Command{{Label: "Deploy", Description: "Deploy the selected service"}, {Label: "Version", Description: "Print version"}}, nil
	}}
	d.palette()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 36)
	d.app.SetScreen(screen)
	frames := make(chan string, 1)
	d.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		var b strings.Builder
		for y := 0; y < 36; y++ {
			for x := 0; x < 100; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				b.WriteRune(r)
			}
			b.WriteByte('\n')
		}
		select {
		case frames <- b.String():
		default:
		}
	})
	done := make(chan error, 1)
	go func() { done <- d.app.Run() }()
	select {
	case frame := <-frames:
		for _, text := range []string{"Loading services", "Commands", "Deploy", "Version", "Enter open"} {
			if !strings.Contains(frame, text) {
				t.Errorf("missing %s in frame:\n%s", text, frame)
			}
		}
	case <-time.After(3 * time.Second):
		t.Error("no frame drawn")
	}
	d.app.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("dashboard did not exit")
	}
}

func TestOverlayButtonsKeyboardAndMouse(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 36)
	ran := false
	form := tview.NewForm().AddButton("Cancel", func() {}).AddButton("Run", func() { ran = true })
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(form, 0, 1, true)
	background := tview.NewTextView().SetText(strings.Repeat("BACKGROUND", 1000))
	root := &overlay{Primitive: layout, background: background, width: 60, height: 10}
	root.SetRect(0, 0, 100, 36)
	var focus func(tview.Primitive)
	var focused tview.Primitive
	focus = func(p tview.Primitive) {
		if focused != nil {
			focused.Blur()
		}
		focused = p
		p.Focus(focus)
	}
	focus(form)
	root.Draw(screen)
	root.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), focus)
	if !form.GetButton(1).HasFocus() {
		t.Fatal("Tab did not select Run")
	}
	root.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	if !ran {
		t.Fatal("Enter did not activate Run")
	}
	ran = false
	x, y, _, _ := form.GetButton(1).GetRect()
	root.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone), focus)
	if !ran {
		t.Fatal("click did not activate Run")
	}
	r, _, _, _ := screen.GetContent(75, 20)
	if r != ' ' {
		t.Fatalf("background leaked through panel: %q", r)
	}
}
