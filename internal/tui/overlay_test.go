package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
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
		for _, text := range []string{"Services", "Commands", "Deploy", "Version", "Enter open"} {
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
