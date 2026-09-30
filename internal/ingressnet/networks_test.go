package ingressnet

import (
	"context"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"testing"
)

type runner struct{}

func (runner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	if args[0] == "service" {
		return command.Result{Output: []byte(`[{"Target":"prod-id"},{"Target":"canary-id"}]`)}, nil
	}
	return command.Result{Output: []byte("noops-" + args[len(args)-1])}, nil
}
func TestAttachedResolvesDockerNetworkIDs(t *testing.T) {
	got, err := Attached(context.Background(), runner{}, "nginx")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["noops-prod-id"] || !got["noops-canary-id"] {
		t.Fatalf("networks = %v", got)
	}
}
