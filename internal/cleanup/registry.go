package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type registryClient struct {
	runner             *command.Runner
	service, container string
	port               string
}

// Registry tags can point to a Docker manifest, a multi-platform manifest
// list, or their OCI equivalents. Restricting HEAD to schema-v2 made the
// registry answer 404 for valid OCI/index tags, which then disappeared from
// the cleanup inventory.
const manifestAccept = "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json"

func (c *registryClient) addRegistryCandidates(ctx context.Context, plan *Plan) error {
	images, err := c.listImages(ctx)
	if err != nil {
		return err
	}
	protected, err := c.protectedDigests(ctx, plan.ProtectedImages)
	if err != nil {
		return err
	}
	var candidates []string
	for _, image := range images {
		digest, err := c.manifestDigest(ctx, image)
		if err != nil {
			return err
		}
		if digest != "" {
			if _, ok := protected[digest]; !ok {
				candidates = append(candidates, image)
			}
		}
	}
	plan.Images, plan.LocalImages = uniqueStrings(candidates), uniqueStrings(candidates)
	return nil
}

func (c *registryClient) listImages(ctx context.Context) ([]string, error) {
	response, err := c.request(ctx, "GET", "/v2/_catalog", "application/json")
	if err != nil {
		return nil, err
	}
	var catalog struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.Unmarshal([]byte(registryResponseBody(response)), &catalog); err != nil {
		return nil, fmt.Errorf("decode registry catalog: %w", err)
	}
	var images []string
	for _, repository := range catalog.Repositories {
		response, err := c.request(ctx, "GET", "/v2/"+repository+"/tags/list", "application/json")
		if err != nil {
			return nil, err
		}
		var tags struct {
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal([]byte(registryResponseBody(response)), &tags); err != nil {
			return nil, fmt.Errorf("decode registry tags for %q: %w", repository, err)
		}
		for _, tag := range tags.Tags {
			images = append(images, "127.0.0.1:"+c.port+"/"+repository+":"+tag)
		}
	}
	sort.Strings(images)
	return uniqueStrings(images), nil
}

func (c *registryClient) protectedDigests(ctx context.Context, images []string) (map[string]struct{}, error) {
	protected := make(map[string]struct{}, len(images))
	for _, image := range images {
		digest, err := c.manifestDigest(ctx, image)
		if err != nil {
			return nil, err
		}
		if digest != "" {
			protected[digest] = struct{}{}
		}
	}
	return protected, nil
}

func (c *registryClient) deleteImage(ctx context.Context, image string, protected map[string]struct{}) (bool, error) {
	digest, err := c.manifestDigest(ctx, image)
	if err != nil || digest == "" {
		return false, err
	}
	if _, ok := protected[digest]; ok {
		return false, nil
	}
	response, err := c.request(ctx, "DELETE", registryManifestPath(image, digest), "")
	return registryManifestDeleted(response), err
}

func (c *registryClient) manifestDigest(ctx context.Context, image string) (string, error) {
	if !strings.HasPrefix(image, "127.0.0.1:") {
		return "", nil
	}
	fullName := strings.TrimPrefix(image, "127.0.0.1:")
	name := imageKey(fullName)
	slash, colon := strings.Index(name, "/"), strings.LastIndex(name, ":")
	if slash < 0 || colon < slash {
		return "", fmt.Errorf("invalid registry image %q", image)
	}
	repo, reference := name[slash+1:colon], name[colon+1:]
	if _, digest, found := strings.Cut(fullName, "@"); found {
		reference = digest
	}
	response, err := c.request(ctx, "HEAD", "/v2/"+repo+"/manifests/"+reference, manifestAccept)
	if err != nil {
		return "", err
	}
	return registryDigest(response), nil
}

func registryManifestPath(image, digest string) string {
	name := imageKey(strings.TrimPrefix(image, "127.0.0.1:"))
	slash := strings.Index(name, "/")
	return "/v2/" + name[slash+1:strings.LastIndex(name, ":")] + "/manifests/" + digest
}

func registryDigest(response string) string {
	for _, line := range strings.Split(response, "\n") {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "docker-content-digest:") {
			return strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	return ""
}
func registryManifestDeleted(response string) bool { return strings.Contains(response, " 202 ") }
func registryResponseBody(response string) string {
	if _, body, ok := strings.Cut(response, "\r\n\r\n"); ok {
		return body
	}
	_, body, _ := strings.Cut(response, "\n\n")
	return body
}

func (c *registryClient) request(ctx context.Context, method, path, accept string) (string, error) {
	if c.container == "" {
		out, err := c.runner.Run(ctx, "docker", []string{"ps", "--filter", "name=" + c.service, "--format", "{{.ID}}"}, command.RunOptions{})
		if err != nil {
			return "", err
		}
		c.container = strings.TrimSpace(string(out.Output))
		if c.container == "" {
			return "", fmt.Errorf("registry service is not running")
		}
	}
	script := `printf '%s %s HTTP/1.1\r\nHost: 127.0.0.1\r\nAccept: %s\r\nConnection: close\r\n\r\n' "$1" "$2" "$3" | nc -w 10 127.0.0.1 5000`
	out, err := c.runner.Run(ctx, "docker", []string{"exec", c.container, "/bin/sh", "-c", script, "noops-cleanup", method, path, accept}, command.RunOptions{})
	if err != nil {
		return "", err
	}
	response := string(out.Output)
	if !strings.Contains(response, " 200 ") && !strings.Contains(response, " 202 ") && !strings.Contains(response, " 404 ") {
		return "", fmt.Errorf("registry request failed: %s", strings.TrimSpace(response))
	}
	return response, nil
}
