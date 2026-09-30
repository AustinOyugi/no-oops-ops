package local

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderNginxStack(t *testing.T) {
	rendered, err := renderTemplate("nginx-stack.yml.tmpl", nginxStackTemplateContents, nginxStackTemplateData{
		HTTPPort:     "8080",
		HTTPSPort:    "8443",
		NetworkName:  "noops-net",
		ConfigPath:   "/var/lib/noops/nginx/conf",
		InternalHost: "ingress.noops.internal",
		NginxService: "noops-nginx_nginx",
	})
	if err != nil {
		t.Fatalf("render nginx stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"image: nginx:1.28-alpine",
		`- "8080:80"`,
		`- "8443:443"`,
		`- "/var/lib/noops/nginx/conf:/etc/nginx/conf.d:ro"`,
		`"noops-net":`,
		`- "ingress.noops.internal"`,
		"wget -q --spider http://127.0.0.1/__noops/health || exit 1",
		"external: true",
		`entrypoint: ["/bin/sh", "-c"]`,
		"command:",
		"- >-",
		"nginx -g 'daemon off;' & nginx_pid=$!",
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
