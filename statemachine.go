package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"path/filepath"
	"os"

	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/boltdb/bolt"
	"omanom.com/bydb/dir"
)

type ByStateMachine struct {
	store *store
	metaDb *bolt.DB
	lastIndex uint64
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {
	return &ByStateMachine{
		store: NewStore(dir.DataPath()),
	}
}

func (s *ByStateMachine) Lookup(q any) (any, error) {
	cmd := q.(Command)
	return s.store.Get(cmd.Part, cmd.Id)
}

func (s *ByStateMachine) getLastUpdateIndex() (uint64, error) {
	var lastIndex uint64

	viewErr := s.metaDb.View(func(tx *bolt.Tx) error {
		b, bucketErr := tx.CreateBucketIfNotExists([]byte("meta"))
		if bucketErr != nil { return bucketErr }

		v := b.Get([]byte("lastUpdateIndex"))

		lastIndex = binary.LittleEndian.Uint64(v)

		return nil
	})

	return lastIndex, viewErr
}

func (s *ByStateMachine) Update(updates []sm.Entry) ([]sm.Entry, error) {
	for i, entry := range updates {
		cmd := Command{}
		jsErr := json.Unmarshal(entry.Cmd, &cmd)
		if jsErr != nil { panic(jsErr) }

		switch cmd.Type {
		case PUT:
		case POST:
			s.store.Put(&cmd.Doc)
			break

		case DEL:
			s.store.Delete(cmd.Part, cmd.Id)
			break

		default:
			panic("unknown command in Update")
		}

		updates[i].Result = sm.Result{ Value: uint64(len(updates[i].Cmd)) }
	}

	return updates, nil
}

func (s *ByStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	metaDb, boltErr := bolt.Open(dir.MetaDbPath(), 0600, nil)
	if boltErr != nil {
		return 0, nil
	}

	s.metaDb = metaDb

	return s.getLastUpdateIndex()
}

func (s *ByStateMachine) PrepareSnapshot() (any, error) {
	lastUpdate, laErr := s.getLastUpdateIndex()
	if laErr != nil { return nil, laErr }

	targetPath := filepath.Join(dir.SnapshotPath(), string(lastUpdate)+".tgz")
	targetFd, fileErr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)

	if fileErr != nil { return nil, fileErr }

	tarballErr := Tar(dir.DataPath(), targetFd)

	return targetPath, tarballErr
}

func (s *ByStateMachine) RecoverFromSnapshot(zip io.Reader, done <-chan struct{}) error {
	return Untar(dir.DataPath(), zip)
}


func (s *ByStateMachine) SaveSnapshot(key any, writer io.Writer, done <-chan struct{}) error {
	path := key.(string)
	fd, err := os.Open(path)
	if err != nil { return err }
	_, copyErr := io.Copy(writer, fd)
	return copyErr
}

func (s *ByStateMachine) Sync() error {
	return nil
}

func (s *ByStateMachine) Close() error {
	return s.metaDb.Close()
}
