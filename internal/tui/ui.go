package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

type dashboard struct {
	execution              *Execution
	modal                  bool
	app                    *tview.Application
	services, tasks        *tview.Table
	status, detail, footer *tview.TextView
	layout                 *tview.Flex
	rows                   []Row
	instances              []Task
	selected               string
	generation             int
	serviceBusy, taskBusy  bool
	taskCancel             context.CancelFunc
	ctx                    context.Context
	query                  func(context.Context) ([]Row, error)
	taskQuery              func(context.Context, string) ([]Task, error)
}

func newDashboard(ctx context.Context, query func(context.Context) ([]Row, error), taskQuery func(context.Context, string) ([]Task, error)) *dashboard {
	d := &dashboard{app: tview.NewApplication(), ctx: ctx, query: query, taskQuery: taskQuery}
	d.services = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	d.tasks = tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	d.services.SetBorder(true).SetTitle(" Services ")
	d.tasks.SetBorder(true).SetTitle(" Tasks ")
	d.status = tview.NewTextView().SetText("Loading services…")
	d.detail = tview.NewTextView().SetWrap(true)
	d.footer = tview.NewTextView().SetText("Tab pane · ↑/↓ select · q quit · refresh every 5s")
	d.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.status, 2, 0, false).AddItem(d.services, 0, 1, true).
		AddItem(d.tasks, 0, 1, false).AddItem(d.detail, 3, 0, false).AddItem(d.footer, 1, 0, false)
	d.services.SetSelectionChangedFunc(func(row, col int) {
		if row > 0 && row <= len(d.rows) && d.rows[row-1].Service != d.selected {
			d.loadTasks(d.rows[row-1].Service)
		}
	})
	d.tasks.SetSelectionChangedFunc(func(row, col int) { d.showDetail(row) })
	d.app.SetRoot(d.layout, true).SetFocus(d.services)
	d.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlC:
			d.app.Stop()
			return nil
		case d.modal:
			return event
		case event.Rune() == ':':
			d.palette()
			return nil
		case event.Rune() == 'e':
			d.actions()
			return nil
		case event.Rune() == 'q':
			d.app.Stop()
			return nil
		case event.Key() == tcell.KeyTab:
			if d.app.GetFocus() == d.services {
				d.app.SetFocus(d.tasks)
			} else {
				d.app.SetFocus(d.services)
			}
			return nil
		}
		return event
	})
	return d
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
func tableRows(table *tview.Table, headers []string, rows [][]string) {
	table.Clear()
	for col, text := range headers {
		table.SetCell(0, col, tview.NewTableCell(text).SetSelectable(false).SetTextColor(tcell.ColorYellow).SetExpansion(1))
	}
	for row, values := range rows {
		for col, text := range values {
			table.SetCell(row+1, col, tview.NewTableCell(tview.Escape(clean(text))).SetExpansion(1))
		}
	}
}
func (d *dashboard) showDetail(row int) {
	if row > 0 && row <= len(d.instances) {
		t := d.instances[row-1]
		d.detail.SetText("Task: " + clean(t.ID) + "\nError: " + clean(t.Error))
	} else {
		d.detail.SetText("")
	}
}
func (d *dashboard) refresh() {
	if !d.serviceBusy {
		d.serviceBusy = true
		go func() {
			ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
			defer cancel()
			rows, err := d.query(ctx)
			if d.ctx.Err() != nil {
				return
			}
			d.app.QueueUpdateDraw(func() {
				d.serviceBusy = false
				if err != nil {
					d.status.SetText("Refresh failed (previous results retained): " + clean(err.Error()))
					return
				}
				d.rows = rows
				values := make([][]string, 0, len(rows))
				selected := 1
				for i, r := range rows {
					values = append(values, []string{r.Environment, r.App, r.Service, r.Replicas, r.State})
					if r.Service == d.selected {
						selected = i + 1
					}
				}
				tableRows(d.services, []string{"ENV", "APP", "SERVICE", "REPLICAS", "STATE"}, values)
				d.status.SetText("Updated " + time.Now().Format("15:04:05"))
				if len(rows) == 0 {
					d.status.SetText("No managed services running in this workspace.")
					d.loadTasks("")
				} else {
					d.services.Select(selected, 0)
					if d.selected != rows[selected-1].Service {
						d.loadTasks(rows[selected-1].Service)
					}
				}
			})
		}()
	}
	if !d.taskBusy && d.selected != "" {
		d.loadTasks(d.selected)
	}
}
func (d *dashboard) loadTasks(service string) {
	if d.taskCancel != nil {
		d.taskCancel()
	}
	d.generation++
	generation := d.generation
	if d.selected != service {
		d.instances = nil
		d.tasks.Clear()
		d.detail.SetText("")
	}
	d.selected = service
	d.tasks.SetTitle(" Tasks · " + clean(service) + " ")
	if service == "" {
		d.taskBusy = false
		return
	}
	d.taskBusy = true
	ctx, cancel := context.WithTimeout(d.ctx, 10*time.Second)
	d.taskCancel = cancel
	go func() {
		defer cancel()
		tasks, err := d.taskQuery(ctx, service)
		if d.ctx.Err() != nil {
			return
		}
		d.app.QueueUpdateDraw(func() {
			if generation != d.generation {
				return
			}
			d.taskBusy = false
			if err != nil {
				d.tasks.SetTitle(" Tasks · refresh failed ")
				d.detail.SetText(clean(err.Error()))
				return
			}
			row, _ := d.tasks.GetSelection()
			id := ""
			if row > 0 && row <= len(d.instances) {
				id = d.instances[row-1].ID
			}
			d.instances = tasks
			values := make([][]string, 0, len(tasks))
			selected := 1
			for i, t := range tasks {
				values = append(values, []string{t.ID, t.Node, t.State, uptime(t, time.Now()), t.Error})
				if t.ID == id {
					selected = i + 1
				}
			}
			tableRows(d.tasks, []string{"TASK", "NODE", "STATE", "UPTIME", "ERROR"}, values)
			d.tasks.SetTitle(" Tasks · " + clean(service) + " ")
			if len(tasks) > 0 {
				d.tasks.Select(selected, 0)
				d.showDetail(selected)
			} else {
				d.detail.SetText("No tasks for this service.")
			}
		})
	}()
}

func Run(parent context.Context, in, out *os.File, query func(context.Context) ([]Row, error), taskQuery func(context.Context, string) ([]Task, error), execution ...Execution) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return fmt.Errorf("noops ui requires an interactive terminal")
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	d := newDashboard(ctx, query, taskQuery)
	if len(execution) > 0 {
		d.execution = &execution[0]
		d.footer.SetText(" : all commands · e service actions · Tab pane · ↑/↓ select · q quit · refresh every 5s")
	}
	defer func() {
		if d.taskCancel != nil {
			d.taskCancel()
		}
	}()
	var once sync.Once
	d.app.SetBeforeDrawFunc(func(tcell.Screen) bool {
		once.Do(func() {
			d.refresh()
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						d.app.Stop()
						return
					case <-ticker.C:
						d.app.QueueUpdateDraw(d.refresh)
					}
				}
			}()
		})
		return false
	})
	return d.app.Run()
}
