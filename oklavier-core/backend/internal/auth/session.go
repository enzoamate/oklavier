package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	BcryptCost        = 12
	SessionTokenBytes = 32
	SessionTTL        = 7 * 24 * time.Hour // 7 days
	MaxLoginAttempts  = 5
	LockoutDuration   = 15 * time.Minute
	RateLimitWindow   = time.Minute
	RateLimitMax      = 5
)

// Password hashing
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	return string(bytes), err
}

func VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Session tokens
func GenerateSessionToken() (raw string, hashed string, err error) {
	b := make([]byte, SessionTokenBytes)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hashed = hex.EncodeToString(h[:])
	return raw, hashed, nil
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// Timing-safe token comparison
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Input validation
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func ValidateEmail(email string) error {
	if len(email) < 3 || len(email) > 254 {
		return fmt.Errorf("invalid email length")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password too long")
	}
	var hasUpper, hasLower, hasNumber bool
	for _, c := range password {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasNumber = true
		}
	}
	if !hasUpper || !hasLower || !hasNumber {
		return fmt.Errorf("password must contain uppercase, lowercase, and a number")
	}
	return nil
}

// Rate limiter with Valkey backend and in-memory fallback
type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	client   *redis.Client
	fallback *inMemoryRateLimiter
}

func NewRateLimiter(valkeyURL string) *RateLimiter {
	rl := &RateLimiter{
		fallback: newInMemoryRateLimiter(),
	}
	if valkeyURL != "" {
		rl.client = redis.NewClient(&redis.Options{
			Addr: valkeyURL,
		})
		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := rl.client.Ping(ctx).Err(); err != nil {
			log.Printf("Valkey connection failed, using in-memory rate limiter: %v", err)
			rl.client = nil
		} else {
			log.Println("Rate limiter connected to Valkey")
		}
	}
	return rl
}

// Ping checks if the Valkey/Redis connection is alive.
func (rl *RateLimiter) Ping() bool {
	if rl == nil {
		return false
	}
	if rl.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return rl.client.Ping(ctx).Err() == nil
	}
	// In-memory fallback is always "healthy"
	return true
}

func (rl *RateLimiter) Allow(ip string) bool {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}

	if rl.client != nil {
		ctx := context.Background()
		key := "ratelimit:" + host
		count, err := rl.client.Incr(ctx, key).Result()
		if err == nil {
			if count == 1 {
				rl.client.Expire(ctx, key, RateLimitWindow)
			}
			return count <= int64(RateLimitMax)
		}
		// Valkey error, fall back to in-memory
	}
	return rl.fallback.Allow(ip)
}

// Get remaining attempts for an IP
func (rl *RateLimiter) Remaining(ip string) int {
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}

	if rl.client != nil {
		ctx := context.Background()
		count, err := rl.client.Get(ctx, "ratelimit:"+host).Int()
		if err == nil {
			remaining := RateLimitMax - count
			if remaining < 0 {
				return 0
			}
			return remaining
		}
	}
	return rl.fallback.Remaining(ip)
}

// Keep the old in-memory implementation as fallback
type inMemoryRateLimiter struct {
	mu      sync.RWMutex
	entries map[string]*rateLimitEntry
}

func newInMemoryRateLimiter() *inMemoryRateLimiter {
	rl := &inMemoryRateLimiter{entries: make(map[string]*rateLimitEntry)}
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.mu.Lock()
			now := time.Now()
			for k, v := range rl.entries {
				if now.After(v.resetAt) {
					delete(rl.entries, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *inMemoryRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}
	now := time.Now()
	entry, exists := rl.entries[host]
	if !exists || now.After(entry.resetAt) {
		rl.entries[host] = &rateLimitEntry{count: 1, resetAt: now.Add(RateLimitWindow)}
		return true
	}
	entry.count++
	return entry.count <= RateLimitMax
}

func (rl *inMemoryRateLimiter) Remaining(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}
	entry, exists := rl.entries[host]
	if !exists || time.Now().After(entry.resetAt) {
		return RateLimitMax
	}
	remaining := RateLimitMax - entry.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

