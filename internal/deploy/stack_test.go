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
			EnvKey:     "DATABASE_URL",
			SecretName: "DATABASE_URL_SECRET",
			SwarmName:  "noops_prod_DATABASE_URL_v2",
		}},
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"source: noops_prod_DATABASE_URL_v2",
		"target: DATABASE_URL_SECRET",
		"noops_prod_DATABASE_URL_v2:",
		"external: true",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}

func TestRenderStackTemplateWrapperMode(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:        "dev-lango",
		Image:              "ghcr.io/austinoyugi/noops-wrapper:latest",
		Network:            "noops-net",
		UseWrapper:         true,
		WrapperImage:       "ghcr.io/austinoyugi/noops-wrapper:latest",
		OriginalEntrypoint: `["java","-jar"]`,
		OriginalCmd:        `["app.jar"]`,
		SecretMappingsJSON: `[{"env_key":"REDIS_PASSWORD","secret_name":"REDIS_PASSWORD_SECRET"}]`,
		Secrets: []SecretBinding{{
			EnvKey:     "REDIS_PASSWORD",
			SecretName: "REDIS_PASSWORD_SECRET",
			SwarmName:  "noops_dev_REDIS_PASSWORD_SECRET_v1",
		}},
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		`entrypoint: ["/bin/sh", "/bootstrap.sh"]`,
		"command: []",
		"NOOPS_ORIGINAL_ENTRYPOINT_JSON:",
		`"java","-jar"`,
		"NOOPS_SECRET_MAPPINGS_JSON:",
		"REDIS_PASSWORD_FILE: /run/secrets/REDIS_PASSWORD_SECRET",
		"source: noops_dev_REDIS_PASSWORD_SECRET_v1",
		"target: REDIS_PASSWORD_SECRET",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}

func TestRenderStackTemplateFileModeSecretTargetIsSecretName(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName: "prod-lango",
		Image:       "registry/lango:v1",
		Network:     "noops-net",
		UseWrapper:  false,
		Secrets: []SecretBinding{{
			EnvKey:     "REDIS_PASSWORD",
			SecretName: "REDIS_PASSWORD_SECRET",
			SwarmName:  "noops_prod_REDIS_PASSWORD_SECRET_v1",
		}},
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	if strings.Contains(output, "entrypoint:") {
		t.Error("file mode should not override entrypoint")
	}
	if !strings.Contains(output, "target: REDIS_PASSWORD_SECRET") {
		t.Errorf("file mode should mount secret to its backing secret name, got:\n%s", output)
	}
	if strings.Contains(output, "REDIS_PASSWORD_FILE:") {
		t.Errorf("file mode should not inject a _FILE environment variable, got:\n%s", output)
	}
}
