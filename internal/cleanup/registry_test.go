package cleanup

import "testing"

func TestRegistryResponseWithoutDigestIsTreatedAsAlreadyDeleted(t *testing.T) {
	response := "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"
	if digest := registryDigest(response); digest != "" {
		t.Fatalf("registryDigest = %q, want empty", digest)
	}
}

func TestRegistryManifestDeleted(t *testing.T) {
	if !registryManifestDeleted("HTTP/1.1 202 Accepted\r\n\r\n") {
		t.Fatal("expected accepted registry deletion to require garbage collection")
	}
	if registryManifestDeleted("HTTP/1.1 404 Not Found\r\n\r\n") {
		t.Fatal("an already-missing manifest must not require garbage collection")
	}
}
