package deploy

import (
	"strings"
	"testing"
)

func TestRenderStackTemplateMountsExternalSecrets(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName: "prod-lango",
		Image:       "registry/lango:v1",
		Network:     "noops-net",
		Secrets: []SecretBinding{{
			EnvKey:    "DATABASE_URL",
			SwarmName: "noops_prod_DATABASE_URL_v2",
		}},
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"source: noops_prod_DATABASE_URL_v2",
		"target: DATABASE_URL",
		"noops_prod_DATABASE_URL_v2:",
		"external: true",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}
