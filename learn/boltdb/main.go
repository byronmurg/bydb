package main

import (
	"log"
	"github.com/boltdb/bolt"
	"fmt"
)

func main() {
	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})	

	db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("MyBucket"))
		if err != nil {
			return err
		}
		return b.Put([]byte("foo"), []byte("bar"))
	})

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("MyBucket"))
		v := b.Get([]byte("foo"))
		fmt.Printf("get recieved %s\n", v)
		return nil
	})
}
