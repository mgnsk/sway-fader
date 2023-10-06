package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mgnsk/sway-fader/fader"
	"github.com/spf13/cobra"
	"go.i3wm.org/i3/v4"
)

var (
	fps                        = 60.0
	defaultTo                  = 1.0
	defaultFrom                = 0.7
	fadeDuration time.Duration = 200 * time.Millisecond
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

var root = &cobra.Command{
	Short: "sway-fader fades in windows on workspace switch and window creation.",
	Long: `
sway-fader fades in windows on workspace switch and window creation.

MIT License

Copyright (c) 2023 Magnus Kokk

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`,
	RunE: func(c *cobra.Command, args []string) error {
		builder := fader.New().WithFPS(fps).WithFadeDuration(fadeDuration)

		for _, target := range appIDTargets {
			sel, from, to, err := parseTarget(target)
			if err != nil {
				return err
			}
			re, err := regexp.Compile(sel)
			if err != nil {
				return err
			}
			builder = builder.WithContainerAppIDFade(re, from, to)
		}

		for _, target := range classTargets {
			sel, from, to, err := parseTarget(target)
			if err != nil {
				return err
			}
			re, err := regexp.Compile(sel)
			if err != nil {
				return err
			}
			builder = builder.WithContainerClassFade(re, from, to)
		}

		builder = builder.WithContainerClassFade(regexp.MustCompile(".*"), defaultFrom, defaultTo)
		f := builder.Build()

		socketPath, err := getSocketPath()
		if err != nil {
			return err
		}

		i3.SocketPathHook = func() (string, error) {
			return socketPath, nil
		}

		// Fade in the focused workspace.
		{
			tree, err := i3.GetTree()
			if err != nil {
				return err
			}

			fader.WalkTree(tree.Root, func(node *i3.Node) bool {
				if node.Type == i3.Con && node.Focused {
					f.WorkspaceFocus(node)
					return false
				}
				return true
			})
		}

		go func() {
			r := i3.Subscribe(i3.WorkspaceEventType, i3.WindowEventType)
			for r.Next() {
				switch ev := r.Event().(type) {
				case *i3.WorkspaceEvent:
					if ev.Change == "focus" {
						f.WorkspaceFocus(&ev.Current)
					}
				case *i3.WindowEvent:
					if ev.Change == "new" {
						f.WindowNew(&ev.Container)
					}
				}
			}
			log.Fatal(r.Close())
		}()

		<-c.Context().Done()

		return c.Context().Err()
	},
}

func main() {
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s", err.Error())
		os.Exit(1)
	}
}

func parseTarget(flagValue string) (selector string, from, to float64, err error) {
	parts := strings.Split(flagValue, ":")
	if len(parts) != 3 {
		return "", 0, 0, fmt.Errorf("invalid number of target components: %s", flagValue)
	}

	from, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid from value in target '%s': %w", flagValue, err)
	}

	to, err = strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("invalid to value in target '%s': %w", flagValue, err)
	}

	return parts[0], from, to, nil
}

func getSocketPath() (string, error) {
	if _, err := exec.LookPath("sway"); err == nil {
		out, err := exec.Command("sway", "--get-socketpath").CombinedOutput()
		return string(out), err
	}

	if _, err := exec.LookPath("i3"); err == nil {
		out, err := exec.Command("i3", "--get-socketpath").CombinedOutput()
		return string(out), err
	}

	return "", fmt.Errorf("could not find sway or i3 executable")
}
