package main

import (
	//"context"
	"flag"
	"fmt"
	"os"
	//"time"
	//"encoding/json"

	"github.com/lni/dragonboat/v4"
	"github.com/lni/dragonboat/v4/config"
	"github.com/lni/dragonboat/v4/logger"
	//"github.com/lni/goutils/syncutil"

	"omanom.com/bydb/dir"
	. "omanom.com/bydb/statemachine"
	. "omanom.com/bydb/api"
)

// @TODO remove this
var (
	// initial nodes count is fixed to three, their addresses are also fixed
	addresses = []string{
		"localhost:63001",
		"localhost:63002",
		"localhost:63003",
	}

	gaddresses = []string{
		"localhost:64001",
		"localhost:64002",
		"localhost:64003",
	}
)

func enableDragonboatLogging() {
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
}

func main() {
	replicaID := flag.Int("replicaid", 1, "ReplicaID to use")
	addr := flag.String("addr", "", "Nodehost address")
	join := flag.Bool("join", false, "Joining a new node")
	flag.Parse()


	// @TODO all this is in place of a proper discovery system
	if len(*addr) == 0 && *replicaID != 1 && *replicaID != 2 && *replicaID != 3 {
		fmt.Fprintf(os.Stderr, "replica id must be 1, 2 or 3 when address is not specified\n")
		os.Exit(1)
	}

	initialMembers := make(map[uint64]string)
	if !*join {
		for idx, v := range addresses {
			initialMembers[uint64(idx+1)] = v
		}
	}

	nodeAddr := initialMembers[uint64(*replicaID)]
	grpcAddr := gaddresses[*replicaID-1]

	
	fmt.Printf("node address: %s\n", nodeAddr)
	enableDragonboatLogging()


	rc := config.Config{
		ReplicaID:          uint64(*replicaID),
		ShardID:            128, //<- @TODO this is a made up shardid
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    100, //<- @TODO this needs to be more in real life
		CompactionOverhead: 20,
	}

	//@TODO this just picks me an unused path
	dir.SetPrefix(fmt.Sprintf("test-run/node%d", *replicaID))
	datadir := dir.RaftPath()

	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}

	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}

	if err := nh.StartOnDiskReplica(initialMembers, *join, NewByStateMachine, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}

	api := NewApi(nh)

	api.Start(grpcAddr)
}
