package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/release"
)

type RemoveResult struct {
	Environment    string
	ManifestPath   string
	StackName      string
	RegistryImages int
	StatePath      string
}

func (s *Service) Remove(ctx context.Context, environment, path string) (RemoveResult, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return RemoveResult{}, fmt.Errorf("resolve manifest path %q: %w", path, err)
	}
	m, err := manifest.Load(absPath)
	if err != nil {
		return RemoveResult{}, err
	}

	active, err := s.deployments.Latest(s.config, m.Name, environment)
	if err != nil {
		return RemoveResult{}, err
	}
	stack := active.StackName
	if stack == "" {
		stack = stackName(environment, m.Name)
	}
	s.logger.InfoContext(ctx, "removing application", "environment", environment, "name", m.Name, "stack", stack)
	if err := s.ingress.Remove(ctx, environment, m.Name); err != nil {
		return RemoveResult{}, fmt.Errorf("remove ingress route: %w", err)
	}
	if err := s.removeStack(ctx, stack); err != nil {
		return RemoveResult{}, err
	}

	metadata, err := release.ListHistory(s.config, m.Name, environment)
	if err != nil {
		return RemoveResult{}, err
	}

	deleted, err := s.removeRegistryImages(ctx, m.Name, metadata)
	if err != nil {
		return RemoveResult{}, err
	}

	statePath, err := appStatePath(s.config.StateDir, m.Name, environment)
	if err != nil {
		return RemoveResult{}, err
	}
	if err := os.RemoveAll(statePath); err != nil {
		return RemoveResult{}, fmt.Errorf("remove application state %q: %w", statePath, err)
	}

	return RemoveResult{
		Environment:    environment,
		ManifestPath:   absPath,
		StackName:      stack,
		RegistryImages: deleted,
		StatePath:      statePath,
	}, nil
}

func (s *Service) removeStack(ctx context.Context, stack string) error {
	result, err := s.runner.Run(ctx, "docker", []string{"stack", "rm", stack}, command.RunOptions{LogCommand: true})
	if err != nil {
		return fmt.Errorf("remove stack %q: %w: %s", stack, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}

func (s *Service) removeRegistryImages(ctx context.Context, appName string, releases []release.Metadata) (int, error) {
	client := &registryDockerClient{
		runner:          s.runner,
		registryService: s.config.RegistryName + "_registry",
		baseURL:         "http://127.0.0.1:" + s.config.RegistryPort,
	}
	refs := make(map[string]struct{})
	for _, item := range releases {
		if item.RegistryImage != "" {
			refs[item.RegistryImage] = struct{}{}
			// A wrapper is derived from this release image when env-mode secrets
			// are used. Deleting its deterministic tag is harmless when no wrapper
			// was built, and avoids touching another environment's wrappers.
			refs[wrappedImageRef(s.config, item.RegistryImage, appName)] = struct{}{}
		}
	}

	deleted := 0
	for ref := range refs {
		if err := client.DeleteImage(ctx, ref); err != nil {
			return deleted, err
		}
		deleted++
	}

	return deleted, nil
}

func appStatePath(stateDir, appName, environment string) (string, error) {
	base := filepath.Join(stateDir, "apps")
	path := filepath.Join(base, appName, environment)
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("application state path escapes the managed apps directory")
	}
	return path, nil
}

type registryClient struct {
	baseURL string
	http    *http.Client
}

// registryDockerClient sends Registry API requests from inside the registry
// container. Docker Desktop can reserve the host port independently, so a
// host-side request to 127.0.0.1:<registry-port> is not reliable.
type registryDockerClient struct {
	runner          *command.Runner
	registryService string
	baseURL         string
	containerID     string
}

func (c *registryDockerClient) DeleteImage(ctx context.Context, ref string) error {
	repository, tag, err := registryReference(c.baseURL, ref)
	if err != nil {
		return err
	}
	return c.DeleteRepositoryTag(ctx, repository, tag)
}

func (c *registryDockerClient) DeleteRepositoryTag(ctx context.Context, repository, tag string) error {
	path := registryAPIPath(repository, "manifests", tag)
	response, err := c.request(ctx, http.MethodHead, path, "application/vnd.docker.distribution.manifest.v2+json")
	if err != nil {
		return fmt.Errorf("resolve registry manifest %q:%s: %w", repository, tag, err)
	}
	status := registryResponseStatus(response)
	if status == http.StatusNotFound {
		return nil
	}
	if status != http.StatusOK {
		return fmt.Errorf("resolve registry manifest %q:%s: registry returned %d", repository, tag, status)
	}
	digest := registryResponseHeader(response, "Docker-Content-Digest")
	if digest == "" {
		return fmt.Errorf("resolve registry manifest %q:%s: registry did not return a digest", repository, tag)
	}

	response, err = c.request(ctx, http.MethodDelete, registryAPIPath(repository, "manifests", digest), "")
	if err != nil {
		return fmt.Errorf("delete registry manifest %q:%s: %w", repository, tag, err)
	}
	status = registryResponseStatus(response)
	if status == http.StatusNotFound || status == http.StatusAccepted {
		return nil
	}
	return fmt.Errorf("delete registry manifest %q:%s: registry returned %d", repository, tag, status)
}

func (c *registryDockerClient) request(ctx context.Context, method, path, accept string) (string, error) {
	if c.containerID == "" {
		result, err := c.runner.Run(ctx, "docker", []string{
			"ps", "--filter", "name=" + c.registryService, "--format", "{{.ID}}",
		}, command.RunOptions{})
		if err != nil {
			return "", fmt.Errorf("find registry container %q: %w", c.registryService, err)
		}
		c.containerID = strings.TrimSpace(string(result.Output))
		if index := strings.IndexByte(c.containerID, '\n'); index >= 0 {
			c.containerID = c.containerID[:index]
		}
		if c.containerID == "" {
			return "", fmt.Errorf("registry container for service %q is not running", c.registryService)
		}
	}

	const script = `printf '%s %s HTTP/1.1\r\nHost: 127.0.0.1\r\nAccept: %s\r\nConnection: close\r\n\r\n' "$1" "$2" "$3" | nc -w 10 127.0.0.1 5000`
	result, err := c.runner.Run(ctx, "docker", []string{
		"exec", c.containerID, "/bin/sh", "-c", script, "noops-registry-request", method, path, accept,
	}, command.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("request registry API: %w: %s", err, strings.TrimSpace(string(result.Output)))
	}
	return string(result.Output), nil
}

func registryAPIPath(repository string, parts ...string) string {
	return (&url.URL{Path: "/v2/" + strings.Join(append([]string{repository}, parts...), "/")}).EscapedPath()
}

func registryResponseStatus(response string) int {
	for _, line := range strings.Split(response, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "HTTP/") {
			status, _ := strconv.Atoi(fields[1])
			return status
		}
	}
	return 0
}

