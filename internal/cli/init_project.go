package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/rossedman/limabean/internal/atomic"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/internal/templates"
)

// initProject writes a project file for the repository in dir (the
// current directory when ""), from what Detect finds there.
func (a *App) initProject(dir string, toStdout, overwrite bool) error {
	if dir == "" {
		dir = "."
	}
	wd, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(wd); err != nil || !fi.IsDir() {
		return notFoundErr("%s isn't a directory", wd)
	}
	target := filepath.Join(wd, project.FileName)
	for _, name := range project.FileNames {
		f := filepath.Join(wd, name)
		if _, err := os.Stat(f); err == nil {
			if !overwrite && !toStdout {
				return conflictErr("%s is already there; limabean up uses it, and --overwrite replaces it", shortPath(f))
			}
			target = f
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	root, err := a.templatesRoot()
	if err != nil {
		return err
	}
	packs, err := templates.Packs(root)
	if err != nil {
		return err
	}
	byTool := map[string]string{}
	for _, p := range packs {
		for _, tool := range p.Tools {
			if _, taken := byTool[tool]; !taken || p.Name == templates.CorePack {
				byTool[tool] = p.Name
			}
		}
	}
	d, err := project.Detect(wd, func(tool string) string { return byTool[tool] })
	if err != nil {
		return err
	}
	body := d.Render()
	// The draft must be a file up accepts; anything else is limabean's bug.
	if _, err := project.Parse([]byte(body), target); err != nil {
		return fmt.Errorf("the project file worked out for this repository doesn't load (please report it): %w", err)
	}
	if toStdout {
		_, err := fmt.Fprint(a.UI.Out, body)
		return err
	}

	if d.Empty() {
		a.UI.Warn("Nothing here says what the VM needs")
		a.UI.Hint("A recipe may fit better: limabean recipes")
	} else {
		a.UI.Section("Found", shortPath(wd))
		a.UI.Hint(d.Found()...)
	}
	if a.UI.Prompting {
		a.UI.Section(filepath.Base(target), "")
		a.UI.Hint(strings.Split(strings.TrimRight(body, "\n"), "\n")...)
		ok, err := a.UI.Confirm("Write "+shortPath(target)+"?", true)
		if err != nil {
			return err
		}
		if !ok {
			a.UI.Info("Nothing written")
			return nil
		}
	}
	if err := atomic.WriteFile(target, []byte(body), 0o644); err != nil {
		return err
	}
	a.UI.OK("Wrote %s", a.UI.Code(shortPath(target)))
	if len(d.Secrets) > 0 {
		a.UI.Hint("Uncomment secrets: once they're set up: limabean secrets set NAME")
	}
	a.UI.Hint("Check it, then: limabean up")
	return nil
}
