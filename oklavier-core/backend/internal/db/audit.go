package db

import (
	"fmt"
	"log"
	"time"
)

type AuditEntry struct {
	ID           string    `db:"id" json:"id"`
	UserID       string    `db:"user_id" json:"user_id"`
	UserEmail    string    `db:"user_email" json:"user_email"`
	Action       string    `db:"action" json:"action"`
	ResourceType string    `db:"resource_type" json:"resource_type"`
	ResourceID   string    `db:"resource_id" json:"resource_id"`
	Details      string    `db:"details" json:"details"`
	IPAddress    string    `db:"ip_address" json:"ip_address"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

func (db *DB) LogAudit(userID, userEmail, action, resourceType, resourceID, details, ip string) {
	// Auto-resolve email from user_id if not provided
	if userEmail == "" && userID != "" {
		db.Get(&userEmail, `SELECT COALESCE(email,'') FROM "user" WHERE id = $1`, userID)
	}
	if _, err := db.Exec(
		`INSERT INTO audit_log (user_id, user_email, action, resource_type, resource_id, details, ip_address) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		userID, userEmail, action, resourceType, resourceID, details, ip,
	); err != nil {
		log.Printf("LogAudit error: %v", err)
	}
}

type PaginatedAudit struct {
	Entries []AuditEntry `json:"entries"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	PerPage int          `json:"per_page"`
}

type CursorAudit struct {
	Entries    []AuditEntry `json:"entries"`
	NextCursor string       `json:"next_cursor,omitempty"`
	PerPage    int          `json:"per_page"`
}

func (db *DB) GetAuditLogCursor(cursor string, perPage int, resourceType, action string) CursorAudit {
	if perPage < 1 {
		perPage = 50
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			log.Printf("GetAuditLogCursor: invalid cursor: %v", err)
			return CursorAudit{Entries: []AuditEntry{}, PerPage: perPage}
		}
		where += fmt.Sprintf(" AND created_at < $%d", argN)
		args = append(args, t)
		argN++
	}
	if resourceType != "" {
		where += fmt.Sprintf(" AND resource_type = $%d", argN)
		args = append(args, resourceType)
		argN++
	}
	if action != "" {
		where += fmt.Sprintf(" AND action = $%d", argN)
		args = append(args, action)
		argN++
	}

	var entries []AuditEntry
	query := fmt.Sprintf("SELECT * FROM audit_log %s ORDER BY created_at DESC LIMIT %d", where, perPage+1)
	db.Select(&entries, query, args...)
	if entries == nil {
		entries = []AuditEntry{}
	}

	var nextCursor string
	if len(entries) > perPage {
		nextCursor = entries[perPage-1].CreatedAt.Format(time.RFC3339Nano)
		entries = entries[:perPage]
	}

	return CursorAudit{Entries: entries, NextCursor: nextCursor, PerPage: perPage}
}

func (db *DB) GetAuditLog(page, perPage int, resourceType, action string) PaginatedAudit {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	where := "WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if resourceType != "" {
		where += fmt.Sprintf(" AND resource_type = $%d", argN)
		args = append(args, resourceType)
		argN++
	}
	if action != "" {
		where += fmt.Sprintf(" AND action = $%d", argN)
		args = append(args, action)
		argN++
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	db.Get(&total, fmt.Sprintf("SELECT COUNT(*) FROM audit_log %s", where), countArgs...)

	var entries []AuditEntry
	query := fmt.Sprintf("SELECT * FROM audit_log %s ORDER BY created_at DESC LIMIT %d OFFSET %d", where, perPage, offset)
	db.Select(&entries, query, args...)
	if entries == nil {
		entries = []AuditEntry{}
	}

	return PaginatedAudit{Entries: entries, Total: total, Page: page, PerPage: perPage}
}
