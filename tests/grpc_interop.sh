#!/bin/sh
# gRPC and Connect against other implementations, over loopback:
#
#   sh tests/grpc_interop.sh <dir with the built grpcsrv and grpccli>
#
#   grpcurl (grpc-go)    unary, server-, client- and bidirectional streaming, an error status, metadata, against the Eider
#                        server, also over TLS (ALPN h2)               [$GRPCURL or on the PATH]
#   a Go client          (tests/grpc_go/client, grpc-go) against the Eider server: 14 cases
#   a Go server          (tests/grpc_go/server) for the Eider client: 13 cases    [`go` on the PATH; builds with the network]
#   curl                 Connect, JSON and protobuf, over HTTP/1.1 and HTTP/2, on the same port as gRPC
#
# Prints `ok   grpc interop: <case>` or `FAIL ...`; `skip` for a tool that is not installed; exits 1 on a failure. The
# Eider-only tests (tests/grpc_test.ei, tests/grpc_wire_test.ei) need none of these.
here=$(cd "$(dirname "$0")/.." && pwd -P)
bin=$1
D=$here/tests/demo
PKI=$here/../eider-http/tests/pki
[ -x "$bin/grpcsrv" ] && [ -x "$bin/grpccli" ] || { echo "FAIL grpc interop: $bin has no grpcsrv and grpccli"; exit 1; }
T=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$T"' EXIT
fail=0

check() { # name, output file, pattern...
  name=$1; out=$2; shift 2
  for pat in "$@"; do
    if ! grep -q -- "$pat" "$out"; then
      echo "FAIL grpc interop: $name (missing: $pat)"; sed 's/^/    | /' "$out" | head -20; fail=1; return
    fi
  done
  echo "ok   grpc interop: $name"
}

wait_for() { # file, pattern
  i=0
  while [ $i -lt 100 ]; do
    grep -q -- "$2" "$1" 2>/dev/null && return 0
    sleep 0.1; i=$((i + 1))
  done
  return 1
}

if [ -f "$PKI/ec.pem" ]; then "$bin/grpcsrv" "$PKI/ec.pem" "$PKI/ec.key" > "$T/tls.log" 2>&1 & else "$bin/grpcsrv" > "$T/tls.log" 2>&1 & fi
"$bin/grpcsrv" > "$T/srv.log" 2>&1 &
wait_for "$T/srv.log" listening && wait_for "$T/tls.log" listening || { echo "FAIL grpc interop: the Eider server did not start"; cat "$T/srv.log" "$T/tls.log"; exit 1; }
port=$(sed -n 's/^listening //p' "$T/srv.log" | head -1)
tport=$(sed -n 's/^listening //p' "$T/tls.log" | head -1)

GRPCURL=${GRPCURL:-$(command -v grpcurl 2>/dev/null)}
if [ -n "$GRPCURL" ] && [ -x "$GRPCURL" ]; then
  G="$GRPCURL -plaintext -import-path $D -proto demo.proto 127.0.0.1:$port"
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -d '{"name":"grpcurl"}' 127.0.0.1:$port demo.Greeter/SayHello > "$T/o" 2>&1
  check "grpcurl: unary" "$T/o" "Hello, grpcurl"
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -d '{"n":3}' 127.0.0.1:$port demo.Greeter/Count > "$T/o" 2>&1
  check "grpcurl: server streaming" "$T/o" '"n": "1"' '"n": "2"' '"n": "3"'
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -d '{"n":1} {"n":2} {"n":39}' 127.0.0.1:$port demo.Greeter/Sum > "$T/o" 2>&1
  check "grpcurl: client streaming" "$T/o" '"n": "42"'
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -d '{"name":"a"} {"name":"b"}' 127.0.0.1:$port demo.Greeter/Chat > "$T/o" 2>&1
  check "grpcurl: bidirectional streaming" "$T/o" "echo: a" "echo: b"
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -d '{"name":"zed"}' 127.0.0.1:$port demo.Greeter/Fail > "$T/o" 2>&1
  check "grpcurl: error status" "$T/o" "Code: NotFound" "Message: no such user: zed"
  $GRPCURL -plaintext -import-path "$D" -proto demo.proto -H 'x-token: tok1' -H 'x-raw-bin: AAECAw==' -d '{"name":"m"}' -v 127.0.0.1:$port demo.Greeter/Meta > "$T/o" 2>&1
  check "grpcurl: metadata, headers and trailers" "$T/o" "token=tok1 raw=4" "x-seen: tok1" "x-done: 1"
  if [ -f "$PKI/root.pem" ]; then
    $GRPCURL -cacert "$PKI/root.pem" -authority localhost -import-path "$D" -proto demo.proto -d '{"name":"tls"}' 127.0.0.1:$tport demo.Greeter/SayHello > "$T/o" 2>&1
    check "grpcurl over TLS (ALPN h2)" "$T/o" "Hello, tls"
    $GRPCURL -cacert "$PKI/root.pem" -authority localhost -import-path "$D" -proto demo.proto -d '{"name":"a"} {"name":"b"}' 127.0.0.1:$tport demo.Greeter/Chat > "$T/o" 2>&1
    check "grpcurl over TLS: bidirectional streaming" "$T/o" "echo: a" "echo: b"
  fi
