package local

import (
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
		"--deploy-hook 'docker service update --force noops-nginx_nginx'",
		"/var/run/docker.sock:/var/run/docker.sock",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}
