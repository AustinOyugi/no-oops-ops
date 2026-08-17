package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
)

type Deployment struct {
	App            string          `json:"app"`
	CreatedAt      time.Time       `json:"created_at"`
	Environment    string          `json:"environment"`
	ReleaseImage   string          `json:"release_image"`
	ReleaseTag     string          `json:"release_tag"`
	SecretBindings []SecretBinding `json:"secret_bindings,omitempty"`
}

type deploymentStore interface {
	Previous(cfg config.Config, appName string, environment string) (Deployment, error)
	Save(cfg config.Config, deployment Deployment) (string, error)
}

type filesystemDeploymentStore struct{}

func newFilesystemDeploymentStore() deploymentStore {
	return filesystemDeploymentStore{}
}

func (filesystemDeploymentStore) Save(cfg config.Config, deployment Deployment) (string, error) {
	dir := deploymentHistoryDir(cfg, deployment.App, deployment.Environment)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create deployment history dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(deployment, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal deployment metadata: %w", err)
	}

	data = append(data, '\n')
	path := filepath.Join(dir, deploymentID(deployment.CreatedAt)+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write deployment metadata %q: %w", path, err)
	}

	return path, nil
}

func (filesystemDeploymentStore) Previous(cfg config.Config, appName string, environment string) (Deployment, error) {
	dir := deploymentHistoryDir(cfg, appName, environment)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Deployment{}, fmt.Errorf("rollback requires at least two successful deployments for %q in %q", appName, environment)
		}
		return Deployment{}, fmt.Errorf("read deployment history dir %q: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}

	if len(names) < 2 {
		return Deployment{}, fmt.Errorf("rollback requires at least two successful deployments for %q in %q", appName, environment)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	path := filepath.Join(dir, names[1])
	data, err := os.ReadFile(path)
	if err != nil {
		return Deployment{}, fmt.Errorf("read deployment metadata %q: %w", path, err)
	}

	var deployment Deployment
	if err := json.Unmarshal(data, &deployment); err != nil {
		return Deployment{}, fmt.Errorf("decode deployment metadata %q: %w", path, err)
	}

	return deployment, nil
}

func deploymentHistoryDir(cfg config.Config, appName string, environment string) string {
	return filepath.Join(cfg.StateDir, "apps", appName, environment, "deployments")
}

func deploymentID(createdAt time.Time) string {
	return createdAt.UTC().Format("20060102-150405")
}