else
  echo "skip grpc interop: grpcurl is not installed (set GRPCURL=/path/to/grpcurl; go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest)"
fi

if command -v curl >/dev/null 2>&1; then
  curl -sS --max-time 20 -H 'Content-Type: application/json' -d '{"name":"curl"}' "http://127.0.0.1:$port/demo.Greeter/SayHello" > "$T/o" 2>&1
  check "Connect (curl, HTTP/1.1, JSON)" "$T/o" '"message":"Hello, curl"'
  if curl -V | grep -q nghttp2; then
    curl -sS --max-time 20 --http2-prior-knowledge -H 'Content-Type: application/json' -d '{"name":"h2"}' "http://127.0.0.1:$port/demo.Greeter/SayHello" > "$T/o" 2>&1
    check "Connect (curl, HTTP/2, JSON)" "$T/o" '"message":"Hello, h2"'
  fi
  curl -sS --max-time 20 -H 'Content-Type: application/json' -d '{"name":"zed"}' -w '\nstatus=%{http_code}\n' "http://127.0.0.1:$port/demo.Greeter/Fail" > "$T/o" 2>&1
  check "Connect: an error is its HTTP status and a JSON body" "$T/o" '"code":"not_found"' "status=404"
  printf '\n\005world' | head -c 0 > /dev/null
  printf '\012\005world' > "$T/req.bin"
  curl -sS --max-time 20 -H 'Content-Type: application/proto' --data-binary @"$T/req.bin" "http://127.0.0.1:$port/demo.Greeter/SayHello" | od -An -c | tr -d ' \n' > "$T/o" 2>&1
  check "Connect (curl, protobuf body)" "$T/o" "Hello,world"
else
  echo "skip grpc interop: curl is not installed (Connect)"
fi

if command -v go >/dev/null 2>&1; then
  if (cd "$here/tests/grpc_go" && go build -o "$T/goclient" ./client && go build -o "$T/goserver" ./server) > "$T/go.log" 2>&1; then
    timeout 120 "$T/goclient" "$port" > "$T/o" 2>&1; echo "exit $?" >> "$T/o"
    check "Go client (grpc-go) against the Eider server" "$T/o" "ok   unary SayHello" "ok   error status NOT_FOUND" "ok   metadata in" "ok   deadline: DEADLINE_EXCEEDED" "ok   server streaming Count" "ok   client streaming Sum" "ok   bidirectional Chat" "ok   cancelled call is CANCELLED" "ok   1 MiB message" "ok   gzip" "ok   30 concurrent calls" "exit 0"
    "$T/goserver" > "$T/gs.log" 2>&1 &
    wait_for "$T/gs.log" listening
    gport=$(sed -n 's/^listening //p' "$T/gs.log" | head -1)
    timeout 120 "$bin/grpccli" "$gport" > "$T/o" 2>&1; echo "exit $?" >> "$T/o"
    check "Eider client against a Go server (grpc-go)" "$T/o" "ok   unary SayHello" "ok   error status NOT_FOUND" "ok   metadata in" "ok   deadline: DEADLINE_EXCEEDED" "ok   server streaming Count" "ok   client streaming Sum" "ok   bidirectional Chat" "ok   cancelled call is CANCELLED" "ok   1 MiB message" "ok   gzip" "ok   30 concurrent calls" "exit 0"
  else
    echo "FAIL grpc interop: go build of tests/grpc_go"; tail -20 "$T/go.log"; fail=1
  fi
else
  echo "skip grpc interop: go is not installed (a Go client and server)"
fi
exit $fail
