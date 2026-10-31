# Building a JSON-RPC service

This guide builds a small JSON-RPC 2.0 service step by step with eider-rpc's
`rpc/jsonrpc` on [eider-http](https://github.com/klxdev/eider-http) (HTTP/1.1 server and
client, router, middleware), on std's sockets, deadlines and thread pool. It was the first
half of eider-lang's "Services" guide; the second half, a REST service with OpenAPI, is
[eider-openapi](https://github.com/klxdev/eider-openapi)'s docs/guide.md.

[`examples/kvstore`](../examples/kvstore/kv.ei) goes further than this page: a key-value
store and a calculator over JSON-RPC, a REST route, graceful shutdown on SIGTERM, the
profiler's labels and endpoints, and an end-to-end test.

> **Note:** The code blocks on this page are marked `eider,ignore`: they import packages
> (`http`, `rpc`, `json`), which a code block alone has no `eider.toml` or `eider.lock` to
> find. Each one was built and run in a package set up as below, and the outputs shown are
> what it printed. examples/kvstore is tested by `scripts/test.sh`.

## What a service is made of

| Piece | Module | |
|---|---|---|
| `http.Request`, `http.Response`, `http.Handler` | `http` | a handler is a plain function value, `fn(http.Request) -> http.Response` |
| the server | `http/server` | `server.listen(addr, cfg)` then `srv.serve(handler, token)`: a pool of worker threads, deadlines on every read and write, limits on heads and bodies, graceful shutdown when the token is cancelled |
| the router | `http/router` | `r.get("/items/:id", h)`, `r.post(...)`; 404 and 405 with `Allow` |
| JSON-RPC 2.0 | `rpc/jsonrpc` (eider-rpc) | a method table, typed parameters, batches and notifications, the HTTP handler and a client |
| OpenAPI | `openapi`, `openapi/schema` (eider-openapi) | one operation value drives routing, validation and the served description |
| middleware | `http/middleware`, `http/metrics`, `http/problem` | request ids, access logs, timeouts, CORS, bearer and Basic auth, metrics, problem+json errors |

The server speaks plain HTTP. Run it on loopback or a private interface behind a
TLS-terminating proxy; TLS 1.3 written in Eider is planned (eider-lang's
`docs/design/roadmap.md`, P1.4).

## A JSON-RPC service, step by step

### 1. The package

```sh
$EIDER/scripts/eider-pkg -C notes init
cd notes
$EIDER/scripts/eider-pkg add http --git https://github.com/klxdev/eider-http.git --version 0.1.0 --uses net,io,clock,threads
$EIDER/scripts/eider-pkg add rpc --git https://github.com/klxdev/eider-rpc.git --version 0.1.0 --uses net,io,clock
```

The `http` package needs `net`, `io`, `clock` and `threads` for its server and client, rpc
`net`, `io` and `clock` for its client calls, and you grant them explicitly (eider-lang's
guide, "Capabilities"):

```toml
[package]
name = "notes"
version = "0.1.0"
main = "main.ei"

[deps.http]
git = "https://github.com/klxdev/eider-http.git"
version = "0.1.0"
uses = ["net", "io", "clock", "threads"]

[deps.rpc]
git = "https://github.com/klxdev/eider-rpc.git"
version = "0.1.0"
uses = ["net", "io", "clock"]
```

To work on checkouts of these repositories instead (offline, or with your own changes),
name them in `EIDER_PKG_OVERRIDE` when you fetch:
`EIDER_PKG_OVERRIDE=http=../eider-http,rpc=../eider-rpc,json=../eider-json eider-pkg fetch`
(eider-lang's docs/site/tooling/eider-pkg.md).

### 2. The state

The server runs handlers on several worker threads at once, so shared state goes in a
lock. `std/sync`'s `Lock[T]` is shared by every copy, and `with(f)` runs `f(state) ->
(new state, result)` as one critical section:

```eider,ignore
// notes.ei
import "std/sync"
import "http"
import "http/router"
import "rpc/jsonrpc"

/// The state, shared by every worker thread: copies of a Lock are the same lock.
pub type Store = { st: sync.Lock[List[List[u8]]] }

pub fn store() -> Store { Store(sync.lock(List.new())) }

impl Store {
    /// Adds a note and returns its id.
    fn add(self, text: List[u8]) -> i64 {
        self.st.with(|notes: List[List[u8]]| -> (List[List[u8]], i64) {
            let mut all = notes.copy()
            all.push(text)
            let id = all.len() as i64
            (all, id)
        })
    }

    /// The note with this id, if there is one.
    fn get(self, id: i64) -> Option[List[u8]] {
        self.st.with(|notes: List[List[u8]]| -> (List[List[u8]], Option[List[u8]]) {
            if id < 1 || id > notes.len() as i64 { return (notes, None) }
            let text = notes[(id - 1) as usize]
            (notes, Some(text))
        })
    }
}
```

Text on the wire is bytes (`List[u8]`), which is what the http and json packages speak
today.

### 3. The methods

A method is `fn(jsonrpc.Params) -> Result[List[u8], jsonrpc.Error]`, its result as JSON
text. `add_typed` composes three functions: one that reads the parameters into a value, the
service logic, and one that writes the result. The logic is then plain values in, a value
or an error out:

```eider,ignore
fn text_arg(p: jsonrpc.Params) -> Result[List[u8], jsonrpc.Error] {
    p.at_most(1)?
    p.str(0, "text")           // positional [text] or named {"text": ...}
}

fn id_arg(p: jsonrpc.Params) -> Result[i64, jsonrpc.Error] {
    p.at_most(1)?
    p.int(0, "id")
}

fn note_or_null(v: Option[List[u8]]) -> List[u8] {
    match v {
        Some(t) => jsonrpc.string(t)
        None => jsonrpc.nothing()
    }
}

/// The JSON-RPC methods: notes.add(text) -> id, notes.get(id) -> text or null.
pub fn rpc(s: Store) -> jsonrpc.Table {
    let mut t = jsonrpc.table()
    t.add_typed("notes.add", text_arg, jsonrpc.int, |text: List[u8]| -> Result[i64, jsonrpc.Error] {
        if text.len() == 0 { return Err(jsonrpc.error(-32000, "a note can't be empty")) }
        Ok(s.add(text))
    })
    t.add_typed("notes.get", id_arg, note_or_null, |id: i64| -> Result[Option[List[u8]], jsonrpc.Error] {
        Ok(s.get(id))
    })
    t
}
```

A missing or wrongly typed parameter is answered with `-32602 Invalid params` and the
reason, before your function runs. The decoders and encoders are written by hand today;
deriving them from the types is planned (JSON derive, eider-lang's roadmap P0.10).

### 4. Routes

```eider,ignore
/// POST /rpc for the methods, GET /health for probes.
pub fn app(s: Store) -> http.Handler {
    let mut r = router.new()
    r.post("/rpc", jsonrpc.handler(rpc(s)))
    r.get("/health", |q: http.Request| -> http.Response { http.text(200, "ok") })
    router.handler(r)
}
```

`jsonrpc.handler` checks the method (`POST`) and content type (`application/json`) and
answers `204` for a request with only notifications. Route patterns take parameters
(`/notes/:id`, read with `q.param("id")`) and a final `*rest`.

### 5. The server

```eider,ignore
// main.ei
import "std/net"
import "std/task"
import "http/server"
import "notes.ei"

fn main() uses io, net, clock, threads {
    let srv = match server.listen(net.ipv4(127, 0, 0, 1, 8080), server.config()) {
        Ok(s) => s
        Err(e) => {
            print(e)
            return
        }
    }
    print("listening on 127.0.0.1:8080")
    let tok = task.root()          // cancel it to shut the server down
    match srv.serve(notes.app(notes.store()), tok) {
        Ok(st) => print(st.requests)
        Err(e) => print(e)
    }
}
```

`serve` blocks until the token is cancelled, then finishes the requests in flight, closes
idle connections and returns the counts. `server.config()` has the defaults (16 workers, a
queue of 64 connections, 5 s idle, 10 s read and write timeouts, 1 MiB bodies); change its
fields before `listen` (`cfg.workers = 4`).

Build it, start it, and call it from another terminal:

```sh
$EIDER/scripts/ei build main.ei -o notes-server
./notes-server
```

```text
$ curl -s localhost:8080/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","method":"notes.add","params":["buy milk"],"id":1}'
{"jsonrpc":"2.0","result":1,"id":1}
$ curl -s localhost:8080/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","method":"notes.get","params":{"id":1},"id":2}'
{"jsonrpc":"2.0","result":"buy milk","id":2}
$ curl -s localhost:8080/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","method":"notes.get","params":[9],"id":3}'
{"jsonrpc":"2.0","result":null,"id":3}
$ curl -s localhost:8080/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","method":"notes.add","params":[""],"id":4}'
{"jsonrpc":"2.0","error":{"code":-32000,"message":"a note can't be empty"},"id":4}
$ curl -s localhost:8080/rpc -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","method":"notes.add","params":[1, 2],"id":5}'
{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params","data":"too many parameters"},"id":5}
```

Batches (a JSON array of calls) and notifications (calls without an `id`, never answered)
work as the specification says.

### 6. Shutting down on a signal

To stop cleanly on SIGTERM or Ctrl-C, watch the signals and cancel the token from a
thread. This is the code from `examples/kvstore/main.ei`:

```eider,ignore
    let tok = task.root()
    signal.watch(signal.TERM).unwrap()
    signal.watch(signal.INT).unwrap()
    let sig_fd = signal.events_fd().unwrap()
    let t = tok
    let waiter = spawn(|| -> i32 {
        let mut going = true
        while going {
            match fd.wait_ready(sig_fd, fd.READY_READ, t.deadline()) {
                Ok(true) => {
                    if signal.pending().unwrap_or(List.new()).len() > 0 { going = false }
                }
                Ok(false) => { going = false }
                Err(e) => { going = false }
            }
        }
        t.cancel()
        0
    })
    // ... srv.serve(handler, tok) as above, then:
    let z = waiter.join()
```

### 7. An end-to-end test

Start the server on port 0 (any free port), call it with `http/client` over loopback,
then cancel the token. No fixed ports and no sleeps, so the test is reliable in parallel
runs:

```eider,ignore
// tests/e2e_test.ei (needs `eider-pkg add json --path ...` too)
import "std/net"
import "std/task"
import "std/time"
import "json"
import "http/server"
import "http/client"
import "rpc/jsonrpc"
import "../notes.ei"

fn show(r: Result[jsonrpc.Reply, String]) uses io {
    match r {
        Ok(jsonrpc.Value(d, n)) => print(String.from_bytes(json.write(d, n)).unwrap())
        Ok(jsonrpc.Fault(e)) => print(e.code)
        Err(e) => print(e)
    }
}

fn main() uses io, net, clock, threads {
    let dl = time.after_ms(10000)
    let srv = server.listen(net.ipv4(127, 0, 0, 1, 0), server.config()).unwrap()
    let addr = srv.addr()
    let tok = task.root()
    let t = tok
    let h = notes.app(notes.store())
    let running = spawn(|| -> server.Stats { srv.serve(h, t).unwrap() })

    let mut c = client.connect(addr, dl).unwrap()
    show(jsonrpc.call(inout c, "/rpc", "notes.add", String.from("[\"buy milk\"]").bytes(), 1, dl))
    show(jsonrpc.call(inout c, "/rpc", "notes.get", String.from("[1]").bytes(), 2, dl))
    show(jsonrpc.call(inout c, "/rpc", "notes.add", String.from("[\"\"]").bytes(), 3, dl))

    tok.cancel()                   // graceful shutdown
    print(running.join().requests)
}
```

```text
1
"buy milk"
-32000
3
```

[`examples/kvstore/tests/e2e_test.ei`](../examples/kvstore/tests/e2e_test.ei) does
the same with batches, notifications, the REST route and four concurrent clients from a
task group.

## Limits to know

- One OS thread serves one connection at a time: `cfg.workers` bounds concurrency, and
  connections beyond the queue get 503. Stackless tasks, which lift this without changing
  `serve`'s signature, are planned.
- A trap in a handler (an overflow, an index out of bounds, `unwrap` on `None`) ends the
  process. Validate input before arithmetic; eider-rpc's examples/kvstore checks for overflow
  before adding. Containing traps is planned.
- A handler that blocks forever holds a worker forever; `middleware.timeout` bounds the
  answer, not the thread. Pass `req.deadline` down to anything that waits.
- Bodies are read whole (up to `Limits.max_body`, 1 MiB by default); streaming bodies are
  planned. Floats aren't in the language yet, so there are no `number` schemas.
- No TLS in the server: put it behind a proxy.

eider-http's `docs/services.md`, eider-rpc's `docs/jsonrpc.md` and eider-openapi's
`docs/openapi.md` describe the architecture, the limits and defaults, and the security
posture in detail.
