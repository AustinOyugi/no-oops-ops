package tui

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTasksHistoryAndUptime(t *testing.T) {
	tasks, err := parseTasks(`{"ID":"old","Node":"node-1","CurrentState":"Failed 2 minutes ago","Error":"exit code 1"}
{"ID":"live","Node":"node-2","CurrentState":"Running 3 hours ago","Error":""}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != "live" || tasks[1].Error != "exit code 1" {
		t.Fatalf("tasks: %+v", tasks)
	}
	now := time.Now()
	tasks[0].StartedAt = now.Add(-52 * time.Hour)
	if got := uptime(tasks[0], now); got != "2d 4h" {
		t.Fatalf("uptime: %s", got)
	}
	tasks[1].StartedAt = now.Add(-time.Hour)
	if uptime(tasks[1], now) != "—" {
		t.Fatal("failed task shown with live uptime")
	}
	if _, err := parseTasks("broken"); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestTaskSelectionRefreshAndLayout(t *testing.T) {
	pane := taskPane{service: "api", tasks: []Task{{ID: "a"}, {ID: "b", Error: "failed"}}, selected: 1}
	pane.accept(taskSnapshot{tasks: []Task{{ID: "b", Error: "failed"}, {ID: "c"}}})
	if pane.selected != 0 || pane.tasks[pane.selected].ID != "b" {
		t.Fatal("selection lost")
	}
	pane.accept(taskSnapshot{err: errors.New("offline")})
	if len(pane.tasks) != 2 || !strings.Contains(pane.message, "offline") {
		t.Fatal("error discarded previous tasks")
	}
	pane.message = "Updated"
	screen := renderDashboard([]Row{{Service: "api"}}, 0, "Updated", pane, true, 80, 24)
	for _, text := range []string{"Services", "TASKS · api [active]", "UPTIME", "Error: failed", "Tab pane"} {
		if !strings.Contains(screen, text) {
			t.Fatalf("missing %q in %q", text, screen)
		}
	}
	small := renderDashboard(nil, 0, "Updated", pane, true, 80, 10)
	if !strings.Contains(small, "TASKS") || strings.Contains(small, "NOOPS") {
		t.Fatal("small terminal did not prioritize focused pane")
	}
	for _, d := range [][2]int{{0, 0}, {1, 1}, {10, 3}} {
		renderDashboard(nil, 0, "", pane, false, d[0], d[1])
	}
}
