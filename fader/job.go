package fader

import (
	"time"

	"go.i3wm.org/i3/v4"
)

func newFadeJob(commands []string, frameDur time.Duration) *fadeJob {
	j := &fadeJob{
		commands: commands,
		quit:     make(chan struct{}),
		done:     make(chan struct{}),
		frameDur: frameDur,
	}

	return j
}

type fadeJob struct {
	commands []string
	quit     chan struct{}
	done     chan struct{}
	frameDur time.Duration
}

func (j *fadeJob) Done() <-chan struct{} {
	return j.done
}

func (j *fadeJob) Run() error {
	defer close(j.done)

	// Run first command immediately and reset ticker for next frame.
	if _, err := i3.RunCommand(j.commands[0]); err != nil {
		return err
	}

	ticker := time.NewTicker(j.frameDur)
	defer ticker.Stop()

	for _, cmd := range j.commands[1:] {
		select {
		case <-j.quit:
			return nil
		case <-ticker.C:
			if _, err := i3.RunCommand(cmd); err != nil {
				return err
			}
		}
	}

	return nil
}

func (j *fadeJob) StopWait() {
	close(j.quit)
	<-j.done
}
