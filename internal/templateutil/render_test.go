package templateutil

import (
	"strconv"
	"strings"
	"testing"
)

func TestEnvironmentPreservesTemplateAndShellSyntax(t *testing.T) {
	value := "$HOME {{.Secret}} \"quoted\" \\path"
	raw, err := Environment(map[string]string{"PAYLOAD": value}, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "PAYLOAD="+value+"\n" {
		t.Fatalf("runtime value changed: %q", raw)
	}
	value += "\nsecond line\tend"
	quoted, err := Environment(map[string]string{"PAYLOAD": value}, true)
	if err != nil {
		t.Fatal(err)
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(string(quoted), "PAYLOAD="), "\n")
	decoded, err := strconv.Unquote(encoded)
	if err != nil || decoded != value {
		t.Fatalf("build value did not round trip: %q: %v", decoded, err)
	}
	if strings.Count(string(quoted), "\n") != 1 {
		t.Fatalf("newline escaped incorrectly: %q", quoted)
	}
}
