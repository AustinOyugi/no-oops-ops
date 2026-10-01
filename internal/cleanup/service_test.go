package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

func TestPlanRetainsRollbackSafeHistory(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	for i, tag := range []string{"old", "previous", "active"} {
		created := time.Date(2026, 8, 1+i, 0, 0, 0, 0, time.UTC)
		writeJSON(t, filepath.Join(dir, "releases", tag+".json"), release.Metadata{App: "sample", Environment: "dev", Tag: tag, RegistryImage: "127.0.0.1:5000/sample:" + tag, CreateAt: created})
		writeJSON(t, filepath.Join(dir, "deployments", tag+".json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseTag: tag, ReleaseImage: "127.0.0.1:5000/sample:" + tag, Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: created})
	}
	svc := NewService(nil, config.Config{StateDir: state})
	plan, err := svc.plan(liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{}}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 1 || len(plan.DeploymentPaths) != 1 || len(plan.Images) != 1 || len(plan.LocalImages) != 1 {
		t.Fatalf("plan = %#v, want one old release, deployment, registry image, and local image", plan)
	}
	if plan.Images[0] != "127.0.0.1:5000/sample:old" {
		t.Errorf("image = %q", plan.Images[0])
	}
}

func TestPlanSelectsEntireOrphanedEnvironment(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:active"
	writeJSON(t, filepath.Join(dir, "releases", "active.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "active", RegistryImage: image, CreateAt: time.Now()})
	writeJSON(t, filepath.Join(dir, "deployments", "active.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "dev-sample_app", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{}}, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 1 || len(plan.DeploymentPaths) != 1 || len(plan.Images) != 0 || len(plan.LocalImages) != 0 {
		t.Fatalf("orphan plan = %#v, want only stale history selected", plan)
	}
}

func TestPlanProtectsLiveImageWithoutMatchingHistoricalServiceName(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:current"
	writeJSON(t, filepath.Join(dir, "releases", "current.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "current", RegistryImage: image, CreateAt: time.Now()})
	// The historical name is deliberately stale: image liveness is not derived
	// from it. A retained Swarm image must still be safe from cleanup.
	writeJSON(t, filepath.Join(dir, "deployments", "current.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "old-service-name", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	live := liveInventory{services: map[string]struct{}{"dev-sample_dev-sample": {}}, images: map[string]struct{}{imageKey(image + "@sha256:abc"): {}}}
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(live, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Images) != 0 || len(plan.ReleasePaths) != 0 {
		t.Fatalf("live image must not be selected: %#v", plan)
	}
}

func TestPlanOrphanedChecksServiceNameOnlyForHistoryCleanup(t *testing.T) {
	state := t.TempDir()
	dir := filepath.Join(state, "apps", "sample", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "deployments"), 0o700); err != nil {
		t.Fatal(err)
	}
	image := "127.0.0.1:5000/sample:current"
	writeJSON(t, filepath.Join(dir, "releases", "current.json"), release.Metadata{App: "sample", Environment: "dev", Tag: "current", RegistryImage: image, CreateAt: time.Now()})
	writeJSON(t, filepath.Join(dir, "deployments", "current.json"), deploy.Deployment{App: "sample", Environment: "dev", ReleaseImage: image, ServiceName: "dev-sample_dev-sample", Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Now()})
	live := liveInventory{services: map[string]struct{}{"dev-sample_dev-sample": {}}, images: map[string]struct{}{image: {}}}
	plan, err := NewService(nil, config.Config{StateDir: state}).plan(live, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 0 || len(plan.DeploymentPaths) != 0 || len(plan.Images) != 0 {
		t.Fatalf("running service must not be treated as orphaned: %#v", plan)
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAutomaticRetentionKeepsThreeBuildsAndLiveImagesWithinScope(t *testing.T) {
	root := t.TempDir()
	for _, scope := range [][2]string{{"api", "prod"}, {"api", "canary"}, {"worker", "prod"}} {
		dir := filepath.Join(root, "apps", scope[0], scope[1])
		for _, sub := range []string{"releases", "deployments"} {
			if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 6; i++ {
			tag := fmt.Sprintf("build-%d", i)
			image := fmt.Sprintf("127.0.0.1:5000/%s:%s-%s", scope[0], scope[1], tag)
			created := time.Date(2026, 9, i+1, 0, 0, 0, 0, time.UTC)
			writeJSON(t, filepath.Join(dir, "releases", tag+".json"), release.Metadata{App: scope[0], Environment: scope[1], Tag: tag, Image: scope[0] + ":" + scope[1] + "-" + tag, RegistryImage: image, CreateAt: created})
			writeJSON(t, filepath.Join(dir, "deployments", tag+".json"), deploy.Deployment{App: scope[0], Environment: scope[1], ReleaseTag: tag, ReleaseImage: image, Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: created})
		}
	}
	live := liveInventory{services: map[string]struct{}{}, images: map[string]struct{}{"127.0.0.1:5000/api:prod-build-0": {}}}
	svc := NewService(nil, config.Config{StateDir: root})
	plan, err := svc.planOptions(live, Options{Keep: 3, App: "api", Environment: "prod", ReleaseRetention: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 2 || len(plan.DeploymentPaths) != 2 || len(plan.Images) != 2 || len(plan.LocalImages) != 4 {
		t.Fatalf("unexpected retention plan: %#v", plan)
	}
	for _, path := range append(plan.ReleasePaths, plan.DeploymentPaths...) {
		if !strings.HasPrefix(path, filepath.Join(root, "apps", "api", "prod")+string(filepath.Separator)) {
			t.Fatalf("unrelated history selected: %s", path)
		}
		if !strings.Contains(path, "build-1.json") && !strings.Contains(path, "build-2.json") {
			t.Fatalf("retained build selected: %s", path)
		}
	}
}

func TestAutomaticRetentionDoesNotKeepExtraHistoricalDeployments(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "api", "prod")
	for _, sub := range []string{"releases", "deployments"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		tag := fmt.Sprintf("build-%d", i)
		image := "127.0.0.1:5000/api:" + tag
		writeJSON(t, filepath.Join(dir, "releases", tag+".json"), release.Metadata{App: "api", Environment: "prod", Tag: tag, RegistryImage: image, CreateAt: time.Unix(int64(i), 0)})
		if i < 3 {
			writeJSON(t, filepath.Join(dir, "deployments", tag+".json"), deploy.Deployment{ReleaseImage: image, Outcome: deploy.SwarmOutcomeCompleted, CreatedAt: time.Unix(int64(i), 0)})
		}
	}
	plan, err := NewService(nil, config.Config{StateDir: root}).planOptions(liveInventory{images: map[string]struct{}{}, services: map[string]struct{}{}}, Options{Keep: 3, App: "api", Environment: "prod", ReleaseRetention: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReleasePaths) != 3 || len(plan.DeploymentPaths) != 3 || len(plan.Images) != 3 {
		t.Fatalf("old deployments expanded build retention: %#v", plan)
	}
}

func TestPlanPrunesUnreferencedGeneratedStackArtifacts(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "api", "prod")
	for _, sub := range []string{"releases", "deployments"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	activeImage := "127.0.0.1:5000/api:active"
	oldImage := "127.0.0.1:5000/api:old"
	writeJSON(t, filepath.Join(dir, "releases", "active.json"), release.Metadata{Tag: "active", RegistryImage: activeImage, CreateAt: time.Unix(2, 0)})
	writeJSON(t, filepath.Join(dir, "releases", "old.json"), release.Metadata{Tag: "old", RegistryImage: oldImage, CreateAt: time.Unix(1, 0)})
	writeJSON(t, filepath.Join(dir, "deployments", "active.json"), deploy.Deployment{ReleaseImage: activeImage, StackName: "prod-api-ractive", CreatedAt: time.Unix(2, 0)})
	writeJSON(t, filepath.Join(dir, "deployments", "old.json"), deploy.Deployment{ReleaseImage: oldImage, StackName: "prod-api-rold", CreatedAt: time.Unix(1, 0)})
	for _, name := range []string{"stack-prod-api-ractive.yml", "stack-prod-api-rold.yml", "stack-prod-api-rorphan.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	plan, err := NewService(nil, config.Config{StateDir: root}).planOptions(liveInventory{images: map[string]struct{}{}, services: map[string]struct{}{}}, Options{Keep: 1, App: "api", Environment: "prod", ReleaseRetention: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.StackPaths) != 2 {
		t.Fatalf("stack artifacts = %v, want old and orphan candidates", plan.StackPaths)
	}
	for _, path := range plan.StackPaths {
		if strings.Contains(path, "ractive") {
			t.Fatalf("retained active stack artifact selected: %s", path)
		}
	}
}

func TestGarbageCollectionWaitsForRegistryRestart(t *testing.T) {
	r := &gcRunner{}
	s := NewService(nil, config.Config{RegistryName: "registry", StateDir: t.TempDir(), DataDir: t.TempDir()})
	s.runner = r
	if err := s.garbageCollect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !r.stopped || !r.collected || !r.restarted || !r.checked {
		t.Fatalf("registry lifecycle incomplete: %#v", r)
	}
}

type gcRunner struct {
	stopped, collected, restarted, checked bool
	failStop, failCollect                  bool
}

func (r *gcRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	switch {
	case args[0] == "service" && args[1] == "scale":
		if strings.HasSuffix(args[2], "=0") {
			r.stopped = true
			if r.failStop {
				return command.Result{}, fmt.Errorf("stop failed after acceptance")
			}
		} else {
			r.restarted = true
		}
	case args[0] == "run":
		if !r.stopped {
			return command.Result{}, fmt.Errorf("GC ran before stopping registry")
		}
		r.collected = true
		if r.failCollect {
			return command.Result{}, fmt.Errorf("GC failed")
		}
	case args[0] == "ps" && r.restarted:
		return command.Result{Output: []byte("registry-container")}, nil
	case args[0] == "exec":
		if !r.restarted {
			return command.Result{}, fmt.Errorf("readiness checked before restart")
		}
		r.checked = true
		return command.Result{Output: []byte("HTTP/1.1 200 OK\r\n\r\n{}")}, nil
	}
	return command.Result{}, nil
}

func TestGarbageCollectionRestartsRegistryOnFailure(t *testing.T) {
	for _, stopFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(stopFailure), func(t *testing.T) {
			r := &gcRunner{failStop: stopFailure, failCollect: !stopFailure}
			s := NewService(nil, config.Config{RegistryName: "registry", StateDir: t.TempDir(), DataDir: t.TempDir()})
			s.runner = r
			if err := s.garbageCollect(context.Background()); err == nil {
				t.Fatal("expected cleanup error")
			}
			if !r.restarted || !r.checked {
				t.Fatalf("registry not restored after failure: %#v", r)
			}
		})
	}
}
