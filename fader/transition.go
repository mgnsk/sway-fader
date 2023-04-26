package fader

import (
	"fmt"
	"regexp"
)

type transition struct {
	appID  *regexp.Regexp
	class  *regexp.Regexp
	from   float64
	to     float64
	frames []float64
}

func (t *transition) writeTo(dst CommandList, conID int64) {
	for i, opacity := range t.frames {
		dst[i].WriteString(fmt.Sprintf(`[con_id=%d] opacity %.2f;`, conID, opacity))
	}
}

func (t *transition) calcFrames(numFrames int) {
	frames := make([]float64, numFrames)

	start := t.from
	end := t.to
	dist := end - start

	for i := 0; i < numFrames; i++ {
		x := float64(i+1) / float64(numFrames)
		frames[i] = x*dist + start
	}

	t.frames = frames
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