func registryResponseHeader(response, name string) string {
	prefix := strings.ToLower(name) + ":"
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return ""
}

func (c *registryClient) Tags(ctx context.Context, repository string) ([]string, error) {
	endpoint := c.endpoint(repository, "tags/list")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list registry tags for %q: %w", repository, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list registry tags for %q: registry returned %s", repository, resp.Status)
	}
	var response struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode registry tags for %q: %w", repository, err)
	}
	return response.Tags, nil
}

func (c *registryClient) DeleteImage(ctx context.Context, ref string) error {
	repository, tag, err := registryReference(c.baseURL, ref)
	if err != nil {
		return err
	}
	return c.DeleteRepositoryTag(ctx, repository, tag)
}

func (c *registryClient) DeleteRepositoryTag(ctx context.Context, repository, tag string) error {
	endpoint := c.endpoint(repository, "manifests", tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("resolve registry manifest %q:%s: %w", repository, tag, err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resolve registry manifest %q:%s: registry returned %s", repository, tag, resp.Status)
	}
	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return fmt.Errorf("resolve registry manifest %q:%s: registry did not return a digest", repository, tag)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodDelete, c.endpoint(repository, "manifests", digest), nil)
	if err != nil {
		return err
	}
	resp, err = c.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete registry manifest %q:%s: %w", repository, tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return fmt.Errorf("delete registry manifest %q:%s: registry returned %s", repository, tag, resp.Status)
}

func (c *registryClient) endpoint(parts ...string) string {
	return c.baseURL + registryAPIPath(parts[0], parts[1:]...)
}

func registryReference(baseURL, ref string) (string, string, error) {
	prefix := strings.TrimPrefix(baseURL, "http://") + "/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", fmt.Errorf("registry image %q does not belong to %s", ref, strings.TrimSuffix(prefix, "/"))
	}
	name := strings.TrimPrefix(ref, prefix)
	index := strings.LastIndex(name, ":")
	if index <= 0 || index == len(name)-1 {
		return "", "", fmt.Errorf("registry image %q does not include a tag", ref)
	}
	return name[:index], name[index+1:], nil
}
