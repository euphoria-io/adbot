package sys

import (
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"

	"euphoria.io/heim/proto"
)

const InitialBalance = 10000

type Cents int64

func (c Cents) String() string {
	if c%100 == 0 {
		return fmt.Sprintf("$%d", c/100)
	}
	return fmt.Sprintf("$%d.%02d", c/100, c%100)
}

type Advertiser struct {
	Nick    string
	Balance Cents
}

func getBalance(b *bolt.Bucket) (Cents, error) {
	bs := b.Get([]byte("balance"))
	if bs == nil {
		return InitialBalance, nil
	}
	c, err := strconv.ParseInt(string(bs), 10, 64)
	if err != nil {
		return 0, err
	}
	return Cents(c), nil
}

func GetAdvertiser(db *DB, userID proto.UserID) (*Advertiser, error) {
	advertiser := &Advertiser{Balance: InitialBalance}
	err := db.View(func(tx *Tx) error {
		b := tx.AdvertiserBucket().Bucket([]byte(userID))
		if b == nil {
			return nil
		}
		cents, err := getBalance(b)
		if err != nil {
			return err
		}
		advertiser.Nick = string(b.Get([]byte("nick")))
		advertiser.Balance = cents
		return nil
	})
	return advertiser, err
}

func SetNick(db *DB, userID proto.UserID, nick string) error {
	return db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		b.Put([]byte("nick"), []byte(nick))
		return nil
	})
}

func Debit(db *DB, userID proto.UserID, cents Cents) (Cents, error) {
	balance := -cents
	err := db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}

		cents, err := getBalance(b)
		if err != nil {
			return err
		}
		balance += cents
		b.Put([]byte("balance"), []byte(fmt.Sprintf("%d", balance)))
		return nil
	})
	if err != nil {
		return 0, err
	}
	return balance, nil
}

func Credit(db *DB, userID proto.UserID, cents Cents) (Cents, error) { return Debit(db, userID, -cents) }
