# Changelog

eider-rpc's versions (semver). Before 1.0 the MAJOR.MINOR is the Eider toolchain's it goes
with (`eider` in eider.toml), the PATCH its own.

## 0.6.0

For Eider 0.6 (`eider = "0.6"`). Requires eider-json 0.6.0 (`std/serial` support).

- Method parameters and results are `serial.Deserialize` and `serial.Serialize` types (`@[derive(Serialize, Deserialize)]`), not `json.Decode` and `json.Encode`. A bad parameter is `-32602` with the serializer's path and message (`amountCents: expected a u32, the number is out of range`; no `$.` prefix).

## 0.5.0

For Eider 0.5 (`eider = "0.5"`). Requires eider-http and eider-json 0.5.0. rpc/service is partial: eider-rpc#1 stays open (the per-method registration and stub are hand-written until trait reflection, eider-lang#154; Connect is not done).

- rpc/service (eider-rpc#1, partly): typed methods over eider-json's derived codecs
  (`Registry.add_json`, `add_value`, `add_none`; named or positional parameters mapped onto a
  record), services as traits with `Service`/`mount`, and `Remote`, a client that can implement
  the same trait. The per-method registration and stub are hand-written until trait
  reflection exists (eider-lang#154); Connect is not done.
- `Table.add_bytes`.

## 0.4.0

For Eider 0.4 (`eider = "0.4"`). Requires eider-http and eider-json 0.4.0. Nothing else changed.

## 0.3.0

For Eider 0.3 (`eider = "0.3"`): every plain string literal interpolates (eider-lang U29), so
the literals with braces here are raw strings or have their braces doubled. Requires eider-http and eider-json 0.3.0.

- examples/kvstore uses eider-json's derived codecs (eider-json 0.3.0): `math.add` and
  `math.div` decode named parameters with a derived decoder (a member other than `a` and `b`
  is "Invalid params", with its path in the data), and the new `kv.stats` writes its result
  with a derived encoder.

## 0.2.0

For Eider 0.2 (`eider = "0.2"`): eider-lang's 0.2 builds programs only through LLVM (clang
17 or newer; U23) and bootstraps the compiler from a WebAssembly seed (seed/eiderc.wasm; U24).
scripts/env.sh recognises such a checkout (seed/eiderc.wasm, or seed/eiderc.c before).
Nothing else changed.

## 0.1.3

The scripts find the Eider toolchain through `eider` (eider-lang#133) instead of a fixed
eider-lang checkout: scripts/env.sh takes `$EIDER_TOOLCHAIN` (a toolchain directory or an
eider-lang checkout; eider-lang's scripts/check-tools.sh sets it to its checkout), else the
toolchain `eider toolchain show --json` names for this repository's `eider = "0.1"` (the
newest installed 0.1 release), else stops with exit 2 and says to install `eider`
(install.sh). eiderc and eider-pkg run through `eider` with that toolchain (`eider eiderc`,
`eider eider-pkg`), or directly when `eider` is not on the PATH. `EIDER_ROOT` and
`EIDER_LANG` no longer choose the toolchain, and `../eider-lang` is no longer the default.
The pinned `.vendor/<name>` fallback for a dependency applies when the toolchain is an
eider-lang checkout. Tested with the released Eider 0.1.0 and with eider-lang main. No
change to the library's code.

## 0.1.2

scripts/env.sh takes a dependency that has no `EIDER_<NAME>` and no sibling checkout from the
toolchain's pinned release in `$EIDER_ROOT/.vendor/<name>` when eider-lang's `vendor.lock`
pins it (its `scripts/vendor.sh fetch <name>` fetches and hash-checks it on first use), else
from its repository. The toolchain's old `vendor/` is no longer looked at (eider-lang #130 and
#124; no backwards compatibility). No change to the library's code.

Needs an Eider 0.1 toolchain; the pinned copy needs one with `vendor.lock` (eider-lang #122),
and without one a dependency comes from a sibling checkout or the network.

## 0.1.1

Imports in the dotted form (eider-lang C8.26, issue #91): `import std.bytes`, `import json`,
`import http.client`, and this package's module as `import rpc.jsonrpc` (instead of
`import "rpc/jsonrpc"` or `"../jsonrpc.ei"`), in the code, the tests, examples/kvstore
(`import kvstore.kv`), the README, docs/guide.md, docs/jsonrpc.md and the program
scripts/build.sh generates to check every module.

Needs an Eider 0.1 toolchain whose compiler knows dotted imports (the eider-lang commit after
the next seed); an older 0.1 compiler stops at the first dotted import (E0001).

Also since 0.1.0: examples/kvstore changes its store in place with `sync.Lock.modify`
(eider-lang C320, issue #90).

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
- `scripts/version.sh` (`check`, `sync`): eider.toml agrees with VERSION; eider-lang's
  `scripts/version.sh check-siblings` and `bump` drive it.
