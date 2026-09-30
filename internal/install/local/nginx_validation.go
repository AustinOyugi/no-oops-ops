package local

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/nginxconfig"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

// A Swarm job can join non-attachable overlay networks, unlike docker run.
// Validate in the same DNS and mount context without touching the serving stack.
func (h *Host) validateNginx(ctx context.Context) error {
	networks, err := h.ingressNetworks(ctx)
	if err != nil {
		return err
	}
	result, err := h.runner.Run(ctx, "docker", []string{"info", "--format", "{{.Swarm.NodeID}}"}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("find nginx validation node: %w: %s", err, result.Output)
	}
	node := strings.TrimSpace(string(result.Output))
	if node == "" {
		return fmt.Errorf("nginx validation requires a Swarm node ID")
	}
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	name := fmt.Sprintf("noops-nginx-check-%x", nonce)
	args := []string{"service", "create", "--detach=true", "--name", name, "--mode", "replicated-job", "--restart-condition", "none", "--constraint", "node.id=" + node}
	for _, mount := range [][2]string{{h.nginxDir(), "/etc/noops/nginx"}, {h.nginxConfigDir(), "/etc/nginx/conf.d"}, {h.nginxACMEWebroot(), "/var/www/certbot"}, {h.nginxCertificateDir(), "/etc/letsencrypt"}, {h.nginxImportedCertificateDir(), "/etc/noops/certificates"}} {
		args = append(args, "--mount", "type=bind,source="+mount[0]+",target="+mount[1]+",readonly")
	}
	args = append(args, "--network", h.networkName)
	for _, network := range networks {
		args = append(args, "--network", network)
	}
	args = append(args, "nginx:1.28-alpine", "nginx", "-c", nginxconfig.ContainerPath, "-t")
	// Also clean up if creation fails after Docker accepted the service.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if result, err := h.runner.Run(cleanupCtx, "docker", []string{"service", "rm", name}, command.RunOptions{}); err != nil {
			h.logger.WarnContext(cleanupCtx, "could not remove nginx validation job", "service", name, "error", err, "output", string(result.Output))
		}
	}()
	result, err = h.runner.Run(ctx, "docker", args, command.RunOptions{LogCommand: true})
	if err != nil {
		return fmt.Errorf("create nginx validation job: %w: %s", err, result.Output)
	}
	checkCtx, cancel := context.WithTimeout(ctx, serviceReadyTimeout)
	defer cancel()
	for {
		result, err := h.runner.Run(checkCtx, "docker", []string{"service", "ps", "--no-trunc", "--format", "{{.CurrentState}}|{{.Error}}", name}, command.RunOptions{})
		if err != nil {
			return fmt.Errorf("inspect nginx validation job: %w: %s", err, result.Output)
		}
		for _, line := range strings.Split(string(result.Output), "\n") {
			if strings.HasPrefix(line, "Complete ") {
				return nil
			}
			if strings.HasPrefix(line, "Failed ") || strings.HasPrefix(line, "Rejected ") {
				logs, _ := h.runner.Run(checkCtx, "docker", []string{"service", "logs", "--raw", name}, command.RunOptions{})
				return fmt.Errorf("validate nginx configuration before stack deploy: %s: %s", strings.TrimSpace(line), strings.TrimSpace(string(logs.Output)))
			}
		}
		select {
		case <-checkCtx.Done():
			return fmt.Errorf("wait for nginx validation: %w", checkCtx.Err())
		case <-time.After(time.Second):
		}
	}
}
