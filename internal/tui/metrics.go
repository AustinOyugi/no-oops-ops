package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
)

type hostMetrics struct {
	Name, DockerName, DiskPath   string
	CPU, RAM, Disk, Load, Uptime string
	DockerConnected, SameHost    bool
	SampledAt                    time.Time
}

func sameDockerHost(endpoint, local, daemon string) bool {
	return runtime.GOOS == "linux" && strings.HasPrefix(endpoint, "unix://") && local != "" && daemon != "" && strings.Split(local, ".")[0] == strings.Split(daemon, ".")[0]
}
func collectMetrics(ctx context.Context) hostMetrics {
	m := hostMetrics{CPU: "—", RAM: "—", Disk: "—", Load: "—", Uptime: "—", DiskPath: "/"}
	m.Name, _ = os.Hostname()
	var info struct{ Name, DockerRootDir string }
	output, err := exec.CommandContext(ctx, "docker", "info", "--format", `{"Name":{{json .Name}},"DockerRootDir":{{json .DockerRootDir}}}`).Output()
	if err == nil && json.Unmarshal(output, &info) == nil {
		m.DockerConnected = true
		m.DockerName = info.Name
		args := []string{"context", "inspect", "--format", "{{.Endpoints.docker.Host}}"}
		endpoint := os.Getenv("DOCKER_HOST")
		if contextName := os.Getenv("DOCKER_CONTEXT"); contextName != "" {
			args = append(args, contextName)
			endpoint = ""
		}
		if endpoint == "" {
			if data, err := exec.CommandContext(ctx, "docker", args...).Output(); err == nil {
				endpoint = strings.TrimSpace(string(data))
			}
		}
		m.SameHost = sameDockerHost(endpoint, m.Name, info.Name)
		if m.SameHost {
			m.DiskPath = info.DockerRootDir
		}
	}
	if m.DockerConnected && !m.SameHost {
		m.SampledAt = time.Now()
		return m
	}
	if percentages, err := cpu.PercentWithContext(ctx, 250*time.Millisecond, false); err == nil && len(percentages) > 0 {
		m.CPU = fmt.Sprintf("%.0f%%", percentages[0])
	}
	if v, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		m.RAM = fmt.Sprintf("%.1f/%.1f GiB (%.0f%%)", float64(v.Used)/(1<<30), float64(v.Total)/(1<<30), v.UsedPercent)
	}
	if v, err := disk.UsageWithContext(ctx, m.DiskPath); err == nil {
		m.Disk = fmt.Sprintf("%.0f%% (%.1f GiB free)", v.UsedPercent, float64(v.Free)/(1<<30))
	}
	if v, err := load.AvgWithContext(ctx); err == nil {
		m.Load = fmt.Sprintf("%.2f/%.2f/%.2f (%d CPUs)", v.Load1, v.Load5, v.Load15, runtime.NumCPU())
	}
	if v, err := host.UptimeWithContext(ctx); err == nil {
		m.Uptime = serviceAge(Row{CreatedAt: time.Now().Add(-time.Duration(v) * time.Second)}, time.Now())
	}
	m.SampledAt = time.Now()
	return m
}
func (m hostMetrics) text(now time.Time) string {
	if m.SampledAt.IsZero() {
		return "HOST · Sampling metrics…"
	}
	stale := ""
	if now.Sub(m.SampledAt) > 6*time.Second {
		stale = " · STALE"
	}
	if m.DockerConnected && !m.SameHost {
		return "DOCKER HOST " + clean(m.DockerName) + " · connected" + stale + "\nCPU/RAM/DISK/LOAD/UPTIME unavailable for remote or VM daemon; run noops on the server."
	}
	source := "HOST "
	if !m.SameHost {
		source = "LOCAL HOST "
	}
	dockerStatus := "connected"
	if !m.DockerConnected {
		dockerStatus = "unavailable"
	}
	return fmt.Sprintf("%s%s · DOCKER %s · UP %s%s\nCPU %s · RAM %s · LOAD %s\nDISK %s: %s", source, clean(m.Name), dockerStatus, m.Uptime, stale, m.CPU, m.RAM, m.Load, clean(m.DiskPath), m.Disk)
}
func (d *dashboard) monitorMetrics() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			ctx, cancel := context.WithTimeout(d.ctx, 1500*time.Millisecond)
			reading := collectMetrics(ctx)
			cancel()
			if d.ctx.Err() != nil {
				return
			}
			d.app.QueueUpdateDraw(func() { d.health.SetText(reading.text(time.Now())) })
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
