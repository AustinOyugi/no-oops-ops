package local

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/ingress"
	"github.com/AustinOyugi/no-oops-ops/internal/nginxconfig"
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

// Opt-in: creates an isolated bridge network and containers, with no published
// ports and no changes to the host's Swarm configuration or existing services.
func TestNginxDockerMigrationAndAtomicConfig(t *testing.T) {
	if os.Getenv("NOOPS_NGINX_DOCKER_TEST") != "1" {
		t.Skip("set NOOPS_NGINX_DOCKER_TEST=1 for Docker integration")
	}
	root := t.TempDir()
	h := NewHost(slog.Default(), filepath.Join(root, "state"), filepath.Join(root, "data"), "test", "shared", "registry", "5000", "nginx", "80", "443")
	h.runner = &installedNetworkRunner{}
	if err := h.WriteNginxStack(context.Background()); err != nil {
		t.Fatal(err)
	}
	docker := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		return string(output), err
	}
	must := func(args ...string) string {
		t.Helper()
		output, err := docker(args...)
		if err != nil {
			t.Fatalf("docker %v: %v\n%s", args, err, output)
		}
		return output
	}
	rendered := must("stack", "config", "--compose-file", h.nginxStackPath())
	if !strings.Contains(rendered, "/etc/noops/nginx/start-nginx.sh") {
		t.Fatalf("shell variables lost during Compose interpolation: %s", rendered)
	}
	name := fmt.Sprintf("noops-test-%d", time.Now().UnixNano())
	must("network", "create", name)
	t.Cleanup(func() {
		if out, err := docker("network", "rm", name); err != nil {
			t.Errorf("cleanup network: %v: %s", err, out)
		}
	})
	must("run", "-d", "--name", name+"-backend", "--network", name, "--network-alias", "legacy-api", "--entrypoint", "/bin/sh", "nginx:1.28-alpine", "-c", "sleep 120")
	t.Cleanup(func() {
		if out, err := docker("rm", "-f", name+"-backend"); err != nil {
			t.Errorf("cleanup backend: %v: %s", err, out)
		}
	})
	// The migration wrapper retains existing static upstream configuration.
	if err := os.WriteFile(h.nginxConfigPath(), []byte("server { listen 80 default_server; location / { proxy_pass http://legacy-api:8080; } }"), 0600); err != nil {
		t.Fatal(err)
	}
	mounts := []string{"--network", name, "--volume", h.nginxDir() + ":/etc/noops/nginx:ro", "--volume", h.nginxConfigDir() + ":/etc/nginx/conf.d:ro"}
	check := append([]string{"run", "--rm"}, mounts...)
	check = append(check, "nginx:1.28-alpine", "nginx", "-c", nginxconfig.ContainerPath, "-t")
	must(check...)
	routes := []ingress.Route{{Environment: "prod", App: "api", Domain: "api.example.test", PathPrefix: "/", Service: "legacy-api", Port: 8080}}
	body, err := ingress.RenderConfig(routes)
	if err != nil {
		t.Fatal(err)
	}
	content, err := nginxconfig.Render(nginxconfig.Data{HTTPConfig: string(body)})
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(h.nginxDir(), "nginx.conf")
	if err := state.WriteFile(main, content, 0600); err != nil {
		t.Fatal(err)
	}
	start := append([]string{"run", "-d", "--name", name + "-nginx"}, mounts...)
	start = append(start, "nginx:1.28-alpine", "nginx", "-c", nginxconfig.ContainerPath, "-g", "daemon off;")
	must(start...)
	t.Cleanup(func() {
		if out, err := docker("rm", "-f", name+"-nginx"); err != nil {
			t.Errorf("cleanup nginx: %v: %s", err, out)
		}
	})
	must("exec", name+"-nginx", "nginx", "-c", nginxconfig.ContainerPath, "-t")
	// Replacing a file under the mounted directory is visible to the container.
	routes[0].Domain = "updated.example.test"
	body, err = ingress.RenderConfig(routes)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := nginxconfig.Render(nginxconfig.Data{HTTPConfig: string(body)})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WriteFile(main, updated, 0600); err != nil {
		t.Fatal(err)
	}
	dump := must("exec", name+"-nginx", "nginx", "-c", nginxconfig.ContainerPath, "-T")
	if !strings.Contains(dump, "updated.example.test") {
		t.Fatal("container pinned to previous config inode")
	}
	must("exec", name+"-nginx", "nginx", "-c", nginxconfig.ContainerPath, "-s", "reload")
	invalid := filepath.Join(h.nginxDir(), "invalid.conf")
	invalidContent, err := nginxconfig.Render(nginxconfig.Data{HTTPConfig: "server { listen 80 default_server; }\nserver { listen 80 default_server; }"})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.WriteFile(invalid, invalidContent, 0600); err != nil {
		t.Fatal(err)
	}
	output, err := docker("exec", name+"-nginx", "nginx", "-c", "/etc/noops/nginx/invalid.conf", "-t")
	if err == nil || !strings.Contains(output, "duplicate default server") {
		t.Fatalf("invalid candidate accepted: %v: %s", err, output)
	}
	preserved, err := os.ReadFile(main)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != string(updated) {
		t.Fatal("invalid candidate changed active config")
	}
	must("exec", name+"-nginx", "wget", "-q", "--spider", "http://127.0.0.1/__noops/health")
}

