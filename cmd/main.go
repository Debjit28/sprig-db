package main

import (
	"fmt"
	"log"

	"github.com/Debjit28/sprig-db/sprig"
)

func main() {

	user := map[string]string{
		"name": "Bhata",
		"age":  "45",
	}

	_ = user

	db, err := sprig.New()

	if err != nil {

		log.Fatal(err)

	}

	coll, err := db.CreateCollection("users")

	if err != nil {

		log.Fatal(err)

	}

	fmt.Printf("%+v\n", coll)

}
