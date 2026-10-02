package tui

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOutputBoundedAndSafe(t *testing.T) {
	b := &outputBuffer{}
	b.Write([]byte(strings.Repeat("a", outputLimit*2)))
	b.Write([]byte("tail"))
	text, _ := b.snapshot()
	if len(text) != outputLimit || !strings.HasSuffix(text, "tail") {
		t.Fatal("buffer not bounded or lost tail")
	}
	if got := outputText("\x1b[31mred\x1b[0m\nnext\x1b]0;title\x07"); got != "red\nnext" {
		t.Fatalf("unsafe output %q", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Write([]byte("line\n"))
				b.snapshot()
			}
		}()
	}
	wg.Wait()
}

func TestStreamCompletesInsideDashboard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDashboard(ctx, func(context.Context) ([]Row, error) { return nil, nil }, func(context.Context, string) ([]Task, error) { return nil, nil })
	d.execution = &Execution{Stream: func(ctx context.Context, action Action, w io.Writer) error {
		fmt.Fprintln(w, "first line")
		time.Sleep(150 * time.Millisecond)
		fmt.Fprintln(w, "last line")
		return nil
	}}
	d.startStream(Action{Label: "Version", Args: []string{"version"}})
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	d.app.SetScreen(screen)
	completed := make(chan string, 1)
	d.app.SetAfterDrawFunc(func(tcell.Screen) {
		if !d.jobRunning {
			select {
			case completed <- d.output.GetText(false):
			default:
			}
		}
	})
	done := make(chan error, 1)
	go func() { done <- d.app.Run() }()
	select {
	case text := <-completed:
		if !strings.Contains(text, "first line") || !strings.Contains(text, "last line") || !strings.Contains(text, "completed successfully") {
			t.Errorf("missing streamed output: %q", text)
		}
	case <-time.After(3 * time.Second):
		t.Error("stream did not finish")
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
