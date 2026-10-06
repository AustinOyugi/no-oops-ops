package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/AustinOyugi/no-oops-ops/internal/config"
	"github.com/AustinOyugi/no-oops-ops/internal/deploy"
	"github.com/rivo/tview"
)

func Rollouts(ctx context.Context, cfg config.Config) ([]deploy.RolloutProgress, error) {
	paths, err := filepath.Glob(filepath.Join(cfg.StateDir, "apps", "*", "*", "rollout.json"))
	if err != nil {
		return nil, err
	}
	var results []deploy.RolloutProgress
	for _, path := range paths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var p deploy.RolloutProgress
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("read rollout %s: %w", path, err)
		}
		if !p.Finished {
			if lock, err := os.Open(filepath.Join(filepath.Dir(path), "operation.lock")); err == nil {
				if syscall.Flock(int(lock.Fd()), syscall.LOCK_SH|syscall.LOCK_NB) == nil {
					syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
					p.FailedStage = p.Stage
					p.Stage = "interrupted"
					p.Error = "Operation lock released; inspect ingress and candidate before retrying."
					p.Finished = true
				}
				lock.Close()
			}
		}
		if p.Finished && p.Stage != "interrupted" && time.Since(p.UpdatedAt) > 90*time.Second {
			continue
		}
		results = append(results, p)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Finished != results[j].Finished {
			return !results[i].Finished
		}
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	return results, nil
}
func rolloutText(p deploy.RolloutProgress, rows []Row) string {
	replicas := func(name string) string {
		for _, row := range rows {
			if row.Service == name {
				return row.Replicas + " " + row.State
			}
		}
		return "not in latest service snapshot"
	}
	traffic := "Traffic handoff unresolved"
	if p.Traffic == "old" {
		traffic = "Ingress has not been promoted"
	}
	if p.Traffic == "new" {
		traffic = "Ingress reconciled to NEW (no HTTP traffic probe)"
	}
	steps := []string{"preparing", "starting_candidate", "waiting_readiness", "ready", "promoting", "ingress_reconciled", "cleaning_old_stacks", "completed"}
	labels := []string{"Prepare", "Start", "Readiness", "Ready", "Promote", "Ingress", "Cleanup requested", "Complete"}
	index := -1
	for i, stage := range steps {
		if p.Stage == stage || p.FailedStage == stage {
			index = i
		}
	}
	var marks []string
	for i, label := range labels {
		mark := "○"
		if i < index {
			mark = "✓"
		} else if i == index {
			mark = "→"
			if p.Error != "" {
				mark = "!"
			} else if p.Finished {
				mark = "✓"
			}
		}
		marks = append(marks, mark+" "+label)
	}
	text := fmt.Sprintf("%s / %s · %s · last transition %s\nOLD %s · %s · %s\nNEW %s · %s · %s\n%s\n%s", p.Environment, p.App, p.Stage, p.UpdatedAt.Format("15:04:05"), p.OldRelease, p.OldService, replicas(p.OldService), p.NewRelease, p.NewService, replicas(p.NewService), traffic, strings.Join(marks, "  "))
	if p.Error != "" {
		text += "\nFailed: " + p.Error
	}
	return cleanLines(text)
}
func cleanLines(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = clean(lines[i])
	}
	return strings.Join(lines, "\n")
}
func (d *dashboard) updateRolloutPane() {
	if len(d.rollouts) == 0 {
		if d.rolloutVisible {
			d.rolloutVisible = false
			d.rebuildLayout()
		}
		return
	}
	if d.rolloutPane == nil {
		d.rolloutPane = tview.NewTextView().SetWrap(true)
		d.rolloutPane.SetBorder(true).SetTitle(" Blue / green rollout ")
	}
	if !d.rolloutVisible {
		d.rolloutVisible = true
		d.rebuildLayout()
	}
	selected := d.rollouts[0]
	for _, p := range d.rollouts {
		if p.OldService == d.selected || p.NewService == d.selected {
			selected = p
			break
		}
	}
	d.rolloutPane.SetTitle(" Blue / green rollout ")
	d.rolloutPane.SetText(rolloutText(selected, d.rows))
}
func (d *dashboard) monitorRollouts() {
	if d.execution == nil || d.execution.Rollouts == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			rows, err := d.execution.Rollouts(d.ctx)
			if d.ctx.Err() != nil {
				return
			}
			d.app.QueueUpdateDraw(func() {
				if err != nil {
					if d.rolloutPane != nil {
						d.rolloutPane.SetTitle(" Rollout · stale: read failed ")
					}
					return
				}
				d.rollouts = rows
				d.updateRolloutPane()
			})
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
