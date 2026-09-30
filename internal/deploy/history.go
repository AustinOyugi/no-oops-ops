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
	"github.com/AustinOyugi/no-oops-ops/internal/state"
)

type Deployment struct {
	App            string          `json:"app"`
	CreatedAt      time.Time       `json:"created_at"`
	Environment    string          `json:"environment"`
	Outcome        SwarmOutcome    `json:"outcome,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	ReleaseImage   string          `json:"release_image"`
	ReleaseTag     string          `json:"release_tag"`
	StackName      string          `json:"stack_name,omitempty"`
	ServiceName    string          `json:"service_name,omitempty"`
	SecretBindings []SecretBinding `json:"secret_bindings,omitempty"`
}

type deploymentStore interface {
	Latest(cfg config.Config, appName string, environment string) (Deployment, error)
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
	// O_EXCL gives callers a clear retryable error rather than silently
	// replacing a history record created by another operation.
	for attempt := 0; attempt < 100; attempt++ {
		candidate := path
		if attempt > 0 {
			candidate = filepath.Join(dir, fmt.Sprintf("%s-%02d.json", deploymentID(deployment.CreatedAt), attempt))
		}
		if _, err := os.Stat(candidate); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect deployment metadata %q: %w", candidate, err)
		}
		if err := state.WriteFile(candidate, data, 0o600); err != nil {
			return "", fmt.Errorf("write deployment metadata %q: %w", candidate, err)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("allocate unique deployment metadata filename after 100 attempts")
}

func (filesystemDeploymentStore) Previous(cfg config.Config, appName string, environment string) (Deployment, error) {
	deployments, err := successfulDeployments(cfg, appName, environment)
	if err != nil {
		return Deployment{}, err
	}
	if len(deployments) < 2 {
		return Deployment{}, fmt.Errorf("rollback requires at least two successful deployments for %q in %q", appName, environment)
	}
	return deployments[1], nil
}

func (filesystemDeploymentStore) Latest(cfg config.Config, appName string, environment string) (Deployment, error) {
	deployments, err := successfulDeployments(cfg, appName, environment)
	if err != nil {
		return Deployment{}, err
	}
	if len(deployments) == 0 {
		return Deployment{}, nil
	}
	return deployments[0], nil
}

func successfulDeployments(cfg config.Config, appName string, environment string) ([]Deployment, error) {
	dir := deploymentHistoryDir(cfg, appName, environment)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read deployment history dir %q: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}

	deployments := make([]Deployment, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("read deployment metadata %q: %w", name, err)
		}
		var deployment Deployment
		if err := json.Unmarshal(data, &deployment); err != nil {
			return nil, fmt.Errorf("decode deployment metadata %q: %w", name, err)
		}
		if deployment.Outcome == "" || deployment.Outcome == SwarmOutcomeCompleted {
			deployments = append(deployments, deployment)
		}
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].CreatedAt.After(deployments[j].CreatedAt)
	})
	return deployments, nil
}

func deploymentHistoryDir(cfg config.Config, appName string, environment string) string {
	return filepath.Join(cfg.StateDir, "apps", appName, environment, "deployments")
}

func deploymentID(createdAt time.Time) string {
	return createdAt.UTC().Format("20060102-150405.000000000")
}
