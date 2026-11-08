# RFC 0001: Why Helvilette Wunjo exists

The following table records the state of this request for comments (RFC):

| Field | Value |
|---|---|
| Status | Draft, open for comments |
| Author | Aleksandr Slokov |
| Created | 2026-11-08 |
| Updated | 2026-11-09, after the second interview round |
| Reviewers | Colleagues of the author and AI agents |

## Overview

Helvilette Wunjo (_Wunjo_) is a proposed command-line tool, written in Go, that deploys
containers from an operator's laptop to the operator's own servers over SSH. It keeps
the parts of Kamal that make deployment light, and it drops the decisions that Kamal
makes for its users when those decisions do not fit an operator's work. This RFC records
why Wunjo exists, who it serves, and what it proposes, so that reviewers can challenge
the reasoning before any code exists. To comment, open an issue or a pull request
against this file. The section [Open questions](#open-questions) lists the points where
the author wants other opinions most.

## Terms

This RFC uses the following terms:

- **Operator**: The person who deploys software to production and gets the call when it
  breaks. In a small team, one operator often carries the whole infrastructure.
- **Kamal**: An open-source deployment tool by 37signals, released under the MIT
  license. Kamal builds a container image, pushes it to a registry, pulls it on each
  server over SSH, and swaps the old container for the new one without dropping
  traffic. Its command-line interface is a Ruby program.
- **kamal-proxy**: The reverse proxy, written in Go, that Kamal runs on each server. It
  sends traffic to the new container once that container passes its health check.
- **DHH**: David Heinemeier Hansson, the creator of Ruby on Rails and of Kamal, and
  co-owner of 37signals.
- **Compose file**: A `compose.yaml` file read by `docker compose`. It describes
  services, images, ports, volumes, GPUs, and logging drivers in Docker's own terms.
- **Digest**: The content hash of a container image, such as `sha256:9f3a…`. A tag such
  as `latest` can point to different images over time. A digest always names the same
  image.
- **Helvilette**: The author's umbrella name for a set of ideas about infrastructure
  software, expressed through software projects. Helvilette is a way to read the
  author's views. It is open to change, and no project has to follow it to the letter.
- **Naughtian**: The family of projects that follow
  [The letter from Naught](the-letter-from-naught.md), a short manifesto
  for tools that serve a lone operator.
- **Wunjo**: ᚹ, the rune of joy. The name states the goal: the operator enjoys using the
  tool. The name comes from the Helvilette fiction, and the fiction places no
  constraint on the software.

## The problem

### Before Kamal

The author deploys containers to a small fleet of servers on a local area network (LAN),
including GPU containers for AI workloads such as ComfyUI. Two paths existed:

- Deploying by hand over SSH works, and it costs the same manual steps on every release.
- Kubernetes automates the steps, and it hides the system so deeply that a lone
  operator struggles to repair it when it breaks.

Kamal sat between the two, and it worked well for a while.

### What Kamal gets right

Kamal makes a set of good decisions on behalf of its users:

- One command deploys a release, and one command rolls it back.
- `kamal-proxy` switches traffic to a new container only after it passes a health check,
  so a release does not drop requests.
- `kamal app logs` and `kamal app exec` reach the right container without the operator
  remembering its name.
- Kamal has few moving parts. Portainer, by comparison, adds Portainer itself to
  production, and production runs best with the fewest moving parts.

Wunjo keeps all of this.

### Where Kamal stops fitting

The author hit the following cases in real deployments:

- **Routing needs a hostname.** `kamal-proxy` selects a service by the hostname of the
  request. On a LAN where clients reach a server by IP address, two services on one
  host have no hostname to tell them apart.
- **The registry must speak HTTPS.** Pushing to an internal Harbor registry over plain
  HTTP failed. Inside a private LAN, TLS between the build host, the registry, and the
  servers adds certificates to manage and protects little. The author found no way to
  state "this registry uses HTTP" in `config/deploy.yml`.
- **The Compose file is thrown away.** The author had a clean Compose file for each
  application. Kamal does not read Compose, so every image, port, volume, and
  environment variable had to be written again in `config/deploy.yml`, and the two
  files then drift apart.
- **The image version is the Git commit.** Kamal tags each image with the commit hash
  of the current checkout. An operator who deploys an image built elsewhere, or who uses
  a mutable tag such as `latest`, has to pass extra flags. The tool assumes that the
  operator cannot track image versions.
- **Anything that is not a web app is an exception.** GPU access, logging drivers such
  as Loki or Fluentd, and volume plugins for network storage (NAS) all go through
  generic `options` fields or hooks. One intent ends up spread across several places.
- **Every deploy prints a log dump.** A successful deploy prints each remote command.
  The operator reads hundreds of lines to learn one fact: the release is live.
- **The tool needs Ruby.** Installing a Ruby runtime is a prerequisite for a tool whose
  job is to run containers.

Reviewers who know a Kamal setting that resolves one of these cases are asked to point
to it. The case then moves to [Open questions](#open-questions).

### The pattern behind the cases

Each case comes from one source. Kamal grew out of 37signals moving its own applications
off the cloud. At 37signals, an application, its repository, and its image are one
thing, every service is a Rails web app with a public hostname, and an operations team
carries the infrastructure below it. Kamal encodes those facts as rules.

DHH describes Rails as _omakase_: the chef chooses the meal. Kamal presents its rules
in the same way. The author compares it to a sushi chef whose fish is excellent. The
guest comes for the fish, scrapes off the ginger, and adds a stronger soy sauce. The
chef refuses to serve the dish any other way. The guest still likes the fish. The
guest also knows what tastes good in their own mouth.

Once Kamal leaves 37signals and reaches another operator, it serves that operator. A
tool exists to serve the person who uses it. When the rules of the author of the tool
outrank the needs of the user, the tool serves the author.

The promise that infrastructure disappears has a cost as well. Complexity moves from
the developer to the operator: the developer gets "push and it runs", and the operator
inherits everything the tool hid. Wunjo accepts the first half of that trade and
refuses the second. It removes chores, and it keeps the infrastructure visible down to
Docker at all times.

## Who Wunjo serves

Wunjo serves the operator. The design assumes that the operator knows Docker and Docker
Compose well, and it does not explain or guard Docker concepts.

Developers in the same team use Wunjo as a convenience. A developer runs the same
deploy command that the operator uses, instead of pasting a Compose file into a web
interface such as Portainer. When the needs of the developer and the operator conflict,
the operator's needs win.

The short form of this priority is "happy dev, happier ops".

## Principles from the letter from Naught

Wunjo is an independent project under the Helvilette name and belongs to the Naughtian
family. [The letter from Naught](the-letter-from-naught.md) sets four architectural
principles and two rules about failure domains. The following table shows what each one means for Wunjo:

| Principle | What it means for Wunjo |
|---|---|
| Minimalism and elegance | An operator who reads Compose reads Wunjo's configuration without learning a new format. |
| Compact codebase | Wunjo stays a thin layer on top of Docker and Compose, so one person can audit it in a day. It does not reimplement Docker. |
| No complex internal state | Wunjo runs no control plane, daemon, or database of its own. Between deploys, it holds no running state. |
| Absolute transparency | Wunjo acts on the operator's choices: any tag, any registry, any Docker option. It records every action, and `--verbose` shows each Docker command it runs. |
| Independent Unix-style tools | Wunjo deploys containers. Provisioning machines stays with tools such as Terraform or Ansible. |
| Zero hostage infrastructure | When the operator removes Wunjo, a working `compose.yaml` remains, and `docker compose up` still runs it. |

The letter rejects _pragmatism_, which it defines as patchwork that keeps a system
running today and piles up debt for later. Wunjo follows minimalism: a small scope,
built to a high standard. A Wunjo feature is done when it is correct and complete for
its scope. "It works for now" does not meet that bar.

## Proposal

### One file

In this proposal, the Compose file is the deploy file. Compose ignores every key that
starts with `x-`, so Wunjo adds its settings under `x-wunjo` and the file stays a valid
Compose file. The following example shows a GPU service:

```yaml
services:
  comfyui:
    image: harbor.lan:8080/ai/comfyui:latest
    gpus: all
    volumes:
      - models:/opt/models
    logging:
      driver: loki
      options:
        loki-url: http://loki.lan:3100/loki/api/v1/push
    x-wunjo:
      hosts: [gpu-node-01]
      health: /system_stats

volumes:
  models:
    driver_opts:
      type: nfs
      o: addr=nas-01.lan,rw
      device: ":/export/models"
```

GPU access, the logging driver, and the NAS volume stay in Docker's own syntax. The
`x-wunjo` block adds only what Docker does not know: which hosts run the service, and
how to tell that a new release is healthy. Supporting services such as PostgreSQL or
Redis are further entries under `services`, with no separate syntax.

Secrets stay in `.env` files, which Compose already reads through `env_file`, so the
Compose file holds no secret values. Reading secrets from Vault or another backend is
planned for a later version.

### What Wunjo keeps from Kamal

Wunjo keeps the deployment experience. The proposed commands are the following:

- `wunjo deploy` pulls the image on each host listed in `x-wunjo` and swaps
  containers after a health check. Multi-host deploys work as they do in Kamal.
- `wunjo rollback` returns a service to a previous release. As in Kamal, each host
  keeps the containers of recent releases, and rollback starts one of them again.
- `wunjo logs` and `wunjo exec` reach the right container by service name.
- `wunjo plan` previews what a deploy changes and which containers restart.

Wunjo runs `kamal-proxy` unchanged, as the same binary that Kamal uses. The transport
is SSH from the operator's laptop, as in Kamal. SSH is not a problem in
this scope, and an agent on each server adds a moving part with no gain here.

### What Wunjo changes

Wunjo changes the rules that come from 37signals:

- **Routing works without a hostname.** A service with a hostname goes through
  `kamal-proxy`. A service without one publishes the ports from its Compose file
  directly, so two IP-only services on one host no longer collide. Because the old and
  new containers cannot bind the same port at once, such a service stops before it
  restarts.
- **The registry is the operator's choice.** Wunjo pushes and pulls through the Docker
  daemons on the laptop and the servers, so their own `insecure-registries` settings
  apply. Wunjo adds no TLS rule of its own.
- **Any tag is valid.** Wunjo records the digest of each image it deploys, so the
  history names the exact image behind a mutable tag such as `latest`.
- **Building stays with Compose.** Wunjo deploys images that already exist in a
  registry. To build an image, the operator runs `docker compose build`. Wunjo has no
  builder of its own.
- **Non-web workloads are first class.** A GPU job or a worker with no HTTP port uses
  the same file and the same commands as a web app.
- **Kamal configuration converts once.** `wunjo import` reads an existing
  `config/deploy.yml` and writes the matching services and `x-wunjo` blocks into
  `compose.yaml`. The Kamal file stays in place, so the project can return to Kamal at
  any time.

### Quiet output

A successful deploy prints one line per service. The following output is an
illustration of the proposed format:

```console
✓ comfyui latest → gpu-node-01  digest sha256:9f3a…
```

A failed deploy names the step that failed, states whether the old release still
serves traffic, and points to the next command:

```console
✗ comfyui latest → gpu-node-01: unhealthy after 60 seconds
  GET /system_stats returned 500
  Previous release sha256:41c7… is still running
  Next: wunjo logs comfyui
```

Wunjo writes the full log to disk on every run. `--verbose` prints it to the terminal.

### GPU releases

A model that fills most of a GPU's memory leaves no room for the new container to start
next to the old one. On such a host, a release has to stop the old container first.
Wunjo states this in `wunjo plan` output and does not claim a zero-downtime swap it
cannot perform. Kamal has the same physical limit.

### Success criterion

Wunjo succeeds when the author prefers it to Kamal and deploys real services with it.

## Non-goals

Wunjo does not do the following:

- Run a control plane, a scheduler, or an orchestrator.
- Reimplement Docker or Compose.
- Require Ruby or any runtime besides a single Go binary.
- Provision or configure servers.
- Replace Kubernetes for workloads that need Kubernetes.
- Build images. `docker compose build` does that.
- Offer a log viewer or dashboard. `wunjo logs` fetches logs, and tools the operator
  already runs, such as Portainer, stay available for browsing them.

## Alternatives considered

The author considered the following tools before proposing a new one:

| Alternative | Why it falls short for this operator |
|---|---|
| Kamal | Every case in [Where Kamal stops fitting](#where-kamal-stops-fitting). Kamal also ships as a container image, which removes the Ruby install and leaves the other cases. |
| Terraform Docker provider | Strong state tracking and drift detection. Replacing a container drops traffic, and secrets in environment variables land in the state file. |
| Ansible | Can do every step, and the operator writes and maintains the whole deploy procedure. |
| Kubernetes | Too opaque for one operator to repair when it breaks. |
| Portainer | Adds Portainer itself as a moving part in production. |
| Docker Swarm | Has start-first rolling updates. Adds a cluster with its own consensus, which the letter from Naught avoids. |
| Uncloud | Close in spirit: Compose files, deploys over SSH. Runs a daemon and a WireGuard mesh with replicated state on every machine. |
| Podman Quadlet | Runs containers as systemd units with pull-based updates. Has no health-checked swap, and moves the operator from Docker to Podman. |
| `DOCKER_HOST=ssh://… docker compose up -d` | The closest baseline. It has no health-checked swap, no rollback history, and no multi-host deploy from one file. |

The last row sets the bar. Each Wunjo feature has to do something that this baseline
plus a short script cannot.

## Open questions

The second interview round settled building, rollback, the proxy, secrets, and Kamal
compatibility. The author asks reviewers to weigh in on the following points:

1. **Services without a hostname.** Publishing ports directly forces a stop-then-start
    release. Is that acceptable for IP-only services on a LAN, or does a better route
    through `kamal-proxy` exist?
1. **Scope of `wunjo import`.** Which keys of `config/deploy.yml` must the import
    convert for a real Kamal project? The proposal lists every key it cannot convert
    and stops. Is that the right behavior?
1. **Drift.** Should `wunjo plan` report containers that someone changed by hand on a
    host, or does that check belong to another Helvilette tool?
1. **Secret backends.** After `.env` files, which backend comes first: Vault,
    1Password, or the Kamal `.kamal/secrets` format?

## What's next

- Read [The letter from Naught](the-letter-from-naught.md) for the principles this RFC
  applies.
- Read [Kamal 2](https://dev.37signals.com/kamal-2/) for the tool this RFC builds on.
- Read the [Compose extension fields](https://docs.docker.com/reference/compose-file/extension/)
  reference for the `x-` mechanism.
