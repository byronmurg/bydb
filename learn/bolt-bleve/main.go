package main

import (
	"fmt"
	"encoding/json"
)

func main() {
	store := NewStore("example_store")

	exampleDoc := Document{
		Id: "123",
		Part: "omanom",
		Data: map[string]any{
			"name": "Byron",
			"age": 31,
		},
	}

	if err := store.Put(&exampleDoc); err != nil {
		panic(err)
	}

	if result, err := store.Search("omanom", `Byron`); err != nil {
		panic(err)
	} else {
		jsRes, _ := json.Marshal(result)
		fmt.Println(string(jsRes))
	}


	if doc, err := store.Get("omanom", "123"); err != nil {
		panic(err)
	} else {
		jsRes, _ := json.Marshal(doc)
		fmt.Println(string(jsRes))
	}


	if result, err := store.Search("omanom", `Byron`); err != nil {
		panic(err)
	} else {
		jsRes, _ := json.Marshal(result)
		fmt.Println(string(jsRes))
	}

	CreateTarball("/tmp/examplestore.tgz", "example_store")

	//store.Purge()
}
