package release

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/environment"
	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

const buildRunnerImage = "docker:27-cli"

// BuildSecretBinding identifies a Swarm secret that BuildKit receives for the
// duration of an isolated build instruction. ID is the Dockerfile secret ID.
type BuildSecretBinding struct {
	ID        string
	SwarmName string
}

func (s *Service) buildSecretBindings(ctx context.Context, manifestPath string, m manifest.Manifest, target string) ([]BuildSecretBinding, error) {
	if m.Env.Build == nil || len(m.Env.Build.Secrets) == 0 {
		return nil, nil
	}
	if m.Env.File == "" {
		return nil, fmt.Errorf("env.build.secrets requires env.file entries backed by from_secret")
	}

	file, err := environment.Load(filepath.Join(filepath.Dir(manifestPath), m.Env.File))
	if err != nil {
		return nil, err
	}
	resolved := environment.Resolve(file, target, m.Env.Build.Secrets)
	refs := make(map[string]string, len(resolved.SecretRefs))
	for _, ref := range resolved.SecretRefs {
		refs[ref.Key] = ref.SecretName
	}

	bindings := make([]BuildSecretBinding, 0, len(m.Env.Build.Secrets))
	for _, key := range m.Env.Build.Secrets {
		secretName := refs[key]
		if secretName == "" {
			return nil, fmt.Errorf("env.build.secrets key %q must be declared with from_secret in %q", key, m.Env.File)
		}
		metadata, err := s.secrets.Latest(ctx, target, secretName)
		if err != nil {
			return nil, fmt.Errorf("resolve build secret %q: %w", key, err)
		}
		bindings = append(bindings, BuildSecretBinding{ID: key, SwarmName: metadata.SwarmName})
	}
	return bindings, nil
}

// buildImageIsolated runs every Docker build in a one-shot Swarm task. When a
// build uses secrets, that task is the only process able to read them; it
// streams each one directly to BuildKit using --secret without writing it to
// the Git checkout or the final image.
func (s *Service) buildImageIsolated(ctx context.Context, image, dockerfile, contextDir string, resources manifest.BuildResources, noCache bool, secrets []BuildSecretBinding) error {
	relativeDockerfile, err := filepath.Rel(contextDir, dockerfile)
	if err != nil || relativeDockerfile == ".." || strings.HasPrefix(relativeDockerfile, ".."+string(filepath.Separator)) {
		return fmt.Errorf("build Dockerfile %q must be within isolated build context %q", dockerfile, contextDir)
	}

	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return fmt.Errorf("create isolated build service name: %w", err)
	}
	name := fmt.Sprintf("noops-build-%x", random)
	defer func() {
		_, _ = s.runner.Run(context.Background(), "docker", []string{"service", "rm", name}, command.RunOptions{})
	}()

	args := []string{
		"service", "create", "--detach", "--name", name,
		"--restart-condition", "none", "--constraint", "node.role==manager",
		"--mount", "type=bind,src=" + contextDir + ",dst=/work,readonly",
		"--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock",
	}
	for _, secret := range secrets {
		args = append(args, "--secret", "source="+secret.SwarmName+",target="+secret.ID+",mode=0400")
	}
	args = append(args, "--entrypoint", "/bin/sh", buildRunnerImage, "-ec", "DOCKER_BUILDKIT=1 docker build \"$@\"", "sh")
	args = append(args, isolatedBuildArgs(image, filepath.ToSlash(relativeDockerfile), resources, noCache, secrets)...)

	s.logger.InfoContext(ctx, "running isolated build", "image", image, "secrets", len(secrets))
	if _, err := s.runner.Run(ctx, "docker", args, command.RunOptions{}); err != nil {
		return fmt.Errorf("start isolated build: %w", err)
	}

	// Reuse the command runner's streamed output while the Swarm task runs.
	// Waiting for logs only after completion makes long cold builds look stuck.
	logsCtx, stopLogs := context.WithCancel(ctx)
	type logResult struct {
		output []byte
		err    error
	}
	logsDone := make(chan logResult, 1)
	go func() {
		result, err := s.runner.Run(logsCtx, "docker", []string{"service", "logs", "--raw", "--follow", name}, command.RunOptions{
			StreamOutput: true,
			Stdout:       os.Stdout,
			Stderr:       os.Stderr,
		})
		logsDone <- logResult{output: result.Output, err: err}
	}()

	buildErr := s.waitForBuildTask(ctx, name)
	stopLogs()
	logs := <-logsDone
	if buildErr != nil {
		if output := strings.TrimSpace(string(logs.output)); output != "" {
			return fmt.Errorf("isolated build failed: %s", output)
		}
		return buildErr
	}
	return nil
}

// isolatedBuildArgs are passed after `sh` to `docker build "$@"`. In
// particular, the first argument is a Docker build option, not the literal
// word "build", because the shell command has already selected that subcommand.
func isolatedBuildArgs(image, relativeDockerfile string, resources manifest.BuildResources, noCache bool, secrets []BuildSecretBinding) []string {
	args := []string{"-t", image, "-f", "/work/" + relativeDockerfile}
	// BuildKit deliberately excludes secret contents from cache keys. A build
	// that consumes an environment secret must therefore bypass cache so a
	// rotated value is reflected in generated client assets (for example,
	// Next.js NEXT_PUBLIC_* values).
	if noCache || len(secrets) > 0 {
		args = append([]string{"--no-cache"}, args...)
	}
	if resources.Memory != "" {
		args = append(args, "--memory", resources.Memory)
	}
	if resources.CPUs != "" {
		cpus, _ := strconv.ParseFloat(resources.CPUs, 64)
		args = append(args, "--cpu-period", "100000", "--cpu-quota", strconv.FormatInt(int64(cpus*100000), 10))
	}
	for _, secret := range secrets {
		args = append(args, "--secret", "id="+secret.ID+",src=/run/secrets/"+secret.ID)
	}
	return append(args, "/work")
}

func (s *Service) waitForBuildTask(ctx context.Context, name string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := s.runner.Run(ctx, "docker", []string{"service", "ps", "--no-trunc", "--format", "{{.CurrentState}}", name}, command.RunOptions{})
		if err != nil {
			return fmt.Errorf("inspect isolated build: %w", err)
		}
		state := strings.TrimSpace(string(result.Output))
		if strings.HasPrefix(state, "Complete") {
			return nil
		}
		if strings.HasPrefix(state, "Failed") || strings.HasPrefix(state, "Rejected") {
			return fmt.Errorf("isolated build task %s", state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
