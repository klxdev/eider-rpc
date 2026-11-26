// A Go gRPC client (grpc-go) against the Eider server of tests/grpc_interop_server.ei:
//   client <port>
// Unary, an error status, metadata (binary too), deadlines, server/client/bidirectional streaming, a cancelled call, a
// 1 MiB message, 30 calls at once. Prints `ok   <case>` or `FAIL <case>`; exits 1 when one failed.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"grpc_go/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var bad = 0

func check(name string, ok bool) {
	if ok {
		fmt.Println("ok   " + name)
	} else {
		fmt.Println("FAIL " + name)
		bad++
	}
}

func main() {
	conn, err := grpc.NewClient("127.0.0.1:"+os.Args[1], grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(8<<20), grpc.MaxCallSendMsgSize(8<<20)))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	ctx := context.Background()
	call := func(c context.Context, m string, in []byte, opts ...grpc.CallOption) ([]byte, error) {
		var out []byte
		err := conn.Invoke(c, "/demo.Greeter/"+m, in, &out, opts...)
		return out, err
	}

	out, err := call(ctx, "SayHello", common.Str("go"))
	check("unary SayHello", err == nil && common.GetStr(out) == "Hello, go")

	var trailer metadata.MD
	_, err = call(ctx, "Fail", common.Str("bob"), grpc.Trailer(&trailer))
	check("error status NOT_FOUND with message", status.Code(err) == codes.NotFound && status.Convert(err).Message() == "no such user: bob")
	check("trailer on a failed call", len(trailer.Get("x-failed")) == 1)

	_, err = call(ctx, "Nope", common.Str("x"))
	check("unknown method is UNIMPLEMENTED", status.Code(err) == codes.Unimplemented)

	var hdr, tr metadata.MD
	mctx := metadata.AppendToOutgoingContext(ctx, "x-token", "abc", "x-raw-bin", string([]byte{0, 255, 1, 2}))
	out, err = call(mctx, "Meta", common.Str("m"), grpc.Header(&hdr), grpc.Trailer(&tr))
	check("metadata in (text and -bin)", err == nil && common.GetStr(out) == "token=abc raw=4")
	check("metadata out (header and trailer)", len(hdr.Get("x-seen")) == 1 && hdr.Get("x-seen")[0] == "abc" && len(tr.Get("x-done")) == 1)

	dctx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	_, err = call(dctx, "Slow", common.Num(2000))
	cancel()
	check("deadline: DEADLINE_EXCEEDED", status.Code(err) == codes.DeadlineExceeded)
	dctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	_, err = call(dctx, "Slow", common.Num(20))
	cancel()
	check("deadline long enough", err == nil)

	// server streaming
	st, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/demo.Greeter/Count")
	var got []int64
	if err == nil {
		st.SendMsg(common.Num(5))
		st.CloseSend()
		for {
			var m []byte
			if e := st.RecvMsg(&m); e != nil {
				err = e
				break
			}
			got = append(got, common.GetNum(m))
		}
	}
	check("server streaming Count", err == io.EOF && fmt.Sprint(got) == "[1 2 3 4 5]")

	// client streaming
	st, err = conn.NewStream(ctx, &grpc.StreamDesc{ClientStreams: true}, "/demo.Greeter/Sum")
	var total int64 = -1
	if err == nil {
		for i := int64(1); i <= 100; i++ {
			st.SendMsg(common.Num(i))
		}
		st.CloseSend()
		var m []byte
		if err = st.RecvMsg(&m); err == nil {
			total = common.GetNum(m)
		}
	}
	check("client streaming Sum", err == nil && total == 5050)

	// bidirectional, each message answered before the next is sent
	st, err = conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/demo.Greeter/Chat")
	ok := err == nil
	for _, w := range []string{"one", "two", "three"} {
		if !ok {
			break
		}
		st.SendMsg(common.Str(w))
		var m []byte
		if e := st.RecvMsg(&m); e != nil || common.GetStr(m) != "echo: "+w {
			ok = false
		}
	}
	st.CloseSend()
	var m []byte
	ok = ok && st.RecvMsg(&m) == io.EOF
	check("bidirectional Chat", ok)

	// cancel
	cctx, cancel := context.WithCancel(ctx)
	st, err = conn.NewStream(cctx, &grpc.StreamDesc{ServerStreams: true, ClientStreams: true}, "/demo.Greeter/Chat")
	st.SendMsg(common.Str("a"))
	var m2 []byte
	st.RecvMsg(&m2)
	cancel()
	err = st.RecvMsg(&m2)
	check("cancelled call is CANCELLED", status.Code(err) == codes.Canceled)

	// a 1 MiB message each way
	big := strings.Repeat("x", 1<<20)
	out, err = call(ctx, "SayHello", common.Str(big))
	check("1 MiB message", err == nil && common.GetStr(out) == "Hello, "+big)

	// 30 at once on one connection
	var wg sync.WaitGroup
	var mu sync.Mutex
	fails := 0
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("n%d", i)
			o, e := call(ctx, "SayHello", common.Str(name))
			if e != nil || common.GetStr(o) != "Hello, "+name {
				mu.Lock()
				fails++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	check("30 concurrent calls", fails == 0)
	if bad > 0 {
		os.Exit(1)
	}
}
