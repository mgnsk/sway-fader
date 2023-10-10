package fader

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.i3wm.org/i3/v4"
)

func newFadeJob(commands []string, frameDur time.Duration) *fadeJob {
	j := &fadeJob{
		commands: commands,
		done:     make(chan struct{}),
		frameDur: frameDur,
	}

	j.wg.Add(1)

	return j
}

type fadeJob struct {
	commands []string
	done     chan struct{}
	wg       sync.WaitGroup
	frameDur time.Duration
}

func (j *fadeJob) run() {
	defer j.wg.Done()

	// Run first command immediately and reset ticker for next frame.
	if _, err := i3.RunCommand(j.commands[0]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err.Error())
	}

	ticker := time.NewTicker(j.frameDur)
	defer ticker.Stop()

	for _, cmd := range j.commands[1:] {
		select {
		case <-j.done:
			return
		case <-ticker.C:
			if _, err := i3.RunCommand(cmd); err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err.Error())
			}
		}
	}
}

func (j *fadeJob) stop() {
	close(j.done)
	j.wg.Wait()
}
