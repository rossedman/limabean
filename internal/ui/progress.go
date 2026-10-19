package ui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Progress is how far a long step has got, such as a download.
type Progress struct {
	Label       string  // what's under way, e.g. "Downloading ubuntu.img"
	Done, Total int64   // bytes so far and in all; Total is 0 when unknown
	Rate        float64 // bytes a second; 0 until known
}

// Fraction is how much is done, from 0 to 1, or -1 when Total isn't known.
func (p Progress) Fraction() float64 {
	if p.Total <= 0 {
		return -1
	}
	return min(float64(p.Done)/float64(p.Total), 1)
}

// String is the progress in words: "Downloading x: 412 MiB of 902 MiB,
// 1.1 MiB/s, about 7m left".
func (p Progress) String() string {
	parts := []string{mib(p.Done)}
	if p.Total > 0 {
		parts[0] = fmt.Sprintf("%s of %s (%d%%)", mib(p.Done), mib(p.Total), int(p.Fraction()*100))
	}
	if p.Rate > 0 {
		parts = append(parts, fmt.Sprintf("%.1f MiB/s", p.Rate/(1<<20)))
		if p.Total > p.Done {
			left := time.Duration(float64(p.Total-p.Done) / p.Rate * float64(time.Second))
			parts = append(parts, "about "+roundLeft(left)+" left")
		}
	}
	return p.Label + ": " + strings.Join(parts, ", ")
}

func mib(n int64) string { return fmt.Sprintf("%d MiB", n>>20) }

// roundLeft rounds a time left to what's worth reading: seconds under a
// minute, then minutes.
func roundLeft(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(math.Ceil(d.Seconds())))
	}
	return fmt.Sprintf("%dm", int(math.Ceil(d.Minutes())))
}

// bar draws a progress bar width cells wide.
func bar(fraction float64, width int) string {
	full := int(math.Round(fraction * float64(width)))
	return strings.Repeat("█", full) + strings.Repeat("░", width-full)
}

// PlainProgressEvery is how often plain mode prints a step's progress.
var PlainProgressEvery = 30 * time.Second

// SpinProgress is Spin with a progress line under the spinner, read from
// progress while fn runs; progress returns false when there's nothing to
// show. In plain mode the progress is printed now and then instead.
func (u *UI) SpinProgress(ctx context.Context, title string, progress func() (Progress, bool), fn func(context.Context) error) error {
	if u.Quiet {
		return fn(ctx)
	}
	if !u.Interactive {
		u.Info("%s", title)
		return u.plainProgress(ctx, progress, fn)
	}
	m := &spinModel{
		spin:     spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(u.style().Foreground(Leaf))),
		title:    title,
		progress: progress,
		action:   func() error { return fn(ctx) },
		u:        u,
	}
	final, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithOutput(u.Err), tea.WithInput(nil)).Run()
	if fm, ok := final.(*spinModel); ok && fm.err != nil {
		return fm.err
	}
	return err
}

// plainProgress runs fn, printing its progress every PlainProgressEvery.
func (u *UI) plainProgress(ctx context.Context, progress func() (Progress, bool), fn func(context.Context) error) error {
	if progress == nil {
		return fn(ctx)
	}
	done := make(chan error, 1)
	go func() { done <- fn(ctx) }()
	tick := time.NewTicker(PlainProgressEvery)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-tick.C:
			if p, ok := progress(); ok {
				u.Hint(p.String())
			}
		}
	}
}

// spinModel is a spinner with a title and, when there's progress to show,
// a bar under it.
type spinModel struct {
	spin     spinner.Model
	title    string
	progress func() (Progress, bool)
	action   func() error
	u        *UI

	cur  Progress
	have bool
	err  error
}

type (
	spinDone     struct{ err error }
	progressTick struct{}
)

// progressEvery is how often the progress line is read.
const progressEvery = 500 * time.Millisecond

func (m *spinModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spin.Tick, func() tea.Msg { return spinDone{m.action()} }}
	if m.progress != nil {
		cmds = append(cmds, func() tea.Msg { return progressTick{} })
	}
	return tea.Batch(cmds...)
}

func (m *spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinDone:
		m.err = msg.err
		m.have = false // leave no bar behind
		return m, tea.Quit
	case progressTick:
		m.cur, m.have = m.progress()
		return m, tea.Tick(progressEvery, func(time.Time) tea.Msg { return progressTick{} })
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Interrupt
		}
	}
	var cmd tea.Cmd
	m.spin, cmd = m.spin.Update(msg)
	return m, cmd
}

func (m *spinModel) View() string {
	v := m.spin.View() + " " + m.title
	if !m.have {
		return v
	}
	line := m.cur.String()
	if f := m.cur.Fraction(); f >= 0 {
		line = m.u.color(Leaf, bar(f, 24)) + " " + line
	}
	return v + "\n  " + line
}
