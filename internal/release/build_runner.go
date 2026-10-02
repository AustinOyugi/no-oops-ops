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

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

const buildRunnerImage = "docker:27-cli"

// buildImageIsolated runs a Docker build in a one-shot Swarm task. Its build
// secrets never leave the task or become image layers.
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
		s.removeTemporaryService(name)
	}()

	args := []string{"service", "create", "--detach", "--name", name, "--restart-condition", "none", "--constraint", "node.role==manager", "--mount", "type=bind,src=" + contextDir + ",dst=/work,readonly", "--mount", "type=bind,src=/var/run/docker.sock,dst=/var/run/docker.sock"}
	for _, secret := range secrets {
		args = append(args, "--secret", "source="+secret.SwarmName+",target="+secret.ID+",mode=0400")
	}

	args = append(args, "--entrypoint", "/bin/sh", buildRunnerImage, "-ec", "DOCKER_BUILDKIT=1 docker build \"$@\"", "sh")
	args = append(args, isolatedBuildArgs(image, filepath.ToSlash(relativeDockerfile), resources, noCache, secrets)...)
	s.logger.InfoContext(ctx, "running isolated build", "image", image, "secrets", len(secrets))
	if _, err := s.runner.Run(ctx, "docker", args, command.RunOptions{}); err != nil {
		return fmt.Errorf("start isolated build: %w", err)
	}
	return s.streamAndWaitForBuild(ctx, name)
}

func (s *Service) streamAndWaitForBuild(ctx context.Context, name string) error {
	logsCtx, stopLogs := context.WithCancel(ctx)
	type logResult struct {
		output []byte
		err    error
	}
	logsDone := make(chan logResult, 1)
	go func() {
		result, err := s.runner.Run(logsCtx, "docker", []string{"service", "logs", "--raw", "--follow", name}, command.RunOptions{StreamOutput: true, Stdout: os.Stdout, Stderr: os.Stderr})
		logsDone <- logResult{result.Output, err}
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

func isolatedBuildArgs(image, relativeDockerfile string, resources manifest.BuildResources, noCache bool, secrets []BuildSecretBinding) []string {
	args := []string{"-t", image, "-f", "/work/" + relativeDockerfile}
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
		status := strings.TrimSpace(string(result.Output))
		if strings.HasPrefix(status, "Complete") {
			return nil
		}
		if strings.HasPrefix(status, "Failed") || strings.HasPrefix(status, "Rejected") {
			return fmt.Errorf("isolated build task %s", status)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
