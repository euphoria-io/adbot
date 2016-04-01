package sys

import (
	"fmt"
	"strings"

	"euphoria.io/heim/proto"
)

func SetUserEnabled(db *DB, userID proto.UserID, enabled bool) error {
	return db.Update(func(tx *Tx) error {
		b, err := tx.OverrideBucket().CreateBucketIfNotExists([]byte("user"))
		if err != nil {
			return err
		}
		v := "0"
		if enabled {
			v = "1"
		}
		b.Put([]byte(userID), []byte(v))
		return nil
	})
}

func userOverrides(tx *Tx) (map[proto.UserID]bool, error) {
	b := tx.OverrideBucket().Bucket([]byte("user"))
	if b == nil {
		return nil, nil
	}
	overrides := map[proto.UserID]bool{}
	err := b.ForEach(func(k, v []byte) error {
		overrides[proto.UserID(k)] = string(v) == "1"
		return nil
	})
	return overrides, err
}

func SetSpendEnabled(db *DB, userID proto.UserID, creativeName string, enabled bool) error {
	return db.Update(func(tx *Tx) error {
		b, err := tx.OverrideBucket().CreateBucketIfNotExists([]byte("spend"))
		if err != nil {
			return err
		}
		k := fmt.Sprintf("%s:%s", userID, creativeName)
		v := "0"
		if enabled {
			v = "1"
		}
		b.Put([]byte(k), []byte(v))
		return nil
	})
}

func spendOverrides(tx *Tx) (map[proto.UserID]map[string]bool, error) {
	b := tx.OverrideBucket().Bucket([]byte("spend"))
	if b == nil {
		return nil, nil
	}
	overrides := map[proto.UserID]map[string]bool{}
	err := b.ForEach(func(k, v []byte) error {
		parts := strings.Split(string(k), ":")
		userID := proto.UserID(parts[0])
		m, ok := overrides[userID]
		if !ok {
			m = map[string]bool{}
			overrides[userID] = m
		}
		m[parts[1]] = string(v) == "1"
		return nil
	})
	return overrides, err
}
