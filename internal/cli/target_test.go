package cli

import "testing"

func TestTargetSupportsBothArgumentForms(t *testing.T) {
	for _, args := range [][]string{{"nats"}, {"dev", "nats"}} {
		got := target("dev", args, targetFlags{service: "server"})
		if got.Environment != "dev" || got.App != "nats" || got.Service != "server" {
			t.Fatalf("%v resolved to %+v", args, got)
		}
	}
}
