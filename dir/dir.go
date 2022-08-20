package dir

import (
	"path/filepath"
)

func RootPath() string {
	return "example-store" //@TODO not this
}

func MetaDbPath() string {
	return filepath.Join(DataPath(), "meta.bolt")
}

func RaftPath() string {
	return filepath.Join(RootPath(), "raft")
}

func DataPath() string {
	return filepath.Join(RootPath(), "data")
}

func SnapshotPath() string {
	return filepath.Join(RootPath(), "snapshots")
}
