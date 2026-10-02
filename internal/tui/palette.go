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
	search := tview.NewInputField().SetLabel("› ").SetPlaceholder("Type a command…").SetFieldWidth(0).
		SetLabelColor(accentColor).SetFieldBackgroundColor(panelColor).SetFieldTextColor(tcell.ColorWhite).SetPlaceholderTextColor(mutedColor)
	search.SetBackgroundColor(panelColor)
	list := tview.NewList().ShowSecondaryText(false).SetMainTextColor(tcell.ColorWhite).
		SetSelectedTextColor(tcell.ColorBlack).SetSelectedBackgroundColor(accentColor).SetHighlightFullLine(true)
	list.SetBackgroundColor(panelColor)
	detail := tview.NewTextView().SetTextColor(mutedColor).SetWrap(true)
	detail.SetBackgroundColor(panelColor)
	footer := tview.NewTextView().SetTextColor(mutedColor)
	footer.SetBackgroundColor(panelColor)
	var filtered []Command
	describe := func(index int) {
		if index >= 0 && index < len(filtered) {
			detail.SetText(filtered[index].Description)
		} else {
			detail.SetText("No matching commands. Try a different search.")
		}
	}
	list.SetChangedFunc(func(index int, _, _ string, _ rune) { describe(index) })
	update := func(text string) {
		list.Clear()
		filtered = nil
		for _, command := range commands {
			if strings.Contains(strings.ToLower(command.Label+" "+command.Description), strings.ToLower(text)) {
				command := command
				filtered = append(filtered, command)
				list.AddItem("  "+command.Label, "", 0, func() { d.commandForm(command) })
			}
		}
		describe(0)
		footer.SetText(fmt.Sprintf("%d commands   ↑↓ choose   Enter open   Esc close", len(filtered)))
	}
	update("")
	search.SetChangedFunc(update)
	search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyUp:
			if len(filtered) > 0 {
				index := list.GetCurrentItem()
				if event.Key() == tcell.KeyDown {
					index++
				} else {
					index--
				}
				list.SetCurrentItem((index + len(filtered)) % len(filtered))
				describe(list.GetCurrentItem())
			}
			return nil
		case tcell.KeyEnter:
			if len(filtered) > 0 {
				d.commandForm(filtered[list.GetCurrentItem()])
			}
			return nil
		case tcell.KeyTab:
			d.app.SetFocus(list)
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
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 2, 0, true).AddItem(list, 0, 1, false).AddItem(detail, 3, 0, false).AddItem(footer, 1, 0, false)
	layout.SetBorder(true).SetTitle(" Commands ").SetTitleAlign(tview.AlignLeft).SetBorderColor(mutedColor).SetBackgroundColor(panelColor).SetBorderPadding(1, 1, 2, 2)
	layout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			d.closeDialog()
			return nil
		}
		return event
	})
	d.showOverlay(layout, search, 76, 22)
}

func (d *dashboard) commandForm(command Command) {
	d.modal = true
	values := map[string]string{}
	for _, field := range command.Fields {
		values[field.Key] = field.Default
	}
	form := tview.NewForm().SetItemPadding(1).SetLabelColor(mutedColor).SetFieldBackgroundColor(tcell.NewHexColor(0x1f2937)).SetFieldTextColor(tcell.ColorWhite).SetButtonBackgroundColor(tcell.NewHexColor(0x1f2937)).SetButtonTextColor(tcell.ColorWhite).SetButtonsAlign(tview.AlignRight)
	form.SetBackgroundColor(panelColor)
	preview := tview.NewTextView().SetWrap(true).SetTextColor(accentColor)
	preview.SetBackgroundColor(panelColor)
	description := tview.NewTextView().SetText(command.Description).SetTextColor(mutedColor).SetWrap(true)
	description.SetBackgroundColor(panelColor)
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
			form.AddInputField(field.Label, field.Default, 0, nil, func(value string) { changed(field.Key, value) })
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
	form.SetBorderPadding(0, 0, 0, 0)
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
			preview.SetText("Complete the form\n" + err.Error())
		} else {
			preview.SetText("Command preview\n" + CommandText(action))
		}
	}
	changing = false
	refresh()
	height := len(command.Fields)*2 + 15
	layout := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(description, 3, 0, false).AddItem(form, 0, 1, true).AddItem(preview, 5, 0, false)
	layout.SetBorder(true).SetTitle(" "+command.Label+" ").SetTitleAlign(tview.AlignLeft).SetBorderColor(mutedColor).SetBackgroundColor(panelColor).SetBorderPadding(1, 1, 2, 2)
	d.showOverlay(layout, form, 84, height)
}
