package storage

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"gopkg.in/yaml.v3"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

func TestThresholds(t *testing.T) {
	p := Policy{MinFreeBytes: 5, WarnFreeBytes: 10}
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	if err := Check(p, Usage{Path: "/docker", Free: 4}, logger); err == nil {
		t.Fatal("low space admitted")
	}
	if err := Check(p, Usage{Free: 5}, logger); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "storage is low") {
		t.Fatal("warning not emitted")
	}
	if err := Check(p, Usage{Free: 10}, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
}
func TestPolicyValidation(t *testing.T) {
	for _, value := range []string{"0", "-1m", "1mm", "oops", "1;echo"} {
		p := Policy{}.Defaults()
		p.LogMaxSize = value
		if p.Validate() == nil {
			t.Errorf("accepted %q", value)
		}
	}
	p := Policy{}.Defaults()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.WarnFreeBytes = p.MinFreeBytes - 1
	if p.Validate() == nil {
		t.Fatal("accepted warning below minimum")
	}
}

type testRunner struct {
	args   []string
	output string
	err    error
}

func (r *testRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	r.args = args
	if len(args) > 0 && args[0] == "context" {
		return command.Result{Output: []byte("unix:///var/run/docker.sock")}, r.err
	}
	return command.Result{Output: []byte(r.output)}, r.err
}
func TestCachePruneUsesOnlyBuilder(t *testing.T) {
	r := &testRunner{}
	if err := PruneCache(context.Background(), r, Policy{BuildCacheBytes: 123}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.args, []string{"builder", "prune", "--force", "--all", "--keep-storage", "123"}) {
		t.Fatal(r.args)
	}
	r.err = errors.New("offline")
	if PruneCache(context.Background(), r, Policy{BuildCacheBytes: 1}) == nil {
		t.Fatal("error lost")
	}
}
func TestInspectRejectsUnknownStorage(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	t.Setenv("DOCKER_CONTEXT", "")
	for _, output := range []string{"", "relative", "/noops-nonexistent-disk"} {
		if _, err := Inspect(context.Background(), &testRunner{output: output}); err == nil {
			t.Fatalf("accepted %q", output)
		}
	}
}
func TestBoundLogsPreservesExplicitConfiguration(t *testing.T) {
	output, err := BoundLogs([]byte("services:\n  app:\n    image: demo\n  custom:\n    image: demo\n    logging:\n      driver: syslog\n"), Policy{}.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Services map[string]struct {
			Logging struct {
				Driver  string
				Options map[string]string
			}
		}
	}
	if err := yaml.Unmarshal(output, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Services["app"].Logging.Options["max-size"] != "10m" || doc.Services["app"].Logging.Options["max-file"] != "3" {
		t.Fatal(string(output))
	}
	if doc.Services["custom"].Logging.Driver != "syslog" {
		t.Fatal("custom logs replaced")
	}
}

func TestBuildRefusesAdmissionBeforeRunning(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	t.Setenv("DOCKER_CONTEXT", "")
	called := false
	r := &testRunner{output: t.TempDir()}
	err := Build(context.Background(), r, Policy{MinFreeBytes: ^uint64(0)}, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("run=%v err=%v", called, err)
	}
}
func TestInspectRejectsRemoteDocker(t *testing.T) {
	t.Setenv("DOCKER_HOST", "ssh://remote")
	t.Setenv("DOCKER_CONTEXT", "")
	if _, err := Inspect(context.Background(), &testRunner{output: t.TempDir()}); err == nil {
		t.Fatal("measured local disk for remote daemon")
	}
}
