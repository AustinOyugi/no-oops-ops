package local

import (
	"context"
	"errors"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderNginxStack(t *testing.T) {
	rendered, err := renderTemplate("nginx-stack.yml.tmpl", nginxStackTemplateContents, nginxStackTemplateData{
		Replicas:      nginxReplicas,
		HTTPPort:      "8080",
		HTTPSPort:     "8443",
		NetworkName:   "noops-net",
		ConfigPath:    "/var/lib/noops/nginx/conf",
		MainConfigDir: "/var/lib/noops/nginx",
		InternalHost:  "ingress.noops.internal",
		NginxService:  "noops-nginx_nginx",
	})
	if err != nil {
		t.Fatalf("render nginx stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"image: nginx:1.28-alpine",
		"published: 8080",
		"published: 8443",
		`- "/var/lib/noops/nginx/conf:/etc/nginx/conf.d:ro"`,
		`"noops-net":`,
		`- "ingress.noops.internal"`,
		"wget -q --spider http://127.0.0.1/__noops/health || exit 1",
		"external: true",
		`entrypoint: ["/bin/sh"]`,
		"command:",
		`command: ["/etc/noops/nginx/start-nginx.sh"]`,
		"replicas: 2",
		"order: start-first",
		"failure_action: rollback",
		"stop_grace_period: 2m",
		"while :; do certbot renew --webroot --webroot-path /var/www/certbot;",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "/var/run/docker.sock") {
		t.Errorf("rendered stack must not mount the Docker socket:\n%s", output)
	}
}

func TestWriteNginxStackPreservesInstalledRoutesAndNetworks(t *testing.T) {
	root := t.TempDir()
	h := NewHost(slog.Default(), filepath.Join(root, "state"), filepath.Join(root, "data"), "test", "noops-net", "registry", "5000", "nginx", "80", "443")
	h.runner = &installedNetworkRunner{}
	if err := os.MkdirAll(h.nginxConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"conf/routes.conf":   defaultNginxConfig,
		"conf/default.conf":  defaultNginxConfig,
		"conf/external.conf": "include /etc/nginx/conf.d/external/*.conf;",
		"networks.json":      `{"noops-prod":true,"noops-canary":true,"noops-net":true,"unused":false}`,
	} {
		if err := os.WriteFile(filepath.Join(h.nginxDir(), name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := h.WriteNginxStack(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(h.nginxConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("legacy routes.conf recreated: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(h.nginxConfigDir(), "external.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "include /etc/nginx/conf.d/external/*.conf;" {
		t.Fatalf("routes overwritten: %s", content)
	}
	stack, err := os.ReadFile(h.nginxStackPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, network := range []string{"noops-prod", "noops-canary"} {
		if strings.Count(string(stack), `"`+network+`":`) != 2 {
			t.Fatalf("network missing attachment or declaration: %s", stack)
		}
	}
	if strings.Contains(string(stack), "unused") {
		t.Fatal("detached network included")
	}
}

type invalidNginxRunner struct {
	calls [][]string
	valid bool
}

func (r *invalidNginxRunner) Run(_ context.Context, name string, args []string, _ command.RunOptions) (command.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if args[0] == "info" {
		return command.Result{Output: []byte("node-id")}, nil
	}
	if args[0] == "service" && args[1] == "inspect" {
		return command.Result{Output: []byte("[]")}, nil
	}
	if args[0] == "service" && args[1] == "ps" {
		if r.valid {
			return command.Result{Output: []byte("Complete 1 second ago|")}, nil
		}
		return command.Result{Output: []byte("Failed 1 second ago|task: non-zero exit (1)")}, nil
	}
	if args[0] == "service" && args[1] == "logs" {
		return command.Result{Output: []byte("duplicate default server in routes.conf:2")}, nil
	}
	return command.Result{}, nil
}

func TestEnsureNginxRejectsInvalidConfigBeforeDeploy(t *testing.T) {
	r := &invalidNginxRunner{}
	h := &Host{runner: r, logger: slog.Default(), stateDir: t.TempDir(), dataDir: t.TempDir()}
	err := h.EnsureNginx(context.Background())
	if err == nil || !strings.Contains(err.Error(), "duplicate default server") {
		t.Fatalf("error = %v", err)
	}
	var validated bool
	for _, call := range r.calls {
		if call[1] == "stack" {
			t.Fatalf("stack changed after validation failure: %v", r.calls)
		}
		if len(call) > 2 && call[1] == "service" && call[2] == "create" {
			validated = call[len(call)-1] == "-t"
			if !strings.Contains(strings.Join(call, " "), "--mode replicated-job") {
				t.Fatalf("validation cannot join overlay networks: %v", call)
			}
		}
	}
	if !validated {
		t.Fatalf("missing validation: %v", r.calls)
	}
	last := r.calls[len(r.calls)-1]
	if last[1] != "service" || last[2] != "rm" {
		t.Fatalf("job not cleaned up: %v", last)
	}

}

type installedNetworkRunner struct {
	live    bool
	missing string
}

func (r *installedNetworkRunner) Run(_ context.Context, _ string, args []string, _ command.RunOptions) (command.Result, error) {
	if args[0] == "service" {
		if r.live {
			return command.Result{Output: []byte(`[{"Target":"live-id"}]`)}, nil
		}
		return command.Result{Output: []byte("[]")}, nil
	}
	if args[0] == "network" && len(args) > 3 && args[2] == "--format" {
		return command.Result{Output: []byte("noops-live")}, nil
	}
	if args[0] == "network" && args[len(args)-1] == r.missing {
		return command.Result{Output: []byte("network " + r.missing + " not found")}, errors.New("exit status 1")
	}
	return command.Result{}, nil
}

func TestNginxValidationUsesApplicationNetworksAndCleansUp(t *testing.T) {
	r := &invalidNginxRunner{valid: true}
	h := &Host{runner: r, logger: slog.Default(), stateDir: t.TempDir(), dataDir: t.TempDir(), networkName: "shared", nginxService: "nginx_nginx"}
	if err := os.MkdirAll(h.nginxDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.nginxDir(), "networks.json"), []byte(`{"prod":true,"canary":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.validateNginx(context.Background()); err != nil {
		t.Fatal(err)
	}
	var creation string
	for _, call := range r.calls {
		if len(call) > 2 && call[2] == "create" {
			creation = strings.Join(call, " ")
		}
	}
	for _, want := range []string{"--network shared", "--network prod", "--network canary", "--constraint node.id==node-id"} {
		if !strings.Contains(creation, want) {
			t.Fatalf("missing %q: %s", want, creation)
		}
	}
	last := r.calls[len(r.calls)-1]
	if last[2] != "rm" {
		t.Fatalf("missing cleanup: %v", last)
	}
}

func TestWriteNginxStackPreservesLiveNetworksWithoutCache(t *testing.T) {
	root := t.TempDir()
	h := NewHost(slog.Default(), filepath.Join(root, "state"), filepath.Join(root, "data"), "test", "shared", "registry", "5000", "nginx", "80", "443")
	h.runner = &installedNetworkRunner{live: true, missing: "deleted"}
	if err := os.MkdirAll(h.nginxDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.nginxDir(), "networks.json"), []byte(`{"deleted":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := h.WriteNginxStack(context.Background()); err != nil {
		t.Fatal(err)
	}
	stack, err := os.ReadFile(h.nginxStackPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(stack), `"noops-live":`) != 2 || strings.Contains(string(stack), `"deleted":`) {
		t.Fatalf("incorrect preserved networks: %s", stack)
	}
}
