package deploy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"gopkg.in/yaml.v3"
)

func (s *Service) ensureNetwork(ctx context.Context, network string) error {
	if network == "" {
		return fmt.Errorf("environment network is required")
	}
	if driver, scope, err := s.inspectNetwork(ctx, network); err == nil {
		return validateOverlayNetwork(network, driver, scope)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	result, err := s.runner.Run(ctx, "docker", []string{"network", "create", "--driver", "overlay", network}, command.RunOptions{LogCommand: true})
	if err != nil {
		// Another deployment may have created the same shared network.
		if driver, scope, inspectErr := s.inspectNetwork(ctx, network); inspectErr == nil {
			return validateOverlayNetwork(network, driver, scope)
		}
		return fmt.Errorf("create overlay network %q: %w: %s", network, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}

func (s *Service) inspectNetwork(ctx context.Context, name string) (string, string, error) {
	result, err := s.runner.Run(ctx, "docker", []string{"network", "inspect", "--format", "{{.Driver}}|{{.Scope}}", name}, command.RunOptions{})
	if err != nil {
		return "", "", err
	}
	parts := strings.Split(strings.TrimSpace(string(result.Output)), "|")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid network inspection for %q", name)
	}
	return parts[0], parts[1], nil
}

func validateOverlayNetwork(name, driver, scope string) error {
	if driver != "overlay" || scope != "swarm" {
		return fmt.Errorf("network %q must be a Swarm overlay network; found driver=%q scope=%q", name, driver, scope)
	}
	return nil
}

// External shared networks persist independently of the stack. Compose-owned
// networks are left to Docker so their declared driver and IPAM options apply.
func (s *Service) ensureStackNetworks(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read stack networks: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("decode stack networks: %w", err)
	}
	root := documentRoot(&doc)
	services := mappingValue(root, "services")
	definitions := mappingValue(root, "networks")
	seen := map[string]bool{}
	for i := 1; services != nil && i < len(services.Content); i += 2 {
		attachments := mappingValue(services.Content[i], "networks")
		if attachments == nil {
			continue
		}
		step := 1
		if attachments.Kind == yaml.MappingNode {
			step = 2
		}
		for j := 0; j < len(attachments.Content); j += step {
			key := attachments.Content[j].Value
			definition := mappingValue(definitions, key)
			external := mappingValue(definition, "external")
			if external == nil || external.Tag != "!!bool" || external.Value != "true" {
				continue
			}
			name := key
			if explicit := mappingValue(definition, "name"); explicit != nil {
				name = explicit.Value
			}
			if seen[name] {
				continue
			}
			if err := s.ensureNetwork(ctx, name); err != nil {
				return err
			}
			seen[name] = true
		}
	}
	return nil
}
