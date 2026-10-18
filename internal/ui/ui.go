// Package ui is limabean's terminal interface: status lines, spinners, boxes
// and tables in the README palette, and huh forms for prompts.
//
// When output isn't a terminal, or plain mode is on, everything is printed
// as plain text and nothing prompts: prompts return their defaults, and the
// caller decides whether a missing answer is an error.
//
// Data (tables, JSON) goes to Out, stdout; everything else (status lines,
// hints, spinners, forms) goes to Err, stderr, so a pipe gets only data.
package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// The palette from the README graphics.
var (
	Leaf  = lipgloss.Color("#8fd16a")
	Leaf2 = lipgloss.Color("#b9e69a")
	Dim   = lipgloss.Color("#6f8a77")
	Red   = lipgloss.Color("#ef7d7d")
	Amber = lipgloss.Color("#f0c36a")
	Ink   = lipgloss.Color("#0d1912")
)

// ErrAborted means the user cancelled a prompt (ctrl-c or esc).
var ErrAborted = errors.New("cancelled")

// UI writes to a terminal, or to plain output.
type UI struct {
	Out, Err io.Writer
	In       io.Reader
	// Interactive: output is a terminal and plain mode is off.
	Interactive bool
	// Prompting: Interactive, and input is a terminal too.
	Prompting bool
	// NoColor keeps the layout but drops color (NO_COLOR, --no-color).
	NoColor bool
	// Quiet drops everything but errors from Err; Verbose adds detail.
	Quiet, Verbose bool
	// FormTimeout, when set, fails a form that isn't finished in time
	// (tests use it so a wrong key sequence fails instead of hanging).
	FormTimeout time.Duration
	// forms, when set, gives each form its own input, in order (tests).
	forms []io.Reader

	r *lipgloss.Renderer
}

// New returns a UI for stdout, stderr and stdin. plain forces plain output,
// as does TERM=dumb; NO_COLOR (or noColor) only drops color.
func New(plain, noColor bool) *UI {
	plain = plain || os.Getenv("TERM") == "dumb"
	interactive := !plain && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
	u := &UI{
		Out: os.Stdout, Err: os.Stderr, In: os.Stdin,
		Interactive: interactive,
		Prompting:   interactive && term.IsTerminal(int(os.Stdin.Fd())),
		NoColor:     noColor || os.Getenv("NO_COLOR") != "",
	}
	u.r = lipgloss.NewRenderer(os.Stderr)
	if u.NoColor {
		u.r.SetColorProfile(termenv.Ascii)
	}
	return u
}

// Report returns a UI whose status lines go to stdout: for commands whose
// human-readable output is the result itself (ps, doctor, status).
func (u *UI) Report() *UI {
	r := *u
	r.Err = u.Out
	r.Quiet = false
	return &r
}

// NewPlain returns a plain UI writing to out, for tests.
func NewPlain(out io.Writer) *UI {
	return &UI{Out: out, Err: out, In: strings.NewReader(""), r: lipgloss.NewRenderer(io.Discard)}
}

// NewPlainStreams returns a plain UI with separate stdout and stderr, for
// tests that check what goes where.
func NewPlainStreams(out, err io.Writer) *UI {
	return &UI{Out: out, Err: err, In: strings.NewReader(""), r: lipgloss.NewRenderer(io.Discard)}
}

// NewTerminal returns an interactive UI reading keys from in and writing to
// out, as if both were a terminal; for driving forms in tests (see Keys).
func NewTerminal(in io.Reader, out io.Writer) *UI {
	return &UI{Out: out, Err: out, In: in, Interactive: true, Prompting: true, FormTimeout: 10 * time.Second, r: lipgloss.NewRenderer(out)}
}

// NewTerminalForms is NewTerminal with one input per form, in the order the
// forms appear. A command that shows several forms in a row needs this: each
// form's reader may read ahead, and keys it took are lost when it closes.
func NewTerminalForms(out io.Writer, forms ...io.Reader) *UI {
	u := NewTerminal(strings.NewReader(""), out)
	u.forms = forms
	return u
}

func (u *UI) style() lipgloss.Style { return u.r.NewStyle() }

func (u *UI) color(c lipgloss.Color, s string) string {
	if !u.Interactive || u.NoColor {
		return s
	}
	return u.style().Foreground(c).Render(s)
}

// Code highlights an inline name or command.
func (u *UI) Code(s string) string { return u.color(Leaf2, s) }

// Dimmed renders secondary text.
func (u *UI) Dimmed(s string) string { return u.color(Dim, s) }

// OK, Bad and Warn print a status line marked ✓, ✗ or !.
func (u *UI) OK(format string, a ...any)   { u.mark("✓", Leaf, format, a...) }
func (u *UI) Bad(format string, a ...any)  { u.mark("✗", Red, format, a...) }
func (u *UI) Warn(format string, a ...any) { u.mark("!", Amber, format, a...) }

func (u *UI) mark(sym string, c lipgloss.Color, format string, a ...any) {
	if u.Quiet && sym != "✗" {
		return
	}
	fmt.Fprintf(u.Err, "  %s %s\n", u.color(c, sym), fmt.Sprintf(format, a...))
}

// Hint prints indented, dimmed lines under a status line.
func (u *UI) Hint(lines ...string) {
	if u.Quiet {
		return
	}
	for _, l := range lines {
		fmt.Fprintf(u.Err, "    %s\n", u.Dimmed(l))
	}
}

// Info prints a progress line.
func (u *UI) Info(format string, a ...any) {
	if u.Quiet {
		return
	}
	fmt.Fprintf(u.Err, "%s %s\n", u.color(Leaf, "›"), fmt.Sprintf(format, a...))
}

