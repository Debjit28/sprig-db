package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/Debjit28/sprig-db/api"
	"github.com/Debjit28/sprig-db/sprig"
)

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	db, err := sprig.New()
	if err != nil {
		log.Fatalf("failed to open sprig db: %v", err)
	}
	defer db.Close()

	owner := "zoro"
	collName := "employee"

	// Define JSON schema for the employee collection (simpler version).
	jsonSchema := map[string]any{
		"type": "object",
		"required": []any{"name", "dob", "post", "years_working"},
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"minLength": 2.0,
				"maxLength": 80.0,
			},
			"dob": map[string]any{
				"type":      "string",
				"minLength": 8.0,
				"maxLength": 20.0,
			},
			"post": map[string]any{
				"type":      "string",
				"minLength": 2.0,
				"maxLength": 80.0,
			},
			"years_working": map[string]any{
				"type":    "number",
				"minimum": 0.0,
				"maximum": 40.0,
			},
		},
	}

	// Convert JSON schema into internal schema and upsert it for owner "zoro".
	schema, err := api.ConvertJSONSchemaToCollectionSchema(collName, jsonSchema)
	if err != nil {
		log.Fatalf("failed to convert employee json schema: %v", err)
	}
	if err := db.CreateCollection(collName); err != nil {
		log.Printf("CreateCollection(%s) warning: %v", collName, err)
	}
	if err := api.NewSchemaStore(db).Upsert(owner, *schema); err != nil {
		log.Fatalf("failed to upsert employee schema: %v", err)
	}

	posts := []string{
		"Junior Developer",
		"Senior Developer",
		"Team Lead",
		"HR Specialist",
		"Account Executive",
		"Support Engineer",
		"Product Manager",
		"Data Analyst",
		"QA Engineer",
		"Designer",
	}

	// Generate between 20 and 40 employees.
	count := 30
	inserted := 0
	for i := 0; i < count; i++ {
		name := names[i%len(names)]
		post := posts[rng.Intn(len(posts))]
		years := rng.Intn(21) // 0–20
		dob := randomDOB(rng)

		doc := sprig.Map{
			"name":          name,
			"dob":           dob,
			"post":          post,
			"years_working": float64(years),
			"_owner":        owner,
		}

		if _, err := db.Coll(collName).Insert(doc); err != nil {
			log.Printf("insert employee %d failed: %v", i+1, err)
			continue
		}
		inserted++
	}

	log.Printf("inserted %d employee documents into collection %q for owner %q", inserted, collName, owner)
}

var names = []string{
	"Alice Johnson",
	"Bob Smith",
	"Carol Davis",
	"David Wilson",
	"Eve Brown",
	"Frank Taylor",
	"Grace Anderson",
	"Henry Thomas",
	"Irene Jackson",
	"Jack White",
	"Karen Harris",
	"Liam Martin",
	"Mia Thompson",
	"Noah Garcia",
	"Olivia Martinez",
	"Paul Robinson",
	"Quinn Clark",
	"Rita Rodriguez",
	"Sam Lewis",
	"Tina Lee",
	"Uma Patel",
	"Victor Singh",
	"Wendy Zhou",
	"Xavier Khan",
	"Yara Mehta",
	"Zane Kapoor",
}

func randomDOB(rng *rand.Rand) string {
	year := rng.Intn(25) + 1970 // 1970–1994
	month := rng.Intn(12) + 1
	day := rng.Intn(28) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

