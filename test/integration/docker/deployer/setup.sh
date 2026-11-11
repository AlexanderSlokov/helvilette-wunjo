#!/bin/bash

# Copied from test/integration/docker/deployer/setup.sh at Kamal 2.12.0. The Go test
# harness builds the binary before Compose starts, so this script checks it in
# place of installing the Kamal gem.
set -euo pipefail

test -x /wunjo/bin/wunjo-integration || { echo "missing /wunjo/bin/wunjo-integration: run the Go harness" >&2; exit 1; }

# Seed the private registry with the proxy image (fetched once via the hub_cache
# mirror) so the proxy.run.registry option stays exercised by the suite — see
# app_with_roles, which pulls its proxy from registry:4443 rather than the Hub.
docker pull basecamp/kamal-proxy:v0.9.2
docker tag basecamp/kamal-proxy:v0.9.2 registry:4443/basecamp/kamal-proxy:v0.9.2
docker push registry:4443/basecamp/kamal-proxy:v0.9.2

# .ssh is on a shared volume that persists between runs. Clean it up as the
# churn of temporary vm IPs can eventually create conflicts.
rm -f /root/.ssh/known_hosts
