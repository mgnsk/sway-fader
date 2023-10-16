package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mgnsk/sway-fader/fader"
	"github.com/mitchellh/go-ps"
	"github.com/spf13/cobra"
	"go.i3wm.org/i3/v4"
)

var (
	fps                        = fader.DefaultFPS
	defaultTo                  = fader.DefaultTo
	defaultFrom                = fader.DefaultFrom
	fadeDuration time.Duration = fader.DefaultDuration
	appIDTargets []string
	classTargets []string
)

func init() {
	root.PersistentFlags().Float64Var(&fps, "fps", fps, "Frames per second for the fade")
	root.PersistentFlags().Float64Var(&defaultFrom, "default-from", defaultFrom, "Default opacity when fade starts")
	root.PersistentFlags().Float64Var(&defaultTo, "default-to", defaultTo, "Default final opacity of fade")
	root.PersistentFlags().DurationVarP(&fadeDuration, "duration", "d", fadeDuration, "Duration of the fade")
	root.PersistentFlags().StringArrayVar(&appIDTargets, "app_id", appIDTargets, `Override fade settings per container app_id. Format: "regex:from:to". Example: --app_id="foot:0.7:0.97" --app_id="org.telegram.desktop:0.8:1.0"`)
	root.PersistentFlags().StringArrayVar(&classTargets, "class", classTargets, `Override fade settings per container class. Format: "regex:from:to". Example: --class="FreeTube:0.7:1.0" --class="Firefox:0.8:1.0"`)
}

func main() {
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err.Error())
		os.Exit(1)
	}
}

var root = &cobra.Command{
	Short: "sway-fader fades in sway or i3 windows on workspace focus and window new events.",
	RunE: func(c *cobra.Command, args []string) error {
		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		i3.SocketPathHook = func() (string, error) {
			return socketPath, nil
		}

		f, err := newFader()
		if err != nil {
			return err
		}

		// Fade in all windows on start.
		{
			tree, err := i3.GetTree()
			if err != nil {
				return err
			}

			walkTree(tree.Root, func(node *i3.Node) bool {
				if node.Type == i3.Con {
					f.StartFade(node)
				}
				return true
			})
		}

		go func() {
			r := i3.Subscribe(i3.WorkspaceEventType, i3.WindowEventType)
			for r.Next() {
				switch ev := r.Event().(type) {
				case *i3.WindowEvent:
					if ev.Change == "new" {
						f.StartFade(&ev.Container)
					}
				case *i3.WorkspaceEvent:
					if ev.Change == "focus" {
						walkTree(&ev.Current, func(node *i3.Node) bool {
							if node.Type == i3.Con {
								f.StartFade(node)
							}
							return true
						})
					}
				}
			}
			log.Fatal(r.Close())
		}()

		<-c.Context().Done()

		return c.Context().Err()
	},
}

func newFader() (*fader.Fader, error) {
	builder := fader.New().WithFPS(fps).WithFadeDuration(fadeDuration)

	for _, target := range appIDTargets {
		re, from, to, err := parseTarget(target)
		if err != nil {
			return nil, err
		}
		builder = builder.WithContainerAppIDFade(re, from, to)
	}

	for _, target := range classTargets {
		re, from, to, err := parseTarget(target)
		if err != nil {
			return nil, err
		}
		builder = builder.WithContainerClassFade(re, from, to)
	}

	return builder.Build(), nil
}

func parseTarget(flagValue string) (selector *regexp.Regexp, from, to float64, err error) {
	parts := strings.Split(flagValue, ":")
	if len(parts) != 3 {
		return nil, 0, 0, fmt.Errorf("invalid number of target components: %s", flagValue)
	}

	from, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid from value in target '%s': %w", flagValue, err)
	}

	to, err = strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid to value in target '%s': %w", flagValue, err)
	}

	re, err := regexp.Compile(parts[0])
	if err != nil {
		return nil, 0, 0, err
	}

	return re, from, to, nil
}

func getSocketPath() (string, error) {
	procs, err := ps.Processes()
	if err != nil {
		return "", err
	}

	for _, wm := range []string{"sway", "i3"} {
		if slices.ContainsFunc(procs, func(p ps.Process) bool {
			return p.Executable() == wm
		}) {
			out, err := exec.Command(wm, "--get-socketpath").CombinedOutput()
			return string(out), err
		}
	}

	return "", fmt.Errorf("could not find a running sway or i3 process")
}

func walkTree(node *i3.Node, f func(node *i3.Node) bool) bool {
	if !f(node) {
		return false
	}

	for _, n := range node.Nodes {
		if !walkTree(n, f) {
			return false
		}
	}

	return true
}
