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

// SwayEventHandler handles events and triggers fades.
type SwayEventHandler struct {
	frameDur    time.Duration
	numFrames   int
	transitions transitionList
	pool        sync.Pool

	jobs []func()
}

// Frames is a command buffer for each frame.
type Frames []*bytes.Buffer

// Window handles window creation events.
func (h *SwayEventHandler) Window(e *i3.WindowEvent) {
	if e.Change == "new" {
		frames := h.getBuffer()

		h.writeConRequests(frames, &e.Container)
		h.jobs = append(h.jobs, h.createJob(frames))
	}
}

// Workspace handles workspace focus events.
func (h *SwayEventHandler) Workspace(e *i3.WorkspaceEvent) {
	if e.Change == "focus" {
		for _, stop := range h.jobs {
			stop()
		}
		h.jobs = h.jobs[:0]

		frames := h.getBuffer()

		walkTree(&e.Current, func(node *i3.Node) {
			if node.Type == i3.Con {
				h.writeConRequests(frames, node)
			}
		})

		h.jobs = append(h.jobs, h.createJob(frames))
	}
}

func walkTree(node *i3.Node, f func(node *i3.Node)) {
	f(node)

	for _, n := range node.Nodes {
		walkTree(n, f)
	}
}

func (h *SwayEventHandler) writeConRequests(dst Frames, con *i3.Node) {
	if con.Type != i3.Con {
		panic(`createConRequests: expected node type "con"`)
	}

	if con.AppID != "" {
		if t := h.transitions.findByAppID(con.AppID); t != nil {
			t.writeTo(dst, con.ID)
			return
		}
	}

	if t := h.transitions.findByClass(con.WindowProperties.Class); t != nil {
		t.writeTo(dst, con.ID)
	}
}

// createJob runs a job and returns a callback which cancels the job
// and waits for pending requests to finish.
func (h *SwayEventHandler) createJob(frames Frames) (stop func()) {
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

	return func() {
		close(done)
		wg.Wait()
	}
}

func (h *SwayEventHandler) getBuffer() Frames {
	frames, ok := h.pool.Get().(Frames)
	if !ok {
		frames = make(Frames, h.numFrames)
		for i := 0; i < h.numFrames; i++ {
			frames[i] = &bytes.Buffer{}
		}
	}
	return frames
}

func (h *SwayEventHandler) putBuffer(frames Frames) {
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

// Builder builds a handler.
type Builder func(*options) error

// NewHandler creates a new event handler.
func NewHandler(fps float64, fadeDur time.Duration) Builder {
	return func(o *options) error {
		o.fps = fps
		o.fadeDur = fadeDur

		return nil
	}
}

// WithContainerAppIDFade configures a container's opacities by app_id.
func (build Builder) WithContainerAppIDFade(appIDRegex string, from, to float64) Builder {
	return func(o *options) error {
		if err := build(o); err != nil {
			return err
		}

		r, err := regexp.Compile(appIDRegex)
		if err != nil {
			return err
		}

		o.transitions = append(o.transitions, transitionOptions{
			appID: r,
			from:  from,
			to:    to,
		})

		return nil
	}
}

// WithContainerClassFade configures a container's opacities by class.
func (build Builder) WithContainerClassFade(classRegex string, from, to float64) Builder {
	return func(o *options) error {
		if err := build(o); err != nil {
			return err
		}

		r, err := regexp.Compile(classRegex)
		if err != nil {
			return err
		}

		o.transitions = append(o.transitions, transitionOptions{
			class: r,
			from:  from,
			to:    to,
		})

		return nil
	}
}

// Build the handler.
func (build Builder) Build() (*SwayEventHandler, error) {
	o := options{}
	if err := build(&o); err != nil {
		return nil, err
	}

	frameDur := time.Duration((1.0 / o.fps) * float64(time.Second))
	numFrames := int(o.fadeDur / frameDur)

	list := make(transitionList, len(o.transitions))

	for i, opt := range o.transitions {
		if opt.appID != nil {
			tr, err := newAppTransition(opt.appID, opt.from, opt.to, numFrames)
			if err != nil {
				return nil, err
			}
			list[i] = tr
		} else if opt.class != nil {
			tr, err := newClassTransition(opt.class, opt.from, opt.to, numFrames)
			if err != nil {
				return nil, err
			}
			list[i] = tr
		}
	}

	return &SwayEventHandler{
		frameDur:    frameDur,
		numFrames:   numFrames,
		transitions: list,
		pool: sync.Pool{
			New: func() any {
				frames := make(Frames, numFrames)
				for i := 0; i < numFrames; i++ {
					frames[i] = &bytes.Buffer{}
				}
				return frames
			},
		},
	}, nil
}

// bytesToString returns an unsafe string using the underlying slice.
func bytesToString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
