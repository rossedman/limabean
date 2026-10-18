package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/limabean/internal/atomic"
	"github.com/rossedman/limabean/internal/project"
	"github.com/rossedman/limabean/recipes"
)

func newRecipesCmd(a *App) *cobra.Command {
	var toStdout, force bool
	cmd := &cobra.Command{
		Use:   "recipes [name]",
		Short: "List starter .limabean.yaml files, or write one into a repository",
		Long: `Recipes are starter project files for common kinds of VMs: a Go service, a
web frontend, a Rails app, a Kubernetes workspace, a sandbox for Claude Code
and more. Each is a .limabean.yaml with comments, ready to edit.

Without a name, recipes lists them. With one, it writes that recipe to
.limabean.yaml in the current directory, so "limabean up" can use it; it
won't replace a project file that's already there unless you pass --overwrite.
--print writes it to standard output instead.

See what a recipe asks for before you use it: the file's keys are described
in "limabean up --help".`,
		Example: `  limabean recipes

  # Start a Go service's project file, then bring the VM up
  cd ~/src/api && limabean recipes go-service && limabean up

  # Read one first
  limabean recipes claude-agent --print`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeRecipes,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return a.listRecipes()
			}
			body, err := recipe(args[0])
			if err != nil {
				return err
			}
			if toStdout {
				_, err := a.UI.Out.Write(body)
				return err
			}
			return a.writeRecipe(args[0], body, force)
		},
	}
	cmd.Flags().BoolVar(&toStdout, "print", false, "write the recipe to standard output")
	cmd.Flags().BoolVar(&force, "overwrite", false, "replace a project file that's already here")
	hiddenAlias(cmd.Flags(), &force, "force", "f", "overwrite")
	return cmd
}

// recipeNames lists the recipes, sorted.
func recipeNames() []string {
	files, _ := fs.Glob(recipes.Files, "*.yaml")
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, strings.TrimSuffix(f, ".yaml"))
	}
	sort.Strings(names)
	return names
}

// recipe returns a recipe's file.
func recipe(name string) ([]byte, error) {
	body, err := fs.ReadFile(recipes.Files, name+".yaml")
	if err != nil {
		return nil, notFoundErr("no recipe %q; there are: %s", name, strings.Join(recipeNames(), ", "))
	}
	return body, nil
}

// recipeDescription is a recipe's first comment line.
func recipeDescription(body []byte) string {
	line, _, _ := strings.Cut(string(body), "\n")
	return strings.TrimSpace(strings.TrimPrefix(line, "#"))
}

func (a *App) listRecipes() error {
	rows := [][]string{}
	for _, name := range recipeNames() {
		body, _ := recipe(name)
		rows = append(rows, []string{name, recipeDescription(body)})
	}
	a.UI.Table([]string{"RECIPE", "FOR"}, rows)
	a.UI.Hint("limabean recipes <name>   writes it to .limabean.yaml here")
	return nil
}

// writeRecipe writes a recipe to the current directory's project file,
// replacing the one there (.yaml or .yml) only with force.
func (a *App) writeRecipe(name string, body []byte, force bool) error {
	target := project.FileName
	for _, f := range project.FileNames {
		if _, err := os.Stat(f); err == nil {
			if !force {
				return conflictErr("%s is already here; --overwrite replaces it, or --print shows the recipe", f)
			}
			target = f
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := atomic.WriteFile(target, body, 0o644); err != nil {
		return err
	}
	wd, _ := os.Getwd()
	a.UI.OK("Wrote %s from the %s recipe", a.UI.Code(filepath.Join(shortPath(wd), target)), name)
	a.UI.Hint("Edit it for your project, then: limabean up")
	return nil
}

func completeRecipes(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return recipeNames(), cobra.ShellCompDirectiveNoFileComp
}
