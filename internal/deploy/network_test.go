package deploy

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

func TestEnsureStackNetworks(t *testing.T) {
	for _, tc := range []struct {
		name, existing               string
		race, createFails, wantError bool
	}{
		{name: "create missing"},
		{name: "reuse existing", existing: "overlay|swarm"},
		{name: "reject incompatible", existing: "bridge|local", wantError: true},
		{name: "concurrent creation", race: true},
		{name: "create failure", createFails: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := `#!/bin/sh
printf '%s\n' "$*" >> "$NETWORK_COMMANDS"
case "$1 $2" in
'network inspect')
  if [ -n "$EXISTING_NETWORK" ]; then echo "$EXISTING_NETWORK"; exit 0; fi
  if [ -f "$NETWORK_MARKER" ]; then echo 'overlay|swarm'; exit 0; fi
  exit 1;;
'network create')
  if [ "$CREATE_FAILS" = yes ]; then exit 1; fi
  touch "$NETWORK_MARKER"
  if [ "$NETWORK_RACE" = yes ]; then exit 1; fi
  echo network-id;;
*) exit 2;;
esac
`
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("NETWORK_COMMANDS", filepath.Join(dir, "commands"))
			t.Setenv("NETWORK_MARKER", filepath.Join(dir, "exists"))
			t.Setenv("EXISTING_NETWORK", tc.existing)
			t.Setenv("NETWORK_RACE", "no")
			t.Setenv("CREATE_FAILS", "no")
			if tc.race {
				t.Setenv("NETWORK_RACE", "yes")
			}
			if tc.createFails {
				t.Setenv("CREATE_FAILS", "yes")
			}
			stack := filepath.Join(dir, "stack.yml")
			// Both aliases reference one external network. Unused and stack-owned
			// networks must not be provisioned by Noops.
			data := `services:
  api:
    networks:
      database: {aliases: [api]}
      duplicate: null
      private: null
networks:
  database: {external: true, name: shared-data}
  duplicate: {external: true, name: shared-data}
  unused: {external: true, name: unused-network}
  private: {driver: overlay}
`
			if err := os.WriteFile(stack, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			s := &Service{runner: command.NewRunner(slog.New(slog.NewTextHandler(io.Discard, nil)))}
			err := s.ensureStackNetworks(context.Background(), stack)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v expected failure=%t", err, tc.wantError)
			}
			commands, err := os.ReadFile(filepath.Join(dir, "commands"))
			if err != nil {
				t.Fatal(err)
			}
			log := string(commands)
			if strings.Contains(log, "unused-network") || strings.Contains(log, "private") {
				t.Fatalf("provisioned unrelated network: %s", log)
			}
			creates := strings.Count(log, "network create --driver overlay shared-data")
			wantCreates := 1
			if tc.existing != "" {
				wantCreates = 0
			}
			if creates != wantCreates {
				t.Fatalf("creates=%d want=%d: %s", creates, wantCreates, log)
			}
		})
	}
}
