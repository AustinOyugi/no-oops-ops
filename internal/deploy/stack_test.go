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
		"target: DATABASE_URL",
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
		ServiceName:     "dev-lango",
		Image:           "127.0.0.1:5000/noops-runtime:latest",
		Network:         "noops-net",
		UseWrapper:      true,
		OriginalCommand: `["java","-jar","app.jar"]`,
		SecretMappings:  "REDIS_PASSWORD=/run/secrets/REDIS_PASSWORD",
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
		`command: ["java","-jar","app.jar"]`,
		"NOOPS_SECRET_MAPPINGS:",
		"REDIS_PASSWORD_FILE: /run/secrets/REDIS_PASSWORD",
		"source: noops_dev_REDIS_PASSWORD_SECRET_v1",
		"target: REDIS_PASSWORD",
		"mode: 0444",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}

func TestRenderStackTemplateFileModeSecretTargetIsEnvKey(t *testing.T) {
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
	if !strings.Contains(output, "target: REDIS_PASSWORD") {
		t.Errorf("file mode should mount secret to its environment key, got:\n%s", output)
	}
	if strings.Contains(output, "REDIS_PASSWORD_FILE:") {
		t.Errorf("file mode should not inject a _FILE environment variable, got:\n%s", output)
	}
	if !strings.Contains(output, "mode: 0444") {
		t.Errorf("file mode should make secrets readable by non-root containers, got:\n%s", output)
	}
}

func TestRenderStackTemplateRendersNamedVolumesAndBindMounts(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:  "dev-keycloak",
		Image:        "registry/keycloak:v1",
		Network:      "noops-net",
		Volumes:      []string{"keycloak-data:/opt/keycloak/data", "./themes:/opt/keycloak/themes:ro", "/srv/keycloak:/backup"},
		NamedVolumes: namedVolumes([]string{"keycloak-data:/opt/keycloak/data", "./themes:/opt/keycloak/themes:ro", "/srv/keycloak:/backup"}),
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"volumes:\n      - keycloak-data:/opt/keycloak/data",
		"- ./themes:/opt/keycloak/themes:ro",
		"- /srv/keycloak:/backup",
		"\nvolumes:\n  keycloak-data:",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
	for _, unexpected := range []string{"  ./themes:", "  /srv/keycloak:"} {
		if strings.Contains(output, unexpected) {
			t.Errorf("bind mount unexpectedly declared as a named volume %q:\n%s", unexpected, output)
		}
	}
}
