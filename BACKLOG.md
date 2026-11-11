# Backlog

This backlog lists the tasks of Phase 1 in the [roadmap](ROADMAP.md): port Kamal 2.12.0
to Go and release version 0.1.0. The plan of record is
[Port Kamal 2.12.0 to Go](docs/plans/port-kamal-to-go.md). Its rules apply to every
task: port the Ruby tests first, match each Docker command string to the character,
and name the Ruby source file at the top of each Go file. No GitHub issues exist for
these tasks yet.

## Documentation and release

The author accepted the port on 2026-11-12 after a demo deploy of
[tmeire/minimal-kamal](https://github.com/tmeire/minimal-kamal), which replaces task
WJ-081. The following tasks document and release version 0.1.0:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-091 | Write a tutorial that deploys minimal-kamal to the test stack: start the stack, add a `demo` destination, deploy, change the code, redeploy, and roll back. | `docs/tutorials/deploy-a-sample-app.md` | None | Each command in the tutorial runs at the commit that adds it, and the page follows the tutorial sections of `AGENTS.md`. |
| WJ-092 | Write a how-to guide that installs `wunjo` from a release binary or with `make install`, and checks it with `wunjo version`. | `docs/how-to/install-wunjo.md` | None | Each command runs, and the guide names the OpenSSH client as a requirement. |
| WJ-093 | Update the README for 0.1.0: a working example with `wunjo`, links to the tutorial and the install guide, and the known differences from Kamal. | `README.md` | WJ-091, WJ-092 | The example runs, and every relative link resolves. |
| WJ-090 | Release version 0.1.0: move `[Unreleased]` to `[0.1.0]` in the changelog, tag `v0.1.0`, and publish the `make release` binaries with `SHA256SUMS`. | `CHANGELOG.md` | WJ-093 | Tag `v0.1.0` exists, the GitHub release holds the four binaries and `SHA256SUMS`, and the checksums match. |
