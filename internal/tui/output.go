package tui

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const outputLimit = 256 * 1024

type outputBuffer struct {
	sync.Mutex
	data     []byte
	revision uint64
}

func (b *outputBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	n := len(p)
	if len(p) >= outputLimit {
		b.data = append(b.data[:0], p[len(p)-outputLimit:]...)
	} else {
		excess := len(b.data) + len(p) - outputLimit
		if excess > 0 {
			copy(b.data, b.data[excess:])
			b.data = b.data[:len(b.data)-excess]
		}
		b.data = append(b.data, p...)
	}
	b.revision++
	return n, nil
}
func (b *outputBuffer) snapshot() (string, uint64) {
	b.Lock()
	defer b.Unlock()
	return string(b.data), b.revision
}

var terminalEscapes = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)

func outputText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, terminalEscapes.ReplaceAllString(s, ""))
}

func (d *dashboard) startStream(action Action) {
	if d.jobRunning {
		d.notice("A command is already running. Press o to view its output.")
		return
	}
	if d.output == nil {
		d.output = tview.NewTextView().SetScrollable(true).SetWrap(true)
		d.output.SetBorder(true)
		d.output.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Rune() {
			case 'p':
				d.outputFollow = !d.outputFollow
				return nil
			case 'x':
				d.cancelStream(false)
				return nil
			case 'X':
				d.cancelStream(true)
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				d.app.SetFocus(d.services)
				return nil
			}
			return event
		})

	}
	d.showOutput()
	d.keepKilledOutput = false
	d.jobRunning = true
	d.outputFollow = true
	d.output.SetText(CommandText(action) + "\n").SetTitle(" Output · " + action.Label + " · running · x kill & keep logs · X kill & close ")
	d.app.SetFocus(d.output)
	d.footer.SetText("x kill, keep logs · Shift+x kill, close view · p pause/follow · Tab panes")
	ctx, cancel := context.WithCancel(d.ctx)
	d.jobCancel = cancel
	buffer := &outputBuffer{}
	fmt.Fprintln(buffer, CommandText(action))
	done := make(chan error, 1)
	go func() { done <- d.execution.Stream(ctx, action, buffer) }()
	go func() {
		defer cancel()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		var revision uint64
		update := func(final bool, err error) {
			text, current := buffer.snapshot()
			if !final && current == revision {
				return
			}
			revision = current
			if final {
				if err != nil {
					text += fmt.Sprintf("\nCommand failed: %v\n", err)
				} else {
					text += "\nCommand completed successfully.\n"
				}
			}
			if d.ctx.Err() != nil {
				return
			}
			d.app.QueueUpdateDraw(func() {
				row, col := d.output.GetScrollOffset()
				d.output.SetText(outputText(text))
				if d.outputFollow {
					d.output.ScrollToEnd()
				} else {
					d.output.ScrollTo(row, col)
				}
				if final {
					d.jobRunning = false
					d.jobCancel = nil
					state := "completed"
					if err != nil {
						state = "failed"
					}
					d.output.SetTitle(" Output · " + action.Label + " · " + state + " · p follow · Esc services ")
					if !d.keepKilledOutput {
						d.hideOutput()
					}
					d.footer.SetText(action.Label + " " + state + " · o review output · r release · : commands")
					d.refresh()
				}
			})
		}
		for {
			select {
			case <-d.ctx.Done():
				return
			case err := <-done:
				update(true, err)
				return
			case <-ticker.C:
				update(false, nil)
			}
		}
	}()
}

var _ io.Writer = (*outputBuffer)(nil)

func (d *dashboard) showOutput() {
	if d.outputVisible || d.output == nil {
		return
	}
	d.outputVisible = true
	d.layout.RemoveItem(d.footer)
	d.layout.ResizeItem(d.services, 0, 2).ResizeItem(d.tasks, 0, 1).ResizeItem(d.detail, 2, 0)
	d.layout.AddItem(d.output, 0, 3, false).AddItem(d.footer, 1, 0, false)
}
func (d *dashboard) hideOutput() {
	if !d.outputVisible {
		return
	}
	d.outputVisible = false
	d.layout.RemoveItem(d.output)
	d.layout.ResizeItem(d.services, 0, 1).ResizeItem(d.tasks, 0, 1).ResizeItem(d.detail, 3, 0)
	if d.app.GetFocus() == d.output {
		d.app.SetFocus(d.services)
	}
}

func (d *dashboard) cancelStream(closeView bool) {
	if !d.jobRunning || d.jobCancel == nil {
		return
	}
	d.keepKilledOutput = !closeView
	d.jobCancel()
	if closeView {
		d.hideOutput()
	}
}
