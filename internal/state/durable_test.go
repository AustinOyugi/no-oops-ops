package state

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteFileReplacesContentAndPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	if err := WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "second" {
		t.Errorf("content = %q, want second", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestAcquireLockHonorsContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operation.lock")
	unlock, err := AcquireLock(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := AcquireLock(ctx, path); err == nil {
		t.Fatal("AcquireLock succeeded while held")
	}
}
