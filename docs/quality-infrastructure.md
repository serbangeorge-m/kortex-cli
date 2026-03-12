# Quality Infrastructure

This document describes the testing and quality assurance infrastructure for kortex-cli.

## Overview

Quality is enforced at two levels:

1. **PR checks** — automated gates that block merging if quality standards are not met
2. **Local tooling** — Make targets for developers to run quality checks before pushing

```text
                     ┌─────────────────────────────────┐
                     │          Pull Request            │
                     └────────────┬────────────────────┘
                                  │
              ┌───────────────────┼───────────────────┐
              ▼                   ▼                    ▼
   ┌──────────────────┐ ┌─────────────────┐ ┌─────────────────┐
   │   CI Checks      │ │  Codecov Check  │ │ Container Test  │
   │ (Ubuntu, macOS,  │ │  (80% project,  │ │ (linux/amd64,   │
   │  Windows)        │ │   80% patch)    │ │  linux/arm64)   │
   └──────────────────┘ └─────────────────┘ └─────────────────┘
              │                   │                    │
              └───────────────────┼───────────────────┘
                                  ▼
                        All must pass to merge
```

---

## PR Checks (`pr-checks.yml`)

Triggered on every pull request. All jobs must pass before merging.

### CI Checks (matrix)

Runs on three operating systems in parallel:

| Runner | OS |
|--------|----|
| `ubuntu-24.04` | Linux (x86_64) |
| `macos-26` | macOS (ARM — Apple Silicon) |
| `windows-2025` | Windows (x86_64) |

**Steps on each runner:**

1. Checkout code
2. Set up Go (version from `go.mod`)
3. `make ci-checks` — runs format check (`gofmt -l`), `go vet`, and `go test -v -race ./...`
4. `make build` — compiles the binary
5. `./kortex-cli version` — verifies the binary executes

**Additional steps on Ubuntu only:**

6. `make test-coverage` — generates `coverage.out`
7. Uploads coverage to Codecov

### Codecov Coverage Gate

Configured in `codecov.yml` at the repository root. Codecov runs as a GitHub status check on every PR after coverage is uploaded.

**Thresholds:**

| Check | Target | Description |
|-------|--------|-------------|
| Project coverage | 80% (1% threshold) | Overall project coverage must stay above 80%. The 1% threshold means a PR passes if coverage drops by at most 1% — prevents flaky failures from test timing or rounding. |
| Patch coverage | 80% | New or changed lines in the PR must have at least 80% test coverage. Prevents merging untested code. |

**Ignored paths:**

- `scripts/**` — utility scripts, not production code
- `cmd/kortex-cli/main.go` — 3-line entry point that calls `NewRootCmd().Execute()` and `os.Exit(1)`, cannot be meaningfully unit tested

### Container Test (matrix)

Validates the binary builds and all tests pass inside a minimal container environment using Podman.

| Platform | Emulation | What it validates |
|----------|-----------|-------------------|
| `linux/amd64` | Native (no QEMU) | Standard x86_64 — most CI and cloud environments |
| `linux/arm64` | QEMU via `docker/setup-qemu-action` | ARM64 — Apple Silicon Macs, ARM cloud instances |

**How it works:**

```bash
podman run --rm \
  --platform linux/amd64 \
  -v "$GITHUB_WORKSPACE:/src:Z" \
  -w /src \
  golang:1.25 \
  sh -c "make ci-checks && make build && ./kortex-cli version"
```

- Uses Podman (daemonless, rootless, pre-installed on Ubuntu runners)
- Mounts the source code as a volume (`:Z` handles SELinux labeling)
- Runs inside a minimal `golang:1.25` image with no extra tools
- Catches hidden dependencies on host-installed tools or filesystem layout
- QEMU is only set up for `linux/arm64` — `linux/amd64` matches the runner's native architecture

---

## Test Categories

The project has three categories of tests, all run by `make test` / `make ci-checks`:

### Unit Tests

Test individual functions and methods in isolation.

- Located alongside source files (`*_test.go`)
- Test `preRun` methods, factory functions, validation logic
- Use `t.TempDir()` for filesystem isolation
- Use fake implementations for dependencies (no mocking frameworks)

