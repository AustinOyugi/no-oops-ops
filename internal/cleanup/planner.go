package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

// plan deliberately uses image references, not historical service names, to
// determine image liveness. A service name is reused as an app is upgraded,
// so it cannot identify a particular deployed version.
func (s *Service) plan(live liveInventory, keep int, orphanedOnly bool) (Plan, error) {
	return s.planOptions(live, Options{Keep: keep, Orphaned: orphanedOnly})
}

func (s *Service) planOptions(live liveInventory, options Options) (Plan, error) {
	keep, orphanedOnly := options.Keep, options.Orphaned
	protectedLocal := make(map[string]struct{})
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
			if options.App != "" && (app.Name() != options.App || env.Name() != options.Environment) {
				for _, item := range releases {
					protectedLocal[item.Image] = struct{}{}
					protected[imageKey(item.RegistryImage)] = struct{}{}
					protectedImages[item.RegistryImage] = struct{}{}
				}
				for _, item := range deployments {
					protected[imageKey(item.ReleaseImage)] = struct{}{}
					protectedImages[item.ReleaseImage] = struct{}{}
				}
				continue
			}
			removeDeployments := make(map[int]bool)
			orphaned := orphanedOnly && !hasLiveDeployment(deployments, live.services)
			if !orphaned {
				for i, item := range releases {
					if i < keep {
						protectedLocal[item.Image] = struct{}{}
						protected[imageKey(item.RegistryImage)] = struct{}{}
						protectedImages[item.RegistryImage] = struct{}{}
					}
				}
			}
			success := successfulDeployments(deployments)
			for i, d := range success {
				if !options.ReleaseRetention && !orphaned && i < keep {
					protected[imageKey(d.ReleaseImage)] = struct{}{}
					protectedImages[d.ReleaseImage] = struct{}{}
				}
			}
			for _, item := range releases {
				_, protectedImage := protected[imageKey(item.RegistryImage)]
				if protectedImage {
					protectedLocal[item.Image] = struct{}{}
				}
				if orphaned || !protectedImage {
					plan.ReleasePaths = append(plan.ReleasePaths, filepath.Join(dir, "releases", item.Tag+".json"))
					if !orphaned && !protectedImage && item.RegistryImage != "" {
						plan.Images = append(plan.Images, item.RegistryImage)
						plan.LocalImages = append(plan.LocalImages, item.RegistryImage)
						if options.ReleaseRetention && item.Image != "" {
							plan.LocalImages = append(plan.LocalImages, item.Image)
						}
					}
				}
			}
			for i, d := range deployments {
				if orphaned {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
					removeDeployments[i] = true
					continue
				}
				if _, ok := protected[imageKey(d.ReleaseImage)]; !ok {
					plan.DeploymentPaths = append(plan.DeploymentPaths, paths[i])
					removeDeployments[i] = true
				}
			}
			retainedStacks := make(map[string]struct{})
			for i, deployment := range deployments {
				if !removeDeployments[i] && deployment.StackName != "" {
					retainedStacks[deployment.StackName] = struct{}{}
				}
			}
			stackPaths, err := staleStackArtifacts(dir, retainedStacks)
			if err != nil {
				return plan, err
			}
			plan.StackPaths = append(plan.StackPaths, stackPaths...)
		}
	}
	if options.App != "" {
		images := plan.Images[:0]
		for _, image := range plan.Images {
			if _, ok := protected[imageKey(image)]; !ok {
				images = append(images, image)
			}
		}
		plan.Images = images
		locals := plan.LocalImages[:0]
		for _, image := range plan.LocalImages {
			_, protectedImage := protected[imageKey(image)]
			_, protectedAlias := protectedLocal[image]
			if !protectedImage && !protectedAlias {
				locals = append(locals, image)
			}
		}
		plan.LocalImages = locals
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

// staleStackArtifacts selects generated blue/green candidate manifests whose
// stack has no retained deployment record. They are state artifacts only: live
// Docker stacks are reconciled by deploy, never removed by retention cleanup.
func staleStackArtifacts(dir string, retained map[string]struct{}) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "stack-*.yml"))
	if err != nil {
		return nil, err
	}
	stale := make([]string, 0, len(paths))
	for _, path := range paths {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "stack-"), ".yml")
		if _, ok := retained[name]; !ok {
			stale = append(stale, path)
		}
	}
	return stale, nil
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
