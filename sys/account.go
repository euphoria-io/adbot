package sys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"euphoria.io/heim-client/client"
	"euphoria.io/heim/proto"
)

const PasswordLength = 16

var (
	ErrAlreadyRegistered = fmt.Errorf("already registered")
)

type Account struct {
	Email    string
	Password string
	Verified bool
}

func Credentials(db *DB) (*Account, error) {
	var account *Account
	err := db.View(func(tx *Tx) error {
		b := tx.AccountBucket()
		emailBytes := b.Get([]byte("email"))
		if emailBytes == nil {
			return nil
		}
		account = &Account{
			Email:    string(emailBytes),
			Password: base64.StdEncoding.EncodeToString(b.Get([]byte("password"))),
			Verified: b.Get([]byte("verified")) != nil,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return account, nil
}

func Register(db *DB, conn *client.Client, email string) error {
	var password string
	err := db.Update(func(tx *Tx) error {
		b := tx.AccountBucket()
		if b.Get([]byte("email")) != nil {
			return fmt.Errorf("already registered")
		}

		b.Put([]byte("email"), []byte(email))

		passwordBytes := make([]byte, PasswordLength)
		if _, err := rand.Read(passwordBytes); err != nil {
			return err
		}
		b.Put([]byte("password"), passwordBytes)

		password = base64.StdEncoding.EncodeToString(passwordBytes)
		return nil
	})
	if err != nil {
		return err
	}

	resp, err := conn.Send(proto.RegisterAccountType, proto.RegisterAccountCommand{
		Namespace: "email",
		ID:        email,
		Password:  password,
	})
	if err != nil {
		dberr := db.Update(func(tx *Tx) error {
			tx.AccountBucket().Delete([]byte("email"))
			return nil
		})
		if dberr != nil {
			return fmt.Errorf("%s (rolling back after error: %s)", dberr, err)
		}
		return err
	}

	reply, ok := resp.(proto.RegisterAccountReply)
	if !ok || !reply.Success {
		return fmt.Errorf("account registration failed: %s", reply.Reason)
	}

	return nil
}

func Verify(db *DB, conn *client.Client, verifyURL string) error {
	resp, err := http.Get(verifyURL)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("verification failed on %s:\n", verifyURL)
		io.Copy(os.Stdout, resp.Body)
		resp.Body.Close()
		return fmt.Errorf(resp.Status)
	}
	return db.Update(func(tx *Tx) error {
		tx.AccountBucket().Put([]byte("verified"), []byte("1"))
		return nil
	})
}