// Detail prints a line only with --verbose.
func (u *UI) Detail(format string, a ...any) {
	if u.Verbose && !u.Quiet {
		fmt.Fprintf(u.Err, "    %s\n", u.Dimmed(fmt.Sprintf(format, a...)))
	}
}

// Section starts a group of status lines, with an optional subtitle.
func (u *UI) Section(title, sub string) {
	switch {
	case u.Quiet:
	case sub == "":
		fmt.Fprintf(u.Err, "\n%s\n", u.title(title))
	case u.Interactive:
		fmt.Fprintf(u.Err, "\n%s  %s\n", u.title(title), u.Dimmed(sub))
	default:
		fmt.Fprintf(u.Err, "\n%s (%s)\n", title, sub)
	}
}

func (u *UI) title(s string) string {
	if !u.Interactive || u.NoColor {
		return s
	}
	return u.style().Bold(true).Foreground(Leaf2).Render(s)
}

// Box prints lines in a rounded box (plain: just the lines).
func (u *UI) Box(lines ...string) {
	switch {
	case u.Quiet:
	case !u.Interactive:
		fmt.Fprintf(u.Err, "\n%s\n", strings.Join(lines, "\n"))
	default:
		box := u.style().Border(lipgloss.RoundedBorder()).BorderForeground(Leaf).Padding(0, 2).MarginTop(1)
		fmt.Fprintln(u.Err, box.Render(strings.Join(lines, "\n")))
	}
}

// Table prints rows under headers: a bordered table in a terminal, aligned
// columns otherwise.
func (u *UI) Table(headers []string, rows [][]string) {
	if !u.Interactive {
		w := tabwriter.NewWriter(u.Out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, strings.Join(headers, "\t"))
		for _, r := range rows {
			fmt.Fprintln(w, strings.Join(r, "\t"))
		}
		w.Flush()
		return
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(u.style().Foreground(Dim)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			s := u.style().Padding(0, 1)
			if row == table.HeaderRow {
				return s.Foreground(Leaf).Bold(true)
			}
			return s
		})
	fmt.Fprintln(u.Out, t.Render())
}

// Spin runs fn behind a spinner titled title. In plain mode it prints the
// title and runs fn.
func (u *UI) Spin(ctx context.Context, title string, fn func(context.Context) error) error {
	if u.Quiet {
		return fn(ctx)
	}
	if !u.Interactive {
		u.Info("%s", title)
		return fn(ctx)
	}
	var fnErr error
	err := spinner.New().
		Context(ctx).
		Type(spinner.Dots).
		Style(u.style().Foreground(Leaf)).
		Title(" " + title).
		Output(u.Err).
		ActionWithErr(func(ctx context.Context) error { fnErr = fn(ctx); return nil }).
		Run()
	if err != nil {
		return err
	}
	return fnErr
}

// Theme is the huh theme in the README palette.
func Theme() *huh.Theme {
	t := huh.ThemeBase()
	t.Focused.Title = t.Focused.Title.Foreground(Leaf2).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(Dim)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(Leaf)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(Leaf)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(Leaf)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(Leaf)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(Dim)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Background(Leaf).Foreground(Ink)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Background(lipgloss.Color("#1b3224"))
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(Leaf)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(Leaf)
	t.Focused.Base = t.Focused.Base.BorderForeground(Leaf)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(Red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(Red)
	return t
}

// Run shows a huh form (only call it when Prompting).
func (u *UI) Run(groups ...*huh.Group) error {
	in := u.In
	if len(u.forms) > 0 {
		in, u.forms = u.forms[0], u.forms[1:]
	}
	form := huh.NewForm(groups...).WithTheme(Theme()).WithShowHelp(true).WithInput(in).WithOutput(u.Err)
	if u.FormTimeout > 0 {
		form = form.WithTimeout(u.FormTimeout)
	}
	err := form.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrAborted
	}
	return err
}

// Confirm asks a yes/no question; without a terminal it returns def.
func (u *UI) Confirm(question string, def bool) (bool, error) {
	if !u.Prompting {
		return def, nil
	}
	v := def
	err := u.Run(huh.NewGroup(huh.NewConfirm().Title(question).Affirmative("Yes").Negative("No").Value(&v)))
	return v, err
}

// Input asks for a line of text; without a terminal it returns def.
func (u *UI) Input(title, def, placeholder string, validate func(string) error) (string, error) {
	if !u.Prompting {
		return def, nil
	}
	v := def
	in := huh.NewInput().Title(title).Placeholder(placeholder).Value(&v)
	if validate != nil {
		in = in.Validate(validate)
	}
	err := u.Run(huh.NewGroup(in))
	return strings.TrimSpace(v), err
}

// Password asks for a value without echoing it.
func (u *UI) Password(title, placeholder string) (string, error) {
	if !u.Prompting {
		return "", errors.New("no terminal to ask for a value on")
	}
	var v string
	err := u.Run(huh.NewGroup(huh.NewInput().Title(title).Placeholder(placeholder).EchoMode(huh.EchoModePassword).Value(&v)))
	return strings.TrimSpace(v), err
}

// Select asks for one of options.
func (u *UI) Select(title string, options []string) (string, error) {
	if !u.Prompting {
		return "", errors.New("no terminal to choose on")
	}
	var v string
	err := u.Run(huh.NewGroup(huh.NewSelect[string]().Title(title).Options(huh.NewOptions(options...)...).Value(&v)))
	return v, err
}
