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
				if d.jobCancel != nil && d.jobRunning {
					d.jobCancel()
				}
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				d.app.SetFocus(d.services)
				return nil
			}
			return event
		})
		d.layout.RemoveItem(d.footer)
		d.layout.AddItem(d.output, 0, 1, false).AddItem(d.footer, 1, 0, false)
	}
	d.jobRunning = true
	d.outputFollow = true
	d.output.SetText(CommandText(action) + "\n").SetTitle(" Output · " + action.Label + " · running · p pause · x cancel ")
	d.app.SetFocus(d.output)
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
