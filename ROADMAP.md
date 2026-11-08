# Roadmap

This roadmap lists the planned work for Helvilette Wunjo, a Go tool that deploys
containers from a Compose file to the operator's own servers over SSH. Each item links
to the document that holds its reasoning.

## Phase 0: Proposal

### RFC 0001: Why Helvilette Wunjo exists

The project will settle why Wunjo exists and what it proposes before writing code. See
[RFC 0001: Why Helvilette Wunjo exists](docs/explanation/rfc-0001-why-helvilette-wunjo.md).

- Collect comments from colleagues and AI agents on the draft.
- Resolve the open questions on routing without a hostname, the scope of
  `wunjo import`, drift, and secret backends.
- Agree on the target version of the first release.

## Later

### Secret backends

Wunjo will read secrets from a backend such as Vault, after the first version reads
them from `.env` files. See
[RFC 0001: Why Helvilette Wunjo exists](docs/explanation/rfc-0001-why-helvilette-wunjo.md#one-file).

- Pick the first backend from the RFC's open questions.
