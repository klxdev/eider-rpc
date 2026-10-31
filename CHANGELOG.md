# Changelog

eider-rpc's versions (semver). Before 1.0 the MAJOR.MINOR is the Eider toolchain's it goes
with (`eider` in eider.toml), the PATCH its own.

## 0.1.0

The first release, for Eider 0.1: eider-lang's `http/jsonrpc` (packages/http) and
`examples/kvstore`, with their history, made a package of their own.

- `rpc/jsonrpc` (also `import "rpc"`): JSON-RPC 2.0 over HTTP on eider-http 0.1.0 and
  eider-json 0.1.0 (git dependencies). The module is unchanged apart from its imports.
- examples/kvstore imports `rpc/jsonrpc`, depends on this package by path and on http and
  json by git.
- docs/jsonrpc.md (the JSON-RPC parts of eider-lang's docs/services.md) and docs/guide.md
  (the JSON-RPC half of eider-lang's Services guide).
- `VERSION`, `eider = "0.1"`, `scripts/build.sh` and `scripts/test.sh` against an eider-lang
  checkout (`EIDER_ROOT`), refusing a toolchain of another MAJOR.MINOR, with
  `EIDER_PKG_OVERRIDE` set for sibling checkouts (scripts/env.sh).
