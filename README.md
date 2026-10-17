<p align="center">
  <img src="docs/assets/banner.png" alt="limabean: disposable Linux dev VMs for macOS, ready for SSH and Claude" width="100%">
</p>

<p align="center">
  <a href="https://github.com/rossedman/limabean/actions/workflows/lint.yml"><img src="https://github.com/rossedman/limabean/actions/workflows/lint.yml/badge.svg" alt="lint"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-8fd16a" alt="MIT license"></a>
  <img src="https://img.shields.io/badge/macOS-Lima%202.x-8fd16a" alt="macOS with Lima 2.x">
</p>

limabean creates disposable Ubuntu dev VMs on your Mac from layered
[Lima](https://lima-vm.io) templates. Each VM has a complete, version-pinned
toolchain and your git identity. It pushes with your SSH keys without ever
copying them in, and appears in Claude Desktop as an SSH connection.

- **One command per VM.** `limabean new dev` is ready in about a minute.
- **Batteries included.** Docker Engine, Go, Python, Kubernetes tooling,
  Terraform and 30+ CLI tools, pinned with mise.
- **Keys stay on your Mac.** SSH keys stay in your agent, a scoped GitHub
  token lives in your Keychain, and your git and AWS config are mounted
  read-only.
- **Secrets without files.** API keys and tokens come from your Keychain,
  1Password, environment variables or commands, chosen when you create a VM.
- **Built for Claude.** Every VM is registered as an SSH connection in Claude
  Desktop.
- **Safe to throw away.** `~/code` lives on its own disk, so `rebuild`
  keeps your repositories, and `rm` refuses to run while you have unpushed
  work.
- **Plain Lima underneath.** Templates are ordinary YAML and shell, and
  `limactl` always works.

## Quick start

```bash
brew install lima
go install github.com/rossedman/limabean/cmd/limabean@latest   # or download a release

limabean init       # save settings, fix ~/.ssh/config, check your setup

# optional: log VMs into GitHub for gh and the API (scoped tokens: docs/github.md)
gh auth token --hostname github.com | limabean github login --with-token

limabean new dev    # create a VM; a form asks which secrets it gets
limabean ssh dev    # or: ssh lima-dev
```

Prebuilt binaries for Apple Silicon and Intel are on the
[releases page](https://github.com/rossedman/limabean/releases).

<p align="center">
  <img src="docs/assets/terminal.png" alt="Creating a VM with limabean new, listing it, then running docker inside it" width="100%">
</p>

## How it works

<p align="center">
  <img src="docs/assets/how-it-works.png" alt="Your Mac connects to the Lima VM over ssh lima-dev, forwards its ssh-agent and mounts the git config read-only. The VM is built from an OS layer, a base layer and an environment." width="100%">
</p>

## Commands

Leave out a VM name and you pick one from a list. Every command has
`--help` with examples, and the [CLI reference](docs/cli.md) has the details.

**Setup**

| Command | What it does |
|---|---|
| `limabean init` | Set up: git config, code directory, SSH `Include`, GitHub token |
| `limabean doctor` | Check everything VMs depend on |
| `limabean bump` | Update the pinned tool versions in `templates/mise/*.toml` |

**VMs**

| Command | What it does |
|---|---|
| `limabean new [name] [env]` | Create a VM, choosing its secrets |
| `limabean ls` | List VMs, or print them as JSON with `--json` |
| `limabean ssh [name] [cmd]` | Open a shell or run a command, starting the VM if needed |
| `limabean get [name] <repo>` | Clone a repository into the VM with ghq |
| `limabean code [name] [repo]` | Open the VM, or a repository in it, in VS Code or Cursor |
| `limabean ports [name]` | List the ports a VM forwards to your Mac |
| `limabean open [name] [port]` | Open a forwarded port in your browser |
| `limabean start [name]` | Start a stopped VM |
| `limabean stop [name]` | Stop a running VM, or with `--idle 2h` every VM nobody is using |
| `limabean rebuild [name]` | Recreate a VM from the current templates, keeping `~/code` |
| `limabean rm [name]` | Delete a VM |
| `limabean snapshot save\|ls\|restore\|rm` | Save a VM, `~/code` included, and roll back to it later |
| `limabean autostop on\|off\|status` | Stop idle VMs automatically in the background |

**Secrets and access**

| Command | What it does |
|---|---|
| `limabean secrets ls` | List secrets and where their values come from |
| `limabean secrets set NAME [ref]` | Add or change a secret (Keychain, 1Password, env var, command) |
| `limabean secrets rm NAME` | Remove a secret |
| `limabean secrets edit` | Edit a secrets file in your editor |
| `limabean secrets sync` | Install secrets in running VMs |
| `limabean github login` | Save a GitHub token for `gh`, the API and https pushes |
| `limabean github status` | Show the token's account and expiry, and which VMs have it |
| `limabean github sync` | Install the token in every running VM |
| `limabean github logout` | Remove the token from the Keychain and VMs |
| `limabean claude sync` | Register running VMs in Claude Desktop, remove the rest |
| `limabean mcp` | Let Claude create, run and snapshot VMs, as an MCP server ([details](docs/claude-desktop.md#let-claude-manage-vms)) |

**More**

| Command | What it does |
|---|---|
| `limabean completion <shell>` | Print a completion script for bash, zsh, fish or PowerShell |
| `limabean version` | Print the version |

## Documentation

- [Getting started](docs/getting-started.md): install, requirements, first VM
- [CLI reference](docs/cli.md): commands, settings, unsaved-work protection
- [Inside a VM](docs/inside-a-vm.md): tools, Docker, shell, paths, updates
- [Secrets](docs/secrets.md): API keys and settings from the Keychain, 1Password and more
- [GitHub access](docs/github.md): a scoped token for `gh`, kept in your Keychain
- [Claude Desktop](docs/claude-desktop.md): how VMs become SSH connections
- [Customizing](docs/customizing.md): templates, adding tools and environments
- [Troubleshooting](docs/troubleshooting.md)
- [Design notes](docs/design.md) and [the Go CLI's design](docs/go-cli.md)

## Contributing

Issues and pull requests are welcome. CI runs `go test`, `go vet`, gofmt,
`shellcheck` and `limactl tmpl validate`, and every pull request gets a
snapshot build to try; see [Customizing](docs/customizing.md#checks).

## License

[MIT](LICENSE)
