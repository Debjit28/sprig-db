package sprig

import (
	"fmt"

	"go.etcd.io/bbolt"
)

// Secondary index bucket naming: _idx_{collection}_{field}
// Each index bucket maps: field_value_bytes -> document_id_bytes

func indexBucketName(coll, field string) []byte {
	return []byte(fmt.Sprintf("_idx_%s_%s", coll, field))
}

// updateIndexes adds or updates index entries for all fields of a document.
func updateIndexes(tx *bbolt.Tx, coll string, id uint64, doc Map) error {
	for field, value := range doc {
		if field == "id" {
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
		if field == "id" {
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
