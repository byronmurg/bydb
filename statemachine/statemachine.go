package statemachine

import (
	"io"
	"path/filepath"
	"encoding/json"
	"os"
	"errors"
	"fmt"
	"strconv"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"

	sm "github.com/lni/dragonboat/v4/statemachine"
	"github.com/boltdb/bolt"
	"omanom.com/bydb/dir"
	"omanom.com/bydb/command"
	"omanom.com/bydb/zipper"
	"omanom.com/bydb/document"
	"omanom.com/bydb/response"
	"omanom.com/bydb/logger"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"


type Shard struct {
	Index bleve.Index
	Block *bolt.DB
}

func (s *Shard) Close() {
	s.Index.Close()
	s.Block.Close()
}


type ShardMap = map[rune]*Shard




func getFirstRune(str string) rune {
	var first rune
	for _, c := range str {
		first = c
		break
	}
	return first
}






type ByStateMachine struct {
	shardMap ShardMap
	metaDb *bolt.DB
	lastIndex uint64
	logger *logger.Logger
	pending []updateEntry
}

func makeDirOrPanic(dir string) {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		panic(err)
	}
}

func createDefaultMapping() *mapping.IndexMappingImpl {
	mapping := bleve.NewIndexMapping()

	indexFieldMapping := bleve.NewDocumentMapping()

	storeOnlyFieldMapping := bleve.NewTextFieldMapping()
	storeOnlyFieldMapping.Analyzer = keyword.Name
	storeOnlyFieldMapping.Store = true
	storeOnlyFieldMapping.Index = true
	storeOnlyFieldMapping.IncludeInAll = false
	storeOnlyFieldMapping.IncludeTermVectors = false

	documentMapping := bleve.NewDocumentStaticMapping()
	documentMapping.AddSubDocumentMapping("index", indexFieldMapping)
	documentMapping.AddFieldMappingsAt("categories", storeOnlyFieldMapping)

	mapping.DefaultMapping = documentMapping

	return mapping
}

func openOrCreateBleve(partitionPath string) (bleve.Index, error) {
    _, err := os.Stat(partitionPath)

    if os.IsNotExist(err) {
		mapping := createDefaultMapping()
		return bleve.New(partitionPath, mapping)
    } else {
		return bleve.Open(partitionPath)
	}
}

func (s *ByStateMachine) OpenShardMap() {
	for _, c := range alphabet {
		blevePath := filepath.Join(dir.IndexPath(), string(c))
		index, indexErr := openOrCreateBleve(blevePath)
		if indexErr != nil { panic(indexErr) }

		boltPath := filepath.Join(dir.BlockPath(), string(c))
		block, blockErr := bolt.Open(boltPath, 0600, nil)
		if blockErr != nil { panic(blockErr) }

		s.shardMap[c] = &Shard{
			Index: index,
			Block: block,
		}
	}
}

func (s *ByStateMachine) CloseShardMap() {
	for _, c := range alphabet {
		s.shardMap[c].Close()
	}
}

func NewByStateMachine(uint64, uint64) sm.IOnDiskStateMachine {

	makeDirOrPanic(dir.IndexPath())
	makeDirOrPanic(dir.BlockPath())
	makeDirOrPanic(dir.SnapshotPath())

	return &ByStateMachine{
		shardMap: ShardMap{},
		logger: logger.New("statemachine"),
	}
}


type searchMatch struct {
	Score float64 `json:"score"`
	Doc *document.Document `json:"doc"`
}

type searchResult struct {
	Matches []*searchMatch `json:"matches"`
}


