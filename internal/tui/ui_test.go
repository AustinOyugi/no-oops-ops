package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestWidgetsEscapeAndNavigation(t *testing.T) {
	table := tview.NewTable()
	tableRows(table, []string{"SERVICE"}, [][]string{{"api\x1b[2J\n[red]"}})
	if got := table.GetCell(1, 0).Text; strings.Contains(got, "\x1b") || strings.Contains(got, "\n") {
		t.Fatalf("unsafe cell %q", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := newDashboard(ctx, func(context.Context) ([]Row, error) { return nil, nil }, func(context.Context, string) ([]Task, error) { return nil, nil })
	d.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if d.app.GetFocus() != d.tasks {
		t.Fatal("Tab did not focus tasks")
	}
	d.instances = []Task{{ID: "full-task-id", Error: "exit 1"}}
	d.showDetail(1)
	if !strings.Contains(d.detail.GetText(false), "exit 1") {
		t.Fatal("missing error detail")
	}
}

func TestActionConfirmationCancelsWithoutExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDashboard(ctx, nil, nil)
	ran := false
	d.execution = &Execution{Run: func(context.Context, Action) error { ran = true; return nil }}
	modal := d.confirm(Action{Label: "Remove", Args: []string{"remove", "prod", "shop", "--service", "api"}})
	modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if ran || d.modal || d.app.GetFocus() != d.services {
		t.Fatal("Cancel executed command or failed to restore dashboard")
	}
}
