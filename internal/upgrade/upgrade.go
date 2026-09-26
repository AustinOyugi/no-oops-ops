// Package upgrade downloads, verifies, and installs No Oops release binaries.
package upgrade

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultRepository  = "AustinOyugi/no-oops-ops"
	defaultAPIBaseURL  = "https://api.github.com"
	defaultDownloadURL = "https://github.com"
	maxDownloadSize    = 256 << 20
)

var (
	repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	versionPattern    = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)
)

type Client struct {
	HTTPClient      *http.Client
	APIBaseURL      string
	DownloadBaseURL string
	Token           string
}

func NewClient() *Client {
	return &Client{
		HTTPClient:      http.DefaultClient,
		APIBaseURL:      defaultAPIBaseURL,
		DownloadBaseURL: defaultDownloadURL,
		Token:           os.Getenv("GITHUB_TOKEN"),
	}
}

func ValidateRepository(repository string) error {
	if !repositoryPattern.MatchString(repository) {
		return fmt.Errorf("upgrade repository %q must use owner/name format", repository)
	}
	return nil
}

func ValidateVersion(version string) error {
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("version %q must use MAJOR.MINOR.PATCH format with an optional v prefix", version)
	}
	return nil
}

func ValidateReleaseTag(version string) error {
	if err := ValidateVersion(version); err != nil || !strings.HasPrefix(version, "v") {
		return fmt.Errorf("upgrade release tag %q must use vMAJOR.MINOR.PATCH format", version)
	}
	return nil
}

func CatalogVersion(releaseTag string) string {
	return strings.TrimPrefix(releaseTag, "v")
}

// CompareVersions compares two validated vMAJOR.MINOR.PATCH versions.
func CompareVersions(left, right string) (int, error) {
	if err := ValidateVersion(left); err != nil {
		return 0, err
	}
	if err := ValidateVersion(right); err != nil {
		return 0, err
	}
	leftParts := strings.Split(strings.TrimPrefix(left, "v"), ".")
	rightParts := strings.Split(strings.TrimPrefix(right, "v"), ".")
	for index := range leftParts {
		leftValue, err := strconv.ParseUint(leftParts[index], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse version %q: %w", left, err)
		}
		rightValue, err := strconv.ParseUint(rightParts[index], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse version %q: %w", right, err)
		}
		if leftValue < rightValue {
			return -1, nil
		}
		if leftValue > rightValue {
			return 1, nil
		}
	}
	return 0, nil
}

func (c *Client) LatestVersion(ctx context.Context, repository string) (string, error) {
	if err := ValidateRepository(repository); err != nil {
		return "", err
	}
	url := strings.TrimRight(c.APIBaseURL, "/") + "/repos/" + repository + "/releases/latest"
	response, err := c.get(ctx, url)
	if err != nil {
		return "", fmt.Errorf("fetch latest release for %q: %w", repository, err)
	}
	defer response.Body.Close()
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&release); err != nil {
		return "", fmt.Errorf("decode latest release for %q: %w", repository, err)
	}
	if err := ValidateReleaseTag(release.TagName); err != nil {
		return "", fmt.Errorf("latest release for %q: %w", repository, err)
	}
	return release.TagName, nil
}

func (c *Client) Download(ctx context.Context, repository, version, destination string) (string, error) {
	if err := ValidateRepository(repository); err != nil {
		return "", err
	}
	if err := ValidateReleaseTag(version); err != nil {
		return "", err
	}
	archive, err := archiveName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	base := strings.TrimRight(c.DownloadBaseURL, "/") + "/" + repository + "/releases/download/" + version
	archivePath := filepath.Join(destination, archive)
	checksumPath := filepath.Join(destination, "checksums.txt")
	if err := c.downloadFile(ctx, base+"/"+archive, archivePath); err != nil {
		return "", err
	}
	if err := c.downloadFile(ctx, base+"/checksums.txt", checksumPath); err != nil {
		return "", err
	}
	if err := verifyChecksum(archivePath, checksumPath); err != nil {
		return "", err
	}
	binaryPath := filepath.Join(destination, "noops")
	if err := extractBinary(archivePath, binaryPath); err != nil {
		return "", err
	}
	if err := validateBinary(ctx, binaryPath, version); err != nil {
		return "", err
	}
	return binaryPath, nil
}

func (c *Client) downloadFile(ctx context.Context, url, destination string) error {
	response, err := c.get(ctx, url)
	if err != nil {
		return fmt.Errorf("download %q: %w", url, err)
	}
	defer response.Body.Close()
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create download %q: %w", destination, err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxDownloadSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write download %q: %w", destination, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close download %q: %w", destination, closeErr)
	}
	if written > maxDownloadSize {
		return fmt.Errorf("download %q exceeds %d bytes", url, maxDownloadSize)
	}
	return nil
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "noops-upgrade")
	if c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("GitHub returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	return response, nil
}

func archiveName(goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "linux":
		osName = "Linux"
	case "darwin":
		osName = "Darwin"
	default:
		return "", fmt.Errorf("upgrades are not supported on %s", goos)
	}
	var archName string
	switch goarch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	default:
		return "", fmt.Errorf("upgrades are not supported on %s/%s", goos, goarch)
	}
	return fmt.Sprintf("noops_%s_%s.tar.gz", osName, archName), nil
}

func verifyChecksum(archivePath, checksumPath string) error {
	checksums, err := os.ReadFile(checksumPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	archiveName := filepath.Base(archivePath)
	want := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == archiveName {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt does not contain %q", archiveName)
	}
	if _, err := hex.DecodeString(want); err != nil || len(want) != sha256.Size*2 {
		return fmt.Errorf("invalid SHA-256 checksum for %q", archiveName)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive for verification: %w", err)
	}
	defer archive.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %q", archiveName)
	}
	return nil
}

func extractBinary(archivePath, destination string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open release gzip: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return errors.New("release archive does not contain noops")
		}
		if err != nil {
			return fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "noops" {
			continue
		}
		if header.Size < 0 || header.Size > maxDownloadSize {
			return fmt.Errorf("release binary has invalid size %d", header.Size)
		}
		binary, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			return fmt.Errorf("create release binary: %w", err)
		}
		_, copyErr := io.CopyN(binary, tarReader, header.Size)
		closeErr := binary.Close()
		if copyErr != nil {
			return fmt.Errorf("extract release binary: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close release binary: %w", closeErr)
		}
		return nil
	}
}

func validateBinary(ctx context.Context, path, version string) error {
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate downloaded noops binary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	got := strings.TrimSpace(string(output))
	gotVersion := strings.TrimPrefix(got, "noops ")
	if got == gotVersion || CatalogVersion(gotVersion) != CatalogVersion(version) {
		return fmt.Errorf("downloaded binary reported %q, want release %q", got, version)
	}
	return nil
}

func ReplaceExecutable(candidate string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve current executable %q: %w", executable, err)
	}
	return replaceFile(candidate, executable)
}

func replaceFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open candidate binary: %w", err)
	}
	defer input.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".noops-upgrade-*")
	if err != nil {
		return fmt.Errorf("prepare executable beside %q: %w; rerun with permission to write that directory", destination, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o755); err != nil {
		temporary.Close()
		return fmt.Errorf("make upgraded executable runnable: %w", err)
	}
	if _, err := io.Copy(temporary, input); err != nil {
		temporary.Close()
		return fmt.Errorf("copy upgraded executable: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync upgraded executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close upgraded executable: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("replace executable %q: %w", destination, err)
	}
	return nil
}
