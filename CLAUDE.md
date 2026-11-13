# CLAUDE.md

Open Food Log: an open format for a personal food log, the Open Food Catalogue, the `ofl` command,
the conformance suite and the Go client (v0 draft until the first outside app writes it in
production).

version_file: none (releases are git tags `vX.Y.Z`; the schema's own version is `ofl_version`)

## Rules

- Until v1 the format is published here from Lifterclub, the reference implementation
  (`scripts/publish-openfoodlog.sh` there). `schema/CHANGELOG.md` is the format's changelog and is
  copied in by that script: don't edit it here.
- Before a pull request: `go run ./cmd/ofl validate` and `go test ./...`.

## Changelog

`CHANGELOG.md` is the running record of this repository. Every change someone would notice adds an
entry under `## Unreleased` in the same pull request; work left undone goes under `## Still to do`
and comes off when it ships. On release, rename `Unreleased` to the release heading and start a new
empty one. `CONTRIBUTING.md` has the contributor version of this rule.

## Commits

Conventional commits (`type(scope): description`), on a branch with a pull request; never commit
to `main` directly. No attribution trailers.
