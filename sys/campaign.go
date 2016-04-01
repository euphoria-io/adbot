package sys

import (
	"encoding/json"
	"fmt"
	"strings"

	"euphoria.io/heim/proto"
)

var MissingCreative = Creative{
	UserID:  House,
	Content: "404 ad not found",
}

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
		b := tx.AdvertiserBucket().Bucket([]byte(userID))
		if b == nil {
			return nil
		}
		ss := b.Bucket([]byte("spends"))
		if ss == nil {
			return nil
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

func MapSpends(db *DB, mapper func(Spend) error) error {
	return db.View(func(tx *Tx) error {
		return tx.SpendBucket().ForEach(func(k, v []byte) error {
			spend := Spend{}
			if err := json.Unmarshal(v, &spend); err != nil {
				return err
			}
			return mapper(spend)
		})
	})
}

func ScaleSpends(db *DB, userID proto.UserID, factor float64) error {
	return db.Update(func(tx *Tx) error {
		b := tx.AdvertiserBucket().Bucket([]byte(userID))
		if b == nil {
			return nil
		}
		ss := b.Bucket([]byte("spends"))
		if ss == nil {
			return nil
		}
		c := ss.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			spend := Spend{}
			if err := json.Unmarshal(v, &spend); err != nil {
				return err
			}
			spend.MaxBid = Cents(float64(spend.MaxBid) * factor)
			var err error
			v, err = json.Marshal(spend)
			if err != nil {
				return err
			}
			ss.Put(k, v)

			globalKey := fmt.Sprintf("%s:%s", userID, spend.CreativeName)
			tx.SpendBucket().Put([]byte(globalKey), v)
		}
		return nil
	})
}

func Select(db *DB, content string, minBid Cents) (*Creative, Cents, error) {
	var (
		creative *Creative
		cost     Cents
	)

	words := ParseWordList(content)
	wl := []string{}
	for w, _ := range words {
		wl = append(wl, w)
	}
	fmt.Printf("auctioning %s at min bid %s\n", strings.Join(wl, ", "), minBid)

	err := db.View(func(tx *Tx) error {
		bids, err := getBids(tx, words, minBid)
		if err != nil {
			return err
		}

		winner, c, ok := bids.Auction()
		if !ok {
			return nil
		}
		cost = c

		b := tx.AdvertiserBucket().Bucket([]byte(winner.UserID))
		if b == nil {
			creative = &MissingCreative
			return nil
		}
		b = b.Bucket([]byte("creatives"))
		if b == nil {
			creative = &MissingCreative
			return nil
		}
		encoded := b.Get([]byte(winner.CreativeName))
		if encoded == nil {
			creative = &MissingCreative
			return nil
		}
		creative = new(Creative)
		if err := json.Unmarshal(encoded, creative); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	if creative != nil {
		fmt.Printf("selecting %s at %s\n", creative.Name, cost)
	}
	return creative, cost, nil
}

func Bill(db *DB, roomName string, userID proto.UserID, cost Cents, creativeName string, impressions int) error {
	memo := fmt.Sprintf("display %s in &%s at CPI of %s", creativeName, roomName, cost/Cents(impressions))
	if _, _, err := Transfer(db, cost, userID, System, memo, true); err != nil {
		return err
	}
	return SaveMetrics(db, userID, Metrics{
		AdsDisplayed: 1,
		Impressions:  uint64(impressions),
		AmountSpent:  uint64(cost),
	})
}

func ResetCampaigns(db *DB) error {
	return db.Update(func(tx *Tx) error {
		ab := tx.AdvertiserBucket()
		b := ab.Bucket([]byte("creatives"))
		if b != nil {
			if err := ab.DeleteBucket([]byte("creatives")); err != nil {
				return err
			}
		}
		b = ab.Bucket([]byte("spends"))
		if b != nil {
			if err := ab.DeleteBucket([]byte("spends")); err != nil {
				return err
			}
		}
		if err := tx.DeleteBucket([]byte("spend")); err != nil {
			return err
		}
		if _, err := tx.CreateBucket([]byte("spend")); err != nil {
			return err
		}
		return nil
	})
}
