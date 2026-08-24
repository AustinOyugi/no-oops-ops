package local

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

//go:embed templates/nginx-stack.yml.tmpl
var nginxStackTemplateContents string

func (h *Host) nginxDir() string {
	return filepath.Join(h.stateDir, "nginx")
}

func (h *Host) nginxStackPath() string {
	return filepath.Join(h.nginxDir(), "stack.yml")
}

func (h *Host) nginxConfigDir() string {
	return filepath.Join(h.nginxDir(), "conf")
}

func (h *Host) nginxConfigPath() string {
	return filepath.Join(h.nginxConfigDir(), "routes.conf")
}

type nginxStackTemplateData struct {
	HTTPPort     string
	HTTPSPort    string
	NetworkName  string
	ConfigPath   string
	InternalHost string
}

func (h *Host) WriteNginxStack(ctx context.Context) error {
	path := h.nginxStackPath()
	h.logger.InfoContext(ctx, "writing nginx stack", "path", path)

	if err := os.MkdirAll(h.nginxDir(), stateDirMode); err != nil {
		return install.PrerequisiteError{
			Check: install.StepWriteNginxStack,
			Err:   fmt.Errorf("create nginx state dir %q: %w", h.nginxDir(), err),
		}
	}
	if err := os.MkdirAll(h.nginxConfigDir(), stateDirMode); err != nil {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("create nginx config dir %q: %w", h.nginxConfigDir(), err)}
	}
	if _, err := os.Stat(h.nginxConfigPath()); os.IsNotExist(err) {
		if err := os.WriteFile(h.nginxConfigPath(), []byte(defaultNginxConfig), installMetadataFileMode); err != nil {
			return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("write nginx config %q: %w", h.nginxConfigPath(), err)}
		}
	} else if err != nil {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("inspect nginx config %q: %w", h.nginxConfigPath(), err)}
	}

	rendered, err := renderTemplate("nginx-stack.yml.tmpl", nginxStackTemplateContents, nginxStackTemplateData{
		HTTPPort:     h.nginxHTTPPort,
		HTTPSPort:    h.nginxHTTPSPort,
		NetworkName:  h.networkName,
		ConfigPath:   h.nginxConfigDir(),
		InternalHost: internalIngressHost,
	})
	if err != nil {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("render nginx stack: %w", err)}
	}

	if err := os.WriteFile(path, append(rendered, '\n'), installMetadataFileMode); err != nil {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("write nginx stack %q: %w", path, err)}
	}
	return nil
}

const internalIngressHost = "ingress.noops.internal"

const defaultNginxConfig = `server {
  listen 80 default_server;
  listen [::]:80 default_server;
  server_name _;

  location = /__noops/health {
    add_header Content-Type text/plain;
    return 200 'ok\n';
  }

  location / { return 404; }
}
`

func (h *Host) InspectNginxService(ctx context.Context) error {
	result, err := h.runner.Run(ctx, "docker", []string{"service", "inspect", h.nginxService}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("inspect nginx service %q: %w: %s", h.nginxService, err, strings.TrimSpace(string(result.Output)))
	}
	return nil
}

func (h *Host) EnsureNginx(ctx context.Context) error {
	h.logger.InfoContext(ctx, "ensuring nginx ingress", "name", h.nginxName, "http_port", h.nginxHTTPPort, "https_port", h.nginxHTTPSPort)
	if h.InspectNginxService(ctx) == nil {
		h.nginxReady = true
		return nil
	}

	result, err := h.runner.Run(ctx, "docker", []string{"stack", "deploy", "--detach=true", "--compose-file", h.nginxStackPath(), h.nginxName}, command.RunOptions{
		StreamOutput: true,
		LogCommand:   true,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	})
	if err != nil {
		return install.PrerequisiteError{Check: install.StepEnsureNginx, Err: fmt.Errorf("deploy nginx stack %q: %w: %s", h.nginxName, err, strings.TrimSpace(string(result.Output)))}
	}
	h.nginxReady = h.InspectNginxService(ctx) == nil
	return nil
}
