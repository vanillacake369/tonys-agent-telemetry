# Contributing

Thanks for taking the time to make `tonys-agent-telemetry` better.

This file covers the practical bits: how to set up a dev environment,
what the test gate looks like, the commit / PR conventions, and the
AI-tool disclosure policy. For project goals + tab walkthrough, see
[`README.md`](./README.md); for architecture deep-dives, see
[`docs/architecture.md`](./docs/architecture.md).

## Dev setup

You need Go **1.26+** (matches `go.mod`). Either:

```sh
# Native — works on macOS + Linux. Brew installs match the CI go-version: stable.
brew install go

# Or Nix, for a hermetic environment matching CI:
nix develop
```

Then:

```sh
git clone https://github.com/vanillacake369/tonys-agent-telemetry
cd tonys-agent-telemetry
make build         # → bin/tonys-agent-telemetry
make hooks-install # one-time, wires .githooks/ as core.hooksPath
```

The pre-commit hook runs `gofmt -l`, `go vet`, `go test -short`, and
(if installed) `golangci-lint run --new-from-rev=HEAD~`. It runs on every
commit; failure aborts the commit so you can't ship a red change.

## Test gate

CI (`.github/workflows/ci.yml`) runs three jobs against every push and PR:

1. **lint** — `gofmt` check, `go vet`, and `golangci-lint` against changes
   since `origin/main` (`--new-from-rev=origin/main`). Accumulated pre-existing
   issues are tolerated; only your diff is graded.
2. **test** — `go test -race -count=1 ./...` across all packages. Plus a
   `go test -coverprofile` artefact for inspection.
3. **build** — pure-Go (`CGO_ENABLED=0`) cross-build matrix:
   linux+darwin × amd64+arm64.

Locally, `make ci` runs the same checks against your working tree.

```sh
make ci          # fmt-check + vet + test-race
make lint-new    # golangci-lint against HEAD~ (or LINT_BASE=origin/main for PR-style)
make demo        # regenerate docs/demo/tour.gif (needs vhs)
```

## Commit & PR conventions

Subjects follow [Conventional Commits](https://www.conventionalcommits.org/),
enforced by `.githooks/commit-msg`. Allowed types:

```
feat   fix    chore   docs   test
refactor   perf   build   ci   style   revert
```

Optional scope in parens, optional `!` for breaking change:

```
feat(signal): add stalled_node detector
fix(tui): tab cycling skips Trends
refactor(catalog)!: rename Item.Tags → Item.Topics
```

PR template (`.github/PULL_REQUEST_TEMPLATE.md`) asks you to confirm:

- [ ] tests added/updated
- [ ] docs updated (README + relevant `docs/*.md`)
- [ ] `make ci` passes locally
- [ ] commits follow Conventional Commits
- [ ] AI-assistance disclosed (see below)

Keep PRs **focused** — one logical change per PR. Mechanical refactors get
their own PR.

## AI-assistance disclosure

A lot of this codebase was written with AI pair programming, and that's
welcome to continue. We only ask two things:

1. **You read every line you propose.** AI-generated code that the author
   hasn't actually understood is rejected, regardless of whether tests
   pass. "I think this might work" is not a contribution.
2. **Disclose in the PR description** roughly how AI was used — e.g.,
   "Claude generated the initial signal detector; I rewrote the rolling
   hash and added the K=50 perf test." This isn't a moral test; it's so
   reviewers know where to apply extra scrutiny.

## File-by-file conventions

- **`internal/signal/`** — pure-function detectors, one per file (SRP).
  Every detector has its own `*_test.go` file. No I/O in this package.
- **`internal/recommender/`** — every `Recommendation` MUST carry both
  `SignalID` and `CatalogItemID`. `policy.go::EnforceEvidence` enforces this
  at boundary. Don't bypass it.
- **`internal/tui/`** — `TabModel.View()` contract is documented at
  `tab` interface declaration: returned string MUST NOT exceed
  `height` rows. `clipContentToHeight` is the safety net but per-tab
  budgeting is the design.
- **Test files** — race-sensitive perf tests use the
  `isRaceEnabled()` build-tag pair (`race_off_test.go` / `race_on_test.go`).
  Live-network tests gate on `TONYS_RUN_LIVE=1` + `CI != "true"`.
- **`scripts/demo/`** — `.tape` is the source of truth for the README GIF.
  `seed.sh` is idempotent and writes only to `/tmp/tonys-demo`.

## Filing issues

- **Bugs** — use the [bug template](.github/ISSUE_TEMPLATE/bug_report.yml).
  Include version (`tonys-agent-telemetry --version`), OS + terminal,
  and steps to reproduce.
- **Features** — use the [feature template](.github/ISSUE_TEMPLATE/feature_request.yml).
- **Security** — see [`SECURITY.md`](./SECURITY.md) — **do not** file
  public issues for vulnerabilities.
- **Questions / usage help** — open a GitHub Discussion. Issues are for
  defects and concrete proposals.

## Release process (maintainer reference)

1. Land all PRs on `main`. CI must be green.
2. Update `CHANGELOG.md` with an `## [X.Y.Z] — YYYY-MM-DD` entry.
3. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z — <one-line>"`.
4. Push the tag — `release.yml` runs goreleaser, signs every artefact
   with cosign keyless, generates SLSA L3 provenance, and publishes
   the GitHub release.
5. If the `HOMEBREW_TAP_TOKEN` secret is set, the formula is auto-pushed
   to `vanillacake369/homebrew-tap`. Otherwise, the formula is generated
   under `dist/homebrew/` for manual push.

## License

By contributing you agree your contributions will be licensed under the
[MIT License](./LICENSE) — same as the rest of the project.
