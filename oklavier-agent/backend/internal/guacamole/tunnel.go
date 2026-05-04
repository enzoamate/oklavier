package guacamole

import (
	"encoding/base64"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// ScreenshotCallback is called when a large image blob is captured from guacd.
type ScreenshotCallback func(pngData []byte)

// Tunnel bridges a WebSocket connection (guacamole-common-js) to a guacd TCP connection.
type Tunnel struct {
	guacdConn     net.Conn
	closed        bool
	mu            sync.Mutex
	OnScreenshot  ScreenshotCallback
	SessionExists func() bool // returns false when session is destroyed → closes tunnel
}

// NewTunnel creates a tunnel from an existing guacd connection.
func NewTunnel(guacdConn net.Conn) *Tunnel {
	return &Tunnel{guacdConn: guacdConn}
}

// HandleWebSocket runs the bidirectional relay between WebSocket and guacd.
// This blocks until either side closes the connection.
func (t *Tunnel) HandleWebSocket(ws *websocket.Conn) {
	done := make(chan struct{})

	// guacd → WebSocket
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		buf := make([]byte, 64*1024)
		var msgBuf strings.Builder
		lastCapture := time.Time{}
		for {
			t.guacdConn.SetReadDeadline(time.Now().Add(60 * time.Second))
			n, err := t.guacdConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Printf("[GuacTunnel] guacd read error: %v", err)
				}
				return
			}
			if n > 0 {
				// Capture screenshots from large blob instructions (every 5s max)
				if t.OnScreenshot != nil && time.Since(lastCapture) > 5*time.Second {
					msgBuf.Write(buf[:n])
					raw := msgBuf.String()
					// Look for large blob instructions: "4.blob,<stream>,<len>.<base64data>;"
					if idx := strings.Index(raw, "4.blob,"); idx >= 0 {
						if end := strings.IndexByte(raw[idx:], ';'); end > 0 {
							blobInst := raw[idx : idx+end]
							// Extract base64 data after the last dot-prefixed field
							parts := strings.SplitN(blobInst, ",", 3)
							if len(parts) == 3 {
								dotIdx := strings.IndexByte(parts[2], '.')
								if dotIdx > 0 {
									b64data := parts[2][dotIdx+1:]
									if len(b64data) > 2000 { // only capture large frames
										if decoded, err := base64.StdEncoding.DecodeString(b64data); err == nil {
											t.OnScreenshot(decoded)
											lastCapture = time.Now()
										}
									}
								}
							}
						}
					}
					// Keep buffer small
					if msgBuf.Len() > 128*1024 {
						msgBuf.Reset()
					}
				}

				if err := ws.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					log.Printf("[GuacTunnel] ws write error: %v", err)
					return
				}
			}
		}
	}()

	// WebSocket → guacd
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		first := true
		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("[GuacTunnel] ws read error: %v", err)
				}
				return
			}
			if first {
				preview := string(msg)
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				log.Printf("[GuacTunnel] first ws→guacd (type=%d, %d bytes): %s", mt, len(msg), preview)
				first = false
			}
			t.guacdConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := t.guacdConn.Write(msg); err != nil {
				log.Printf("[GuacTunnel] guacd write error: %v", err)
				return
			}
		}
	}()

	// Poll Valkey to detect session destruction (replaces KillCh)
	if t.SessionExists != nil {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if !t.SessionExists() {
						log.Printf("[GuacTunnel] Session no longer exists in Valkey, closing tunnel")
						t.Close()
						ws.Close()
						return
					}
				case <-done:
					return
				}
			}
		}()
	}

	<-done
	t.Close()
}

// Close cleans up the tunnel.
func (t *Tunnel) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		t.guacdConn.Close()
	}
}
