package config

import (
	"os"
	"flag"
	"gopkg.in/yaml.v3"
)

type raftConfig struct {
	ShardId            uint64 `yaml:"shard_id"`
	SnapshotEntries    uint64 `yaml:"snapshot_entries"`
	CompactionOverhead uint64 `yaml:"compaction_overhead"`
	ElectionRTT        uint64 `yaml:"election_rtt"`
	HeartbeatRTT       uint64 `yaml:"heartbeat_rtt"`
	RTTMillisecond     uint64 `yaml:"rtt_millisecond"`
}

type Config struct {

	ReplicaId uint64 `yaml:"replica_id"`
	Address string `yaml:"address"`
	GrpcAddress string `yaml:"grpc_address"`

	InitialNodes []string `yaml:"initial_nodes"`

	Raft raftConfig
}

func (s *Config) IsInitNode() bool {
	return s.ReplicaId <= uint64(len(s.InitialNodes))
}

func (s *Config) InitialNodeMap() map[uint64]string {
	ret := map[uint64]string{}

	for idx, host := range s.InitialNodes {
		ret[uint64(idx+1)] = host
	}

	return ret
}

func (s *Config) NodeAddress() string {
	if s.IsInitNode() {
		return s.InitialNodes[int(s.ReplicaId-1)]
	} else {
		return s.Address
	}
}

var (
	configFile = "test-run-config.yml" //@TODO this is for testing
)

func LoadConfig() (*Config, error) {
	cnf := Config{
		// These are effectively our defaults
		GrpcAddress: "localhost:64001",
		Raft: raftConfig{
			ShardId: 128,
			SnapshotEntries: 100,
			CompactionOverhead: 20,
			ElectionRTT: 10,
			HeartbeatRTT: 1,
			RTTMillisecond: 200,
		},
	}

	flag.StringVar(&configFile, "config", configFile, "config file location")
	flag.Uint64Var(&cnf.ReplicaId, "replicaid", cnf.ReplicaId, "replica number of this host")
	flag.StringVar(&cnf.Address, "addr", cnf.Address, "nodehost address")
	flag.StringVar(&cnf.GrpcAddress, "gaddr", cnf.GrpcAddress, "gRPC address")

	flag.Parse()


	rawConfig, readErr := os.ReadFile(configFile)
	if readErr != nil { return nil, readErr }

	yamlErr := yaml.Unmarshal(rawConfig, &cnf)
	if yamlErr != nil { return nil, yamlErr }

	// Re-parse to apply command line overrides
	flag.Parse()

	return &cnf, nil
}
