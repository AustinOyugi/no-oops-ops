package environment

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeEnvFixture(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFromConfigSelectsOnlyRequestedKeys(t *testing.T) {
	dir := t.TempDir()
	writeEnvFixture(t, dir, "configs/db.yml", `sections:
- items:
  - {key: HOST, values: {prod: database, dev: localhost}}
  - {key: PASSWORD, from_secret: database-password}
  - {key: UNUSED, from_config: missing.yml}
`)
	path := writeEnvFixture(t, dir, "apps/api/env.yml", `sections:
- items:
  - {key: HOST, from_config: ../../configs/db.yml}
  - key: DB_HOST
    from_config: {file: ../../configs/db.yml, key: HOST}
  - {key: TOKEN, from_config: {file: ../../configs/db.yml, key: PASSWORD}}
`)
	file, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	result := Resolve(file, "prod", []string{"TOKEN"})
	if !reflect.DeepEqual(result.Values, map[string]string{"HOST": "database", "DB_HOST": "database"}) {
		t.Fatalf("values: %#v", result.Values)
	}
	if len(result.SecretRefs) != 1 || result.SecretRefs[0] != (SecretRef{Key: "TOKEN", SecretName: "database-password"}) {
		t.Fatalf("secrets: %#v", result.SecretRefs)
	}
	if got := Resolve(file, "dev", nil); got.Values["HOST"] != "localhost" || len(got.SecretRefs) != 0 {
		t.Fatalf("dev: %#v", got)
	}
}

func TestFromConfigLocalOverrides(t *testing.T) {
	dir := t.TempDir()
	writeEnvFixture(t, dir, "defaults.yml", `sections:
- items:
  - {key: HOST, value: fallback, values: {dev: shared-dev, prod: shared-prod}}
`)
	for _, test := range []struct {
		name, extra string
		want        map[string]string
	}{
		{"environment", "values: {dev: local-dev}", map[string]string{"dev": "local-dev", "prod": "shared-prod", "other": "fallback"}},
		{"default", "value: local", map[string]string{"dev": "local", "prod": "local", "other": "local"}},
		{"empty", `value: ""`, map[string]string{"dev": "", "prod": "", "other": ""}},
		{"empty environment", `values: {dev: ""}`, map[string]string{"dev": "", "prod": "shared-prod", "other": "fallback"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := writeEnvFixture(t, dir, "env.yml", "sections:\n- items:\n  - key: HOST\n    from_config: defaults.yml\n    "+test.extra+"\n")
			file, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			for env, want := range test.want {
				got := Resolve(file, env, nil)
				value, ok := got.Values["HOST"]
				if !ok || value != want {
					t.Fatalf("%s: %#v, want %q", env, got, want)
				}
			}
		})
	}
}

func TestFromConfigNestedPaths(t *testing.T) {
	dir := t.TempDir()
	writeEnvFixture(t, dir, "configs/base.yml", "sections:\n- items:\n  - {key: HOST, value: database}\n")
	writeEnvFixture(t, dir, "configs/alias.yml", "sections:\n- items:\n  - {key: RENAMED, from_config: {file: base.yml, key: HOST}}\n")
	writeEnvFixture(t, dir, "apps/api/env.yml", "sections:\n- items:\n  - {key: DB_HOST, from_config: {file: ../../configs/alias.yml, key: RENAMED}}\n")
	file, err := LoadForManifest(filepath.Join(dir, "apps/api/app.yml")+"#api", "env.yml", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := Resolve(file, "prod", nil).Values["DB_HOST"]; got != "database" {
		t.Fatalf("value=%q", got)
	}
	file, err = LoadForManifest("unused", "", "")
	if err != nil || len(file.Sections) != 0 {
		t.Fatalf("optional file: %#v %v", file, err)
	}
}

func TestFromConfigErrors(t *testing.T) {
	for _, test := range []struct{ name, config, reference, want string }{
		{"missing file", "", `from_config: missing.yml`, "read env file"},
		{"missing key", "  - {key: OTHER, value: x}\n", `from_config: config.yml`, "not found"},
		{"duplicate key", "  - {key: HOST, value: a}\n  - {key: HOST, value: b}\n", `from_config: config.yml`, "duplicated"},
		{"cycle", "  - {key: HOST, from_config: env.yml}\n", `from_config: config.yml`, "cycle"},
		{"conflicting sources", "  - {key: HOST, value: a}\n", "from_config: config.yml\n    from_secret: secret", "cannot be combined"},
		{"empty path", "", `from_config: ""`, "must not be empty"},
		{"invalid type", "", `from_config: [config.yml]`, "must be an alias"},
		{"unknown field", "", `from_config: {path: config.yml}`, "unknown from_config field"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			writeEnvFixture(t, dir, "config.yml", "sections:\n- items:\n"+test.config)
			path := writeEnvFixture(t, dir, "env.yml", "sections:\n- items:\n  - key: HOST\n    "+test.reference+"\n")
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %s", err, test.want)
			}
		})
	}
}

