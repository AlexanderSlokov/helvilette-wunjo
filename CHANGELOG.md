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
- **Wunjo renders the ERB subset of RFC 0001 in deploy files and accessory files.**
  `ENV["NAME"]`, `ENV.fetch`, string literals, comments, and the `<%-` and `-%>`
  markers give the same result as Ruby 3.4. Any other expression stops before Wunjo
  contacts a server, with an error that names the file, the line, and the expression.
- **Deploy files keep their key order and follow YAML 1.1, as in Kamal.** Wunjo reads
  `yes` and `on` as true, `3000:30` as a base-60 integer, and `:quiet` as a symbol,
  so `--env` and `--label` flags come out in the order of the file.
