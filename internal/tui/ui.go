package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

type taskSnapshot struct {
	service    string
	generation int
	tasks      []Task
	err        error
}

type snapshot struct {
	rows []Row
	err  error
}

// Run owns the terminal only for the duration of the dashboard. Docker reads
// run separately, with a timeout, so navigation and cancellation stay responsive.
func Run(parent context.Context, in, out *os.File, query func(context.Context) ([]Row, error), taskQuery func(context.Context, string) ([]Task, error)) error {
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return fmt.Errorf("noops ui requires an interactive terminal")
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	old, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(in.Fd()), old)
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
	results := make(chan snapshot, 1)
	busy := false
	refresh := func() {
		if busy {
			return
		}
		busy = true
		go func() {
			readCtx, done := context.WithTimeout(ctx, 10*time.Second)
			defer done()
			rows, err := query(readCtx)
			select {
			case results <- snapshot{rows, err}:
			case <-ctx.Done():
			}
		}()
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	refresh()
	var rows []Row
	selected := 0
	message := "Loading services…"
	escape := ""
	taskResults := make(chan taskSnapshot, 1)
	pane := taskPane{message: "Select a service"}
	taskFocus := false
	generation := 0
	taskBusy := false
	var taskCancel context.CancelFunc
	defer func() {
		if taskCancel != nil {
			taskCancel()
		}
	}()
	loadTasks := func(service string) {
		if taskCancel != nil {
			taskCancel()
		}
		generation++
		current := generation
		if pane.service != service {
			pane = taskPane{service: service, message: "Loading tasks…"}
		}
		if service == "" {
			taskBusy = false
			pane = taskPane{message: "Select a service"}
			return
		}
		taskBusy = true
		readCtx, done := context.WithTimeout(ctx, 10*time.Second)
		taskCancel = done
		go func() {
			defer done()
			tasks, err := taskQuery(readCtx, service)
			select {
			case taskResults <- taskSnapshot{service, current, tasks, err}:
			case <-ctx.Done():
			}
		}()
	}
	selectedService := func() string {
		if selected < len(rows) {
			return rows[selected].Service
		}
		return ""
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			refresh()
			if !taskBusy {
				loadTasks(selectedService())
			}
		case result := <-taskResults:
			if result.generation == generation && result.service == pane.service {
				taskBusy = false
				pane.accept(result)
			}
		case result := <-results:
			busy = false
			if result.err != nil {
				message = "Refresh failed (previous results retained): " + result.err.Error()
			} else {
				id := ""
				if selected < len(rows) {
					id = rows[selected].ID
				}
				rows = result.rows
				selected = 0
				for i := range rows {
					if rows[i].ID == id {
						selected = i
						break
					}
				}
				message = "Updated " + time.Now().Format("15:04:05")
				if pane.service != selectedService() {
					loadTasks(selectedService())
				}
			}
		default:
		}
		width, height, err := term.GetSize(int(out.Fd()))
		if err != nil {
			return err
		}
		fmt.Fprint(out, renderDashboard(rows, selected, message, pane, taskFocus, width, height))
		fds := []unix.PollFd{{Fd: int32(in.Fd()), Events: unix.POLLIN}}
		_, err = unix.Poll(fds, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return nil
		}
		if fds[0].Revents&unix.POLLIN == 0 {
			continue
		}
		var buf [32]byte
		n, err := in.Read(buf[:])
		if err != nil {
			return err
		}
		if n == 0 {
			return nil
		}
		for _, key := range buf[:n] {
			if key == 'q' || key == 3 {
				return nil
			}
			if key == '\t' {
				taskFocus = !taskFocus
				escape = ""
				continue
			}
			if key == 27 {
				escape = "\x1b"
				continue
			}
			if escape == "\x1b" && key == '[' {
				escape += "["
				continue
			}
			if escape == "\x1b[" {
				if taskFocus {
					if key == 'A' && pane.selected > 0 {
						pane.selected--
					}
					if key == 'B' && pane.selected+1 < len(pane.tasks) {
						pane.selected++
					}
				} else {
					previous := selectedService()
					if key == 'A' && selected > 0 {
						selected--
					}
					if key == 'B' && selected+1 < len(rows) {
						selected++
					}
					if previous != selectedService() {
						loadTasks(selectedService())
					}
				}
			}
			escape = ""
		}
	}
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
func render(rows []Row, selected int, message string, width, height int) string {
	if width < 1 || height < 1 {
		return ""
	}
	serviceWidth := width - 55
	if serviceWidth < 8 {
		serviceWidth = 8
	}
	formatRow := func(marker, env, app, service, replicas, state string) string {
		return fmt.Sprintf("%s %-10s %-18s %-*s %-9s %s", marker, cell(env, 10), cell(app, 18), serviceWidth, cell(service, serviceWidth), cell(replicas, 9), state)
	}
	var lines []string
	lines = append(lines, "NOOPS · Services", message, "", formatRow(" ", "ENV", "APP", "SERVICE", "REPLICAS", "STATE"))
	capacity := height - 6
	if capacity < 0 {
		capacity = 0
	}
	start := 0
	if selected >= capacity && capacity > 0 {
		start = selected - capacity + 1
	}
	if len(rows) == 0 && capacity > 0 && strings.HasPrefix(message, "Updated") {
		lines = append(lines, "  No managed services running in this workspace.")
	}
	for i := start; i < len(rows) && i < start+capacity; i++ {
		r := rows[i]
		marker := " "
		if i == selected {
			marker = ">"
		}
		lines = append(lines, formatRow(marker, r.Environment, r.App, r.Service, r.Replicas, r.State))
	}
	lines = append(lines, "", "↑/↓ select · q quit · refresh every 5s")
	var b strings.Builder
	b.WriteString("\x1b[H")
	for i, line := range lines {
		if i >= height {
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

func cell(value string, width int) string {
	runes := []rune(clean(value))
	if len(runes) > width {
		return string(runes[:width-1]) + "…"
	}
	return string(runes)
}
