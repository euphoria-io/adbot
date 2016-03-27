package sys

import (
	"github.com/boltdb/bolt"
)

type DBFunc func(*Tx) error

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	sys := &DB{db}
	err = sys.Update(func(tx *Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("account")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("room")); err != nil {
			return err
		}
		return nil
	})
	return sys, err
}

type DB struct {
	*bolt.DB
}

func (db *DB) Update(f DBFunc) error {
	return db.DB.Update(func(tx *bolt.Tx) error { return f(&Tx{tx}) })
}

func (db *DB) View(f DBFunc) error {
	return db.DB.View(func(tx *bolt.Tx) error { return f(&Tx{tx}) })
}

type Tx struct {
	*bolt.Tx
}

func (tx *Tx) AccountBucket() *bolt.Bucket { return tx.Bucket([]byte("account")) }
func (tx *Tx) RoomBucket() *bolt.Bucket    { return tx.Bucket([]byte("room")) }
