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

	mu       sync.Mutex
	status   string
	done     chan struct{}
	finished chan struct{}
}

func newSwarmProgressIndicator(output *os.File) *swarmProgressIndicator {
	return &swarmProgressIndicator{
		output: output,
		active: output != nil && term.IsTerminal(int(output.Fd())),
	}
}

func (p *swarmProgressIndicator) Start(service string, desiredTasks int) {
	if !p.active {
		return
	}

	p.mu.Lock()
	p.status = fmt.Sprintf("Deploying %s: preparing rollout (0/%d tasks)", service, desiredTasks)
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
	p.status = formatSwarmProgress(updateState, runningTasks, desiredTasks, monitoring)
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
	status := p.status
	p.mu.Unlock()
	_, _ = fmt.Fprintf(p.output, "\r\033[2K%s %s", frames[frame%len(frames)], status)
}

func formatSwarmProgress(updateState string, runningTasks int, desiredTasks int, monitoring bool) string {
	if monitoring {
		return fmt.Sprintf("Validating rollout (%d/%d tasks)", runningTasks, desiredTasks)
	}
	if updateState != "" {
		return fmt.Sprintf("Rolling out: %s (%d/%d tasks)", updateState, runningTasks, desiredTasks)
	}
	return fmt.Sprintf("Starting tasks (%d/%d)", runningTasks, desiredTasks)
}
