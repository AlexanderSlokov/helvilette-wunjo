# To do

This file lists the checks that the author does before task WJ-081 in the
[backlog](BACKLOG.md), which deploys one real service with `wunjo deploy`. Agents do
not pick up these items.

## Inventory for the first real deploy

The following items describe the service and the server for WJ-081:

- [ ] **Service.** Pick a service that deploys with Kamal 2.x and has a
    `config/deploy.yml`.
- [ ] **Server.** Prepare a server that accepts SSH with a key from your agent and
    that you can rebuild, so a failed deploy affects no production traffic.
- [ ] **Registry.** Confirm the image registry and the account that
    `registry/username` names.
- [ ] **Secret adapter.** Note the `kamal secrets fetch` adapter that `.kamal/secrets`
    calls, for example 1Password. Task WJ-060 ports that adapter first.
- [ ] **Destinations and hooks.** List the destination files, such as
    `config/deploy.staging.yml`, and the scripts in `.kamal/hooks/`.
- [ ] **ERB.** Run `grep -n "<%" config/deploy*.yml` and confirm that each match uses
    `ENV["NAME"]`, `ENV.fetch`, or a comment. Wunjo stops on any other ERB, as
    [Known differences from Kamal](docs/plans/port-kamal-to-go.md#known-differences-from-kamal)
    describes.
