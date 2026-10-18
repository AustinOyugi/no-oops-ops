package environment

import (
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
	file, err := LoadForManifest(filepath.Join(dir, "apps/api/app.yml")+"#api", "env.yml")
	if err != nil {
		t.Fatal(err)
	}
	if got := Resolve(file, "prod", nil).Values["DB_HOST"]; got != "database" {
		t.Fatalf("value=%q", got)
	}
	file, err = LoadForManifest("unused", "")
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
		{"invalid type", "", `from_config: [config.yml]`, "must be a file path"},
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
