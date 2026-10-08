// Package ui adds optional ANSI SGR styling to CLI output using only the
// standard library. Styling is decided per output stream.
package ui

import (
	"os"
	"strings"
)

// Painter wraps text in ANSI escapes when color is enabled. The zero value
// leaves text unchanged.
type Painter struct {
	on bool
}

// For returns a Painter for text written to f, honoring NO_COLOR, TERM=dumb,
// FORCE_COLOR and whether f is a character device.
func For(f *os.File) Painter {
	return Painter{on: Enabled(f, os.Getenv)}
}

// Enabled reports whether color should be emitted to f. NO_COLOR (non-empty)
// and TERM=dumb always disable color. Otherwise a non-empty FORCE_COLOR enables
// it, and without it color is enabled only when f is a terminal.
func Enabled(f *os.File, getenv func(string) string) bool {
	if getenv("NO_COLOR") != "" || getenv("TERM") == "dumb" {
		return false
	}
	if getenv("FORCE_COLOR") != "" {
		return true
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Enabled reports whether this Painter emits escape sequences.
func (p Painter) Enabled() bool { return p.on }

// Paint wraps text with the given SGR codes followed by a reset.
func (p Painter) Paint(text string, codes ...string) string {
	if !p.on || text == "" {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

// Bold styles text as bold.
func (p Painter) Bold(text string) string { return p.Paint(text, "1") }

// Dim styles text as dim.
func (p Painter) Dim(text string) string { return p.Paint(text, "2") }

// Red styles text as red.
func (p Painter) Red(text string) string { return p.Paint(text, "31") }

// Green styles text as green.
func (p Painter) Green(text string) string { return p.Paint(text, "32") }

// Yellow styles text as yellow.
func (p Painter) Yellow(text string) string { return p.Paint(text, "33") }

// Cyan styles text as cyan.
func (p Painter) Cyan(text string) string { return p.Paint(text, "36") }
