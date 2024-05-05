package fader

import (
	"fmt"
	"time"

	"go.i3wm.org/i3/v4"
)

func newFadeJob(t transition, conID i3.NodeID, frameDur time.Duration) *fadeJob {
	j := &fadeJob{
		transition: t,
		conID:      conID,
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
		frameDur:   frameDur,
	}

	return j
}

type fadeJob struct {
	transition transition
	conID      i3.NodeID
	quit       chan struct{}
	done       chan struct{}
	frameDur   time.Duration
}

func (j *fadeJob) Done() <-chan struct{} {
	return j.done
}

func (j *fadeJob) Run() error {
	defer close(j.done)

	// Run first command immediately and reset ticker for next frame.
	if _, err := i3.RunCommand(createCommand(j.conID, j.transition[0])); err != nil {
		return err
	}

	ticker := time.NewTicker(j.frameDur)
	defer ticker.Stop()

	for _, opacity := range j.transition[1:] {
		select {
		case <-j.quit:
			return nil
		case <-ticker.C:
			if _, err := i3.RunCommand(createCommand(j.conID, opacity)); err != nil {
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

func createCommand(conID i3.NodeID, opacity float64) string {
	return fmt.Sprintf(`[con_id=%d] opacity %.4f;`, conID, opacity)
}
