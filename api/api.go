package api

import (
	"net"
	"log"
	"context"
	"github.com/lni/dragonboat/v4"
	"google.golang.org/grpc"

	pb "omanom.com/bydb/proto"
	"omanom.com/bydb/command"
)


type Api struct {
	pb.UnimplementedByDbServer	
	raft *dragonboat.NodeHost
	shardId uint64
}

func (s Api) Hello(ctx context.Context, grt *pb.Greeting) (*pb.Greeting, error) {
	log.Printf("api Hello recieved %s", grt.Msg)
	return &pb.Greeting{ Msg:"hello" }, nil
}

func (s Api) Crud(ctx context.Context, gCmd *pb.Command) (*pb.Response, error) {
	log.Printf("api crud command %s", gCmd.Raw)
	cmd, err := command.ParseCommand(gCmd.Raw)

	if err != nil { return nil, err }

	cs := s.raft.GetNoOPSession(s.shardId)
	response := pb.Response{}

	switch cmd.Type {
	case command.PUT, command.POST, command.DEL:
		log.Printf("api crud is alter")
		_, err := s.raft.SyncPropose(ctx, cs, []byte(gCmd.Raw))
		if err != nil {
			return nil, err
		}

		response.Document = "ok"

	case command.GET:
		log.Printf("api crud is get")
		result, err := s.raft.SyncRead(ctx, s.shardId, gCmd.Raw)
		// @TODO not just put to std
		if err != nil {
			return nil, err
		} else {
			document := result.([]byte)
			response.Document = string(document)
		}

	default:
		panic("not yet implemented")
	}

	return &response, nil
}

func (s Api) Start(address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
	  log.Fatalf("failed to start api: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterByDbServer(grpcServer, s)
	grpcServer.Serve(lis)
}

func NewApi(nh *dragonboat.NodeHost) *Api {
	return &Api{
		raft: nh,
		shardId: 128, //@TODO made up shardId
	}
}
