package statemachine

import (
	"encoding/binary"
	"io"
	"path/filepath"
	"os"
	"log"

	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/boltdb/bolt"
	"omanom.com/bydb/dir"
	. "omanom.com/bydb/store"
	"omanom.com/bydb/command"
	"omanom.com/bydb/zipper"
)

type ByStateMachine struct {
	store *Store
	metaDb *bolt.DB
	lastIndex uint64
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {
	return &ByStateMachine{
		store: NewStore(dir.DataPath()),
	}
}

func (s *ByStateMachine) Lookup(q any) (any, error) {
	raw := q.(string)
	log.Printf("Lookup recieved %s", raw)
	cmd, err := command.ParseCommand(raw)
	if err != nil { return nil, err }
	return s.store.GetRaw(cmd.Part, cmd.Id)
}

func (s *ByStateMachine) getLastUpdateIndex() (uint64, error) {
	var lastIndex uint64 = 0

	viewErr := s.metaDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("meta"))
		if b == nil { return nil }

		v := b.Get([]byte("lastUpdateIndex"))

		if len(v) == 0 { return nil }

		lastIndex = binary.LittleEndian.Uint64(v)

		return nil
	})

	return lastIndex, viewErr
}

func (s *ByStateMachine) updateLastUpdateIndex() error {
	log.Printf("updating last log %d", s.lastIndex)
	return s.metaDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("meta"))
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, s.lastIndex)
		return b.Put([]byte("lastUpdateIndex"), buf)
	})
}

func (s *ByStateMachine) Update(updates []sm.Entry) ([]sm.Entry, error) {
	log.Printf("SM in Update")
	var appliedIndex uint64 = 0

	for i, entry := range updates {
		log.Printf("Update recieved %s", entry.Cmd)
		cmd, jsErr := command.ParseCommand(string(entry.Cmd))
		if jsErr != nil { panic(jsErr) }

		switch cmd.Type {
		case command.PUT, command.POST:
			err := s.store.Put(&cmd.Doc)
			if err != nil { return updates, err }

		case command.DEL:
			err := s.store.Delete(cmd.Part, cmd.Id)
			if err != nil { return updates, err }

		default:
			panic("unknown command in Update")
		}

		updates[i].Result = sm.Result{ Value: uint64(len(updates[i].Cmd)) }
		appliedIndex = entry.Index
	}

	s.lastIndex = appliedIndex

	return updates, s.updateLastUpdateIndex()
}

func (s *ByStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	log.Printf("in open")
	metaDb, boltErr := bolt.Open(dir.MetaDbPath(), 0600, nil)
	if boltErr != nil {
		return 0, nil
	}

	// Make sure that the metabucket is created
	bucketCreateErr := metaDb.Update(func (tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("meta"))
		return err
	})

	if bucketCreateErr != nil { return 0, bucketCreateErr }

	s.metaDb = metaDb

	return s.getLastUpdateIndex()
}

func (s *ByStateMachine) PrepareSnapshot() (any, error) {
	lastUpdate, laErr := s.getLastUpdateIndex()
	if laErr != nil { return nil, laErr }

	targetPath := filepath.Join(dir.SnapshotPath(), string(lastUpdate)+".tgz")
	targetFd, fileErr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)

	if fileErr != nil { return nil, fileErr }

	tarballErr := zipper.Tar(dir.DataPath(), targetFd)

	return targetPath, tarballErr
}

func (s *ByStateMachine) RecoverFromSnapshot(zip io.Reader, done <-chan struct{}) error {
	return zipper.Untar(dir.DataPath(), zip)
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
