package tui

import (
	"context"
	"fmt"
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

func TestPaletteAvailableWithoutSelectionAndInvalidForm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDashboard(ctx, nil, nil)
	called := false
	d.execution = &Execution{Palette: func(row *Row) ([]Command, error) {
		called = true
		if row != nil {
			t.Fatal("unexpected selection")
		}
		return []Command{}, nil
	}}
	d.palette()
	if !called || !d.modal {
		t.Fatal("palette unavailable without services")
	}
	d.closeDialog()
	d.commandForm(Command{Label: "Required field", Fields: []Field{{Key: "name", Label: "Name"}}, Build: func(values map[string]string) (Action, error) {
		if values["name"] == "" {
			return Action{}, fmt.Errorf("name required")
		}
		return Action{Args: []string{"secret", "list", values["name"]}}, nil
	}})
	layout, ok := d.app.GetFocus().(*tview.InputField)
	if !ok {
		t.Fatalf("form did not focus input: %T", d.app.GetFocus())
	}
	layout.SetText("prod")
	layout.SetText("")
}

func TestNewReleaseAvailableWithoutRunningService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := newDashboard(ctx, nil, nil)
	d.execution = &Execution{Palette: func(row *Row) ([]Command, error) {
		if row != nil {
			t.Fatal("unexpected running service")
		}
		return []Command{{Label: "Release", Build: func(map[string]string) (Action, error) {
			return Action{Label: "Release", Args: []string{"release", "prod", "new-app", "--service", "api"}}, nil
		}}}, nil
	}}
	d.newRelease()
	if !d.modal {
		t.Fatal("new release form not opened without running services")
	}
}
