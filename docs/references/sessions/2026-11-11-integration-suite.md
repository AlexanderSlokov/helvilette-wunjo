# Session record: integration suite on 2026-11-11

This record holds the results of the ported integration suite in `test/integration/`,
which deploys Kamal's sample apps to Docker hosts in containers with the `wunjo`
binary. The suite is the done-when condition of backlog task WJ-080.

## Run

The run used the following code, command, and host:

| Item | Value |
|---|---|
| Code | Commit `2dfa0f6`. The working tree matched it for all code outside unit tests. |
| Command | `cd test/integration && go test -tags integration -count=1 -timeout 150m -v .` |
| Host | Linux 7.0.0-42-generic, x86_64, 8 CPUs, 13 GB of memory |
| Tools | Docker 29.9.2, Docker Compose 5.7.0, Go 1.27.4 |
| Total time | 454 seconds, with the Compose images already built |

## Results

Each test starts its own Compose stack. The time of each test includes starting the
stack.

| Test | Ruby source | Result | Seconds |
|---|---|---|---|
| `TestAccessoryBootStopStartRestartLogsRemove` | `accessory_test.rb` | Pass | 34.3 |
| `TestProxiedAccessoryBootStopStartRestartRemove` | `accessory_test.rb` | Pass | 47.4 |
| `TestAppStopStartBootLogsImagesContainersExecRemove` | `app_test.rb` | Pass | 31.2 |
| `TestAppParallelRoles` | `app_test.rb` | Pass | 25.0 |
| `TestAppCustomErrorPages` | `app_test.rb` | Pass | 27.1 |
| `TestLockAcquireReleaseStatus` | `lock_test.rb` | Pass | 17.2 |
| `TestDeployRedeployRollbackDetailsAndAudit` | `main_test.rb` | Pass | 41.2 |
| `TestAppWithRoles` | `main_test.rb` | Pass | 27.9 |
| `TestConfig` | `main_test.rb` | Pass | 6.7 |
| `TestAliases` | `main_test.rb` | Pass | 20.1 |
| `TestDeployAndPruneWithDestinations` | `main_test.rb` | Pass | 18.7 |
| `TestSetupAndRemove` | `main_test.rb` | Pass | 30.9 |
| `TestTwoApps` | `main_test.rb` | Pass | 32.6 |
| `TestDeployWithTraefik` | `main_test.rb` | Pass | 21.0 |
| `TestDeployWithACustomCertificate` | `main_test.rb` | Pass | 17.8 |
| `TestProxyBootRebootStopStartRestartLogsRemove` | `proxy_test.rb` | Pass | 23.9 |
| `TestDeployingABadImage` | `broken_deploy_test.rb` | Pass | 24.7 |

## Earlier run

An earlier run on the same host, before commit `ee89cff`, passed 8 of the 17 tests
in 549 seconds. Its failures came from four differences from Kamal, which commit
`ee89cff` fixes with a unit test each, and from the test client, which rejected the
custom certificate because it names the host only in its Common Name.
