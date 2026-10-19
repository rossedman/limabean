package project

import (
	"slices"
	"strings"
)

// SchemaURL is where the docs site publishes Schema, for editors.
const SchemaURL = "https://rossedman.io/limabean/limabean.schema.json"

// SchemaComment is the first line limabean writes into a project file, so
// editors with the YAML language server (VS Code, Cursor, Zed, Neovim)
// complete and check its keys.
const SchemaComment = "# yaml-language-server: $schema=" + SchemaURL + "\n"

// Schema is the JSON Schema of a project file. Its patterns are the ones
// Load checks with; packs and presets are what editors offer, and other
// names still pass, since a Mac can have packs of its own. domain is the
// pattern a network.allow domain must match.
func Schema(packs, presets []string, domain string) map[string]any {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	pattern := func(desc, re string) map[string]any {
		s := str(desc)
		s["pattern"] = re
		return s
	}
	list := func(desc string, items map[string]any) map[string]any {
		return map[string]any{"type": "array", "description": desc, "items": items, "uniqueItems": true}
	}
	size := func(desc string) map[string]any {
		return map[string]any{"type": "integer", "minimum": 0, "description": desc}
	}
	var reserved []string
	for name := range reservedEnv {
		reserved = append(reserved, name)
	}
	slices.Sort(reserved)

	return map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"$id":                  SchemaURL,
		"title":                ".limabean.yaml",
		"description":          "The VM a repository needs, for limabean up. Every key is optional. https://rossedman.github.io/limabean/projects/",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"name": pattern("The VM's name: letters, digits and dashes. Default: the directory's name.", nameRE.String()),
			"env":  pattern("The environment template. Default: scratch.", nameRE.String()),
			"repo": str("What to clone: owner/repo, host/owner/repo or a URL. Default: the checkout's origin."),
			"packs": list("The VM's packs besides core (limabean packs lists them). The PACKS setting doesn't apply.", map[string]any{
				"anyOf": []any{
					map[string]any{"enum": packs},
					map[string]any{"type": "string", "pattern": nameRE.String()},
				},
			}),
			"without": map[string]any{
				"type": "array", "items": map[string]any{"type": "string"}, "deprecated": true,
				"description": "No longer used: a project's VM gets only core and the packs it lists.",
			},
			"tools": map[string]any{
				"type":          "object",
				"description":   "The project's own mise tools, tool: version, for what no pack has. Any mise backend: aqua:, npm:, pipx:, cargo:, go:. Changing them rebuilds the VM, keeping ~/code.",
				"maxProperties": 100,
				"propertyNames": map[string]any{"pattern": toolRE.String(), "maxLength": 200},
				"additionalProperties": map[string]any{
					"type": []string{"string", "number"}, "pattern": versionRE.String(), "maxLength": 100,
					"description": "An exact version, like 1.9.8.",
				},
			},
			"apt": map[string]any{
				"type": "array", "uniqueItems": true, "maxItems": 100,
				"description": "The project's own Ubuntu packages. Changing them rebuilds the VM, keeping ~/code.",
				"items":       map[string]any{"type": "string", "pattern": aptRE.String(), "maxLength": 100},
			},
			"cpus":   size("CPUs. Default: the CPUS setting, or 2."),
			"memory": size("GiB of memory. Default: the MEMORY setting, or 4."),
			"disk":   size("GiB of system disk. Default: the DISK setting, or 10. Lima can grow a disk but not shrink one."),
			"network": map[string]any{
				"type": "object", "additionalProperties": false,
				"description": "What the VM may reach. A file can tighten a VM's network, never widen it.",
				"properties": map[string]any{
					"allow": list("Domains and presets the VM may reach; others are blocked.", map[string]any{
						"anyOf": []any{
							map[string]any{"enum": presets},
							map[string]any{"type": "string", "pattern": `^(\*\.)?` + strings.TrimPrefix(domain, "^")},
						},
					}),
					"locked": map[string]any{"type": "boolean", "description": "Take sudo and Docker away from the VM's user, so the allowlist holds."},
				},
			},
			"secrets": list("Names of secrets the VM needs, besides the shared ones. Values stay on your Mac: limabean secrets set NAME.",
				map[string]any{"type": "string", "pattern": secretRE.String()}),
			"ports": list("Ports the project serves, shown as http://localhost:<port> after up.",
				map[string]any{"type": "integer", "minimum": 1, "maximum": 65535}),
			"setup": str("Shell commands run once in the clone, as you, with bash -e, after the VM is created."),
			"start": str("Shell commands run in the clone, in the background, after every up. A change needs approving."),
			"vars": map[string]any{
				"type":          "object",
				"description":   "Plain variables for the VM's shells, setup and start. Anything secret goes under secrets.",
				"maxProperties": 200,
				"propertyNames": map[string]any{
					"pattern": secretRE.String(),
					"not": map[string]any{"anyOf": []any{
						map[string]any{"enum": reserved},
						map[string]any{"pattern": "^(LD|DYLD)_"},
					}},
				},
				"additionalProperties": map[string]any{
					"type": []string{"string", "number", "boolean"}, "maxLength": 4096, "pattern": `^[^\n\r]*$`,
				},
			},
			"claude": map[string]any{
				"type": "object", "additionalProperties": false,
				"description": "Plugins for Claude Code in the VM, fetched inside it.",
				"properties": map[string]any{
					"marketplaces": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object", "additionalProperties": false,
							"required": []string{"marketplace", "plugins"},
							"properties": map[string]any{
								"marketplace": map[string]any{
									"type":        "string",
									"description": "A GitHub owner/repo, or an https git URL.",
									"anyOf": []any{
										map[string]any{"pattern": githubRE.String()},
										map[string]any{"pattern": gitURLRE.String()},
									},
								},
								"plugins": map[string]any{
									"type": "array", "minItems": 1, "uniqueItems": true,
									"description": "Plugins to turn on from this marketplace, as plugin@marketplace.",
									"items":       map[string]any{"type": "string", "pattern": pluginRE.String()},
								},
							},
						},
					},
				},
			},
		},
	}
}
