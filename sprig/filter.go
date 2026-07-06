package sprig

import (
	"errors"
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	FilterTypeEQ = "eq"
)

// ErrNoFilter is returned when Update or Delete is called without any filters.
var ErrNoFilter = errors.New("refusing to mutate without filters — add at least one Eq() filter or use a direct approach")

func eq(a, b any) bool {
	return a == b
}

type comparison func(a, b any) bool

type compFilter struct {
	kvs  Map
	comp comparison
}

func (f compFilter) apply(record Map) bool {
	for k, v := range f.kvs {
		value, ok := record[k]
		if !ok {
			return false
		}
		if k == "id" {
			// Handle both int and uint64 types for id comparison.
			switch vid := v.(type) {
			case int:
				return f.comp(value, uint64(vid))
			case uint64:
				return f.comp(value, vid)
			case float64:
				return f.comp(value, uint64(vid))
			default:
				return f.comp(value, v)
			}
		}
		return f.comp(value, v)
	}
	return true
}

type Filter struct {
	hopper      *Sprig
	coll        string
	compFilters []compFilter
	slct        []string
	limit       int
	offset      int
}

func NewFilter(db *Sprig, coll string) *Filter {
	return &Filter{
		hopper:      db,
		coll:        coll,
		compFilters: make([]compFilter, 0),
	}
}

func (f *Filter) Eq(values Map) *Filter {
	filt := compFilter{
		comp: eq,
		kvs:  values,
	}
	f.compFilters = append(f.compFilters, filt)
	return f
}

// Insert inserts the given values into the collection.
func (f *Filter) Insert(values Map) (uint64, error) {
	f.hopper.mu.Lock()
	defer f.hopper.mu.Unlock()

	tx, err := f.hopper.db.Begin(true)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	collBucket, err := tx.CreateBucketIfNotExists([]byte(f.coll))
	if err != nil {
		return 0, err
	}
	id, err := collBucket.NextSequence()
	if err != nil {
		return 0, err
	}
	b, err := f.hopper.Encoder.Encode(values)
	if err != nil {
		return 0, err
	}
	if err := collBucket.Put(uint64Bytes(id), b); err != nil {
		return 0, err
	}

	// Maintain secondary indexes.
	if err := updateIndexes(tx, f.coll, id, values); err != nil {
		return 0, err
	}

	return id, tx.Commit()
}

// Find returns paginated, filtered query results.
// Uses index-accelerated lookups when a single-field Eq filter is present.
func (f *Filter) Find() (*QueryResult, error) {
	f.hopper.mu.RLock()
	defer f.hopper.mu.RUnlock()

	tx, err := f.hopper.db.Begin(false)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return &QueryResult{Data: []Map{}, Total: 0, Offset: f.offset, Limit: f.limit}, nil
	}

	// Try index-accelerated path: single Eq filter on a single field.
	if records, total, ok := f.tryIndexFind(tx, bucket); ok {
		return &QueryResult{
			Data:   records,
			Total:  total,
			Offset: f.offset,
			Limit:  f.limit,
		}, nil
	}

	// Fallback: full scan.
	records, total, err := f.findFiltered(bucket)
	if err != nil {
		return nil, err
	}
	return &QueryResult{
		Data:   records,
		Total:  total,
		Offset: f.offset,
		Limit:  f.limit,
	}, nil
}

