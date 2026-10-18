package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	bbolt "go.etcd.io/bbolt"
)

// APIToken represents an API access token for programmatic access.
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"token_hash"`
	Prefix     string     `json:"prefix"` // e.g. "pds_a1b2..."
	CreatedBy  string     `json:"created_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// GenerateAPIToken generates a secure raw token (pds_ + 32 random hex chars),
// its SHA256 hex digest, and its 8-character display prefix.
func GenerateAPIToken() (rawToken string, tokenHash string, prefix string, err error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random token bytes: %w", err)
	}
	rawToken = "pds_" + hex.EncodeToString(bytes)
	tokenHash = HashAPIToken(rawToken)
	prefix = rawToken[:8] + "..."
	return rawToken, tokenHash, prefix, nil
}

// HashAPIToken hashes a raw API token using SHA256.
func HashAPIToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}

// SaveToken stores or updates an APIToken in the tokens bucket.
func (s *BoltStore) SaveToken(token *APIToken) error {
	if token == nil || strings.TrimSpace(token.ID) == "" {
		return fmt.Errorf("%w: token and token ID cannot be empty", ErrInvalidInput)
	}
	if strings.TrimSpace(token.TokenHash) == "" {
		return fmt.Errorf("%w: token hash cannot be empty", ErrInvalidInput)
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		if b == nil {
			return errors.New("tokens bucket does not exist")
		}
		data, err := json.Marshal(token)
		if err != nil {
			return err
		}
		return b.Put([]byte(token.ID), data)
	})
}

// GetToken retrieves an APIToken by its ID.
func (s *BoltStore) GetToken(id string) (*APIToken, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: token id cannot be empty", ErrInvalidInput)
	}
	var token APIToken
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		if b == nil {
			return ErrTokenNotFound
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrTokenNotFound, id)
		}
		return json.Unmarshal(data, &token)
	})
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// ListTokens returns all stored API tokens sorted by creation date descending.
func (s *BoltStore) ListTokens() ([]*APIToken, error) {
	var tokens []*APIToken
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var tok APIToken
			if err := json.Unmarshal(v, &tok); err != nil {
				return err
			}
			tokens = append(tokens, &tok)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].CreatedAt.After(tokens[j].CreatedAt)
	})
	return tokens, nil
}

// DeleteToken revokes and removes an API token by ID.
func (s *BoltStore) DeleteToken(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: token id cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		if b == nil {
			return ErrTokenNotFound
		}
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("%w: %s", ErrTokenNotFound, id)
		}
		return b.Delete([]byte(id))
	})
}

// ValidateToken verifies a raw API token string against stored token hashes,
// checks expiration, and updates LastUsedAt timestamp upon successful validation.
func (s *BoltStore) ValidateToken(rawToken string) (*APIToken, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, fmt.Errorf("%w: raw token cannot be empty", ErrInvalidInput)
	}
	targetHash := HashAPIToken(rawToken)
	now := time.Now()

	var matched APIToken
	var matchedKey []byte
	var needsUpdate bool

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTokens)
		if b == nil {
			return ErrTokenNotFound
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var tok APIToken
			if err := json.Unmarshal(v, &tok); err != nil {
				continue
			}
			if tok.TokenHash == targetHash {
				if tok.ExpiresAt != nil && now.After(*tok.ExpiresAt) {
					return ErrTokenExpired
				}
				matched = tok
				matchedKey = append([]byte(nil), k...)
				if tok.LastUsedAt == nil || now.Sub(*tok.LastUsedAt) > time.Minute {
					needsUpdate = true
				}
				return nil
			}
		}
		return ErrTokenNotFound
	})
	if err != nil {
		return nil, err
	}

	if needsUpdate && len(matchedKey) > 0 {
		_ = s.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(bucketTokens)
			if b == nil {
				return nil
			}
			data := b.Get(matchedKey)
			if data == nil {
				return nil
			}
			var tok APIToken
			if err := json.Unmarshal(data, &tok); err != nil {
				return nil
			}
			tok.LastUsedAt = &now
			if updatedData, err := json.Marshal(&tok); err == nil {
				_ = b.Put(matchedKey, updatedData)
			}
			return nil
		})
		matched.LastUsedAt = &now
	}

	return &matched, nil
}
