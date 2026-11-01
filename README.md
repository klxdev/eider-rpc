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

then `eider fetch` and `import rpc.jsonrpc`.

## Developing

```sh
sh scripts/build.sh     # fetch the dependencies, check every module
sh scripts/test.sh      # build, then tests/*.ei and examples/*/tests/*.ei against their .out files
EIDER_TOOLCHAIN=../eider-lang sh scripts/test.sh   # the same, against an eider-lang checkout
```

The scripts use the toolchain [`eider`](https://github.com/klxdev/eider-lang/blob/main/docs/site/tooling/eider.md)
picks: `$EIDER_TOOLCHAIN` (an installed toolchain's directory, or an eider-lang checkout for
unreleased work), else the installed release for `eider.toml`'s `eider = "0.1"` (what
`eider toolchain show` prints here). Install `eider` and the latest toolchain with
eider-lang's `install.sh --toolchain` (from its latest release); without either, the
scripts stop with exit 2 and say so. The toolchain's `VERSION` must have this package's
MAJOR.MINOR (the `eider` field of `eider.toml`), else they stop with exit 2 and say which
release to use. eiderc and eider-pkg run through `eider` (`eider eiderc ...`, `eider eider-pkg
...`, with that toolchain), or directly when `eider` is not on the PATH.

The dependencies are git packages. To build offline or against checkouts with uncommitted
changes, eider-pkg's `EIDER_PKG_OVERRIDE` (`name=dir,...`) takes them from local
directories; `scripts/env.sh` sets it for you for http and json: `$EIDER_HTTP` /
`$EIDER_JSON`, else the sibling checkouts `../eider-http` and `../eider-json`, else (json) the
pinned release `.vendor/json` of an eider-lang checkout used as the toolchain (fetched there
and hash-checked on first use by its `scripts/vendor.sh`), else their repositories (network). eider.lock keeps the git sources either
way (a library doesn't commit its lock).

eider-lang's `scripts/verify.sh` builds and tests this repository against its own checkout
when it is next to it (`scripts/check-tools.sh`).

## Versions

`VERSION` and `eider.toml` hold this package's version (semver). Before 1.0 its MAJOR.MINOR
follows the Eider toolchain it goes with (`eider = "0.1"`) and its PATCH is its own;
CHANGELOG.md lists the releases. A release is tagged `v<version>`.
