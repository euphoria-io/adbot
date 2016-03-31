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
	House  = "house"
	System = "system"
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

func getBalance(tx *Tx, userID proto.UserID) (Cents, error) {
	var b *bolt.Bucket
	if tx.Writable() {
		var err error
		b, err = tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return 0, err
		}
	} else {
		b = tx.AdvertiserBucket().Bucket([]byte(userID))
	}
	var bs []byte
	if b != nil {
		bs = b.Get([]byte("balance"))
	}
	if bs == nil {
		if userID == House || userID == System {
			return 0, nil
		}
		sb := tx.StimulusBucket()
		bs := sb.Get([]byte("stimulus"))
		if bs != nil {
			c, err := strconv.ParseInt(string(bs), 10, 64)
			if err != nil {
				return 0, err
			}
			stimulus := Cents(c)
			if tx.Writable() {
				_, balance, err := transfer(tx, stimulus, House, userID, "euphoria commercial speech stimulus", true)
				return balance, err
			}
			return stimulus, nil
		}
		return 0, nil
	}
	c, err := strconv.ParseInt(string(bs), 10, 64)
	if err != nil {
		return 0, err
	}
	return Cents(c), nil
}

func GetAdvertiser(db *DB, userID proto.UserID) (*Advertiser, error) {
	advertiser := &Advertiser{}
	err := db.View(func(tx *Tx) error {
		b := tx.AdvertiserBucket().Bucket([]byte(userID))
		if b == nil {
			return nil
		}
		cents, err := getBalance(tx, userID)
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

func transfer(tx *Tx, cents Cents, from, to proto.UserID, memo string, force bool) (fromBalance, toBalance Cents, err error) {
	fromBucket, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(from))
	if err != nil {
		return
	}
	fromLedger, err := fromBucket.CreateBucketIfNotExists([]byte("ledger"))
	if err != nil {
		return
	}
	fromBalance, err = getBalance(tx, from)
	if err != nil {
		return
	}

	toBucket, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(to))
	if err != nil {
		return
	}
	toLedger, err := toBucket.CreateBucketIfNotExists([]byte("ledger"))
	if err != nil {
		return
	}

	bs := toBucket.Get([]byte("balance"))
	if bs != nil {
		var c int64
		c, err = strconv.ParseInt(string(bs), 10, 64)
		if err != nil {
			return
		}
		toBalance = Cents(c)
	}

	fromBalance -= cents
	toBalance += cents
	if fromBalance < 0 && !force && from != House && from != System {
		err = ErrInsufficientFunds
		return
	}

	txID, err := snowflake.New()
	if err != nil {
		return
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
		return
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
		return
	}

	fromBucket.Put([]byte("balance"), []byte(fmt.Sprintf("%d", fromBalance)))
	fromLedger.Put([]byte(txID.String()), fromEntryBytes)
	toBucket.Put([]byte("balance"), []byte(fmt.Sprintf("%d", toBalance)))
	toLedger.Put([]byte(txID.String()), toEntryBytes)
	return
}

func Transfer(db *DB, cents Cents, from, to proto.UserID, memo string, force ...bool) (fromBalance, toBalance Cents, err error) {
	err = db.Update(func(tx *Tx) error {
		var err error
		fromBalance, toBalance, err = transfer(tx, cents, from, to, memo, len(force) > 0 && force[0])
		return err
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

func AddStimulus(db *DB, amount Cents) error {
	return db.Update(func(tx *Tx) error {
		b := tx.StimulusBucket()
		former := Cents(0)
		bs := b.Get([]byte("stimulus"))
		if bs != nil {
			c, err := strconv.ParseInt(string(bs), 10, 64)
			if err != nil {
				return err
			}
			former = Cents(c)
		}
		b.Put([]byte("stimulus"), []byte(fmt.Sprintf("%d", former+amount)))

		memo := "euphoria commercial speech stimulus refresher"
		c := tx.AdvertiserBucket().Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			userID := proto.UserID(k)
			if userID == House || userID == System {
				continue
			}
			if _, _, err := transfer(tx, amount, House, userID, memo, true); err != nil {
				return err
			}
		}
		return nil
	})
}
