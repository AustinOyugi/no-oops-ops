package deploy

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

const swarmProgressRefreshInterval = 120 * time.Millisecond

// swarmProgressIndicator keeps an interactive deployment status on one line.
// It deliberately disables itself for redirected output so log collectors and
// CI retain newline-delimited records.
type swarmProgressIndicator struct {
	output *os.File
	active bool

	mu            sync.Mutex
	updateState   string
	runningTasks  int
	desiredTasks  int
	monitoring    bool
	startedAt     time.Time
	monitoringAt  time.Time
	monitorWindow time.Duration
	done          chan struct{}
	finished      chan struct{}
}

func newSwarmProgressIndicator(output *os.File) *swarmProgressIndicator {
	return &swarmProgressIndicator{
		output: output,
		active: output != nil && term.IsTerminal(int(output.Fd())),
	}
}

func (p *swarmProgressIndicator) Start(service string, desiredTasks int, monitorWindow time.Duration) {
	if !p.active {
		return
	}

	p.mu.Lock()
	p.updateState = fmt.Sprintf("preparing rollout for %s", service)
	p.runningTasks = 0
	p.desiredTasks = desiredTasks
	p.monitoring = false
	p.startedAt = time.Now()
	p.monitoringAt = time.Time{}
	p.monitorWindow = monitorWindow
	p.done = make(chan struct{})
	p.finished = make(chan struct{})
	done := p.done
	finished := p.finished
	p.mu.Unlock()

	go func() {
		defer close(finished)
		ticker := time.NewTicker(swarmProgressRefreshInterval)
		defer ticker.Stop()

		for frame := 0; ; frame++ {
			p.render(frame)
			select {
			case <-done:
				return
			case <-ticker.C:
			}
		}
	}()
}

func (p *swarmProgressIndicator) Update(updateState string, runningTasks int, desiredTasks int, monitoring bool) {
	if !p.active {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done == nil {
		return
	}
	p.updateState = updateState
	p.runningTasks = runningTasks
	p.desiredTasks = desiredTasks
	if monitoring && !p.monitoring {
		p.monitoringAt = time.Now()
	}
	if !monitoring {
		p.monitoringAt = time.Time{}
	}
	p.monitoring = monitoring
}

func (p *swarmProgressIndicator) Stop() {
	if !p.active {
		return
	}

	p.mu.Lock()
	if p.done == nil {
		p.mu.Unlock()
		return
	}
	done := p.done
	finished := p.finished
	p.done = nil
	p.mu.Unlock()

	close(done)
	<-finished
	_, _ = fmt.Fprint(p.output, "\r\033[2K")
}

func (p *swarmProgressIndicator) render(frame int) {
	frames := [...]string{"|", "/", "-", "\\"}
	p.mu.Lock()
	status := formatSwarmProgress(
		p.updateState,
		p.runningTasks,
		p.desiredTasks,
		p.monitoring,
		time.Since(p.startedAt),
		time.Since(p.monitoringAt),
		p.monitorWindow,
	)
	p.mu.Unlock()
	_, _ = fmt.Fprintf(p.output, "\r\033[2K%s %s", frames[frame%len(frames)], status)
}

func formatSwarmProgress(updateState string, runningTasks int, desiredTasks int, monitoring bool, elapsed, monitoringElapsed, monitorWindow time.Duration) string {
	if monitoring {
		return fmt.Sprintf("Validating rollout (%d/%d tasks, %s/%s)", runningTasks, desiredTasks, formatProgressDuration(monitoringElapsed), formatProgressDuration(monitorWindow))
	}
	if updateState != "" {
		return fmt.Sprintf("Rolling out: %s (%d/%d tasks, %s elapsed)", updateState, runningTasks, desiredTasks, formatProgressDuration(elapsed))
	}
	return fmt.Sprintf("Starting tasks (%d/%d, %s elapsed)", runningTasks, desiredTasks, formatProgressDuration(elapsed))
}

func formatProgressDuration(value time.Duration) string {
	return value.Truncate(time.Second).String()
}
