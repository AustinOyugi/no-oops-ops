package tui

import (
	"context"
	"fmt"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"io"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Action struct {
	Label     string
	Args      []string
	HideTasks bool
}
type Execution struct {
	Rollouts  func(context.Context) ([]deploy.RolloutProgress, error)
	CanStream func(Action) bool
	Stream    func(context.Context, Action, io.Writer) error
	Palette   func(*Row) ([]Command, error)
	Resolve   func(Row) ([]Action, error)
	Run       func(context.Context, Action) error
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
	modal := tview.NewModal().SetBackgroundColor(panelColor).SetTextColor(tcell.ColorWhite).SetButtonBackgroundColor(tcell.NewHexColor(0x1f2937)).SetButtonTextColor(tcell.ColorWhite).SetText(tview.Escape(clean(text))).AddButtons([]string{"Close"}).SetDoneFunc(func(int, string) { d.closeDialog() })
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
	var forms []Command
	if d.execution.Palette != nil {
		forms, err = d.execution.Palette(&d.rows[row-1])
		if err != nil {
			d.notice(err.Error())
			return
		}
	}
	d.modal = true
	menu := tview.NewList().ShowSecondaryText(true).SetMainTextColor(tcell.ColorWhite).SetSecondaryTextColor(mutedColor).SetSelectedTextColor(tcell.ColorBlack).SetSelectedBackgroundColor(accentColor).SetHighlightFullLine(true)
	menu.SetBackgroundColor(panelColor)
	menu.SetBorder(true).SetTitle(" Service actions · Esc close ").SetTitleAlign(tview.AlignLeft).SetBorderColor(mutedColor).SetBorderPadding(1, 1, 2, 2)
	for _, action := range actions {
		action := action
		menu.AddItem(action.Label, CommandText(action), 0, func() {
			label := action.Label
			if label == "List releases" {
				label = "Release list"
			}
			if label == "Platform status" {
				label = "Status"
			}
			for _, form := range forms {
				if form.Label == label {
					d.commandForm(form)
					return
				}
			}
			d.confirm(action)
		})
	}
	menu.SetDoneFunc(d.closeDialog)
	d.showOverlay(menu, menu, 84, len(actions)*2+6)
}
func (d *dashboard) confirm(action Action) *tview.Modal {
	d.modal = true
	modal := tview.NewModal().SetBackgroundColor(panelColor).SetTextColor(tcell.ColorWhite).SetButtonBackgroundColor(tcell.NewHexColor(0x1f2937)).SetButtonTextColor(tcell.ColorWhite).SetText(tview.Escape("Run this command?\n\n" + CommandText(action))).AddButtons([]string{"Cancel", "Run"}).
		SetDoneFunc(func(_ int, button string) {
			d.closeDialog()
			if button != "Run" {
				return
			}
			if d.jobRunning {
				d.notice("A command is already running. Press o to view output.")
				return
			}
			if d.execution.Stream != nil && d.execution.CanStream != nil && d.execution.CanStream(action) {
				d.startStream(action)
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
