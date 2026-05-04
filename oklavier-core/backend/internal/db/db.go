package db

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"oklavier-api/internal/models"
)

type DB struct {
	*sqlx.DB
	EncryptionKey string
}

func New(host, port, user, password, dbname string) (*DB, error) {
	sslmode := "disable"
	if host != "localhost" && host != "127.0.0.1" {
		sslmode = "require" // Enforce TLS for remote DB connections
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)
	conn, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	return &DB{DB: conn}, nil
}

// Users
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := db.Get(&user, `SELECT user_id, username, pw_hash, salt, first_name, last_name,
		locked, realm, anonymous, created as created_date, failed_pw_attempts
		FROM users WHERE username = $1`, username)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	err := db.Get(&user, `SELECT user_id, username, pw_hash, salt, first_name, last_name,
		locked, realm, anonymous, created as created_date, failed_pw_attempts
		FROM users WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (db *DB) GetUsers() ([]models.User, error) {
	var users []models.User
	err := db.Select(&users, "SELECT * FROM users ORDER BY username")
	return users, err
}

func (db *DB) UpdateUserLock(userID string, locked bool, attempts int) error {
	_, err := db.Exec("UPDATE users SET locked = $1, failed_pw_attempts = $2 WHERE user_id = $3",
		locked, attempts, userID)
	return err
}

// Images
func (db *DB) GetImages() ([]models.Image, error) {
	var images []models.Image
	err := db.Select(&images, `SELECT image_id, friendly_name, name, description, image_src,
		cores, memory, enabled, image_type, hidden, x_res, y_res
		FROM images WHERE enabled = true AND hidden = false ORDER BY friendly_name`)
	return images, err
}

func (db *DB) GetImageByID(imageID string) (*models.Image, error) {
	var image models.Image
	err := db.Get(&image, `SELECT image_id, friendly_name, name, description, image_src,
		cores, memory, enabled, image_type, hidden, x_res, y_res
		FROM images WHERE image_id = $1`, imageID)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// SessionRecord for DB insert
type SessionRecord struct {
	SessionID            uuid.UUID `db:"session_id"`
	UserID            uuid.UUID `db:"user_id"`
	ImageID           uuid.UUID `db:"image_id"`
	ContainerID       string    `db:"container_id"`
	ContainerIP       string    `db:"container_ip"`
	OperationalStatus string    `db:"operational_status"`
	CreatedDate       time.Time `db:"created_date"`
	StartDate         time.Time `db:"start_date"`
	KeepaliveDate     time.Time `db:"keepalive_date"`
	ExpirationDate    time.Time `db:"expiration_date"`
	Token             string    `db:"token"` // VNC password
}

// Sessions
func (db *DB) GetSessionsByUser(userID string) ([]models.Session, error) {
	var sessions []models.Session
	err := db.Select(&sessions,
		`SELECT session_id, container_id, container_ip, hostname, operational_status,
		 server_id, user_id, image_id, start_date, created_date, keepalive_date, expiration_date,
		 docker_network FROM sessions
		 WHERE user_id = $1 AND operational_status NOT IN ('deleted','deleting')
		 ORDER BY created_date DESC`, userID)
	return sessions, err
}

func (db *DB) GetAllSessions() ([]models.Session, error) {
	var sessions []models.Session
	err := db.Select(&sessions,
		`SELECT session_id, container_id, container_ip, hostname, operational_status,
		 server_id, user_id, image_id, start_date, created_date, keepalive_date, expiration_date,
		 docker_network FROM sessions
		 WHERE operational_status NOT IN ('deleted','deleting')
		 ORDER BY created_date DESC`)
	return sessions, err
}

func (db *DB) CreateSessionRecord(s *SessionRecord) error {
	_, err := db.Exec(`INSERT INTO sessions (session_id, user_id, image_id, container_id, container_ip,
		operational_status, created_date, start_date, keepalive_date, expiration_date, token)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		s.SessionID, s.UserID, s.ImageID, s.ContainerID, s.ContainerIP,
		s.OperationalStatus, s.CreatedDate, s.StartDate, s.KeepaliveDate, s.ExpirationDate, s.Token)
	return err
}

func (db *DB) GetSessionVNCPassword(sessionID string) (string, error) {
	var token string
	err := db.Get(&token, "SELECT COALESCE(token, '') FROM sessions WHERE session_id = $1", sessionID)
	return token, err
}

// Old legacy methods removed - now in workspaces.go

// Servers
func (db *DB) GetServers() ([]models.Server, error) {
	var servers []models.Server
	err := db.Select(&servers, "SELECT * FROM servers ORDER BY hostname")
	return servers, err
}

// Zones
func (db *DB) GetZones() ([]models.Zone, error) {
	var zones []models.Zone
	err := db.Select(&zones, "SELECT * FROM zones ORDER BY zone_name")
	return zones, err
}

// Config settings
func (db *DB) GetConfigSetting(category, name string) (string, error) {
	var value string
	err := db.Get(&value,
		"SELECT setting_value FROM settings WHERE setting_category = $1 AND setting_name = $2",
		category, name)
	return value, err
}
