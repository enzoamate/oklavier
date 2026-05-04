package models

import (
	"time"

	"github.com/lib/pq"
)

type Workspace struct {
	ID           string  `db:"id" json:"id"`
	Name         string  `db:"name" json:"name"`
	FriendlyName string  `db:"friendly_name" json:"friendly_name"`
	Description  *string `db:"description" json:"description"`
	ImageSrc     string  `db:"image_src" json:"image_src"`
	DockerImage  string  `db:"docker_image" json:"docker_image"`
	Cores        float64 `db:"cores" json:"cores"`
	Memory       int64   `db:"memory" json:"memory"`
	SHMSize      string  `db:"shm_size" json:"shm_size"`
	Enabled      bool    `db:"enabled" json:"enabled"`
	Category     string  `db:"category" json:"category"`
	XRes         int     `db:"x_res" json:"x_res"`
	YRes         int     `db:"y_res" json:"y_res"`

	// Registry auth
	DockerRegistry string `db:"docker_registry" json:"docker_registry"`
	DockerUser     string `db:"docker_user" json:"docker_user"`
	DockerPassword string `db:"docker_password" json:"docker_password"`

	// Session / resources
	SessionTimeLimit  int   `db:"session_time_limit" json:"session_time_limit"`
	GPUCount          int   `db:"gpu_count" json:"gpu_count"`
	UncompressedSizeMB int  `db:"uncompressed_size_mb" json:"uncompressed_size_mb"`

	// Restrictions
	RestrictToAgent  string `db:"restrict_to_agent" json:"restrict_to_agent"`
	RestrictToRegion string `db:"restrict_to_region" json:"restrict_to_region"`

	// Advanced JSON configs (stored as TEXT in DB)
	RunConfig      string `db:"run_config" json:"run_config"`
	ExecConfig     string `db:"exec_config" json:"exec_config"`
	VolumeMappings string `db:"volume_mappings" json:"volume_mappings"`

	// Extra
	Categories     pq.StringArray `db:"categories" json:"categories"`
	Notes          *string        `db:"notes" json:"notes"`
	Persistent     bool           `db:"persistent" json:"persistent"`
	PersistentSize string         `db:"persistent_size" json:"persistent_size"`

	// Server workspace
	WorkspaceType    string `db:"workspace_type" json:"workspace_type"`       // "container" or "server"
	ServerHostname   string `db:"server_hostname" json:"server_hostname"`
	ServerPort       int    `db:"server_port" json:"server_port"`
	ServerProtocol   string `db:"server_protocol" json:"server_protocol"`     // "rdp" or "vnc"
	ServerUsername   string `db:"server_username" json:"server_username"`
	ServerPassword   string `db:"server_password" json:"server_password"`
	ServerDomain     string `db:"server_domain" json:"server_domain"`
	ServerIgnoreCert bool   `db:"server_ignore_cert" json:"server_ignore_cert"`
	ServerSecurity   string `db:"server_security" json:"server_security"`     // "any", "nla", "tls", "rdp"
	ServerAuthMode       string `db:"server_auth_mode" json:"server_auth_mode"`             // "static" or "prompt"
	ServerAllowRemember  bool   `db:"server_allow_remember" json:"server_allow_remember"`   // allow users to save credentials
	ServerDefaultSettings string `db:"server_default_settings" json:"server_default_settings"` // JSON default display settings

	// Recording
	RecordSessions bool `db:"record_sessions" json:"record_sessions"`
}

type WorkspaceSession struct {
	ID          string     `db:"id" json:"id"`
	UserID      string     `db:"user_id" json:"user_id"`
	WorkspaceID string     `db:"workspace_id" json:"workspace_id"`
	PodName     string     `db:"pod_name" json:"pod_name"`
	ServiceName string     `db:"service_name" json:"service_name"`
	ContainerIP string     `db:"container_ip" json:"container_ip"`
	VNCPassword string     `db:"vnc_password" json:"vnc_password"`
	Status      string     `db:"status" json:"status"`
	AgentID     string     `db:"agent_id" json:"agent_id"`
	SessionType string     `db:"session_type" json:"session_type"` // "container" or "server"
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at"`
	KeepaliveAt *time.Time `db:"keepalive_at" json:"keepalive_at"`
}

type WorkspaceSessionWithImage struct {
	WorkspaceSession
	ImageName     string `db:"image_name" json:"image_name"`
	ImageSrc      string `db:"image_src" json:"image_src"`
	DockerImage   string `db:"docker_image" json:"docker_image"`
	WorkspaceType string `db:"workspace_type" json:"workspace_type"`
	UserName      string `db:"user_name" json:"user_name"`
	UserEmail     string `db:"user_email" json:"user_email"`
}
