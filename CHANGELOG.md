# Changelog

All notable changes to Helvilette Wunjo are listed here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **The `wunjo` binary builds from `cmd/wunjo` and prints its version with
  `wunjo version`.** `make test` runs `go vet` and every Go test, and `make build`
  writes `bin/wunjo`.
- **The `NOTICE` file carries Kamal's MIT notice.** The port ships Kamal's 45 test
  fixtures and its configuration documents, which the validator reads.
