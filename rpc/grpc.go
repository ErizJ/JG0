package rpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"net"
	"time"
)

//conn, err := grpc.Dial("localhost:9111", grpc.WithTransportCredentials(insecure.NewCredentials()))
//if err != nil {
//panic(err)
//}
//defer func(conn *grpc.ClientConn) {
//	err := conn.Close()
//	if err != nil {
//
//	}
//}(conn)
//rpcClient := api.NewGoodsApiClient(conn)
//rsp, err := rpcClient.Find(context.TODO(), &api.GoodsRequest{})
//if err != nil {
//panic(err)
//}

type MsGrpcServer struct {
	listen   net.Listener
	g        *grpc.Server
	register []func(g *grpc.Server)
	ops      []grpc.ServerOption
}

type MsGrpcOption interface {
	Apply(s *MsGrpcServer)
}

func NewGrpcServer(addr string, ops ...MsGrpcOption) (*MsGrpcServer, error) {
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	ms := &MsGrpcServer{}
	ms.listen = listen
	for _, op := range ops {
		op.Apply(ms)
	}
	ms.g = grpc.NewServer(ms.ops...)
	return ms, nil
}

func (s *MsGrpcServer) Run() error {
	for _, f := range s.register {
		f(s.g)
	}
	return s.g.Serve(s.listen)
}

func (s *MsGrpcServer) Stop() {
	s.g.Stop()
}

func (s *MsGrpcServer) Register(f func(g *grpc.Server)) {
	s.register = append(s.register, f)
}

type DefaultGrpcOption struct {
	f func(s *MsGrpcServer)
}

func (o *DefaultGrpcOption) Apply(s *MsGrpcServer) {
	o.f(s)
}

func GrpcWithOptions(options ...grpc.ServerOption) MsGrpcOption {
	return &DefaultGrpcOption{f: func(s *MsGrpcServer) {
		s.ops = append(s.ops, options...)
	}}
}

type MsGrpcClient struct {
	Conn *grpc.ClientConn
}

func NewGrpcClient(config *MsGrpcClientConfig) (*MsGrpcClient, error) {
	var ctx = context.Background()
	var dialOptions = config.dialOptions

	if config.Block {
		//阻塞
		if config.DialTimeout > time.Duration(0) {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.DialTimeout)
			defer cancel()
		}
		dialOptions = append(dialOptions, grpc.WithBlock())
	}
	if config.KeepAlive != nil {
		dialOptions = append(dialOptions, grpc.WithKeepaliveParams(*config.KeepAlive))
	}
	conn, err := grpc.DialContext(ctx, config.Address, dialOptions...)
	if err != nil {
		return nil, err
	}
	return &MsGrpcClient{
		Conn: conn,
	}, nil
}

type MsGrpcClientConfig struct {
	Address     string
	Block       bool
	DialTimeout time.Duration
	ReadTimeout time.Duration
	Direct      bool
	KeepAlive   *keepalive.ClientParameters
	dialOptions []grpc.DialOption
}

func DefaultGrpcClientConfig(addr string) *MsGrpcClientConfig {
	return &MsGrpcClientConfig{
		dialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		DialTimeout: time.Second * 3,
		ReadTimeout: time.Second * 2,
		Block:       true,
		Address:     addr,
	}
}
