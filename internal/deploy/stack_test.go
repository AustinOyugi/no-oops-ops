package deploy

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
	"gopkg.in/yaml.v3"
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
		filepath.Join(filepath.Dir(path), ".env"), filepath.Join(filepath.Dir(path), "data") + ":/data", filepath.Join(filepath.Dir(path), "config.yml"),
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered Compose stack missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "x-noops") {
		t.Errorf("generated stack must not contain x-noops:\n%s", output)
	}
	var generated yaml.Node
	if err := yaml.Unmarshal(rendered, &generated); err != nil {
		t.Fatal(err)
	}
	root := documentRoot(&generated)
	attachments := mappingValue(mappingValue(mappingValue(root, "services"), "dev-api"), "networks")
	if attachments == nil || len(attachments.Content) != 2 || attachments.Content[0].Value != "platform" || attachments.Content[1].Value != "noops-dev" {
		t.Fatal("expected original and environment network attachments")
	}
	if mappingValue(mappingValue(root, "networks"), "platform") == nil {
		t.Fatal("original network definition was removed")
	}
}

func TestEnvironmentNetworkMergesComposeNetworks(t *testing.T) {
	for _, tc := range []struct {
		name, attachments, definitions string
		mapping, conflict              bool
	}{
		{name: "list", attachments: "[shared-data]", definitions: "{shared-data: {external: true, name: shared-data}}"},
		{name: "mapping and aliases", attachments: "{shared-data: {aliases: [shared-postgres], ipv4_address: 10.1.0.5}}", definitions: "{shared-data: {driver: overlay, attachable: true, ipam: {config: [{subnet: 10.1.0.0/24}]}}}", mapping: true},
		{name: "existing environment attachment", attachments: "[shared-data, noops-prod]", definitions: "{shared-data: {external: true}, noops-prod: {external: true, name: noops-prod}}"},
		{name: "environment aliases", attachments: "{noops-prod: {aliases: [database]}, shared-data: null}", definitions: "{shared-data: {external: true}}", mapping: true},
		{name: "empty", attachments: "null", definitions: "null"},
		{name: "conflicting managed network", attachments: "[noops-prod]", definitions: "{noops-prod: {name: another-network, external: true}}", conflict: true},
		{name: "invalid attachments", attachments: "shared-data", definitions: "{}", conflict: true},
		{name: "invalid definitions", attachments: "[]", definitions: "[shared-data]", conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte("services:\n  db:\n    networks: "+tc.attachments+"\nnetworks: "+tc.definitions+"\n"), &doc); err != nil {
				t.Fatal(err)
			}
			root := documentRoot(&doc)
			service := mappingValue(mappingValue(root, "services"), "db")
			beforeShared, _ := yaml.Marshal(mappingValue(mappingValue(root, "networks"), "shared-data"))
			beforeAliases, _ := yaml.Marshal(mappingValue(mappingValue(service, "networks"), "shared-data"))
			if err := setEnvironmentNetwork(root, service, "noops-prod"); err != nil {
				if !tc.conflict {
					t.Fatal(err)
				}
				return
			} else if tc.conflict {
				t.Fatal("expected conflicting network error")
			}
			// Applying the merge twice must neither duplicate nor alter declarations.
			once, _ := yaml.Marshal(root)
			if err := setEnvironmentNetwork(root, service, "noops-prod"); err != nil {
				t.Fatal(err)
			}
			twice, _ := yaml.Marshal(root)
			if string(once) != string(twice) {
				t.Fatal("merge is not idempotent")
			}
			attachments := mappingValue(service, "networks")
			count := 0
			if tc.mapping {
				for i := 0; i < len(attachments.Content); i += 2 {
					if attachments.Content[i].Value == "noops-prod" {
						count++
					}
				}
			} else {
				for _, entry := range attachments.Content {
					if entry.Value == "noops-prod" {
						count++
					}
				}
			}
			if count != 1 {
				t.Fatalf("environment attachments=%d", count)
			}
			afterShared, _ := yaml.Marshal(mappingValue(mappingValue(root, "networks"), "shared-data"))
			afterAliases, _ := yaml.Marshal(mappingValue(attachments, "shared-data"))
			if string(beforeShared) != string(afterShared) || string(beforeAliases) != string(afterAliases) {
				t.Fatal("shared network definition or options changed")
			}
			if mappingValue(mappingValue(mappingValue(root, "networks"), "noops-prod"), "external").Value != "true" {
				t.Fatal("environment network must be external")
			}
		})
	}
}

