package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/AustinOyugi/no-oops-ops/internal/catalog"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

// EnvironmentLogs discovers existing environment-owned Swarm services, then
// follows them concurrently. Docker's default task/service prefixes identify
// each source; serialization permits callers to supply non-concurrent writers.
func (s *Service) EnvironmentLogs(ctx context.Context, environment string, options LogOptions, stdout, stderr io.Writer) error {
	if err := options.Validate(); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`).MatchString(environment) {
		return fmt.Errorf("invalid environment %q", environment)
	}
	names, err := s.environmentLogServices(ctx, environment)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("no deployed services found for environment %q", environment)
	}
	var lock sync.Mutex
	out := lockedLogWriter{lock: &lock, writer: stdout}
	diagnostic := lockedLogWriter{lock: &lock, writer: stderr}
	results := make(chan error, len(names))
	for _, name := range names {
		go func(name string) {
			err := streamServiceLogs(ctx, name, options, out, diagnostic)
			if err != nil {
				fmt.Fprintln(diagnostic, err)
			}
			results <- err
		}(name)
	}
	var failures []error
	for range names {
		if err := <-results; err != nil {
			failures = append(failures, err)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return errors.Join(failures...)
}

type lockedLogWriter struct {
	lock   *sync.Mutex
	writer io.Writer
}

func (w lockedLogWriter) Write(p []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if w.writer == nil {
		return len(p), nil
	}
	return w.writer.Write(p)
}

func (s *Service) environmentLogServices(ctx context.Context, environment string) ([]string, error) {
	apps, err := catalog.Load(s.config.Workspace)
	if err != nil {
		return nil, err
	}
	// Known identities let us recover older deployments without local history.
	identities := map[string]bool{}
	for alias := range apps.Apps {
		path, err := catalog.Resolve(s.config.Workspace, alias)
		if err != nil {
			return nil, err
		}
		services, err := manifest.Services(path)
		if err != nil {
			return nil, err
		}
		for _, service := range services {
			m, err := manifest.LoadService(path, service)
			if err != nil {
				return nil, err
			}
			if m.ShouldDeploy() {
				identities[m.Name] = true
			}
		}
	}
	// History retains exact names of compacted blue/green stacks, and apps that
	// were removed from the catalog but still have a live service.
	known := map[string]bool{}
	history, err := filepath.Glob(filepath.Join(s.config.StateDir, "apps", "*", environment, "deployments", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range history {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read deployment history: %w", err)
		}
		var active Deployment
		if err := json.Unmarshal(data, &active); err != nil {
			return nil, fmt.Errorf("decode deployment history %q: %w", path, err)
		}
		name := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		known[logServiceName(environment, name, active)] = true
	}
	result, err := s.runner.Run(ctx, "docker", []string{"service", "ls", "--format", "{{.Name}}"}, command.RunOptions{})
	if err != nil {
		return nil, fmt.Errorf("list environment services: %w: %s", err, strings.TrimSpace(string(result.Output)))
	}
	selected := map[string]bool{}
	for _, name := range strings.Fields(string(result.Output)) {
		if known[name] {
			selected[name] = true
			continue
		}
		for identity := range identities {
			if name == swarmServiceName(environment, identity) {
				selected[name] = true
				break
			}
			pattern := `^` + regexp.QuoteMeta(stackName(environment, identity)) + `-r[a-z0-9-]+_app$`
			if regexp.MustCompile(pattern).MatchString(name) {
				selected[name] = true
				break
			}
		}
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
