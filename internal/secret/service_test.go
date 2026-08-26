package secret

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
)

type recordingRunner struct {
	args          []string
	history       [][]string
	stdin         string
	runs          int
	failFirstWith string
}

func (r *recordingRunner) Run(_ context.Context, _ string, args []string, opts command.RunOptions) (command.Result, error) {
	r.args = append([]string(nil), args...)
	r.history = append(r.history, append([]string(nil), args...))
	r.runs++
	if opts.Stdin != nil {
		data, err := io.ReadAll(opts.Stdin)
		if err != nil {
			return command.Result{}, err
		}
		r.stdin = string(data)
	}
	if r.runs == 1 && r.failFirstWith != "" {
		return command.Result{Output: []byte(r.failFirstWith)}, errors.New("exit status 1")
	}
	return command.Result{}, nil
}

func TestSetCreatesVersionedSwarmSecretAndPersistsOnlyMetadata(t *testing.T) {
	runner := &recordingRunner{}
	service := &Service{
		config: config.Config{StateDir: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		runner: runner,
		store:  newFilesystemStore(),
		now:    func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) },
	}

	result, err := service.Set(context.Background(), "prod", "DATABASE_URL", bytes.NewBufferString("postgres://secret"))
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if result.SwarmName != "noops_prod_DATABASE_URL_v1" {
		t.Errorf("SwarmName = %q", result.SwarmName)
	}
	if got, want := runner.args, []string{"secret", "create", "noops_prod_DATABASE_URL_v1", "-"}; !equalStrings(got, want) {
		t.Errorf("docker args = %v, want %v", got, want)
	}
	if runner.stdin != "postgres://secret" {
		t.Errorf("stdin = %q", runner.stdin)
	}

	items, err := service.List(context.Background(), "prod")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 1 || items[0].Version != 1 || items[0].Key != "DATABASE_URL" {
		t.Errorf("List = %#v", items)
	}
}

func TestSetIncrementsExistingSecretVersion(t *testing.T) {
	runner := &recordingRunner{}
	service := &Service{
		config: config.Config{StateDir: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		runner: runner,
		store:  newFilesystemStore(),
		now:    time.Now,
	}

	if _, err := service.Set(context.Background(), "prod", "DATABASE_URL", bytes.NewBufferString("one")); err != nil {
		t.Fatalf("first Set returned error: %v", err)
	}
	result, err := service.Set(context.Background(), "prod", "DATABASE_URL", bytes.NewBufferString("two"))
	if err != nil {
		t.Fatalf("second Set returned error: %v", err)
	}
	if result.Version != 2 || result.SwarmName != "noops_prod_DATABASE_URL_v2" {
		t.Errorf("result = %#v", result)
	}

	latest, err := service.Latest(context.Background(), "prod", "DATABASE_URL")
	if err != nil {
		t.Fatalf("Latest returned error: %v", err)
	}
	if latest.Version != 2 || latest.SwarmName != "noops_prod_DATABASE_URL_v2" {
		t.Errorf("latest = %#v", latest)
	}
}

func TestSetSkipsSwarmSecretVersionsMissingFromLocalMetadata(t *testing.T) {
	runner := &recordingRunner{failFirstWith: "secret noops_prod_DATABASE_URL_v1 already exists"}
	service := &Service{
		config: config.Config{StateDir: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		runner: runner,
		store:  newFilesystemStore(),
		now:    time.Now,
	}

	result, err := service.Set(context.Background(), "prod", "DATABASE_URL", bytes.NewBufferString("rotated"))
	if err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if result.Version != 2 || result.SwarmName != "noops_prod_DATABASE_URL_v2" {
		t.Errorf("result = %#v", result)
	}
	if runner.runs != 2 {
		t.Errorf("docker runs = %d, want 2", runner.runs)
	}
}

func TestSetRejectsUnsafeIdentifiers(t *testing.T) {
	service := NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), config.Config{StateDir: t.TempDir()})
	if _, err := service.Set(context.Background(), "../prod", "DATABASE_URL", bytes.NewBufferString("value")); err == nil {
		t.Fatal("Set returned nil error")
	}
}

func TestSetRejectsEmptyOrWhitespaceValue(t *testing.T) {
	for name, value := range map[string]string{
		"empty":      "",
		"newline":    "\n",
		"whitespace": " \t\n",
	} {
		t.Run(name, func(t *testing.T) {
			runner := &recordingRunner{}
			service := &Service{
				config: config.Config{StateDir: t.TempDir()},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				runner: runner,
				store:  newFilesystemStore(),
				now:    time.Now,
			}

			if _, err := service.Set(context.Background(), "prod", "DATABASE_URL", bytes.NewBufferString(value)); err == nil {
				t.Fatal("Set returned nil error")
			}
			if len(runner.args) != 0 {
				t.Errorf("docker should not be called, args = %v", runner.args)
			}
		})
	}
}

func TestDeleteRemovesEveryVersionAndMetadata(t *testing.T) {
	runner := &recordingRunner{}
	service := &Service{
		config: config.Config{StateDir: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		runner: runner,
		store:  newFilesystemStore(),
		now:    time.Now,
	}

	for version := 1; version <= 2; version++ {
		metadata := Metadata{Environment: "prod", Key: "DATABASE_URL", Version: version, SwarmName: swarmName("prod", "DATABASE_URL", version)}
		if _, err := service.store.Save(service.config.StateDir, metadata); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := service.Delete(context.Background(), "prod", "DATABASE_URL")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if len(deleted) != 2 || runner.runs != 2 {
		t.Fatalf("deleted = %#v, docker calls = %d", deleted, runner.runs)
	}
	if got, want := runner.history[0], []string{"secret", "rm", "noops_prod_DATABASE_URL_v2"}; !equalStrings(got, want) {
		t.Errorf("first docker args = %v, want %v", got, want)
	}
	items, err := service.List(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("List after Delete = %#v, want no metadata", items)
	}
}

func TestDeleteKeepsMetadataWhenSwarmRefusesRemoval(t *testing.T) {
	runner := &recordingRunner{failFirstWith: "secret is in use"}
	service := &Service{
		config: config.Config{StateDir: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		runner: runner,
		store:  newFilesystemStore(),
		now:    time.Now,
	}
	metadata := Metadata{Environment: "prod", Key: "DATABASE_URL", Version: 1, SwarmName: "noops_prod_DATABASE_URL_v1"}
	if _, err := service.store.Save(service.config.StateDir, metadata); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Delete(context.Background(), "prod", "DATABASE_URL"); err == nil {
		t.Fatal("Delete returned nil error")
	}
	items, err := service.List(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SwarmName != metadata.SwarmName {
		t.Errorf("List after failed Delete = %#v, want retained metadata", items)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
