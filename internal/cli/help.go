package cli

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rossedman/limabean/internal/ui"
)

// Help and usage use the same palette as the rest of the CLI: headings like
// doctor's sections, command names in green, examples with dimmed comments.
// Colors only appear in a terminal; --plain, LIMABEAN_PLAIN and NO_COLOR
// turn them off.

const helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

const usageTemplate = `{{heading "Usage:"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{heading "Aliases:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{heading "Examples:"}}
{{examples .Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{heading "Commands:"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{command (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{heading .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{command (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{heading "More:"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{command (rpad .Name .NamePadding)}} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{heading "Flags:"}}
{{flags (.LocalFlags.FlagUsages | trimTrailingWhitespaces)}}{{end}}{{if .HasAvailableInheritedFlags}}

{{heading "Global flags:"}}
{{flags (.InheritedFlags.FlagUsages | trimTrailingWhitespaces)}}{{end}}{{if .HasAvailableSubCommands}}

{{dim (printf "Run \"%s [command] --help\" for more about a command." .CommandPath)}}{{end}}{{if not .HasParent}}

{{dim "Docs:   https://rossedman.io/limabean/"}}
{{dim "Issues: https://github.com/rossedman/limabean/issues"}}{{end}}
`

// setupHelp gives every command limabean's help and usage templates.
func setupHelp(root *cobra.Command, a *App) {
	r := lipgloss.NewRenderer(os.Stdout)
	color := func() bool {
		return !a.plain && os.Getenv("LIMABEAN_PLAIN") == "" && os.Getenv("NO_COLOR") == "" &&
			term.IsTerminal(int(os.Stdout.Fd()))
	}
	style := func(s lipgloss.Style, text string) string {
		if !color() {
			return text
		}
		return s.Render(text)
	}
	heading := r.NewStyle().Bold(true).Foreground(ui.Leaf2)
	name := r.NewStyle().Foreground(ui.Leaf)
	dim := r.NewStyle().Foreground(ui.Dim)
	code := r.NewStyle().Foreground(ui.Leaf2)

	cobra.AddTemplateFunc("heading", func(s string) string { return style(heading, s) })
	cobra.AddTemplateFunc("command", func(s string) string {
		// Padding comes first, so colour codes don't upset the alignment.
		trimmed := strings.TrimRight(s, " ")
		return style(name, trimmed) + s[len(trimmed):]
	})
	cobra.AddTemplateFunc("dim", func(s string) string { return style(dim, s) })
	cobra.AddTemplateFunc("examples", func(s string) string {
		lines := strings.Split(s, "\n")
		for i, l := range lines {
			t := strings.TrimSpace(l)
			indent := l[:len(l)-len(strings.TrimLeft(l, " "))]
			switch {
			case strings.HasPrefix(t, "#"):
				lines[i] = indent + style(dim, t)
			case t != "":
				lines[i] = indent + style(code, t)
			}
		}
		return strings.Join(lines, "\n")
	})
	cobra.AddTemplateFunc("flags", func(s string) string {
		return flagNameRE.ReplaceAllStringFunc(s, func(m string) string { return style(name, m) })
	})
	// List commands in the order they're added: the order you'd use them.
	cobra.EnableCommandSorting = false
	root.SetHelpTemplate(helpTemplate)
	root.SetUsageTemplate(usageTemplate)
}

// flagNameRE finds flag names at the start of each flag usage line, like
// "  -f, --force" or "      --config string".
var flagNameRE = regexp.MustCompile(`(?m)^\s+(-\w, )?--[\w-]+`)
