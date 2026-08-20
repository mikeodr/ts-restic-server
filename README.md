# ts-restic-server

`ts-restic-server` serves an embedded [restic](https://restic.net/) [rest-server](https://github.com/restic/rest-server) over Tailscale's tsnet allowing it to appear as a node on your Tailnet. It uses the connecting Tailscale peer's identity for rest-server proxy authentication, so no separate rest-server process or local HTTP proxy is required.

## Requirements

- Go 1.26 or newer
- Tailscale access to the tailnet
- `restic` client v0.7.1 or newer

## Run

Set the auth key and start the gateway:

```sh
export TS_AUTHKEY='tskey-...'
go run ./cmd/rest-server --path ./foobar
```

The gateway listens on Tailscale port `443`. `tsnet` provides the HTTPS certificate for the node hostname. With the default hostname, the service is available at:

```text
https://restic-gw.<tailnet>.ts.net/
```

The default repository data directory mirrors rest-server:

```text
$TMPDIR/restic
```

Use `--path` to select a persistent location. This is recommended because the operating system temporary directory may be removed.

## Use With Restic

```bash
restic -r rest:https://restic-gw.tail-scale.ts.net/my_repo
```

Restic repository data is encrypted by restic. Tailscale provides transport encryption and authenticates the connecting peer before the request reaches rest-server.

## Private Repositories

By default, repository paths are not required to match the Tailscale identity. Enable private repositories with:

```sh
go run ./cmd/rest-server \
  --path ./foobar \
  --private-repos
```

When `--private-repos` is enabled, the first repository path component must exactly match the identity sent in `X-Tailscale-User`. For example, if the gateway identifies the peer as `alice@example.com`, use:

```sh
restic -r rest:https://restic-gw.<tailnet>.ts.net/alice@example.com init
```

To determine your Tailnet username, you can do this via `tailscale whoami`.
This is the `User.Name` field:

```bash
tailscale whoami
Machine:
  Name:          host.tail-scale.ts.net
  ID:            xxxxxxxxxx11CNTRL
  Addresses:     [100.85.32.40/32 fd7a:115c:a1e0::a701:2028/128]
User:
  Name:     name@loginprovider.com # <---- This field 
  ID:       99999999999999999
Tailnet:
  Name:                My Tailnet
  MagicDNS Suffix:     tail-scale.ts.net
  ```

Create a repository at the path that contains your username.

```bash
restic -r rest:https://restic-gw.tail-scale.ts.net/alice@example.com
```

You can create sub repos also:

```bash
restic -r rest:https://restic-gw.tail-scale.ts.net/alice@example.com/other_repo
```

For tagged devices, the gateway uses the node's computed name when no Tailscale login name is available.

## Capability Grants

Use `--capability` to require a Tailscale app capability before a client can access the gateway. The capability name is application-defined; this example uses `example.com/restic-backup`:

```sh
go run ./cmd/rest-server \
  --path ./foobar \
  --hostname restic-gw \
  --capability example.com/restic-backup
```

First assign `tag:restic-server` to the gateway node, either by registering it with a tagged auth key or by assigning the tag in the Tailscale admin console; declaring `tagOwners` below only authorizes that assignment. Then grant the capability to the clients that should be allowed to use the gateway. For example, the following Tailscale ACL policy grants it to members of `group:restic-clients` when connecting to `tag:restic-server`:

```json
{
  "groups": {
    "group:restic-clients": ["alice@example.com"]
  },
  "tagOwners": {
    "tag:restic-server": ["autogroup:admin"]
  },
  "grants": [
    {
      "src": ["group:restic-clients"],
      "dst": ["tag:restic-server"],
      "ip": ["443"],
      "app": {
        "example.com/restic-backup": ["*"]
      }
    }
  ]
}
```

Run restic against the gateway as usual:

```sh
restic -r rest:https://restic-gw.tail-scale.ts.net/my_repo
```

Clients without the capability receive `403 Forbidden`.

## Build

```sh
go build -o rest-server ./cmd/rest-server
./rest-server --path /var/lib/restic
```

The process needs permission to read and write the selected repository directory.
