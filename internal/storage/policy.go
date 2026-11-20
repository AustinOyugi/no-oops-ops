// Package storage applies local Docker-host storage safeguards. Cache pruning
// is a retention target, not a filesystem quota.
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/platform/command"
	"github.com/shirou/gopsutil/v4/disk"
)

type Policy struct {
	MinFreeBytes    uint64 `yaml:"min_free_bytes"`
	WarnFreeBytes   uint64 `yaml:"warn_free_bytes"`
	BuildCacheBytes uint64 `yaml:"build_cache_bytes"`
	LogMaxSize      string `yaml:"log_max_size"`
	LogMaxFiles     int    `yaml:"log_max_files"`
}

func (p Policy) Defaults() Policy {
	if p.MinFreeBytes == 0 {
		p.MinFreeBytes = 5 << 30
	}
	if p.WarnFreeBytes == 0 {
		p.WarnFreeBytes = 10 << 30
	}
	if p.BuildCacheBytes == 0 {
		p.BuildCacheBytes = 10 << 30
	}
	if p.LogMaxSize == "" {
		p.LogMaxSize = "10m"
	}
	if p.LogMaxFiles == 0 {
		p.LogMaxFiles = 3
	}
	return p
}
func (p Policy) Validate() error {
	if p.WarnFreeBytes < p.MinFreeBytes {
		return fmt.Errorf("storage.warn_free_bytes must be at least min_free_bytes")
	}
	if p.LogMaxFiles < 1 {
		return fmt.Errorf("storage.log_max_files must be positive")
	}
	size := strings.ToLower(p.LogMaxSize)
	if len(size) > 0 && strings.ContainsRune("kmg", rune(size[len(size)-1])) {
		size = size[:len(size)-1]
	}
	if n, err := strconv.ParseUint(size, 10, 64); err != nil || n == 0 {
		return fmt.Errorf("storage.log_max_size must be a positive Docker size, such as 10m")
	}
	return nil
}

type Runner interface {
	Run(context.Context, string, []string, command.RunOptions) (command.Result, error)
}
type Usage struct {
	Path string
	Free uint64
}

// Inspect fails closed when the daemon's data directory is not locally
// accessible. Noops' deployment target is a local Linux Docker host.
func Inspect(ctx context.Context, r Runner) (Usage, error) {
	endpoint := os.Getenv("DOCKER_HOST")
	if os.Getenv("DOCKER_CONTEXT") != "" {
		endpoint = ""
	}
	if endpoint == "" {
		result, err := r.Run(ctx, "docker", []string{"context", "inspect", "--format", "{{.Endpoints.docker.Host}}"}, command.RunOptions{})
		if err != nil {
			return Usage{}, fmt.Errorf("locate local Docker endpoint: %w", err)
		}
		endpoint = strings.TrimSpace(string(result.Output))
	}
	if !strings.HasPrefix(endpoint, "unix://") {
		return Usage{}, fmt.Errorf("storage safeguards require a local Unix Docker endpoint, got %q", endpoint)
	}
	result, err := r.Run(ctx, "docker", []string{"info", "--format", "{{.DockerRootDir}}"}, command.RunOptions{})
	if err != nil {
		return Usage{}, fmt.Errorf("locate Docker storage: %w", err)
	}
	path := strings.TrimSpace(string(result.Output))
	if !filepath.IsAbs(path) {
		return Usage{}, fmt.Errorf("Docker returned an invalid storage directory %q", path)
	}
	usage, err := disk.UsageWithContext(ctx, path)
	if err != nil {
		return Usage{}, fmt.Errorf("inspect Docker storage %q on the local host: %w", path, err)
	}
	return Usage{Path: path, Free: usage.Free}, nil
}
func Check(p Policy, usage Usage, logger *slog.Logger) error {
	if usage.Free < p.MinFreeBytes {
		return fmt.Errorf("Docker storage %s has %d free bytes, below the build minimum %d; run noops cleanup --host --apply or expand the filesystem", usage.Path, usage.Free, p.MinFreeBytes)
	}
	if usage.Free < p.WarnFreeBytes {
		logger.Warn("Docker storage is low", "path", usage.Path, "free_bytes", usage.Free, "warn_free_bytes", p.WarnFreeBytes)
	}
	return nil
}
func PruneCache(ctx context.Context, r Runner, p Policy) error {
	if p.BuildCacheBytes == 0 {
		return nil
	}
	out, err := r.Run(ctx, "docker", []string{"builder", "prune", "--force", "--all", "--keep-storage", strconv.FormatUint(p.BuildCacheBytes, 10)}, command.RunOptions{})
	if err != nil {
		return fmt.Errorf("prune unused Docker build cache: %w: %s", err, strings.TrimSpace(string(out.Output)))
	}
	return nil
}
func (p Policy) LogArgs() []string {
	if p.LogMaxSize == "" {
		return nil
	}
	return []string{"--log-driver", "json-file", "--log-opt", "max-size=" + p.LogMaxSize, "--log-opt", "max-file=" + strconv.Itoa(p.LogMaxFiles)}
}
