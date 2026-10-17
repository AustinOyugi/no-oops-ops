package environment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLayersPrecedence(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"db.yml":      "sections:\n- items:\n  - {key: HOST, value: database}\n  - {key: PASSWORD, from_secret: db-password}\n  - {key: MODE, value: default}\n",
		"cache.yml":   "sections:\n- items:\n  - {key: HOST, value: cache}\n",
		"service.yml": "sections:\n- items:\n  - {key: HOST, values: {prod: service}}\n  - {key: PASSWORD, value: overridden}\n  - {key: MODE, from_secret: mode}\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file, err := LoadLayers(filepath.Join(dir, "app.yml")+"#api", []string{"db.yml", "cache.yml"}, "service.yml")
	if err != nil {
		t.Fatal(err)
	}
	got := Resolve(file, "prod", []string{"PASSWORD", "MODE"})
	if got.Values["HOST"] != "service" || got.Values["PASSWORD"] != "overridden" {
		t.Fatalf("unexpected values: %#v", got)
	}
	if _, ok := got.Values["MODE"]; ok {
		t.Fatal("secret override retained ordinary value")
	}
	if len(got.SecretRefs) != 1 || got.SecretRefs[0].SecretName != "mode" {
		t.Fatalf("unexpected secrets: %#v", got.SecretRefs)
	}
	file, err = LoadLayers(filepath.Join(dir, "app.yml"), []string{"db.yml", "cache.yml"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if Resolve(file, "prod", nil).Values["HOST"] != "cache" {
		t.Fatal("later config must win")
	}
	if _, err := LoadLayers(filepath.Join(dir, "app.yml"), []string{"missing.yml"}, ""); err == nil {
		t.Fatal("missing config accepted")
	}
}
