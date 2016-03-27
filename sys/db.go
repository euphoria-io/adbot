package sys

import (
	"reflect"
	"strings"

	"github.com/boltdb/bolt"
)

type DBFunc func(*Tx) error

func buckets() []string {
	bucketList := []string{}
	t := reflect.TypeOf(&Tx{})
	n := t.NumMethod()
	for i := 0; i < n; i++ {
		method := t.Method(i)
		if strings.HasSuffix(method.Name, "Bucket") {
			bucket := strings.ToLower(method.Name[:len(method.Name)-len("Bucket")])
			switch bucket {
			case "":
			case "check":
			case "create":
			case "delete":
			default:
				bucketList = append(bucketList, bucket)
			}
		}
	}
	return bucketList
}

func Open(path string) (*DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	sys := &DB{db}
	err = sys.Update(func(tx *Tx) error {
		for _, bucket := range buckets() {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
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

func (tx *Tx) AccountBucket() *bolt.Bucket    { return tx.Bucket([]byte("account")) }
func (tx *Tx) AdvertiserBucket() *bolt.Bucket { return tx.Bucket([]byte("advertiser")) }
func (tx *Tx) RoomBucket() *bolt.Bucket       { return tx.Bucket([]byte("room")) }
