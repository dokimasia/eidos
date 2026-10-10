# Managed by ergon init. Add repository settings to .ergon/local/Makefile and run ergon init sync.
#
# make check is the gate of the repository. Each language adds its own fmt, lint, test, generate,
# audit and check targets as prerequisites of the targets below, and CI runs make check-<language>
# per language. Every target runs its tools through ergon tool run, which installs the version that
# .ergon.yaml names. make help lists the targets in groups. A line ##@ <name> starts a group: the
# common targets, then the targets of each language, and the targets of .ergon/local/Makefile after
# a line of its own.

.DEFAULT_GOAL := check

# The command that runs ergon for the tools of the targets. The key ergon of the section common of
# .ergon.yaml sets it.
ERGON ?= ergon

# verify-generated runs the command of the variable $(1) and fails when the run changes a file of
# the repository, with the target $(2) that updates the files in its message. It compares the
# changes of the working tree before and after the run, so it also runs on a tree with uncommitted
# changes.
verify-generated = before="$$(git diff HEAD --binary 2>/dev/null; git ls-files --others --exclude-standard)"; \
	$($(1)) || exit 1; \
	after="$$(git diff HEAD --binary 2>/dev/null; git ls-files --others --exclude-standard)"; \
	if [ "$$before" != "$$after" ]; then \
	echo "$(2): the generated files are out of date, so run make $(2) and commit the result:" >&2; \
	git status --short >&2; exit 1; fi

.PHONY: help fmt lint test generate audit check

##@ Common

help: ## List the targets in groups
	@awk 'BEGIN {FS = ":.*## "} /^##@ / {group = substr($$0, 5); next} /^[a-z][a-z-]*:.*## / {if (group != shown) {printf "%s%s\n", (shown == "" ? "" : "\n"), group; shown = group} printf "  %-28s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format the sources of every language
lint: ## Lint the sources of every language
test: ## Run the tests of every language
generate: ## Run the generators of every language
audit: ## Run the vulnerability scan of every language
check: ## Run the gate of every language

##@ Go
#
# The Go targets run in every module that go list -m lists, which is every module of go.work. The
# section go of .ergon.yaml sets their tools and options, and the steps of check-go. One run
# overrides an option on the command line, such as make fuzz-go GO_FUZZ_MATCH=FuzzDecode.

# The go command.
GO ?= go

# The tools of the Go targets, which ergon installs at the versions of the section go.
GOLANGCI_LINT := $(ERGON) tool run go.golangci-lint --
GOVULNCHECK := $(ERGON) tool run go.govulncheck --
BENCHSTAT := $(ERGON) tool run go.benchstat --
DOKIMI_MUTATE_GO := $(ERGON) tool run go.dokimi-mutate-go --

# The options of the section go.
GO_PATHS ?= ./...
GO_TEST_ARGS ?= -count=1
GO_RACE_ARGS ?= -count=1 -p=1
GO_FUZZ_MATCH ?= .
GO_FUZZ_TIME ?= 30s
GO_FUZZ_ARGS ?= -fuzzminimizetime=5s
GO_BENCH_MATCH ?= .
GO_BENCH_TIME ?= 1s
GO_BENCH_COUNT ?= 6
GO_BENCH_ARGS ?= -benchmem
GO_MUTATE_WORKERS ?= 1
GO_MUTATE_TIMEOUT ?= 0s
GO_MUTATE_ARGS ?=
GO_GENERATE_COMMAND ?= go generate
GO_GENERATE_ARGS ?=
GO_AUDIT_ARGS ?=

# The files of two runs of bench-go that benchstat-go compares.
GO_BENCH_OLD ?=
GO_BENCH_NEW ?=

# The run of the generators in every module, which generate-go and verify-generate-go run.
GO_GENERATE = $(GO) list -m -f '{{.Dir}}' | while IFS= read -r dir; do echo "$(GO_GENERATE_COMMAND) $$dir"; \
	(cd "$$dir" && $(GO_GENERATE_COMMAND) $(GO_GENERATE_ARGS) $(GO_PATHS)) || exit 1; done

.PHONY: fmt-go lint-go test-go race-go fuzz-go bench-go benchstat-go mutate-go generate-go verify-generate-go audit-go check-go
fmt: fmt-go
lint: lint-go
test: test-go
generate: generate-go
audit: audit-go
check: check-go

fmt-go: ## Format the Go sources of every module with the formatters of .golangci.yml
	@$(GO) list -m -f '{{.Dir}}' | while IFS= read -r dir; do echo "golangci-lint fmt $$dir"; \
		(cd "$$dir" && $(GOLANGCI_LINT) fmt $(GO_PATHS)) || exit 1; done
