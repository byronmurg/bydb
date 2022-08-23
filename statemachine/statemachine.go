package statemachine

import (
	"encoding/binary"
	"io"
	"path/filepath"
	"encoding/json"
	"os"
	"sync"

	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/boltdb/bolt"
	"omanom.com/bydb/dir"
	. "omanom.com/bydb/store"
	"omanom.com/bydb/command"
	"omanom.com/bydb/zipper"
	"omanom.com/bydb/response"
	"omanom.com/bydb/logger"
)

type ByStateMachine struct {
	store *Store
	metaDb *bolt.DB
	lastIndex uint64
	logger *logger.Logger
	entries []sm.Entry
	entryMutex sync.Mutex
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {
	return &ByStateMachine{
		store: NewStore(dir.DataPath()),
		logger: logger.New("statemachine"),
	}
}

func (s *ByStateMachine) Lookup(q any) (any, error) {
	raw := q.(string)
	s.logger.Debugf("Lookup recieved %s", raw)

	cmd, err := command.ParseCommand(raw)
	if err != nil { return nil, err }

	res := response.Response{
		Code: 500,
		Body: "server error",
	}

	switch cmd.Type {
	case command.GET:
		raw, getErr := s.store.GetRaw(cmd.Part, cmd.Id)
		if getErr != nil { return nil, getErr }

		if raw == nil || len(raw) == 0 {
			res.Code = 404
			res.Body = "not found"
		} else {
			res.Code = 200
			res.Body = string(raw)
		}

	case command.SEARCH:
		results, searchErr := s.store.Search(cmd.Part, cmd.Query)
		if searchErr != nil { return res, searchErr }
		jsResults, jsErr := json.Marshal(results)
		if jsErr != nil { return res, jsErr }
		res.Code = 200
		res.Body = string(jsResults)
	default:
		panic("unknown command passed to lookup: "+cmd.Raw)
	}

	return res, nil
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
	s.logger.Debugf("updating last log %d", s.lastIndex)
	return s.metaDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("meta"))
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, s.lastIndex)
		return b.Put([]byte("lastUpdateIndex"), buf)
	})
}

func (s *ByStateMachine) Update(updates []sm.Entry) ([]sm.Entry, error) {
	s.logger.Debug("in Update")

	s.entryMutex.Lock()
	defer s.entryMutex.Unlock()

	s.entries = append(s.entries, updates...)

	return updates, nil
}

func (s *ByStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	s.logger.Debug("in open")

	mkdirErr := os.MkdirAll(dir.DataPath(), os.ModePerm)
	if mkdirErr != nil {
		return 0, mkdirErr
	}

	metaDb, boltErr := bolt.Open(dir.MetaDbPath(), 0600, nil)
	if boltErr != nil {
		return 0, boltErr
	}

	s.metaDb = metaDb

	// Make sure that the metabucket is created
	bucketCreateErr := metaDb.Update(func (tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("meta"))
		return err
	})

	if bucketCreateErr != nil { return 0, bucketCreateErr }

	return s.getLastUpdateIndex()
}

func (s *ByStateMachine) PrepareSnapshot() (any, error) {
	lastUpdate, laErr := s.getLastUpdateIndex()
	if laErr != nil { return nil, laErr }

	targetPath := filepath.Join(dir.SnapshotPath(), string(lastUpdate)+".tgz")

	s.logger.Debugf("preparing snapshot %s", targetPath)

	targetFd, fileErr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)

	if fileErr != nil { return nil, fileErr }

	tarballErr := zipper.Tar(dir.DataPath(), targetFd)

	return targetPath, tarballErr
}

func (s *ByStateMachine) RecoverFromSnapshot(zip io.Reader, done <-chan struct{}) error {
	s.logger.Debug("recovering from snapshot")
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
	s.logger.Debug("in sync")

	var appliedIndex uint64 = 0
	if s.metaDb == nil { panic("metadb is closed") }

	s.entryMutex.Lock()
	defer s.entryMutex.Unlock()

	for _, entry := range s.entries {
		s.logger.Debugf("Update recieved %s", entry.Cmd)
		cmd, jsErr := command.ParseCommand(string(entry.Cmd))
		if jsErr != nil { panic(jsErr) }

		switch cmd.Type {
		case command.PUT, command.POST:
			err := s.store.Put(&cmd.Doc)
			if err != nil { return err }

		case command.DEL:
			err := s.store.Delete(cmd.Part, cmd.Id)
			if err != nil { return err }

		default:
			panic("unknown command in sync")
		}

		appliedIndex = entry.Index
	}

	s.lastIndex = appliedIndex
	s.entries = []sm.Entry{}

	return s.updateLastUpdateIndex()
}

func (s *ByStateMachine) Close() error {
	s.logger.Debug("in close")
	err := s.metaDb.Close()
	s.metaDb = nil
	return err
}
