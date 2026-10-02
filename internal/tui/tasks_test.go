package tui

import (
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
