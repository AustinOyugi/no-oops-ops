package local

import "testing"

func TestIsRetryableDockerDesktopBindMountError(t *testing.T) {
	transient := `invalid mount config for type "bind": bind source path does not exist: /host_mnt/Users/me/work/.noops/config.yml`
	if !isRetryableDockerDesktopBindMountError(transient) {
		t.Fatal("expected Docker Desktop bind-mount propagation error to be retryable")
	}

	for _, taskError := range []string{
		`invalid mount config for type "bind": bind source path does not exist: /srv/noops/config.yml`,
		`failed to pull image: registry is unavailable`,
		`invalid mount config for type "volume": volume not found`,
	} {
		if isRetryableDockerDesktopBindMountError(taskError) {
			t.Errorf("unexpected retryable error: %q", taskError)
		}
	}
}
