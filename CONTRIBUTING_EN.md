# Contributing Guide

Thank you for your interest in and contributions to `zhenyi`.

## Before You Start

- Please read the root `README.md` and `LICENSE` first (this project is **AGPL-3.0 + Commercial Dual License**).
- Submitting a PR means you agree that your contribution will be licensed under this project's dual-license model (see "Contributor License and CLA" below for details).

## How to Submit Issues

- **Bug**: Provide reproducible steps, expected behavior, actual behavior, logs/stack, Go version, and OS.
- **Feature**: Describe use case, API design draft, compatibility impact, and alternatives.

## How to Submit PRs

- **One PR solves one problem**: Avoid large, complex changes.
- **Maintain backward compatibility**: Clearly state API changes in PR description and update documentation accordingly.
- **Write tests**: Bug fixes and new features should include corresponding test cases when possible.

### Repository hygiene (before every PR)

Do not commit generated `logs/`, `*.log`, `.env`, real secrets, or local-only PEM/config. Run `git status` before pushing. See "Repository hygiene" at the end of [SECURITY_EN.md](SECURITY_EN.md).

### Local Checks (Before Submission)

To match CI behavior as closely as possible, prefer the repository `Makefile` targets (instead of hand-written commands). Recommended:

- `make release-check` (closest to CI: docs-check + go test -count=1 + bug-check)
- Or at least `make test-unit` (fast) / `make test` (includes `-race`, stricter)

```bash
go test ./... -count=1
go vet ./...
test -z "$(gofmt -l .)"
```

The repository also provides `Makefile`:

```bash
make test
make test-unit
make bug-check
make docs-check
make release-check
```

## Common Failure Troubleshooting

- **Wrong Go version**
  - This project declares Go version in `go.mod`; ensure your local Go matches CI (see Go version badge in root `README.md`).
- **`make bug-check` fails due to missing tools**
  - `bug-check` runs `staticcheck`, `gosec`; install locally if missing, or use `make bug-check-strict` to match CI behavior.
- **Script engine/example related issues**
  - Single-node examples (`im_single_demo`/`im_single_client`) don't require Etcd/NATS; multi-process examples typically need external dependencies reachable (see `docs/EXAMPLES.md`).
- **Documentation link check fails**
  - Run `make docs-check` to see broken link list, fix relative paths or adjust documentation locations.

## Contributor License and CLA

To ensure open source release and dual-license consistency for this project, all contributors need to agree to the contributor license terms:

- Please read and agree to `CLA.md` before your first contribution.
- Submitting a PR confirms that you have the right to contribute that code and agree to the terms in `CLA.md`.
