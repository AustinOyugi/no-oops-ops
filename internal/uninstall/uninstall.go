package uninstall

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type Options struct {
	Purge bool
}

type Metadata struct {
	StateDir string
	DataDir  string
	Network  Network
	Registry Registry
	Nginx    Nginx
}

type Network struct {
	Name string
}

type Registry struct {
	Name string
}

type Nginx struct {
	Name string
}

type Host interface {
	LoadInstallation(context.Context) (Metadata, error)
	RemoveApps(context.Context, Metadata) error
	RemoveRegistry(context.Context, Metadata) error
	RemoveNginx(context.Context, Metadata) error
	RemoveNetwork(context.Context, Metadata) error
	RemoveGeneratedState(context.Context, Metadata) error
	RemoveData(context.Context, Metadata) error
	RemoveInstallMetadata(context.Context, Metadata) error
}

type Service struct {
	host Host
}

func New(host Host) (*Service, error) {
	if host == nil {
		return nil, fmt.Errorf("uninstall host is required")
	}
	return &Service{host: host}, nil
}

func (s *Service) Run(ctx context.Context, options Options) error {
	metadata, err := s.host.LoadInstallation(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("load installation metadata: %w", err)
	}

	steps := []struct {
		name string
		run  func(context.Context, Metadata) error
	}{
		{name: "remove managed application stacks", run: s.host.RemoveApps},
		{name: "remove registry stack", run: s.host.RemoveRegistry},
		{name: "remove nginx stack", run: s.host.RemoveNginx},
		{name: "remove shared network", run: s.host.RemoveNetwork},
		{name: "remove generated state", run: s.host.RemoveGeneratedState},
	}
	if options.Purge {
		steps = append(steps, struct {
			name string
			run  func(context.Context, Metadata) error
		}{name: "remove persistent data", run: s.host.RemoveData})
	}
	steps = append(steps, struct {
		name string
		run  func(context.Context, Metadata) error
	}{name: "remove installation metadata", run: s.host.RemoveInstallMetadata})

	for _, step := range steps {
		if err := step.run(ctx, metadata); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}
