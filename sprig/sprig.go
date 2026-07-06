package sprig

import (
	"fmt"
	"os"
	"sync"

	"go.etcd.io/bbolt"
)

const (
	defaultDBName = "default"
	ext           = "sprig"
)

type Map map[string]any

// QueryResult holds paginated query results.
type QueryResult struct {
	Data   []Map `json:"data"`
	Total  int   `json:"total"`
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
}

type Sprig struct {
	mu              sync.RWMutex
	currentDatabase string
	*Options
	db *bbolt.DB
}

func New(options ...OptFunc) (*Sprig, error) {
	opts := &Options{
		Encoder: JSONEncoder{},
		Decoder: JSONDecoder{},
		DBName:  defaultDBName,
	}
	for _, fn := range options {
		fn(opts)
	}
	dbname := fmt.Sprintf("%s.%s", opts.DBName, ext)
	db, err := bbolt.Open(dbname, 0666, nil)
	if err != nil {
		return nil, err
	}
	return &Sprig{
		currentDatabase: dbname,
		db:              db,
		Options:         opts,
	}, nil
}

// Close cleanly closes the underlying bbolt database.
func (h *Sprig) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db != nil {
		return h.db.Close()
	}
	return nil
}

func (h *Sprig) DropDatabase(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.db != nil {
		h.db.Close()
	}
	dbname := fmt.Sprintf("%s.%s", name, ext)
	return os.Remove(dbname)
}

func (h *Sprig) CreateCollection(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	tx, err := h.db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.CreateBucketIfNotExists([]byte(name))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (h *Sprig) Coll(name string) *Filter {
	return NewFilter(h, name)
}

// ListCollections returns the names of all collections (top-level buckets).
func (h *Sprig) ListCollections() ([]string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var names []string
	err := h.db.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, _ *bbolt.Bucket) error {
			n := string(name)
			// Skip internal buckets (indexes, users)
			if len(n) > 0 && n[0] != '_' {
				names = append(names, n)
			}
			return nil
		})
	})
	return names, err
}

