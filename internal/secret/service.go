package secret

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type commandRunner interface {
	Run(context.Context, string, []string, command.RunOptions) (command.Result, error)
}

type Service struct {
	config config.Config
	logger *slog.Logger
	runner commandRunner
	store  filesystemStore
	now    func() time.Time
}

func NewService(logger *slog.Logger, cfg config.Config) *Service {
	return &Service{
		config: cfg,
		logger: logger,
		runner: command.NewRunner(logger),
		store:  newFilesystemStore(),
		now:    time.Now,
	}
}

func (s *Service) Set(ctx context.Context, environment string, key string, value io.Reader) (Metadata, error) {
	if err := validateIdentifier("environment", environment); err != nil {
		return Metadata{}, err
	}
	if err := validateIdentifier("secret key", key); err != nil {
		return Metadata{}, err
	}

	secretValue, err := io.ReadAll(value)
	if err != nil {
		return Metadata{}, fmt.Errorf("read secret value: %w", err)
	}
	if len(bytes.TrimSpace(secretValue)) == 0 {
		return Metadata{}, fmt.Errorf("secret value must not be empty")
	}
	defer clear(secretValue)

	items, err := s.store.List(s.config.StateDir, environment)
	if err != nil {
		return Metadata{}, err
	}

	version := nextVersion(items, key)
	metadata, err := s.createNextVersion(ctx, environment, key, version, secretValue)
	if err != nil {
		return Metadata{}, err
	}

	if _, err := s.store.Save(s.config.StateDir, metadata); err != nil {
		return Metadata{}, err
	}

	return metadata, nil
}

func (s *Service) createNextVersion(ctx context.Context, environment string, key string, version int, value []byte) (Metadata, error) {
	for attempt := 0; attempt < 100; attempt++ {
		metadata := Metadata{
			CreatedAt:   s.now().UTC(),
			Environment: environment,
			Key:         key,
			SwarmName:   swarmName(environment, key, version),
			Version:     version,
		}

		s.logger.InfoContext(ctx, "creating secret", "environment", environment, "key", key, "version", version)
		result, err := s.runner.Run(ctx, "docker", []string{"secret", "create", metadata.SwarmName, "-"}, command.RunOptions{Stdin: bytes.NewReader(value)})
		if err == nil {
			return metadata, nil
		}

		output := strings.TrimSpace(string(result.Output))
		if strings.Contains(strings.ToLower(output), "already exists") {
			version++
			continue
		}
		if output != "" {
			return Metadata{}, fmt.Errorf("create Docker Swarm secret %q: %w: %s", metadata.SwarmName, err, output)
		}
		return Metadata{}, fmt.Errorf("create Docker Swarm secret %q: %w", metadata.SwarmName, err)
	}

	return Metadata{}, fmt.Errorf("create Docker Swarm secret for %q: no available version found", key)
}

func (s *Service) List(ctx context.Context, environment string) ([]Metadata, error) {
	if err := validateIdentifier("environment", environment); err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "listing secrets", "environment", environment)
	return s.store.List(s.config.StateDir, environment)
}

func (s *Service) Latest(ctx context.Context, environment string, key string) (Metadata, error) {
	if err := validateIdentifier("environment", environment); err != nil {
		return Metadata{}, err
	}
	if err := validateIdentifier("secret key", key); err != nil {
		return Metadata{}, err
	}

	items, err := s.store.List(s.config.StateDir, environment)
	if err != nil {
		return Metadata{}, err
	}

	var latest Metadata
	for _, item := range items {
		if item.Key == key && item.Version > latest.Version {
			latest = item
		}
	}
	if latest.Version == 0 {
		return Metadata{}, fmt.Errorf("secret %q was not found in environment %q", key, environment)
	}

	s.logger.InfoContext(ctx, "resolved secret", "environment", environment, "key", key, "version", latest.Version)
	return latest, nil
}

func validateIdentifier(label string, value string) error {
	if !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s %q must contain only letters, numbers, dots, underscores, or hyphens and start with a letter or number", label, value)
	}
	return nil
}

func nextVersion(items []Metadata, key string) int {
	version := 0
	for _, item := range items {
		if item.Key == key && item.Version > version {
			version = item.Version
		}
	}
	return version + 1
}

func swarmName(environment string, key string, version int) string {
	return fmt.Sprintf("noops_%s_%s_v%d", environment, key, version)
}
