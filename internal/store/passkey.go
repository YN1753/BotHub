package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const (
	KeyPasskeyUserID     = "passkey_user_id"
	KeyAndroidCertSHA256 = "android_cert_sha256"
)

type PasskeyRecord struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	AAGUID         string   `json:"aaguid"`
	CredentialJSON string   `json:"-"`
	DeviceType     string   `json:"credentialDeviceType"`
	BackedUp       bool     `json:"credentialBackedUp"`
	Transports     []string `json:"transports"`
	RegisteredAt   int64    `json:"registeredAt"`
	LastUsedAt     int64    `json:"lastUsedAt,omitempty"`
}

func (s *Store) migratePasskeysTable() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS passkey_credentials (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	aaguid TEXT NOT NULL DEFAULT '',
	credential_json TEXT NOT NULL,
	device_type TEXT NOT NULL DEFAULT 'multiDevice',
	backed_up INTEGER NOT NULL DEFAULT 1,
	transports TEXT NOT NULL DEFAULT '',
	registered_at INTEGER NOT NULL,
	last_used_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_passkey_registered ON passkey_credentials(registered_at DESC);
`
	_, err := s.db.Exec(ddl)
	return err
}

func (s *Store) EnsurePasskeyUserID() ([]byte, string, error) {
	stored, _ := s.GetConfig(KeyPasskeyUserID)
	stored = strings.TrimSpace(stored)
	if stored != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(stored); err == nil && len(raw) >= 16 {
			return raw, stored, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(buf)
	if err := s.SetConfig(KeyPasskeyUserID, encoded); err != nil {
		return nil, "", err
	}
	return buf, encoded, nil
}

func (s *Store) CountPasskeys() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM passkey_credentials`).Scan(&n)
	return n, err
}

func (s *Store) ListPasskeys() ([]PasskeyRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, name, aaguid, credential_json, device_type, backed_up, transports, registered_at, last_used_at
		FROM passkey_credentials
		ORDER BY registered_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PasskeyRecord, 0)
	for rows.Next() {
		var r PasskeyRecord
		var backedUp int
		var transportsCSV string
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.AAGUID,
			&r.CredentialJSON,
			&r.DeviceType,
			&backedUp,
			&transportsCSV,
			&r.RegisteredAt,
			&r.LastUsedAt,
		); err != nil {
			return nil, err
		}
		r.BackedUp = backedUp != 0
		r.Transports = splitCSV(transportsCSV)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetPasskey(id string) (*PasskeyRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, sql.ErrNoRows
	}
	var r PasskeyRecord
	var backedUp int
	var transportsCSV string
	err := s.db.QueryRow(`
		SELECT id, name, aaguid, credential_json, device_type, backed_up, transports, registered_at, last_used_at
		FROM passkey_credentials WHERE id = ?
	`, id).Scan(
		&r.ID,
		&r.Name,
		&r.AAGUID,
		&r.CredentialJSON,
		&r.DeviceType,
		&backedUp,
		&transportsCSV,
		&r.RegisteredAt,
		&r.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	r.BackedUp = backedUp != 0
	r.Transports = splitCSV(transportsCSV)
	return &r, nil
}

func (s *Store) SavePasskey(r PasskeyRecord) error {
	r.ID = strings.TrimSpace(r.ID)
	if r.ID == "" || strings.TrimSpace(r.CredentialJSON) == "" {
		return errors.New("invalid passkey record")
	}
	if r.DeviceType == "" {
		r.DeviceType = "multiDevice"
	}
	if r.RegisteredAt <= 0 {
		r.RegisteredAt = time.Now().Unix()
	}
	backedUp := 0
	if r.BackedUp {
		backedUp = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO passkey_credentials (id, name, aaguid, credential_json, device_type, backed_up, transports, registered_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			aaguid = excluded.aaguid,
			credential_json = excluded.credential_json,
			device_type = excluded.device_type,
			backed_up = excluded.backed_up,
			transports = excluded.transports,
			last_used_at = excluded.last_used_at
	`,
		r.ID,
		strings.TrimSpace(r.Name),
		strings.TrimSpace(r.AAGUID),
		r.CredentialJSON,
		r.DeviceType,
		backedUp,
		strings.Join(r.Transports, ","),
		r.RegisteredAt,
		r.LastUsedAt,
	)
	return err
}

func (s *Store) RenamePasskey(id, name string) error {
	res, err := s.db.Exec(`UPDATE passkey_credentials SET name = ? WHERE id = ?`, strings.TrimSpace(name), strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) TouchPasskeyUsage(id, credentialJSON string, usedAt int64) error {
	if usedAt <= 0 {
		usedAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`
		UPDATE passkey_credentials
		SET credential_json = ?, last_used_at = ?
		WHERE id = ?
	`, credentialJSON, usedAt, strings.TrimSpace(id))
	return err
}

func (s *Store) DeletePasskey(id string) error {
	res, err := s.db.Exec(`DELETE FROM passkey_credentials WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if clean := strings.TrimSpace(p); clean != "" {
			out = append(out, clean)
		}
	}
	return out
}
