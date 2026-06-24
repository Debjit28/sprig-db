package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
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
		bucket, err := tx.CreateBucket([]byte("users"))
		if err != nil {
			return err
		}

		id := uuid.New()

		for k, v := range user {
			if err := bucket.Put([]byte(k), []byte(v)); err != nil {
				return err
			}
		}

		if err := bucket.Put([]byte("id"), []byte(id.String())); err != nil {
			return err
		}

		return nil
	})

	userData := make(map[string]string)
	if err := db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		if bucket == nil {

			return fmt.Errorf("bucket (%s) not found", "users")

		}

		bucket.ForEach(func(k, v []byte) error {
			userData[string(k)] = string(v)
			return nil
		})

		return nil
	}); err != nil {
		log.Fatal(err)
	}

	fmt.Println(user)
}
