package store

import (
	"os"
	"omanom.com/bydb/logger"
	"path/filepath"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/boltdb/bolt"
	"encoding/json"

	. "omanom.com/bydb/document"
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
	logger *logger.Logger
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
		if err := p.index.Index(doc.Id, doc); err != nil {
			return err
		}
		
		return nil
	})
}

func (p *partition) GetRaw(id string) ([]byte, error) {

	// Bolt returns internal bytes so we need a buffer
	// to copy results into
	var raw []byte

	// Start a bolt read transaction
	err := p.block.View(func(tx *bolt.Tx) error {
		// Get the bucket, if nil is returned then
		// the bucket doesn't exist so just return
		b := tx.Bucket([]byte(p.name))
		if b == nil { return nil }

		// Get the raw bytes and copy them into the
		// return buffer
		doc := b.Get([]byte(id))
		raw = make([]byte, len(doc))
		copy(raw, doc)

		return nil
	})

	return raw, err
}

func (p *partition) Get(id string) (*Document, error) {
	// This method just calls GetRaw and unmarshals the
	// result into a Document structure

	doc := &Document{}

	rawDoc, getErr := p.GetRaw(id)

	if getErr != nil {
		return nil, getErr
	}

	jsErr := json.Unmarshal(rawDoc, doc)

	return doc, jsErr
}

func (p *partition) Delete(id string) error {

	return p.block.Update(func(tx *bolt.Tx) error {
		b, bucketErr := tx.CreateBucketIfNotExists([]byte(p.name))
		if bucketErr != nil { return bucketErr }

		// Delete from bolt. This won't be commited until
		// the end of this function
		if err := b.Delete([]byte(id)); err != nil {
			return err
		}

		// Delete from the index. If this fails then
		// returning an error will cancel the bolt
		// transaction also.
		if err := p.index.Delete(id); err != nil {
			return err
		}
		
		return nil
	})
}

func (p *partition) Search(searchStr string) (*searchResult, error) {
	p.logger.Debugf(`search in part %s with match (%s)`, p.name, searchStr)

	query := bleve.NewQueryStringQuery(searchStr)

	search := bleve.NewSearchRequest(query)
	blSearchResults, err := p.index.Search(search)
	if err != nil {
		return nil, err
	}

	res := searchResult{}

	viewErr := p.block.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(p.name))
		if b == nil { return nil }

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

func OpenPartition(name string, partitionPath string, logger *logger.Logger) (*partition, error) {
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
		logger: logger,
	}

	return &part, nil
}



type store struct {
	basePath string
	logger *logger.Logger
}

func (s *store) getPartition(part string) (*partition, error) {
	blevePath := filepath.Join(s.basePath, "part", part)
	return OpenPartition(part, blevePath, s.logger)
}

func (s *store) Put(doc *Document) error {
	s.logger.Debugf("store PUT part:%s id:%s", doc.Part, doc.Id)
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

func (s *store) GetRaw(partStr string, id string) ([]byte, error) {
	part, err := s.getPartition(partStr)
	if err != nil {
		return nil, err
	}
	defer part.Close()

	return part.GetRaw(id)
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

// Export as-is
type Store = store

func NewStore(basePath string) (*store) {
	return &store{
		basePath: basePath,
		logger: logger.New("store"),
	}
}
