# Backlog

This backlog lists the tasks of Phase 1 in the [roadmap](ROADMAP.md): port Kamal 2.12.0
to Go and release version 0.1.0. The plan of record is
[Port Kamal 2.12.0 to Go](docs/plans/port-kamal-to-go.md). Its rules apply to every
task: port the Ruby tests first, match each Docker command string to the character,
and name the Ruby source file at the top of each Go file. No GitHub issues exist for
these tasks yet.

## Command-line interface

The following task ports the `wunjo` commands:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-050 | Port `cli/`: deploy, redeploy, rollback, app, accessory, proxy, build, registry, server, lock, prune, secrets, aliases, hooks, and the health-check poller. | `internal/kamal/cli/`, `cmd/wunjo/` | None | The ported `test/cli/` tests pass with the fake runner, and `wunjo --help` lists the same commands as `kamal --help` at version 2.12.0. |

## Secret adapters

The following task ports the vendor adapters:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-060 | Port the ten adapters in `secrets/adapters/`. | `internal/kamal/secrets/adapters/` | WJ-050 | The ported `test/secrets/` adapter tests pass with a named fake for each vendor tool. |

## Output

The following task ports logging and OpenTelemetry (OTel) shipping:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-070 | Port `output/` and `otel_shipper.rb`. | `internal/kamal/output/` | WJ-050 | The ported `test/output/`, `test/otel_shipper_test.rb`, and `test/configuration/output_test.rb` pass. |

## Integration and release

The following tasks prove the port end to end and release it:

| ID | Task | Files | Depends on | Done when |
|---|---|---|---|---|
| WJ-080 | Port the integration suite in `test/integration/`. | `test/integration/` | WJ-060, WJ-070 | The ported integration tests pass with the `wunjo` binary. |
| WJ-081 | Deploy one of the author's real services from its existing `config/deploy.yml`. | None | WJ-080 | `wunjo deploy` succeeds without edits to the file, and the author accepts the result. |
| WJ-090 | Release version 0.1.0. | `CHANGELOG.md` | WJ-081 | Tag `v0.1.0` exists, and `CHANGELOG.md` describes the release. |
