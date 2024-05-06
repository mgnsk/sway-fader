package fader

import "time"

// Fader constants.
const (
	DefaultFrom     = 0.7
	DefaultTo       = 1.0
	DefaultFPS      = 60.0
	DefaultDuration = 200 * time.Millisecond
	DefaultEase     = "linear" // camelCase function name from https://github.com/fogleman/ease
)