**Example:** `pkg/cmd/init_test.go` (`TestInitCmd_PreRun`)

### E2E Tests

Test full command execution through Cobra.

- Execute via `NewRootCmd().SetArgs(...).Execute()`
- Verify stdout output, persistence, and side effects
- Use real storage with `t.TempDir()`

**Example:** `pkg/cmd/init_test.go` (`TestInitCmd_E2E`)

### Contract Tests

Test the CLI from the perspective of an external consumer (the kortex desktop app).

- Located in `pkg/cmd/contract_test.go`
- Use `execCmd()` / `mustExecCmd()` / `mustParseWorkspacesList()` helpers that create a fresh `NewRootCmd()` per invocation
- `execCmd()` returns `(stdout, stderr, error)` for full output inspection
- Ten test groups:

| Group | Tests | What it validates |
|-------|-------|-------------------|
| `TestContract_Lifecycle` | 4 | Full CRUD flow, duplicate init creates separate workspaces, multi-workspace management, command aliases |
| `TestContract_JSONSchema` | 5 | Top-level `items` key, non-null empty array, exact field names (`id`, `name`, `paths.source`, `paths.configuration`), deterministic output, typed/untyped parsing agreement |
| `TestContract_OutputFormat` | 5 | `init` outputs exactly one line (the ID), `init --verbose` outputs structured labels, `version` outputs non-empty string, `remove` outputs exactly one line (the ID), errors returned via `Execute()` |
| `TestContract_StorageResilience` | 5 | Corrupted JSON returns error (no panic), empty file treated as empty list, init works with empty file, isolated storage paths, persistence across command invocations |
| `TestContract_HelpText` | 5 | Root help lists all commands, init help lists all flags, workspace help lists subcommands, workspace list help has `--output`, workspace remove help shows `ID` |
| `TestContract_Stderr` | 4 | Error messages appear in stderr for invalid operations, successful commands produce empty stderr |
| `TestContract_SpecialCharacters` | 4 | Workspace names with spaces and unicode round-trip correctly, source directories with spaces and unicode are stored and returned exactly |
| `TestContract_UnknownInputs` | 3 | Unknown command shows help text, unknown flag returns error, extra arguments to no-args command returns error |
| `TestContract_CLIStandards` | 7 | Successful commands return nil error (exit 0), failed commands return non-nil error (exit 1), `--help` works on all subcommands, data commands write only to stdout, error commands write to stderr, non-verbose init is machine-parseable, JSON output is pure JSON |
| `TestContract_FlagBehavior` | 6 | `--storage` flag isolates data between paths, `--workspace-configuration` sets custom config path, config defaults to `<source>/.kortex`, name auto-generated from source basename, name deduplication on conflict, auto-generated name is never empty |

These tests catch breaking changes to the CLI's external interface that would affect the desktop app.

---

## Local Quality Tooling

### Quality Report

```bash
make quality-report
```

Runs coverage gap analysis and mutation testing locally:

- Prints total coverage, number of uncovered functions, and partially covered functions
- If `gremlins` is installed, runs mutation testing and prints the kill score
- If `gremlins` is not installed, prints a skip message with install instructions

### Mutation Testing

```bash
make test-mutate
```

Runs mutation testing in isolation. Requires gremlins:

```bash
go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
```

---

## Configuration Files

| File | Purpose |
|------|---------|
| `codecov.yml` | Coverage thresholds for PR status checks (80% project, 80% patch) |
| `.github/workflows/pr-checks.yml` | PR gate — CI checks, coverage upload, container tests |
| `Makefile` | Local targets: `test`, `test-coverage`, `test-mutate`, `quality-report`, `ci-checks` |

---

## Adding New Code

When adding new features or commands:

1. Write unit tests for validation and edge cases
2. Write E2E tests for full command execution
3. If the feature affects CLI output (stdout, JSON, exit codes), add contract tests in `contract_test.go`
4. Run `make ci-checks` locally to verify everything passes
5. Ensure new code has at least 80% test coverage (Codecov patch check will enforce this)
6. Optionally run `make quality-report` to check for remaining gaps
