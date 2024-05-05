package fader

import (
	"fmt"
	"time"

	"go.i3wm.org/i3/v4"
)

func newFadeJob(t transition, id i3.NodeID, frameDur time.Duration) *fadeJob {
	j := &fadeJob{
		done:       make(chan struct{}),
		transition: t,
		id:         id,
		frameDur:   frameDur,
	}

	return j
}

type fadeJob struct {
	done       chan struct{}
	transition transition
	id         i3.NodeID
	frameDur   time.Duration
}

func (j *fadeJob) Done() <-chan struct{} {
	return j.done
}

func (j *fadeJob) Run() error {
	defer close(j.done)

	// Run first command immediately and reset ticker for next frame.
	if _, err := i3.RunCommand(createCommand(j.id, j.transition[0])); err != nil {
		return err
	}

	ticker := time.NewTicker(j.frameDur)
	defer ticker.Stop()

	for _, opacity := range j.transition[1:] {
		select {
		case <-ticker.C:
			if _, err := i3.RunCommand(createCommand(j.id, opacity)); err != nil {
				return err
			}
		}
	}

	return nil
}

func createCommand(id i3.NodeID, opacity float64) string {
	return fmt.Sprintf(`[con_id=%d] opacity %.4f;`, id, opacity)
}
