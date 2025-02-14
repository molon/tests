package grpc_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/test/bufconn"
)

type panicServer struct {
	pb.UnimplementedGreeterServer
}

func (s *panicServer) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	if in.Name == "panic" {
		time.Sleep(100 * time.Millisecond) // 给其他请求一些时间先执行
		panic("test panic")
	}
	time.Sleep(50 * time.Millisecond) // 模拟一些处理时间
	return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func TestGRPCServerPanic(t *testing.T) {
	t.Run("WithRecovery", func(t *testing.T) {
		t.Log("Starting test with recovery...")
		listener := bufconn.Listen(1024 * 1024)
		server := grpc.NewServer(
			grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic occurred: %v", r)
					}
				}()
				return handler(ctx, req)
			}),
		)
		pb.RegisterGreeterServer(server, &panicServer{})

		// 启动服务器
		go func() {
			t.Log("Starting gRPC server...")
			if err := server.Serve(listener); err != nil {
				t.Logf("Server exited with error: %v", err)
			}
		}()

		// 等待服务器启动
		time.Sleep(100 * time.Millisecond)

		// 创建客户端连接
		dialer := func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}
		conn, err := grpc.NewClient(
			"bufnet",
			grpc.WithContextDialer(dialer),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		assert.NoError(t, err)
		defer conn.Close()

		client := pb.NewGreeterClient(conn)

		// 先发送一个正常请求，确保服务正常
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r, err := client.SayHello(ctx, &pb.HelloRequest{Name: "test"})
		assert.NoError(t, err)
		assert.Equal(t, "Hello test", r.GetMessage())

		// 发送会导致 panic 的请求
		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
		defer cancel2()
		_, err = client.SayHello(ctx2, &pb.HelloRequest{Name: "panic"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic occurred")

		// 再次发送请求，验证服务器是否仍在运行
		ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second)
		defer cancel3()
		r, err = client.SayHello(ctx3, &pb.HelloRequest{Name: "after_panic"})
		assert.NoError(t, err)
		assert.Equal(t, "Hello after_panic", r.GetMessage())
	})

	t.Run("WithoutRecovery", func(t *testing.T) {
		if os.Getenv("TEST_PANIC") == "1" {
			runServerWithoutRecovery(t)
			return
		}

		// 在子进程中运行测试
		cmd := exec.Command(os.Args[0], "-test.run", "TestGRPCServerPanic/WithoutRecovery")
		cmd.Env = append(os.Environ(), "TEST_PANIC=1")
		output, err := cmd.CombinedOutput()
		t.Log("Test output:", string(output))

		// 验证进程是否如预期般 panic
		if err == nil {
			t.Fatal("Process should have panicked")
		}
		// 验证输出中是否包含我们期望的 panic 信息
		assert.True(t, strings.Contains(string(output), "test panic"), "Output should contain panic message")
		assert.NotContains(t, string(output), "Should not reach this point")
		// !!! 结论是，在没有 recovery中间件 的情况下，进程会直接挂掉，所以生产环境还是要搞个 recovery 中间件 !!!
	})
}

func runServerWithoutRecovery(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer() // 没有 recovery 中间件
	pb.RegisterGreeterServer(server, &panicServer{})

	// 启动服务器
	go func() {
		t.Log("Starting gRPC server without recovery...")
		if err := server.Serve(listener); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建客户端连接
	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
	conn, err := grpc.NewClient(
		"bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewGreeterClient(conn)

	// 先发送一个正常请求，确保服务正常
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := client.SayHello(ctx, &pb.HelloRequest{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if r.GetMessage() != "Hello test" {
		t.Fatal("Unexpected response")
	}
	t.Log("First request succeeded")

	// 发送会导致 panic 的请求
	t.Log("Sending request that will cause panic...")
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	client.SayHello(ctx2, &pb.HelloRequest{Name: "panic"})

	// 这里的代码不会被执行到，因为上面的请求会导致进程 panic
	t.Fatal("Should not reach this point")
}
