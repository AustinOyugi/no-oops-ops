package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigInstallUpgrade(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.yml")
	run := func(verb string) error {
		cmd := NewRootCommand(context.Background())
		cmd.SetArgs([]string{"--workspace", root, "config", verb, "db", source})
		return cmd.Execute()
	}
	if err := os.WriteFile(source, []byte("sections: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("upgrade"); err == nil {
		t.Fatal("upgrade before install accepted")
	}
	if err := run("install"); err != nil {
		t.Fatal(err)
	}
	if err := run("install"); err == nil {
		t.Fatal("duplicate install accepted")
	}
	updated := "sections:\n- items:\n  - {key: HOST, value: db}\n"
	if err := os.WriteFile(source, []byte(updated), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("upgrade"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "configs", "db.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != updated {
		t.Fatal("upgrade did not replace config")
	}
}
