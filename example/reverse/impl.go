package main

import (
	"context"
	"fmt"
	"net"

	testpb "github.com/holdno/gopherCron/example/reverse/pb"
)

type Center struct {
	testpb.UnimplementedCenterServer
}

var _ testpb.CenterServer = (*Center)(nil)

func (s *Center) OnCall(ctx context.Context, req *testpb.OnCallReq) (*testpb.OnCallReply, error) {
	return &testpb.OnCallReply{
		Response: "center got: " + req.Data,
	}, nil
}

type Agent struct {
	testpb.UnimplementedAgentServer
}

var _ testpb.CenterServer = (*Center)(nil)

func (s *Agent) OnCall(ctx context.Context, req *testpb.OnCallReq) (*testpb.OnCallReply, error) {
	return &testpb.OnCallReply{
		Response: "agent got: " + req.Data,
	}, nil
}

type FakeListener struct {
	connChan chan net.Conn
}

func (s *FakeListener) Accept() (net.Conn, error) {
	conn := <-s.connChan
	fmt.Println("fake listener got connect")
	return conn, nil
}

func (s *FakeListener) Close() error {
	return nil
}

func (s *FakeListener) Addr() net.Addr {
	return &net.IPAddr{
		IP:   []byte("localtest"),
		Zone: "ipv4",
	}
}

func fakeListener(c chan net.Conn) net.Listener {
	return &FakeListener{
		connChan: c,
	}
}
