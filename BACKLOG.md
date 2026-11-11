# Backlog

This backlog lists the tasks of Phase 1 in the [roadmap](ROADMAP.md): port Kamal 2.12.0
to Go and release version 0.1.0. The plan of record is
[Port Kamal 2.12.0 to Go](docs/plans/port-kamal-to-go.md). Its rules apply to every
task: port the Ruby tests first, match each Docker command string to the character,
and name the Ruby source file at the top of each Go file. No GitHub issues exist for
these tasks yet.

## Integration and release

The following tasks prove the port end to end and release it:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-081 | Deploy one of the author's real services from its existing `config/deploy.yml`. | None | None | `wunjo deploy` succeeds without edits to the file, and the author accepts the result. |
| WJ-090 | Release version 0.1.0. | `CHANGELOG.md` | WJ-081 | Tag `v0.1.0` exists, and `CHANGELOG.md` describes the release. |
