package main

import (
	"fmt"
	"log"

	"github.com/Debjit28/sprig-db/sprig"
)

func main() {

	db, err := sprig.New()

	if err != nil {

		log.Fatal(err)

	}

	user := map[string]string{
		"name": "Bhata",
		"age":  "45",
	}

	id, err := db.Insert("users", user)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", id)

}
