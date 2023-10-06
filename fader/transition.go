package fader

import (
	"fmt"
	"regexp"

	"go.i3wm.org/i3/v4"
)

type fade struct {
	re *regexp.Regexp
	t  *transition
}

func newFade(re *regexp.Regexp, from, to float64, steps int) *fade {
	return &fade{
		re: re,
		t:  newTransition(from, to, steps),
	}
}

type fadeList []*fade

func (l *fadeList) find(s string) *transition {
	for _, f := range *l {
		if f.re.MatchString(s) {
			return f.t
		}
	}
	return nil
}

const cacheSize = 64

func newTransition(from, to float64, steps int) *transition {
	return &transition{
		frames: calcFrames(from, to, steps),
		cache:  make(map[i3.NodeID][]string, cacheSize),
	}
}

type transition struct {
	frames []float64
	cache  map[i3.NodeID][]string
}

func (t *transition) writeTo(dst Frames, conID i3.NodeID) {
	commands, ok := t.cache[conID]
	if !ok {
		commands = make([]string, len(t.frames))

		for i, opacity := range t.frames {
			commands[i] = fmt.Sprintf(`[con_id=%d] opacity %.2f;`, conID, opacity)
		}

		if len(commands) == cacheSize {
			clear(commands)
		}

		t.cache[conID] = commands
	}

	for i, cmd := range commands {
		dst[i].WriteString(cmd)
	}
}

func calcFrames(from, to float64, steps int) []float64 {
	frames := make([]float64, steps)

	dist := to - from

	for i := 0; i < steps; i++ {
		// TODO: configure transition type
		x := float64(i+1) / float64(steps)
		frames[i] = x*dist + from
	}

	return frames
}
