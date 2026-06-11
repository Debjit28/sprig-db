package main

import (
	"fmt"
	"log"

	"go.etcd.io/bbolt"
)

func main() {
	db, err := bbolt.Open(".db", 0666, nil)
	if err != nil {
		log.Fatal(err)
	}
	user := map[string]string{
		"name": "Bhata",
		"age":  "45",
	}
	db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucket([]byte("user"))
		if err != nil {
			return err
		}

		for k, v := range user {
			if err := bucket.Put([]byte(k), []byte(v)); err != nil {
				return err
			}
		}

		return nil
	})

	fmt.Println("works.!!!")
}
