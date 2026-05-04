package db

import (
	"fmt"
	"log"
)

type Group struct {
	ID          string  `db:"id" json:"id"`
	Name        string  `db:"name" json:"name"`
	Description string  `db:"description" json:"description"`
	Color       string  `db:"color" json:"color"`
	IsDefault   bool    `db:"is_default" json:"is_default"`
	MaxSessions int     `db:"max_sessions" json:"max_sessions"`
	MaxCPU      float64 `db:"max_cpu" json:"max_cpu"`
	MaxMemory   int64   `db:"max_memory" json:"max_memory"`
}

func (db *DB) GetGroups() ([]Group, error) {
	var groups []Group
	err := db.Select(&groups, "SELECT id, name, COALESCE(description,'') as description, color, COALESCE(is_default, false) as is_default, COALESCE(max_sessions, 0) as max_sessions, COALESCE(max_cpu, 0) as max_cpu, COALESCE(max_memory, 0) as max_memory FROM oklavier_group ORDER BY is_default DESC, name")
	return groups, err
}

func (db *DB) GetGroupsPaginated(page, perPage int, search string) ([]Group, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	var total int
	if search != "" {
		db.Get(&total, `SELECT COUNT(*) FROM oklavier_group WHERE name ILIKE $1 OR COALESCE(description,'') ILIKE $1`, "%"+search+"%")
	} else {
		db.Get(&total, `SELECT COUNT(*) FROM oklavier_group`)
	}

	var groups []Group
	if search != "" {
		query := fmt.Sprintf(`SELECT id, name, COALESCE(description,'') as description, color, COALESCE(is_default, false) as is_default, COALESCE(max_sessions, 0) as max_sessions, COALESCE(max_cpu, 0) as max_cpu, COALESCE(max_memory, 0) as max_memory
			FROM oklavier_group WHERE name ILIKE $1 OR COALESCE(description,'') ILIKE $1
			ORDER BY is_default DESC, name LIMIT %d OFFSET %d`, perPage, offset)
		err := db.Select(&groups, query, "%"+search+"%")
		return groups, total, err
	}

	query := fmt.Sprintf(`SELECT id, name, COALESCE(description,'') as description, color, COALESCE(is_default, false) as is_default
		FROM oklavier_group ORDER BY is_default DESC, name LIMIT %d OFFSET %d`, perPage, offset)
	err := db.Select(&groups, query)
	return groups, total, err
}

func (db *DB) CreateGroup(name, description, color string, maxSessions int, maxCPU float64, maxMemory int64) error {
	_, err := db.Exec("INSERT INTO oklavier_group (name, description, color, max_sessions, max_cpu, max_memory) VALUES ($1, $2, $3, $4, $5, $6)", name, description, color, maxSessions, maxCPU, maxMemory)
	return err
}

func (db *DB) UpdateGroup(id, name, description, color string, maxSessions int, maxCPU float64, maxMemory int64) error {
	_, err := db.Exec("UPDATE oklavier_group SET name = $2, description = $3, color = $4, max_sessions = $5, max_cpu = $6, max_memory = $7 WHERE id = $1", id, name, description, color, maxSessions, maxCPU, maxMemory)
	return err
}

func (db *DB) DeleteGroup(id string) {
	// Cannot delete the default group
	if _, err := db.Exec("DELETE FROM oklavier_group WHERE id = $1 AND is_default = false", id); err != nil {
		log.Printf("DeleteGroup error: %v", err)
	}
}

// User groups
func (db *DB) GetUserGroups(userID string) ([]Group, error) {
	var groups []Group
	err := db.Select(&groups, `SELECT g.id, g.name, COALESCE(g.description,'') as description, g.color
		FROM oklavier_group g JOIN user_group ug ON g.id = ug.group_id WHERE ug.user_id = $1`, userID)
	return groups, err
}

func (db *DB) SetUserGroups(userID string, groupIDs []string) error {
	if _, err := db.Exec("DELETE FROM user_group WHERE user_id = $1", userID); err != nil {
		log.Printf("SetUserGroups delete error: %v", err)
	}
	for _, gid := range groupIDs {
		if _, err := db.Exec("INSERT INTO user_group (user_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, gid); err != nil {
			log.Printf("SetUserGroups insert error: %v", err)
		}
	}
	return nil
}

// Workspace groups
func (db *DB) GetWorkspaceGroups(workspaceID string) ([]Group, error) {
	var groups []Group
	err := db.Select(&groups, `SELECT g.id, g.name, COALESCE(g.description,'') as description, g.color
		FROM oklavier_group g JOIN workspace_group wg ON g.id = wg.group_id WHERE wg.workspace_id = $1`, workspaceID)
	return groups, err
}

func (db *DB) SetWorkspaceGroups(workspaceID string, groupIDs []string) error {
	if _, err := db.Exec("DELETE FROM workspace_group WHERE workspace_id = $1", workspaceID); err != nil {
		log.Printf("SetWorkspaceGroups delete error: %v", err)
	}
	if len(groupIDs) == 0 {
		// No groups selected → assign to default group automatically
		if _, err := db.Exec(`INSERT INTO workspace_group (workspace_id, group_id)
			SELECT $1, id FROM oklavier_group WHERE is_default = true
			ON CONFLICT DO NOTHING`, workspaceID); err != nil {
			log.Printf("SetWorkspaceGroups default group insert error: %v", err)
		}
	} else {
		for _, gid := range groupIDs {
			if _, err := db.Exec("INSERT INTO workspace_group (workspace_id, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", workspaceID, gid); err != nil {
				log.Printf("SetWorkspaceGroups insert error: %v", err)
			}
		}
	}
	return nil
}

