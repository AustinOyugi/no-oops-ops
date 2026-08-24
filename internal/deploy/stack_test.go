package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

func TestRenderComposeStackPreservesUnknownComposeFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte(`services:
  api:
    image: example/api:latest
    entrypoint: ./entrypoint.sh
    command: [serve]
    healthcheck: {test: [CMD, "true"]}
    env_file: [.env]
    environment: {LOG_LEVEL: info}
    labels: {owner: platform}
    ports: ["9090:8080"]
    networks: [platform]
    volumes: [./data:/data]
    configs: [app-config]
    secrets: [existing-secret]
    deploy:
      replicas: 2
      placement: {constraints: [node.role == worker]}
      resources: {limits: {memory: 256M}}
      restart_policy: {condition: any}
      update_config: {parallelism: 2}
    x-future-compose-field: retained
    x-noops:
      service: {internal_port: 8080}
      env: {file: noops.env.yml}
networks: {platform: {external: true}}
volumes: {data: {}}
configs: {app-config: {file: ./config.yml}}
secrets: {existing-secret: {external: true}}
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderComposeStack(m, "secure.registry/api@sha256:abc", []SecretBinding{{EnvKey: "DB_PASSWORD", SwarmName: "noops_dev_DB_PASSWORD_v1"}}, WrapperConfig{}, "noops-dev", "dev-api", "/state/.env")
	if err != nil {
		t.Fatal(err)
	}
	output := string(rendered)
	for _, want := range []string{
		"entrypoint: ./entrypoint.sh", "command: [serve]", "owner: platform", "x-future-compose-field: retained",
		"constraints: [node.role == worker]", "memory: 256M", "condition: any", "parallelism: 2",
		"image: secure.registry/api@sha256:abc", "source: noops_dev_DB_PASSWORD_v1", "external: true",
		"- noops-dev",
		filepath.Join(filepath.Dir(path), ".env"), filepath.Join(filepath.Dir(path), "data") + ":/data", filepath.Join(filepath.Dir(path), "config.yml"),
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered Compose stack missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "x-noops") {
		t.Errorf("generated stack must not contain x-noops:\n%s", output)
	}
}

func TestReleaseStackNameUsesReleaseSpecificSwarmSafeSuffix(t *testing.T) {
	if got, want := releaseStackName("prod", "lango", "2026-08-24T10:30:00Z"), "prod-lango-r2026-08-24t103000z"; got != want {
		t.Errorf("releaseStackName() = %q, want %q", got, want)
	}
}

func TestCandidateStackNameIsUniquePerDeployment(t *testing.T) {
	tag := "20260824-133728"
	first := candidateStackName("dev", "lango", tag, time.Date(2026, 8, 24, 13, 37, 28, 1, time.UTC))
	second := candidateStackName("dev", "lango", tag, time.Date(2026, 8, 24, 13, 37, 28, 2, time.UTC))
	if first == second {
		t.Fatalf("candidate stack names must differ for repeated deployments: %q", first)
	}
	if !strings.HasPrefix(first, "dev-lango-r20260824-133728-") {
		t.Errorf("candidate stack name = %q, want release-specific prefix", first)
	}
}

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

func TestRenderStackTemplateRendersSwarmUpdateAndRollbackPolicies(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName:             "prod-lango",
		Image:                   "registry/lango:v1",
		Network:                 "noops-net",
		Parallelism:             1,
		RolloutDelay:            "10s",
		RolloutOrder:            "start-first",
		RolloutMonitor:          "1m50s",
		MaxFailureRatio:         0,
		FailureAction:           "rollback",
		RollbackParallelism:     1,
		RollbackDelay:           "0s",
		RollbackOrder:           "start-first",
		RollbackMonitor:         "1m50s",
		RollbackMaxFailureRatio: 0,
		RollbackFailureAction:   "pause",
	})
	if err != nil {
		t.Fatalf("render stack: %v", err)
	}

	output := string(rendered)
	for _, want := range []string{
		"update_config:",
		"monitor: 1m50s",
		"max_failure_ratio: 0",
		"failure_action: rollback",
		"rollback_config:",
		"failure_action: pause",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
}
