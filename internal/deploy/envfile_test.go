package deploy

import "testing"

func TestLoadOptionalEnvFileAllowsAnEmptyPath(t *testing.T) {
	envFile, err := LoadOptionalEnvFile("")
	if err != nil {
		t.Fatalf("LoadOptionalEnvFile returned error: %v", err)
	}
	if len(envFile.Sections) != 0 {
		t.Errorf("sections = %#v, want no sections", envFile.Sections)
	}
}
