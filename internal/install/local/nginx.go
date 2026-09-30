package local

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/ingressnet"
	"github.com/AustinOyugi/no-oops-ops/internal/install"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
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
func (h *Host) nginxACMEWebroot() string    { return filepath.Join(h.dataDir, "nginx", "acme-webroot") }
func (h *Host) nginxCertificateDir() string { return filepath.Join(h.dataDir, "nginx", "letsencrypt") }
func (h *Host) nginxImportedCertificateDir() string {
	return filepath.Join(h.dataDir, "nginx", "certificates")
}

type nginxStackTemplateData struct {
	HTTPPort               string
	HTTPSPort              string
	EnvironmentNetworks    []string
	NetworkName            string
	ConfigPath             string
	InternalHost           string
	ACMEWebroot            string
	CertificateDir         string
	ImportedCertificateDir string
	NginxService           string
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
	for _, certificatePath := range []string{h.nginxACMEWebroot(), h.nginxCertificateDir(), h.nginxImportedCertificateDir()} {
		if err := os.MkdirAll(certificatePath, stateDirMode); err != nil {
			return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("create nginx certificate directory %q: %w", certificatePath, err)}
		}
	}
	_, defaultErr := os.Stat(filepath.Join(h.nginxConfigDir(), "default.conf"))
	if defaultErr != nil && !os.IsNotExist(defaultErr) {
		return fmt.Errorf("inspect nginx default config: %w", defaultErr)
	}
	if _, err := os.Stat(h.nginxConfigPath()); os.IsNotExist(err) && os.IsNotExist(defaultErr) {
		if err := state.WriteFile(h.nginxConfigPath(), []byte(defaultNginxConfig), installMetadataFileMode); err != nil {
			return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("write nginx config %q: %w", h.nginxConfigPath(), err)}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("inspect nginx config %q: %w", h.nginxConfigPath(), err)}
	}

	// Remove only the bootstrap config left by older reinstall behavior.
	// Other legacy route files may contain active upstreams and must be preserved.
	if defaultErr == nil {
		legacy, err := os.ReadFile(h.nginxConfigPath())
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read legacy nginx config: %w", err)
		}
		if string(legacy) == defaultNginxConfig {
			if err := os.Remove(h.nginxConfigPath()); err != nil {
				return fmt.Errorf("remove duplicate bootstrap nginx config: %w", err)
			}
		}
	}

	networks := make(map[string]bool)
	networkData, err := os.ReadFile(filepath.Join(h.nginxDir(), "networks.json"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read ingress networks: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(networkData, &networks); err != nil {
			return fmt.Errorf("decode ingress networks: %w", err)
		}
	}
	liveNetworks, err := ingressnet.Attached(ctx, h.runner, h.nginxService)
	if err != nil {
		return err
	}
	for network := range liveNetworks {
		networks[network] = true
	}
	var environmentNetworks []string
	for network, attached := range networks {
		if attached && network != h.networkName {
			result, err := h.runner.Run(ctx, "docker", []string{"network", "inspect", network}, command.RunOptions{})
			if err != nil {
				if strings.Contains(string(result.Output), "not found") || strings.Contains(string(result.Output), "No such network") {
					continue
				}
				return fmt.Errorf("inspect preserved ingress network %q: %w: %s", network, err, result.Output)
			}
			environmentNetworks = append(environmentNetworks, network)
		}
	}
	sort.Strings(environmentNetworks)
	rendered, err := renderTemplate("nginx-stack.yml.tmpl", nginxStackTemplateContents, nginxStackTemplateData{
		EnvironmentNetworks:    environmentNetworks,
		HTTPPort:               h.nginxHTTPPort,
		HTTPSPort:              h.nginxHTTPSPort,
		NetworkName:            h.networkName,
		ConfigPath:             h.nginxConfigDir(),
		InternalHost:           internalIngressHost,
		ACMEWebroot:            h.nginxACMEWebroot(),
		CertificateDir:         h.nginxCertificateDir(),
		ImportedCertificateDir: h.nginxImportedCertificateDir(),
		NginxService:           h.nginxService,
	})
	if err != nil {
		return install.PrerequisiteError{Check: install.StepWriteNginxStack, Err: fmt.Errorf("render nginx stack: %w", err)}
	}

	if err := state.WriteFile(path, append(rendered, '\n'), installMetadataFileMode); err != nil {
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
	// Validate with the same image and mounts before changing the running stack.
	result, err := h.runner.Run(ctx, "docker", []string{
		"run", "--rm", "--network", "none",
		"--volume", h.nginxConfigDir() + ":/etc/nginx/conf.d:ro",
		"--volume", h.nginxACMEWebroot() + ":/var/www/certbot:ro",
		"--volume", h.nginxCertificateDir() + ":/etc/letsencrypt:ro",
		"--volume", h.nginxImportedCertificateDir() + ":/etc/noops/certificates:ro",
		"nginx:1.28-alpine", "nginx", "-t",
	}, command.RunOptions{LogCommand: true})
	if err != nil {
		return install.PrerequisiteError{Check: install.StepEnsureNginx, Err: fmt.Errorf("validate nginx configuration before stack deploy: %w: %s", err, strings.TrimSpace(string(result.Output)))}
	}
	// Stack deploy is idempotent. Always apply the rendered stack so updates to
	// the nginx or certbot definition take effect on an existing installation.
	result, err = h.runner.Run(ctx, "docker", []string{"stack", "deploy", "--detach=true", "--compose-file", h.nginxStackPath(), h.nginxName}, command.RunOptions{
		StreamOutput: true,
		LogCommand:   true,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	})
	if err != nil {
		return install.PrerequisiteError{Check: install.StepEnsureNginx, Err: fmt.Errorf("deploy nginx stack %q: %w: %s", h.nginxName, err, strings.TrimSpace(string(result.Output)))}
	}
	if err := h.waitForServiceReady(ctx, h.nginxService); err != nil {
		h.nginxReady = false
		return install.PrerequisiteError{Check: install.StepEnsureNginx, Err: err}
	}
	if err := h.waitForServiceReady(ctx, h.nginxName+"_certbot"); err != nil {
		h.nginxReady = false
		return install.PrerequisiteError{Check: install.StepEnsureNginx, Err: err}
	}
	h.nginxReady = true
	return nil
}
