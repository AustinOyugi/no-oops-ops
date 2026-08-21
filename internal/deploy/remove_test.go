package deploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistryClientDeletesManifestByDigest(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Docker-Content-Digest", "sha256:abc")
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	client := &registryClient{baseURL: server.URL, http: server.Client()}
	if err := client.DeleteImage(context.Background(), strings.TrimPrefix(server.URL, "http://")+"/quay.io/example/app:release"); err != nil {
		t.Fatalf("delete image: %v", err)
	}

	want := []string{
		"HEAD /v2/quay.io/example/app/manifests/release",
		"DELETE /v2/quay.io/example/app/manifests/sha256:abc",
	}
	if len(requests) != len(want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i, requests[i], want[i])
		}
	}
}

func TestRegistryReferenceRejectsExternalImage(t *testing.T) {
	if _, _, err := registryReference("http://127.0.0.1:5000", "docker.io/example/app:latest"); err == nil {
		t.Fatal("expected external registry reference to be rejected")
	}
}

func TestAppStatePathStaysUnderManagedApps(t *testing.T) {
	path, err := appStatePath("/tmp/noops", "app", "dev")
	if err != nil {
		t.Fatalf("app state path: %v", err)
	}
	if path != "/tmp/noops/apps/app/dev" {
		t.Errorf("path = %q", path)
	}
}
