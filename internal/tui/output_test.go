package tui

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
			if d.outputVisible {
				t.Error("completed output pane still visible")
			}
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

func TestCancelKeepOutputOrClose(t *testing.T) {
	for _, closeView := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		d := newDashboard(ctx, nil, nil)
		d.output = tview.NewTextView()
		d.showOutput()
		d.app.SetFocus(d.output)
		jobCtx, jobCancel := context.WithCancel(ctx)
		d.jobCancel = jobCancel
		d.jobRunning = true
		d.cancelStream(closeView)
		if jobCtx.Err() == nil {
			t.Fatal("command not canceled")
		}
		if d.outputVisible == closeView {
			t.Fatalf("close=%v visible=%v", closeView, d.outputVisible)
		}
		if d.keepKilledOutput == closeView {
			t.Fatal("wrong output retention")
		}
		cancel()
	}
}

func TestLogsHideTasksUntilStreamStops(t *testing.T) {
	for _, closeView := range []bool{false, true} {
		t.Run(fmt.Sprint(closeView), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d := newDashboard(ctx, func(context.Context) ([]Row, error) { return nil, nil }, func(context.Context, string) ([]Task, error) { return nil, nil })
			d.execution = &Execution{Stream: func(ctx context.Context, _ Action, w io.Writer) error {
				fmt.Fprintln(w, "live service log")
				<-ctx.Done()
				return nil
			}}
			d.startStream(Action{Label: "Logs", Args: []string{"logs", "prod", "api"}, HideTasks: true})
			hasPane := func(p tview.Primitive) bool {
				for i := 0; i < d.layout.GetItemCount(); i++ {
					if d.layout.GetItem(i) == p {
						return true
					}
				}
				return false
			}
			if hasPane(d.tasks) || hasPane(d.detail) || !hasPane(d.output) {
				t.Fatal("logs did not replace task section")
			}
			d.rollouts = []deploy.RolloutProgress{{App: "api"}}
			d.updateRolloutPane()
			if hasPane(d.tasks) {
				t.Fatal("rollout restored hidden tasks")
			}
			d.app.SetFocus(d.services)
			d.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
			if d.app.GetFocus() != d.output {
				t.Fatal("Tab focused hidden task pane")
			}
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			d.app.SetScreen(screen)
			completed := make(chan bool, 1)
			stopped := false
			d.app.SetAfterDrawFunc(func(tcell.Screen) {
				if !stopped && strings.Contains(d.output.GetText(false), "live service log") {
					stopped = true
					go d.app.QueueUpdateDraw(func() { d.cancelStream(closeView) })
				}
				if !d.jobRunning {
					select {
					case completed <- hasPane(d.tasks) && hasPane(d.detail) && d.outputVisible == !closeView:
					default:
					}
				}
			})
			done := make(chan error, 1)
			go func() { done <- d.app.Run() }()
			select {
			case restored := <-completed:
				if !restored {
					t.Error("tasks not restored or incorrect retained output")
				}
			case <-time.After(3 * time.Second):
				t.Error("stream did not stop")
			}
			d.app.Stop()
			<-done
		})
	}
}
