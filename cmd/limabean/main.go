// Command limabean creates and manages disposable Linux dev VMs on macOS.
// See https://github.com/rossedman/limabean.
package main

import (
	"os"
	"runtime/debug"

	"github.com/rossedman/limabean"
	"github.com/rossedman/limabean/internal/cli"
)

// version is set at build time: -ldflags "-X main.version=1.2.3".
var version = ""

func main() {
	os.Exit(cli.Execute(resolveVersion(), limabean.Templates))
}

// resolveVersion falls back to the module version for `go install` builds.
func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