lint-go: ## Lint every module with .golangci.yml, and check its format
	@$(GO) list -m -f '{{.Dir}}' | { status=0; while IFS= read -r dir; do echo "golangci-lint $$dir"; \
		(cd "$$dir" && $(GOLANGCI_LINT) run $(GO_PATHS) && $(GOLANGCI_LINT) fmt --diff $(GO_PATHS)) \
			|| status=1; done; exit $$status; }
test-go: ## Run the Go tests of every module
	@$(GO) list -m -f '{{.Dir}}' | { status=0; while IFS= read -r dir; do echo "go test $$dir"; \
		$(GO) -C "$$dir" test $(GO_TEST_ARGS) $(GO_PATHS) || status=1; done; exit $$status; }
race-go: ## Run the Go tests of every module under the race detector
	@$(GO) list -m -f '{{.Dir}}' | { status=0; while IFS= read -r dir; do echo "go test -race $$dir"; \
		$(GO) -C "$$dir" test -race $(GO_RACE_ARGS) $(GO_PATHS) || status=1; done; exit $$status; }
fuzz-go: ## Fuzz each fuzz target of every module whose name matches GO_FUZZ_MATCH, for GO_FUZZ_TIME each
	@$(GO) list -m -f '{{.Dir}}' | while IFS= read -r dir; do \
		list="$$($(GO) -C "$$dir" test -list '$(GO_FUZZ_MATCH)' $(GO_PATHS))" || { printf '%s\n' "$$list"; exit 1; }; \
		targets="$$(printf '%s\n' "$$list" | awk '/^Fuzz/ { name[n++] = $$1; next } /^ok/ { for (i = 0; i < n; i++) print $$2, name[i]; n = 0 }')"; \
		if [ -z "$$targets" ]; then echo "no fuzz target matches $(GO_FUZZ_MATCH) in $$dir"; continue; fi; \
		printf '%s\n' "$$targets" | while read -r pkg name; do echo "go test -fuzz $$name $$pkg"; \
			$(GO) -C "$$dir" test -run '^$$' -fuzz "^$$name\$$" -fuzztime $(GO_FUZZ_TIME) $(GO_FUZZ_ARGS) "$$pkg" || exit 1; \
		done || exit 1; \
	done
bench-go: ## Run the benchmarks of every module whose names match GO_BENCH_MATCH, in the format of benchstat
	@$(GO) list -m -f '{{.Dir}}' | while IFS= read -r dir; do echo "go test -bench $$dir"; \
		$(GO) -C "$$dir" test -run '^$$' -bench '$(GO_BENCH_MATCH)' -benchtime $(GO_BENCH_TIME) \
			-count $(GO_BENCH_COUNT) $(GO_BENCH_ARGS) $(GO_PATHS) || exit 1; done
benchstat-go: ## Compare the results of bench-go in GO_BENCH_OLD with those in GO_BENCH_NEW
	@if [ -z "$(GO_BENCH_OLD)" ] || [ -z "$(GO_BENCH_NEW)" ]; then \
		echo "benchstat-go: set GO_BENCH_OLD and GO_BENCH_NEW to the files of two runs of bench-go" >&2; exit 2; fi
	$(BENCHSTAT) $(GO_BENCH_OLD) $(GO_BENCH_NEW)
mutate-go: ## Run dokimi-mutate-go on every module, and fail on a mutant that the tests miss
	@$(GO) list -m -f '{{.Dir}}' | { status=0; while IFS= read -r dir; do echo "dokimi-mutate-go $$dir"; \
		$(DOKIMI_MUTATE_GO) -C "$$dir" -workers $(GO_MUTATE_WORKERS) -timeout $(GO_MUTATE_TIMEOUT) $(GO_MUTATE_ARGS) \
			$(GO_PATHS) || status=1; done; exit $$status; }
generate-go: ## Run the generators of every module, such as go generate
	@$(GO_GENERATE)
verify-generate-go: ## Fail when the generators of a module change a file of the repository
	@$(call verify-generated,GO_GENERATE,generate-go)
audit-go: ## Scan every Go module for known vulnerabilities that its code reaches
	@$(GO) list -m -f '{{.Dir}}' | { status=0; while IFS= read -r dir; do echo "govulncheck $$dir"; \
		$(GOVULNCHECK) -C "$$dir" $(GO_AUDIT_ARGS) $(GO_PATHS) || status=1; done; exit $$status; }
check-go: lint-go test-go race-go audit-go ## Run the gate of Go
