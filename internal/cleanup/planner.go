package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

// plan deliberately uses image references, not historical service names, to
// determine image liveness. A service name is reused as an app is upgraded,
// so it cannot identify a particular deployed version.
func (s *Service) plan(live liveInventory, keep int, orphanedOnly bool) (Plan, error) {
	protected := make(map[string]struct{}, len(live.images))
	protectedImages := make(map[string]struct{}, len(live.images))
	for image := range live.images {
		protected[imageKey(image)] = struct{}{}
		protectedImages[image] = struct{}{}
	}
	var plan Plan
	root := filepath.Join(s.cfg.StateDir, "apps")
	apps, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return plan, nil
	}
	if err != nil {
		return plan, err
	}
	for _, app := range apps {
		if !app.IsDir() {
			continue
		}
		envs, err := os.ReadDir(filepath.Join(root, app.Name()))
		if err != nil {
			return plan, err
		}
		for _, env := range envs {
			if !env.IsDir() {
				continue
			}
			dir := filepath.Join(root, app.Name(), env.Name())
			releases, err := release.ListHistory(s.cfg, app.Name(), env.Name())
			if err != nil {
				return plan, err
			}
			sort.Slice(releases, func(i, j int) bool { return releases[i].CreateAt.After(releases[j].CreateAt) })
			deployments, paths, err := readDeployments(dir)
			if err != nil {
				return plan, err
			}
			orphaned := orphanedOnly && !hasLiveDeployment(deployments, live.services)
			if !orphaned {
				for i, item := range releases {
					if i < keep {
						protected[imageKey(item.RegistryImage)] = struct{}{}
						protectedImages[item.RegistryImage] = struct{}{}
					}
				}
			}
			success := successfulDeployments(deployments)
			for i, d := range success {
				if !orphaned && i < keep {
					protected[imageKey(d.ReleaseImage)] = struct{}{}
					protectedImages[d.ReleaseImage] = struct{}{}
				}
			}
			for _, item := range releases {
				_, protectedImage := protected[imageKey(item.RegistryImage)]
				if orphaned || !protectedImage {
					plan.ReleasePaths = append(plan.ReleasePaths, filepath.Join(dir, "releases", item.Tag+".json"))
					if !orphaned && !protectedImage && item.RegistryImage != "" {
						plan.Images = append(plan.Images, item.RegistryImage)
						plan.LocalImages = append(plan.LocalImages, item.RegistryImage)
					}
				}
			}
			for i, d := range deployments {
				if orphaned {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
					continue
				}
				if _, ok := protected[imageKey(d.ReleaseImage)]; !ok {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
				}
			}
		}
	}
	plan.Images = uniqueStrings(plan.Images)
	plan.LocalImages = uniqueStrings(plan.LocalImages)
	for image := range protectedImages {
		if image != "" {
			plan.ProtectedImages = append(plan.ProtectedImages, image)
		}
	}
	sort.Strings(plan.ProtectedImages)
	plan.Protected = len(protected)
	return plan, nil
}

func successfulDeployments(deployments []deploy.Deployment) []deploy.Deployment {
	success := make([]deploy.Deployment, 0, len(deployments))
	for _, d := range deployments {
		if d.Outcome == "" || d.Outcome == deploy.SwarmOutcomeCompleted {
			success = append(success, d)
		}
	}
	sort.Slice(success, func(i, j int) bool { return success[i].CreatedAt.After(success[j].CreatedAt) })
	return success
}

func readDeployments(dir string) ([]deploy.Deployment, []string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "deployments"))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var items []deploy.Deployment
	var paths []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, "deployments", e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		var d deploy.Deployment
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, nil, err
		}
		items = append(items, d)
		paths = append(paths, path)
	}
	return items, paths, nil
}

func hasLiveDeployment(deployments []deploy.Deployment, liveServices map[string]struct{}) bool {
	for _, deployment := range deployments {
		if _, ok := liveServices[deployment.ServiceName]; ok {
			return true
		}
	}
	return false
}
