package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joshuarubin/go-sway"
	"github.com/mgnsk/sway-fader/fader"
	"github.com/spf13/cobra"
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
		writeClient, err := sway.New(c.Context())
		if err != nil {
			return err
		}

		b := fader.NewHandler(writeClient, fps, fadeDuration)

		b = b.WithContainerClassFade(".*", defaultFrom, defaultTo)

		for _, target := range appIDTargets {
			sel, from, to, err := parseTarget(target)
			if err != nil {
				return err
			}
			b = b.WithContainerAppIDFade(sel, from, to)
		}

		for _, target := range classTargets {
			sel, from, to, err := parseTarget(target)
			if err != nil {
				return err
			}
			b = b.WithContainerClassFade(sel, from, to)
		}

		h, err := b.Build()
		if err != nil {
			return err
		}

		return sway.Subscribe(
			c.Context(),
			h,
			sway.EventTypeWorkspace,
			sway.EventTypeWindow,
		)
	},
}

func main() {
	if err := root.ExecuteContext(context.TODO()); err != nil {
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
