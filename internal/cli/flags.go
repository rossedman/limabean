package cli

import "github.com/spf13/pflag"

// hiddenAlias adds a hidden bool flag that sets the same variable as target,
// so an old or familiar name keeps working without showing up in help.
func hiddenAlias(fs *pflag.FlagSet, p *bool, name, short, target string) {
	fs.BoolVarP(p, name, short, false, "same as --"+target)
	_ = fs.MarkHidden(name)
}
