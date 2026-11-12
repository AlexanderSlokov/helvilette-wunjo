# Roadmap

This roadmap lists the planned work for Helvilette Wunjo, a Go tool that deploys
containers to the operator's own servers over SSH. Wunjo starts as a port of Kamal and
then adds its own extensions. Each item links to the document that holds its reasoning.

## Phase 0: Proposal

### RFC 0001: Why Helvilette Wunjo exists

The project will settle why Wunjo exists and what it proposes before writing code. See
[RFC 0001: Why Helvilette Wunjo exists](docs/explanation/rfc-0001-why-helvilette-wunjo.md).

- Collect comments from colleagues and AI agents on the draft.
- Resolve the open questions on routing without a hostname, the scope of
  `wunjo import`, drift, and secret backends.

## Phase 1: Port Kamal 2.12.0 to Go (released as 0.1.0 on 2026-11-12)

Wunjo will behave like Kamal 2.12.0, command for command, as a single Go binary. See
[Port Kamal 2.12.0 to Go](docs/plans/port-kamal-to-go.md).

- Port Kamal's tests first, and match every Docker command string.
- Render the ERB subset that the RFC defines, and reject other ERB with a clear error.
- Keep Kamal's MIT notice in `NOTICE`.
- Port three of Kamal's nine secret adapters: AWS Secrets Manager, GCP Secret Manager,
  and Doppler.
- Finish with a deploy that the author accepts. On 2026-11-12 the author accepted a
  deploy of [tmeire/minimal-kamal](https://github.com/tmeire/minimal-kamal) to the
  test stack in `test/integration/`, in place of a deploy of the author's own service.
- Release 0.1.0 with a tutorial, an install guide, and binaries for Linux and macOS.

## Phase 2: Wunjo extensions (target 0.2.0)

Wunjo will add the features that Kamal leaves out, as separate code on top of the port.
See [RFC 0001: Proposal](docs/explanation/rfc-0001-why-helvilette-wunjo.md#proposal).

- Let the builder trust a registry with an internal CA or plain HTTP.
- Read a Compose file with `x-wunjo` blocks, and convert Kamal files with
  `wunjo import`.
- Route services that have no hostname.
- Record the image digest of every deploy.
- Print one line per service on success and a short report on failure.
- Preview changes with `wunjo plan`, including GPU services that must stop first.

## Later

### Day-2 view

Wunjo will offer `wunjo ui`, a terminal view of each host for day-2 operations, as k9s
does for a Kubernetes cluster. See
[RFC 0001: Day-2 view](docs/explanation/rfc-0001-why-helvilette-wunjo.md#day-2-view).

- Build it after Phase 2, when the digest history and `wunjo plan` exist.
- First version, read-only: show images, containers, `kamal-proxy` state, disk usage,
  and locks per host, and tail the logs of one service across its hosts.
- Read data over SSH with read-only Docker commands, with nothing installed on the
  servers.
- Later version: add rollback, lock, and prune, each mapped to a CLI command that the
  view prints before it runs.

### Secret backends

Wunjo will read secrets from more vaults with `wunjo secrets fetch`. See
[RFC 0001: Why Helvilette Wunjo exists](docs/explanation/rfc-0001-why-helvilette-wunjo.md#one-file).

- Add adapters that Kamal lacks: HashiCorp Vault and Infisical.
- Port Kamal's other six adapters (1Password, Bitwarden, Bitwarden Secrets Manager,
  LastPass, Enpass, and Passbolt) one at a time, when a user asks for one in an issue
  or in Kamal's GitHub Discussions.