// Check if user can access workspace (admin bypasses)
func (db *DB) UserCanAccessWorkspace(userID, workspaceID string) bool {
	// Check if workspace has any group restrictions
	var count int
	db.Get(&count, "SELECT COUNT(*) FROM workspace_group WHERE workspace_id = $1", workspaceID)
	if count == 0 {
		return true // No restrictions
	}

	// Check if workspace is in the default group (everyone has access)
	var allGroup int
	db.Get(&allGroup, `SELECT COUNT(*) FROM workspace_group wg
		JOIN oklavier_group g ON g.id = wg.group_id
		WHERE wg.workspace_id = $1 AND g.is_default = true`, workspaceID)
	if allGroup > 0 {
		return true
	}

	// Check if user is in any of the workspace's groups
	var access int
	db.Get(&access, `SELECT COUNT(*) FROM user_group ug
		JOIN workspace_group wg ON ug.group_id = wg.group_id
		WHERE ug.user_id = $1 AND wg.workspace_id = $2`, userID, workspaceID)
	return access > 0
}

// OIDC role mapping
type OIDCGroupMapping struct {
	ID       string `db:"id" json:"id"`
	OIDCRole string `db:"oidc_role" json:"oidc_role"`
	GroupID  string `db:"group_id" json:"group_id"`
}

func (db *DB) GetOIDCMappings() ([]OIDCGroupMapping, error) {
	var mappings []OIDCGroupMapping
	err := db.Select(&mappings, "SELECT id, oidc_role, group_id FROM oidc_role_mapping ORDER BY oidc_role")
	return mappings, err
}

func (db *DB) CreateOIDCMapping(oidcRole, groupID string) error {
	_, err := db.Exec("INSERT INTO oidc_role_mapping (oidc_role, group_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", oidcRole, groupID)
	return err
}

func (db *DB) DeleteOIDCMapping(id string) {
	if _, err := db.Exec("DELETE FROM oidc_role_mapping WHERE id = $1", id); err != nil {
		log.Printf("DeleteOIDCMapping error: %v", err)
	}
}

// Sync user groups from OIDC roles + track all roles
func (db *DB) SyncUserGroupsFromOIDC(userID string, oidcRoles []string, providerName string) {
	// 1. Store all roles we see (auto-discover)
	for _, role := range oidcRoles {
		if _, err := db.Exec(`INSERT INTO oidc_role (role) VALUES ($1)
			ON CONFLICT (role) DO UPDATE SET last_seen = NOW(), user_count = (
				SELECT COUNT(DISTINCT user_id) FROM user_oidc_role WHERE role = $1
			)`, role); err != nil {
			log.Printf("SyncUserGroupsFromOIDC upsert role error: %v", err)
		}
	}

	// 2. Store user's current roles (replace old ones)
	if _, err := db.Exec("DELETE FROM user_oidc_role WHERE user_id = $1", userID); err != nil {
		log.Printf("SyncUserGroupsFromOIDC delete user roles error: %v", err)
	}
	for _, role := range oidcRoles {
		if _, err := db.Exec("INSERT INTO user_oidc_role (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING", userID, role); err != nil {
			log.Printf("SyncUserGroupsFromOIDC insert user role error: %v", err)
		}
	}

	// 3. Update user as SSO
	if _, err := db.Exec(`UPDATE "user" SET auth_provider = 'oidc', oidc_provider_name = $2, last_login = NOW() WHERE id = $1`, userID, providerName); err != nil {
		log.Printf("SyncUserGroupsFromOIDC update user auth error: %v", err)
	}

	// 4. Map roles to groups
	mappings, err := db.GetOIDCMappings()
	if err != nil {
		return
	}

	groupIDs := make(map[string]bool)
	for _, role := range oidcRoles {
		for _, m := range mappings {
			if m.OIDCRole == role {
				groupIDs[m.GroupID] = true
			}
		}
	}

	ids := make([]string, 0, len(groupIDs))
	for id := range groupIDs {
		ids = append(ids, id)
	}
	if len(ids) > 0 {
		db.SetUserGroups(userID, ids)
	}

	// 5. Update role counts
	for _, role := range oidcRoles {
		if _, err := db.Exec(`UPDATE oidc_role SET user_count = (SELECT COUNT(DISTINCT user_id) FROM user_oidc_role WHERE role = $1) WHERE role = $1`, role); err != nil {
			log.Printf("SyncUserGroupsFromOIDC update role count error: %v", err)
		}
	}
}

// Get all discovered OIDC roles
type OIDCRole struct {
	Role      string `db:"role" json:"role"`
	UserCount int    `db:"user_count" json:"user_count"`
	LastSeen  string `db:"last_seen" json:"last_seen"`
}

func (db *DB) GetAllOIDCRoles() ([]OIDCRole, error) {
	var roles []OIDCRole
	err := db.Select(&roles, "SELECT role, user_count, last_seen FROM oidc_role ORDER BY role")
	return roles, err
}

// Get user's OIDC roles
func (db *DB) GetUserOIDCRoles(userID string) ([]string, error) {
	var roles []string
	err := db.Select(&roles, "SELECT role FROM user_oidc_role WHERE user_id = $1 ORDER BY role", userID)
	return roles, err
}
