package main

import (
	"fmt"
	"os"

	"encoding/json"
	

	"github.com/lni/dragonboat/v4"
	raftconfig "github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"

	"omanom.com/bydb/dir"
	"omanom.com/bydb/config"
	. "omanom.com/bydb/statemachine"
	. "omanom.com/bydb/api"
)

func enableDragonboatLogging() {
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
}

func isFirstRun() bool {
    _, err := os.Stat(dir.DataPath())
    return os.IsNotExist(err)
}

func Dump(input any) {
	bytes, err := json.MarshalIndent(input, "", "  ")
	if err != nil { panic(err) }
	fmt.Println(string(bytes))
}

func main() {

	cnf, cfErr := config.LoadConfig()
	if cfErr != nil { panic(cfErr) }
	Dump(cnf)

	replicaID := cnf.ReplicaId

	//@TODO this just picks me an unused path
	dir.SetPrefix(fmt.Sprintf("test-run/node-%d", replicaID))

	isInitNode := cnf.IsInitNode()
	join := isFirstRun() && !isInitNode

	initialMembers := make(map[uint64]string)
	if isFirstRun() && !join {
		initialMembers = cnf.InitialNodeMap()
	}

	nodeAddr := cnf.NodeAddress()

	if nodeAddr == "" {
		panic("unable to determine node address")
	}

	grpcAddr := cnf.GrpcAddress

	if grpcAddr == "" {
		panic("unable to determine grpc address")
	}

	fmt.Println("replicaID: ", replicaID)
	fmt.Println("node address: ", nodeAddr)
	fmt.Println("join: ", join)
	fmt.Println("is init node: ", isInitNode)
	fmt.Println("is firt run: ", isFirstRun())

	enableDragonboatLogging()

	/*
	var nodeHostId string
	if isInitNode != 0 {
		nodeHostId = fmt.Sprintf("initNode-%d", *replicaID)
	}
	*/

	rc := raftconfig.Config{
		ReplicaID:          cnf.ReplicaId,
		ShardID:            cnf.Raft.ShardId,
		ElectionRTT:        cnf.Raft.ElectionRTT,
		HeartbeatRTT:       cnf.Raft.HeartbeatRTT,
		CheckQuorum:        true,
		SnapshotEntries:    cnf.Raft.SnapshotEntries,
		CompactionOverhead: cnf.Raft.CompactionOverhead,
	}

	datadir := dir.RaftPath()

	nhc := raftconfig.NodeHostConfig{
		WALDir:         datadir,
		//NodeHostID:     nodeHostId,
		NodeHostDir:    datadir,
		RTTMillisecond: cnf.Raft.RTTMillisecond,
		RaftAddress:    nodeAddr,
	}

	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}

	if err := nh.StartOnDiskReplica(initialMembers, join, NewByStateMachine, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}

	api := NewApi(nh)

	api.Start(grpcAddr)
}
