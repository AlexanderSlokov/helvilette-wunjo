# Session record: tutorial run on 2026-11-12

This record holds the results of one run of the tutorial
[Deploy a sample app with Wunjo](../../tutorials/deploy-a-sample-app.md), which deploys
[tmeire/minimal-kamal](https://github.com/tmeire/minimal-kamal) to the test stack in
`test/integration/`. The run is the done-when condition of backlog task WJ-091.

## Run

The run used the following code, commands, and host:

| Item | Value |
|---|---|
| Code | Commit `e8f5601`. The working tree differed from it only in documentation. |
| App | minimal-kamal at commit `20294ecdb8395918290d12519778f9e78f76fe99` |
| Commands | The commands of the tutorial, in order, in one shell |
| Host | Linux 7.0.0-42-generic, x86_64, 8 CPUs, 13 GB of memory |
| Tools | Docker 29.9.2, Docker Compose 5.7.0, Go 1.27.4 |
| Compose project | `wunjo-demo`, with images partly cached from an earlier build |

## Results

The following table lists what each step measured:

| Step | Result | Measure |
|---|---|---|
| `wj deploy -d demo`, first deploy | Pass, no `ERROR` line | 777.1 seconds |
| `wj deploy -d demo` after the heading change | Pass, heading reads "Things to do" | 161.5 seconds |
| Requests to `/` through the load balancer during the second deploy, one every 0.2 seconds | 741 of 741 returned HTTP status 200 | 741 requests |
| To-do items after the second deploy | Both items present on `vm1` | 2 of 2 |
| `wj rollback -d demo <previous commit>` | Pass, heading reads "My Todos" | 1.4 seconds |
| Size of the `deployer` container after the first deploy | | 12.8 GB |
| Size of the `vm1` and `vm2` containers after the first deploy | | 167 MB each |

## Differences from the tutorial text

The run started the stack with `TEST_ID=demo` set as well as `COMPOSE_PROJECT_NAME`.
Only the hooks of Kamal's sample apps read `TEST_ID`, and minimal-kamal ships sample
hooks only, so the variable did not affect the run. The traffic probe ran from the
host with `curl`, in parallel with the second deploy.
