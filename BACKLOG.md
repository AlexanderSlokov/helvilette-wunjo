# Backlog

This backlog lists the tasks of Phase 1 in the [roadmap](ROADMAP.md): port Kamal 2.12.0
to Go and release version 0.1.0. The plan of record is
[Port Kamal 2.12.0 to Go](docs/plans/port-kamal-to-go.md). Its rules apply to every
task: port the Ruby tests first, match each Docker command string to the character,
and name the Ruby source file at the top of each Go file. No GitHub issues exist for
these tasks yet.

## Release

The author accepted the port on 2026-11-12 after a demo deploy of
[tmeire/minimal-kamal](https://github.com/tmeire/minimal-kamal), which replaces task
WJ-081. Tasks WJ-091, WJ-092, and WJ-093 wrote the tutorial, the install guide, and
the README for version 0.1.0. The following task releases it:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-090 | Release version 0.1.0: merge `mvp` into `main`, move `[Unreleased]` to `[0.1.0]` in the changelog, tag `v0.1.0`, run `make release` at the tag, and publish the binaries with `SHA256SUMS`. | `CHANGELOG.md` | None | Tag `v0.1.0` exists, the GitHub release holds the four binaries and `SHA256SUMS`, the checksums match, and the `curl` and `git clone --branch v0.1.0` commands of `docs/how-to/install-wunjo.md` and `docs/tutorials/deploy-a-sample-app.md` run against the published release. |
