package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The banner's art: a Bayer-dithered sweep of greens, light at the left
// fading into the dark, drawn with half-block pixels (two per cell).
const (
	bannerW = 40 // pixels, and columns
	bannerH = 12 // pixels; 6 lines
)

var (
	bannerRamp = []lipgloss.Color{"#1f4a2a", "#2f6b3a", "#3f8f45", "#6fb84a", "#a6dc6e", "#d8f5a2"}
	cream      = lipgloss.Color("#f2f7ec")
	bayer      = [4][4]float64{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
)

// ditherPixel returns the color of pixel (x, y), or "" where the dither
// leaves it empty.
func ditherPixel(x, y int) lipgloss.Color {
	v := 1 - (float64(x)/bannerW*0.85 + float64(y)/bannerH*0.3)
	v = v*1.2 - 0.05
	if v <= (bayer[y%4][x%4]+0.5)/16 {
		return ""
	}
	t := math.Min(0.9999, 0.25+0.75*v) * float64(len(bannerRamp))
	return bannerRamp[int(math.Max(0, t))]
}

// art renders the dither as lines of half blocks.
func (u *UI) art() []string {
	lines := make([]string, 0, bannerH/2)
	for y := 0; y < bannerH; y += 2 {
		var b strings.Builder
		for x := 0; x < bannerW; x++ {
			top, bottom := ditherPixel(x, y), ditherPixel(x, y+1)
			switch {
			case top == "" && bottom == "":
				b.WriteString(" ")
			case bottom == "":
				b.WriteString(u.style().Foreground(top).Render("▀"))
			case top == "":
				b.WriteString(u.style().Foreground(bottom).Render("▄"))
			default:
				b.WriteString(u.style().Foreground(top).Background(bottom).Render("▀"))
			}
		}
		lines = append(lines, b.String())
	}
	return lines
}

// Banner prints the dither art beside the name, version and a subtitle.
// Plain output gets nothing: it's decoration.
func (u *UI) Banner(version, subtitle string) {
	if !u.Interactive {
		return
	}
	bold := u.style().Bold(true)
	text := []string{
		"",
		bold.Foreground(cream).Render("lima") + bold.Foreground(Leaf).Render("bean") + " " + u.Dimmed(version),
		u.Dimmed("Disposable Linux dev VMs,"),
		u.Dimmed("ready for SSH and Claude."),
		"",
		u.color(Leaf, "›") + " " + subtitle,
	}
	fmt.Fprintln(u.Out)
	for i, l := range u.art() {
		fmt.Fprintf(u.Out, "  %s   %s\n", l, text[i])
	}
	fmt.Fprintln(u.Out)
}
