package tui

import (
	"strings"
	"testing"
	"time"
)

func TestMetricsSourcesAndStaleness(t *testing.T) {
	now := time.Now()
	remote := hostMetrics{Name: "laptop", DockerName: "production", DockerConnected: true, CPU: "77%", SampledAt: now}
	text := remote.text(now)
	if !strings.Contains(text, "production") || !strings.Contains(text, "unavailable") || strings.Contains(text, "77%") {
		t.Fatalf("remote displayed laptop usage: %s", text)
	}
	local := hostMetrics{Name: "production", DockerConnected: true, SameHost: true, CPU: "77%", RAM: "1/4 GiB", DiskPath: "/var/lib/docker", Disk: "80%", Load: "1.2", Uptime: "3d", SampledAt: now}
	for _, word := range []string{"production", "77%", "RAM", "/var/lib/docker", "LOAD", "3d"} {
		if !strings.Contains(local.text(now), word) {
			t.Fatalf("missing metric %s", word)
		}
	}
	if !strings.Contains(local.text(now.Add(10*time.Second)), "STALE") {
		t.Fatal("stale data not labeled")
	}
	local.DockerConnected = false
	local.SameHost = false
	if !strings.Contains(local.text(now), "LOCAL HOST") || !strings.Contains(local.text(now), "unavailable") {
		t.Fatal("disconnected Docker source not labeled")
	}
	for _, endpoint := range []string{"ssh://production", "tcp://production:2376"} {
		if sameDockerHost(endpoint, "production", "production") {
			t.Fatal("remote endpoint classified as local")
		}
	}
}
