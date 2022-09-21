package statemachine


import (
	"os"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/v2/mapping"
)


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
