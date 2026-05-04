package auth

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenBlacklist stores revoked JWT token IDs (JTI) with TTL.
// Uses Redis/Valkey when available, falls back to in-memory.
type TokenBlacklist struct {
	client   *redis.Client
	fallback *inMemoryBlacklist
}

func NewTokenBlacklist(valkeyURL string) *TokenBlacklist {
	bl := &TokenBlacklist{
		fallback: newInMemoryBlacklist(),
	}
	if valkeyURL != "" {
		bl.client = redis.NewClient(&redis.Options{
			Addr: valkeyURL,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := bl.client.Ping(ctx).Err(); err != nil {
			log.Printf("Blacklist: Valkey connection failed, using in-memory: %v", err)
			bl.client = nil
		} else {
			log.Println("Blacklist connected to Valkey")
		}
	}
	return bl
}

// Blacklist adds a token JTI to the blacklist with a TTL equal to the remaining token lifetime.
func (bl *TokenBlacklist) Blacklist(jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // already expired, no need to blacklist
	}
	if bl.client != nil {
		ctx := context.Background()
		return bl.client.Set(ctx, "blacklist:"+jti, "1", ttl).Err()
	}
	bl.fallback.Set(jti, ttl)
	return nil
}

// IsBlacklisted checks if a token JTI has been revoked.
func (bl *TokenBlacklist) IsBlacklisted(jti string) bool {
	if bl.client != nil {
		ctx := context.Background()
		val, err := bl.client.Exists(ctx, "blacklist:"+jti).Result()
		if err != nil {
			// Valkey error, fall back
			return bl.fallback.Has(jti)
		}
		return val > 0
	}
	return bl.fallback.Has(jti)
}

// In-memory blacklist fallback
type blacklistEntry struct {
	expiresAt time.Time
}

type inMemoryBlacklist struct {
	mu      sync.RWMutex
	entries map[string]*blacklistEntry
}

func newInMemoryBlacklist() *inMemoryBlacklist {
	bl := &inMemoryBlacklist{entries: make(map[string]*blacklistEntry)}
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			bl.mu.Lock()
			now := time.Now()
			for k, v := range bl.entries {
				if now.After(v.expiresAt) {
					delete(bl.entries, k)
				}
			}
			bl.mu.Unlock()
		}
	}()
	return bl
}

func (bl *inMemoryBlacklist) Set(jti string, ttl time.Duration) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.entries[jti] = &blacklistEntry{expiresAt: time.Now().Add(ttl)}
}

func (bl *inMemoryBlacklist) Has(jti string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	entry, exists := bl.entries[jti]
	if !exists {
		return false
	}
	if time.Now().After(entry.expiresAt) {
		return false
	}
	return true
}
