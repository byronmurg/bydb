package dir

import (
	"path/filepath"
)

var (
	prefix = ""
)

func SetPrefix(s string) {
	prefix = s
}

func RootPath() string {
	return prefix
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

func BlockPath() string {
	return filepath.Join(DataPath(), "block")
}

func IndexPath() string {
	return filepath.Join(DataPath(), "index")
}

func SnapshotPath() string {
	return filepath.Join(RootPath(), "snapshots")
}
