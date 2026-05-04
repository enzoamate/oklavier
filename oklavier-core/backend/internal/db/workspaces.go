package db

import (
	"fmt"
	"log"
	"time"

	"oklavier-api/internal/crypto"
	"oklavier-api/internal/models"
)

// MigrateEncryptCredentials encrypts any existing plaintext credentials in the
// workspace table and sensitive settings. Should be called once at startup.
func (db *DB) MigrateEncryptCredentials() {
	if db.EncryptionKey == "" {
		return
	}

	// Encrypt workspace passwords
	type credRow struct {
		ID             string `db:"id"`
		ServerPassword string `db:"server_password"`
		DockerPassword string `db:"docker_password"`
	}
	var rows []credRow
	db.Select(&rows, `SELECT id, COALESCE(server_password,'') as server_password, COALESCE(docker_password,'') as docker_password FROM workspace`)
	migrated := 0
	for _, r := range rows {
		needsUpdate := false
		sp := r.ServerPassword
		dp := r.DockerPassword
		if sp != "" && !crypto.IsEncrypted(sp) {
			if enc, err := crypto.Encrypt(sp, db.EncryptionKey); err == nil {
				sp = enc
				needsUpdate = true
			}
		}
		if dp != "" && !crypto.IsEncrypted(dp) {
			if enc, err := crypto.Encrypt(dp, db.EncryptionKey); err == nil {
				dp = enc
				needsUpdate = true
			}
		}
		if needsUpdate {
			db.Exec(`UPDATE workspace SET server_password=$1, docker_password=$2 WHERE id=$3`, sp, dp, r.ID)
			migrated++
		}
	}
	if migrated > 0 {
		log.Printf("Encryption migration: encrypted credentials for %d workspace(s)", migrated)
	}

	// Encrypt sensitive settings
	for key := range sensitiveSettingKeys {
		var val string
		if err := db.Get(&val, `SELECT value FROM settings WHERE key = $1`, key); err != nil || val == "" {
			continue
		}
		if !crypto.IsEncrypted(val) {
			if enc, err := crypto.Encrypt(val, db.EncryptionKey); err == nil {
				db.Exec(`UPDATE settings SET value=$1, updated_at=NOW() WHERE key=$2`, enc, key)
				log.Printf("Encryption migration: encrypted setting %s", key)
			}
		}
	}
}

// decryptWorkspace decrypts sensitive fields on a single workspace in-place.
func (db *DB) decryptWorkspace(w *models.Workspace) {
	if db.EncryptionKey == "" {
		return
	}
	if dec, err := crypto.Decrypt(w.ServerPassword, db.EncryptionKey); err == nil {
		w.ServerPassword = dec
	}
	if dec, err := crypto.Decrypt(w.DockerPassword, db.EncryptionKey); err == nil {
		w.DockerPassword = dec
	}
}

// decryptWorkspaces decrypts sensitive fields on a slice of workspaces.
func (db *DB) decryptWorkspaces(workspaces []models.Workspace) {
	for i := range workspaces {
		db.decryptWorkspace(&workspaces[i])
	}
}

// encryptWorkspaceFields encrypts sensitive fields before DB write.
func (db *DB) encryptWorkspaceFields(w *models.Workspace) {
	if db.EncryptionKey == "" {
		return
	}
	if w.ServerPassword != "" && !crypto.IsEncrypted(w.ServerPassword) {
		if enc, err := crypto.Encrypt(w.ServerPassword, db.EncryptionKey); err == nil {
			w.ServerPassword = enc
		}
	}
	if w.DockerPassword != "" && !crypto.IsEncrypted(w.DockerPassword) {
		if enc, err := crypto.Encrypt(w.DockerPassword, db.EncryptionKey); err == nil {
			w.DockerPassword = enc
		}
	}
}

