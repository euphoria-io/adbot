package sys

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/boltdb/bolt"

	"euphoria.io/heim/proto"
	"euphoria.io/heim/proto/snowflake"
)

const (
	InitialBalance = 10000
	House          = "house"
	System         = "system"
)

var ErrInsufficientFunds = fmt.Errorf("insufficient funds")

type Cents int64

type LedgerEntry struct {
	TxID    snowflake.Snowflake
	Cents   Cents
	Balance Cents
	From    proto.UserID
	To      proto.UserID
	Memo    string
}

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

func getBalance(userID proto.UserID, b *bolt.Bucket) (Cents, error) {
	bs := b.Get([]byte("balance"))
	if bs == nil {
		if userID == House || userID == System {
			return 0, nil
		}
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
		cents, err := getBalance(userID, b)
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
		return b.Put([]byte("nick"), []byte(nick))
	})
}

func Transfer(db *DB, cents Cents, from, to proto.UserID, memo string, force ...bool) (fromBalance, toBalance Cents, err error) {
	err = db.Update(func(tx *Tx) error {
		fromBucket, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(from))
		if err != nil {
			return err
		}
		fromLedger, err := fromBucket.CreateBucketIfNotExists([]byte("ledger"))
		if err != nil {
			return err
		}
		fromBalance, err = getBalance(from, fromBucket)
		if err != nil {
			return err
		}

		toBucket, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(to))
		if err != nil {
			return err
		}
		toLedger, err := toBucket.CreateBucketIfNotExists([]byte("ledger"))
		if err != nil {
			return err
		}
		toBalance, err = getBalance(to, toBucket)
		if err != nil {
			return err
		}

		fromBalance -= cents
		toBalance += cents
		if fromBalance < 0 && (len(force) == 0 || !force[0]) && from != House && from != System {
			return ErrInsufficientFunds
		}

		txID, err := snowflake.New()
		if err != nil {
			return err
		}
		fromEntry := LedgerEntry{
			TxID:    txID,
			Cents:   cents,
			From:    from,
			To:      to,
			Memo:    memo,
			Balance: fromBalance,
		}
		fromEntryBytes, err := json.Marshal(fromEntry)
		if err != nil {
			return err
		}
		toEntry := LedgerEntry{
			TxID:    txID,
			Cents:   cents,
			From:    from,
			To:      to,
			Memo:    memo,
			Balance: toBalance,
		}
		toEntryBytes, err := json.Marshal(toEntry)
		if err != nil {
			return err
		}

		fromBucket.Put([]byte("balance"), []byte(fmt.Sprintf("%d", fromBalance)))
		fromLedger.Put([]byte(txID.String()), fromEntryBytes)
		toBucket.Put([]byte("balance"), []byte(fmt.Sprintf("%d", toBalance)))
		toLedger.Put([]byte(txID.String()), toEntryBytes)
		return nil
	})
	return
}

func Ledger(db *DB, userID proto.UserID, maxEntries int) ([]LedgerEntry, error) {
	entries := []LedgerEntry{}
	err := db.View(func(tx *Tx) error {
		b := tx.AdvertiserBucket().Bucket([]byte(userID))
		if b != nil {
			fmt.Printf("looking up ledger\n")
			b = b.Bucket([]byte("ledger"))
		}
		if b == nil {
			fmt.Printf("no ledger\n")
			return nil
		}
		fmt.Printf("trying cursor\n")
		c := b.Cursor()
		for k, v := c.Last(); k != nil && len(entries) < maxEntries; k, v = c.Prev() {
			entry := LedgerEntry{}
			if err := json.Unmarshal(v, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(entries)/2; i++ {
		entries[i], entries[len(entries)-i-1] = entries[len(entries)-i-1], entries[i]
	}
	return entries, nil
}

func ResetBalances(db *DB) error {
	return db.Update(func(tx *Tx) error {
		userBuckets := tx.AdvertiserBucket()
		err := userBuckets.ForEach(func(k, v []byte) error {
			if v == nil {
				b := userBuckets.Bucket(k)
				b.Delete([]byte("balance"))
				lb := b.Bucket([]byte("ledger"))
				if lb != nil {
					if err := b.DeleteBucket([]byte("ledger")); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		if err := tx.DeleteBucket([]byte("metrics")); err != nil {
			return err
		}
		if _, err := tx.CreateBucket([]byte("metrics")); err != nil {
			return err
		}
		return nil
	})
}
