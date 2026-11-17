package store

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

// ConfigSpec describes the mutability, sensitivity, and validation contract of a configuration key.
type ConfigSpec struct {
	Editable  bool
	Sensitive bool
	IsURL     bool
}

// ConfigRegistry is the single source of truth for configuration metadata across store and HTTP handlers.
var ConfigRegistry = map[string]ConfigSpec{
	KeyCFAPIToken:        {Editable: true, Sensitive: true},
	KeyNezhaURL:          {Editable: true, IsURL: true},
	KeyNezhaDBPath:       {Editable: true},
	KeyNezhaWebhookToken: {Editable: true, Sensitive: true},
	KeyBiliSessdata:      {Editable: true, Sensitive: true},
	KeyDeepSeekAPIKey:    {Editable: true, Sensitive: true},
	KeyDeepSeekBaseURL:   {Editable: true, IsURL: true},
	KeyAIModel:           {Editable: true},
	KeyActivePersona:     {Editable: true},
	KeyLightAPIKey:       {Editable: true, Sensitive: true},
	KeyLightBaseURL:      {Editable: true, IsURL: true},
	KeyLightModel:        {Editable: true},
	KeyPoolForLight:      {Editable: true},
	KeyPoolFallback:      {Editable: true},
	KeyAppBearerToken:    {Editable: true, Sensitive: true},
	KeyAndroidCertSHA256: {Editable: true},
	KeyMorningCity:       {Editable: true},
	KeyRadarHNMinScore:   {Editable: true},
	KeyRadarLimit:        {Editable: true},
	KeyPushChannel:       {Editable: false},
	KeyFreeModelPool:     {Editable: false},
	KeyPasskeyUserID:     {Editable: false},
	KeyAdminPassHash:     {Editable: false, Sensitive: true},
	KeySessionSecret:     {Editable: false, Sensitive: true},
}

func LookupConfigSpec(key string) (ConfigSpec, bool) {
	spec, ok := ConfigRegistry[strings.TrimSpace(key)]
	return spec, ok
}

// MaskSensitiveConfigs redacts all registered sensitive values in-place using MaskSecretValue.
func MaskSensitiveConfigs(cfgs map[string]string) {
	for k, spec := range ConfigRegistry {
		if !spec.Sensitive {
			continue
		}
		if raw, ok := cfgs[k]; ok && strings.TrimSpace(raw) != "" {
			cfgs[k] = MaskSecretValue(raw)
		}
	}
}

// GenerateAppBearerToken creates a cryptographically random bh_ prefixed bearer token.
func GenerateAppBearerToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "bh_" + hex.EncodeToString(b), nil
}

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
