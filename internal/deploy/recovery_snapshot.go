package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/state"
	"gopkg.in/yaml.v3"
)

// snapshotRecovery keeps a successful deployment's generated environment and
// stack separate from files that a later, failed deploy can overwrite.
func (s *Service) snapshotRecovery(d Deployment, source string) (string, error) {
	relative := filepath.Join("recovery", deploymentID(d.CreatedAt), "stack.yml")
	target := filepath.Join(appDir(s.config, d.App, d.Environment), relative)
	content, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	env, err := os.ReadFile(envPath(s.config, d.App, d.Environment))
	if err != nil {
		return "", err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return "", err
	}
	selected := mappingValue(documentRoot(&doc), "services")
	if selected == nil || len(selected.Content) != 2 {
		return "", fmt.Errorf("cannot snapshot ambiguous recovery stack")
	}
	files := mappingValue(selected.Content[1], "env_file")
	generated := envPath(s.config, d.App, d.Environment)
	var rewrite func(*yaml.Node)
	rewrite = func(node *yaml.Node) {
		if node == nil {
			return
		}
		if node.Kind == yaml.ScalarNode && (node.Value == generated || node.Value == ".env") {
			node.Value = filepath.Join(filepath.Dir(target), ".env")
			return
		}
		for _, child := range node.Content {
			rewrite(child)
		}
	}
	rewrite(files)
	content, err = yaml.Marshal(&doc)
	if err != nil {
		return "", err
	}
	if err := state.WriteFile(filepath.Join(filepath.Dir(target), ".env"), env, 0600); err != nil {
		return "", err
	}
	if err := state.WriteFile(target, content, 0600); err != nil {
		return "", err
	}
	return relative, nil
}
func recoverySnapshotPath(root, relative string) (string, error) {
	clean := filepath.Clean(relative)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid recovery snapshot path %q", relative)
	}
	return filepath.Join(root, clean), nil
}
