package fader

import (
	"fmt"
	"regexp"
)

const cacheSize = 64

type transition struct {
	appID  *regexp.Regexp
	class  *regexp.Regexp
	frames []float64
	cache  map[int64][]string
}

func newAppTransition(appID *regexp.Regexp, from, to float64, steps int) (*transition, error) {
	return &transition{
		appID:  appID,
		frames: calcFrames(from, to, steps),
		cache:  make(map[int64][]string, cacheSize),
	}, nil
}

func newClassTransition(class *regexp.Regexp, from, to float64, steps int) (*transition, error) {
	return &transition{
		class:  class,
		frames: calcFrames(from, to, steps),
		cache:  make(map[int64][]string, cacheSize),
	}, nil
}

func newTransition(from, to float64, steps int) (*transition, error) {
	return &transition{
		frames: calcFrames(from, to, steps),
		cache:  make(map[int64][]string, cacheSize),
	}, nil
}

func (t *transition) writeTo(dst CommandList, conID int64) {
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

type transitionList []*transition

func (list transitionList) findByAppID(appID string) *transition {
	for _, s := range list {
		if s.appID != nil && s.appID.MatchString(appID) {
			return s
		}
	}
	return nil
}

func (list transitionList) findByClass(class string) *transition {
	for _, s := range list {
		if s.class != nil && s.class.MatchString(class) {
			return s
		}
	}
	return nil
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