func TestRenderComposeStackEnvSecretDoesNotInjectFileVariable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	data := []byte(`services:
  rabbitmq:
    image: rabbitmq:3.13-management-alpine
    hostname: rabbitmq
    environment:
      RABBITMQ_DEFAULT_USER: icpak
    x-noops:
      service:
        internal_port: 5672
      env:
        file: env.yml
        secrets:
          resolution: env
          resolvable:
            - RABBITMQ_DEFAULT_PASS
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	binding := SecretBinding{
		EnvKey:     "RABBITMQ_DEFAULT_PASS",
		SecretName: "ICPAK_RABBITMQ_PASSWORD",
		SwarmName:  "noops_prod_ICPAK_RABBITMQ_PASSWORD_v1",
	}
	wrapper := WrapperConfig{
		UseWrapper:    true,
		WrapperImage:  "127.0.0.1:5000/rabbitmq-wrapper:latest",
		EffectiveExec: EffectiveExecution{Entrypoint: []string{"docker-entrypoint.sh"}, Cmd: []string{"rabbitmq-server"}},
		SecretMappings: []SecretMapping{{
			EnvKey:     binding.EnvKey,
			SecretName: binding.SecretName,
		}},
	}

	rendered, err := renderComposeStack(m, "rabbitmq:3.13-management-alpine", []SecretBinding{binding}, wrapper, "noops-prod", "prod-rabbitmq", "/state/.env")
	if err != nil {
		t.Fatal(err)
	}
	output := string(rendered)
	for _, want := range []string{
		"image: 127.0.0.1:5000/rabbitmq-wrapper:latest",
		"RABBITMQ_DEFAULT_USER: icpak",
		"NOOPS_SECRET_MAPPINGS: RABBITMQ_DEFAULT_PASS=/run/secrets/RABBITMQ_DEFAULT_PASS",
		"source: noops_prod_ICPAK_RABBITMQ_PASSWORD_v1",
		"target: RABBITMQ_DEFAULT_PASS",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered Compose stack missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "RABBITMQ_DEFAULT_PASS_FILE") {
		t.Errorf("env mode should not inject RabbitMQ's deprecated _FILE variable, got:\n%s", output)
	}
}

func TestReleaseStackNameUsesReleaseSpecificSwarmSafeSuffix(t *testing.T) {
	if got, want := releaseStackName("prod", "sample", "2026-08-24T10:30:00Z"), "prod-sample-r2026-08-24t103000z"; got != want {
		t.Errorf("releaseStackName() = %q, want %q", got, want)
	}
}

func TestCandidateStackNameIsUniquePerDeployment(t *testing.T) {
	tag := "20260824-133728"
	first := candidateStackName("dev", "sample", tag, time.Date(2026, 8, 24, 13, 37, 28, 1, time.UTC))
	second := candidateStackName("dev", "sample", tag, time.Date(2026, 8, 24, 13, 37, 28, 2, time.UTC))
	if first == second {
		t.Fatalf("candidate stack names must differ for repeated deployments: %q", first)
	}
	if !strings.HasPrefix(first, "dev-sample-r20260824-133728-") {
		t.Errorf("candidate stack name = %q, want release-specific prefix", first)
	}
}

func TestCandidateStackNameFitsDockerSwarmServiceLimit(t *testing.T) {
	createdAt := time.Date(2026, 8, 27, 19, 38, 37, 1, time.UTC)
	stack := candidateStackName(
		"prod",
		"sample-builder-service",
		"20260827-025448",
		createdAt,
	)
	service := stack + "_" + blueGreenCandidateServiceName
	if len(service) > maxSwarmServiceNameLength {
		t.Fatalf("Swarm service name %q has length %d, want at most %d", service, len(service), maxSwarmServiceNameLength)
	}
	fullName := releaseStackName("prod", "sample-builder-service", "20260827-025448-"+fmt.Sprintf("%d", createdAt.UnixNano()))
	digest := sha256.Sum256([]byte(fullName))
	if !strings.HasSuffix(stack, "-"+fmt.Sprintf("%x", digest[:5])) {
		t.Fatalf("candidate stack %q should retain a digest suffix after truncation", stack)
	}
}

func TestRenderStackTemplateMountsExternalSecrets(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName: "prod-sample",
		Image:       "registry/sample:v1",
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
		ServiceName:     "dev-sample",
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
		"source: noops_dev_REDIS_PASSWORD_SECRET_v1",
		"target: REDIS_PASSWORD",
		"mode: 0444",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered stack does not contain %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "REDIS_PASSWORD_FILE:") {
		t.Errorf("env mode should not inject a _FILE environment variable, got:\n%s", output)
	}
}

func TestRenderStackTemplateFileModeSecretTargetIsEnvKey(t *testing.T) {
	rendered, err := renderStackTemplate(stackTemplateData{
		ServiceName: "prod-sample",
		Image:       "registry/sample:v1",
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
		ServiceName:             "prod-sample",
		Image:                   "registry/sample:v1",
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
