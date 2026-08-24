package uninstall

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestRunPreservesDataByDefault(t *testing.T) {
	host := &fakeHost{}
	service, err := New(host)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}

	want := []string{"load", "apps", "registry", "nginx", "network", "state", "metadata"}
	if !reflect.DeepEqual(host.calls, want) {
		t.Fatalf("calls = %v, want %v", host.calls, want)
	}
}

func TestRunPurgesDataBeforeRemovingMetadata(t *testing.T) {
	host := &fakeHost{}
	service, err := New(host)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Run(context.Background(), Options{Purge: true}); err != nil {
		t.Fatal(err)
	}

	want := []string{"load", "apps", "registry", "nginx", "network", "state", "data", "metadata"}
	if !reflect.DeepEqual(host.calls, want) {
		t.Fatalf("calls = %v, want %v", host.calls, want)
	}
}

func TestRunIsIdempotentWhenMetadataIsMissing(t *testing.T) {
	host := &fakeHost{loadErr: os.ErrNotExist}
	service, err := New(host)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Run(context.Background(), Options{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(host.calls, []string{"load"}) {
		t.Fatalf("calls = %v", host.calls)
	}
}

func TestRunKeepsMetadataWhenAnEarlierStepFails(t *testing.T) {
	host := &fakeHost{registryErr: errors.New("registry unavailable")}
	service, err := New(host)
	if err != nil {
		t.Fatal(err)
	}

	if err := service.Run(context.Background(), Options{}); err == nil {
		t.Fatal("Run() error = nil")
	}
	want := []string{"load", "apps", "registry"}
	if !reflect.DeepEqual(host.calls, want) {
		t.Fatalf("calls = %v, want %v", host.calls, want)
	}
}

type fakeHost struct {
	calls       []string
	loadErr     error
	registryErr error
}

func (h *fakeHost) LoadInstallation(context.Context) (Metadata, error) {
	h.calls = append(h.calls, "load")
	return Metadata{}, h.loadErr
}
func (h *fakeHost) RemoveApps(context.Context, Metadata) error {
	h.calls = append(h.calls, "apps")
	return nil
}
func (h *fakeHost) RemoveRegistry(context.Context, Metadata) error {
	h.calls = append(h.calls, "registry")
	return h.registryErr
}
func (h *fakeHost) RemoveNginx(context.Context, Metadata) error {
	h.calls = append(h.calls, "nginx")
	return nil
}
func (h *fakeHost) RemoveNetwork(context.Context, Metadata) error {
	h.calls = append(h.calls, "network")
	return nil
}
func (h *fakeHost) RemoveGeneratedState(context.Context, Metadata) error {
	h.calls = append(h.calls, "state")
	return nil
}
func (h *fakeHost) RemoveData(context.Context, Metadata) error {
	h.calls = append(h.calls, "data")
	return nil
}
func (h *fakeHost) RemoveInstallMetadata(context.Context, Metadata) error {
	h.calls = append(h.calls, "metadata")
	return nil
}
