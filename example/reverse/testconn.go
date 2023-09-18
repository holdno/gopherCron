package main

import (
	"context"
	"fmt"
	"net"
	"time"

	testpb "github.com/holdno/gopherCron/example/reverse/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	go server()
	time.Sleep(time.Second * 3)
	client()

	time.Sleep(time.Second * 20)
	fmt.Println("over")
}

func client() {
	srv := grpc.NewServer()
	testpb.RegisterAgentServer(srv, &Agent{})
	connChan := make(chan net.Conn)
	go srv.Serve(fakeListener(connChan))

	conn, err := net.Dial("tcp", "0.0.0.0:20000")
	if err != nil {
		panic("tcp dial error: " + err.Error())
	}

	conn2, err := net.Dial("tcp", "0.0.0.0:20000")
	if err != nil {
		panic("tcp dial error: " + err.Error())
	}

	connChan <- conn2
	// time.Sleep(time.Second * 10)
	cc, err := grpc.DialContext(context.Background(), "target:///", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return conn, nil
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("failed to dial center " + err.Error())
	}

	centerClient := testpb.NewCenterClient(cc)
	resp, err := centerClient.OnCall(context.Background(), &testpb.OnCallReq{
		Data: "i'm agent",
	})
	if err != nil {
		panic("failed to call center, " + err.Error())
	}

	fmt.Println("got center response: " + resp.Response)
}

func server() {
	listen, err := net.Listen("tcp", "0.0.0.0:20000")
	if err != nil {
		panic(err)
	}

	// conn, err := listen.Accept()
	// if err != nil {
	// 	panic(err)
	// }
	// var buf [128]byte
	// n, err := conn.Read(buf[:])
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(string(buf[:n]))

	srv := grpc.NewServer()
	testpb.RegisterCenterServer(srv, &Center{})

	connChan := make(chan net.Conn)
	clientChan := make(chan net.Conn)
	go srv.Serve(fakeListener(connChan))
	i := 0
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				panic("accept error: " + err.Error())
			}
			if i == 0 {
				connChan <- conn
			} else {
				clientChan <- conn
			}
			i++
		}
	}()

	time.Sleep(time.Second * 10)
	cc, err := grpc.DialContext(context.Background(), "target:///", grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return <-clientChan, nil
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic("failed to dial agent " + err.Error())
	}
	agentClient := testpb.NewAgentClient(cc)
	fmt.Println(cc.GetState().String())
	resp, err := agentClient.OnCall(context.Background(), &testpb.OnCallReq{
		Data: "i'm center",
	})
	if err != nil {
		panic("failed to call agent, " + err.Error())
	}

	fmt.Println("got agent response: " + resp.Response)
}
