# eider-rpc

RPC services for [Eider](https://github.com/klxdev/eider-lang), on
[eider-http](https://github.com/klxdev/eider-http) and
[eider-json](https://github.com/klxdev/eider-json). Today that is JSON-RPC 2.0 over HTTP
`POST` (`rpc/jsonrpc`): a dispatch table, typed methods (decoder, function, encoder),
batches and notifications as the specification says, the HTTP handler and a client.
Typed services generated from Eider types are the next step (eider-lang's roadmap P1.1).

```eider,ignore
import rpc.jsonrpc

let mut t = jsonrpc.table()
t.add("math.add", |p: jsonrpc.Params| -> Result[List[u8], jsonrpc.Error] {
    Ok(jsonrpc.int(p.int(0, "a")? + p.int(1, "b")?))
})
r.post("/rpc", jsonrpc.handler(t))      // r: an http/router Router
```

- [docs/jsonrpc.md](docs/jsonrpc.md): the module, its limits and tests.
- [docs/guide.md](docs/guide.md): a JSON-RPC service step by step.
- [examples/kvstore](examples/kvstore): a key-value store and a calculator over JSON-RPC, a
  REST route, graceful shutdown on SIGTERM, the profiler's labels and endpoints, with an
  end-to-end test (eider-profiler's load test uses it too).

## Using it

```toml
[deps.rpc]
git = "https://github.com/klxdev/eider-rpc.git"
version = "0.1.0"
uses = ["net", "io", "clock"]                 # for the client calls

[deps.http]
git = "https://github.com/klxdev/eider-http.git"
version = "0.1.0"
uses = ["net", "io", "clock", "threads"]      # the server
```

then `eider-pkg fetch` (eider-lang's `scripts/eider-pkg`) and `import rpc.jsonrpc`.

## Developing

```sh
sh scripts/build.sh     # fetch the dependencies, check every module
sh scripts/test.sh      # build, then tests/*.ei and examples/*/tests/*.ei against their .out files
```

The toolchain is the eider-lang checkout `$EIDER_ROOT` (default `../eider-lang`); its
`VERSION` must have this package's MAJOR.MINOR (the `eider` field of `eider.toml`), else the
scripts stop with exit 2 and say which release to use.

The dependencies are git packages. To build offline or against checkouts with uncommitted
changes, eider-pkg's `EIDER_PKG_OVERRIDE` (`name=dir,...`) takes them from local
directories; `scripts/env.sh` sets it for you for http and json: `$EIDER_HTTP` /
`$EIDER_JSON`, else the sibling checkouts `../eider-http` and `../eider-json`, else (json) the
toolchain's pinned copy `$EIDER_ROOT/vendor/json`. eider.lock keeps the git sources either
way (a library doesn't commit its lock).

eider-lang's `scripts/verify.sh` builds and tests this repository against its own checkout
when it is next to it (`scripts/check-tools.sh`).

## Versions

`VERSION` and `eider.toml` hold this package's version (semver). Before 1.0 its MAJOR.MINOR
follows the Eider toolchain it goes with (`eider = "0.1"`) and its PATCH is its own;
CHANGELOG.md lists the releases. A release is tagged `v<version>`.
