package db

import (
	"log"

	"oklavier-api/internal/crypto"
)

// sensitiveSettingKeys lists setting keys whose values are encrypted at rest.
var sensitiveSettingKeys = map[string]bool{
	"s3.secret_key": true,
}

func (db *DB) GetSetting(key string) string {
	var value string
	err := db.Get(&value, `SELECT value FROM settings WHERE key = $1`, key)
	if err != nil {
		return ""
	}
	if sensitiveSettingKeys[key] && db.EncryptionKey != "" {
		if dec, err := crypto.Decrypt(value, db.EncryptionKey); err == nil {
			return dec
		}
	}
	return value
}

func (db *DB) SetSetting(key, value string) {
	storeValue := value
	if sensitiveSettingKeys[key] && db.EncryptionKey != "" && value != "" && !crypto.IsEncrypted(value) {
		if enc, err := crypto.Encrypt(value, db.EncryptionKey); err == nil {
			storeValue = enc
		}
	}
	_, err := db.Exec(`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`, key, storeValue)
	if err != nil {
		log.Printf("SetSetting error: %v", err)
	}
}

func (db *DB) GetSettings(prefix string) map[string]string {
	type kv struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	var rows []kv
	db.Select(&rows, `SELECT key, value FROM settings WHERE key LIKE $1`, prefix+"%")
	result := make(map[string]string)
	for _, r := range rows {
		v := r.Value
		if sensitiveSettingKeys[r.Key] && db.EncryptionKey != "" {
			if dec, err := crypto.Decrypt(v, db.EncryptionKey); err == nil {
				v = dec
			}
		}
		result[r.Key] = v
	}
	return result
}
