package local

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type nginxHealthRunner struct {
	status, health string
	replicas       int
	scaled         bool
}

func (r *nginxHealthRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	output := ""
	switch {
	case len(args) == 3 && args[1] == "inspect":
	case args[0] == "service" && args[1] == "scale":
		r.scaled = true
		r.replicas = 2
	case args[0] == "service" && args[1] == "inspect":
		output = fmt.Sprintf("%d|%s", r.replicas, r.status)
	case args[0] == "service" && args[1] == "ps":
		if !strings.Contains(strings.Join(args, " "), "desired-state=running") {
			return command.Result{}, fmt.Errorf("must exclude draining tasks")
		}
		output = "task1\ntask2"
	case args[0] == "inspect":
		output = "running|container"
	case args[0] == "container":
		output = r.health
	default:
		return command.Result{}, fmt.Errorf("unexpected command %v", args)
	}
	return command.Result{Output: []byte(output)}, nil
}
func TestNginxReadinessRequiresCompletedUpdateAndHealthyReplicas(t *testing.T) {
	for _, tc := range []struct {
		status, health string
		ready, failed  bool
	}{
		{"completed", "healthy", true, false}, {"updating", "healthy", false, false},
		{"completed", "starting", false, false}, {"completed", "unhealthy", false, false},
		{"paused", "healthy", false, true}, {"rollback_completed", "healthy", false, true},
	} {
		t.Run(tc.status+"/"+tc.health, func(t *testing.T) {
			h := &Host{runner: &nginxHealthRunner{status: tc.status, health: tc.health, replicas: 2}, nginxService: "nginx"}
			ready, err := h.inspectNginxHealthy(context.Background(), 2, true)
			if ready != tc.ready || (err != nil) != tc.failed {
				t.Fatalf("ready=%v error=%v", ready, err)
			}
		})
	}
}
func TestNginxUpgradePreparesRedundancyUsingExistingService(t *testing.T) {
	r := &nginxHealthRunner{replicas: 1, health: "healthy"}
	h := &Host{runner: r, nginxService: "nginx"}
	if err := h.prepareNginxUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !r.scaled {
		t.Fatal("existing singleton not scaled before replacement")
	}
}
