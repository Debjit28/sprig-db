package sprig

import (
	"testing"
)

func TestDelete(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	id, err := db.Coll("users").Insert(Map{"name": "foo"})
	if err != nil {
		t.Fatal(err)
	}
	delete := Map{"id": id}
	if err := db.Coll("users").Eq(delete).Delete(); err != nil {
		t.Fatal(err)
	}
	result, err := db.Coll("users").Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 0 {
		t.Fatalf("expected to have 0 records got %d", len(result.Data))
	}
}

func TestUpdate(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	_, err = db.Coll("users").Insert(Map{"name": "foo"})
	if err != nil {
		t.Fatal(err)
	}
	values := Map{"name": "bar"}
	results, err := db.Coll("users").Update(values)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected to have 1 result got %d", len(results))
	}
	result, err := db.Coll("users").Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected to have 1 result got %d", len(result.Data))
	}
	if result.Data[0]["name"] != values["name"] {
		t.Fatalf("expected name to be %s got %s", values["name"], result.Data[0]["name"])
	}
}

func TestInsert(t *testing.T) {
	values := []Map{
		{"name": "Foo", "age": 10},
		{"name": "Bar", "age": 88.3},
		{"name": "Baz", "age": 10},
	}

	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	for i, data := range values {
		id, err := db.Coll("users").Insert(data)
		if err != nil {
			t.Fatal(err)
		}
		if id != uint64(i+1) {
			t.Fatalf("expect ID %d got %d", i+1, id)
		}
	}
	result, err := db.Coll("users").Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != len(values) {
		t.Fatalf("expecting %d result got %d", len(values), len(result.Data))
	}
}

func TestFind(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	coll := "users"
	db.Coll(coll).Insert(Map{"username": "James007"})
	db.Coll(coll).Insert(Map{"username": "Alice"})
	db.Coll(coll).Insert(Map{"username": "Bob"})
	db.Coll(coll).Insert(Map{"username": "Mike"})

	result, err := db.Coll("users").Eq(Map{"username": "James007"}).Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("expected 1 result got %d", len(result.Data))
	}
}

func TestPagination(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	for i := 0; i < 10; i++ {
		_, err := db.Coll("items").Insert(Map{"index": i})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Get first page (5 items)
	result, err := db.Coll("items").Limit(5).Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 5 {
		t.Fatalf("expected 5 results got %d", len(result.Data))
	}
	if result.Total != 10 {
		t.Fatalf("expected total 10 got %d", result.Total)
	}

	// Get second page (5 items)
	result, err = db.Coll("items").Offset(5).Limit(5).Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 5 {
		t.Fatalf("expected 5 results got %d", len(result.Data))
	}

	// Offset beyond total
	result, err = db.Coll("items").Offset(20).Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 0 {
		t.Fatalf("expected 0 results got %d", len(result.Data))
	}
}

func TestCreateCollection(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	err = db.CreateCollection("products")
	if err != nil {
		t.Fatal(err)
	}

	// Verify we can insert into the collection
	id, err := db.Coll("products").Insert(Map{"name": "Widget"})
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("expected id 1 got %d", id)
	}
}

func TestListCollections(t *testing.T) {
	db, err := New(WithDBName("test"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test")

	db.Coll("users").Insert(Map{"name": "Alice"})
	db.Coll("products").Insert(Map{"name": "Widget"})

	collections, err := db.ListCollections()
	if err != nil {
		t.Fatal(err)
	}
	if len(collections) != 2 {
		t.Fatalf("expected 2 collections got %d", len(collections))
	}
}

func BenchmarkInsertMassive(b *testing.B) {
	db, err := New(WithDBName("test_bench_insert"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.DropDatabase("test_bench_insert")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Coll("test_bench_coll").Insert(Map{"index": i, "payload": "this is a test payload for benchmarking the storage capabilities of sprig-db"})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFindMassive(b *testing.B) {
	db, err := New(WithDBName("test_bench_find"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.DropDatabase("test_bench_find")

	for i := 0; i < 100; i++ {
		db.Coll("test_bench_coll").Insert(Map{"index": i, "username": "benchmark_user"})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Coll("test_bench_coll").Eq(Map{"username": "benchmark_user"}).Find()
		if err != nil {
			b.Fatal(err)
		}
	}
}
