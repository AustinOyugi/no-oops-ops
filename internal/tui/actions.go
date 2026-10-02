package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
)

type Action struct {
	Label string
	Args  []string
}
type Execution struct {
	Resolve func(Row) ([]Action, error)
	Run     func(context.Context, Action) error
}

func CommandText(action Action) string {
	words := []string{"noops"}
	for _, arg := range action.Args {
		if strings.ContainsAny(arg, " \t\n\"'`$;\\") {
			arg = strconv.Quote(arg)
		}
		words = append(words, arg)
	}
	return strings.Join(words, " ")
}

func (d *dashboard) closeDialog() {
	d.modal = false
	d.app.SetRoot(d.layout, true).SetFocus(d.services)
}
func (d *dashboard) notice(text string) {
	d.modal = true
	modal := tview.NewModal().SetText(tview.Escape(clean(text))).AddButtons([]string{"Close"}).SetDoneFunc(func(int, string) { d.closeDialog() })
	d.app.SetRoot(modal, true).SetFocus(modal)
}
func (d *dashboard) actions() {
	if d.execution == nil {
		return
	}
	row, _ := d.services.GetSelection()
	if row < 1 || row > len(d.rows) {
		d.notice("Select a service first.")
		return
	}
	actions, err := d.execution.Resolve(d.rows[row-1])
	if err != nil {
		d.notice(err.Error())
		return
	}
	d.modal = true
	menu := tview.NewList().ShowSecondaryText(true)
	menu.SetBorder(true).SetTitle(" Noops commands · Esc to cancel ")
	for _, action := range actions {
		action := action
		menu.AddItem(action.Label, CommandText(action), 0, func() { d.confirm(action) })
	}
	menu.SetDoneFunc(d.closeDialog)
	d.app.SetRoot(menu, true).SetFocus(menu)
}
func (d *dashboard) confirm(action Action) *tview.Modal {
	d.modal = true
	modal := tview.NewModal().SetText(tview.Escape("Run this command?\n\n" + CommandText(action))).AddButtons([]string{"Cancel", "Run"}).
		SetDoneFunc(func(_ int, button string) {
			d.closeDialog()
			if button != "Run" {
				return
			}
			var err error
			if !d.app.Suspend(func() { err = d.execution.Run(d.ctx, action) }) {
				d.notice("Unable to suspend dashboard for command execution.")
				return
			}
			d.refresh()
			if err != nil {
				d.notice(fmt.Sprintf("%s failed: %v", action.Label, err))
			} else {
				d.notice(action.Label + " completed successfully.")
			}
		})
	d.app.SetRoot(modal, true).SetFocus(modal)
	return modal
}
