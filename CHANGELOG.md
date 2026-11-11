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
- **Wunjo reads `.kamal/secrets` files as Kamal does.** The common file, the
  destination file, `$(command)` substitution, `$VAR` references, and in-process
  `kamal secrets` commands follow Kamal 2.12.0 with dotenv 3.2.0, and each command
  runs once per deploy.
- **Wunjo reads and validates `config/deploy.yml` as Kamal 2.12.0 does.** Roles,
  accessories, env tags, proxy, builder, SSH, and destination files load in Kamal's
  order, and an invalid file fails with Kamal's error message, such as
  `servers/web/hosts: should be an array`.
- **Wunjo builds the same Docker commands as Kamal 2.12.0.** App, accessory, proxy,
  builder (local, remote, hybrid, pack, and cloud), registry, lock, prune, audit, and
  hook commands match Kamal's 215 command tests to the character.
- **Wunjo runs commands on hosts with your own `ssh` client.** It keeps one connection
  per host open, starts at most `sshkit/max_concurrent_starts` connections at once,
  retries DNS failures, and logs each command in SSHKit's format. `make
  test-integration` runs a command on sshd in Docker through an SSH agent.
