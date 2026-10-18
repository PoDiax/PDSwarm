package store

import (
	"encoding/json"
	"fmt"
	"time"

	bbolt "go.etcd.io/bbolt"
)

type Stack struct {
	Name        string    `json:"name"`
	ComposeYAML string    `json:"compose_yaml"`
	Status      string    `json:"status"`
	IsExternal  bool      `json:"is_external,omitempty"`
	WorkingDir  string    `json:"working_dir,omitempty"`
	ConfigFile  string    `json:"config_file,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *BoltStore) GetStack(name string) (*Stack, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: stack name cannot be empty", ErrInvalidInput)
	}
	var stack Stack
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStacks)
		data := b.Get([]byte(name))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrStackNotFound, name)
		}
		return json.Unmarshal(data, &stack)
	})
	if err != nil {
		return nil, err
	}
	return &stack, nil
}

func (s *BoltStore) ListStacks() ([]*Stack, error) {
	stacks := make([]*Stack, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStacks)
		return b.ForEach(func(k, v []byte) error {
			var st Stack
			if err := json.Unmarshal(v, &st); err != nil {
				return err
			}
			stacks = append(stacks, &st)
			return nil
		})
	})
	return stacks, err
}

func (s *BoltStore) SaveStack(stack *Stack) error {
	if stack == nil || stack.Name == "" {
		return fmt.Errorf("%w: stack or stack name cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStacks)
		data, err := json.Marshal(stack)
		if err != nil {
			return err
		}
		return b.Put([]byte(stack.Name), data)
	})
}

func (s *BoltStore) DeleteStack(name string) error {
	if name == "" {
		return fmt.Errorf("%w: stack name cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketStacks)
		return b.Delete([]byte(name))
	})
}