// tryIndexFind attempts to use a secondary index for a single-field Eq filter.
// Returns (records, total, true) on success, or (nil, 0, false) if index path is not applicable.
func (f *Filter) tryIndexFind(tx *bbolt.Tx, bucket *bbolt.Bucket) ([]Map, int, bool) {
	if len(f.compFilters) != 1 {
		return nil, 0, false
	}
	filter := f.compFilters[0]
	if len(filter.kvs) != 1 {
		return nil, 0, false
	}

	// Extract the single field and value.
	var field string
	var value any
	for k, v := range filter.kvs {
		field = k
		value = v
	}

	// Skip index for "id" lookups — those are direct key lookups.
	if field == "id" {
		return nil, 0, false
	}

	ids, err := lookupByIndex(tx, f.coll, field, value)
	if err != nil || ids == nil {
		return nil, 0, false // no index, fall back
	}

	// Fetch documents by their IDs from the index.
	var all []Map
	for _, id := range ids {
		v := bucket.Get(uint64Bytes(id))
		if v == nil {
			continue // document was deleted but index is stale
		}
		record := Map{"id": id}
		if err := f.hopper.Decoder.Decode(v, &record); err != nil {
			continue
		}
		// Double-check the filter matches (index may be stale after updates).
		if filter.apply(record) {
			all = append(all, record)
		}
	}

	total := len(all)

	// Apply offset.
	start := f.offset
	if start > total {
		start = total
	}
	result := all[start:]

	// Apply limit.
	if f.limit > 0 && len(result) > f.limit {
		result = result[:f.limit]
	}

	// Apply select projection.
	for i, record := range result {
		result[i] = f.applySelect(record)
	}

	if result == nil {
		result = []Map{}
	}

	return result, total, true
}

// Update updates matching documents with the given values.
func (f *Filter) Update(values Map) ([]Map, error) {
	if len(f.compFilters) == 0 {
		return nil, ErrNoFilter
	}

	f.hopper.mu.Lock()
	defer f.hopper.mu.Unlock()

	tx, err := f.hopper.db.Begin(true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return nil, fmt.Errorf("bucket (%s) not found", f.coll)
	}
	records, _, err := f.findFiltered(bucket)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		for k, v := range values {
			record[k] = v
		}
		b, err := f.hopper.Encoder.Encode(record)
		if err != nil {
			return nil, err
		}
		id := record["id"].(uint64)
		if err := bucket.Put(uint64Bytes(id), b); err != nil {
			return nil, err
		}
		if err := updateIndexes(tx, f.coll, id, record); err != nil {
			return nil, err
		}
	}
	return records, tx.Commit()
}

// Delete deletes all matching documents.
func (f *Filter) Delete() error {
	if len(f.compFilters) == 0 {
		return ErrNoFilter
	}

	f.hopper.mu.Lock()
	defer f.hopper.mu.Unlock()

	tx, err := f.hopper.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return fmt.Errorf("bucket (%s) not found", f.coll)
	}
	records, _, err := f.findFiltered(bucket)
	if err != nil {
		return err
	}
	for _, r := range records {
		id := r["id"].(uint64)
		idbytes := uint64Bytes(id)
		if err := bucket.Delete(idbytes); err != nil {
			return err
		}
		if err := removeIndexes(tx, f.coll, id, r); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Limit sets the maximum number of results to return.
func (f *Filter) Limit(n int) *Filter {
	f.limit = n
	return f
}

// Offset sets the number of results to skip.
func (f *Filter) Offset(n int) *Filter {
	f.offset = n
	return f
}

// Select specifies which fields to include in results.
func (f *Filter) Select(values ...string) *Filter {
	f.slct = append(f.slct, values...)
	return f
}

func (f *Filter) findFiltered(bucket *bbolt.Bucket) ([]Map, int, error) {
	all := []Map{}
	err := bucket.ForEach(func(k, v []byte) error {
		record := Map{
			"id": uint64FromBytes(k),
		}
		if err := f.hopper.Decoder.Decode(v, &record); err != nil {
			return err
		}
		include := true
		for _, filter := range f.compFilters {
			if !filter.apply(record) {
				include = false
				break
			}
		}
		if include {
			all = append(all, record)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	total := len(all)

	// Apply offset.
	start := f.offset
	if start > total {
		start = total
	}
	result := all[start:]

	// Apply limit.
	if f.limit > 0 && len(result) > f.limit {
		result = result[:f.limit]
	}

	// Apply select projection.
	for i, record := range result {
		result[i] = f.applySelect(record)
	}

	return result, total, nil
}

func (f *Filter) applySelect(record Map) Map {
	if len(f.slct) == 0 {
		return record
	}
	data := Map{}
	for _, key := range f.slct {
		if _, ok := record[key]; ok {
			data[key] = record[key]
		}
	}
	return data
}
