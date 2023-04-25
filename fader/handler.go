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
	client    sway.Client
	ticker    *time.Ticker
	frameDur  time.Duration
	numFrames int
	settings  []*opacitySetting

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
		for _, cancel := range h.jobs {
			cancel()
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

func (h *SwayEventHandler) findAppIDTarget(appID string) *opacitySetting {
	for _, s := range h.settings {
		if s.appID != nil && s.appID.MatchString(appID) {
			return s
		}
	}
	return nil
}

func (h *SwayEventHandler) findClassTarget(class string) *opacitySetting {
	for _, s := range h.settings {
		if s.class != nil && s.class.MatchString(class) {
			return s
		}
	}
	return nil
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
		if s := h.findAppIDTarget(*con.AppID); s != nil {
			for i, opacity := range s.frames {
				dst[i].WriteString(fmt.Sprintf(`[con_id=%d] opacity %.2f;`, con.ID, opacity))
			}
			foundAppID = true
		}
	}

	if !foundAppID {
		var class string
		if p := con.WindowProperties; p != nil {
			class = p.Class
		}
		if s := h.findClassTarget(class); s != nil {
			for i, opacity := range s.frames {
				dst[i].WriteString(fmt.Sprintf(`[con_id=%d] opacity %.2f;`, con.ID, opacity))
			}
		}
	}
}

// createJob runs a job and returns a callback which cancels the job
// and waits for pending requests to finish.
func (h *SwayEventHandler) createJob(ctx context.Context, cmdList CommandList) (stop func()) {
	// Run first command synchronously and reset ticker for next frame.
	if _, err := h.client.RunCommand(ctx, cmdList[0].String()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err.Error())
		return
	}
	h.ticker.Reset(h.frameDur)

	cancel := make(chan struct{})
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for _, cmd := range cmdList[1:] {
			select {
			case <-ctx.Done():
				return
			case <-cancel:
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
		close(cancel)
		wg.Wait()
	}
}

type opacitySetting struct {
	appID  *regexp.Regexp
	class  *regexp.Regexp
	from   float64
	to     float64
	frames []float64
}

func (s *opacitySetting) calcFrames(numFrames int) {
	frames := make([]float64, numFrames)

	start := s.from
	end := s.to
	dist := end - start

	for i := 0; i < numFrames; i++ {
		x := float64(i+1) / float64(numFrames)
		frames[i] = x*dist + start
	}

	s.frames = frames
}

type options struct {
	client   sway.Client
	fps      float64
	fadeDur  time.Duration
	settings []*opacitySetting
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

		o.settings = append(o.settings, &opacitySetting{
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

		o.settings = append(o.settings, &opacitySetting{
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

	for _, s := range o.settings {
		s.calcFrames(numFrames)
	}

	return &SwayEventHandler{
		client:       o.client,
		ticker:       time.NewTicker(frameDur),
		frameDur:     frameDur,
		numFrames:    numFrames,
		settings:     o.settings,
		EventHandler: sway.NoOpEventHandler(),
	}, nil
}
