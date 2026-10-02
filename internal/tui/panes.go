package tui

import (
	"fmt"
	"strings"
	"time"
)

type taskPane struct {
	service  string
	tasks    []Task
	selected int
	message  string
}

func (p *taskPane) accept(result taskSnapshot) {
	if result.err != nil {
		p.message = "Task refresh failed (previous results retained): " + result.err.Error()
		return
	}
	id := ""
	if p.selected < len(p.tasks) {
		id = p.tasks[p.selected].ID
	}
	p.tasks = result.tasks
	p.selected = 0
	for i := range p.tasks {
		if p.tasks[i].ID == id {
			p.selected = i
			break
		}
	}
	p.message = "Updated " + time.Now().Format("15:04:05")
}

func renderDashboard(rows []Row, selected int, message string, pane taskPane, focus bool, width, height int) string {
	// On tiny terminals show the focused pane, keeping the keyboard controls visible.
	serviceHeight := height / 2
	if serviceHeight < 7 {
		serviceHeight = 7
	}
	showServices := height >= 16 || !focus
	showTasks := height >= 16 || focus
	var lines []string
	if showServices {
		h := serviceHeight
		if !showTasks {
			h = height
		}
		serviceScreen := render(rows, selected, message, width, h)
		// Reuse service table layout without terminal commands or its own footer.
		serviceScreen = strings.TrimPrefix(serviceScreen, "\x1b[H")
		serviceScreen = strings.TrimSuffix(serviceScreen, "\x1b[J")
		serviceScreen = strings.ReplaceAll(serviceScreen, "\x1b[K", "")
		serviceLines := strings.Split(serviceScreen, "\r\n")
		if len(serviceLines) >= 2 {
			serviceLines = serviceLines[:len(serviceLines)-2]
		}
		if !focus && len(serviceLines) > 0 {
			serviceLines[0] = "NOOPS · Services [active]"
		}
		lines = append(lines, serviceLines...)
	}
	if showTasks {
		if showServices {
			for len(lines) < serviceHeight-1 {
				lines = append(lines, "")
			}
			lines = append(lines, strings.Repeat("─", max(0, width-1)))
		}
		title := "TASKS · " + pane.service
		if focus {
			title += " [active]"
		}
		lines = append(lines, title, pane.message, fmt.Sprintf("  %-12s %-16s %-12s %-10s %s", "TASK", "NODE", "STATE", "UPTIME", "ERROR"))
		capacity := max(0, height-len(lines)-3)
		start := 0
		if capacity > 0 && pane.selected >= capacity {
			start = pane.selected - capacity + 1
		}
		if len(pane.tasks) == 0 && capacity > 0 && strings.HasPrefix(pane.message, "Updated") {
			lines = append(lines, "  No tasks for this service.")
		}
		for i := start; i < len(pane.tasks) && i < start+capacity; i++ {
			t := pane.tasks[i]
			marker := " "
			if i == pane.selected {
				marker = ">"
			}
			lines = append(lines, fmt.Sprintf("%s %-12s %-16s %-12s %-10s %s", marker, cell(t.ID, 12), cell(t.Node, 16), cell(t.State, 12), uptime(t, time.Now()), t.Error))
		}
		if pane.selected < len(pane.tasks) {
			lines = append(lines, "Task: "+pane.tasks[pane.selected].ID, "Error: "+pane.tasks[pane.selected].Error)
		}
	}
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	footer := "Tab pane · ↑/↓ select · q quit · refresh every 5s"
	if len(lines) >= height {
		lines = lines[:max(0, height-1)]
	}
	lines = append(lines, footer)
	var b strings.Builder
	b.WriteString("\x1b[H")
	for i, line := range lines {
		if width < 1 || i >= height {
			break
		}
		runes := []rune(clean(line))
		if len(runes) >= width {
			runes = runes[:width-1]
		}
		b.WriteString(string(runes))
		b.WriteString("\x1b[K")
		if i+1 < len(lines) && i+1 < height {
			b.WriteString("\r\n")
		}
	}
	b.WriteString("\x1b[J")
	return b.String()
}
