package deploy

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAllDesiredTasksRunningRequiresAllDesiredTasks(t *testing.T) {
	if allDesiredTasksRunning(1, 3) {
		t.Error("1 running task should not satisfy 3 desired tasks")
	}
	if !allDesiredTasksRunning(3, 3) {
		t.Error("3 running tasks should satisfy 3 desired tasks")
	}
}

func TestParseServiceUpdateStatus(t *testing.T) {
	state, message, image := parseServiceUpdateStatus("rollback_completed|rollback completed|registry/sample:v1\n")
	if state != "rollback_completed" || message != "rollback completed" || image != "registry/sample:v1" {
		t.Errorf("parseServiceUpdateStatus() = (%q, %q, %q), want (%q, %q, %q)", state, message, image, "rollback_completed", "rollback completed", "registry/sample:v1")
	}
}

func TestParseTaskDiagnostics(t *testing.T) {
	diagnostics := parseTaskDiagnostics("abc123|node-1|Running|Rejected 4 seconds ago|No such image\n")
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(diagnostics))
	}

	got := diagnostics[0]
	if got.ID != "abc123" || got.Node != "node-1" || got.DesiredState != "Running" || got.CurrentState != "Rejected 4 seconds ago" || got.Error != "No such image" {
		t.Errorf("diagnostic = %#v", got)
	}
}

func TestRolloutMonitorAllowsFullMonitorAfterConvergence(t *testing.T) {
	started := time.Date(2026, time.September, 6, 13, 50, 40, 0, time.UTC)
	state := newRolloutMonitorState(started, 90*time.Second, 90*time.Second)

	if timedOut, monitoring, completed, _ := state.observe(started.Add(60*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("first convergence observation = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(90*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("at the old convergence deadline = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(149*time.Second), true); timedOut || !monitoring || completed {
		t.Fatalf("before the full monitor window = timedOut:%t monitoring:%t completed:%t, want false:true:false", timedOut, monitoring, completed)
	}
	if timedOut, monitoring, completed, _ := state.observe(started.Add(150*time.Second), true); timedOut || !monitoring || !completed {
		t.Fatalf("after the full monitor window = timedOut:%t monitoring:%t completed:%t, want false:true:true", timedOut, monitoring, completed)
	}
}

func TestRolloutMonitorTimesOutBeforeConvergence(t *testing.T) {
	started := time.Date(2026, time.September, 6, 13, 50, 40, 0, time.UTC)
	state := newRolloutMonitorState(started, 90*time.Second, 30*time.Second)

	if timedOut, _, _, _ := state.observe(started.Add(91*time.Second), false); !timedOut {
		t.Fatal("expected convergence timeout before any successful convergence")
	}
}

func TestRolloutMonitorFailsUnreadyServiceAtWindowEnd(t *testing.T) {
	start := time.Now()
	for _, lostAt := range []time.Duration{20 * time.Second, 70 * time.Second} {
		state := newRolloutMonitorState(start, 90*time.Second, time.Minute)
		state.observe(start.Add(10*time.Second), true)
		timedOut, _, completed, _ := state.observe(start.Add(lostAt), false)
		if lostAt < 70*time.Second && (timedOut || completed) {
			t.Fatal("monitor ended early")
		}
		timedOut, _, completed, _ = state.observe(start.Add(70*time.Second), false)
		if !timedOut || completed {
			t.Fatal("unready service must fail when monitor ends")
		}
	}
}

func TestRolloutMonitorRecoveryDoesNotExtendWindow(t *testing.T) {
	start := time.Now()
	state := newRolloutMonitorState(start, 90*time.Second, time.Minute)
	state.observe(start.Add(10*time.Second), true)
	state.observe(start.Add(20*time.Second), false)
	if timedOut, monitoring, completed, _ := state.observe(start.Add(60*time.Second), true); timedOut || !monitoring || completed {
		t.Fatal("monitor must continue until configured end")
	}
	if timedOut, _, completed, _ := state.observe(start.Add(70*time.Second), true); timedOut || !completed {
		t.Fatal("recovered service should pass at original monitor end")
	}
}

func fmtBool(v bool) string {
	if v {
		return "recovered"
	}
	return "never recovered"
}

func TestRolloutConvergenceDeadlineInclusive(t *testing.T) {
	start := time.Now()
	state := newRolloutMonitorState(start, time.Minute, time.Minute)
	if timedOut, _, _, _ := state.observe(start.Add(time.Minute), true); !timedOut {
		t.Fatal("late convergence must time out")
	}
}

func TestSwarmConvergedRequiresImageAndTasks(t *testing.T) {
	for _, state := range []string{"", "completed"} {
		if swarmConverged(state, "app:v1", "app:v1", 0, 1) {
			t.Fatal("missing tasks counted as converged")
		}
		if swarmConverged(state, "app:v10", "app:v1", 1, 1) {
			t.Fatal("wrong image counted as converged")
		}
		if !swarmConverged(state, "app:v1@sha256:abc", "app:v1", 1, 1) {
			t.Fatal("resolved image digest did not converge")
		}
		if !swarmConverged(state, "app:v1", "app:v1", 1, 1) {
			t.Fatal("healthy rollout did not converge")
		}
	}
	if swarmConverged("updating", "app:v1", "app:v1", 1, 1) {
		t.Fatal("active update counted as converged")
	}
}

func TestStopTimedOutRollout(t *testing.T) {
	for _, previous := range []string{"true", "false", "invalid"} {
		t.Run(previous, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "commands")
			script := "#!/bin/sh\nif [ \"$2\" = inspect ]; then echo " + previous + "; exit 0; fi\nprintf '%s\\n' \"$*\" > \"$COMMAND_LOG\"\n"
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("COMMAND_LOG", logPath)
			s := &Service{runner: command.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))}
			err := s.stopTimedOutRollout(context.Background(), "sample")
			if previous == "invalid" {
				if err == nil {
					t.Fatal("expected invalid inspect response error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			want := "service scale sample=0"
			if previous == "true" {
				want = "service update --detach --rollback sample"
			}
			if strings.TrimSpace(string(got)) != want {
				t.Fatalf("command = %q, want %q", got, want)
			}
		})
	}
}

func TestWaitForSwarmConvergence(t *testing.T) {
	for _, running := range []bool{true, false} {
		t.Run(fmtBool(running), func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\ncase \"$2\" in\ninspect) echo 'completed||app:v1';;\nps) "
			if running {
				script += "echo 'Running 1 second ago'"
			} else {
				script += "echo 'Starting 1 second ago'"
			}
			script += ";;\nesac\n"
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			s := &Service{runner: command.NewRunner(logger), logger: logger}
			outcome, count, err := s.waitForSwarmConvergence(context.Background(), "sample", "app:v1", 1, time.Second, 0)
			if running {
				if err != nil || outcome != SwarmOutcomeCompleted || count != 1 {
					t.Fatalf("outcome=%s count=%d err=%v", outcome, count, err)
				}
			} else if err == nil || outcome != SwarmOutcomeTimedOut {
				t.Fatalf("outcome=%s err=%v", outcome, err)
			}
		})
	}
}