func (db *DB) GetWorkspaces() ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := db.Select(&workspaces, `SELECT id, name, friendly_name, description, image_src, docker_image,
		cores, memory, shm_size, enabled, category, x_res, y_res,
		COALESCE(docker_registry,'') as docker_registry, COALESCE(docker_user,'') as docker_user, COALESCE(docker_password,'') as docker_password,
		COALESCE(session_time_limit,0) as session_time_limit, COALESCE(gpu_count,0) as gpu_count, COALESCE(uncompressed_size_mb,0) as uncompressed_size_mb,
		COALESCE(restrict_to_agent,'') as restrict_to_agent, COALESCE(restrict_to_region,'') as restrict_to_region,
		COALESCE(run_config,'{}') as run_config, COALESCE(exec_config,'{}') as exec_config, COALESCE(volume_mappings,'{}') as volume_mappings,
		COALESCE(categories,'{}') as categories, notes,
		COALESCE(persistent,false) as persistent, COALESCE(persistent_size,'') as persistent_size,
		COALESCE(workspace_type,'container') as workspace_type, COALESCE(server_hostname,'') as server_hostname, COALESCE(server_port,0) as server_port,
		COALESCE(server_protocol,'') as server_protocol, COALESCE(server_username,'') as server_username, COALESCE(server_password,'') as server_password,
		COALESCE(server_domain,'') as server_domain, COALESCE(server_ignore_cert,true) as server_ignore_cert, COALESCE(server_security,'') as server_security, COALESCE(server_auth_mode,'static') as server_auth_mode, COALESCE(server_allow_remember,false) as server_allow_remember, COALESCE(server_default_settings,'{}') as server_default_settings,
		COALESCE(record_sessions,false) as record_sessions
		FROM workspace WHERE enabled = true ORDER BY friendly_name`)
	if err == nil {
		db.decryptWorkspaces(workspaces)
	}
	return workspaces, err
}

