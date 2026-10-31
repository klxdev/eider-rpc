# JSON-RPC 2.0 (rpc/jsonrpc)

`rpc/jsonrpc` (also `import "rpc"`) is JSON-RPC 2.0 over HTTP `POST`, on eider-http's
server, router and client and eider-json. It is the first milestone of Eider's RPC work
(eider-lang's docs/design/roadmap.md P1.1; decisions C7 and C47 in eider-lang's
docs/decisions.md). Until eider-lang U19 it was `http/jsonrpc` in eider-lang's
packages/http; this is that module with its history, and the JSON-RPC parts of what is now
eider-http's docs/services.md.

```eider,ignore
import "std/net"
import "std/task"
import "http/server"
import "http/router"
import "rpc/jsonrpc"

fn main() uses io, net, clock, threads {
    let mut rpc = jsonrpc.table()
    rpc.add("math.add", |p: jsonrpc.Params| -> Result[List[u8], jsonrpc.Error] {
        Ok(jsonrpc.int(p.int(0, "a")? + p.int(1, "b")?))
    })
    let mut r = router.new()
    r.post("/rpc", jsonrpc.handler(rpc))
    let srv = server.listen(net.ipv4(127, 0, 0, 1, 8080), server.config()).unwrap()
    let stats = srv.serve(router.handler(r), task.root()).unwrap()
}
```

## The module

A `Table` maps method names to `fn(Params) -> Result[List[u8], Error]` (`jsonrpc.Method`;
the result as JSON text). `Params` reads positional or named parameters with typed accessors
(`int`, `str`, `bool`, `raw`, `none`, `at_most`); a wrong or missing one is `-32602` with
the reason in `data`. `add_typed(name, decode, encode, f)` composes a decoder, a typed
function and an encoder, which is how a typed dispatch table is written until JSON derive
(P0.10) can generate the decoders. The encoders for results are `jsonrpc.int`, `string`,
`boolean`, `array`, `nothing` (null), and `error(code, message)` makes an `Error`.

Everything the specification requires is covered by the spec's own examples in the tests:
parse errors, invalid requests (with or without a readable id), unknown methods,
notifications (never answered), batches (an empty one, all-notification ones, invalid
members). Results are written in the order of the calls (the spec allows any order); the
calls of a batch run one after another on the connection's worker.

Over HTTP (`handler(table)`): `POST` with `Content-Type: application/json` (else 405 /
415), `200` with the response, or `204` when there is nothing to send back. The client side
has `call` and `notify` on an eider-http `client.Conn`, and `parse_reply`, which checks the
version, the id and that there is exactly one of `result` and `error`.

## Limits

| What | Default | Where | Over it |
|---|---|---|---|
| Calls in a batch | 100 | `Table.max_batch` | one -32600 |
| JSON nesting | 512 | eider-json `MAX_DEPTH` | -32700 |
| Body | 1 MiB | eider-http `Limits.max_body` | 413 |

Every other limit (heads, deadlines, workers) is the server's: eider-http's
docs/services.md, "Limits and defaults".

## Capabilities

The module's server side needs nothing; `call` and `notify` need `net, io, clock`. A
program grants rpc `net, io, clock` and http what its server needs (`net, io, clock,
threads`, and `fs` for http/profiling's endpoints): examples/kvstore/eider.toml.

## Testing

`sh scripts/test.sh` runs:

- `tests/jsonrpc_test`: the specification's examples (section 7), typed methods, errors
  with data, the HTTP handler's method and media-type checks, the client's reply checks;
- `examples/kvstore/tests/e2e_test`: the example service on an ephemeral loopback port,
  called with the client (JSON-RPC on one connection, a batch, the REST route, four
  concurrent clients from a task group) and shut down with its token; also once under
  ThreadSanitizer.

eider-profiler's tests build examples/kvstore with `--profile` and put it under load.

## Next

Typed services (P1.1): with JSON derive (P0.10) and `dyn Trait` (P0.7), a service is a
trait whose methods become `add_typed` entries and a client stub, generated from the types
instead of written by hand; then Connect (HTTP + JSON) as a second wire format, and OpenRPC
descriptions of the methods.
