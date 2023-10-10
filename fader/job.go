package fader

import (
	"fmt"
	"os"
	"sync"
	"time"

	"go.i3wm.org/i3/v4"
)

func newFadeJob(frames Frames, frameDur time.Duration) *fadeJob {
	j := &fadeJob{
		frames:   frames,
		done:     make(chan struct{}),
		frameDur: frameDur,
	}

	j.wg.Add(1)

	return j
}

type fadeJob struct {
	frames   Frames
	done     chan struct{}
	wg       sync.WaitGroup
	frameDur time.Duration
}

func (j *fadeJob) run() {
	defer j.wg.Done()

	// Run first command immediately and reset ticker for next frame.
	if _, err := i3.RunCommand(bytesToString(j.frames[0].Bytes())); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err.Error())
	}

	ticker := time.NewTicker(j.frameDur)
	defer ticker.Stop()

	for _, frame := range j.frames[1:] {
		select {
		case <-j.done:
			return
		case <-ticker.C:
			if _, err := i3.RunCommand(bytesToString(frame.Bytes())); err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err.Error())
			}
		}
	}
}

func (j *fadeJob) stop() {
	close(j.done)
	j.wg.Wait()
}
