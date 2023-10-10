package fader

import (
	"regexp"
)

type transition []float64

type easeFunction func(float64) float64

type fade struct {
	re *regexp.Regexp
	t  transition
}

func newFade(re *regexp.Regexp, from, to float64, steps int, f easeFunction) *fade {
	return &fade{
		re: re,
		t:  newTransition(from, to, steps, f),
	}
}

type fadeList []*fade

func (l fadeList) find(s string) transition {
	for _, f := range l {
		if f.re.MatchString(s) {
			return f.t
		}
	}
	return nil
}

const cacheSize = 64

func newTransition(from, to float64, steps int, f easeFunction) transition {
	frames := make([]float64, steps)

	dist := to - from

	for i := 0; i < steps; i++ {
		x := float64(i+1) / float64(steps)
		x = f(x)
		frames[i] = x*dist + from
	}

	return frames
}