func (db *DB) GetWorkspace(id string) (*models.Workspace, error) {
	var w models.Workspace
	err := db.Get(&w, `SELECT id, name, friendly_name, description, image_src, docker_image,
		cores, memory, shm_size, enabled, category, x_res, y_res,
		COALESCE(docker_registry,'') as docker_registry, COALESCE(docker_user,'') as docker_user, COALESCE(docker_password,'') as docker_password,
		COALESCE(session_time_limit,0) as session_time_limit, COALESCE(gpu_count,0) as gpu_count, COALESCE(uncompressed_size_mb,0) as uncompressed_size_mb,
		COALESCE(restrict_to_agent,'') as restrict_to_agent, COALESCE(restrict_to_region,'') as restrict_to_region,
		COALESCE(run_config,'{}') as run_config, COALESCE(exec_config,'{}') as exec_config, COALESCE(volume_mappings,'{}') as volume_mappings,
		COALESCE(categories,'{}') as categories, notes,
		COALESCE(persistent,false) as persistent, COALESCE(persistent_size,'') as persistent_size,
		COALESCE(workspace_type,'container') as workspace_type, COALESCE(server_hostname,'') as server_hostname, COALESCE(server_port,0) as server_port,
		COALESCE(server_protocol,'') as server_protocol, COALESCE(server_username,'') as server_username, COALESCE(server_password,'') as server_password,
		COALESCE(server_domain,'') as server_domain, COALESCE(server_ignore_cert,true) as server_ignore_cert, COALESCE(server_security,'') as server_security, COALESCE(server_auth_mode,'static') as server_auth_mode, COALESCE(server_allow_remember,false) as server_allow_remember, COALESCE(server_default_settings,'{}') as server_default_settings,
		COALESCE(record_sessions,false) as record_sessions
		FROM workspace WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	db.decryptWorkspace(&w)
	return &w, nil
}

func (db *DB) CreateWorkspaceSession(s *models.WorkspaceSession) error {
	sessionType := s.SessionType
	if sessionType == "" {
		sessionType = "container"
	}
	_, err := db.Exec(`INSERT INTO workspace_session
		(id, user_id, workspace_id, pod_name, service_name, container_ip, vnc_password, status, agent_id, expires_at, session_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		s.ID, s.UserID, s.WorkspaceID, s.PodName, s.ServiceName, s.ContainerIP, s.VNCPassword, s.Status, s.AgentID, s.ExpiresAt, sessionType)
	return err
}

func (db *DB) GetUserSessions(userID string) ([]models.WorkspaceSessionWithImage, error) {
	var sessions []models.WorkspaceSessionWithImage
	err := db.Select(&sessions, `SELECT ws.id, ws.user_id, ws.workspace_id, ws.pod_name, ws.service_name,
		ws.container_ip, ws.vnc_password, ws.status, COALESCE(ws.agent_id, '') as agent_id, COALESCE(ws.session_type, 'container') as session_type, ws.created_at, ws.expires_at, ws.keepalive_at,
		w.friendly_name as image_name, w.image_src as image_src, w.docker_image as docker_image, COALESCE(w.workspace_type, 'container') as workspace_type
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		WHERE ws.user_id = $1 AND ws.status != 'deleted'
		ORDER BY ws.created_at DESC`, userID)
	return sessions, err
}

func (db *DB) GetAllActiveSessions() ([]models.WorkspaceSessionWithImage, error) {
	var sessions []models.WorkspaceSessionWithImage
	err := db.Select(&sessions, `SELECT ws.id, ws.user_id, ws.workspace_id, ws.pod_name, ws.service_name,
		ws.container_ip, ws.vnc_password, ws.status, COALESCE(ws.agent_id, '') as agent_id, COALESCE(ws.session_type, 'container') as session_type, ws.created_at, ws.expires_at, ws.keepalive_at,
		w.friendly_name as image_name, w.image_src as image_src, w.docker_image as docker_image, COALESCE(w.workspace_type, 'container') as workspace_type,
		COALESCE(u.name, '') as user_name, COALESCE(u.email, '') as user_email
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		LEFT JOIN "user" u ON u.id = ws.user_id
		WHERE ws.status IN ('running', 'starting')
		ORDER BY ws.created_at DESC`)
	return sessions, err
}

func (db *DB) GetSession(id string) (*models.WorkspaceSessionWithImage, error) {
	var s models.WorkspaceSessionWithImage
	err := db.Get(&s, `SELECT ws.id, ws.user_id, ws.workspace_id, ws.pod_name, ws.service_name,
		ws.container_ip, ws.vnc_password, ws.status, COALESCE(ws.agent_id, '') as agent_id, COALESCE(ws.session_type, 'container') as session_type, ws.created_at, ws.expires_at, ws.keepalive_at,
		w.friendly_name as image_name, w.image_src as image_src, w.docker_image as docker_image, COALESCE(w.workspace_type, 'container') as workspace_type
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		WHERE ws.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) UpdateSessionStatus(id, status string) error {
	_, err := db.Exec(`UPDATE workspace_session SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (db *DB) UpdateSessionIP(id, ip string) error {
	_, err := db.Exec(`UPDATE workspace_session SET container_ip = $1, updated_at = NOW() WHERE id = $2`, ip, id)
	return err
}

func (db *DB) DeleteSession(id string) error {
	_, err := db.Exec(`UPDATE workspace_session SET status = 'deleted', updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (db *DB) GetSessionVNCPasswordByID(id string) (string, error) {
	var pw string
	err := db.Get(&pw, `SELECT COALESCE(vnc_password, '') FROM workspace_session WHERE id = $1`, id)
	return pw, err
}

// Admin workspace CRUD

func (db *DB) GetAllWorkspaces() ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := db.Select(&workspaces, `SELECT id, name, friendly_name, description, image_src, docker_image,
		cores, memory, shm_size, enabled, category, x_res, y_res,
		COALESCE(docker_registry,'') as docker_registry, COALESCE(docker_user,'') as docker_user, COALESCE(docker_password,'') as docker_password,
		COALESCE(session_time_limit,0) as session_time_limit, COALESCE(gpu_count,0) as gpu_count, COALESCE(uncompressed_size_mb,0) as uncompressed_size_mb,
		COALESCE(restrict_to_agent,'') as restrict_to_agent, COALESCE(restrict_to_region,'') as restrict_to_region,
		COALESCE(run_config,'{}') as run_config, COALESCE(exec_config,'{}') as exec_config, COALESCE(volume_mappings,'{}') as volume_mappings,
		COALESCE(categories,'{}') as categories, notes,
		COALESCE(persistent,false) as persistent, COALESCE(persistent_size,'') as persistent_size,
		COALESCE(workspace_type,'container') as workspace_type, COALESCE(server_hostname,'') as server_hostname, COALESCE(server_port,0) as server_port,
		COALESCE(server_protocol,'') as server_protocol, COALESCE(server_username,'') as server_username, COALESCE(server_password,'') as server_password,
		COALESCE(server_domain,'') as server_domain, COALESCE(server_ignore_cert,true) as server_ignore_cert, COALESCE(server_security,'') as server_security, COALESCE(server_auth_mode,'static') as server_auth_mode, COALESCE(server_allow_remember,false) as server_allow_remember, COALESCE(server_default_settings,'{}') as server_default_settings,
		COALESCE(record_sessions,false) as record_sessions
		FROM workspace ORDER BY friendly_name`)
	if err == nil {
		db.decryptWorkspaces(workspaces)
	}
	return workspaces, err
}

func (db *DB) CreateWorkspace(w *models.Workspace) error {
	db.encryptWorkspaceFields(w)
	wType := w.WorkspaceType
	if wType == "" {
		wType = "container"
	}
	_, err := db.Exec(`INSERT INTO workspace (name, friendly_name, description, image_src, docker_image,
		cores, memory, category, docker_registry, docker_user, docker_password,
		session_time_limit, gpu_count, uncompressed_size_mb, shm_size,
		restrict_to_agent, restrict_to_region,
		run_config, exec_config, volume_mappings,
		categories, notes, persistent, persistent_size,
		workspace_type, server_hostname, server_port, server_protocol,
		server_username, server_password, server_domain, server_ignore_cert, server_security, server_auth_mode, server_allow_remember, server_default_settings,
		record_sessions)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36,$37)`,
		w.Name, w.FriendlyName, w.Description, w.ImageSrc, w.DockerImage,
		w.Cores, w.Memory, w.Category, w.DockerRegistry, w.DockerUser, w.DockerPassword,
		w.SessionTimeLimit, w.GPUCount, w.UncompressedSizeMB, w.SHMSize,
		w.RestrictToAgent, w.RestrictToRegion,
		w.RunConfig, w.ExecConfig, w.VolumeMappings,
		w.Categories, w.Notes, w.Persistent, w.PersistentSize,
		wType, w.ServerHostname, w.ServerPort, w.ServerProtocol,
		w.ServerUsername, w.ServerPassword, w.ServerDomain, w.ServerIgnoreCert, w.ServerSecurity, w.ServerAuthMode, w.ServerAllowRemember, w.ServerDefaultSettings,
		w.RecordSessions)
	return err
}

func (db *DB) UpdateWorkspace(w *models.Workspace) error {
	db.encryptWorkspaceFields(w)
	wType := w.WorkspaceType
	if wType == "" {
		wType = "container"
	}
	_, err := db.Exec(`UPDATE workspace SET name=$2, friendly_name=$3, description=$4, image_src=$5,
		docker_image=$6, cores=$7, memory=$8, category=$9,
		docker_registry=$10, docker_user=$11, docker_password=$12,
		session_time_limit=$13, gpu_count=$14, uncompressed_size_mb=$15, shm_size=$16,
		restrict_to_agent=$17, restrict_to_region=$18,
		run_config=$19, exec_config=$20, volume_mappings=$21,
		categories=$22, notes=$23, persistent=$24, persistent_size=$25,
		workspace_type=$26, server_hostname=$27, server_port=$28, server_protocol=$29,
		server_username=$30, server_password=$31, server_domain=$32, server_ignore_cert=$33, server_security=$34,
		server_auth_mode=$35, server_allow_remember=$36, server_default_settings=$37,
		record_sessions=$38, updated_at=NOW() WHERE id=$1`,
		w.ID, w.Name, w.FriendlyName, w.Description, w.ImageSrc, w.DockerImage,
		w.Cores, w.Memory, w.Category,
		w.DockerRegistry, w.DockerUser, w.DockerPassword,
		w.SessionTimeLimit, w.GPUCount, w.UncompressedSizeMB, w.SHMSize,
		w.RestrictToAgent, w.RestrictToRegion,
		w.RunConfig, w.ExecConfig, w.VolumeMappings,
		w.Categories, w.Notes, w.Persistent, w.PersistentSize,
		wType, w.ServerHostname, w.ServerPort, w.ServerProtocol,
		w.ServerUsername, w.ServerPassword, w.ServerDomain, w.ServerIgnoreCert, w.ServerSecurity, w.ServerAuthMode, w.ServerAllowRemember, w.ServerDefaultSettings,
		w.RecordSessions)
	return err
}

func (db *DB) RemoveWorkspace(id string) error {
	// Delete associated sessions first (FK constraint)
	if _, err := db.Exec(`DELETE FROM workspace_session WHERE workspace_id = $1`, id); err != nil {
		log.Printf("RemoveWorkspace delete sessions error: %v", err)
	}
	// Delete group-workspace associations
	if _, err := db.Exec(`DELETE FROM workspace_group WHERE workspace_id = $1`, id); err != nil {
		log.Printf("RemoveWorkspace delete group associations error: %v", err)
	}
	res, err := db.Exec(`DELETE FROM workspace WHERE id = $1`, id)
	if err != nil {
		log.Printf("RemoveWorkspace error: %v", err)
		return err
	}
	rows, _ := res.RowsAffected()
	log.Printf("RemoveWorkspace id=%s rows_affected=%d", id, rows)
	return nil
}

func (db *DB) ToggleWorkspace(id string, enabled bool) {
	if _, err := db.Exec(`UPDATE workspace SET enabled = $1, updated_at = NOW() WHERE id = $2`, enabled, id); err != nil {
		log.Printf("ToggleWorkspace error: %v", err)
	}
}

func (db *DB) GetExpiredSessions() ([]models.WorkspaceSession, error) {
	var sessions []models.WorkspaceSession
	err := db.Select(&sessions, `SELECT id, user_id, workspace_id, pod_name, service_name, container_ip, vnc_password, status, agent_id, COALESCE(session_type, 'container') as session_type, created_at, updated_at, expires_at, keepalive_at FROM workspace_session WHERE expires_at < NOW() AND status = 'running'`)
	return sessions, err
}

func (db *DB) DeleteExpiredSession(id string) {
	if _, err := db.Exec(`DELETE FROM workspace_session WHERE id = $1`, id); err != nil {
		log.Printf("DeleteExpiredSession error: %v", err)
	}
}

// Paginated queries

func (db *DB) GetWorkspacesPaginated(page, perPage int, search string) ([]models.Workspace, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	var total int
	if search != "" {
		db.Get(&total, `SELECT COUNT(*) FROM workspace WHERE friendly_name ILIKE $1 OR docker_image ILIKE $1`, "%"+search+"%")
	} else {
		db.Get(&total, `SELECT COUNT(*) FROM workspace`)
	}

	var workspaces []models.Workspace
	query := `SELECT id, name, friendly_name, description, image_src, docker_image,
		cores, memory, shm_size, enabled, category, x_res, y_res,
		COALESCE(docker_registry,'') as docker_registry, COALESCE(docker_user,'') as docker_user, COALESCE(docker_password,'') as docker_password,
		COALESCE(session_time_limit,0) as session_time_limit, COALESCE(gpu_count,0) as gpu_count, COALESCE(uncompressed_size_mb,0) as uncompressed_size_mb,
		COALESCE(restrict_to_agent,'') as restrict_to_agent, COALESCE(restrict_to_region,'') as restrict_to_region,
		COALESCE(run_config,'{}') as run_config, COALESCE(exec_config,'{}') as exec_config, COALESCE(volume_mappings,'{}') as volume_mappings,
		COALESCE(categories,'{}') as categories, notes, COALESCE(persistent,false) as persistent, COALESCE(persistent_size,'') as persistent_size,
		COALESCE(workspace_type,'container') as workspace_type, COALESCE(server_hostname,'') as server_hostname, COALESCE(server_port,0) as server_port,
		COALESCE(server_protocol,'') as server_protocol, COALESCE(server_username,'') as server_username, COALESCE(server_password,'') as server_password,
		COALESCE(server_domain,'') as server_domain, COALESCE(server_ignore_cert,true) as server_ignore_cert, COALESCE(server_security,'') as server_security, COALESCE(server_auth_mode,'static') as server_auth_mode, COALESCE(server_allow_remember,false) as server_allow_remember, COALESCE(server_default_settings,'{}') as server_default_settings,
		COALESCE(record_sessions,false) as record_sessions
		FROM workspace`

	if search != "" {
		query += ` WHERE friendly_name ILIKE $1 OR docker_image ILIKE $1 ORDER BY friendly_name LIMIT $2 OFFSET $3`
		err := db.Select(&workspaces, query, "%"+search+"%", perPage, offset)
		if err == nil {
			db.decryptWorkspaces(workspaces)
		}
		return workspaces, total, err
	}

	query += ` ORDER BY friendly_name LIMIT $1 OFFSET $2`
	err := db.Select(&workspaces, query, perPage, offset)
	if err == nil {
		db.decryptWorkspaces(workspaces)
	}
	return workspaces, total, err
}

func (db *DB) GetActiveSessionsCursor(cursor string, perPage int, search string) ([]models.WorkspaceSessionWithImage, string, error) {
	if perPage < 1 {
		perPage = 25
	}

	where := `WHERE ws.status IN ('running', 'starting')`
	args := []interface{}{}
	argN := 1

	if cursor != "" {
		t, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		where += fmt.Sprintf(" AND ws.created_at < $%d", argN)
		args = append(args, t)
		argN++
	}
	if search != "" {
		where += fmt.Sprintf(` AND (w.friendly_name ILIKE $%d OR COALESCE(u.email,'') ILIKE $%d OR COALESCE(u.name,'') ILIKE $%d)`, argN, argN, argN)
		args = append(args, "%"+search+"%")
		argN++
	}

	query := fmt.Sprintf(`SELECT ws.id, ws.user_id, ws.workspace_id, ws.pod_name, ws.service_name,
		ws.container_ip, ws.vnc_password, ws.status, COALESCE(ws.agent_id, '') as agent_id, COALESCE(ws.session_type, 'container') as session_type, ws.created_at, ws.expires_at, ws.keepalive_at,
		w.friendly_name as image_name, w.image_src as image_src, w.docker_image as docker_image, COALESCE(w.workspace_type, 'container') as workspace_type,
		COALESCE(u.name, '') as user_name, COALESCE(u.email, '') as user_email
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		LEFT JOIN "user" u ON u.id = ws.user_id
		%s ORDER BY ws.created_at DESC LIMIT %d`, where, perPage+1)

	var sessions []models.WorkspaceSessionWithImage
	if err := db.Select(&sessions, query, args...); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(sessions) > perPage {
		nextCursor = sessions[perPage-1].CreatedAt.Format(time.RFC3339Nano)
		sessions = sessions[:perPage]
	}

	return sessions, nextCursor, nil
}

func (db *DB) GetActiveSessionsPaginated(page, perPage int, search string) ([]models.WorkspaceSessionWithImage, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 25
	}
	offset := (page - 1) * perPage

	baseWhere := `WHERE ws.status IN ('running', 'starting')`
	args := []interface{}{}
	argN := 1

	if search != "" {
		baseWhere += fmt.Sprintf(` AND (w.friendly_name ILIKE $%d OR COALESCE(u.email,'') ILIKE $%d OR COALESCE(u.name,'') ILIKE $%d)`, argN, argN, argN)
		args = append(args, "%"+search+"%")
		argN++
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		LEFT JOIN "user" u ON u.id = ws.user_id
		%s`, baseWhere)
	db.Get(&total, countQuery, args...)

	var sessions []models.WorkspaceSessionWithImage
	query := fmt.Sprintf(`SELECT ws.id, ws.user_id, ws.workspace_id, ws.pod_name, ws.service_name,
		ws.container_ip, ws.vnc_password, ws.status, COALESCE(ws.agent_id, '') as agent_id, COALESCE(ws.session_type, 'container') as session_type, ws.created_at, ws.expires_at, ws.keepalive_at,
		w.friendly_name as image_name, w.image_src as image_src, w.docker_image as docker_image, COALESCE(w.workspace_type, 'container') as workspace_type,
		COALESCE(u.name, '') as user_name, COALESCE(u.email, '') as user_email
		FROM workspace_session ws
		JOIN workspace w ON w.id = ws.workspace_id
		LEFT JOIN "user" u ON u.id = ws.user_id
		%s ORDER BY ws.created_at DESC LIMIT %d OFFSET %d`, baseWhere, perPage, offset)
	err := db.Select(&sessions, query, args...)
	return sessions, total, err
}
