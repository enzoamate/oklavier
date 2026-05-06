package guacamole

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// CreateServerSessionRequest is the request from the core backend.
type CreateServerSessionRequest struct {
	SessionID       string `json:"session_id"`
	UserID          string `json:"user_id"`
	Lang            string `json:"lang"`
	DefaultSettings string `json:"default_settings"`
	Protocol        string `json:"protocol"` // "rdp" or "vnc"
	Hostname        string `json:"hostname"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Domain          string `json:"domain"`
	IgnoreCert      bool   `json:"ignore_cert"`
	Security        string `json:"security"` // "any", "nla", "tls", "rdp"
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	ReadOnly        bool   `json:"read_only"` // shadow session: read-only connection
	// Recording
	RecordSessions bool   `json:"record_sessions"`
	WorkspaceName  string `json:"workspace_name"`
	// S3 config (passed from core when recording is enabled)
	S3Endpoint  string `json:"s3_endpoint"`
	S3AccessKey string `json:"s3_access_key"`
	S3SecretKey string `json:"s3_secret_key"`
	S3Bucket    string `json:"s3_bucket"`
	S3Region    string `json:"s3_region"`
}

// ServerSession tracks an active server session (serializable to JSON for Valkey storage).
type ServerSession struct {
	SessionID       string            `json:"session_id"`
	UserID          string            `json:"user_id"`
	Lang            string            `json:"lang"`
	DefaultSettings string            `json:"default_settings"`
	Protocol        string            `json:"protocol"`
	Hostname        string            `json:"hostname"`
	Port            int               `json:"port"`
	Params          map[string]string `json:"params"`
	IsShadow        bool              `json:"is_shadow"`
	RecordingPath   string            `json:"recording_path"`
	WorkspaceName   string            `json:"workspace_name"`
	S3Endpoint      string            `json:"s3_endpoint"`
	S3AccessKey     string            `json:"s3_access_key"`
	S3SecretKey     string            `json:"s3_secret_key"`
	S3Bucket        string            `json:"s3_bucket"`
	S3Region        string            `json:"s3_region"`
}

const (
	sessionKeyPrefix    = "session:"
	screenshotKeyPrefix = "screenshot:"
	sessionTTL          = 24 * time.Hour
	screenshotTTL       = 60 * time.Second
	valkeyTimeout       = 5 * time.Second
)

// Manager manages guacamole server sessions via Valkey (Redis).
type Manager struct {
	guacdAddr string
	valkey    *redis.Client
}

// NewManager creates a new guacamole session manager backed by Valkey.
func NewManager(guacdAddr string, valkey *redis.Client) *Manager {
	return &Manager{
		guacdAddr: guacdAddr,
		valkey:    valkey,
	}
}

// CreateSession registers a server session in Valkey.
func (m *Manager) CreateSession(req CreateServerSessionRequest) error {
	key := sessionKeyPrefix + req.SessionID

	// Check if session already exists
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	exists, err := m.valkey.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("valkey exists check: %w", err)
	}
	if exists > 0 {
		return fmt.Errorf("session %s already exists", req.SessionID)
	}

	params := m.buildParams(req)

	lang := req.Lang
	if lang == "" {
		lang = "en"
	}
	recordingPath := ""
	if req.RecordSessions {
		recordingPath = "/tmp/guac-recordings/" + req.SessionID
	}

	sess := &ServerSession{
		SessionID:       req.SessionID,
		UserID:          req.UserID,
		Lang:            lang,
		DefaultSettings: req.DefaultSettings,
		Protocol:        req.Protocol,
		Hostname:        req.Hostname,
		Port:            req.Port,
		Params:          params,
		RecordingPath:   recordingPath,
		WorkspaceName:   req.WorkspaceName,
		IsShadow:        req.ReadOnly,
		S3Endpoint:      req.S3Endpoint,
		S3AccessKey:     req.S3AccessKey,
		S3SecretKey:     req.S3SecretKey,
		S3Bucket:        req.S3Bucket,
		S3Region:        req.S3Region,
	}

	data, err := json.Marshal(sess)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel2()
	if err := m.valkey.Set(ctx2, key, data, sessionTTL).Err(); err != nil {
		return fmt.Errorf("valkey set: %w", err)
	}

	log.Printf("[GuacManager] Session %s registered in Valkey: %s → %s:%d",
		req.SessionID, req.Protocol, req.Hostname, req.Port)
	return nil
}

// GetSession returns a session by ID from Valkey.
func (m *Manager) GetSession(sessionID string) *ServerSession {
	key := sessionKeyPrefix + sessionID
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	data, err := m.valkey.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}
	var sess ServerSession
	if err := json.Unmarshal(data, &sess); err != nil {
		log.Printf("[GuacManager] Failed to unmarshal session %s: %v", sessionID, err)
		return nil
	}
	return &sess
}

// SessionExists checks if a session key exists in Valkey (lightweight check for tunnel polling).
func (m *Manager) SessionExists(sessionID string) bool {
	key := sessionKeyPrefix + sessionID
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	exists, err := m.valkey.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

// GetSessionInfo returns session info needed for recording upload before destroy.
func (m *Manager) GetSessionInfo(sessionID string) (recordingPath, userID, workspaceName, s3Endpoint, s3AccessKey, s3SecretKey, s3Bucket, s3Region string) {
	sess := m.GetSession(sessionID)
	if sess == nil {
		return
	}
	return sess.RecordingPath, sess.UserID, sess.WorkspaceName, sess.S3Endpoint, sess.S3AccessKey, sess.S3SecretKey, sess.S3Bucket, sess.S3Region
}

// DestroySession removes a session from Valkey. Active tunnels will detect the missing key and close.
func (m *Manager) DestroySession(sessionID string) error {
	key := sessionKeyPrefix + sessionID

	// Check existence first for error reporting
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	exists, err := m.valkey.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("valkey exists: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Delete session and screenshot keys
	ctx2, cancel2 := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel2()
	m.valkey.Del(ctx2, key, screenshotKeyPrefix+sessionID)

	log.Printf("[GuacManager] Session %s destroyed (removed from Valkey)", sessionID)
	return nil
}

// ConnectForTunnel opens a TCP connection to guacd, performs the handshake, then returns
// the raw TCP connection for bidirectional relay.
func (m *Manager) ConnectForTunnel(sessionID string, clientWidth, clientHeight int) (net.Conn, error) {
	sess := m.GetSession(sessionID)
	if sess == nil {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Open raw TCP to guacd
	conn, err := net.DialTimeout("tcp", m.guacdAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("guacd connect: %w", err)
	}

	// Step 1: send "select"
	if _, err := io.WriteString(conn, encodeInstruction("select", sess.Protocol)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("select: %w", err)
	}

	// Step 2: read "args" byte-by-byte (no bufio to avoid stealing data from relay)
	args, err := readInstructionRaw(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read args: %w", err)
	}

	fields := decodeInstruction(args)
	if len(fields) == 0 || fields[0] != "args" {
		conn.Close()
		return nil, fmt.Errorf("expected args, got: %s", args)
	}
	// Step 3: send client capabilities — use actual browser viewport if provided
	w := strconv.Itoa(clientWidth)
	h := strconv.Itoa(clientHeight)
	if clientWidth <= 0 {
		w = sess.Params["width"]
		if w == "" {
			w = "1920"
		}
	}
	if clientHeight <= 0 {
		h = sess.Params["height"]
		if h == "" {
			h = "1080"
		}
	}
	io.WriteString(conn, encodeInstruction("size", w, h, "96"))
	// Declare supported audio formats (guacamole-common-js can play L8/L16 via Web Audio API)
	if sess.Params["disable-audio"] != "true" {
		io.WriteString(conn, encodeInstruction("audio", "audio/L8", "audio/L16"))
	} else {
		io.WriteString(conn, encodeInstruction("audio"))
	}
	io.WriteString(conn, encodeInstruction("video"))
	io.WriteString(conn, encodeInstruction("image", "image/png", "image/jpeg"))

	// Step 4: send "connect" with params in the order guacd expects
	argNames := fields[1:]
	connectArgs := make([]string, len(argNames))
	for i, name := range argNames {
		// Handle protocol version negotiation — guacd sends VERSION_X_Y_Z as first arg
		if strings.HasPrefix(name, "VERSION_") {
			connectArgs[i] = name // echo back the version to confirm support
		} else {
			connectArgs[i] = sess.Params[name]
		}
	}
	if _, err := io.WriteString(conn, encodeInstruction("connect", connectArgs...)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect: %w", err)
	}

	// Don't read "ready" here — let the relay forward it to the browser
	log.Printf("[GuacManager] Tunnel handshake done for session %s (%s → %s:%d)", sessionID, sess.Protocol, sess.Hostname, sess.Port)
	return conn, nil
}

// Allowed settings that can be updated from the viewer
var allowedSettings = map[string]bool{
	"enable-font-smoothing": true, "enable-wallpaper": true, "enable-theming": true,
	"enable-desktop-composition": true, "enable-full-window-drag": true, "enable-menu-animations": true,
	"console-audio": true, "disable-audio": true, "color-depth": true, "server-layout": true,
	"timezone": true, "display-scale": true, "dpi": true,
}

// UpdateSessionParams updates whitelisted guacd connection parameters for a session in Valkey.
func (m *Manager) UpdateSessionParams(sessionID string, updates map[string]string) {
	sess := m.GetSession(sessionID)
	if sess == nil {
		return
	}
	for k, v := range updates {
		if allowedSettings[k] {
			sess.Params[k] = v
		}
	}
	// Write back to Valkey
	data, err := json.Marshal(sess)
	if err != nil {
		log.Printf("[GuacManager] Failed to marshal updated session %s: %v", sessionID, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	m.valkey.Set(ctx, sessionKeyPrefix+sessionID, data, sessionTTL)
}

// SetScreenshot stores a screenshot for the session in Valkey.
func (m *Manager) SetScreenshot(sessionID string, data []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	m.valkey.Set(ctx, screenshotKeyPrefix+sessionID, data, screenshotTTL)
}

// GetScreenshot returns the last screenshot for the session from Valkey.
func (m *Manager) GetScreenshot(sessionID string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	data, err := m.valkey.Get(ctx, screenshotKeyPrefix+sessionID).Bytes()
	if err != nil {
		return nil
	}
	return data
}

// SessionCount returns the number of active server sessions in Valkey.
func (m *Manager) SessionCount() int {
	ctx, cancel := context.WithTimeout(context.Background(), valkeyTimeout)
	defer cancel()
	keys, err := m.valkey.Keys(ctx, sessionKeyPrefix+"*").Result()
	if err != nil {
		return 0
	}
	return len(keys)
}

// readInstructionRaw reads a single guacp instruction byte-by-byte from a raw connection.
// No bufio to avoid stealing data from the relay.
func readInstructionRaw(conn net.Conn) (string, error) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var buf strings.Builder
	b := make([]byte, 1)
	for {
		_, err := conn.Read(b)
		if err != nil {
			return "", err
		}
		buf.WriteByte(b[0])
		if b[0] == ';' {
			conn.SetReadDeadline(time.Time{}) // clear deadline
			return buf.String(), nil
		}
	}
}

func (m *Manager) buildParams(req CreateServerSessionRequest) map[string]string {
	width := req.Width
	height := req.Height
	if width == 0 {
		width = 1920
	}
	if height == 0 {
		height = 1080
	}

	params := map[string]string{
		"hostname": req.Hostname,
		"port":     strconv.Itoa(req.Port),
		"width":    strconv.Itoa(width),
		"height":   strconv.Itoa(height),
	}

	// Default keyboard layout based on lang
	langLayouts := map[string]string{"fr": "fr-fr-azerty", "de": "de-de-qwertz", "es": "es-es-qwerty", "it": "it-it-qwerty", "pt": "pt-br-qwerty", "sv": "sv-se-qwerty", "ja": "ja-jp-qwerty"}
	defaultLayout := "en-us-qwerty"
	if layout, ok := langLayouts[req.Lang]; ok {
		defaultLayout = layout
	}

	switch req.Protocol {
	case "rdp":
		params["username"] = req.Username
		params["password"] = req.Password
		if req.Domain != "" {
			params["domain"] = req.Domain
		}
		if req.IgnoreCert {
			params["ignore-cert"] = "true"
		}
		if req.Security != "" {
			params["security"] = req.Security
		}
		params["color-depth"] = "32"
		params["resize-method"] = "display-update"
		params["server-layout"] = defaultLayout
		params["enable-font-smoothing"] = "true"
		params["enable-wallpaper"] = "true"
		params["enable-theming"] = "true"
		params["enable-desktop-composition"] = "true"
		params["enable-full-window-drag"] = "true"
		params["enable-menu-animations"] = "true"
		params["client-name"] = "Cloud"
		params["enable-printing"] = "true"
		params["printer-name"] = "Oklavier PDF"
		params["enable-drive"] = "true"
		params["drive-name"] = "Oklavier Drive"
		params["drive-path"] = "/tmp/guac-drive"
		params["create-drive-path"] = "true"
		// Microphone redirection — the browser pushes mic samples through a
		// Guac audio output stream that guacd forwards as RDP audio-input.
		// Toggle is exposed in the Peripherals modal; declaring the channel
		// here just unlocks it on the wire.
		params["enable-audio-input"] = "true"
		// Audio is enabled by declaring MIME types in the handshake
		// console-audio is only for admin/console RDP sessions
	case "vnc":
		if req.Password != "" {
			params["password"] = req.Password
		}
		params["color-depth"] = "24"
		params["disable-audio"] = "true"
	}

	// Session recording (not for shadow sessions)
	if req.RecordSessions && !req.ReadOnly {
		params["recording-path"] = "/tmp/guac-recordings"
		params["recording-name"] = req.SessionID
		params["recording-include-keys"] = "true"
		params["create-recording-path"] = "true"
	}

	// Read-only mode for shadow sessions
	if req.ReadOnly {
		params["read-only"] = "true"
	}

	return params
}
