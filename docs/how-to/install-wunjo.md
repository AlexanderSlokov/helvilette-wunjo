# Install Wunjo

This page shows how to install the `wunjo` command-line tool on Linux or macOS, from a
release binary or from source, and how to check that it runs. `wunjo` is one static
binary of about 9 MB with no runtime to install, so the installation copies one file
into a directory on your `PATH`. The installation worked when `wunjo version` prints
the version number.

## Before you begin

You need the following:

- Linux or macOS on amd64 or arm64.
- The OpenSSH client, `ssh`. Wunjo runs every command on your servers through it.
    Most Linux and macOS machines include it. Check it with `ssh -V`.
- Docker with the Buildx plugin on the machine that builds your images, which is your
    laptop unless `builder/remote` in `config/deploy.yml` names a remote builder.
- To install from source: Git, GNU Make, and Go 1.27.3 or later. Check the Go version
    with `go version`.

## Install a release binary

Each release publishes four binaries and a `SHA256SUMS` file on the
[releases page](https://github.com/AlexanderSlokov/helvilette-wunjo/releases). To install
one of them, do the following:

1. Download the binary for your platform and the checksum file:

    ```shell
    curl -fsSLO https://github.com/AlexanderSlokov/helvilette-wunjo/releases/download/v0.1.0/<binary>
    curl -fsSLO https://github.com/AlexanderSlokov/helvilette-wunjo/releases/download/v0.1.0/SHA256SUMS
    ```

    where `<binary>` is one of `wunjo_linux_amd64`, `wunjo_linux_arm64`,
    `wunjo_darwin_amd64`, or `wunjo_darwin_arm64`. A Mac with Apple silicon uses
    `wunjo_darwin_arm64`.

1. Check the binary against its checksum:

    ```shell
    grep <binary> SHA256SUMS | shasum -a 256 -c
    ```

    The output is similar to this:

    ```console
    wunjo_linux_amd64: OK
    ```

    If the output ends with `FAILED`, delete the file and download it again.

1. Install the binary as `wunjo` into `~/.local/bin`:

    ```shell
    mkdir -p ~/.local/bin
    install -m 0755 <binary> ~/.local/bin/wunjo
    ```

## Install from source

The `make install` target builds a static binary from a checkout and writes it to
`~/.local/bin/wunjo`. To install from source, do the following:

1. Clone the repository at the release tag:

    ```shell
    git clone --branch v0.1.0 https://github.com/AlexanderSlokov/helvilette-wunjo.git
    cd helvilette-wunjo
    ```

1. Build and install the binary:

    ```shell
    make install
    ```

    To install into another directory, set `PREFIX`. For example,
    `make install PREFIX=/usr/local` writes `/usr/local/bin/wunjo`.

## Check the installation

Run the following command:

```shell
wunjo version
```

The output is similar to this:

```console
0.1.0
```

If the shell prints `command not found`, the directory that holds `wunjo` is missing
from your `PATH`. Add `export PATH="$HOME/.local/bin:$PATH"` to your shell's startup
file, such as `~/.bashrc` or `~/.zshrc`, and open a new terminal.

## What's next

- [Deploy a sample app with Wunjo](../tutorials/deploy-a-sample-app.md)
- [Known differences from Kamal](../plans/port-kamal-to-go.md#known-differences-from-kamal)
