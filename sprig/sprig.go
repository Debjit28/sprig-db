package sprig

import (
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	defaultDBName = "admin"
)

type Sprig struct {
	db   *bbolt.DB
	name string
}

type Collection struct {
	db *bbolt.DB

	name string
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
	coll := Collection{
		db:   s.db,
		name: name,
	}
	err := s.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket([]byte(name))
		if err != nil {
			return err
		}

		return nil

	})

	if err != nil {

		return nil, err
	}

	return &coll, nil

}
