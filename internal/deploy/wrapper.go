package deploy

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type ImageMetadata struct {
	Entrypoint []string
	Cmd        []string
}

type EffectiveExecution struct {
	Entrypoint []string
	Cmd        []string
}

type SecretMapping struct {
	EnvKey     string `json:"env_key"`
	SecretName string `json:"secret_name"`
}

type WrapperConfig struct {
	UseWrapper     bool
	WrapperImage   string
	OriginalImage  string
	EffectiveExec  EffectiveExecution
	SecretMappings []SecretMapping
}

//go:embed templates/bootstrap.sh
var bootstrapScript []byte

func pullImage(ctx context.Context, runner *command.Runner, imageRef string) error {
	result, err := runner.Run(ctx, "docker", []string{"pull", imageRef}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("pull image %q: %w: %s", imageRef, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}

func inspectImage(ctx context.Context, runner *command.Runner, imageRef string) (ImageMetadata, error) {
	entrypointResult, err := runner.Run(ctx, "docker", []string{
		"inspect",
		"--format",
		`{{json .Config.Entrypoint}}`,
		imageRef,
	}, command.RunOptions{})
	if err != nil {
		return ImageMetadata{}, fmt.Errorf("inspect image entrypoint %q: %w", imageRef, err)
	}

	cmdResult, err := runner.Run(ctx, "docker", []string{
		"inspect",
		"--format",
		`{{json .Config.Cmd}}`,
		imageRef,
	}, command.RunOptions{})
	if err != nil {
		return ImageMetadata{}, fmt.Errorf("inspect image cmd %q: %w", imageRef, err)
	}

	var entrypoint, cmd []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(entrypointResult.Output))), &entrypoint); err != nil {
		return ImageMetadata{}, fmt.Errorf("parse image entrypoint: %w", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(cmdResult.Output))), &cmd); err != nil {
		return ImageMetadata{}, fmt.Errorf("parse image cmd: %w", err)
	}

	return ImageMetadata{Entrypoint: entrypoint, Cmd: cmd}, nil
}

func ResolveEffectiveExecution(imgMeta ImageMetadata, manifestEntrypoint, manifestCmd []string) EffectiveExecution {
	ep := imgMeta.Entrypoint
	if len(manifestEntrypoint) > 0 {
		ep = manifestEntrypoint
	}

	cmd := imgMeta.Cmd
	if len(manifestCmd) > 0 {
		cmd = manifestCmd
	}

	return EffectiveExecution{Entrypoint: ep, Cmd: cmd}
}

func BuildWrapperConfig(
	resolutionMode string,
	imageRef string,
	imgMeta ImageMetadata,
	manifestCmd []string,
	secretBindings []SecretBinding,
) WrapperConfig {
	if resolutionMode != "env" || len(secretBindings) == 0 {
		return WrapperConfig{UseWrapper: false}
	}

	execution := ResolveEffectiveExecution(imgMeta, nil, manifestCmd)
	if len(execution.Entrypoint) == 0 && len(execution.Cmd) == 0 {
		return WrapperConfig{}
	}

	mappings := make([]SecretMapping, len(secretBindings))
	for i, binding := range secretBindings {
		mappings[i] = SecretMapping{
			EnvKey:     binding.EnvKey,
			SecretName: binding.SecretName,
		}
	}

	return WrapperConfig{
		UseWrapper:     true,
		OriginalImage:  imageRef,
		EffectiveExec:  execution,
		SecretMappings: mappings,
	}
}

func jsonStringSlice(s []string) string {
	if s == nil {
		return "[]"
	}
	data, _ := json.Marshal(s)
	return string(data)
}

func secretMappingsValue(mappings []SecretMapping) string {
	values := make([]string, len(mappings))
	for i, mapping := range mappings {
		values[i] = mapping.EnvKey + "=/run/secrets/" + mapping.EnvKey
	}
	return strings.Join(values, ",")
}

func wrappedImageRef(cfg config.Config, applicationImage, applicationName string) string {
	sum := sha256.Sum256(append([]byte(applicationImage+"\x00"), bootstrapScript...))
	return fmt.Sprintf("127.0.0.1:%s/%s:%x", cfg.RegistryPort, applicationName, sum[:12])
}

func wrappedImageDockerfile(applicationImage string) string {
	return fmt.Sprintf("FROM %s\nCOPY bootstrap.sh /bootstrap.sh\n", applicationImage)
}

func (s *Service) buildWrappedImage(ctx context.Context, applicationImage, applicationName string) (string, error) {
	contextDir, err := os.MkdirTemp("", "noops-wrapper-*")
	if err != nil {
		return "", fmt.Errorf("create wrapper build context: %w", err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {

		}
	}(contextDir)

	dockerfilePath := filepath.Join(contextDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(wrappedImageDockerfile(applicationImage)), 0o644); err != nil {
		return "", fmt.Errorf("write wrapper Dockerfile: %w", err)
	}

	if err := os.WriteFile(filepath.Join(contextDir, "bootstrap.sh"), bootstrapScript, 0o644); err != nil {
		return "", fmt.Errorf("write wrapper bootstrap script: %w", err)
	}

	image := wrappedImageRef(s.config, applicationImage, applicationName)
	result, err := s.runner.Run(ctx, "docker", []string{"build", "-t", image, "-f", dockerfilePath, contextDir}, command.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("build %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	result, err = s.runner.Run(ctx, "docker", []string{"push", image}, command.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("push %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}

	return image, nil
}