func TestInstalledAliasesUseSelectedWorkspace(t *testing.T) {
	envDir := t.TempDir()
	source := writeEnvFixture(t, envDir, "source-with-different-name.yml", "sections:\n- items:\n  - {key: HOST, value: first}\n  - {key: EXTRA, value: unused}\n")
	first, second := t.TempDir(), t.TempDir()
	if _, err := Install(context.Background(), first, "database", source); err != nil {
		t.Fatal(err)
	}
	other := writeEnvFixture(t, envDir, "second.yml", "sections:\n- items:\n  - {key: HOST, value: second}\n")
	if _, err := Install(context.Background(), second, "database", other); err != nil {
		t.Fatal(err)
	}
	path := writeEnvFixture(t, envDir, "env.yml", `sections:
- items:
  - {key: HOST, from_config: database}
  - {key: APP_HOST, from_config: {alias: database, key: HOST}, values: {dev: local}}
`)
	for root, want := range map[string]string{first: "first", second: "second"} {
		file, err := LoadWithWorkspace(path, root)
		if err != nil {
			t.Fatal(err)
		}
		got := Resolve(file, "prod", nil).Values
		if len(got) != 2 || got["HOST"] != want || got["APP_HOST"] != want {
			t.Fatalf("workspace %s: %#v", root, got)
		}
		if Resolve(file, "dev", nil).Values["APP_HOST"] != "local" {
			t.Fatal("alias override ignored")
		}
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "workspace is required") {
		t.Fatalf("missing workspace: %v", err)
	}
	if _, err := LoadWithWorkspace(path, t.TempDir()); err == nil || !strings.Contains(err.Error(), "noops config install database") {
		t.Fatalf("missing alias: %v", err)
	}
	if _, err := Upgrade(context.Background(), first, "database", other); err != nil {
		t.Fatal(err)
	}
	file, err := LoadWithWorkspace(path, first)
	if err != nil {
		t.Fatal(err)
	}
	if Resolve(file, "prod", nil).Values["HOST"] != "second" {
		t.Fatal("alias did not pick up upgraded config")
	}
}

func TestNestedAliasAndCycle(t *testing.T) {
	root := t.TempDir()
	writeEnvFixture(t, root, "configs/database.yml", "sections:\n- items:\n  - {key: HOST, from_config: {alias: defaults, key: SERVER}}\n")
	writeEnvFixture(t, root, "configs/defaults.yml", "sections:\n- items:\n  - {key: SERVER, value: database}\n")
	path := writeEnvFixture(t, root, "apps/api/env.yml", "sections:\n- items:\n  - {key: HOST, from_config: database}\n")
	file, err := LoadForManifest(filepath.Join(root, "apps/api/app.yml"), "env.yml", root)
	if err != nil {
		t.Fatal(err)
	}
	if Resolve(file, "prod", nil).Values["HOST"] != "database" {
		t.Fatal("nested alias failed")
	}
	writeEnvFixture(t, root, "configs/defaults.yml", "sections:\n- items:\n  - {key: SERVER, from_config: {alias: database, key: HOST}}\n")
	if _, err := LoadWithWorkspace(path, root); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("alias cycle: %v", err)
	}
}

func TestInvalidAliasReferences(t *testing.T) {
	for _, ref := range []string{`{alias: ../outside}`, `{alias: database, file: database.yml}`} {
		path := writeEnvFixture(t, t.TempDir(), "env.yml", "sections:\n- items:\n  - key: HOST\n    from_config: "+ref+"\n")
		if _, err := Load(path); err == nil {
			t.Fatalf("invalid alias accepted: %s", ref)
		}
	}
}
