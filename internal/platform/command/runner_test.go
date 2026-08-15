package command

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRunnerLogsCommandFailureWithExitCodeAndOutput(t *testing.T) {
	var logs bytes.Buffer
	runner := NewRunner(slog.New(slog.NewTextHandler(&logs, nil)))

	_, err := runner.Run(
		context.Background(),
		"/bin/sh",
		[]string{"-c", "printf 'registry unavailable' >&2; exit 7"},
		RunOptions{LogCommand: true},
	)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	output := logs.String()
	for _, want := range []string{"level=ERROR", "msg=\"command failed\"", "exit_code=7", "registry unavailable"} {
		if !strings.Contains(output, want) {
			t.Errorf("logs = %q, want %q", output, want)
		}
	}
	if strings.Contains(output, "command completed") {
		t.Errorf("logs = %q, should not report command completion", output)
	}
}

func TestRunnerLogsCommandCompletionOnSuccess(t *testing.T) {
	var logs bytes.Buffer
	runner := NewRunner(slog.New(slog.NewTextHandler(&logs, nil)))

	_, err := runner.Run(context.Background(), "/bin/sh", []string{"-c", "exit 0"}, RunOptions{LogCommand: true})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := logs.String()
	if !strings.Contains(output, "level=INFO msg=\"command completed\"") {
		t.Errorf("logs = %q, want completion event", output)
	}
	if strings.Contains(output, "command failed") {
		t.Errorf("logs = %q, should not report command failure", output)
	}
}
