// A Go gRPC server (grpc-go) with demo.Greeter, for the Eider client in tests/grpc_interop_client.ei.
// Prints `listening <port>`.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"grpc_go/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type greeter struct{}

func sayHello(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in []byte
	if err := dec(&in); err != nil {
		return nil, err
	}
	return common.Str("Hello, " + common.GetStr(in)), nil
}

func fail(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in []byte
	if err := dec(&in); err != nil {
		return nil, err
	}
	grpc.SetTrailer(ctx, metadata.Pairs("x-failed", "yes"))
	return nil, status.Error(codes.NotFound, "no such user: "+common.GetStr(in))
}

func meta(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in []byte
	if err := dec(&in); err != nil {
		return nil, err
	}
	md, _ := metadata.FromIncomingContext(ctx)
	tok := "-"
	if v := md.Get("x-token"); len(v) > 0 {
		tok = v[0]
	}
	raw := 0
	if v := md.Get("x-raw-bin"); len(v) > 0 {
		raw = len(v[0])
	}
	grpc.SetHeader(ctx, metadata.Pairs("x-seen", tok))
	grpc.SetTrailer(ctx, metadata.Pairs("x-done", "1"))
	return common.Str(fmt.Sprintf("token=%s raw=%d", tok, raw)), nil
}

func slow(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var in []byte
	if err := dec(&in); err != nil {
		return nil, err
	}
	n := common.GetNum(in)
	select {
	case <-time.After(time.Duration(n) * time.Millisecond):
		return in, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

func count(srv any, st grpc.ServerStream) error {
	var in []byte
	if err := st.RecvMsg(&in); err != nil {
		return err
	}
	for i := int64(1); i <= common.GetNum(in); i++ {
		if err := st.SendMsg(common.Num(i)); err != nil {
			return err
		}
	}
	return nil
}

func sum(srv any, st grpc.ServerStream) error {
	var total int64
	for {
		var in []byte
		err := st.RecvMsg(&in)
		if err == io.EOF {
			return st.SendMsg(common.Num(total))
		}
		if err != nil {
			return err
		}
		total += common.GetNum(in)
	}
}

func chat(srv any, st grpc.ServerStream) error {
	for {
		var in []byte
		err := st.RecvMsg(&in)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := st.SendMsg(common.Str("echo: " + common.GetStr(in))); err != nil {
			return err
		}
	}
}

func main() {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := grpc.NewServer(grpc.MaxRecvMsgSize(8 << 20))
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "demo.Greeter",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "SayHello", Handler: sayHello},
			{MethodName: "Fail", Handler: fail},
			{MethodName: "Meta", Handler: meta},
			{MethodName: "Slow", Handler: slow},
		},
		Streams: []grpc.StreamDesc{
			{StreamName: "Count", Handler: count, ServerStreams: true},
			{StreamName: "Sum", Handler: sum, ClientStreams: true},
			{StreamName: "Chat", Handler: chat, ServerStreams: true, ClientStreams: true},
		},
	}, struct{}{})
	fmt.Printf("listening %d\n", l.Addr().(*net.TCPAddr).Port)
	if err := s.Serve(l); err != nil {
		panic(err)
	}
}
