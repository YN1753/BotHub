package store

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

func (s *Store) GetConfig(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", nil
	}
	var val string
	err := s.db.QueryRow(`SELECT value FROM configs WHERE key = ?`, trimmed).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s *Store) GetAllConfigs() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM configs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, rows.Err()
}

func (s *Store) SetConfig(key, value string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return errors.New("config key cannot be empty")
	}
	_, err := s.db.Exec(`INSERT INTO configs (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		trimmed, value, NowShanghai())
	return err
}

func HashPassword(raw string) string {
	sum := sha256.Sum256([]byte("vesper-bothub::" + raw))
	return hex.EncodeToString(sum[:])
}

func VerifyPassword(raw, hash string) bool {
	computed := HashPassword(raw)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(strings.TrimSpace(hash))) == 1
}
