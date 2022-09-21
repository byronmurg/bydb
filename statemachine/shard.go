package statemachine

import (
	"encoding/json"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/boltdb/bolt"

	"omanom.com/bydb/dir"
	"omanom.com/bydb/document"
	"omanom.com/bydb/command"
	"omanom.com/bydb/logger"

)

type Shard struct {
	index bleve.Index
	block *bolt.DB
	logger *logger.Logger
}

func (s *Shard) Close() {
	s.index.Close()
	s.block.Close()
}


func OpenShard(c string, l *logger.Logger) (*Shard, error) {
	blevePath := filepath.Join(dir.IndexPath(), string(c))
	index, indexErr := openOrCreateBleve(blevePath)
	if indexErr != nil {
		return nil, indexErr
	}

	boltPath := filepath.Join(dir.BlockPath(), string(c))
	block, blockErr := bolt.Open(boltPath, 0600, nil)
	if blockErr != nil {
		return nil, blockErr
	}

	return &Shard{
		index: index,
		block: block,
		logger: l.Extend("shard-"+c),
	}, nil
}

func (p *Shard) GetDocumentString(id, part string) (string, bool, error) {
	var raw []byte
	getErr := p.block.View(func(tx *bolt.Tx) error {
		// Get the bucket, if nil is returned then
		// the bucket doesn't exist so just return
		b := tx.Bucket([]byte(part))
		if b == nil {
			return nil
		}

		// Get the raw bytes and copy them into the
		// return buffer
		doc := b.Get([]byte(id))
		raw = make([]byte, len(doc))
		copy(raw, doc)

		return nil
	})

	if getErr != nil {
		return "", false, getErr
	}

	if raw == nil || len(raw) == 0 {
		return "", false, nil
	} else {
		return string(raw), true, nil
	}
}

func (p *Shard) FindExistingDocumentsForUpdates(shardEntries []*updateEntry) error {

	// Start a block view transaction to find existing documents
	return p.block.View(func(tx *bolt.Tx) error {

		for _, entry := range shardEntries {

			// If the doc was found in pending we can skip
			if entry.ExistingDoc != nil {
				continue
			}

			bucket := tx.Bucket([]byte(entry.Cmd.Part))
			if bucket == nil {
				continue
			}

			rawDoc := bucket.Get([]byte(entry.Cmd.FullId()))

			if len(rawDoc) == 0 {
				continue
			}

			doc := document.Document{}
			jsErr := json.Unmarshal(rawDoc, &doc)
			entry.ExistingDoc = &doc

			if jsErr != nil {
				return jsErr
			}
		}

		return nil
	})
}

func (p *Shard) ApplyUpdates(shardEntries []*updateEntry) {
	
	// Now we actually commit the valid entries
	updateErr := p.block.Update(func(tx *bolt.Tx) error {
		indexBatch := p.index.NewBatch()

		for _, entry := range shardEntries {
			blockBucket, bucketErr := tx.CreateBucketIfNotExists([]byte(entry.Cmd.Part))
			if bucketErr != nil {
				return bucketErr
			}

			switch entry.Cmd.Type {
			case command.PUT, command.POST:

				p.logger.Debug("write ", entry.Cmd.Part, "->", entry.Cmd.Id)

				if err := blockBucket.Put([]byte(entry.Cmd.FullId()), entry.Cmd.BytesDoc); err != nil {
					return err
				}

				if err := indexBatch.Index(entry.Cmd.FullId(), entry.Cmd.Doc); err != nil {
					return err
				}
			case command.DEL:

				p.logger.Debug("delete ", entry.Cmd.Part, "->", entry.Cmd.Id)

				if err := blockBucket.Delete([]byte(entry.Cmd.FullId())); err != nil {
					return err
				}

				indexBatch.Delete(entry.Cmd.FullId())
			}
		}

		return p.index.Batch(indexBatch)
	})

	// Have to panic if the update fails
	if updateErr != nil {
		panic(updateErr)
	}
}

func (p *Shard) Search(part string, searchStr string) (*searchResult, error) {

	// Make sure that we search in the part
	searchStr = "+part:"+part+" "+searchStr

	query := bleve.NewQueryStringQuery(searchStr)

	search := bleve.NewSearchRequest(query)
	blSearchResults, err := p.index.Search(search)
	if err != nil {
		return nil, err
	}

	res := searchResult{}

	viewErr := p.block.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(part))
		if b == nil {
			return nil
		}

		for _, hit := range blSearchResults.Hits {
			match := searchMatch{Doc: &document.Document{}}

			match.Score = hit.Score

			jsDoc := b.Get([]byte(hit.ID))
			if jsDoc == nil {
				//@TODO error here as index is incorrect
				continue
			}

			jsErr := json.Unmarshal(jsDoc, match.Doc)
			if jsErr != nil {
				return jsErr
			}

			res.Matches = append(res.Matches, &match)
		}

		return nil
	})

	if viewErr != nil {
		return nil, viewErr
	}

	return &res, nil
}
