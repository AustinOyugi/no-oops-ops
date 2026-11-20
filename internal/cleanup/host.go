package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"github.com/AustinOyugi/no-oops-ops/internal/storage"
	"os"
)

// Host cleanup never prunes volumes, networks or registry manifests. All
// recorded release images are retained, as well as Swarm current/rollback
// specs and non-terminal tasks, including services scaled to zero.
func (s *Service) runHost(ctx context.Context, options Options) (Plan, error) {
	if options.App != "" || options.Orphaned || options.ReleaseRetention {
		return Plan{}, fmt.Errorf("host cleanup cannot be scoped to app or orphaned releases")
	}
	if options.Apply {
		unlock, err := state.AcquireLock(ctx, filepath.Join(s.cfg.StateDir, "registry.lock"))
		if err != nil {
			return Plan{}, err
		}
		defer unlock()
	}
	plan, err := s.hostPlan(ctx)
	if err != nil || !options.Apply {
		return plan, err
	}
	if err := storage.PruneCache(ctx, s.runner, s.cfg.Storage); err != nil {
		return plan, err
	}
	for _, id := range plan.Containers {
		// Inspect again to avoid deleting a container that has since restarted.
		result, err := s.runner.Run(ctx, "docker", []string{"container", "inspect", "--format", "{{.State.Status}}|{{.State.ExitCode}}", id}, command.RunOptions{})
		if err != nil {
			return plan, fmt.Errorf("recheck failed container %s: %w", id, err)
		}
		if !failedContainer(strings.TrimSpace(string(result.Output))) {
			continue
		}
		if _, err := s.runner.Run(ctx, "docker", []string{"container", "rm", id}, command.RunOptions{}); err != nil {
			return plan, fmt.Errorf("remove failed container %s: %w", id, err)
		}
	}
	// Reinventory after container cleanup and immediately before image deletion.
	fresh, err := s.hostPlan(ctx)
	if err != nil {
		return plan, err
	}
	candidates := make(map[string]bool)
	for _, id := range fresh.LocalImages {
		candidates[id] = true
	}
	for _, id := range plan.LocalImages {
		if candidates[id] {
			if err := s.removeLocalImage(ctx, id); err != nil {
				return plan, err
			}
		}
	}
	return plan, nil
}
func failedContainer(value string) bool {
	status, code, ok := strings.Cut(value, "|")
	n, err := strconv.Atoi(code)
	return ok && err == nil && status == "exited" && n != 0
}
func (s *Service) hostPlan(ctx context.Context) (Plan, error) {
	live, err := s.liveServices(ctx)
	if err != nil {
		return Plan{}, err
	}
	refs := make(map[string]struct{})
	for image := range live.images {
		refs[image] = struct{}{}
	}
	// Protect durable releases even if Docker has lost every service spec.
	root := filepath.Join(s.cfg.StateDir, "apps")
	apps, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return Plan{}, err
	}
	for _, app := range apps {
		if !app.IsDir() {
			continue
		}
		envs, err := os.ReadDir(filepath.Join(root, app.Name()))
		if err != nil {
			return Plan{}, err
		}
		for _, env := range envs {
			if !env.IsDir() {
				continue
			}
			releases, err := release.ListHistory(s.cfg, app.Name(), env.Name())
			if err != nil {
				return Plan{}, err
			}
			for _, item := range releases {
				for _, ref := range []string{item.ImmutableImage(), item.RegistryImage, item.Image} {
					if ref != "" {
						refs[ref] = struct{}{}
					}
				}
			}
			deployments, _, err := readDeployments(filepath.Join(root, app.Name(), env.Name()))
			if err != nil {
				return Plan{}, err
			}
			for _, item := range deployments {
				if item.DeployedImage != "" {
					refs[item.DeployedImage] = struct{}{}
				}
				if item.ReleaseImage != "" {
					refs[item.ReleaseImage] = struct{}{}
				}
			}
		}
	}
	protected := make(map[string]bool)
	for ref := range refs {
		out, err := s.runner.Run(ctx, "docker", []string{"image", "inspect", "--format", "{{.Id}}", ref}, command.RunOptions{})
		if err != nil {
			if strings.Contains(string(out.Output), "No such image") || strings.Contains(string(out.Output), "No such object") {
				continue
			}
			return Plan{}, fmt.Errorf("resolve protected image %q: %w", ref, err)
		}
		id := strings.TrimSpace(string(out.Output))
		if id == "" {
			return Plan{}, fmt.Errorf("resolve protected image %q: empty image ID", ref)
		}
		protected[id] = true
	}
	out, err := s.runner.Run(ctx, "docker", []string{"container", "ls", "--all", "--quiet"}, command.RunOptions{})
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{BuildCache: true, Protected: len(protected)}
	for _, id := range strings.Fields(string(out.Output)) {
		detail, err := s.runner.Run(ctx, "docker", []string{"container", "inspect", "--format", "{{.Image}}|{{.State.Status}}|{{.State.ExitCode}}", id}, command.RunOptions{})
		if err != nil {
			return Plan{}, err
		}
		image, status, ok := strings.Cut(strings.TrimSpace(string(detail.Output)), "|")
		if !ok || image == "" {
			return Plan{}, fmt.Errorf("invalid container inventory for %s", id)
		}
		protected[image] = true
		if failedContainer(status) {
			plan.Containers = append(plan.Containers, id)
		}
	}
	out, err = s.runner.Run(ctx, "docker", []string{"image", "ls", "--no-trunc", "--format", "{{json .}}"}, command.RunOptions{})
	if err != nil {
		return Plan{}, err
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out.Output)), "\n") {
		if line == "" {
			continue
		}
		var item struct {
			ID string `json:"ID"`
		}
		if err := json.Unmarshal([]byte(line), &item); err != nil || item.ID == "" {
			return Plan{}, fmt.Errorf("invalid Docker image inventory: %q", line)
		}
		if !protected[item.ID] && !seen[item.ID] {
			plan.LocalImages = append(plan.LocalImages, item.ID)
			seen[item.ID] = true
		}
	}
	sort.Strings(plan.LocalImages)
	sort.Strings(plan.Containers)
	return plan, nil
}
