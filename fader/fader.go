// Package fader implements sway container fading.
package fader

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/fogleman/ease"
	"go.i3wm.org/i3/v4"
)

// Fader runs fades on containers.
type Fader struct {
	frameDur   time.Duration
	numFrames  int
	appFades   fadeList
	classFades fadeList
	running    map[i3.NodeID]*fadeJob
}

// StartFade starts a preconfigured fade on container.
func (h *Fader) StartFade(node *i3.Node) {
	if node.Type != i3.Con {
		panic(fmt.Sprintf("createConRequests: expected node type 'con', got %s", node.Type))
	}

	for id, job := range h.running {
		select {
		case <-job.Done():
			delete(h.running, id)
		default:
		}
	}

	if _, ok := h.running[node.ID]; ok {
		return
	}

	if t := h.getTransition(node); t != nil {
		job := newFadeJob(t, node.ID, h.frameDur)
		h.running[node.ID] = job
		go func() {
			if err := job.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %s", err.Error())
			}
		}()
	}
}

func (h *Fader) getTransition(con *i3.Node) transition {
	if con.AppID != "" {
		if t := h.appFades.find(con.AppID); t != nil {
			return t
		}
	}

	return h.classFades.find(con.WindowProperties.Class)
}

var defaultEaseFn = ease.Linear

type options struct {
	fps         float64
	fadeDur     time.Duration
	transitions []transitionOptions
}

type transitionOptions struct {
	appID, class *regexp.Regexp
	from, to     float64
	ease         string
}

func (transitionOptions) getEaseFunction() easeFunction {
	return defaultEaseFn
}

// Builder builds a fader.
type Builder func(*options)

// New creates a new fader.
func New() Builder {
	return func(*options) {}
}

// WithFadeDuration configures the fade duration.
func (build Builder) WithFadeDuration(d time.Duration) Builder {
	return func(o *options) {
		build(o)

		if d > 0 {
			o.fadeDur = d
		}
	}
}

// WithFPS configures the framerate for transitions.
func (build Builder) WithFPS(fps float64) Builder {
	return func(o *options) {
		build(o)

		if fps > 0 {
			o.fps = fps
		}
	}
}

// WithContainerAppIDFade configures a container's opacities by app_id.
func (build Builder) WithContainerAppIDFade(r *regexp.Regexp, from, to float64) Builder {
	return func(o *options) {
		build(o)

		o.transitions = append(o.transitions, transitionOptions{
			appID: r,
			from:  from,
			to:    to,
		})
	}
}

// WithContainerClassFade configures a container's opacities by class.
func (build Builder) WithContainerClassFade(r *regexp.Regexp, from, to float64) Builder {
	return func(o *options) {
		build(o)

		o.transitions = append(o.transitions, transitionOptions{
			class: r,
			from:  from,
			to:    to,
		})
	}
}

// Build the handler.
func (build Builder) Build() *Fader {
	o := options{
		fps:     DefaultFPS,
		fadeDur: DefaultDuration,
	}

	build(&o)

	frameDur := time.Duration((1.0 / o.fps) * float64(time.Second))
	numFrames := int(o.fadeDur / frameDur)

	appFades := fadeList{}
	classFades := fadeList{}

	for _, opt := range o.transitions {
		if opt.appID != nil {
			appFades = append(appFades, newFade(opt.appID, opt.from, opt.to, numFrames, opt.getEaseFunction()))
		} else if opt.class != nil {
			classFades = append(classFades, newFade(opt.class, opt.from, opt.to, numFrames, opt.getEaseFunction()))
		}
	}

	classFades = append(classFades, newFade(regexp.MustCompile(`.*`), DefaultFrom, DefaultTo, numFrames, defaultEaseFn))

	return &Fader{
		frameDur:   frameDur,
		numFrames:  numFrames,
		appFades:   appFades,
		classFades: classFades,
		running:    map[i3.NodeID]*fadeJob{},
	}
}
