package main

import (
	"os"
	"path/filepath"
	"github.com/blevesearch/bleve/v2"
	"github.com/boltdb/bolt"
	"encoding/json"
)

type searchMatch struct {
	Score float64 `json:"score"`
	Doc *Document `json:"doc"`
}

type searchResult struct {
	Matches []*searchMatch `json:"matches"`
}


type partition struct {
	index bleve.Index
	block *bolt.DB
	name string
}

func (p *partition) Close() {
	p.index.Close()
	p.block.Close()
}

func (p *partition) Add(doc *Document) error {

	// Marshal the document into json []bytes
	jsDoc, jsErr := json.Marshal(doc)
	// Best to return before the db transaction begins
	if jsErr != nil { return jsErr }

	// Begin a block bolt transaction
	return p.block.Update(func(tx *bolt.Tx) error {
		// The bucket may not yet exist, so we get or create it.
		b, bucketErr := tx.CreateBucketIfNotExists([]byte(p.name))
		if bucketErr != nil { return bucketErr }

		// The block db is updated first
		if err := b.Put([]byte(doc.Id), []byte(jsDoc)); err != nil {
			return err
		}

		// 
		if err := p.index.Index(doc.Id, doc.Data); err != nil {
			return err
		}
		
		return nil
	})
}

func (p *partition) Get(id string) (*Document, error) {

	ret := &Document{}

	vErr := p.block.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(p.name))

		v := b.Get([]byte(id))

		return json.Unmarshal(v, ret)
	})

	if vErr != nil {
		return nil, vErr
	}

	return ret, nil
}

func (p *partition) Delete(id string) error {

	return p.block.Update(func(tx *bolt.Tx) error {
		b, bucketErr := tx.CreateBucketIfNotExists([]byte(p.name))
		if bucketErr != nil { return bucketErr }

		if err := b.Delete([]byte(id)); err != nil {
			return err
		}

		if err := p.index.Delete(id); err != nil {
			return err
		}
		
		return nil
	})
}

func (p *partition) Search(searchStr string) (*searchResult, error) {
	query := bleve.NewMatchQuery(searchStr)

	search := bleve.NewSearchRequest(query)
	blSearchResults, err := p.index.Search(search)
	if err != nil {
		return nil, err
	}

	res := searchResult{}

	viewErr := p.block.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(p.name))
		for _, hit := range(blSearchResults.Hits) {
			match := searchMatch{ Doc:&Document{} }

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

func openOrCreateBleve(partitionPath string) (bleve.Index, error) {
    _, err := os.Stat(partitionPath)

    if os.IsNotExist(err) {
		mapping := bleve.NewIndexMapping()
		return bleve.New(partitionPath, mapping)
    } else {
		return bleve.Open(partitionPath)
	}
}

func OpenPartition(name string, partitionPath string) (*partition, error) {
	index, blErr := openOrCreateBleve(partitionPath)
	if blErr != nil { return nil, blErr }


	boltPath := filepath.Join(partitionPath, "block.bolt")
	block, boltErr := bolt.Open(boltPath, 0600, nil)
	if boltErr != nil {
		return nil, boltErr
	}

	part := partition{
		index: index,
		block: block,
		name: name,
	}

	return &part, nil
}



type store struct {
	basePath string
}

func (s *store) getPartition(part string) (*partition, error) {
	blevePath := filepath.Join(s.basePath, "part", part)
	return OpenPartition(part, blevePath)
}

func (s *store) Put(doc *Document) error {
	part, err := s.getPartition(doc.Part)
	if err != nil {
		return err
	}
	defer part.Close()

	return part.Add(doc)
}

func (s *store) Get(partStr string, id string) (*Document, error) {
	part, err := s.getPartition(partStr)
	if err != nil {
		return nil, err
	}
	defer part.Close()

	return part.Get(id)
}

func (s *store) Delete(partStr string, id string) error {
	part, err := s.getPartition(partStr)
	if err != nil {
		return err
	}
	defer part.Close()

	return part.Delete(id)
}

func (s *store) Search(partStr string, searchStr string) (*searchResult, error) {
	part, err := s.getPartition(partStr)
	if err != nil {
		return nil, err
	}
	defer part.Close()

	return part.Search(searchStr)
}

func (s *store) Purge() error {
	return os.RemoveAll(s.basePath)
}

func NewStore(basePath string) (*store) {
	return &store{ basePath: basePath }
}
