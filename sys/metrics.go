package sys

import (
	"encoding/json"

	"github.com/boltdb/bolt"

	"euphoria.io/heim/proto"
)

type Metrics struct {
	AdsDisplayed       uint64
	Impressions        uint64
	AmountSpent        uint64
	AmountSpentByHouse uint64
}

func (m *Metrics) Incr(n Metrics) *Metrics {
	m.AdsDisplayed += n.AdsDisplayed
	m.Impressions += n.Impressions
	m.AmountSpent += n.AmountSpent
	m.AmountSpentByHouse += n.AmountSpentByHouse
	return m
}

func (m *Metrics) Load(b *bolt.Bucket, key []byte) error {
	encoded := b.Get(key)
	if encoded == nil {
		return nil
	}
	return json.Unmarshal(encoded, m)
}

func (m *Metrics) Save(b *bolt.Bucket, key []byte) error {
	encoded, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return b.Put(key, encoded)
}

func SaveMetrics(db *DB, userID proto.UserID, m Metrics) error {
	if userID == House {
		m.AmountSpentByHouse = m.AmountSpent
	}
	return db.Update(func(tx *Tx) error {
		b := tx.MetricsBucket()
		if userID != "" {
			var usrm Metrics
			if err := usrm.Load(b, []byte(userID)); err != nil {
				return err
			}
			if err := usrm.Incr(m).Save(b, []byte(userID)); err != nil {
				return err
			}
		}
		var sysm Metrics
		if err := sysm.Load(b, []byte("system")); err != nil {
			return err
		}
		return sysm.Incr(m).Save(b, []byte("system"))
	})
}

func LoadMetrics(db *DB, userID proto.UserID) (m Metrics, err error) {
	err = db.View(func(tx *Tx) error {
		return m.Load(tx.MetricsBucket(), []byte(userID))
	})
	return
}
