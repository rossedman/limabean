# The checks CI runs, for running before you push: `make check`.
# CONTRIBUTING.md explains each one.

GOLANGCI_LINT ?= golangci-lint
MKDOCS ?= mkdocs
FUZZTIME ?= 30s
SITE ?= /tmp/limabean-site

.PHONY: build test cover bench fuzz lint docs docs-build check

build:
	go build -o limabean ./cmd/limabean

test:
	go test -race ./...

# Fails below 90% total coverage, as CI does.
cover:
	go test -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | awk '/^total:/ { sub("%", "", $$3); print "total coverage: " $$3 "%"; exit !($$3 >= 90) }'

# Saves results to bench.out; compare two runs with benchstat old.out bench.out.
bench:
	go test -run='^$$' -bench=. -benchmem -count=6 ./... | tee bench.out

# Each fuzz target for FUZZTIME (make fuzz FUZZTIME=5m for longer).
fuzz:
	@for pkg in $$(go list ./...); do \
		for target in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
			echo "== $$pkg $$target"; \
			go test -run='^$$' -fuzz="^$$target\$$" -fuzztime=$(FUZZTIME) -fuzzminimizetime=5s $$pkg || exit 1; \
		done; \
	done

lint:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...
	$(GOLANGCI_LINT) run ./...
	go run ./tools/doccheck ./...
	shellcheck templates/features/*.sh templates/files/limabean-network
	shellcheck -s sh templates/files/env.sh templates/files/limabean-github-git templates/files/limabean-info templates/files/limabean-statusline
	actionlint

# Regenerates docs/reference/commands from the commands' help.
docs:
	go run ./tools/gendocs

# Builds the site outside the repository, so site/ is never committed.
docs-build:
	$(MKDOCS) build --strict -d $(SITE)

check: lint cover docs docs-build
	go test -run='^$$' -bench=. -benchtime=1x ./...
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
	@git diff --exit-code docs/reference/commands || { echo "docs/reference/commands is stale: commit what make docs wrote"; exit 1; }
	@echo "all checks passed"
