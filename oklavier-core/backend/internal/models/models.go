package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	UserID           uuid.UUID  `db:"user_id" json:"user_id"`
	Username         string     `db:"username" json:"username"`
	PasswordHash     string     `db:"pw_hash" json:"-"`
	Salt             string     `db:"salt" json:"-"`
	FirstName        *string    `db:"first_name" json:"first_name"`
	LastName         *string    `db:"last_name" json:"last_name"`
	Locked           bool       `db:"locked" json:"locked"`
	Realm            string     `db:"realm" json:"realm"`
	Anonymous        bool       `db:"anonymous" json:"is_anonymous"`
	CreatedDate      time.Time  `db:"created_date" json:"created_date"`
	FailedPWAttempts int        `db:"failed_pw_attempts" json:"-"`
}

type SessionToken struct {
	SessionTokenID uuid.UUID `db:"session_token_id" json:"session_token_id"`
	UserID         uuid.UUID `db:"user_id" json:"user_id"`
	Token          string    `json:"token"`
	ExpirationDate time.Time `db:"expiration_date" json:"expiration_date"`
	Authorizations []int     `json:"authorizations"`
}

type Image struct {
	ImageID       uuid.UUID `db:"image_id" json:"image_id"`
	FriendlyName  string    `db:"friendly_name" json:"friendly_name"`
	Name          string    `db:"name" json:"name"`
	Description   *string   `db:"description" json:"description"`
	ImageSrc      string    `db:"image_src" json:"image_src"`
	Cores         float64   `db:"cores" json:"cores"`
	Memory        int64     `db:"memory" json:"memory"`
	Enabled       bool      `db:"enabled" json:"enabled"`
	ImageType     string    `db:"image_type" json:"image_type"`
	Hidden        bool      `db:"hidden" json:"hidden"`
	XRes          int       `db:"x_res" json:"x_res"`
	YRes          int       `db:"y_res" json:"y_res"`
}

type Session struct {
	SessionID            uuid.UUID  `db:"session_id" json:"session_id"`
	ContainerID       *string    `db:"container_id" json:"container_id"`
	ContainerIP       *string    `db:"container_ip" json:"container_ip"`
	Hostname          *string    `db:"hostname" json:"hostname"`
	OperationalStatus string     `db:"operational_status" json:"operational_status"`
	ServerID          *uuid.UUID `db:"server_id" json:"server_id"`
	UserID            uuid.UUID  `db:"user_id" json:"user_id"`
	ImageID           uuid.UUID  `db:"image_id" json:"image_id"`
	StartDate         *time.Time `db:"start_date" json:"start_date"`
	CreatedDate       time.Time  `db:"created_date" json:"created_date"`
	KeepaliveDate     *time.Time `db:"keepalive_date" json:"keepalive_date"`
	ExpirationDate    *time.Time `db:"expiration_date" json:"expiration_date"`
	Token             *string    `db:"token" json:"-"`
	ViewOnlyToken     *string    `db:"view_only_token" json:"-"`
	DockerNetwork     *string    `db:"docker_network" json:"docker_network"`
}

type Server struct {
	ServerID          uuid.UUID `db:"server_id" json:"server_id"`
	Hostname          string    `db:"hostname" json:"hostname"`
	Port              int       `db:"port" json:"port"`
	OperationalStatus string    `db:"operational_status" json:"operational_status"`
	Memory            int64     `db:"memory" json:"memory"`
	Cores             int       `db:"cores" json:"cores"`
}

type Zone struct {
	ZoneID        uuid.UUID `db:"zone_id" json:"zone_id"`
	ZoneName      string    `db:"zone_name" json:"zone_name"`
	ProxyHostname string    `db:"proxy_hostname" json:"proxy_hostname"`
	ProxyPort     int       `db:"proxy_port" json:"proxy_port"`
}

type Group struct {
	GroupID uuid.UUID `db:"group_id" json:"group_id"`
	Name    string    `db:"name" json:"name"`
}

// Auth
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token           string `json:"token"`
	UserID          string `json:"user_id"`
	Username        string `json:"username"`
	IsAdmin         bool   `json:"is_admin"`
	AuthorizedViews []int  `json:"authorized_views"`
}

type LoginSettings struct {
	LoginLogo          string        `json:"login_logo"`
	LoginCaption       string        `json:"login_caption"`
	HeaderLogo         string        `json:"header_logo"`
	HTMLTitle          string        `json:"html_title"`
	FaviconLogo        string        `json:"favicon_logo"`
	UsernameInputLabel string        `json:"username_input_label"`
	NoticeMessage      *string       `json:"notice_message"`
	NoticeTitle        string        `json:"notice_title"`
	LoginAssistance    *string       `json:"login_assistance"`
	SAML               SAMLConfig    `json:"saml"`
	OIDC               OIDCConfig    `json:"oidc"`
}

type SAMLConfig struct {
	Configs []SAMLProvider `json:"saml_configs"`
}

type SAMLProvider struct {
	SAMLID      string  `json:"saml_id"`
	DisplayName string  `json:"display_name"`
	LogoURL     *string `json:"logo_url"`
}

type OIDCConfig struct {
	Configs []OIDCProvider `json:"oidc_configs"`
}

type OIDCProvider struct {
	OIDCID      string  `json:"oidc_id"`
	DisplayName string  `json:"display_name"`
	LogoURL     *string `json:"logo_url"`
	LoginURL    *string `json:"login_url"`
}
