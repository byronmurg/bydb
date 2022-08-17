package main

import (
	"fmt"

	"github.com/blevesearch/bleve/v2"
)

type Person struct {
	Name string
	Age int
	Alive bool
	Address map[string]string
}

func createIndex() {
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New("example.bleve", mapping)
	if err != nil {
		return
	}
	defer index.Close()

	data := Person{
		Name: "Byron",
		Age: 31,
		Alive: true,
		Address: map[string]string{
			"postcode": "W9 1UH",
		},
	}

	// index some data
	index.Index("id", data)
	return
}

func queryIndex(q string) {
	index, err := bleve.Open("example.bleve")
	if err != nil { panic(err) }
	defer index.Close()

	query := bleve.NewMatchQuery(q)

	search := bleve.NewSearchRequest(query)
	searchResults, err := index.Search(search)
	if err != nil {
		panic(err)
	}
	fmt.Println(q, searchResults)
}

func main() {
	createIndex()
	queryIndex(`name:"Foo"`)
	queryIndex(`address.postcode:W9\ 1UH`)
}

