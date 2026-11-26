# gRPC and Connect (`rpc/def`, `rpc/grpc`, `rpc/connect`, `rpc/gen`)

One service definition, served over gRPC (HTTP/2), Connect (HTTP/1.1 or HTTP/2, unary) and JSON-RPC 2.0, on eider-http's HTTP/2 (`http/h2`) and eider-proto's messages. Nothing here needs another language to build or test; the interop checks against grpcurl, grpc-go and curl are optional (`tests/grpc_interop.sh`).

## Define a service

From a `.proto` file (the usual way): generate the messages and the stubs,

```sh
eider run examples/protoc_grpc/main.ei <out dir> <include dir> <strip prefix|-> <package prefix|-> api/control.proto
```

which writes, for every file the root imports, the message module (eider-proto: records with `@[derive(Serialize, Deserialize)]`, a `registry()`) and, for files with services, `<module>_grpc.ei`: `ControlHandlers` (one function per rpc), `control_service(reg, handlers) -> def.Service` and `ControlClient` / `control_client(channel, reg)`.

By hand, with `rpc/def`:

```eider
let svc = def.service("demo.Greeter")
    .unary("SayHello", def.codec[pb.HelloRequest](reg, "demo.HelloRequest"), def.codec[pb.HelloReply](reg, "demo.HelloReply"),
           |c: def.Ctx, q: pb.HelloRequest| -> Result[pb.HelloReply, def.Status] { ... })
    .bidi("Chat", qc, rc, |c: def.Ctx, inp: def.In[pb.HelloRequest], out: def.Out[pb.HelloReply]| -> Result[(), def.Status] {
        while true {
            match inp.recv()? {
                Some(m) => { out.send(reply(m))? }
                None => { break }
            }
        }
        Ok(())
    })
```

The four kinds: `unary`, `server_stream`, `client_stream`, `bidi`. A handler gets a `def.Ctx` (request metadata, `deadline`, `set_header`, `set_trailer`) and fails with `Err(def.status(def.NOT_FOUND, "..."))`.

## Serve it

```eider
// one port: gRPC, Connect and ordinary REST routes
let mut cfg = server.config()
cfg.h2 = Some(h2.options())                       // HTTP/2 on the HTTP/1.1 listener (off by default)
cfg.h2_wrap = Some(grpc.middleware(services))     // application/grpc streams are gRPC, the rest goes to the router
r.post("/demo.Greeter/:method", connect.handler(services))   // Connect
// over TLS: secure.server_from_files(...).with_h2() offers ALPN h2
```

A server that speaks only gRPC: `h2.serve(wire, h2.options(), grpc.handler(services))` on any connected duplex, or `h2.serve_listener`. The JSON-RPC form: `def.mount_jsonrpc(inout table, svc)` mounts the unary methods as `demo.Greeter.SayHello`.

What the server does as the protocol says: 5-byte message framing; `grpc-status` / `grpc-message` (percent-encoded) / `grpc-status-details-bin` in the trailers, or in the headers alone for a call that fails before its first message; `UNIMPLEMENTED` for an unknown service or method; `grpc-timeout` is the deadline (reads and writes stop at it: `DEADLINE_EXCEEDED`); metadata, `-bin` keys base64; a reset stream is `CANCELLED`; a trap in a handler is `INTERNAL`; a message over 4 MiB is `RESOURCE_EXHAUSTED`. **Compression** is gzip, per message, off unless asked: a client with `grpc.call_opts().with_compression()`, a handler with `ctx.set_compression(true)` (used when the caller's `grpc-accept-encoding` has gzip); a compressed message is always understood, bounded by the receive limit and eider-compression's ratio limit (a gzip bomb is `RESOURCE_EXHAUSTED`), another `grpc-encoding` is `UNIMPLEMENTED`.

## Call it

```eider
let h2c = h2.dial("host", 50051, h2.options(), dl).unwrap()       // or secure.connect_h2, or h2.connect over any duplex
let ch = grpc.channel(h2c, "host:50051")
let r = grpc.unary(ch, "/demo.Greeter/SayHello", qc, rc, req, grpc.call_opts().with_timeout(time.seconds(5)))
let call = grpc.bidi(ch, "/demo.Greeter/Chat", qc, rc, grpc.call_opts()).unwrap()      // call.send / recv / close_send / cancel / done
```

With the generated client: `ctlg.control_client(ch, reg).info(req, opts)`; server-streaming methods return a `grpc.Call` to `recv` from, client-streaming and bidirectional ones a `grpc.Call` to `send` and `recv` on. A status is `Err(def.Status)`; headers and trailers of the response are `call.header_metadata()` and `call.trailer_metadata()`.

### A channel that survives its connection

`grpc.dial_channel(dial, authority)` makes the connection with `dial` (any function that returns an `h2.Client`) and makes a new one whenever the connection has ended, so a restarted server costs a call at most a retry. `channel.with_retry(attempts, initial, max)` repeats a unary call that failed `UNAVAILABLE` (doubling the wait, within the call's own timeout). `grpc.keepalive(channel, interval, timeout)` pings the connection and closes it when a ping is not answered, so a silent peer is noticed. `channel.close()` ends the connection.

## Connect

`connect.handler(services)` serves unary calls (`application/json`, `application/proto`) over any HTTP version and server-streaming methods (`application/connect+json` / `+proto`: one request envelope in, envelopes out, then an end-stream envelope carrying the error and trailing metadata) over HTTP/1.1. `connect.middleware(services)` (for `server.Config.h2_wrap`, next to `grpc.middleware`) serves all four kinds, bidirectional included, over HTTP/2. Envelope compression is not supported; the JSON of a message leaves out the `unknown_fields` bookkeeping member.

## Docker's BuildKit endpoints

`/grpc` and `/session` upgrade an HTTP/1.1 request (`Connection: Upgrade`, `Upgrade: h2c`) and then speak HTTP/2 on the hijacked connection, with the roles reversed on `/session`: the daemon is the **gRPC server** on `/grpc` (`h2.accept_server(req, opts, grpc.handler(services))`) and the gRPC **client** on `/session` (`h2.accept_client(req, opts, |c| { let ch = grpc.channel(c, "..."); ... })`), calling the Docker CLI, which serves file sync, auth and secrets over the connection it made. `tests/grpc_upgrade_test.ei` plays both, and `tests/control_test.ei` serves BuildKit's real Control service (its generated messages and stubs) and calls it.

## Tests

Eider only: `tests/grpc_test` (all four kinds, statuses and trailers, metadata, deadlines, cancellation, Connect and JSON-RPC from the same service), `tests/grpc_wire_test` (raw HTTP/2 requests and the exact bytes, heads and trailers back), `tests/grpc_upgrade_test`, `tests/stubs_test`, `tests/control_test`, `tests/grpc_resilience_test` (reconnect, retry, keepalive), `tests/connect_stream_test` (Connect streaming by hand). Optional (`tests/grpc_interop.sh`; `skip` without the tool): grpcurl (plain and TLS), a Go client and a Go server (grpc-go, `tests/grpc_go`), curl for Connect.

Not here: server reflection, load balancing and name resolution, the gRPC-Web and Connect GET variants, Connect envelope compression, zstd for gRPC.
