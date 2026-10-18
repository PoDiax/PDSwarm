package store

import (
	"encoding/json"
	"fmt"
	"time"

	bbolt "go.etcd.io/bbolt"
)

type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *BoltStore) CountUsers() (int, error) {
	var count int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		count = b.Stats().KeyN
		return nil
	})
	return count, err
}

func (s *BoltStore) GetUser(username string) (*User, error) {
	if username == "" {
		return nil, fmt.Errorf("%w: username cannot be empty", ErrInvalidInput)
	}
	var user User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data := b.Get([]byte(username))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrUserNotFound, username)
		}
		return json.Unmarshal(data, &user)
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *BoltStore) SaveUser(user *User) error {
	if user == nil || user.Username == "" {
		return fmt.Errorf("%w: user or username cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.Username), data)
	})
}

// ListUsers returns all users from the database.
func (s *BoltStore) ListUsers() ([]*User, error) {
	var users []*User
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var u User
			if err := json.Unmarshal(v, &u); err != nil {
				return err
			}
			users = append(users, &u)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

// DeleteUser removes a user record from the database.
func (s *BoltStore) DeleteUser(username string) error {
	if username == "" {
		return fmt.Errorf("%w: username cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketUsers)
		if b == nil {
			return fmt.Errorf("%w: %s", ErrUserNotFound, username)
		}
		if b.Get([]byte(username)) == nil {
			return fmt.Errorf("%w: %s", ErrUserNotFound, username)
		}
		return b.Delete([]byte(username))
	})
}

