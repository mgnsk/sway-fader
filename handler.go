package main

import (
	"bytes"
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

	cancel func()
	cmdBuf *bytes.Buffer
	sway.EventHandler
}

func walkTree(node *sway.Node, f func(node *sway.Node)) {
	f(node)

	for _, n := range node.Nodes {
		walkTree(n, f)
	}

	for _, n := range node.FloatingNodes {
		walkTree(n, f)
	}
}

// Workspace handles workspace focus events.
func (h *SwayEventHandler) Workspace(ctx context.Context, e sway.WorkspaceEvent) {
	switch e.Change {
	case sway.WorkspaceFocus:
		if h.cancel != nil {
			h.cancel()
			h.cancel = nil
		}

		if e.Current == nil {
			return
		}

		tree, err := h.client.GetTree(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err.Error())
			return
		}

		var visibleContainers []*sway.Node

		walkTree(tree, func(node *sway.Node) {
			if (node.Type == sway.NodeCon || node.Type == sway.NodeFloatingCon) &&
				node.Visible != nil && *node.Visible {
				visibleContainers = append(visibleContainers, node)
			}
		})

		if len(visibleContainers) > 0 {
			h.cancel = h.runJob(ctx, visibleContainers)
		}
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

// runJob runs a job and returns a callback which cancels the job
// and waits for pending requests to finish. runJob must not be called
// again until the previous stop has returned unless it's the first call.
func (h *SwayEventHandler) runJob(ctx context.Context, containers []*sway.Node) (stop func()) {
	cancel := make(chan struct{})
	wg := sync.WaitGroup{}
	requests := make([][]string, h.numFrames)

	for _, node := range containers {
		foundAppID := false

		if node.AppID != nil {
			if s := h.findAppIDTarget(*node.AppID); s != nil {
				for i, opacity := range s.frames {
					requests[i] = append(requests[i], fmt.Sprintf(`[app_id="%s"] opacity %.2f`, *node.AppID, opacity))
				}
				foundAppID = true
			}
		}

		if !foundAppID {
			var class string
			if p := node.WindowProperties; p != nil {
				class = p.Class
			}
			if s := h.findClassTarget(class); s != nil {
				for i, opacity := range s.frames {
					requests[i] = append(requests[i], fmt.Sprintf(`[class="%s"] opacity %.2f`, class, opacity))
				}
			}
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for _, targets := range requests {
			select {
			case <-ctx.Done():
				return
			case <-cancel:
				return
			case <-h.ticker.C:
				cmd := strings.Join(targets, "; ")

				if _, err := h.client.RunCommand(ctx, cmd); err != nil {
					fmt.Fprintf(os.Stderr, "%s", err.Error())
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
		cmdBuf:       &bytes.Buffer{},
		EventHandler: sway.NoOpEventHandler(),
	}, nil
}
