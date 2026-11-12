# Deploy a sample app with Wunjo

This tutorial deploys a small to-do app to two servers with `wunjo`, changes its code,
deploys it again, and rolls it back. The servers are Docker containers on your machine,
so you need no cloud account and you break nothing. The app is
[minimal-kamal](https://github.com/tmeire/minimal-kamal), a Go app that its author
deploys with Kamal 2. Wunjo deploys it with the same `config/deploy.yml`, which shows
that a Kamal app moves to Wunjo without changes to its files. You know that the
tutorial worked when the page in your browser shows the new heading after the redeploy
and the old heading after the rollback.

## Objectives

- Start a _test stack_: Docker containers that act as your laptop, your servers, and
    a load balancer.
- Point an existing Kamal app at the test stack with a destination file.
- Deploy the app, deploy a change, and roll the change back with `wunjo`.

## Before you begin

You need the following:

- A Linux machine with Docker Engine and the Docker Compose plugin. The test stack
    runs Docker inside privileged containers. This tutorial was run with Docker 29.9.2
    and Docker Compose 5.7.0.
- About 20 GB of free disk space. After the first deploy, the `deployer` container
    alone holds 12.8 GB of images and build cache, as
    [the session record](../references/sessions/2026-11-12-tutorial-run.md) shows.
- Git, and Go 1.27.3 or later to build `wunjo` for the test stack. Check the Go
    version with `go version`.
- Network access to Docker Hub, GitHub, and the Go module proxy. The first deploy
    downloads the Go toolchain image and the app's modules.

You do not need `wunjo` on your machine. The test stack runs a copy that you build in
the first steps.

## Start the test stack

The test stack is the one that runs Kamal's integration suite in this repository. It
starts the following containers:

| Container | Role |
|---|---|
| `deployer` | Your laptop. It has `wunjo`, Docker for builds, and Git. |
| `vm1`, `vm2`, `vm3` | Three servers, each with an SSH server and Docker. |
| `load_balancer` | An nginx load balancer in front of `vm1` and `vm2`. |
| `registry` | A private image registry for Kamal's integration suite. This tutorial does not use it. |
| `hub-cache` | A cache of Docker Hub images for the servers. |
| `otel_collector` | A log collector for Kamal's integration suite. |
| `shared` | Generates the SSH key and the registry certificate for the others. |

To start the stack, do the following:

1. Clone this repository and build the Linux `wunjo` binary that the `deployer`
    container runs:

    ```shell
    git clone --branch v0.1.0 https://github.com/AlexanderSlokov/helvilette-wunjo.git
    cd helvilette-wunjo
    CGO_ENABLED=0 go build -o bin/wunjo-integration ./cmd/wunjo
    ```

1. Name the Compose project, so the stack stays apart from a run of the integration
    suite:

    ```shell
    cd test/integration
    export COMPOSE_PROJECT_NAME=wunjo-demo
    ```

1. Build the images and start the stack:

    ```shell
    docker compose build
    docker compose up -d --no-build --wait
    ```

    The `--wait` flag returns when every container passes its health check. The first
    build takes a few minutes.

1. Prepare the `deployer` container:

    ```shell
    docker compose exec -T --workdir / deployer ./setup.sh
    ```

1. Define two shell functions for the rest of the tutorial. The `dep` function runs a
    command in the app's directory on `deployer`, and `wj` runs `wunjo` there:

    ```shell
    dep() { docker compose exec -T --workdir /minimal-kamal deployer "$@"; }
    wj()  { dep wunjo "$@"; }
    ```

    The functions exist only in the current terminal. Run each later command from the
    `test/integration` directory in that terminal.

## Point the app at the test stack

The app's `config/deploy.yml` names its author's server, a public hostname, and a
Google Cloud vault for its secrets. A _destination_ is a file named
`config/deploy.<name>.yml` that overrides parts of `config/deploy.yml` for one
environment. You select it with `-d <name>`. To add a destination named `demo`, do the
following:

1. Clone the app into `deployer` at the commit that this tutorial uses:

    ```shell
    docker compose exec -T deployer git clone -q https://github.com/tmeire/minimal-kamal.git /minimal-kamal
    dep git checkout -q 20294ecdb8395918290d12519778f9e78f76fe99
    ```

1. Create the file `config/deploy.demo.yml`. It sends the app to `vm1` and `vm2` as
    `root`, serves plain HTTP for the hostname `localhost`, and keeps the data under
    `/root`:

    ```shell
    dep sh -c 'cat > config/deploy.demo.yml' <<'EOF'
    servers:
      web:
        - vm1
        - vm2
    ssh:
      user: root
    proxy:
      ssl: false
      hosts:
        - localhost
    volumes:
      - "/root/minimal-kamal/storage:/app/data:rw"
    EOF
    ```

1. Create the file `.kamal/secrets.demo` with test values. With `-d demo`, Wunjo reads
    this file in place of `.kamal/secrets`, so it does not call the Google Cloud vault:

    ```shell
    dep sh -c 'printf "SMTP_PASS=demo\nLITESTREAM_APPLICATION_SECRET={}\n" > .kamal/secrets.demo'
    ```

1. Commit both files. Wunjo tags each image with the Git commit, as Kamal does:

    ```shell
    dep git add -A
    dep git -c user.email=me@example.com -c user.name=Me commit -qm "Add the demo destination"
    ```

1. Print the configuration that a deploy to `demo` uses:

    ```shell
    wj config -d demo | head -8
    ```

    The output is similar to this:

    ```console
    ---
    :roles:
    - web
    :hosts:
    - vm1
    - vm2
    :primary_host: vm1
    :version: 55a3af8f4ffa4df839f1368a3009f64fa7072e75
    ```

    The step worked when `:hosts:` lists `vm1` and `vm2`.

## Deploy the app

The app runs as the user with ID 65532, so that user must own the data directory on
each server. To deploy the app, do the following:

1. Create the data directory on both servers:

    ```shell
    wj server exec -d demo "mkdir -p /root/minimal-kamal/storage && chown -R 65532:65532 /root/minimal-kamal/storage"
    ```

1. Deploy:

    ```shell
    wj deploy -d demo
    ```

    The app sets `registry/server` to `localhost:5555`, so Wunjo starts a registry on
    `deployer` and forwards its port to each server over SSH, as Kamal 2 does. Wunjo
    then builds the image, pushes it, starts `kamal-proxy` on `vm1` and `vm2`, and
    starts the app behind it. The first deploy took 777 seconds at commit `e8f5601`,
    most of it to download the Go toolchain image and the app's modules, as
    [the session record](../references/sessions/2026-11-12-tutorial-run.md)
    shows. The output ends with lines similar to this:

    ```console
    Prune old containers and images...
    ...
    Releasing the deploy lock...
      Finished all in 777.1 seconds
    ```

    The deploy worked when the last line reads `Finished all` and no line starts with
    `ERROR`.

1. Find the port of the load balancer on your machine:

    ```shell
    docker compose port load_balancer 80
    ```

    The output is similar to this:

    ```console
    0.0.0.0:32768
    ```

1. Open `http://localhost:<port>` in your browser, where `<port>` is the number after
    the colon. The page shows the heading **My Todos**. Add two or three to-do items.

## Deploy a change

To change the heading of the page and deploy the change, do the following:

1. Edit the template and commit the change:

    ```shell
    dep sed -i 's|<h1>My Todos</h1>|<h1>Things to do</h1>|' templates/index.html
    dep git -c user.email=me@example.com -c user.name=Me commit -qam "Rename the heading"
    ```

1. Deploy again, and reload the page in your browser while the deploy runs:

    ```shell
    wj deploy -d demo
    ```

    The page keeps loading during the deploy, because `kamal-proxy` sends traffic to
    the new container only after its health check at `/up` passes. In the run of the
    session record, this deploy took 161 seconds, and the 741 requests to the page
    during the deploy all returned HTTP status 200.

1. Reload the page. The heading reads **Things to do**, and your to-do items are still
    there, because the data lives in the directory that you created on the server.

## Roll back the change

Wunjo keeps the previous container on each server, so a rollback starts it again. To
roll back the change, do the following:

1. List the app containers on each server:

    ```shell
    wj app containers -d demo
    ```

    The output shows one container per version, similar to this:

    ```console
    App Host: vm1
    CONTAINER ID   IMAGE                                                     ...   STATUS                    NAMES
    1114b383fd64   localhost:5555/digitalpowertools/minimal-kamal:12038178   ...   Up 2 seconds              minimal-kamal-web-demo-12038178...
    f52f6768c144   localhost:5555/digitalpowertools/minimal-kamal:55a3af8f   ...   Exited (2) 1 second ago   minimal-kamal-web-demo-55a3af8f...
    ```

1. Roll back to the version before your change:

    ```shell
    wj rollback -d demo "$(dep git rev-parse HEAD~1)"
    ```

    The rollback took 1.4 seconds in the run of the session record, because the image
    is already on each server.

1. Reload the page. The heading reads **My Todos** again.

> [!NOTE]
> The app stores its data in SQLite, a database in one file, and is made for one server.
> Each of `vm1` and `vm2` keeps its own file, so the two servers hold different to-do
> lists. If an item seems to disappear when you reload the page, the load balancer sent
> the request to the other server.

## Clean up

To stop the test stack and delete its containers, run the following command from the
`test/integration` directory:

```shell
docker compose down -t 0
```

To also delete the stack's volumes and images, run
`docker compose down -t 0 --volumes --rmi local`.

## What's next

- [Install Wunjo](../how-to/install-wunjo.md) on your laptop to deploy to your own
    servers.
- List the other commands with `wunjo help`, for example `wunjo details`,
    `wunjo app logs`, and `wunjo audit`.
- [Known differences from Kamal](../plans/port-kamal-to-go.md#known-differences-from-kamal)
- [RFC 0001: Why Helvilette Wunjo exists](../explanation/rfc-0001-why-helvilette-wunjo.md)
