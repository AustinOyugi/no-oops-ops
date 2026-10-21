package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

var errIncompleteRenameCatalog = errors.New("incomplete catalog for rename detection")

// retiredDeployments traces live stacks back to deployment and release records.
// Image equality alone is not ownership: replicas of an application can share
// both an image and a Git repository. Never adopt a currently declared service.
func (s *Service) retiredDeployments(ctx context.Context, m manifest.Manifest, environment string) ([]Deployment, error) {
	entries, err := os.ReadDir(filepath.Join(s.config.StateDir, "apps"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	declared, uniqueSource, err := s.declaredServices(m)
	if errors.Is(err, errIncompleteRenameCatalog) {
		// Without a complete inventory, an old identity may still be declared.
		// Leave existing stacks alone and allow the requested deployment to proceed.
		s.logger.WarnContext(ctx, "skipping automatic rename detection", "reason", err.Error())
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var candidates []Deployment
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == m.Name || declared[entry.Name()] {
			continue
		}
		previous, err := s.deployments.Latest(s.config, entry.Name(), environment)
		if err != nil {
			return nil, err
		}
		if previous.StackName == "" || previous.App != entry.Name() || previous.Environment != environment {
			continue
		}
		// A stack must belong to its recorded identity, not merely resemble the
		// new name. This also excludes arbitrary stacks in malformed history.
		if stack, ok := appStackForService(previous.StackName+"_app", stackName(environment, previous.App)); !ok || stack != previous.StackName {
			continue
		}
		if previous.ManifestPath != "" {
			if filepath.Clean(previous.ManifestPath) != filepath.Clean(m.Path) {
				continue
			}
		}
		// Legacy history has no manifest path. Require both the same Git
		// source and image repository, unique among current catalog services.
		if !uniqueSource || m.Build.Source.Git == nil {
			continue
		}
		record, err := s.releases.Find(s.config, previous.App, environment, previous.ReleaseTag)
		if err != nil {
			return nil, fmt.Errorf("trace previous stack %q: %w", previous.StackName, err)
		}
		if record.Git == nil || record.Git.URL != m.Build.Source.Git.URL || imageRepository(record.Image) != m.Image.Repository {
			continue
		}
		candidates = append(candidates, previous)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	result, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "--format", "{{.Name}}"}, command.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("trace previous application stacks: %w", err)
	}
	live := make(map[string]bool)
	for _, service := range strings.Fields(string(result.Output)) {
		stack, _, ok := strings.Cut(service, "_")
		if ok {
			live[stack] = true
		}
	}
	var retired []Deployment
	for _, previous := range candidates {
		if live[previous.StackName] {
			retired = append(retired, previous)
		}
	}
	legacy := 0
	for _, previous := range retired {
		if previous.ManifestPath == "" {
			legacy++
		}
	}
	if legacy > 1 {
		return nil, fmt.Errorf("multiple previous stacks match service %q; cannot establish rename ownership", m.Name)
	}
	sort.Slice(retired, func(i, j int) bool { return retired[i].CreatedAt.After(retired[j].CreatedAt) })
	return retired, nil
}

func imageRepository(image string) string {
	image, _, _ = strings.Cut(image, "@")
	if colon := strings.LastIndex(image, ":"); colon > strings.LastIndex(image, "/") {
		image = image[:colon]
	}
	return image
}

func (s *Service) declaredServices(m manifest.Manifest) (map[string]bool, bool, error) {
	paths := map[string]bool{m.Path: true}
	if s.config.Workspace != "" {
		file, err := catalog.Load(s.config.Workspace)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, false, fmt.Errorf("%w: %w", errIncompleteRenameCatalog, err)
		}
		for _, app := range file.Apps {
			paths[filepath.Clean(filepath.Join(s.config.Workspace, app.Manifest))] = true
		}
	}
	declared := make(map[string]bool)
	uniqueSource := true
	for path := range paths {
		names, err := manifest.Services(path)
		if err != nil {
			if path != m.Path {
				return nil, false, fmt.Errorf("%w: %w", errIncompleteRenameCatalog, err)
			}
			return nil, false, err
		}
		for _, name := range names {
			declared[name] = true
			if path == m.Path && name == m.Name {
				continue
			}
			other, err := manifest.LoadService(path, name)
			if err != nil {
				if path != m.Path {
					return nil, false, fmt.Errorf("%w: %w", errIncompleteRenameCatalog, err)
				}
				return nil, false, err
			}
			if m.Build.Source.Git != nil && other.Build.Source.Git != nil && other.Build.Source.Git.URL == m.Build.Source.Git.URL {
				uniqueSource = false
			}
		}
	}
	return declared, uniqueSource, nil
}
