package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joshuarubin/go-sway"
	"github.com/spf13/cobra"
)

var (
	fps          float64
	defaultTo    float64
	defaultFrom  float64
	fadeDuration time.Duration
	appIDTargets []string
	classTargets []string
)

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

func main() {
	root := &cobra.Command{
		Short: "sway-fader fades in visible containers on workspace focus.",
		Long: `
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

			b := NewHandler(writeClient, fps, fadeDuration)

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
			)
		},
	}

	root.PersistentFlags().Float64Var(&fps, "fps", 60, "Frames per second for the fade")
	root.PersistentFlags().Float64Var(&defaultFrom, "default-from", 0.7, "Default opacity when fade starts")
	root.PersistentFlags().Float64Var(&defaultTo, "default-to", 1.0, "Default final opacity of fade")
	root.PersistentFlags().DurationVarP(&fadeDuration, "duration", "d", 200*time.Millisecond, "Duration of the fade")
	root.PersistentFlags().StringArrayVar(&appIDTargets, "app_id", nil, `Override fade settings per container app_id. Format: "regex:from:to". Example: --app_id="foot:0.7:0.97" --app_id="org.telegram.desktop:0.8:1.0"`)
	root.PersistentFlags().StringArrayVar(&classTargets, "class", nil, `Override fade settings per container class. Format: "regex:from:to". Example: --class="FreeTube:0.7:1.0" --class="Firefox:0.8:1.0"`)

	if err := root.ExecuteContext(context.TODO()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
