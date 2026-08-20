package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
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
	secretBindings []SecretBinding,
	wrapperImage string,
) WrapperConfig {
	if resolutionMode != "env" || len(secretBindings) == 0 {
		return WrapperConfig{UseWrapper: false}
	}

	execution := ResolveEffectiveExecution(imgMeta, nil, nil)
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
		WrapperImage:   wrapperImage,
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

func secretMappingsString(mappings []SecretMapping) string {
	data, _ := json.Marshal(mappings)
	return string(data)
}

func wrappedImageRef(cfg config.Config, applicationImage, wrapperImage string) string {
	sum := sha256.Sum256([]byte(applicationImage + "\x00" + wrapperImage))
	return fmt.Sprintf("127.0.0.1:%s/noops-runtime:%x", cfg.RegistryPort, sum[:12])
}

func wrappedImageDockerfile(applicationImage, wrapperImage string) string {
	return fmt.Sprintf("FROM %s\nCOPY --from=%s /bootstrap.sh /bootstrap.sh\n", applicationImage, wrapperImage)
}

func (s *Service) buildWrappedImage(ctx context.Context, applicationImage, wrapperImage string) (string, error) {
	image := wrappedImageRef(s.config, applicationImage, wrapperImage)
	dockerfile := wrappedImageDockerfile(applicationImage, wrapperImage)
	result, err := s.runner.Run(ctx, "docker", []string{"build", "--pull", "-t", image, "-f", "-", "."}, command.RunOptions{Stdin: bytes.NewBufferString(dockerfile)})
	if err != nil {
		return "", fmt.Errorf("build %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}
	result, err = s.runner.Run(ctx, "docker", []string{"push", image}, command.RunOptions{})
	if err != nil {
		return "", fmt.Errorf("push %q: %w: %s", image, err, strings.TrimSpace(string(result.Output)))
	}
	return image, nil
}
