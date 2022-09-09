package statemachine

import (
	"io"
	"path/filepath"
	"encoding/json"
	"os"
	"sync"
	"fmt"
	"strconv"

	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/boltdb/bolt"
	"omanom.com/bydb/dir"
	. "omanom.com/bydb/store"
	"omanom.com/bydb/command"
	"omanom.com/bydb/zipper"
	"omanom.com/bydb/document"
	"omanom.com/bydb/response"
	"omanom.com/bydb/logger"
)

type ByStateMachine struct {
	store *Store
	metaDb *bolt.DB
	lastIndex uint64
	logger *logger.Logger
	pending []*command.Command
	pendingMutex sync.RWMutex
	diskMutex sync.Mutex
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {

	mkdirErr := os.MkdirAll(dir.DataPath(), os.ModePerm)
	if mkdirErr != nil {
		panic(mkdirErr)
	}

	mkSnapdirErr := os.MkdirAll(dir.SnapshotPath(), os.ModePerm)
	if mkSnapdirErr != nil {
		panic(mkSnapdirErr)
	}


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

		{
			s.pendingMutex.RLock()
			defer s.pendingMutex.RUnlock()

			// @TODO iterate through pending
			for i := len(s.pending)-1 ; i >= 0; i-- {
				pending := s.pending[i]
				if cmd.Id == pending.Id && cmd.Part == pending.Part {
					switch pending.Type {
					case command.DEL:
						res.Code = 404
						res.Body = "not found"
						return res, nil
					case command.POST, command.PUT:
						res.Code = 200
						res.Body = cmd.StringDoc
						return res, nil
					}
				}
			}
		}

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
		s.logger.Debug("retrieved lastIndex", string(v))

		index, err := strconv.ParseUint(string(v), 10, 64)
		lastIndex = index

		return err
	})

	s.lastIndex = lastIndex

	return lastIndex, viewErr
}

func (s *ByStateMachine) updateLastUpdateIndex() error {
	s.logger.Logf("updating last log %d", s.lastIndex)
	return s.metaDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("meta"))
		str := fmt.Sprintf("%d", s.lastIndex)
		return b.Put([]byte("lastUpdateIndex"), []byte(str))
	})
}

func (s *ByStateMachine) Update(updates []sm.Entry) ([]sm.Entry, error) {
	s.logger.Debug("in Update")

	for ui, entry := range updates {
		// Parse the command string into a cmd struct
		cmd, jsErr := command.ParseCommand(string(entry.Cmd))
		// It should havel already been validated so panic if
		// there's an issue.
		if jsErr != nil { panic(jsErr) }

		var existingDoc *document.Document = nil

		// See if we can find an existing document in
		// the penging queue.
		s.logger.Debug("searching in pending cmds")
		s.pendingMutex.RLock()
		for i := len(s.pending)-1 ; i >= 0; i-- {
			pending := s.pending[i]
			if cmd.Id == pending.Id && cmd.Part == pending.Part {
				// There is a pending entry for this document
				existingDoc = pending.Doc
				break
			}
		}
		s.pendingMutex.RUnlock()

		// If no document is in the queue, search for one in
		// state
		if existingDoc == nil {
			s.logger.Debug("searching in disk state")
			doc, getErr := s.store.Get(cmd.Part, cmd.Id)
			if getErr != nil {
				return updates, getErr
			}
			existingDoc = doc
		}

		switch cmd.Type {
		case command.PUT, command.DEL:
			if existingDoc == nil {
				updates[ui].Result.Value = 404
				continue
			}
			if existingDoc.Updated != cmd.Ts {
				updates[ui].Result.Value = 409
				continue
			}
		case command.POST:
			if existingDoc != nil {
				updates[ui].Result.Value = 409
				continue
			}
		}

		updates[ui].Result.Value = 200

		// The sync operation has to know about the applied index
		// so set it here
		cmd.Index = entry.Index

		s.pendingMutex.Lock()
		s.pending = append(s.pending, cmd)
		s.pendingMutex.Unlock()
	}

	return updates, nil
}

func (s *ByStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	s.logger.Debug("in open")

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
	s.diskMutex.Lock()
	defer s.diskMutex.Unlock()
	lastUpdate, laErr := s.getLastUpdateIndex()
	if laErr != nil { return nil, laErr }

	fileName := fmt.Sprintf("%d.tgz", lastUpdate)
	targetPath := filepath.Join(dir.SnapshotPath(), fileName)

	s.logger.Debugf("preparing snapshot %s", targetPath)

	targetFd, fileErr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)

	if fileErr != nil { return nil, fileErr }

	tarballErr := zipper.Tar(dir.DataPath(), targetFd)

	return targetPath, tarballErr
}

func (s *ByStateMachine) RecoverFromSnapshot(zip io.Reader, done <-chan struct{}) error {
	s.diskMutex.Lock()
	defer s.diskMutex.Unlock()
	s.logger.Debug("recovering from snapshot")

	s.store.CloseAllPartitions()

	zipErr := zipper.Untar(dir.DataPath(), zip)
	if zipErr != nil { return zipErr }

	lastUpdate, lastUpdateErr := s.getLastUpdateIndex()
	s.lastIndex = lastUpdate
	return lastUpdateErr
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

	if s.metaDb == nil { panic("metadb is closed") }

	go func() {
		s.diskMutex.Lock()
		defer s.diskMutex.Unlock()

		s.pendingMutex.RLock()
		pending := s.pending
		s.pendingMutex.RUnlock()

		// Iterate through the pending interactions and write them
		// to disk. We assume that they have already been validated.
		for _, cmd := range pending {
			switch cmd.Type {
			case command.PUT, command.POST:
				err := s.store.PutBytes(cmd.Doc, cmd.BytesDoc)
				if err != nil { panic(err) }

			case command.DEL:
				err := s.store.Delete(cmd.Part, cmd.Id)
				if err != nil { panic(err) }

			default:
				panic("unknown command in sync")
			}

			// The command contains the index as applied
			s.lastIndex = cmd.Index
		}

		// Clear the pending queue
		s.pendingMutex.Lock()
		s.pending = s.pending[len(pending):]
		s.pendingMutex.Unlock()

		luiErr := s.updateLastUpdateIndex()
		if luiErr != nil { panic(luiErr) }
	}()

	return nil
}

func (s *ByStateMachine) Close() error {
	s.logger.Debug("in close")
	err := s.metaDb.Close()
	s.metaDb = nil
	return err
}
