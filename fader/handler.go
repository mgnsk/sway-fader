package fader

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/joshuarubin/go-sway"
)

// SwayEventHandler handles sway events and triggers fades.
type SwayEventHandler struct {
	client      sway.Client
	ticker      *time.Ticker
	frameDur    time.Duration
	numFrames   int
	transitions transitionList

	jobs []func()
	sway.EventHandler
}

// CommandList is a list of sway commands.
type CommandList []strings.Builder

// Window handles window creation events.
func (h *SwayEventHandler) Window(ctx context.Context, e sway.WindowEvent) {
	switch e.Change {
	case sway.WindowNew:
		requests := make(CommandList, h.numFrames)
		h.createConRequests(requests, &e.Container)
		h.jobs = append(h.jobs, h.createJob(ctx, requests))
	}
}

// Workspace handles workspace focus events.
func (h *SwayEventHandler) Workspace(ctx context.Context, e sway.WorkspaceEvent) {
	switch e.Change {
	case sway.WorkspaceFocus:
		for _, stop := range h.jobs {
			stop()
		}
		h.jobs = h.jobs[:0]

		if e.Current == nil {
			return
		}

		requests := make(CommandList, h.numFrames)
		walkTree(e.Current, func(node *sway.Node) {
			if node.Type == sway.NodeCon {
				h.createConRequests(requests, node)
			}
		})

		h.jobs = append(h.jobs, h.createJob(ctx, requests))
	}
}

func walkTree(node *sway.Node, f func(node *sway.Node)) {
	f(node)

	for _, n := range node.Nodes {
		walkTree(n, f)
	}
}

func (h *SwayEventHandler) createConRequests(dst CommandList, con *sway.Node) {
	if con.Type != sway.NodeCon {
		panic(`createConRequests: expected node type "con"`)
	}

	foundAppID := false
	if con.AppID != nil {
		if t := h.transitions.findByAppID(*con.AppID); t != nil {
			t.writeTo(dst, con.ID)
			foundAppID = true
		}
	}

	if !foundAppID {
		var class string
		if p := con.WindowProperties; p != nil {
			class = p.Class
		}
		if t := h.transitions.findByClass(class); t != nil {
			t.writeTo(dst, con.ID)
		}
	}
}

// createJob runs a job and returns a callback which cancels the job
// and waits for pending requests to finish.
func (h *SwayEventHandler) createJob(ctx context.Context, cmdList CommandList) (stop func()) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		// Run first command immediately and reset ticker for next frame.
		if _, err := h.client.RunCommand(ctx, cmdList[0].String()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s", err.Error())
			return
		}

		h.ticker.Reset(h.frameDur)

		for _, cmd := range cmdList[1:] {
			select {
			case <-ctx.Done():
				return
			case <-h.ticker.C:
				if _, err := h.client.RunCommand(ctx, cmd.String()); err != nil {
					fmt.Fprintf(os.Stderr, "error: %s", err.Error())
					return
				}
			}
		}
	}()

	return func() {
		cancel()
		wg.Wait()
	}
}

type options struct {
	client      sway.Client
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

// NewHandler creates a new sway event handler.
func NewHandler(client sway.Client, fps float64, fadeDur time.Duration) Builder {
	return func(o *options) error {
		o.client = client
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
		client:       o.client,
		ticker:       time.NewTicker(frameDur),
		frameDur:     frameDur,
		numFrames:    numFrames,
		transitions:  list,
		EventHandler: sway.NoOpEventHandler(),
	}, nil
}
