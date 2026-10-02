package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

func TestCatalogRecoversLegacyServices(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  nyota:\n    manifest: compose.yml\n"), 0600)
	os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services:\n  nyota:\n    image: nginx\n  nyota-web:\n    image: nginx\n"), 0600)
	rows := []Row{
		{Service: "prod-nyota-web-r20260827-033832-1787804872142815460_app", Untracked: true},
		{Service: "prod-nyota_prod-nyota", Untracked: true},
		{Service: "noops-build-25662d30678d", Untracked: true},
		{Service: "prod-nyota-web-other_app", Untracked: true},
	}
	resolveCatalogServices(config.Config{Workspace: root}, rows)
	if rows[0].Untracked || rows[0].App != "nyota-web" || rows[0].Environment != "prod" {
		t.Fatalf("web ownership: %+v", rows[0])
	}
	if rows[1].Untracked || rows[1].App != "nyota" {
		t.Fatalf("stable ownership: %+v", rows[1])
	}
	if !rows[2].Untracked || !rows[3].Untracked {
		t.Fatalf("unrelated names mapped: %+v", rows)
	}
	os.WriteFile(filepath.Join(root, "apps.yml"), []byte("apps:\n  nyota:\n    manifest: compose.yml\n  duplicate:\n    manifest: compose.yml\n"), 0600)
	rows[0].Untracked = true
	resolveCatalogServices(config.Config{Workspace: root}, rows)
	if !rows[0].Untracked {
		t.Fatal("ambiguous catalog mapped")
	}
}
