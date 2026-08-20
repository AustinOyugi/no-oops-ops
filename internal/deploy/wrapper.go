package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
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
	UseWrapper      bool
	WrapperImage    string
	OriginalImage   string
	EffectiveExec   EffectiveExecution
	SecretMappings  []SecretMapping
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
	m manifest.Manifest,
	secretBindings []SecretBinding,
	wrapperImage string,
) WrapperConfig {
	if resolutionMode != "env" || len(secretBindings) == 0 {
		return WrapperConfig{UseWrapper: false}
	}

	execution := ResolveEffectiveExecution(imgMeta, nil, nil)

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
	parts := make([]string, len(mappings))
	for i, m := range mappings {
		parts[i] = m.EnvKey + "=" + m.SecretName
	}
	return strings.Join(parts, ",")
}
