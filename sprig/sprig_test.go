package sprig

import (
	"sync"
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
	results, err := db.Coll("users").Eq(Map{"name": "foo"}).Update(values)
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

// --- New tests for the fixes ---

func TestIndexAcceleratedFind(t *testing.T) {
	db, err := New(WithDBName("test_idx"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_idx")

	// Insert 100 docs with a "city" field.
	for i := 0; i < 100; i++ {
		city := "Berlin"
		if i%10 == 0 {
			city = "Tokyo"
		}
		_, err := db.Coll("people").Insert(Map{"name": "person", "city": city})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Query with single-field Eq — should use index path.
	result, err := db.Coll("people").Eq(Map{"city": "Tokyo"}).Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 10 {
		t.Fatalf("expected 10 Tokyo results, got %d", len(result.Data))
	}
	if result.Total != 10 {
		t.Fatalf("expected total 10, got %d", result.Total)
	}

	// Verify all returned docs actually have city=Tokyo.
	for _, doc := range result.Data {
		if doc["city"] != "Tokyo" {
			t.Fatalf("expected city=Tokyo, got %v", doc["city"])
		}
	}
}

func TestConcurrentInsertFind(t *testing.T) {
	db, err := New(WithDBName("test_concurrent"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_concurrent")

	var wg sync.WaitGroup
	errCh := make(chan error, 200)

	// 10 writers, each inserting 50 docs.
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, err := db.Coll("concurrent").Insert(Map{"worker": workerID, "index": i})
				if err != nil {
					errCh <- err
					return
				}
			}
		}(w)
	}

	// 10 readers running concurrently.
	for r := 0; r < 10; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := db.Coll("concurrent").Find()
				if err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for e := range errCh {
		t.Fatalf("concurrent error: %v", e)
	}

	// Verify final count = 10 writers × 50 docs = 500.
	result, err := db.Coll("concurrent").Find()
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 500 {
		t.Fatalf("expected 500 total docs, got %d", result.Total)
	}
}

func TestUnfilteredUpdateBlocked(t *testing.T) {
	db, err := New(WithDBName("test_nofilter"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_nofilter")

	db.Coll("items").Insert(Map{"name": "a"})
	db.Coll("items").Insert(Map{"name": "b"})

	// Update without filter should fail.
	_, err = db.Coll("items").Update(Map{"name": "hacked"})
	if err == nil {
		t.Fatal("expected error for unfiltered Update, got nil")
	}
	if err != ErrNoFilter {
		t.Fatalf("expected ErrNoFilter, got %v", err)
	}

	// Verify data is unchanged.
	result, _ := db.Coll("items").Find()
	for _, doc := range result.Data {
		if doc["name"] == "hacked" {
			t.Fatal("unfiltered update should not have modified data")
		}
	}
}

func TestUnfilteredDeleteBlocked(t *testing.T) {
	db, err := New(WithDBName("test_nofilter_del"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_nofilter_del")

	db.Coll("items").Insert(Map{"name": "a"})
	db.Coll("items").Insert(Map{"name": "b"})

	// Delete without filter should fail.
	err = db.Coll("items").Delete()
	if err == nil {
		t.Fatal("expected error for unfiltered Delete, got nil")
	}
	if err != ErrNoFilter {
		t.Fatalf("expected ErrNoFilter, got %v", err)
	}

	// Verify data still exists.
	result, _ := db.Coll("items").Find()
	if len(result.Data) != 2 {
		t.Fatalf("expected 2 docs after blocked delete, got %d", len(result.Data))
	}
}

func TestBigEndianKeyOrder(t *testing.T) {
	db, err := New(WithDBName("test_keyorder"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_keyorder")

	// Insert 3 docs — IDs will be 1, 2, 3.
	db.Coll("ordered").Insert(Map{"val": "first"})
	db.Coll("ordered").Insert(Map{"val": "second"})
	db.Coll("ordered").Insert(Map{"val": "third"})

	result, err := db.Coll("ordered").Find()
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(result.Data))
	}

	// Verify iteration order matches insertion order.
	expected := []string{"first", "second", "third"}
	for i, doc := range result.Data {
		if doc["val"] != expected[i] {
			t.Fatalf("doc %d: expected val=%s, got %v", i, expected[i], doc["val"])
		}
	}
}

func TestSensitiveFieldsNotIndexed(t *testing.T) {
	db, err := New(WithDBName("test_sensitive"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.DropDatabase("test_sensitive")

	// Insert a user with a password field.
	db.Coll("users").Insert(Map{"username": "alice", "password": "secret123"})

	// The password field should NOT have an index bucket.
	// We can verify by checking that the index lookup returns nil (no bucket).
	tx, err := db.db.Begin(false)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	idxBucket := tx.Bucket(indexBucketName("users", "password"))
	if idxBucket != nil {
		t.Fatal("password field should not be indexed, but index bucket exists")
	}

	// Username should be indexed.
	usernameBucket := tx.Bucket(indexBucketName("users", "username"))
	if usernameBucket == nil {
		t.Fatal("username field should be indexed, but index bucket does not exist")
	}
}

// --- Benchmarks ---

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