func TestNginxDockerSupervisorDrainsRequests(t *testing.T) {
	if os.Getenv("NOOPS_NGINX_DOCKER_TEST") != "1" {
		t.Skip("set NOOPS_NGINX_DOCKER_TEST=1 for Docker integration")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	script, err := renderTemplate("nginx-start.sh.tmpl", nginxStartTemplateContents, struct{ ConfigPath string }{nginxconfig.ContainerPath})
	if err != nil {
		t.Fatal(err)
	}
	config, err := nginxconfig.Render(nginxconfig.Data{HTTPConfig: "server { listen 80; root /etc/noops/nginx; location /payload { limit_rate 32k; } }"})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"start-nginx.sh": script, "nginx.conf": config, "payload": []byte(strings.Repeat("x", 96*1024))} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0644); err != nil {
			t.Fatal(err)
		}
	}
	docker := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		return string(out), err
	}
	must := func(args ...string) string {
		t.Helper()
		out, err := docker(args...)
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, out)
		}
		return out
	}
	name := fmt.Sprintf("noops-drain-%d", time.Now().UnixNano())
	must("network", "create", name)
	t.Cleanup(func() { docker("network", "rm", name) })
	must("run", "-d", "--name", name, "--network", name, "--network-alias", "ingress", "--volume", root+":/etc/noops/nginx:ro", "--stop-signal", "SIGQUIT", "--entrypoint", "/bin/sh", "nginx:1.28-alpine", "/etc/noops/nginx/start-nginx.sh")
	t.Cleanup(func() { docker("rm", "-f", name) })
	for i := 0; i < 50; i++ {
		if _, err := docker("exec", name, "wget", "-q", "--spider", "http://127.0.0.1/payload"); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	client := name + "-client"
	must("run", "-d", "--name", client, "--network", name, "--volume", root+":/result", "--entrypoint", "/bin/sh", "nginx:1.28-alpine", "-c", "touch /result/started; wget -q -O /result/received http://ingress/payload")
	t.Cleanup(func() { docker("rm", "-f", client) })
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(root, "received")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	must("stop", "--time", "15", name)
	if exit := strings.TrimSpace(must("wait", client)); exit != "0" {
		t.Fatalf("client exited %s", exit)
	}
	body, err := os.ReadFile(filepath.Join(root, "received"))
	if err != nil || len(body) != 96*1024 {
		t.Fatalf("request interrupted: bytes=%d error=%v", len(body), err)
	}
	logs := must("logs", name)
	if !strings.Contains(logs, "gracefully shutting down") {
		t.Fatalf("no graceful shutdown: %s", logs)
	}
	// A dead master must not leave the supervisor sleeping for five minutes.
	must("start", name)
	deadline = time.Now().Add(10 * time.Second)
	for {
		if _, err := docker("exec", name, "nginx", "-c", nginxconfig.ContainerPath, "-s", "stop"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restarted master did not start")
		}
		time.Sleep(100 * time.Millisecond)
	}
	must("wait", name)
}
