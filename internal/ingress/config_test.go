package ingress

import (
	"strings"
	"testing"
)

func TestRenderConfigGroupsRoutesByDomainAndPrefersLongerPrefixes(t *testing.T) {
	rendered, err := RenderConfig([]Route{
		{Environment: "dev", App: "api", Domain: "example.test", PathPrefix: "/", Service: "dev-api_dev-api", Port: 8080},
		{Environment: "dev", App: "admin", Domain: "example.test", PathPrefix: "/admin", Service: "dev-admin_dev-admin", Port: 9090},
	})
	if err != nil {
		t.Fatalf("render config: %v", err)
	}
	output := string(rendered)
	if strings.Count(output, "server_name example.test;") != 1 {
		t.Fatalf("expected one virtual host:\n%s", output)
	}
	admin := strings.Index(output, "location /admin")
	root := strings.Index(output[admin+len("location /admin"):], "location / {")
	if root >= 0 {
		root += admin + len("location /admin")
	}
	if admin < 0 || root < 0 || admin > root {
		t.Errorf("expected /admin before /:\n%s", output)
	}
	for _, want := range []string{
		"location = /__noops/health",
		"set $upstream dev-api_dev-api;",
		"proxy_pass http://$upstream:8080$request_uri;",
		"proxy_set_header X-Forwarded-Proto $scheme;",
		"server_name ingress.noops.internal;",
		"location /dev/admin/ {",
		"rewrite ^/dev/admin/?(.*)$ /$1 break;",
		"proxy_pass http://$upstream:9090$uri$is_args$args;",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered config does not contain %q:\n%s", want, output)
		}
	}
}

func TestRenderFilesSeparatesDomainsAndInternalServices(t *testing.T) {
	files, err := RenderFiles([]Route{
		{Environment: "dev", App: "lango", Domain: "lango.example.test", PathPrefix: "/", Service: "dev-lango_dev-lango", Port: 8080},
		{Environment: "prod", App: "lango", Domain: "lango.example.test", PathPrefix: "/api", Service: "prod-lango_prod-lango", Port: 8080},
		{Environment: "dev", App: "accounts", Domain: "accounts.example.test", PathPrefix: "/", Service: "dev-accounts_dev-accounts", Port: 8081},
	})
	if err != nil {
		t.Fatalf("render files: %v", err)
	}
	for _, path := range []string{
		"external/lango-example-test.conf",
		"external/accounts-example-test.conf",
		"internal/dev-lango.conf",
		"internal/prod-lango.conf",
		"internal/dev-accounts.conf",
	} {
		if _, ok := files[path]; !ok {
			t.Errorf("missing generated nginx file %q", path)
		}
	}
}
