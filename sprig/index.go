package sprig

import (
	"fmt"

	"go.etcd.io/bbolt"
)

// Secondary index bucket naming: _idx_{collection}_{field}
// Each index bucket maps: field_value_bytes -> document_id_bytes

// sensitiveFields are fields that should never be indexed.
var sensitiveFields = map[string]bool{
	"password": true,
	"secret":   true,
	"token":    true,
}

func indexBucketName(coll, field string) []byte {
	return []byte(fmt.Sprintf("_idx_%s_%s", coll, field))
}

// updateIndexes adds or updates index entries for all fields of a document.
func updateIndexes(tx *bbolt.Tx, coll string, id uint64, doc Map) error {
	for field, value := range doc {
		if field == "id" || sensitiveFields[field] {
			continue
		}
		bucketName := indexBucketName(coll, field)
		idxBucket, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return err
		}
		key := []byte(fmt.Sprintf("%v:%d", value, id))
		if err := idxBucket.Put(key, uint64Bytes(id)); err != nil {
			return err
		}
	}
	return nil
}

// removeIndexes removes index entries for all fields of a document.
func removeIndexes(tx *bbolt.Tx, coll string, id uint64, doc Map) error {
	for field, value := range doc {
		if field == "id" || sensitiveFields[field] {
			continue
		}
		bucketName := indexBucketName(coll, field)
		idxBucket := tx.Bucket(bucketName)
		if idxBucket == nil {
			continue
		}
		key := []byte(fmt.Sprintf("%v:%d", value, id))
		if err := idxBucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

// lookupByIndex queries the secondary index for a given collection, field, and value.
// Returns the list of matching document IDs.
func lookupByIndex(tx *bbolt.Tx, coll, field string, value any) ([]uint64, error) {
	bucketName := indexBucketName(coll, field)
	idxBucket := tx.Bucket(bucketName)
	if idxBucket == nil {
		return nil, nil // no index exists, caller should fall back to full scan
	}

	prefix := []byte(fmt.Sprintf("%v:", value))
	var ids []uint64
	c := idxBucket.Cursor()
	for k, v := c.Seek(prefix); k != nil; k, v = c.Next() {
		// Check that the key still starts with our prefix
		if len(k) < len(prefix) {
			break
		}
		match := true
		for i := 0; i < len(prefix); i++ {
			if k[i] != prefix[i] {
				match = false
				break
			}
		}
		if !match {
			break
		}
		ids = append(ids, uint64FromBytes(v))
	}
	return ids, nil
}
