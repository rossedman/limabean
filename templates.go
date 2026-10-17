// Package limabean embeds the VM templates, so the limabean binary works on
// its own. See internal/templates for how they're written out for Lima.
package limabean

import "embed"

// Templates is the templates/ directory: Lima templates, provisioning
// scripts and the files they install.
//
//go:embed all:templates
var Templates embed.FS
