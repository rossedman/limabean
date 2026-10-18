# Recipes

Recipes are starter [`.limabean.yaml`](projects.md) files for common kinds
of VMs. Pick the closest one, write it into your repository and edit it:

```bash
limabean recipes                         # list them
cd ~/src/api
limabean recipes go-service              # writes .limabean.yaml here
limabean up
```

`limabean recipes <name> --print` shows one without writing it, and
`--force` replaces a project file that's already there. Every recipe is a
plain project file: change the packs, size, ports and setup to fit, and
commit it so everyone on the project gets the same VM.

The recipes live in
[`recipes/`](https://github.com/rossedman/limabean/tree/main/recipes), and
limabean's tests bring every one of them up, so their packs, presets and
plugins are always real.

| Recipe | For |
|---|---|
| [`go-service`](#go-service) | A Go service with Postgres in Docker. |
| [`grpc`](#grpc) | A gRPC service in Go, with buf and grpcurl. |
| [`node-web`](#node-web) | A web frontend: Node, pnpm and a dev server (Vite, Next.js and the like). |
| [`python-api`](#python-api) | A Python web API (FastAPI, Django, Flask) managed with uv. |
| [`python-data`](#python-data) | Data work in Python: notebooks and libraries that want memory. |
| [`rust`](#rust) | A Rust crate or CLI. |
| [`java-spring`](#java-spring) | A Java service: Spring Boot, Quarkus or plain Gradle or Maven. |
| [`rails`](#rails) | A Ruby on Rails app with Postgres in Docker. |
| [`kubernetes`](#kubernetes) | Kubernetes work: a kind cluster in Docker, with kubectl, helm and k9s. |
| [`terraform`](#terraform) | Infrastructure with Terraform and the AWS CLI. |
| [`claude-agent`](#claude-agent) | A locked-down sandbox for Claude Code agents. |
| [`review`](#review) | Reviewing code you don't trust yet, with scanners at hand. |
| [`minimal`](#minimal) | The smallest VM: core shell tools, plus the Python and Docker every VM has. |

## Languages and frameworks

### go-service

For an HTTP or worker service in Go. Docker runs its Postgres (or anything else in `compose.yaml`) next to it, and the default packs bring golangci-lint, gofumpt and `psql`.

```yaml
--8<-- "recipes/go-service.yaml"
```

### grpc

For a gRPC service in Go: the `api` pack adds `buf` and `grpcurl`, and both the HTTP and gRPC ports are listed.

```yaml
--8<-- "recipes/grpc.yaml"
```

### node-web

For a frontend with a dev server. Setup picks pnpm, bun or npm from the lockfile, and Claude gets Anthropic's `frontend-design` plugin.

```yaml
--8<-- "recipes/node-web.yaml"
```

### python-api

For a FastAPI, Django or Flask app whose dependencies uv manages.

```yaml
--8<-- "recipes/python-api.yaml"
```

### python-data

For notebooks and data libraries: more CPUs and memory, and Jupyter's port. Start Jupyter with `uv run jupyter lab --ip 0.0.0.0`, then `limabean open`.

```yaml
--8<-- "recipes/python-data.yaml"
```

### rust

For a crate or CLI. More CPUs for parallel builds and a bigger disk for `target/`.

```yaml
--8<-- "recipes/rust.yaml"
```

### java-spring

For a JVM service. Setup warms Gradle's or Maven's cache through the project's wrapper.

```yaml
--8<-- "recipes/java-spring.yaml"
```

### rails

For a Rails app: Ruby, Node for the asset pipeline, and Postgres in Docker.

```yaml
--8<-- "recipes/rails.yaml"
```

## Infrastructure

### kubernetes

For platform work against a local cluster. Setup creates a kind cluster named `dev` once; kubectl, helm and k9s are ready for it.

```yaml
--8<-- "recipes/kubernetes.yaml"
```

### terraform

For infrastructure code. Your Mac's `~/.aws` is mounted read-only, so profiles and SSO sessions from the Mac work; run `aws sso login` on the Mac.

```yaml
--8<-- "recipes/terraform.yaml"
```

## Claude and untrusted code

### claude-agent

For handing a repository to Claude Code. The network reaches only GitHub, the package registries and Anthropic, the VM's user has no sudo or Docker, and Claude gets the `commit-commands` and `pr-review-toolkit` plugins. Run it headless with `limabean ssh <name> 'cd ~/code/... && claude -p "..."'`, or open it from [Claude Desktop](claude-desktop.md).

```yaml
--8<-- "recipes/claude-agent.yaml"
```

### review

For reading a repository you don't trust yet. No secrets at all, a network that reaches only GitHub and Anthropic, no sudo or Docker, and syft, grype and cosign to check its dependencies. Claude gets the `security-guidance` and `code-review` plugins.

```yaml
--8<-- "recipes/review.yaml"
```

## Starting from nothing

### minimal

For when you'd rather add packs as you go: `limabean rebuild <name> --with go`.

```yaml
--8<-- "recipes/minimal.yaml"
```

## Make your own

A recipe is only a starting point; the [reference](projects.md#reference)
lists every key. Some changes that come up often:

- **Another language next to the main one**: add its pack, such as
  `packs: [rust]`, or drop one you don't need with `without`.
- **A sandbox**: add `network.allow` with the [presets](network.md) setup
  needs, and `locked: true`. If you don't know the list, find it with
  [learn mode](network.md#learn-what-a-vm-needs).
- **Secrets**: name them under `secrets`, and set each up once on your Mac
  with `limabean secrets set NAME --vm <name>` ([secrets](secrets.md)).
- **Claude plugins**: add a marketplace and its plugins under
  [`claude`](projects.md#claude-plugins).

Have one that others would use? Recipes are plain files in
[`recipes/`](https://github.com/rossedman/limabean/tree/main/recipes): open
a pull request with yours.
