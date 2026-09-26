package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLatestVersionUsesConfiguredRepository(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/example/fork/releases/latest" {
			t.Errorf("path = %q", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer server.Close()
	client := NewClient()
	client.APIBaseURL = server.URL
	client.HTTPClient = server.Client()

	got, err := client.LatestVersion(context.Background(), "example/fork")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v1.2.3" {
		t.Errorf("latest version = %q, want v1.2.3", got)
	}
}

func TestDownloadVerifiesAndValidatesReleaseBinary(t *testing.T) {
	archive, err := archiveName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	archiveBytes := releaseArchive(t, "#!/bin/sh\necho 'noops 1.2.3'\n")
	digest := sha256.Sum256(archiveBytes)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch filepath.Base(request.URL.Path) {
		case archive:
			_, _ = response.Write(archiveBytes)
		case "checksums.txt":
			_, _ = fmt.Fprintf(response, "%x  %s\n", digest, archive)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	client := NewClient()
	client.DownloadBaseURL = server.URL
	client.HTTPClient = server.Client()

	binary, err := client.Download(context.Background(), "example/fork", "v1.2.3", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(binary); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("downloaded binary mode: info=%v err=%v", info, err)
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	archive, err := archiveName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skip(err)
	}
	archiveBytes := releaseArchive(t, "#!/bin/sh\necho 'noops 1.2.3'\n")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if filepath.Base(request.URL.Path) == archive {
			_, _ = response.Write(archiveBytes)
			return
		}
		_, _ = fmt.Fprintf(response, "%064d  %s\n", 0, archive)
	}))
	defer server.Close()
	client := NewClient()
	client.DownloadBaseURL = server.URL
	client.HTTPClient = server.Client()

	_, err = client.Download(context.Background(), "example/fork", "v1.2.3", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("error = %v, want checksum mismatch", err)
	}
}

func TestCompareVersions(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{left: "v1.2.3", right: "v1.2.4", want: -1},
		{left: "1.2.3", right: "v1.2.4", want: -1},
		{left: "v1.10.0", right: "v1.9.9", want: 1},
		{left: "v2.0.0", right: "v2.0.0", want: 0},
	} {
		got, err := CompareVersions(test.left, test.right)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestReplaceFileAtomicallyInstallsCandidate(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "candidate")
	destination := filepath.Join(directory, "noops")
	if err := os.WriteFile(candidate, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(candidate, destination); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("installed contents = %q, want new", got)
	}
}

func releaseArchive(t *testing.T, binary string) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "noops", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write([]byte(binary)); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