func (p *Shard) Search(part string, searchStr string) (*searchResult, error) {

	query := bleve.NewQueryStringQuery(searchStr)

	search := bleve.NewSearchRequest(query)
	blSearchResults, err := p.Index.Search(search)
	if err != nil {
		return nil, err
	}

	res := searchResult{}

	viewErr := p.Block.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(part))
		if b == nil { return nil }

		for _, hit := range(blSearchResults.Hits) {
			match := searchMatch{ Doc:&document.Document{} }

			match.Score = hit.Score

			jsDoc := b.Get([]byte(hit.ID))
			if jsDoc == nil {
				//@TODO error here as index is incorrect
				continue
			}


			jsErr := json.Unmarshal(jsDoc, match.Doc)
			if jsErr != nil { return jsErr }

			res.Matches = append(res.Matches, &match)
		}

		return nil
	})

	if viewErr != nil { return nil, viewErr }

	return &res, nil
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

	shard, shardExists := s.shardMap[getFirstRune(cmd.Part)]

	if ! shardExists {
		return nil, errors.New("no such shard "+ cmd.Part)
	}

	switch cmd.Type {
	case command.GET:

		var raw []byte
		getErr := shard.Block.View(func (tx *bolt.Tx) error {
			// Get the bucket, if nil is returned then
			// the bucket doesn't exist so just return
			b := tx.Bucket([]byte(cmd.Part))
			if b == nil { return nil }

			// Get the raw bytes and copy them into the
			// return buffer
			doc := b.Get([]byte(cmd.Id))
			raw = make([]byte, len(doc))
			copy(raw, doc)

			return nil
		})

		if getErr != nil { return nil, getErr }

		if raw == nil || len(raw) == 0 {
			res.Code = 404
			res.Body = "not found"
		} else {
			res.Code = 200
			res.Body = string(raw)
		}

	case command.SEARCH:
		results, searchErr := shard.Search(cmd.Part, cmd.Query)
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


type updateEntry struct {
	Entry *sm.Entry
	Cmd *command.Command
	ExistingDoc *document.Document
	ShardId rune
}

func (s *ByStateMachine) Update(updates []sm.Entry) ([]sm.Entry, error) {
	s.logger.Debug("in Update")

	// First we parse the command strings into
	// update structs
	var updateEntries []*updateEntry
	var updateIndex uint64 = s.lastIndex

	for i, entry := range updates {
		// Parse the command string into a cmd struct
		cmd, jsErr := command.ParseCommand(string(entry.Cmd))
		// It should havel already been validated so panic if
		// there's an issue.
		if jsErr != nil { panic(jsErr) }

		update := &updateEntry{
			Entry: &updates[i],
			Cmd: cmd,
			ShardId: getFirstRune(cmd.Part),
		}

		// Find the highest index
		if entry.Index > updateIndex {
			updateIndex = entry.Index
		}

		updateEntries = append(updateEntries, update)
	}

	// Then range through each shard
	for c, shard := range s.shardMap {

		// Find all entries that pertain to this shard
		var shardEntries []*updateEntry
		for _, entry := range updateEntries {
			if entry.ShardId == c {
				shardEntries = append(shardEntries, entry)
			}
		}

		// Skip if there are no shard entries
		if len(shardEntries) == 0 {
			continue
		}

		go func() {

			// Start a block view transaction to find existing documents
			shard.Block.View(func(tx *bolt.Tx) error {

				for _, entry := range shardEntries {

					bucket := tx.Bucket([]byte(entry.Cmd.Part))
					if bucket == nil { continue }

					rawDoc := bucket.Get([]byte(entry.Cmd.Id))

					if len(rawDoc) == 0 {
						continue
					}

					doc := document.Document{}
					jsErr := json.Unmarshal(rawDoc, &doc)
					entry.ExistingDoc = &doc

					if jsErr != nil { return jsErr }
				}

				return nil
			})

			var validEntries []*updateEntry
			// Now go back through and decide what to do with them all
			for _, entry := range shardEntries {

				switch entry.Cmd.Type {
				case command.PUT, command.DEL:
					if entry.ExistingDoc == nil {
						entry.Entry.Result.Value = 404
					} else if entry.ExistingDoc.Updated != entry.Cmd.Ts {
						entry.Entry.Result.Value = 409
					} else {
						entry.Entry.Result.Value = 200
						validEntries = append(validEntries, entry)
					}
				case command.POST:
					if entry.ExistingDoc != nil {
						entry.Entry.Result.Value = 409
					} else {
						entry.Entry.Result.Value = 200
						validEntries = append(validEntries, entry)
					}
				}

				s.logger.Debug("command ", entry.Cmd.Id, " = ", entry.Entry.Result.Value)
			}


			// Now we actually commit the valid entries

			updateErr := shard.Block.Update(func(tx *bolt.Tx) error {
				indexBatch := shard.Index.NewBatch()

				for _, entry := range validEntries {
					blockBucket, bucketErr := tx.CreateBucketIfNotExists([]byte(entry.Cmd.Part))
					if bucketErr != nil { return bucketErr }

					switch entry.Cmd.Type {
					case command.PUT, command.POST:

						s.logger.Debug("write", entry.Cmd.Id)

						if err := blockBucket.Put([]byte(entry.Cmd.Id), entry.Cmd.BytesDoc); err != nil {
							return err
						}

						if err := indexBatch.Index(entry.Cmd.Id, entry.Cmd.Doc); err != nil {
							return err
						}
					case command.DEL:

						s.logger.Debug("delete", entry.Cmd.Id)
						
						if err := blockBucket.Delete([]byte(entry.Cmd.Id)); err != nil {
							return err
						}

						indexBatch.Delete(entry.Cmd.Id)
					}
				}

				return shard.Index.Batch(indexBatch)
			})

			// Have to panic if the update fails
			if updateErr != nil {
				panic(updateErr)
			}
		}()
	}

	s.lastIndex = updateIndex
	if err := s.updateLastUpdateIndex(); err != nil {
		return updates, err
	}

	return updates, nil
}

func (s *ByStateMachine) Open(stopc <-chan struct{}) (uint64, error) {
	s.logger.Debug("in open")

	s.OpenShardMap()

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

	fileName := fmt.Sprintf("%d.tgz", lastUpdate)
	targetPath := filepath.Join(dir.SnapshotPath(), fileName)

	s.logger.Debugf("preparing snapshot %s", targetPath)

	targetFd, fileErr := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, 0600)

	if fileErr != nil { return nil, fileErr }

	tarballErr := zipper.Tar(dir.DataPath(), targetFd)

	return targetPath, tarballErr
}

func (s *ByStateMachine) RecoverFromSnapshot(zip io.Reader, done <-chan struct{}) error {
	s.logger.Debug("recovering from snapshot")

	zipErr := zipper.Untar(dir.DataPath(), zip)
	if zipErr != nil { return zipErr }

	lastUpdate, lastUpdateErr := s.getLastUpdateIndex()
	s.lastIndex = lastUpdate
	return lastUpdateErr
}


func (s *ByStateMachine) SaveSnapshot(key any, writer io.Writer, done <-chan struct{}) error {
	path := key.(string)
	s.logger.Debug("saving snapshot ", path)
	fd, err := os.Open(path)
	if err != nil { return err }
	_, copyErr := io.Copy(writer, fd)
	return copyErr
}

func (s *ByStateMachine) Sync() error {
	return nil
}

func (s *ByStateMachine) Close() error {
	s.logger.Debug("in close")
	err := s.metaDb.Close()
	s.CloseShardMap()
	s.metaDb = nil
	return err
}
