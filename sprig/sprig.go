package sprig

import (
	"fmt"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

const (
	defaultDBName = "admin"
)

type M map[string]string

type Sprig struct {
	db *bbolt.DB
}

type Collection struct {
	*bbolt.Bucket
}

func New() (*Sprig, error) {

	dbname := fmt.Sprintf("%s.sprig", defaultDBName)

	db, err := bbolt.Open(dbname, 0666, nil)

	if err != nil {
		return nil, err
	}

	return &Sprig{
		db: db,
	}, nil

}

func (s *Sprig) CreateCollection(name string) (*Collection, error) {
	coll := Collection{}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		var (
			err    error
			bucket *bbolt.Bucket
		)

		if bucket == nil {
			bucket, err = tx.CreateBucket([]byte(name))
			if err != nil {
				return err
			}

		}

		coll.Bucket = bucket

		return nil

	})

	if err != nil {

		return nil, err
	}

	return &coll, nil

}

func (s *Sprig) Insert(collName string, data M) (uuid.UUID, error) {

	id := uuid.New()

	coll, err := s.CreateCollection(collName)
	if err != nil {
		return id, err
	}

	for k, v := range data {

		if err := coll.Put([]byte(k), []byte(v)); err != nil {

			return id, err

		}

	}

	if err := coll.Put([]byte("id"), []byte(id.String())); err != nil {
		return id, err
	}

	return id, nil

}

func (s *Sprig) Select(coll string, k string, query any) {

}
