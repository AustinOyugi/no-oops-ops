package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Field struct {
	Key, Label, Default string
	Boolean             bool
	Options             func(map[string]string) []string
	Disabled            func(map[string]string) bool
}
type Command struct {
	Label, Description string
	Fields             []Field
	Build              func(map[string]string) (Action, error)
}

func (d *dashboard) palette() {
	if d.execution == nil || d.execution.Palette == nil {
		return
	}
	var selected *Row
	row, _ := d.services.GetSelection()
	if row > 0 && row <= len(d.rows) {
		r := d.rows[row-1]
		selected = &r
	}
	commands, err := d.execution.Palette(selected)
	if err != nil {
		d.notice(err.Error())
		return
	}
	d.modal = true
	search := tview.NewInputField().SetLabel("Search: ")
	list := tview.NewList().ShowSecondaryText(true)
	update := func(text string) {
		list.Clear()
		for _, command := range commands {
			command := command
			if strings.Contains(strings.ToLower(command.Label+" "+command.Description), strings.ToLower(text)) {
				list.AddItem(command.Label, command.Description, 0, func() { d.commandForm(command) })
			}
		}
	}
	update("")
	search.SetChangedFunc(update)
	search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyDown || event.Key() == tcell.KeyTab || event.Key() == tcell.KeyEnter {
			if list.GetItemCount() > 0 {
				d.app.SetFocus(list)
			}
			return nil
		}
		return event
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			d.app.SetFocus(search)
			return nil
		}
		return event
	})
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 3, 0, true).AddItem(list, 0, 1, false)
	layout.SetBorder(true).SetTitle(" Commands · Tab switch · Esc close ")
	layout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			d.closeDialog()
			return nil
		}
		return event
	})
	d.app.SetRoot(layout, true).SetFocus(search)
}

func (d *dashboard) commandForm(command Command) {
	d.modal = true
	values := map[string]string{}
	for _, field := range command.Fields {
		values[field.Key] = field.Default
	}
	form := tview.NewForm()
	preview := tview.NewTextView().SetWrap(true)
	fields := map[string]tview.FormItem{}
	changing := true
	var refresh func()
	changed := func(key, value string) {
		values[key] = value
		if !changing {
			refresh()
		}
	}
	for _, field := range command.Fields {
		field := field
		if field.Boolean {
			form.AddCheckbox(field.Label, field.Default == "true", func(checked bool) {
				value := "false"
				if checked {
					value = "true"
				}
				changed(field.Key, value)
			})
		} else if field.Options != nil {
			options := field.Options(values)
			index := 0
			for i, value := range options {
				if value == values[field.Key] {
					index = i
				}
			}
			form.AddDropDown(field.Label, options, index, func(option string, _ int) { changed(field.Key, option) })
			if len(options) > 0 {
				values[field.Key] = options[index]
			}
		} else {
			form.AddInputField(field.Label, field.Default, 45, nil, func(value string) { changed(field.Key, value) })
		}
		fields[field.Key] = form.GetFormItem(form.GetFormItemCount() - 1)
	}
	form.AddButton("Cancel", d.closeDialog)
	form.AddButton("Run", func() {
		action, err := command.Build(values)
		if err != nil {
			preview.SetText("Cannot run: " + err.Error())
			return
		}
		d.confirm(action)
	})
	form.SetCancelFunc(d.closeDialog)
	form.SetBorder(true).SetTitle(" " + command.Label + " ")
	refresh = func() {
		changing = true
		defer func() { changing = false }()
		for _, field := range command.Fields {
			disabled := field.Disabled != nil && field.Disabled(values)
			if item, ok := fields[field.Key].(*tview.DropDown); ok {
				item.SetDisabled(disabled)
				if field.Options != nil {
					options := field.Options(values)
					index := 0
					for i, value := range options {
						if value == values[field.Key] {
							index = i
						}
					}
					item.SetOptions(options, func(option string, _ int) { changed(field.Key, option) })
					if len(options) > 0 {
						values[field.Key] = options[index]
						item.SetCurrentOption(index)
					} else {
						values[field.Key] = ""
					}
				}
			}
		}
		action, err := command.Build(values)
		form.GetButton(1).SetDisabled(err != nil)
		if err != nil {
			preview.SetText(fmt.Sprintf("%s\n\nCannot run: %s", command.Description, err))
		} else {
			preview.SetText(command.Description + "\n\n" + CommandText(action))
		}
	}
	changing = false
	refresh()
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(form, 0, 1, true).AddItem(preview, 5, 0, false)
	d.app.SetRoot(layout, true).SetFocus(form)
}
