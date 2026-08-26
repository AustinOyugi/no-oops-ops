package release

import (
	"context"
	"crypto/rand"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

const gitClientImage = "alpine/git:2.47.2"

//go:embed scripts/git-fetch.sh
var gitFetchScript []byte

var gitCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type GitMetadata struct {
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
	Secret string `json:"secret,omitempty"`
}

func (s *Service) gitBuildContext(ctx context.Context, environment string, build manifest.NoOpsBuild) (string, GitMetadata, func(), error) {
	git := build.Source.Git
	settings, ok := git.Environments[environment]
	if !ok {
		return "", GitMetadata{}, nil, fmt.Errorf("x-noops.build.source.git has no configuration for environment %q", environment)
	}
	s.logger.InfoContext(ctx, "fetching Git build source", "environment", environment, "repository", git.URL, "ref", settings.Ref, "secret_configured", settings.Secret != "")
	root, err := os.MkdirTemp(s.config.DataDir, "build-source-")
	if err != nil {
		return "", GitMetadata{}, nil, fmt.Errorf("create temporary Git build context: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	scriptPath := filepath.Join(root, "git-fetch.sh")
	if err := os.WriteFile(scriptPath, gitFetchScript, 0o700); err != nil {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("write Git fetch script: %w", err)
	}
	args := []string{"run", "--rm", "--entrypoint", "/bin/sh", "--mount", "type=bind,src=" + root + ",dst=/work"}
	if settings.Secret != "" {
		metadata, err := s.secrets.Latest(ctx, environment, settings.Secret)
		if err != nil {
			cleanup()
			return "", GitMetadata{}, nil, fmt.Errorf("read source secret %q for environment %q: %w", settings.Secret, environment, err)
		}
		return s.gitBuildContextWithSwarmSecret(ctx, root, git.URL, settings.Ref, metadata.SwarmName, settings.Secret, cleanup)
	}
	args = append(args, gitClientImage, "/work/git-fetch.sh", git.URL, settings.Ref)
	result, err := s.runner.Run(ctx, "docker", args, command.RunOptions{})
	if err != nil {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("fetch Git build source: %s", gitFailure(result.Output))
	}
	commit := ""
	for _, line := range strings.Fields(string(result.Output)) {
		if gitCommitPattern.MatchString(line) {
			commit = line
		}
	}
	if commit == "" {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("fetch Git build source: did not receive a commit SHA")
	}
	s.logger.InfoContext(ctx, "Git build source fetched", "environment", environment, "repository", git.URL, "commit", commit)
	return filepath.Join(root, "repository"), GitMetadata{URL: git.URL, Ref: settings.Ref, Commit: commit, Secret: settings.Secret}, cleanup, nil
}

func (s *Service) gitBuildContextWithSwarmSecret(ctx context.Context, root, repository, ref, secretName, credential string, cleanup func()) (string, GitMetadata, func(), error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("create Git fetch service name: %w", err)
	}
	name := fmt.Sprintf("noops-git-fetch-%x", random)
	defer func() {
		_, _ = s.runner.Run(context.Background(), "docker", []string{"service", "rm", name}, command.RunOptions{})
	}()
	args := []string{"service", "create", "--detach", "--name", name, "--restart-condition", "none", "--constraint", "node.role==manager", "--mount", "type=bind,src=" + root + ",dst=/work", "--secret", "source=" + secretName + ",target=git-token,mode=0400", "--entrypoint", "/bin/sh", gitClientImage, "/work/git-fetch.sh", repository, ref}
	if _, err := s.runner.Run(ctx, "docker", args, command.RunOptions{}); err != nil {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("start Git fetch service: %w", err)
	}
	if err := s.waitForGitFetchTask(ctx, name); err != nil {
		cleanup()
		return "", GitMetadata{}, nil, err
	}
	result, err := s.runner.Run(ctx, "docker", []string{"service", "logs", "--raw", name}, command.RunOptions{})
	if err != nil {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("fetch Git build source: %s", gitFailure(result.Output))
	}
	commit := ""
	for _, line := range strings.Fields(string(result.Output)) {
		if gitCommitPattern.MatchString(line) {
			commit = line
		}
	}
	if commit == "" {
		cleanup()
		return "", GitMetadata{}, nil, fmt.Errorf("fetch Git build source: did not receive a commit SHA")
	}
	s.logger.InfoContext(ctx, "Git build source fetched", "repository", repository, "commit", commit)
	return filepath.Join(root, "repository"), GitMetadata{URL: repository, Ref: ref, Commit: commit, Secret: credential}, cleanup, nil
}

func (s *Service) waitForGitFetchTask(ctx context.Context, name string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		result, err := s.runner.Run(ctx, "docker", []string{"service", "ps", "--no-trunc", "--format", "{{.CurrentState}}", name}, command.RunOptions{})
		if err != nil {
			return fmt.Errorf("inspect Git fetch service: %w", err)
		}
		state := strings.TrimSpace(string(result.Output))
		if strings.HasPrefix(state, "Complete") {
			return nil
		}
		if strings.HasPrefix(state, "Failed") || strings.HasPrefix(state, "Rejected") {
			return fmt.Errorf("fetch Git build source: Git fetch task %s", state)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func gitFailure(output []byte) string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "fatal:") {
			if strings.Contains(line, "could not read Username") {
				return "Git credential was rejected or does not have access to the repository"
			}
			return line
		}
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return "Git client exited without an error message"
}
