package fader

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"go.i3wm.org/i3/v4"
)

// Fader handles events and triggers fades.
type Fader struct {
	frameDur   time.Duration
	numFrames  int
	appFades   fadeList
	classFades fadeList
	pool       sync.Pool

	jobs []func()
}

// Frames is a command buffer for each frame.
type Frames []*bytes.Buffer

// WindowNew handles window new event.
func (h *Fader) WindowNew(window *i3.Node) {
	frames := h.getBuffer()

	h.writeConRequests(frames, window)
	h.runJob(frames)
}

// WorkspaceFocus handles workspace focus event.
func (h *Fader) WorkspaceFocus(workspace *i3.Node) {
	for _, stop := range h.jobs {
		stop()
	}
	h.jobs = h.jobs[:0]

	frames := h.getBuffer()

	WalkTree(workspace, func(node *i3.Node) bool {
		if node.Type == i3.Con {
			h.writeConRequests(frames, node)
		}
		return true
	})

	h.runJob(frames)
}

func (h *Fader) writeConRequests(dst Frames, con *i3.Node) {
	if con.Type != i3.Con {
		panic(`createConRequests: expected node type "con"`)
	}

	if con.AppID != "" {
		if t := h.appFades.find(con.AppID); t != nil {
			t.writeTo(dst, con.ID)
			return
		}
	}

	if t := h.classFades.find(con.WindowProperties.Class); t != nil {
		t.writeTo(dst, con.ID)
	}
}

func (h *Fader) runJob(frames Frames) {
	wg := sync.WaitGroup{}
	done := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer h.putBuffer(frames)

		// Run first command immediately and reset ticker for next frame.
		if _, err := i3.RunCommand(bytesToString(frames[0].Bytes())); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err.Error())
		}

		ticker := time.NewTicker(h.frameDur)
		defer ticker.Stop()

		for _, frame := range frames[1:] {
			select {
			case <-done:
				return
			case <-ticker.C:
				if _, err := i3.RunCommand(bytesToString(frame.Bytes())); err != nil {
					fmt.Fprintf(os.Stderr, "error: %s", err.Error())
				}
			}
		}
	}()

	h.jobs = append(h.jobs, func() {
		close(done)
		wg.Wait()
	})
}

func (h *Fader) getBuffer() Frames {
	frames, ok := h.pool.Get().(Frames)
	if !ok {
		frames = make(Frames, h.numFrames)
		for i := 0; i < h.numFrames; i++ {
			frames[i] = &bytes.Buffer{}
		}
	}
	return frames
}

func (h *Fader) putBuffer(frames Frames) {
	for _, buf := range frames {
		buf.Reset()
	}
	h.pool.Put(frames)
}

type options struct {
	fps         float64
	fadeDur     time.Duration
	transitions []transitionOptions
}

type transitionOptions struct {
	appID, class *regexp.Regexp
	from, to     float64
}

// Builder builds a fader.
type Builder func(*options)

// New creates a new fader.
func New() Builder {
	return func(o *options) {}
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
			appFades = append(appFades, newFade(opt.appID, opt.from, opt.to, numFrames))
		} else if opt.class != nil {
			classFades = append(classFades, newFade(opt.class, opt.from, opt.to, numFrames))
		}
	}

	classFades = append(classFades, newFade(regexp.MustCompile(`.*`), DefaultFrom, DefaultTo, numFrames))

	return &Fader{
		frameDur:   frameDur,
		numFrames:  numFrames,
		appFades:   appFades,
		classFades: classFades,
	}
}

// WalkTree walks i3 node tree.
func WalkTree(node *i3.Node, f func(node *i3.Node) bool) bool {
	if !f(node) {
		return false
	}

	for _, n := range node.Nodes {
		if !WalkTree(n, f) {
			return false
		}
	}

	return true
}

// bytesToString returns an unsafe string using the underlying slice.
func bytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
