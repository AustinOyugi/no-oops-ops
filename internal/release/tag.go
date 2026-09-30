package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AustinOyugi/no-oops-ops/internal/manifest"
)

// deterministicTag keeps release names short while identifying both the
// immutable source (when Git is used) and No Oops' normalized build inputs.
func deterministicTag(m manifest.Manifest, environment, contextDir, dockerfile string, git *GitMetadata) (string, error) {
	input, err := normalizedBuildHash(m, environment, dockerfile)
	if err != nil {
		return "", err
	}

	if git != nil {
		if len(git.Commit) < 12 {
			return "", fmt.Errorf("git commit %q is too short for release tag", git.Commit)
		}

		return fmt.Sprintf("sha-%s-%s", git.Commit[:12], input[:10]), nil
	}

	context, err := directoryHash(contextDir)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("ctx-%s-%s", context[:12], input[:10]), nil
}

func normalizedBuildHash(m manifest.Manifest, environment, dockerfile string) (string, error) {
	manifestBytes, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("encode build inputs: %w", err)
	}
	var dockerfileBytes []byte
	if dockerfile != "" {
		dockerfileBytes, err = os.ReadFile(dockerfile)
		if err != nil {
			return "", fmt.Errorf("read Dockerfile for release tag: %w", err)
		}
	}
	h := sha256.New()
	_, _ = h.Write([]byte(environment + "\x00"))
	_, _ = h.Write(manifestBytes)
	_, _ = h.Write([]byte("\x00"))
	_, _ = h.Write(dockerfileBytes)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func directoryHash(root string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("build context contains unsupported file %q", rel)
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk build context: %w", err)
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		_, _ = io.WriteString(h, rel+"\x00")
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("read build context file %q: %w", rel, err)
		}
		_, copyErr := io.Copy(h, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("hash build context file %q: %w", rel, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close build context file %q: %w", rel, closeErr)
		}
		_, _ = io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sourceTagForReference(reference string, m manifest.Manifest, environment string) (string, error) {
	h, err := normalizedBuildHash(m, environment, "")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(reference)))
	return fmt.Sprintf("src-%x-%s", sum[:6], h[:10]), nil
}
