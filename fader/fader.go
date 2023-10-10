package fader

import (
	"bytes"
	"fmt"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"github.com/fogleman/ease"
	"go.i3wm.org/i3/v4"
)

// Frames is a command buffer for each frame.
type Frames []*bytes.Buffer

// Fader runs fades on containers.
type Fader struct {
	frameDur   time.Duration
	numFrames  int
	appFades   fadeList
	classFades fadeList
	pool       sync.Pool
	cache      map[i3.NodeID][]string
	running    map[i3.NodeID]*fadeJob
}

// RunFade runs a preconfigured fade on container.
func (h *Fader) RunFade(node *i3.Node) {
	if node.Type != i3.Con {
		panic(fmt.Sprintf("createConRequests: expected node type 'con', got %s", node.Type))
	}

	if job, ok := h.running[node.ID]; ok {
		job.stop()
		h.putFrames(job.frames)
	}

	if t := h.getTransition(node); t != nil {
		frames := h.getTransitionFrames(t, node.ID)
		job := newFadeJob(frames, h.frameDur)
		go job.run()
		h.running[node.ID] = job
	}
}

func (h *Fader) getTransitionFrames(t transition, conID i3.NodeID) Frames {
	commands := h.getCommands(t, conID)
	frames := h.getFrames()

	for i, cmd := range commands {
		frames[i].WriteString(cmd)
	}

	return frames
}

func (h *Fader) getTransition(con *i3.Node) transition {
	if con.AppID != "" {
		if t := h.appFades.find(con.AppID); t != nil {
			return t
		}
	}

	return h.classFades.find(con.WindowProperties.Class)
}

func (h *Fader) getCommands(t transition, conID i3.NodeID) []string {
	commands, ok := h.cache[conID]
	if !ok {
		commands = make([]string, len(t))

		for i, opacity := range t {
			commands[i] = fmt.Sprintf(`[con_id=%d] opacity %.4f;`, conID, opacity)
		}

		h.cache[conID] = commands
	}

	return commands
}

func (h *Fader) getFrames() Frames {
	frames := make(Frames, h.numFrames)
	for i := 0; i < h.numFrames; i++ {
		buf := h.pool.Get().(*bytes.Buffer)
		buf.Reset()
		frames[i] = buf
	}
	return frames
}

func (h *Fader) putFrames(frames Frames) {
	for _, buf := range frames {
		h.pool.Put(buf)
	}
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
		pool: sync.Pool{
			New: func() any {
				return &bytes.Buffer{}
			},
		},
		cache:   map[i3.NodeID][]string{},
		running: map[i3.NodeID]*fadeJob{},
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