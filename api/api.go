package api

import (
	"net"
	"time"
	"context"
	"github.com/lni/dragonboat/v4"
	"google.golang.org/grpc"

	pb "omanom.com/bydb/proto"
	"omanom.com/bydb/command"
	. "omanom.com/bydb/response"
	"omanom.com/bydb/logger"
)


type Api struct {
	pb.UnimplementedByDbServer	
	raft *dragonboat.NodeHost
	shardId uint64
	logger *logger.Logger
}

func (s Api) Hello(ctx context.Context, grt *pb.Greeting) (*pb.Greeting, error) {
	s.logger.Debugf("Hello recieved %s", grt.Msg)
	return &pb.Greeting{ Msg:"server active" }, nil //@TODO actually check
}

func (s Api) Crud(ctx context.Context, gCmd *pb.Command) (*pb.Response, error) {
	s.logger.Debugf("crud command %s", gCmd.Raw)

	response := pb.Response{}

	start := time.Now()

	cmd, err := command.ParseCommand(gCmd.Raw)
	if err != nil {
		response.Code = 400
		response.Document = "bad request format"
		return &response, nil
	}

	cs := s.raft.GetNoOPSession(s.shardId)

	switch cmd.Type {
	case command.PUT, command.POST, command.DEL:
		s.logger.Debugf("crud is alter")
		_, err := s.raft.SyncPropose(ctx, cs, []byte(gCmd.Raw))
		if err != nil {
			return nil, err
		}

		response.Code = 200
		response.Document = "ok"

	case command.GET:
		s.logger.Debugf("crud is get")
		result, err := s.raft.SyncRead(ctx, s.shardId, gCmd.Raw)
		if err != nil {
			return nil, err
		}

		res := result.(Response)
		response.Code = res.Code
		response.Document = res.Body

	case command.SEARCH:
		s.logger.Debugf("crud is search")
		result, err := s.raft.SyncRead(ctx, s.shardId, gCmd.Raw)
		if err != nil {
			return nil, err
		}

		res := result.(Response)
		response.Code = res.Code
		response.Document = res.Body

	default:
		response.Code = 405
		response.Document = "unknown command"
	}

	duration := time.Now().Sub(start)
	response.Duration = int64(duration)

	return &response, nil
}

func (s Api) Start(address string) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
	  s.logger.Fatalf("failed to start api: %v", err)
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
		logger: logger.New("api"),
	}
}
