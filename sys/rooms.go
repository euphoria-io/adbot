package sys

import "strings"

func Rooms(db *DB) ([]string, error) {
	var rooms []string
	err := db.View(func(tx *Tx) error {
		return tx.RoomBucket().ForEach(func(k, v []byte) error {
			if roomName := string(k); !strings.HasPrefix(string(roomName), "&") {
				rooms = append(rooms, roomName)
			}
			return nil
		})
	})
	return rooms, err
}

func Join(db *DB, roomName string) (bool, error) {
	var ok bool
	err := db.Update(func(tx *Tx) error {
		b := tx.RoomBucket()
		ok = b.Get([]byte(roomName)) == nil
		b.Put([]byte(roomName), []byte("1"))
		return nil
	})
	return ok, err
}

func Part(db *DB, roomName string) (bool, error) {
	var ok bool
	err := db.Update(func(tx *Tx) error {
		b := tx.RoomBucket()
		ok = b.Get([]byte(roomName)) != nil
		b.Delete([]byte(roomName))
		return nil
	})
	return ok, err
}
