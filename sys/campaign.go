package sys

import (
	"encoding/json"
	"fmt"

	"euphoria.io/heim/proto"
)

type Creative struct {
	UserID  proto.UserID
	Name    string
	Content string
}

func NewCreative(db *DB, userID proto.UserID, name, content string) (creative *Creative, replaced bool, err error) {
	creative = &Creative{
		UserID:  userID,
		Name:    name,
		Content: content,
	}
	err = db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		cs, err := b.CreateBucketIfNotExists([]byte("creatives"))
		if err != nil {
			return err
		}
		if cs.Get([]byte(name)) != nil {
			replaced = true
		}
		encoded, err := json.Marshal(creative)
		if err != nil {
			return err
		}
		cs.Put([]byte(name), encoded)
		return nil
	})
	return
}

func DeleteCreative(db *DB, userID proto.UserID, name string) (deleted bool, err error) {
	err = db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		cs, err := b.CreateBucketIfNotExists([]byte("creatives"))
		if err != nil {
			return err
		}
		if cs.Get([]byte(name)) != nil {
			deleted = true
			cs.Delete([]byte(name))
		}
		return nil
	})
	return
}

func Creatives(db *DB, userID proto.UserID) ([]Creative, error) {
	creatives := []Creative{}
	err := db.View(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		cs, err := b.CreateBucketIfNotExists([]byte("creatives"))
		if err != nil {
			return err
		}
		return cs.ForEach(func(k, v []byte) error {
			creative := Creative{}
			if err := json.Unmarshal(v, &creative); err != nil {
				return err
			}
			creatives = append(creatives, creative)
			return nil
		})
	})
	return creatives, err
}

type Spend struct {
	UserID       proto.UserID
	CreativeName string
	MaxBid       Cents
	Keywords     WordList
}

func NewSpend(db *DB, userID proto.UserID, creativeName, keywords string, maxBid Cents) (spend *Spend, replaced bool, err error) {
	spend = &Spend{
		UserID:       userID,
		CreativeName: creativeName,
		MaxBid:       maxBid,
		Keywords:     ParseWordList(keywords),
	}
	err = db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		ss, err := b.CreateBucketIfNotExists([]byte("spends"))
		if err != nil {
			return err
		}
		if ss.Get([]byte(creativeName)) != nil {
			replaced = true
		}
		encoded, err := json.Marshal(spend)
		if err != nil {
			return err
		}
		ss.Put([]byte(creativeName), encoded)

		globalKey := fmt.Sprintf("%s:%s", userID, creativeName)
		tx.SpendBucket().Put([]byte(globalKey), encoded)
		return nil
	})
	return
}

func DeleteSpend(db *DB, userID proto.UserID, creativeName string) (deleted bool, err error) {
	err = db.Update(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		ss, err := b.CreateBucketIfNotExists([]byte("spends"))
		if err != nil {
			return err
		}
		if ss.Get([]byte(creativeName)) != nil {
			deleted = true
			ss.Delete([]byte(creativeName))
		}

		globalKey := fmt.Sprintf("%s:%s", userID, creativeName)
		tx.SpendBucket().Delete([]byte(globalKey))
		return nil
	})
	return
}

func Spends(db *DB, userID proto.UserID) ([]Spend, error) {
	spends := []Spend{}
	err := db.View(func(tx *Tx) error {
		b, err := tx.AdvertiserBucket().CreateBucketIfNotExists([]byte(userID))
		if err != nil {
			return err
		}
		ss, err := b.CreateBucketIfNotExists([]byte("spends"))
		if err != nil {
			return err
		}
		return ss.ForEach(func(k, v []byte) error {
			spend := Spend{}
			if err := json.Unmarshal(v, &spend); err != nil {
				return err
			}
			spends = append(spends, spend)
			return nil
		})
	})
	return spends, err
}

func MatchSpends(db *DB, content string) ([]Spend, error) {
	words := ParseWordList(content)
	spends := []Spend{}
	err := db.View(func(tx *Tx) error {
		return tx.SpendBucket().ForEach(func(k, v []byte) error {
			spend := Spend{}
			if err := json.Unmarshal(v, &spend); err != nil {
				return err
			}
			if words.Match(spend.Keywords) {
				spends = append(spends, spend)
			}
			return nil
		})
	})
	return spends, err
}
